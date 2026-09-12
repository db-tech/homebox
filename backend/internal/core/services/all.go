// Package services provides the core business logic for the application.
package services

import (
	"github.com/sysadminsmedia/homebox/backend/internal/core/currencies"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/mealideas"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/productlookup"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/voiceentry"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
)

type AllServices struct {
	User              *UserService
	Group             *GroupService
	Items             *ItemService
	BackgroundService *BackgroundService
	Currencies        *currencies.CurrencyRegistry
	ProductLookup     *productlookup.Service
	MealIdeas         *mealideas.Service
	VoiceEntry        *voiceentry.Service
}

type OptionsFunc func(*options)

type options struct {
	autoIncrementAssetID bool
	currencies           []currencies.Currency
	productLookup        bool
	mealIdeas            mealIdeasOptions
	voice                voiceOptions
}

type voiceOptions struct {
	enabled bool
	apiKey  string
}

type mealIdeasOptions struct {
	enabled bool
	apiKey  string
	model   string
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
		MealIdeas:         mealideas.New(options.mealIdeas.enabled, options.mealIdeas.apiKey, options.mealIdeas.model),
		VoiceEntry: voiceentry.New(
			options.voice.enabled,
			options.voice.apiKey,
			options.mealIdeas.apiKey,
			options.mealIdeas.model,
		),
	}
}

// WithMealIdeas enables meal suggestions built from the pantry. Off by default
// here for the same reason as the barcode lookup: anything constructing the
// services without saying so must never reach out to a third party, and this
// one would send item names rather than a barcode.
func WithMealIdeas(enabled bool, apiKey, model string) func(*options) {
	return func(o *options) {
		o.mealIdeas = mealIdeasOptions{enabled: enabled, apiKey: apiKey, model: model}
	}
}

// WithVoiceEntry enables adding an item by speaking it. Off by default here for
// the same reason as the other two: anything constructing the services without
// saying so must never reach out to a third party, and this one would send a
// recording of somebody's voice.
func WithVoiceEntry(enabled bool, apiKey string) func(*options) {
	return func(o *options) {
		o.voice = voiceOptions{enabled: enabled, apiKey: apiKey}
	}
}
