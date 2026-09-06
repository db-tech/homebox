// Package mealideas turns what is actually in the pantry into a handful of
// concrete things to cook, picked so that whatever goes off first gets used up.
//
// This is the second and larger of the two places where Homebox talks to a
// third party, and the only one that sends anything about what you own. It
// sends the name, the quantity and the days left of pantry items, and nothing
// else: no locations, no prices, no notes, no account or group identifier. It
// is off unless Recipes.Enabled is set, and it only ever runs when somebody
// presses the button - never in the background, never on a page load.
//
// The model is deliberately not asked to work anything out. What is in stock
// and what expires when is decided here, before the request goes out; the model
// only chooses dishes and explains them. Anything it names that is not on the
// list it was given is dropped, so a suggestion can never rest on an
// ingredient that was invented.
package mealideas

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

const defaultEndpoint = "https://api.anthropic.com/v1/messages"

// defaultModel is the small, fast model. The job is choosing among a short list
// of ingredients, which does not need a large one.
const defaultModel = "claude-haiku-4-5-20251001"

const anthropicVersion = "2023-06-01"

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
	enabled  bool
	apiKey   string
	model    string
	endpoint string
	client   *http.Client
}

func New(enabled bool, apiKey, model string) *Service {
	if model == "" {
		model = defaultModel
	}

	return &Service{
		enabled:  enabled,
		apiKey:   strings.TrimSpace(apiKey),
		model:    model,
		endpoint: defaultEndpoint,
		// Longer than the barcode lookup on purpose: this is a deliberate tap
		// with a spinner, not somebody standing there with a tin in hand.
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *Service) Enabled() bool {
	return s != nil && s.enabled && s.apiKey != ""
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

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Suggest asks for meal ideas built on the given stock.
func (s *Service) Suggest(ctx context.Context, stock []Stock) ([]Idea, error) {
	switch {
	case s == nil || !s.enabled:
		return nil, ErrDisabled
	case s.apiKey == "":
		return nil, ErrNoKey
	case len(stock) == 0:
		return nil, ErrEmptyPantry
	}

	body, err := json.Marshal(anthropicRequest{
		Model:     s.model,
		MaxTokens: 1024,
		System:    systemPrompt,
		Messages:  []anthropicMessage{{Role: "user", Content: describe(stock)}},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", s.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var parsed anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("meal ideas: unreadable response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if parsed.Error != nil {
			return nil, fmt.Errorf("meal ideas: %s", parsed.Error.Message)
		}
		return nil, fmt.Errorf("meal ideas: unexpected status %d", resp.StatusCode)
	}

	return parseIdeas(textOf(parsed), stock)
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

func textOf(r anthropicResponse) string {
	var b strings.Builder
	for _, part := range r.Content {
		if part.Type == "text" {
			b.WriteString(part.Text)
		}
	}
	return b.String()
}

// parseIdeas reads the model's answer and strips anything it made up.
func parseIdeas(text string, stock []Stock) ([]Idea, error) {
	raw := strings.TrimSpace(text)

	// A fenced block is asked against but arrives often enough to be worth
	// tolerating rather than failing on.
	if start := strings.Index(raw, "{"); start >= 0 {
		if end := strings.LastIndex(raw, "}"); end > start {
			raw = raw[start : end+1]
		}
	}

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
