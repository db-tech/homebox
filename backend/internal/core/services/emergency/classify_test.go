package emergency

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

func classifier(t *testing.T, reply string, status int) (*Classifier, *[]byte) {
	t.Helper()

	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(server.Close)

	c := NewClassifier(true, "server-key", "")
	c.client.Endpoint = server.URL

	return c, &body
}

func answered(items string) string {
	payload, _ := json.Marshal(map[string]any{
		"content": []map[string]string{{"type": "text", "text": items}},
	})
	return string(payload)
}

func TestClassify_SortsNamesIntoGroups(t *testing.T) {
	c, _ := classifier(t, answered(`{"items":[
		{"name":"Ravioli Dose","group":"grains","grams":800},
		{"name":"Sonnenblumenöl","group":"fats","grams":1000},
		{"name":"Akkuschrauber","group":"none","grams":0}
	]}`), http.StatusOK)

	got, err := c.Classify(context.Background(), "group-key", []string{"Ravioli Dose", "Sonnenblumenöl", "Akkuschrauber"})
	require.NoError(t, err)

	assert.Equal(t, Grains, got["Ravioli Dose"].Category)
	assert.Equal(t, 800, got["Ravioli Dose"].Grams)
	assert.Equal(t, Fats, got["Sonnenblumenöl"].Category)

	// "none" has to arrive as no category at all, not as a bogus one.
	assert.Empty(t, got["Akkuschrauber"].Category)
	assert.Equal(t, 0, got["Akkuschrauber"].Grams)
}

func TestClassify_MatchesNamesBackDespiteTidiedCapitalisation(t *testing.T) {
	// The model reformats names even when told not to. A suggestion that cannot
	// be matched back would be dropped, which looks like the model ignoring the
	// item.
	c, _ := classifier(t, answered(`{"items":[{"name":"ravioli dose","group":"grains","grams":800}]}`), http.StatusOK)

	got, err := c.Classify(context.Background(), "k", []string{"Ravioli Dose"})
	require.NoError(t, err)
	assert.Equal(t, Grains, got["Ravioli Dose"].Category, "keyed by the name that went in")
}

func TestClassify_NamesNobodyAskedAboutAreDropped(t *testing.T) {
	// Writing a group onto an item that was not in the request would be the
	// worst available outcome: a wrong figure on an item nobody looked at.
	c, _ := classifier(t, answered(`{"items":[
		{"name":"Reis","group":"grains","grams":500},
		{"name":"Erfundenes Produkt","group":"dairy","grams":500}
	]}`), http.StatusOK)

	got, err := c.Classify(context.Background(), "k", []string{"Reis"})
	require.NoError(t, err)

	assert.Len(t, got, 1)
	assert.Contains(t, got, "Reis")
	assert.NotContains(t, got, "Erfundenes Produkt")
}

func TestClassify_RejectsAnInvalidGroup(t *testing.T) {
	c, _ := classifier(t, answered(`{"items":[{"name":"Schrauben","group":"hardware","grams":100}]}`), http.StatusOK)

	got, err := c.Classify(context.Background(), "k", []string{"Schrauben"})
	require.NoError(t, err)
	assert.Empty(t, got["Schrauben"].Category, "an unknown group must not become a real one")
}

func TestClassify_ImplausibleSizesAreDiscarded(t *testing.T) {
	c, _ := classifier(t, answered(`{"items":[
		{"name":"Salz","group":"none","grams":1},
		{"name":"Kartoffeln","group":"grains","grams":900000},
		{"name":"Mehl","group":"grains","grams":-500}
	]}`), http.StatusOK)

	got, err := c.Classify(context.Background(), "k", []string{"Salz", "Kartoffeln", "Mehl"})
	require.NoError(t, err)

	// Zero means "nobody has said", which keeps the item out of the totals
	// rather than putting a nonsense figure into them.
	assert.Equal(t, 0, got["Salz"].Grams)
	assert.Equal(t, 0, got["Kartoffeln"].Grams)
	assert.Equal(t, 0, got["Mehl"].Grams)
	assert.Equal(t, Grains, got["Kartoffeln"].Category, "a bad size must not lose a good group")
}

func TestClassify_PartialAnswerIsStillUseful(t *testing.T) {
	c, _ := classifier(t, answered(`{"items":[{"name":"Reis","group":"grains","grams":500}]}`), http.StatusOK)

	got, err := c.Classify(context.Background(), "k", []string{"Reis", "Nudeln", "Linsen"})
	require.NoError(t, err)

	assert.Len(t, got, 1, "the two it skipped are simply absent")
	assert.Contains(t, got, "Reis")
}

func TestClassify_SendsEveryNameOnceAndUsesTheGroupKey(t *testing.T) {
	c, body := classifier(t, answered(`{"items":[]}`), http.StatusOK)

	_, err := c.Classify(context.Background(), "group-key", []string{"Reis", "reis", "  ", "Nudeln"})
	require.NoError(t, err)

	sent := string(*body)
	assert.Equal(t, 1, strings.Count(strings.ToLower(sent), "reis"), "duplicates collapse")
	assert.Contains(t, sent, "Nudeln")
}

func TestClassify_NothingToDoMakesNoRequest(t *testing.T) {
	c, body := classifier(t, answered(`{"items":[]}`), http.StatusOK)

	got, err := c.Classify(context.Background(), "k", []string{"", "   "})
	require.NoError(t, err)

	assert.Empty(t, got)
	assert.Empty(t, *body, "nothing should have been sent")
}

func TestClassify_DisabledAndKeylessAreDistinct(t *testing.T) {
	off := NewClassifier(false, "k", "")
	_, err := off.Classify(context.Background(), "k", []string{"Reis"})
	assert.ErrorIs(t, err, ErrClassifyDisabled)

	keyless := NewClassifier(true, "", "")
	_, err = keyless.Classify(context.Background(), "", []string{"Reis"})
	assert.ErrorIs(t, err, ErrClassifyNoKey)
}

func TestClassify_ApiErrorIsReported(t *testing.T) {
	c, _ := classifier(t, `{"error":{"message":"rate limited"}}`, http.StatusTooManyRequests)

	_, err := c.Classify(context.Background(), "k", []string{"Reis"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limited")
}

func TestClassify_ToleratesAFencedAnswer(t *testing.T) {
	fenced := "```json\n{\"items\":[{\"name\":\"Reis\",\"group\":\"grains\",\"grams\":500}]}\n```"
	c, _ := classifier(t, answered(fenced), http.StatusOK)

	got, err := c.Classify(context.Background(), "k", []string{"Reis"})
	require.NoError(t, err)
	assert.Equal(t, Grains, got["Reis"].Category)
}
