// Package mealideas turns what is actually in the pantry into a handful of
// concrete things to cook, picked so that whatever goes off first gets used up.
//
// This is the second and larger of the two places where Homebox talks to a
// third party, and the only one that sends anything about what you own. It
// sends the name, the quantity and the days left of pantry items, and nothing
// else: no locations, no prices, no notes, no account or group identifier. It
// It runs only when somebody presses the button - never in the background,
// never on a page load - and only when a key has been configured. The key
// normally comes from the group, set in the settings, so a household can switch
// this on for itself; a server-wide key may be supplied instead. Setting
// Recipes.Enabled to false forbids it outright, whatever any group has stored.
//
// The model is deliberately not asked to work anything out. What is in stock
// and what expires when is decided here, before the request goes out; the model
// only chooses dishes and explains them. Anything it names that is not on the
// list it was given is dropped, so a suggestion can never rest on an
// ingredient that was invented.
package mealideas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sysadminsmedia/homebox/backend/internal/core/services/llm"
)

var (
	// ErrDisabled is returned when the feature is switched off by config.
	ErrDisabled = errors.New("meal ideas are disabled")
	// ErrNoKey is returned when it is switched on but no API key was given.
	ErrNoKey = errors.New("meal ideas are enabled but no API key is configured")
	// ErrEmptyPantry is returned when there is nothing to suggest anything from.
	ErrEmptyPantry = errors.New("no pantry items to work from")
)

// Stock is one line of what the pantry holds, as it goes out.
type Stock struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
	// DaysLeft is nil when the item has no best-before date. It is worked out
	// here rather than sent as a date, so the model never has to know today's
	// date to reason about freshness.
	DaysLeft *int `json:"daysLeft,omitempty"`
}

// Idea is one thing to cook.
type Idea struct {
	Title string `json:"title"`
	// Why says what makes this worth cooking now, which is nearly always that
	// something in it is about to go off.
	Why string `json:"why"`
	// Uses names items from the pantry, guaranteed to exist: anything the model
	// named that was not on the list is removed before this is returned.
	Uses []string `json:"uses"`
	// Missing is what you would have to buy. Not checked against anything -
	// it is by definition not in the pantry.
	Missing []string `json:"missing"`
}

type Service struct {
	enabled bool
	// apiKey is the server-wide fallback, used when a group has none of its own.
	apiKey string
	client *llm.Client
}

func New(enabled bool, apiKey, model string) *Service {
	return &Service{
		enabled: enabled,
		apiKey:  strings.TrimSpace(apiKey),
		// Longer than the barcode lookup on purpose: this is a deliberate tap
		// with a spinner, not somebody standing there with a tin in hand.
		client: llm.NewClient(model, 30*time.Second),
	}
}

// Enabled says whether the server permits meal ideas at all. It says nothing
// about whether a key has been configured - that is per group, and asking here
// would need a group to ask about.
func (s *Service) Enabled() bool {
	return s != nil && s.enabled
}

const systemPrompt = `You suggest meals from what somebody already has in their pantry.

You are given a list of items with a quantity and, where known, how many days are left before the item is past its best-before date. A negative number means it is already past it.

Rules:
- Suggest 2 to 3 dishes, ordered so that the one using the most soon-to-expire items comes first.
- Only list an item under "uses" if it appears in the pantry list, spelled exactly as given.
- Everything else a dish needs goes under "missing". Assume salt, pepper, oil and water are on hand and never mention them.
- Quantities are counts of packages, not weights, so do not state amounts and do not write out a recipe. One or two sentences per dish.
- "why" says what makes this worth cooking now. Name the item that is running out of time.
- Do not suggest anything for an item that is already well past its date; say it should be thrown out instead of cooked with.
- Write in the same language as the item names.

Reply with JSON only, no prose and no code fence:
{"ideas":[{"title":"...","why":"...","uses":["..."],"missing":["..."]}]}`

// Suggest asks for meal ideas built on the given stock, using the group's key
// or, when it has none, the server-wide one.
func (s *Service) Suggest(ctx context.Context, apiKey string, stock []Stock) ([]Idea, error) {
	if s == nil || !s.enabled {
		return nil, ErrDisabled
	}

	key := strings.TrimSpace(apiKey)
	if key == "" {
		key = s.apiKey
	}

	switch {
	case key == "":
		return nil, ErrNoKey
	case len(stock) == 0:
		return nil, ErrEmptyPantry
	}

	text, err := s.client.Complete(ctx, key, systemPrompt, describe(stock), 1024)
	if err != nil {
		return nil, err
	}

	return parseIdeas(text, stock)
}

// describe renders the pantry as the short list the model works from. Sorted by
// urgency so the ordering carries information even before it is read.
func describe(stock []Stock) string {
	ordered := make([]Stock, len(stock))
	copy(ordered, stock)
	sort.SliceStable(ordered, func(i, j int) bool {
		return urgency(ordered[i]) < urgency(ordered[j])
	})

	var b strings.Builder
	b.WriteString("Pantry:\n")

	for _, line := range ordered {
		fmt.Fprintf(&b, "- %s (%d)", line.Name, line.Quantity)
		switch {
		case line.DaysLeft == nil:
			b.WriteString(", no best-before date")
		case *line.DaysLeft < 0:
			fmt.Fprintf(&b, ", %d days PAST its best-before date", -*line.DaysLeft)
		default:
			fmt.Fprintf(&b, ", %d days left", *line.DaysLeft)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// urgency sorts dated items ahead of undated ones, soonest first.
func urgency(s Stock) int {
	if s.DaysLeft == nil {
		return 1 << 30
	}
	return *s.DaysLeft
}

// parseIdeas reads the model's answer and strips anything it made up.
func parseIdeas(text string, stock []Stock) ([]Idea, error) {
	raw := llm.ExtractJSON(text)

	var parsed struct {
		Ideas []Idea `json:"ideas"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("meal ideas: could not read the suggestions: %w", err)
	}

	known := make(map[string]string, len(stock))
	for _, line := range stock {
		known[strings.ToLower(strings.TrimSpace(line.Name))] = line.Name
	}

	ideas := make([]Idea, 0, len(parsed.Ideas))
	for _, idea := range parsed.Ideas {
		idea.Title = strings.TrimSpace(idea.Title)
		if idea.Title == "" {
			continue
		}

		// A dish resting on an ingredient that is not in the cupboard is worse
		// than no suggestion: it reads as "you have everything" when you do
		// not. Anything unrecognised is moved to what you would have to buy.
		uses := make([]string, 0, len(idea.Uses))
		for _, used := range idea.Uses {
			if actual, ok := known[strings.ToLower(strings.TrimSpace(used))]; ok {
				uses = append(uses, actual)
			} else if trimmed := strings.TrimSpace(used); trimmed != "" {
				idea.Missing = append(idea.Missing, trimmed)
			}
		}

		idea.Uses = uses
		ideas = append(ideas, idea)
	}

	if len(ideas) == 0 {
		return nil, errors.New("meal ideas: no usable suggestion came back")
	}

	return ideas, nil
}
