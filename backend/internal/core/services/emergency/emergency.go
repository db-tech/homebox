// Package emergency measures a pantry against the German federal
// recommendation for how much food a household should keep at home.
//
// The figures come from the Ernährungsvorsorge portal run by the Federal
// Ministry of Food and Agriculture, which states them per person for ten days
// at 2,200 kcal a day:
//
//	Getränke                            20 l
//	Gemüse, Pilze                       4.0 kg
//	Getreideprodukte, Brot, Kartoffeln  3.3 kg
//	Obst                                2.5 kg
//	Milch, Milcherzeugnisse             2.5 kg
//	Eier, Fleisch, Wurst und Fisch      1.2 kg
//	Fette, Öl                           330 g
//
// https://www.ernaehrungsvorsorge.de/private-vorsorge/notvorrat/vorratstabelle
//
// Everything here counts in grams, drinks included: water is a gram per
// millilitre, and for a stockpile target the rest is close enough. One unit
// means a tin and a bottle can be added together without a conversion table.
package emergency

import "strings"

// Category is one of the groups the federal table is broken into.
type Category string

const (
	Drinks     Category = "drinks"
	Grains     Category = "grains"
	Vegetables Category = "vegetables"
	Fruit      Category = "fruit"
	Dairy      Category = "dairy"
	Protein    Category = "protein"
	Fats       Category = "fats"
)

// DefaultDays is what the federal advice builds to. Three days is described as
// already worth having, which is why the figure is adjustable rather than baked
// into the targets.
const DefaultDays = 10

// perPersonPerDay is the table above divided by its ten days, in grams. Kept as
// whole grams so that no target is ever a rounding artefact.
var perPersonPerDay = map[Category]int{
	Drinks:     2000, // 20 l / 10 d - 1.5 l to drink plus 0.5 l to cook with
	Vegetables: 400,  // 4.0 kg / 10 d
	Grains:     330,  // 3.3 kg / 10 d
	Fruit:      250,  // 2.5 kg / 10 d
	Dairy:      250,  // 2.5 kg / 10 d
	Protein:    120,  // 1.2 kg / 10 d
	Fats:       33,   // 330 g / 10 d
}

// Order is the order the categories are reported in: the biggest target first,
// which is also roughly the order they matter in.
var Order = []Category{Drinks, Vegetables, Grains, Fruit, Dairy, Protein, Fats}

// Holding is one item's contribution: how many packages, and how much one
// package holds.
type Holding struct {
	Category   Category
	Quantity   int
	NetWeightG int
}

// Line is how one category stands.
type Line struct {
	Category Category `json:"category"`
	TargetG  int      `json:"targetGrams"`
	StockG   int      `json:"stockGrams"`
	// MissingG is what is still to buy, never negative: being over the target
	// is not a surplus to be spent somewhere else.
	MissingG int `json:"missingGrams"`
	// Percent is capped at 100 so a full category reads as done rather than as
	// a score to beat.
	Percent int `json:"percent"`
}

// Balance is the whole picture.
type Balance struct {
	People int    `json:"people"`
	Days   int    `json:"days"`
	Lines  []Line `json:"lines"`
	// Percent over all categories, weighted by target rather than by category,
	// so that being short of 20 litres of water counts for more than being
	// short of 300 grams of oil.
	Percent int `json:"percent"`
}

// Calculate measures holdings against the recommendation for the given
// household. People and days below one fall back to one person and the
// recommended ten days, so a household that has not set them still sees
// something meaningful.
func Calculate(holdings []Holding, people, days int) Balance {
	if people < 1 {
		people = 1
	}
	if days < 1 {
		days = DefaultDays
	}

	stock := make(map[Category]int, len(Order))
	for _, h := range holdings {
		// An item nobody has weighed contributes nothing. Guessing here would
		// quietly inflate the figure that the whole page exists to report.
		if h.NetWeightG <= 0 || h.Quantity <= 0 {
			continue
		}
		if _, known := perPersonPerDay[h.Category]; !known {
			continue
		}

		stock[h.Category] += h.Quantity * h.NetWeightG
	}

	balance := Balance{People: people, Days: days, Lines: make([]Line, 0, len(Order))}

	var totalTarget, totalCovered int

	for _, category := range Order {
		target := perPersonPerDay[category] * people * days
		have := stock[category]

		missing := target - have
		if missing < 0 {
			missing = 0
		}

		covered := have
		if covered > target {
			covered = target
		}

		percent := 100
		if target > 0 {
			percent = covered * 100 / target
		}

		balance.Lines = append(balance.Lines, Line{
			Category: category,
			TargetG:  target,
			StockG:   have,
			MissingG: missing,
			Percent:  percent,
		})

		totalTarget += target
		totalCovered += covered
	}

	if totalTarget > 0 {
		balance.Percent = totalCovered * 100 / totalTarget
	}

	return balance
}

// ParseCategory turns stored text into a Category, rejecting anything that is
// not one of the seven.
func ParseCategory(value string) (Category, bool) {
	candidate := Category(strings.TrimSpace(strings.ToLower(value)))
	if _, ok := perPersonPerDay[candidate]; ok {
		return candidate, true
	}

	return "", false
}
