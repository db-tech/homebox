// Package services provides the core business logic for the application.
package services

import (
	"github.com/sysadminsmedia/homebox/backend/internal/core/currencies"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/productlookup"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
)

type AllServices struct {
	User              *UserService
	Group             *GroupService
	Items             *ItemService
	BackgroundService *BackgroundService
	Currencies        *currencies.CurrencyRegistry
	ProductLookup     *productlookup.Service
}

type OptionsFunc func(*options)

type options struct {
	autoIncrementAssetID bool
	currencies           []currencies.Currency
	productLookup        bool
}

func WithAutoIncrementAssetID(v bool) func(*options) {
	return func(o *options) {
		o.autoIncrementAssetID = v
	}
}

// WithProductLookup enables resolving unknown barcodes at OpenFoodFacts. Off by
// default here so that anything constructing the services without saying so -
// tests, tooling - never reaches out to a third party.
func WithProductLookup(v bool) func(*options) {
	return func(o *options) {
		o.productLookup = v
	}
}

func WithCurrencies(v []currencies.Currency) func(*options) {
	return func(o *options) {
		o.currencies = v
	}
}

func New(repos *repo.AllRepos, opts ...OptionsFunc) *AllServices {
	if repos == nil {
		panic("repos cannot be nil")
	}

	defaultCurrencies, err := currencies.CollectionCurrencies(
		currencies.CollectDefaults(),
	)
	if err != nil {
		panic("failed to collect default currencies")
	}

	options := &options{
		autoIncrementAssetID: true,
		currencies:           defaultCurrencies,
		productLookup:        false,
	}

	for _, opt := range opts {
		opt(options)
	}

	return &AllServices{
		User:  &UserService{repos},
		Group: &GroupService{repos},
		Items: &ItemService{
			repo:                 repos,
			autoIncrementAssetID: options.autoIncrementAssetID,
		},
		BackgroundService: &BackgroundService{repos, Latest{}},
		Currencies:        currencies.NewCurrencyService(options.currencies),
		ProductLookup:     productlookup.New(options.productLookup),
	}
}
