package v1

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/hay-kot/httpkit/errchain"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services"
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
)

// defaultExpiryWindow is used when the caller does not ask for a specific
// window. Two weeks is far enough ahead to act on, close enough to stay short.
const defaultExpiryWindow = 14

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
