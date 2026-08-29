package repo

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setStock puts an item at a known quantity so the consumption maths in the
// tests below start from a predictable place.
func setStock(t *testing.T, itm ItemOut, quantity, minStock int) {
	t.Helper()

	_, err := tRepos.Items.UpdateByGroup(context.Background(), tGroup.ID, ItemUpdate{
		ID:         itm.ID,
		Name:       itm.Name,
		LocationID: itm.Location.ID,
		Quantity:   quantity,
		MinStock:   minStock,
	})
	require.NoError(t, err)
}

func TestConsumptionRepository_ConsumeReducesQuantity(t *testing.T) {
	itm := useItems(t, 1)[0]
	setStock(t, itm, 10, 0)

	entry, err := tRepos.Consumption.Create(context.Background(), tGroup.ID, itm.ID, ConsumptionCreate{
		Amount: 3,
		Type:   ConsumptionTypeConsume,
		Note:   "one can of tomatoes",
	})
	require.NoError(t, err)
	assert.Equal(t, 3, entry.Amount)
	assert.Equal(t, ConsumptionTypeConsume, entry.Type)
	assert.False(t, entry.Date.IsZero(), "date should default to now")

	updated, err := tRepos.Items.GetOne(context.Background(), itm.ID)
	require.NoError(t, err)
	assert.Equal(t, 7, updated.Quantity)
}

func TestConsumptionRepository_RestockIncreasesQuantity(t *testing.T) {
	itm := useItems(t, 1)[0]
	setStock(t, itm, 2, 0)

	_, err := tRepos.Consumption.Create(context.Background(), tGroup.ID, itm.ID, ConsumptionCreate{
		Amount: 6,
		Type:   ConsumptionTypeRestock,
	})
	require.NoError(t, err)

	updated, err := tRepos.Items.GetOne(context.Background(), itm.ID)
	require.NoError(t, err)
	assert.Equal(t, 8, updated.Quantity)
}

func TestConsumptionRepository_CorrectionLeavesQuantityAlone(t *testing.T) {
	itm := useItems(t, 1)[0]
	setStock(t, itm, 5, 0)

	_, err := tRepos.Consumption.Create(context.Background(), tGroup.ID, itm.ID, ConsumptionCreate{
		Amount: 2,
		Type:   ConsumptionTypeCorrection,
		Note:   "miscounted last week",
	})
	require.NoError(t, err)

	updated, err := tRepos.Items.GetOne(context.Background(), itm.ID)
	require.NoError(t, err)
	assert.Equal(t, 5, updated.Quantity)
}

// Consuming more than is in stock must fail and leave the stock untouched -
// otherwise the log would claim a movement that never happened.
func TestConsumptionRepository_ConsumeBeyondStockFails(t *testing.T) {
	itm := useItems(t, 1)[0]
	setStock(t, itm, 2, 0)

	_, err := tRepos.Consumption.Create(context.Background(), tGroup.ID, itm.ID, ConsumptionCreate{
		Amount: 5,
		Type:   ConsumptionTypeConsume,
	})
	require.ErrorIs(t, err, ErrInsufficientStock)

	updated, err := tRepos.Items.GetOne(context.Background(), itm.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, updated.Quantity, "failed consume must not change stock")

	log, err := tRepos.Consumption.GetByItem(context.Background(), tGroup.ID, itm.ID)
	require.NoError(t, err)
	assert.Empty(t, log, "failed consume must not leave a log entry")
}

func TestConsumptionRepository_UnknownTypeRejected(t *testing.T) {
	itm := useItems(t, 1)[0]
	setStock(t, itm, 5, 0)

	_, err := tRepos.Consumption.Create(context.Background(), tGroup.ID, itm.ID, ConsumptionCreate{
		Amount: 1,
		Type:   "teleport",
	})
	require.Error(t, err)

	updated, err := tRepos.Items.GetOne(context.Background(), itm.ID)
	require.NoError(t, err)
	assert.Equal(t, 5, updated.Quantity)
}

func TestConsumptionRepository_GetByItemIsNewestFirst(t *testing.T) {
	itm := useItems(t, 1)[0]
	setStock(t, itm, 20, 0)

	now := time.Now()
	for i, d := range []time.Time{now.AddDate(0, 0, -5), now.AddDate(0, 0, -1), now.AddDate(0, 0, -3)} {
		_, err := tRepos.Consumption.Create(context.Background(), tGroup.ID, itm.ID, ConsumptionCreate{
			Amount: i + 1,
			Type:   ConsumptionTypeConsume,
			Date:   d,
		})
		require.NoError(t, err)
	}

	log, err := tRepos.Consumption.GetByItem(context.Background(), tGroup.ID, itm.ID)
	require.NoError(t, err)
	require.Len(t, log, 3)

	for i := 1; i < len(log); i++ {
		assert.False(t, log[i].Date.After(log[i-1].Date), "entries must be newest first")
	}
}

// The log is scoped to the group; another group must not be able to read it.
func TestConsumptionRepository_GetByItemIsGroupScoped(t *testing.T) {
	itm := useItems(t, 1)[0]
	setStock(t, itm, 5, 0)

	_, err := tRepos.Consumption.Create(context.Background(), tGroup.ID, itm.ID, ConsumptionCreate{
		Amount: 1,
		Type:   ConsumptionTypeConsume,
	})
	require.NoError(t, err)

	other, err := tRepos.Groups.GroupCreate(context.Background(), "other-group")
	require.NoError(t, err)

	log, err := tRepos.Consumption.GetByItem(context.Background(), other.ID, itm.ID)
	require.NoError(t, err)
	assert.Empty(t, log)
}

func TestConsumptionRepository_CreateIsGroupScoped(t *testing.T) {
	itm := useItems(t, 1)[0]
	setStock(t, itm, 5, 0)

	other, err := tRepos.Groups.GroupCreate(context.Background(), "other-group-create")
	require.NoError(t, err)

	_, err = tRepos.Consumption.Create(context.Background(), other.ID, itm.ID, ConsumptionCreate{
		Amount: 1,
		Type:   ConsumptionTypeConsume,
	})
	require.Error(t, err, "must not be able to consume another group's item")

	updated, err := tRepos.Items.GetOne(context.Background(), itm.ID)
	require.NoError(t, err)
	assert.Equal(t, 5, updated.Quantity)
}

func TestConsumptionRepository_Delete(t *testing.T) {
	itm := useItems(t, 1)[0]
	setStock(t, itm, 5, 0)

	entry, err := tRepos.Consumption.Create(context.Background(), tGroup.ID, itm.ID, ConsumptionCreate{
		Amount: 2,
		Type:   ConsumptionTypeConsume,
	})
	require.NoError(t, err)

	require.NoError(t, tRepos.Consumption.Delete(context.Background(), tGroup.ID, entry.ID))

	log, err := tRepos.Consumption.GetByItem(context.Background(), tGroup.ID, itm.ID)
	require.NoError(t, err)
	assert.Empty(t, log)

	// Deleting a log entry is a bookkeeping fix and must not move stock.
	updated, err := tRepos.Items.GetOne(context.Background(), itm.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, updated.Quantity)
}

func TestConsumptionRepository_Statistics(t *testing.T) {
	itm := useItems(t, 1)[0]
	setStock(t, itm, 100, 0)

	for _, c := range []ConsumptionCreate{
		{Amount: 4, Type: ConsumptionTypeConsume},
		{Amount: 3, Type: ConsumptionTypeConsume},
		{Amount: 10, Type: ConsumptionTypeRestock},
		{Amount: 1, Type: ConsumptionTypeCorrection},
	} {
		_, err := tRepos.Consumption.Create(context.Background(), tGroup.ID, itm.ID, c)
		require.NoError(t, err)
	}

	stats, err := tRepos.Consumption.GetStatistics(context.Background(), tGroup.ID, 7)
	require.NoError(t, err)

	var found *ConsumptionSummary
	for i := range stats {
		if stats[i].ItemID == itm.ID {
			found = &stats[i]
			break
		}
	}

	require.NotNil(t, found, "item should appear in the statistics")
	assert.Equal(t, 7, found.TotalConsumed)
	assert.Equal(t, 10, found.TotalRestocked)
	assert.Equal(t, 4, found.Entries)
	assert.Equal(t, itm.Name, found.ItemName)
	// 7 consumed over a 7 day window is 7 per week.
	assert.InDelta(t, 7.0, found.AveragePerWeek, 0.001)
}

// Entries older than the requested period must not be counted.
func TestConsumptionRepository_StatisticsRespectsPeriod(t *testing.T) {
	itm := useItems(t, 1)[0]
	setStock(t, itm, 100, 0)

	_, err := tRepos.Consumption.Create(context.Background(), tGroup.ID, itm.ID, ConsumptionCreate{
		Amount: 9,
		Type:   ConsumptionTypeConsume,
		Date:   time.Now().AddDate(0, 0, -60),
	})
	require.NoError(t, err)

	stats, err := tRepos.Consumption.GetStatistics(context.Background(), tGroup.ID, 7)
	require.NoError(t, err)

	for _, s := range stats {
		if s.ItemID == itm.ID {
			t.Fatalf("entry from 60 days ago must not appear in a 7 day window")
		}
	}
}

// Two people taking things out at the same time must not lose a movement.
// Reading the quantity and writing back a computed value would let one of the
// two writes overwrite the other; the conditional UPDATE in Create prevents it.
func TestConsumptionRepository_ConcurrentConsumeDoesNotLoseMovements(t *testing.T) {
	itm := useItems(t, 1)[0]
	setStock(t, itm, 50, 0)

	const workers = 10

	var wg sync.WaitGroup
	errs := make(chan error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := tRepos.Consumption.Create(context.Background(), tGroup.ID, itm.ID, ConsumptionCreate{
				Amount: 1,
				Type:   ConsumptionTypeConsume,
			})
			errs <- err
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	updated, err := tRepos.Items.GetOne(context.Background(), itm.ID)
	require.NoError(t, err)
	assert.Equal(t, 50-workers, updated.Quantity, "every concurrent consume must be applied exactly once")

	log, err := tRepos.Consumption.GetByItem(context.Background(), tGroup.ID, itm.ID)
	require.NoError(t, err)
	assert.Len(t, log, workers)
}

// Consuming down to exactly zero must succeed; going one further must not.
func TestConsumptionRepository_ConcurrentConsumeStopsAtZero(t *testing.T) {
	itm := useItems(t, 1)[0]
	setStock(t, itm, 3, 0)

	const workers = 8

	var wg sync.WaitGroup
	results := make(chan error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := tRepos.Consumption.Create(context.Background(), tGroup.ID, itm.ID, ConsumptionCreate{
				Amount: 1,
				Type:   ConsumptionTypeConsume,
			})
			results <- err
		}()
	}

	wg.Wait()
	close(results)

	succeeded := 0
	for err := range results {
		if err == nil {
			succeeded++
			continue
		}
		require.ErrorIs(t, err, ErrInsufficientStock)
	}

	assert.Equal(t, 3, succeeded, "exactly the available stock may be consumed")

	updated, err := tRepos.Items.GetOne(context.Background(), itm.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, updated.Quantity)
}
