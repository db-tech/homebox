package v1

import (
	"errors"
	"net/http"
	"sync"

	"github.com/hay-kot/httpkit/errchain"
	"github.com/rs/zerolog/log"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/emergency"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/validate"
	"github.com/sysadminsmedia/homebox/backend/internal/web/adapters"
)

// Working out what is missing from the items already in the pantry.
//
// Two sources, deliberately in this order. The product database knows what a
// package actually holds, which is a fact; the model can only say what a name
// probably is, which is a judgement. So every barcode is asked first, and the
// model is left with what is genuinely unknown.
//
// Nothing here writes anything. The suggestions go back to be looked at, and a
// separate call applies the ones that survive that.

type (
	// StockSuggestion is one proposed change, with where it came from.
	StockSuggestion struct {
		ID   string `json:"id"`
		Name string `json:"name"`

		// Current state, so the screen can show what is being replaced.
		CurrentCategory string `json:"currentCategory"`
		CurrentWeight   int    `json:"currentWeight"`

		Category string `json:"category"`
		Weight   int    `json:"weight"`

		// WeightSource is "openfoodfacts" for a manufacturer's figure and
		// "estimate" for the model's. The difference decides how much the
		// resulting percentage is worth, so it travels with the number.
		WeightSource   string `json:"weightSource,omitempty"`
		CategorySource string `json:"categorySource,omitempty"`
	}

	StockSuggestionResult struct {
		Suggestions []StockSuggestion `json:"suggestions"`
		// Remaining is how many items still had gaps but were not looked at in
		// this round, so the screen can offer another go instead of pretending
		// the list was complete.
		Remaining int `json:"remaining"`
		// Classified is false when the model was not consulted - switched off,
		// or no key - so the screen can say why only some gaps were filled.
		Classified bool   `json:"classified"`
		Note       string `json:"note,omitempty"`
	}

	StockAssignRequest struct {
		Assignments []repo.StockpileAssignment `json:"assignments"`
	}

	StockAssignResult struct {
		Changed int `json:"changed"`
	}
)

const (
	// maxLookups caps the product-database calls per round. OpenFoodFacts is a
	// volunteer service and a hundred requests in a burst is not a polite way
	// to treat it; the screen offers another round instead.
	maxLookups = 60
	// lookupWorkers keeps that burst to a trickle.
	lookupWorkers = 4
	// maxClassify caps one model call. Far more than a household needs, and a
	// bound so a runaway inventory cannot produce a giant prompt.
	maxClassify = 200
)

// HandleStockSuggestions godoc
//
//	@Summary	Propose stockpile groups and package sizes for items that lack them
//	@Tags		Pantry
//	@Produce	json
//	@Success	200	{object}	StockSuggestionResult
//	@Router		/v1/pantry/emergency/suggestions [GET]
//	@Security	Bearer
func (ctrl *V1Controller) HandleStockSuggestions() errchain.HandlerFunc {
	fn := func(r *http.Request) (StockSuggestionResult, error) {
		auth := services.NewContext(r.Context())

		pantry, err := ctrl.repo.Items.QueryPantry(auth, auth.GID)
		if err != nil {
			return StockSuggestionResult{}, err
		}

		gaps := make([]repo.ItemSummary, 0, len(pantry))
		for _, itm := range pantry {
			if itm.EmergencyCategory == "" || itm.NetWeight <= 0 {
				gaps = append(gaps, itm)
			}
		}

		result := StockSuggestionResult{Suggestions: []StockSuggestion{}}

		if len(gaps) == 0 {
			return result, nil
		}

		// One round only. Whatever is left over is reported rather than
		// silently dropped.
		batch := gaps
		if len(batch) > maxClassify {
			batch = batch[:maxClassify]
			result.Remaining = len(gaps) - maxClassify
		}

		proposals := make([]StockSuggestion, len(batch))
		for i, itm := range batch {
			proposals[i] = StockSuggestion{
				ID:              itm.ID.String(),
				Name:            itm.Name,
				CurrentCategory: itm.EmergencyCategory,
				CurrentWeight:   itm.NetWeight,
				Category:        itm.EmergencyCategory,
				Weight:          itm.NetWeight,
			}
		}

		ctrl.fillFromProductDatabase(auth, batch, proposals)
		ctrl.fillFromModel(auth, batch, proposals, &result)

		// Anything that ended up proposing nothing new is noise on a review
		// screen, so it does not go back.
		for i := range proposals {
			p := proposals[i]
			if p.Category == p.CurrentCategory && p.Weight == p.CurrentWeight {
				continue
			}
			result.Suggestions = append(result.Suggestions, p)
		}

		return result, nil
	}

	return adapters.Command(fn, http.StatusOK)
}

// fillFromProductDatabase asks OpenFoodFacts about every barcode with a missing
// package size, a few at a time.
func (ctrl *V1Controller) fillFromProductDatabase(ctx services.Context, items []repo.ItemSummary, into []StockSuggestion) {
	if !ctrl.svc.ProductLookup.Enabled() {
		return
	}

	type job struct{ index int }

	queue := make(chan job)
	var wg sync.WaitGroup
	var mu sync.Mutex

	asked := 0

	for worker := 0; worker < lookupWorkers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := range queue {
				product, err := ctrl.svc.ProductLookup.Lookup(ctx, items[j.index].Barcode)
				if err != nil || !product.Found {
					continue
				}

				mu.Lock()
				if product.AmountGrams > 0 && into[j.index].Weight <= 0 {
					into[j.index].Weight = product.AmountGrams
					into[j.index].WeightSource = "openfoodfacts"
				}
				if product.Category != "" && into[j.index].Category == "" {
					into[j.index].Category = product.Category
					into[j.index].CategorySource = "openfoodfacts"
				}
				mu.Unlock()
			}
		}()
	}

	for i, itm := range items {
		if itm.Barcode == "" || itm.NetWeight > 0 || asked >= maxLookups {
			continue
		}
		asked++
		queue <- job{index: i}
	}

	close(queue)
	wg.Wait()
}

// fillFromModel asks the model about whatever the product database could not
// answer. Only the names go out - no barcodes, no quantities, no locations.
func (ctrl *V1Controller) fillFromModel(ctx services.Context, items []repo.ItemSummary, into []StockSuggestion, result *StockSuggestionResult) {
	names := make([]string, 0, len(items))
	for i := range into {
		if into[i].Category == "" || into[i].Weight <= 0 {
			names = append(names, items[i].Name)
		}
	}

	if len(names) == 0 {
		result.Classified = true
		return
	}

	key, err := ctrl.repo.Groups.RecipesAPIKey(ctx, ctx.GID)
	if err != nil {
		log.Warn().Err(err).Msg("stock suggestions: could not read the key")
		return
	}

	suggestions, err := ctrl.svc.StockClassifier.Classify(ctx, key, names)
	switch {
	case errors.Is(err, emergency.ErrClassifyDisabled), errors.Is(err, emergency.ErrClassifyNoKey):
		// Not a failure: the product database half still did its work, and the
		// screen says why the rest is unfilled.
		result.Note = err.Error()
		return
	case err != nil:
		log.Warn().Err(err).Msg("stock classification failed")
		result.Note = err.Error()
		return
	}

	result.Classified = true

	for i := range into {
		suggestion, ok := suggestions[items[i].Name]
		if !ok {
			continue
		}

		if into[i].Category == "" && suggestion.Category != "" {
			into[i].Category = string(suggestion.Category)
			into[i].CategorySource = "estimate"
		}
		if into[i].Weight <= 0 && suggestion.Grams > 0 {
			into[i].Weight = suggestion.Grams
			into[i].WeightSource = "estimate"
		}
	}
}

// HandleStockAssign godoc
//
//	@Summary	Set the stockpile group and package size on several items
//	@Tags		Pantry
//	@Produce	json
//	@Param		payload	body		StockAssignRequest	true	"Assignments"
//	@Success	200		{object}	StockAssignResult
//	@Router		/v1/pantry/emergency/assign [PUT]
//	@Security	Bearer
func (ctrl *V1Controller) HandleStockAssign() errchain.HandlerFunc {
	fn := func(r *http.Request, body StockAssignRequest) (StockAssignResult, error) {
		auth := services.NewContext(r.Context())

		if len(body.Assignments) == 0 {
			return StockAssignResult{}, nil
		}

		// A group that is not one of the seven would sit in the database
		// unreadable by the page that has to add it up.
		for _, assignment := range body.Assignments {
			if assignment.Category == "" {
				continue
			}
			if _, ok := emergency.ParseCategory(assignment.Category); !ok {
				return StockAssignResult{}, validate.NewRequestError(
					errors.New("unknown stockpile group: "+assignment.Category),
					http.StatusUnprocessableEntity,
				)
			}
		}

		changed, err := ctrl.repo.Items.AssignStockpileFields(auth, auth.GID, body.Assignments)
		if err != nil {
			return StockAssignResult{}, err
		}

		return StockAssignResult{Changed: changed}, nil
	}

	return adapters.Action(fn, http.StatusOK)
}
