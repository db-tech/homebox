package emergency

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sysadminsmedia/homebox/backend/internal/core/services/llm"
)

// Sorting existing items into the stockpiling groups.
//
// Assigning a group is a judgement about a name - "Ravioli Dose" belongs with
// the grains - and a language model is good at that. A package size is not: how
// much is in a tin of tomatoes is a fact about that product, and nothing in the
// name says whether it is 400 g or 2.5 kg.
//
// So a size from here is explicitly an estimate, offered only where the product
// database has nothing, and marked as such all the way to the screen. The
// percentage on the stockpile page has to stay something you can trust, which
// means being able to see what it rests on.

// Suggestion is what to put on one item.
type Suggestion struct {
	// Category is empty when the item is not food at all, which is the answer
	// for most of a household inventory.
	Category Category `json:"category"`
	// Grams is 0 when no sensible size could be given.
	Grams int `json:"grams"`
}

// An estimate outside these bounds is likelier a mistake than a package: below
// it a rounding error, above it nothing that lives in a kitchen cupboard.
const (
	minGrams = 5
	maxGrams = 25_000
)

type Classifier struct {
	enabled bool
	apiKey  string
	client  *llm.Client
}

func NewClassifier(enabled bool, apiKey, model string) *Classifier {
	return &Classifier{
		enabled: enabled,
		apiKey:  strings.TrimSpace(apiKey),
		// A list of a hundred names is one call, but a slow one.
		client: llm.NewClient(model, 60*time.Second),
	}
}

func (c *Classifier) Enabled() bool {
	return c != nil && c.enabled
}

const classifySystemPrompt = `You sort household inventory items into the groups of the German federal food stockpiling recommendation.

For each name you are given, return:
- "group": one of "drinks", "vegetables", "grains", "fruit", "dairy", "protein", "fats", or "none".
- "grams": the size of one typical package in grams, counting millilitres as grams. Use 0 if you cannot give a sensible figure.

The groups mean:
- drinks: water, juice, anything drunk as it is
- vegetables: vegetables, mushrooms, pulses, beans, lentils, tinned tomatoes
- grains: pasta, rice, bread, flour, cereal, potatoes, biscuits, and tinned meals whose bulk is pasta or potato
- fruit: fruit, dried fruit, nuts
- dairy: milk, cheese, yoghurt, cream, condensed milk
- protein: meat, sausage, fish, eggs, tinned meat dishes
- fats: oil, butter, margarine, lard

Rules:
- "none" is the right answer for most items. Tools, cables, furniture, cleaning products, medicines, pet supplies and anything not eaten are all "none". Do not stretch to find a food group.
- Coffee beans, loose tea, salt, pepper, spices, sugar and baking ingredients are "none". Nobody lives off them and the recommendation does not count them.
- Return every name you were given, exactly as given, once each. Invent no names.
- A "grams" figure is a typical package for that kind of product. If the name gives no clue, use 0 rather than guessing wildly.

Reply with JSON only, no prose and no code fence:
{"items":[{"name":"...","group":"none","grams":0}]}`

// Classify sorts the given item names. The result is keyed by the name as it was
// passed in; names the model failed to return are simply absent, so a partial
// answer is still useful.
func (c *Classifier) Classify(ctx context.Context, apiKey string, names []string) (map[string]Suggestion, error) {
	if c == nil || !c.enabled {
		return nil, ErrClassifyDisabled
	}

	key := strings.TrimSpace(apiKey)
	if key == "" {
		key = c.apiKey
	}
	if key == "" {
		return nil, ErrClassifyNoKey
	}

	wanted := make(map[string]string, len(names))
	list := make([]string, 0, len(names))

	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		// Matched back case-insensitively: the model tidies capitalisation even
		// when told not to, and an unmatched name would be silently dropped.
		folded := strings.ToLower(trimmed)
		if _, seen := wanted[folded]; seen {
			continue
		}
		wanted[folded] = trimmed
		list = append(list, trimmed)
	}

	if len(list) == 0 {
		return map[string]Suggestion{}, nil
	}

	text, err := c.client.Complete(ctx, key, classifySystemPrompt, strings.Join(list, "\n"), 4096)
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Items []struct {
			Name  string `json:"name"`
			Group string `json:"group"`
			Grams int    `json:"grams"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(llm.ExtractJSON(text)), &parsed); err != nil {
		return nil, fmt.Errorf("stockpile classify: could not read the answer: %w", err)
	}

	out := make(map[string]Suggestion, len(parsed.Items))

	for _, entry := range parsed.Items {
		original, ok := wanted[strings.ToLower(strings.TrimSpace(entry.Name))]
		if !ok {
			// A name nobody asked about. Dropping it is the only safe thing:
			// applying it would write a group onto the wrong item.
			continue
		}

		suggestion := Suggestion{}

		if strings.TrimSpace(entry.Group) != "none" {
			if category, valid := ParseCategory(entry.Group); valid {
				suggestion.Category = category
			}
		}

		if entry.Grams >= minGrams && entry.Grams <= maxGrams {
			suggestion.Grams = entry.Grams
		}

		out[original] = suggestion
	}

	return out, nil
}
