package v1

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hay-kot/httpkit/errchain"
	"github.com/rs/zerolog/log"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/mealideas"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/productlookup"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/validate"
	"github.com/sysadminsmedia/homebox/backend/internal/web/adapters"
)

type (
	// ExpiringQuery selects how far ahead the expiry warning looks.
	ExpiringQuery struct {
		Within int `schema:"within"`
	}

	// BarcodeQuery carries the scanned barcode to look up.
	BarcodeQuery struct {
		Barcode string `schema:"barcode" validate:"required"`
	}

	// ConsumptionStatsQuery selects the period the statistics cover.
	ConsumptionStatsQuery struct {
		Days int `schema:"days"`
	}

	// ScanResult is everything the scanner needs after reading a code: the
	// items that already carry it, and - only when there are none - what a
	// product database knows about it.
	ScanResult struct {
		Barcode string             `json:"barcode"`
		Items   []repo.ItemSummary `json:"items"`
		// Suggestion is nil unless the barcode is unknown locally and the
		// lookup is enabled and produced something.
		Suggestion *productlookup.Product `json:"suggestion,omitempty" extensions:"x-nullable,x-omitempty"`
	}
)

// defaultExpiryWindow is used when the caller does not ask for a specific
// window. Two weeks is far enough ahead to act on, close enough to stay short.
const defaultExpiryWindow = 14

// MealIdeasResult is a handful of things to cook from what is in the cupboard.
// Empty rather than an error when the pantry is empty: nothing to suggest is a
// normal answer, not a failure.
type MealIdeasResult struct {
	Ideas []mealideas.Idea `json:"ideas"`
}

// HandleMealIdeas godoc
//
//	@Summary	Suggest meals from what the pantry holds
//	@Tags		Pantry
//	@Produce	json
//	@Success	200	{object}	MealIdeasResult
//	@Router		/v1/pantry/meal-ideas [GET]
//	@Security	Bearer
//
// The days left on each item are worked out here rather than by the model, so a
// suggestion can never rest on it having got the arithmetic of a date wrong.
func (ctrl *V1Controller) HandleMealIdeas() errchain.HandlerFunc {
	fn := func(r *http.Request) (MealIdeasResult, error) {
		auth := services.NewContext(r.Context())

		items, err := ctrl.repo.Items.QueryPantry(auth, auth.GID)
		if err != nil {
			return MealIdeasResult{}, err
		}

		if len(items) == 0 {
			return MealIdeasResult{Ideas: []mealideas.Idea{}}, nil
		}

		stock := make([]mealideas.Stock, 0, len(items))
		for _, itm := range items {
			line := mealideas.Stock{Name: itm.Name, Quantity: itm.Quantity}

			if expiry := itm.ExpiryDate.Time(); !expiry.IsZero() {
				left := daysUntil(expiry)
				line.DaysLeft = &left
			}

			stock = append(stock, line)
		}

		// Read the key only once there is actually something to ask about, so a
		// pantry with nothing in it never even touches the stored secret.
		key, err := ctrl.repo.Groups.RecipesAPIKey(auth, auth.GID)
		if err != nil {
			return MealIdeasResult{}, err
		}

		ideas, err := ctrl.svc.MealIdeas.Suggest(auth, key, stock)
		switch {
		case errors.Is(err, mealideas.ErrEmptyPantry):
			return MealIdeasResult{Ideas: []mealideas.Idea{}}, nil
		case errors.Is(err, mealideas.ErrDisabled), errors.Is(err, mealideas.ErrNoKey):
			// The UI hides the button when the status endpoint says the feature
			// is off; this is the guard for anything calling the route anyway.
			return MealIdeasResult{}, validate.NewRequestError(err, http.StatusNotFound)
		case err != nil:
			log.Warn().Err(err).Msg("meal ideas failed")
			return MealIdeasResult{}, validate.NewRequestError(err, http.StatusBadGateway)
		}

		return MealIdeasResult{Ideas: ideas}, nil
	}

	return adapters.Command(fn, http.StatusOK)
}

// daysUntil counts whole days from today to a date, so that "today" is 0 and
// anything already past is negative.
func daysUntil(at time.Time) int {
	startOfDay := func(t time.Time) time.Time {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	}

	return int(startOfDay(at).Sub(startOfDay(time.Now())).Hours() / 24)
}

// HandleItemsExpiring godoc
//
//	@Summary	Get items that expire soon
//	@Tags		Pantry
//	@Produce	json
//	@Param		within	query	int	false	"days to look ahead (default 14)"
//	@Success	200		{array}	repo.ItemSummary
//	@Router		/v1/pantry/expiring [GET]
//	@Security	Bearer
func (ctrl *V1Controller) HandleItemsExpiring() errchain.HandlerFunc {
	fn := func(r *http.Request, q ExpiringQuery) ([]repo.ItemSummary, error) {
		auth := services.NewContext(r.Context())

		within := q.Within
		if within <= 0 {
			within = defaultExpiryWindow
		}

		return ctrl.repo.Items.QueryExpiring(auth, auth.GID, within)
	}

	return adapters.Query(fn, http.StatusOK)
}

// HandleItemsLowStock godoc
//
//	@Summary	Get items at or below their minimum stock
//	@Tags		Pantry
//	@Produce	json
//	@Success	200	{array}	repo.ItemSummary
//	@Router		/v1/pantry/low-stock [GET]
//	@Security	Bearer
func (ctrl *V1Controller) HandleItemsLowStock() errchain.HandlerFunc {
	fn := func(r *http.Request) ([]repo.ItemSummary, error) {
		auth := services.NewContext(r.Context())
		return ctrl.repo.Items.QueryLowStock(auth, auth.GID)
	}

	return adapters.Command(fn, http.StatusOK)
}

// HandleItemsByBarcode godoc
//
//	@Summary	Look up items by barcode
//	@Tags		Pantry
//	@Produce	json
//	@Param		barcode	query	string	true	"barcode to look up"
//	@Success	200		{array}	repo.ItemSummary
//	@Router		/v1/pantry/barcode [GET]
//	@Security	Bearer
func (ctrl *V1Controller) HandleItemsByBarcode() errchain.HandlerFunc {
	fn := func(r *http.Request, q BarcodeQuery) ([]repo.ItemSummary, error) {
		auth := services.NewContext(r.Context())
		return ctrl.repo.Items.QueryByBarcode(auth, auth.GID, q.Barcode)
	}

	return adapters.Query(fn, http.StatusOK)
}

// HandleBarcodeScan godoc
//
//	@Summary	Resolve a scanned barcode
//	@Tags		Pantry
//	@Produce	json
//	@Param		barcode	query		string	true	"scanned barcode"
//	@Success	200		{object}	ScanResult
//	@Router		/v1/pantry/scan [GET]
//	@Security	Bearer
//
// One round trip for the whole scan: the local items first, and a product
// database suggestion only when nothing local matches. Keeping it in one call
// means the scanner does not have to decide when to ask.
func (ctrl *V1Controller) HandleBarcodeScan() errchain.HandlerFunc {
	fn := func(r *http.Request, q BarcodeQuery) (ScanResult, error) {
		auth := services.NewContext(r.Context())

		items, err := ctrl.repo.Items.QueryByBarcode(auth, auth.GID, q.Barcode)
		if err != nil {
			return ScanResult{}, err
		}

		result := ScanResult{Barcode: q.Barcode, Items: items}

		// Nothing goes out while a local item already answers the question.
		if len(items) > 0 {
			return result, nil
		}

		product, err := ctrl.svc.ProductLookup.Lookup(auth, q.Barcode)
		switch {
		case errors.Is(err, productlookup.ErrLookupDisabled):
			// Switched off; the user types the name. Not an error.
		case err != nil:
			// The lookup is a convenience. If OpenFoodFacts is slow or down the
			// scan must still work, so log it and carry on without a suggestion.
			log.Warn().Err(err).Msg("product lookup failed")
		case product.Found:
			result.Suggestion = &product
		}

		return result, nil
	}

	return adapters.Query(fn, http.StatusOK)
}

// HandleConsumptionStatistics godoc
//
//	@Summary	Consumption statistics per item
//	@Tags		Pantry
//	@Produce	json
//	@Param		days	query	int	false	"period in days (default 30)"
//	@Success	200		{array}	repo.ConsumptionSummary
//	@Router		/v1/pantry/consumption/statistics [GET]
//	@Security	Bearer
func (ctrl *V1Controller) HandleConsumptionStatistics() errchain.HandlerFunc {
	fn := func(r *http.Request, q ConsumptionStatsQuery) ([]repo.ConsumptionSummary, error) {
		auth := services.NewContext(r.Context())
		return ctrl.repo.Consumption.GetStatistics(auth, auth.GID, q.Days)
	}

	return adapters.Query(fn, http.StatusOK)
}

// HandleConsumptionLogGet godoc
//
//	@Summary	Get the consumption log of an item
//	@Tags		Item Consumption
//	@Produce	json
//	@Param		id	path	string	true	"Item ID"
//	@Success	200	{array}	repo.ConsumptionEntry
//	@Router		/v1/items/{id}/consumption [GET]
//	@Security	Bearer
func (ctrl *V1Controller) HandleConsumptionLogGet() errchain.HandlerFunc {
	fn := func(r *http.Request, itemID uuid.UUID) ([]repo.ConsumptionEntry, error) {
		auth := services.NewContext(r.Context())
		return ctrl.repo.Consumption.GetByItem(auth, auth.GID, itemID)
	}

	return adapters.CommandID("id", fn, http.StatusOK)
}

// HandleConsumptionCreate godoc
//
//	@Summary	Record a stock movement for an item
//	@Tags		Item Consumption
//	@Produce	json
//	@Param		id		path		string					true	"Item ID"
//	@Param		payload	body		repo.ConsumptionCreate	true	"Entry Data"
//	@Success	201		{object}	repo.ConsumptionEntry
//	@Router		/v1/items/{id}/consumption [POST]
//	@Security	Bearer
func (ctrl *V1Controller) HandleConsumptionCreate() errchain.HandlerFunc {
	fn := func(r *http.Request, itemID uuid.UUID, body repo.ConsumptionCreate) (repo.ConsumptionEntry, error) {
		auth := services.NewContext(r.Context())

		entry, err := ctrl.repo.Consumption.Create(auth, auth.GID, itemID, body)
		if err != nil {
			// Taking out more than is in stock is the user getting ahead of
			// themselves, not the server falling over - say so with a 4xx.
			switch {
			case errors.Is(err, repo.ErrInsufficientStock):
				return entry, validate.NewRequestError(err, http.StatusConflict)
			case errors.Is(err, repo.ErrItemNotFound):
				return entry, validate.NewRequestError(err, http.StatusNotFound)
			}
		}

		return entry, err
	}

	return adapters.ActionID("id", fn, http.StatusCreated)
}

// HandleConsumptionDelete godoc
//
//	@Summary	Delete a consumption log entry
//	@Tags		Item Consumption
//	@Produce	json
//	@Param		id	path	string	true	"Consumption Entry ID"
//	@Success	204
//	@Router		/v1/consumption/{id} [DELETE]
//	@Security	Bearer
func (ctrl *V1Controller) HandleConsumptionDelete() errchain.HandlerFunc {
	fn := func(r *http.Request, entryID uuid.UUID) (any, error) {
		auth := services.NewContext(r.Context())
		return nil, ctrl.repo.Consumption.Delete(auth, auth.GID, entryID)
	}

	return adapters.CommandID("id", fn, http.StatusNoContent)
}
