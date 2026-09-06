package productlookup

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/sysadminsmedia/homebox/backend/internal/core/services/emergency"
)

// Working out what a package holds, and which of the federal stockpiling
// groups it belongs to, from what OpenFoodFacts says about it.
//
// Both are suggestions. They are pre-filled into the form and can be corrected
// there, and neither is ever applied to an item without somebody seeing it: a
// wrong weight or a wrong group would quietly distort the one figure the
// stockpile page exists to report.

// amountPattern matches the shapes OpenFoodFacts quantity strings actually
// come in: "400 g", "1,5 L", "330ml", "2 x 250 g", "6x33cl".
var amountPattern = regexp.MustCompile(`(?i)^\s*(?:(\d+)\s*[x×]\s*)?(\d+(?:[.,]\d+)?)\s*(kg|g|l|ml|cl|dl)\b`)

// parseAmountGrams reads a package size in grams, counting a millilitre as a
// gram. Water is exactly that, and for a stockpile target everything else is
// close enough that a conversion table would be false precision.
//
// Returns 0 when the string says nothing usable, which means "unweighed" and
// keeps the item out of the totals rather than guessing at it.
func parseAmountGrams(quantity string) int {
	match := amountPattern.FindStringSubmatch(quantity)
	if match == nil {
		return 0
	}

	count := 1
	if match[1] != "" {
		parsed, err := strconv.Atoi(match[1])
		if err != nil || parsed < 1 {
			return 0
		}
		count = parsed
	}

	// German labels write halves with a comma.
	value, err := strconv.ParseFloat(strings.Replace(match[2], ",", ".", 1), 64)
	if err != nil || value <= 0 {
		return 0
	}

	perUnit := 0.0
	switch strings.ToLower(match[3]) {
	case "kg", "l":
		perUnit = value * 1000
	case "g", "ml":
		perUnit = value
	case "cl":
		perUnit = value * 10
	case "dl":
		perUnit = value * 100
	}

	grams := int(perUnit*float64(count) + 0.5)
	if grams <= 0 {
		return 0
	}

	return grams
}

// categoryHints maps a word appearing in an OpenFoodFacts category tag to a
// stockpiling group. Ordered most specific first, because the tags are nested
// and a tin of beans carries both "canned-vegetables" and "vegetables".
//
// Deliberately incomplete. A guess that is only sometimes right is worse than
// no guess when the alternative is a form field the user glances at anyway, so
// anything ambiguous is left blank for them to pick.
var categoryHints = []struct {
	needle   string
	category emergency.Category
}{
	{"water", emergency.Drinks},
	{"beverage", emergency.Drinks},
	{"drink", emergency.Drinks},
	{"juice", emergency.Drinks},

	{"milk", emergency.Dairy},
	{"dairy", emergency.Dairy},
	{"cheese", emergency.Dairy},
	{"yogurt", emergency.Dairy},
	{"yoghurt", emergency.Dairy},

	{"oil", emergency.Fats},
	{"butter", emergency.Fats},
	{"margarine", emergency.Fats},
	{"fats", emergency.Fats},

	{"fish", emergency.Protein},
	{"meat", emergency.Protein},
	{"sausage", emergency.Protein},
	{"egg", emergency.Protein},
	{"poultry", emergency.Protein},

	{"legume", emergency.Vegetables},
	{"bean", emergency.Vegetables},
	{"lentil", emergency.Vegetables},
	{"vegetable", emergency.Vegetables},
	{"mushroom", emergency.Vegetables},
	{"tomato", emergency.Vegetables},

	{"fruit", emergency.Fruit},
	{"nut", emergency.Fruit},
	{"berries", emergency.Fruit},

	{"pasta", emergency.Grains},
	{"rice", emergency.Grains},
	{"bread", emergency.Grains},
	{"cereal", emergency.Grains},
	{"flour", emergency.Grains},
	{"potato", emergency.Grains},
	{"noodle", emergency.Grains},
}

// guessCategory picks a stockpiling group from OpenFoodFacts category tags, or
// returns an empty string when nothing matches clearly.
func guessCategory(tags []string) string {
	joined := strings.ToLower(strings.Join(tags, " "))
	if joined == "" {
		return ""
	}

	for _, hint := range categoryHints {
		if strings.Contains(joined, hint.needle) {
			return string(hint.category)
		}
	}

	return ""
}
