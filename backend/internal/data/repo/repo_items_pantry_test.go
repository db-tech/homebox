package repo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sysadminsmedia/homebox/backend/internal/data/types"
)

// updatePantry sets the pantry-specific fields on an item, leaving the rest at
// the values the factory produced.
func updatePantry(t *testing.T, itm ItemOut, quantity, minStock int, barcode string, expiry time.Time) {
	t.Helper()

	_, err := tRepos.Items.UpdateByGroup(context.Background(), tGroup.ID, ItemUpdate{
		ID:         itm.ID,
		Name:       itm.Name,
		LocationID: itm.Location.ID,
		Quantity:   quantity,
		MinStock:   minStock,
		Barcode:    barcode,
		ExpiryDate: types.DateFromTime(expiry),
	})
	require.NoError(t, err)
}

func containsItem(items []ItemSummary, id interface{ String() string }) bool {
	for _, i := range items {
		if i.ID.String() == id.String() {
			return true
		}
	}
	return false
}

func TestItemsRepository_PantryFieldsRoundTrip(t *testing.T) {
	itm := useItems(t, 1)[0]
	expiry := time.Now().AddDate(0, 0, 5)

	updatePantry(t, itm, 4, 2, "4001234567890", expiry)

	got, err := tRepos.Items.GetOne(context.Background(), itm.ID)
	require.NoError(t, err)

	assert.Equal(t, 2, got.MinStock)
	assert.Equal(t, "4001234567890", got.Barcode)
	assert.Equal(t, expiry.Format("2006-01-02"), got.ExpiryDate.Time().Format("2006-01-02"))
}

func TestItemsRepository_QueryExpiring(t *testing.T) {
	items := useItems(t, 3)

	soon := items[0]
	later := items[1]
	never := items[2]

	updatePantry(t, soon, 1, 0, "", time.Now().AddDate(0, 0, 3))
	updatePantry(t, later, 1, 0, "", time.Now().AddDate(0, 0, 90))
	updatePantry(t, never, 1, 0, "", time.Time{})

	got, err := tRepos.Items.QueryExpiring(context.Background(), tGroup.ID, 14)
	require.NoError(t, err)

	assert.True(t, containsItem(got, soon.ID), "item expiring in 3 days should be listed")
	assert.False(t, containsItem(got, later.ID), "item expiring in 90 days should not be listed")
	assert.False(t, containsItem(got, never.ID), "item without expiry date should never be listed")
}

// Already-expired items must stay on the list rather than silently dropping off.
func TestItemsRepository_QueryExpiringIncludesAlreadyExpired(t *testing.T) {
	itm := useItems(t, 1)[0]
	updatePantry(t, itm, 1, 0, "", time.Now().AddDate(0, 0, -7))

	got, err := tRepos.Items.QueryExpiring(context.Background(), tGroup.ID, 14)
	require.NoError(t, err)

	assert.True(t, containsItem(got, itm.ID))
}

func TestItemsRepository_QueryExpiringIsSortedSoonestFirst(t *testing.T) {
	items := useItems(t, 3)
	updatePantry(t, items[0], 1, 0, "", time.Now().AddDate(0, 0, 9))
	updatePantry(t, items[1], 1, 0, "", time.Now().AddDate(0, 0, 2))
	updatePantry(t, items[2], 1, 0, "", time.Now().AddDate(0, 0, 5))

	got, err := tRepos.Items.QueryExpiring(context.Background(), tGroup.ID, 30)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(got), 3)

	for i := 1; i < len(got); i++ {
		assert.False(t, got[i].ExpiryDate.Time().Before(got[i-1].ExpiryDate.Time()),
			"expiring items must be sorted soonest first")
	}
}

func TestItemsRepository_QueryLowStock(t *testing.T) {
	items := useItems(t, 4)

	below := items[0]   // 1 of 3 -> low
	atLimit := items[1] // 3 of 3 -> low (at the limit counts)
	above := items[2]   // 9 of 3 -> fine
	untracked := items[3]

	updatePantry(t, below, 1, 3, "", time.Time{})
	updatePantry(t, atLimit, 3, 3, "", time.Time{})
	updatePantry(t, above, 9, 3, "", time.Time{})
	updatePantry(t, untracked, 0, 0, "", time.Time{})

	got, err := tRepos.Items.QueryLowStock(context.Background(), tGroup.ID)
	require.NoError(t, err)

	assert.True(t, containsItem(got, below.ID), "quantity below minimum should be listed")
	assert.True(t, containsItem(got, atLimit.ID), "quantity at the minimum should be listed")
	assert.False(t, containsItem(got, above.ID), "quantity above minimum should not be listed")
	assert.False(t, containsItem(got, untracked.ID), "minStock 0 means untracked and must be excluded")
}

func TestItemsRepository_QueryByBarcode(t *testing.T) {
	items := useItems(t, 2)
	updatePantry(t, items[0], 1, 0, "4001234567890", time.Time{})
	updatePantry(t, items[1], 1, 0, "9999999999999", time.Time{})

	got, err := tRepos.Items.QueryByBarcode(context.Background(), tGroup.ID, "4001234567890")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, items[0].ID, got[0].ID)

	none, err := tRepos.Items.QueryByBarcode(context.Background(), tGroup.ID, "0000000000000")
	require.NoError(t, err)
	assert.Empty(t, none)
}

// An empty barcode must not match every item that has no barcode set.
func TestItemsRepository_QueryByBarcodeEmptyReturnsNothing(t *testing.T) {
	useItems(t, 2)

	got, err := tRepos.Items.QueryByBarcode(context.Background(), tGroup.ID, "")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestItemsRepository_PantryQueriesAreGroupScoped(t *testing.T) {
	itm := useItems(t, 1)[0]
	updatePantry(t, itm, 1, 5, "4001234567890", time.Now().AddDate(0, 0, 2))

	other, err := tRepos.Groups.GroupCreate(context.Background(), "other-group-pantry")
	require.NoError(t, err)

	expiring, err := tRepos.Items.QueryExpiring(context.Background(), other.ID, 30)
	require.NoError(t, err)
	assert.Empty(t, expiring)

	low, err := tRepos.Items.QueryLowStock(context.Background(), other.ID)
	require.NoError(t, err)
	assert.Empty(t, low)

	byBarcode, err := tRepos.Items.QueryByBarcode(context.Background(), other.ID, "4001234567890")
	require.NoError(t, err)
	assert.Empty(t, byBarcode)
}

// Creating with the pantry fields set is what the scanner does when a tin is
// unpacked, so the values must survive the create without an extra update.
func TestItemsRepository_CreateWithPantryFields(t *testing.T) {
	location, err := tRepos.Locations.Create(context.Background(), tGroup.ID, locationFactory())
	require.NoError(t, err)
	t.Cleanup(func() { _ = tRepos.Locations.delete(context.Background(), location.ID) })

	expiry := time.Now().AddDate(0, 0, 20)

	created, err := tRepos.Items.Create(context.Background(), tGroup.ID, ItemCreate{
		Name:       "Dosentomaten",
		LocationID: location.ID,
		Barcode:    "4001234567890",
		MinStock:   4,
		ExpiryDate: types.DateFromTime(expiry),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tRepos.Items.Delete(context.Background(), created.ID) })

	assert.Equal(t, "4001234567890", created.Barcode)
	assert.Equal(t, 4, created.MinStock)
	assert.Equal(t, expiry.Format("2006-01-02"), created.ExpiryDate.Time().Format("2006-01-02"))

	// And it must be picked up by the views straight away.
	expiring, err := tRepos.Items.QueryExpiring(context.Background(), tGroup.ID, 30)
	require.NoError(t, err)
	assert.True(t, containsItem(expiring, created.ID))
}

// An item created without an expiry date must not look like it expired long ago.
func TestItemsRepository_CreateWithoutExpiryStaysOutOfTheWarningList(t *testing.T) {
	location, err := tRepos.Locations.Create(context.Background(), tGroup.ID, locationFactory())
	require.NoError(t, err)
	t.Cleanup(func() { _ = tRepos.Locations.delete(context.Background(), location.ID) })

	created, err := tRepos.Items.Create(context.Background(), tGroup.ID, ItemCreate{
		Name:       "Schraubenzieher",
		LocationID: location.ID,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tRepos.Items.Delete(context.Background(), created.ID) })

	expiring, err := tRepos.Items.QueryExpiring(context.Background(), tGroup.ID, 3650)
	require.NoError(t, err)
	assert.False(t, containsItem(expiring, created.ID))
}
