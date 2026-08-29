// Package productlookup resolves a scanned barcode to a product name using the
// OpenFoodFacts database.
//
// This is the only part of Homebox that talks to a third party. It sends the
// barcode digits and nothing else - no item names, no quantities, no account or
// group identifier - and only when a user scans a code that no local item
// carries. It is off entirely when Options.ProductLookup is false.
package productlookup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// defaultEndpoint is the OpenFoodFacts v2 product read API. Only the fields we
// actually use are requested, which keeps the response small. It is a field on
// the service rather than a constant so the tests can point it at a stub and
// never touch the real service.
const defaultEndpoint = "https://world.openfoodfacts.org/api/v2/product/%s.json?fields=product_name,product_name_de,brands,quantity"

// OpenFoodFacts asks clients to identify themselves so they can contact the
// author about misbehaving traffic.
const userAgent = "Homebox-Pantry/1.0 (self-hosted; https://github.com/sysadminsmedia/homebox)"

// ErrLookupDisabled is returned when the feature is switched off by config.
var ErrLookupDisabled = errors.New("product lookup is disabled")

// Product is what a lookup yields. Found is false when the barcode is simply
// not in the database, which is a normal outcome and not an error.
type Product struct {
	Found  bool   `json:"found"`
	Name   string `json:"name"`
	Brand  string `json:"brand"`
	Amount string `json:"amount"`
	Source string `json:"source"`
}

type Service struct {
	enabled  bool
	endpoint string
	client   *http.Client
}

func New(enabled bool) *Service {
	return &Service{
		enabled:  enabled,
		endpoint: defaultEndpoint,
		// A scan is interactive: the user is standing there with a can in hand.
		// Better to give up quickly and let them type than to make them wait.
		client: &http.Client{Timeout: 4 * time.Second},
	}
}

func (s *Service) Enabled() bool {
	return s.enabled
}

// offResponse mirrors the parts of the OpenFoodFacts payload we read.
type offResponse struct {
	Status  int `json:"status"`
	Product struct {
		ProductName   string `json:"product_name"`
		ProductNameDE string `json:"product_name_de"`
		Brands        string `json:"brands"`
		Quantity      string `json:"quantity"`
	} `json:"product"`
}

// Lookup resolves a barcode. A barcode that is not in the database yields
// Product{Found: false} and no error - only a genuine failure to ask returns one.
func (s *Service) Lookup(ctx context.Context, barcode string) (Product, error) {
	if !s.enabled {
		return Product{}, ErrLookupDisabled
	}

	barcode = strings.TrimSpace(barcode)
	if !isPlausibleBarcode(barcode) {
		return Product{Found: false, Source: "openfoodfacts"}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf(s.endpoint, url.PathEscape(barcode)), nil)
	if err != nil {
		return Product{}, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return Product{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	// 404 is how OpenFoodFacts reports an unknown barcode.
	if resp.StatusCode == http.StatusNotFound {
		return Product{Found: false, Source: "openfoodfacts"}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return Product{}, fmt.Errorf("openfoodfacts returned %s", resp.Status)
	}

	var payload offResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Product{}, err
	}

	if payload.Status != 1 {
		return Product{Found: false, Source: "openfoodfacts"}, nil
	}

	// Prefer the German name when the entry has one; this is a German pantry.
	name := strings.TrimSpace(payload.Product.ProductNameDE)
	if name == "" {
		name = strings.TrimSpace(payload.Product.ProductName)
	}

	// An entry can exist with no usable name at all. Treat that as not found -
	// an empty suggestion is worse than none, it just looks broken.
	if name == "" {
		return Product{Found: false, Source: "openfoodfacts"}, nil
	}

	return Product{
		Found:  true,
		Name:   name,
		Brand:  firstBrand(payload.Product.Brands),
		Amount: strings.TrimSpace(payload.Product.Quantity),
		Source: "openfoodfacts",
	}, nil
}

// isPlausibleBarcode keeps obvious nonsense from ever leaving the server. Real
// EAN/UPC codes are 8 to 14 digits.
func isPlausibleBarcode(s string) bool {
	if len(s) < 8 || len(s) > 14 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// firstBrand takes the leading entry of the comma separated brand list.
func firstBrand(brands string) string {
	if brands == "" {
		return ""
	}
	return strings.TrimSpace(strings.Split(brands, ",")[0])
}
