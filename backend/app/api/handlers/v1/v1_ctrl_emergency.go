package v1

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/hay-kot/httpkit/errchain"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/emergency"
	"github.com/sysadminsmedia/homebox/backend/internal/web/adapters"
)

type (
	// EmergencyItem is an item that is in a stockpiling group but cannot be
	// counted, so the page can point at it instead of silently ignoring it.
	EmergencyItem struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	// EmergencyResult is the whole stockpile picture.
	EmergencyResult struct {
		Balance emergency.Balance `json:"balance"`
		// Unweighed are items in a group that nobody has said the size of.
		// They contribute nothing, and saying so is the point of listing them.
		Unweighed []EmergencyItem `json:"unweighed"`
		// Uncategorised are pantry items not assigned to any group at all.
		Uncategorised []EmergencyItem `json:"uncategorised"`
		// Checklist holds the ids ticked off on the non-food list.
		Checklist []string `json:"checklist"`
	}

	// EmergencySettings is what the household can change.
	EmergencySettings struct {
		People    int      `json:"people"`
		Days      int      `json:"days"`
		Checklist []string `json:"checklist"`
	}
)

// maxListed caps the "these are not being counted" lists. The point is to show
// there is work outstanding and where to start, not to reproduce the inventory.
const maxListed = 50

// HandleEmergencyStock godoc
//
//	@Summary	Measure the pantry against the federal stockpiling recommendation
//	@Tags		Pantry
//	@Produce	json
//	@Success	200	{object}	EmergencyResult
//	@Router		/v1/pantry/emergency [GET]
//	@Security	Bearer
func (ctrl *V1Controller) HandleEmergencyStock() errchain.HandlerFunc {
	fn := func(r *http.Request) (EmergencyResult, error) {
		auth := services.NewContext(r.Context())

		people, days, checklist, err := ctrl.repo.Groups.EmergencySettings(auth, auth.GID)
		if err != nil {
			return EmergencyResult{}, err
		}

		assigned, err := ctrl.repo.Items.QueryEmergencyStock(auth, auth.GID)
		if err != nil {
			return EmergencyResult{}, err
		}

		holdings := make([]emergency.Holding, 0, len(assigned))
		unweighed := make([]EmergencyItem, 0)

		for _, itm := range assigned {
			category, ok := emergency.ParseCategory(itm.EmergencyCategory)
			if !ok {
				continue
			}

			if itm.NetWeight <= 0 {
				if len(unweighed) < maxListed {
					unweighed = append(unweighed, EmergencyItem{ID: itm.ID.String(), Name: itm.Name})
				}
				continue
			}

			holdings = append(holdings, emergency.Holding{
				Category:   category,
				Quantity:   itm.Quantity,
				NetWeightG: itm.NetWeight,
			})
		}

		// Food already in the pantry but not assigned to a group is the other
		// half of the work outstanding, and the more common half at first.
		pantry, err := ctrl.repo.Items.QueryPantry(auth, auth.GID)
		if err != nil {
			return EmergencyResult{}, err
		}

		uncategorised := make([]EmergencyItem, 0)
		for _, itm := range pantry {
			if itm.EmergencyCategory != "" || len(uncategorised) >= maxListed {
				continue
			}
			uncategorised = append(uncategorised, EmergencyItem{ID: itm.ID.String(), Name: itm.Name})
		}

		return EmergencyResult{
			Balance:       emergency.Calculate(holdings, people, days),
			Unweighed:     unweighed,
			Uncategorised: uncategorised,
			Checklist:     decodeChecklist(checklist),
		}, nil
	}

	return adapters.Command(fn, http.StatusOK)
}

// HandleEmergencySettings godoc
//
//	@Summary	Set the household size, the number of days and the ticked checklist
//	@Tags		Pantry
//	@Produce	json
//	@Param		payload	body		EmergencySettings	true	"Settings"
//	@Success	200		{object}	EmergencySettings
//	@Router		/v1/pantry/emergency [PUT]
//	@Security	Bearer
//
// Echoes the settings back after clamping, so the form shows what was actually
// stored rather than what was typed.
func (ctrl *V1Controller) HandleEmergencySettings() errchain.HandlerFunc {
	fn := func(r *http.Request, body EmergencySettings) (EmergencySettings, error) {
		auth := services.NewContext(r.Context())

		// Clamped rather than rejected: these come from a stepper, and a
		// household of minus one is a slip, not something worth an error page.
		people := clamp(body.People, 1, 50)
		days := clamp(body.Days, 1, 90)

		ticked := body.Checklist
		if ticked == nil {
			ticked = []string{}
		}

		encoded, err := encodeChecklist(ticked)
		if err != nil {
			return EmergencySettings{}, err
		}

		if err := ctrl.repo.Groups.SetEmergencySettings(auth, auth.GID, people, days, encoded); err != nil {
			return EmergencySettings{}, err
		}

		return EmergencySettings{People: people, Days: days, Checklist: ticked}, nil
	}

	return adapters.Action(fn, http.StatusOK)
}

func clamp(value, low, high int) int {
	switch {
	case value < low:
		return low
	case value > high:
		return high
	default:
		return value
	}
}

// decodeChecklist reads the stored ticks. Anything unreadable is treated as
// nothing ticked rather than as an error: a corrupt preference must not make
// the whole stockpile page fail to load.
func decodeChecklist(stored string) []string {
	ticked := []string{}
	if strings.TrimSpace(stored) == "" {
		return ticked
	}

	if err := json.Unmarshal([]byte(stored), &ticked); err != nil {
		return []string{}
	}

	return ticked
}

func encodeChecklist(ticked []string) (string, error) {
	if len(ticked) == 0 {
		return "", nil
	}

	encoded, err := json.Marshal(ticked)
	if err != nil {
		return "", err
	}

	return string(encoded), nil
}
