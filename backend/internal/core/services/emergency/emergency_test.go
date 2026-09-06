package emergency

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func lineFor(t *testing.T, b Balance, c Category) Line {
	t.Helper()

	for _, l := range b.Lines {
		if l.Category == c {
			return l
		}
	}

	t.Fatalf("category %s missing from the balance", c)
	return Line{}
}

func TestCalculate_TargetsMatchThePublishedTable(t *testing.T) {
	// One person, ten days: exactly the figures the federal table prints.
	b := Calculate(nil, 1, 10)

	expected := map[Category]int{
		Drinks:     20_000,
		Vegetables: 4_000,
		Grains:     3_300,
		Fruit:      2_500,
		Dairy:      2_500,
		Protein:    1_200,
		Fats:       330,
	}

	for category, grams := range expected {
		assert.Equal(t, grams, lineFor(t, b, category).TargetG, "target for %s", category)
	}
}

func TestCalculate_ScalesWithPeopleAndDays(t *testing.T) {
	b := Calculate(nil, 3, 5)

	assert.Equal(t, 3, b.People)
	assert.Equal(t, 5, b.Days)
	assert.Equal(t, 2000*3*5, lineFor(t, b, Drinks).TargetG)
	assert.Equal(t, 330*3*5, lineFor(t, b, Grains).TargetG)
}

func TestCalculate_FallsBackToOnePersonAndTheRecommendedTenDays(t *testing.T) {
	b := Calculate(nil, 0, 0)

	assert.Equal(t, 1, b.People)
	assert.Equal(t, DefaultDays, b.Days)
	assert.Equal(t, 20_000, lineFor(t, b, Drinks).TargetG)
}

func TestCalculate_MultipliesPackagesByTheirContents(t *testing.T) {
	b := Calculate([]Holding{
		{Category: Vegetables, Quantity: 6, NetWeightG: 400},
		{Category: Vegetables, Quantity: 2, NetWeightG: 800},
	}, 1, 10)

	line := lineFor(t, b, Vegetables)
	assert.Equal(t, 6*400+2*800, line.StockG)
	assert.Equal(t, 4_000-4_000, line.MissingG, "6x400 plus 2x800 is exactly the 4 kg target")
	assert.Equal(t, 100, line.Percent)
}

func TestCalculate_UnweighedItemsCountForNothing(t *testing.T) {
	// The whole point of the page is to report how much is really there.
	// Treating an unweighed tin as anything at all would inflate that.
	b := Calculate([]Holding{
		{Category: Grains, Quantity: 20, NetWeightG: 0},
		{Category: Grains, Quantity: 0, NetWeightG: 500},
	}, 1, 10)

	assert.Equal(t, 0, lineFor(t, b, Grains).StockG)
	assert.Equal(t, 3_300, lineFor(t, b, Grains).MissingG)
}

func TestCalculate_ItemsWithoutACategoryAreIgnored(t *testing.T) {
	b := Calculate([]Holding{
		{Category: "", Quantity: 10, NetWeightG: 1000},
		{Category: "screws", Quantity: 10, NetWeightG: 1000},
	}, 1, 10)

	assert.Equal(t, 0, b.Percent)
}

func TestCalculate_SurplusDoesNotSpillIntoOtherCategories(t *testing.T) {
	// Ten times the water needed does not make up for having no food.
	b := Calculate([]Holding{
		{Category: Drinks, Quantity: 200, NetWeightG: 1000},
	}, 1, 10)

	drinks := lineFor(t, b, Drinks)
	assert.Equal(t, 200_000, drinks.StockG, "the real figure is still reported")
	assert.Equal(t, 0, drinks.MissingG)
	assert.Equal(t, 100, drinks.Percent, "capped rather than 1000%")

	assert.Less(t, b.Percent, 100, "the household is not stocked just because of the water")
	assert.Equal(t, 3_300, lineFor(t, b, Grains).MissingG)
}

func TestCalculate_OverallShareIsWeightedByTarget(t *testing.T) {
	// Filling the oil - the smallest target by far - must barely move the
	// overall figure, or the number would flatter a nearly empty cupboard.
	b := Calculate([]Holding{{Category: Fats, Quantity: 1, NetWeightG: 330}}, 1, 10)

	assert.Equal(t, 100, lineFor(t, b, Fats).Percent)
	assert.LessOrEqual(t, b.Percent, 2)
}

func TestCalculate_EveryCategoryIsAlwaysReported(t *testing.T) {
	b := Calculate(nil, 1, 10)

	require.Len(t, b.Lines, len(Order))
	for i, category := range Order {
		assert.Equal(t, category, b.Lines[i].Category, "reported in a fixed order")
	}
}

func TestParseCategory(t *testing.T) {
	for _, good := range []string{"drinks", "  Grains ", "FATS"} {
		_, ok := ParseCategory(good)
		assert.True(t, ok, "%q should be accepted", good)
	}

	for _, bad := range []string{"", "tools", "getränke"} {
		_, ok := ParseCategory(bad)
		assert.False(t, ok, "%q should be rejected", bad)
	}
}
