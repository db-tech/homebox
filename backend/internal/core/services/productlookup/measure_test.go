package productlookup

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/emergency"
)

func TestParseAmountGrams(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"400 g", 400},
		{"400g", 400},
		{"1 kg", 1000},
		{"0,5 kg", 500},
		{"1,5 L", 1500},
		{"1.5 l", 1500},
		{"330 ml", 330},
		{"330ml", 330},
		{"33 cl", 330},
		{"5 dl", 500},
		// Multipacks: what matters is what the package holds in total.
		{"2 x 250 g", 500},
		{"6x33cl", 1980},
		{"3 × 200 ml", 600},
		// Trailing detail is common and must not stop it reading the front.
		{"500 g (abgetropft 300 g)", 500},
		{"1 l Mehrweg", 1000},
	}

	for _, c := range cases {
		assert.Equal(t, c.want, parseAmountGrams(c.in), "%q", c.in)
	}
}

func TestParseAmountGrams_UnreadableMeansUnweighed(t *testing.T) {
	// Zero keeps the item out of the totals. Guessing would inflate the one
	// figure the stockpile page exists to report.
	for _, in := range []string{"", "1 Stück", "grosse Packung", "g 400", "0 g", "-2 kg", "abc"} {
		assert.Equal(t, 0, parseAmountGrams(in), "%q should not be read as an amount", in)
	}
}

func TestGuessCategory(t *testing.T) {
	cases := []struct {
		tags []string
		want emergency.Category
	}{
		{[]string{"en:beverages", "en:waters"}, emergency.Drinks},
		{[]string{"en:dairies", "en:milks"}, emergency.Dairy},
		{[]string{"en:fats", "en:vegetable-oils"}, emergency.Fats},
		{[]string{"en:meats", "en:canned-meats"}, emergency.Protein},
		{[]string{"en:canned-vegetables", "en:legumes"}, emergency.Vegetables},
		{[]string{"en:fruits", "en:canned-fruits"}, emergency.Fruit},
		{[]string{"en:pastas"}, emergency.Grains},
		{[]string{"en:rice"}, emergency.Grains},
	}

	for _, c := range cases {
		assert.Equal(t, string(c.want), guessCategory(c.tags), "%v", c.tags)
	}
}

func TestGuessCategory_MoreSpecificTagWins(t *testing.T) {
	// A tin of beans carries both tags. It belongs with the vegetables, and the
	// ordering of the hints is what decides that - so it is worth asserting.
	assert.Equal(t, string(emergency.Vegetables), guessCategory([]string{"en:plant-based-foods", "en:legumes"}))

	// Milk chocolate is not dairy stock, but it does say "milk". This is the
	// kind of case the guess gets wrong, which is why it is only ever a
	// suggestion shown in a form rather than something applied silently.
	assert.Equal(t, string(emergency.Dairy), guessCategory([]string{"en:chocolates", "en:milk-chocolates"}))
}

func TestGuessCategory_NothingClearMeansNoSuggestion(t *testing.T) {
	for _, tags := range [][]string{nil, {}, {"en:snacks"}, {"en:hardware"}} {
		assert.Empty(t, guessCategory(tags), "%v", tags)
	}
}
