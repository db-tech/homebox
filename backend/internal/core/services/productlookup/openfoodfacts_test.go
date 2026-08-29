package productlookup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stub returns a service pointed at a local test server instead of the real
// OpenFoodFacts. handler also records what was requested.
func stub(t *testing.T, handler http.HandlerFunc) (*Service, *httptest.Server) {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return &Service{
		enabled:  true,
		endpoint: srv.URL + "/api/v2/product/%s.json",
		client:   &http.Client{Timeout: 4 * time.Second},
	}, srv
}

func TestLookup_FoundPrefersGermanName(t *testing.T) {
	svc, _ := stub(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":1,"product":{
			"product_name":"Chopped Tomatoes",
			"product_name_de":"Gehackte Tomaten",
			"brands":"Mutti, Barilla",
			"quantity":"400 g"}}`))
	})

	got, err := svc.Lookup(context.Background(), "4001234567890")
	require.NoError(t, err)

	assert.True(t, got.Found)
	assert.Equal(t, "Gehackte Tomaten", got.Name)
	assert.Equal(t, "Mutti", got.Brand, "only the first brand should be taken")
	assert.Equal(t, "400 g", got.Amount)
}

func TestLookup_FallsBackToInternationalName(t *testing.T) {
	svc, _ := stub(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":1,"product":{"product_name":"Chopped Tomatoes","brands":"Mutti"}}`))
	})

	got, err := svc.Lookup(context.Background(), "4001234567890")
	require.NoError(t, err)
	assert.True(t, got.Found)
	assert.Equal(t, "Chopped Tomatoes", got.Name)
}

// An unknown barcode is a normal outcome, not a failure.
func TestLookup_UnknownBarcodeIsNotAnError(t *testing.T) {
	svc, _ := stub(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":0}`))
	})

	got, err := svc.Lookup(context.Background(), "4001234567890")
	require.NoError(t, err)
	assert.False(t, got.Found)
	assert.Empty(t, got.Name)
}

func TestLookup_NotFoundStatusIsNotAnError(t *testing.T) {
	svc, _ := stub(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	got, err := svc.Lookup(context.Background(), "4001234567890")
	require.NoError(t, err)
	assert.False(t, got.Found)
}

// An entry with no usable name must not produce an empty suggestion; that just
// looks broken in the UI.
func TestLookup_EntryWithoutNameCountsAsNotFound(t *testing.T) {
	svc, _ := stub(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":1,"product":{"product_name":"  ","brands":"Mutti"}}`))
	})

	got, err := svc.Lookup(context.Background(), "4001234567890")
	require.NoError(t, err)
	assert.False(t, got.Found)
}

func TestLookup_UpstreamErrorIsReported(t *testing.T) {
	svc, _ := stub(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := svc.Lookup(context.Background(), "4001234567890")
	require.Error(t, err)
}

func TestLookup_SendsIdentifyingUserAgent(t *testing.T) {
	var gotUA string
	svc, _ := stub(t, func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`{"status":0}`))
	})

	_, err := svc.Lookup(context.Background(), "4001234567890")
	require.NoError(t, err)
	assert.Contains(t, gotUA, "Homebox", "OpenFoodFacts asks clients to identify themselves")
}

// The request must carry the barcode and nothing else about the user.
func TestLookup_SendsOnlyTheBarcode(t *testing.T) {
	var gotPath, gotQuery, gotCookie string
	svc, _ := stub(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotCookie = r.Header.Get("Cookie")
		_, _ = w.Write([]byte(`{"status":0}`))
	})

	_, err := svc.Lookup(context.Background(), "4001234567890")
	require.NoError(t, err)

	assert.Equal(t, "/api/v2/product/4001234567890.json", gotPath)
	assert.Empty(t, gotQuery)
	assert.Empty(t, gotCookie)
}

// With the feature switched off nothing may leave the server at all.
func TestLookup_DisabledMakesNoRequest(t *testing.T) {
	called := false
	svc, _ := stub(t, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		_, _ = w.Write([]byte(`{"status":1,"product":{"product_name":"x"}}`))
	})
	svc.enabled = false

	_, err := svc.Lookup(context.Background(), "4001234567890")
	require.ErrorIs(t, err, ErrLookupDisabled)
	assert.False(t, called, "a disabled lookup must not reach out")
	assert.False(t, svc.Enabled())
}

// Anything that is not a plausible EAN/UPC is rejected before a request is made,
// so a stray QR payload never reaches a third party.
func TestLookup_ImplausibleBarcodeNeverLeavesTheServer(t *testing.T) {
	for _, code := range []string{
		"",
		"123",
		"abcdefgh",
		"4001234567890123456",
		"400123456789x",
		"https://example.com/secret",
	} {
		called := false
		svc, _ := stub(t, func(w http.ResponseWriter, _ *http.Request) {
			called = true
			_, _ = w.Write([]byte(`{"status":0}`))
		})

		got, err := svc.Lookup(context.Background(), code)
		require.NoError(t, err, "code %q", code)
		assert.False(t, got.Found, "code %q", code)
		assert.False(t, called, "code %q must not be sent upstream", code)
	}
}

func TestLookup_AcceptsCommonBarcodeLengths(t *testing.T) {
	// EAN-8, UPC-A, EAN-13 and ITF-14.
	for _, code := range []string{"12345670", "012345678905", "4001234567890", "14001234567890"} {
		called := false
		svc, _ := stub(t, func(w http.ResponseWriter, _ *http.Request) {
			called = true
			_, _ = w.Write([]byte(`{"status":0}`))
		})

		_, err := svc.Lookup(context.Background(), code)
		require.NoError(t, err)
		assert.True(t, called, "code %q should be looked up", code)
	}
}

func TestLookup_HonoursContextCancellation(t *testing.T) {
	svc, _ := stub(t, func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		_, _ = w.Write([]byte(`{"status":0}`))
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := svc.Lookup(ctx, "4001234567890")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "context") || strings.Contains(err.Error(), "deadline"),
		"expected a context error, got %v", err)
}
