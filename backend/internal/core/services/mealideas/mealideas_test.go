package mealideas

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func days(n int) *int { return &n }

// stub answers like the Anthropic API and records what it was sent, so a test
// can assert on the request as well as on the reply.
func stub(t *testing.T, reply string, status int) (*Service, *http.Request, *[]byte) {
	t.Helper()

	var seen http.Request
	var body []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = *r
		body, _ = io.ReadAll(r.Body)

		w.Header().Set("content-type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(server.Close)

	svc := New(true, "test-key", "")
	svc.endpoint = server.URL

	return svc, &seen, &body
}

func answer(ideas string) string {
	payload, _ := json.Marshal(map[string]any{
		"content": []map[string]string{{"type": "text", "text": ideas}},
	})
	return string(payload)
}

func TestSuggest_DisabledSendsNothing(t *testing.T) {
	svc := New(false, "test-key", "")
	_, err := svc.Suggest(context.Background(), "", []Stock{{Name: "Sahne", Quantity: 1}})
	assert.ErrorIs(t, err, ErrDisabled)
}

func TestSuggest_NoKeyAnywhereIsRefused(t *testing.T) {
	svc := New(true, "  ", "")

	// The server permits it; nobody has configured a key. That is not an error
	// in the deployment, it is a thing the user still has to do.
	assert.True(t, svc.Enabled())

	_, err := svc.Suggest(context.Background(), "", []Stock{{Name: "Sahne", Quantity: 1}})
	assert.ErrorIs(t, err, ErrNoKey)
}

func TestSuggest_GroupKeyIsUsedInPreferenceToTheServerOne(t *testing.T) {
	svc, seen, _ := stub(t, answer(`{"ideas":[{"title":"S","why":"w","uses":[],"missing":[]}]}`), http.StatusOK)

	_, err := svc.Suggest(context.Background(), "group-key", []Stock{{Name: "Reis", Quantity: 1}})
	require.NoError(t, err)

	assert.Equal(t, "group-key", seen.Header.Get("x-api-key"))
}

func TestSuggest_ServerKeyStandsInWhenTheGroupHasNone(t *testing.T) {
	svc, seen, _ := stub(t, answer(`{"ideas":[{"title":"S","why":"w","uses":[],"missing":[]}]}`), http.StatusOK)

	_, err := svc.Suggest(context.Background(), "   ", []Stock{{Name: "Reis", Quantity: 1}})
	require.NoError(t, err)

	assert.Equal(t, "test-key", seen.Header.Get("x-api-key"), "the service was built with this one")
}

func TestSuggest_DisabledBeatsAnyStoredKey(t *testing.T) {
	svc := New(false, "", "")

	_, err := svc.Suggest(context.Background(), "group-key", []Stock{{Name: "Reis", Quantity: 1}})
	assert.ErrorIs(t, err, ErrDisabled, "a group must not be able to override the server saying no")
}

func TestSuggest_EmptyPantryNeverReachesOut(t *testing.T) {
	svc, _, body := stub(t, answer(`{"ideas":[]}`), http.StatusOK)

	_, err := svc.Suggest(context.Background(), "", nil)
	assert.ErrorIs(t, err, ErrEmptyPantry)
	assert.Empty(t, *body, "nothing should have been sent")
}

func TestSuggest_SendsOnlyNameQuantityAndDaysLeft(t *testing.T) {
	svc, seen, body := stub(t, answer(`{"ideas":[{"title":"Rahmsauce","why":"Sahne","uses":["Sahne"],"missing":[]}]}`), http.StatusOK)

	_, err := svc.Suggest(context.Background(), "", []Stock{
		{Name: "Sahne", Quantity: 2, DaysLeft: days(3)},
		{Name: "Reis", Quantity: 1},
	})
	require.NoError(t, err)

	sent := string(*body)
	assert.Contains(t, sent, "Sahne")
	assert.Contains(t, sent, "3 days left")
	assert.Contains(t, sent, "no best-before date")

	// The whole promise of the feature is that nothing else about the pantry
	// leaves the server. If a field is ever added to Stock, this should fail.
	for _, forbidden := range []string{"location", "Location", "price", "Price", "groupId", "id\":"} {
		assert.NotContains(t, sent, forbidden, "%q must never be sent", forbidden)
	}

	assert.Equal(t, "test-key", seen.Header.Get("x-api-key"))
	assert.Equal(t, anthropicVersion, seen.Header.Get("anthropic-version"))
}

func TestSuggest_MostUrgentItemGoesFirst(t *testing.T) {
	svc, _, body := stub(t, answer(`{"ideas":[{"title":"X","why":"y","uses":[],"missing":[]}]}`), http.StatusOK)

	_, err := svc.Suggest(context.Background(), "", []Stock{
		{Name: "Reis", Quantity: 1},
		{Name: "Nudeln", Quantity: 1, DaysLeft: days(200)},
		{Name: "Sahne", Quantity: 1, DaysLeft: days(2)},
		{Name: "Joghurt", Quantity: 1, DaysLeft: days(-4)},
	})
	require.NoError(t, err)

	sent := string(*body)
	order := []string{"Joghurt", "Sahne", "Nudeln", "Reis"}
	at := -1
	for _, name := range order {
		next := strings.Index(sent, name)
		require.Greater(t, next, at, "%s is out of order", name)
		at = next
	}

	assert.Contains(t, sent, "4 days PAST its best-before date")
}

func TestSuggest_InventedIngredientsAreNotClaimedAsInStock(t *testing.T) {
	svc, _, _ := stub(t, answer(
		`{"ideas":[{"title":"Rahmschnitzel","why":"Sahne","uses":["Sahne","Schweineschnitzel"],"missing":[]}]}`,
	), http.StatusOK)

	ideas, err := svc.Suggest(context.Background(), "", []Stock{{Name: "Sahne", Quantity: 1, DaysLeft: days(2)}})
	require.NoError(t, err)
	require.Len(t, ideas, 1)

	assert.Equal(t, []string{"Sahne"}, ideas[0].Uses, "only what is really there may be claimed")
	assert.Contains(t, ideas[0].Missing, "Schweineschnitzel", "the rest becomes something to buy")
}

func TestSuggest_MatchesItemNamesRegardlessOfCase(t *testing.T) {
	svc, _, _ := stub(t, answer(
		`{"ideas":[{"title":"Reisgericht","why":"weil","uses":["reis"],"missing":[]}]}`,
	), http.StatusOK)

	ideas, err := svc.Suggest(context.Background(), "", []Stock{{Name: "Reis", Quantity: 1}})
	require.NoError(t, err)

	// Reported back with the spelling the pantry uses, so the name can be
	// matched against the item list in the UI.
	assert.Equal(t, []string{"Reis"}, ideas[0].Uses)
}

func TestSuggest_ToleratesAFencedAnswer(t *testing.T) {
	svc, _, _ := stub(t, answer("```json\n{\"ideas\":[{\"title\":\"Suppe\",\"why\":\"w\",\"uses\":[],\"missing\":[]}]}\n```"), http.StatusOK)

	ideas, err := svc.Suggest(context.Background(), "", []Stock{{Name: "Brühe", Quantity: 1}})
	require.NoError(t, err)
	assert.Equal(t, "Suppe", ideas[0].Title)
}

func TestSuggest_UnusableAnswerIsAnError(t *testing.T) {
	svc, _, _ := stub(t, answer("I could not think of anything."), http.StatusOK)

	_, err := svc.Suggest(context.Background(), "", []Stock{{Name: "Reis", Quantity: 1}})
	assert.Error(t, err)
}

func TestSuggest_EmptyIdeaListIsAnError(t *testing.T) {
	svc, _, _ := stub(t, answer(`{"ideas":[{"title":"   ","why":"","uses":[],"missing":[]}]}`), http.StatusOK)

	_, err := svc.Suggest(context.Background(), "", []Stock{{Name: "Reis", Quantity: 1}})
	assert.Error(t, err, "a suggestion with no title is not a suggestion")
}

func TestSuggest_ApiErrorIsReported(t *testing.T) {
	svc, _, _ := stub(t, `{"error":{"message":"credit balance is too low"}}`, http.StatusBadRequest)

	_, err := svc.Suggest(context.Background(), "", []Stock{{Name: "Reis", Quantity: 1}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credit balance is too low")
}
