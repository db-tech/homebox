package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/group"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/item"
)

// QueryExpiring returns the non-archived items of a group whose expiry date falls
// on or before "within" days from now, soonest first. Items that already expired
// are included so that they don't silently drop off the warning list.
//
// Items without an expiry date are never returned - the field is opt-in.
func (e *ItemsRepository) QueryExpiring(ctx context.Context, gid uuid.UUID, within int) ([]ItemSummary, error) {
	cutoff := time.Now().AddDate(0, 0, within)

	q := e.db.Item.Query().Where(
		item.HasGroupWith(group.ID(gid)),
		item.Archived(false),
		item.ExpiryDateNotNil(),
		// Rows written before the NULL handling in UpdateByGroup was fixed may
		// still carry the zero time; those mean "not set", not "long expired".
		item.ExpiryDateNEQ(time.Time{}),
		item.ExpiryDateLTE(cutoff),
	).Order(
		ent.Asc(item.FieldExpiryDate),
	).WithLabel().WithLocation()

	return mapItemsSummaryErr(q.All(ctx))
}

// QueryLowStock returns the non-archived items of a group that have a minimum
// stock configured and whose stock has fallen to or below it.
//
// A min_stock of 0 means "not tracked" and is always excluded.
//
// One product can be split across several items. Scanning the same barcode with
// two different best-before dates deliberately creates two of them, because an
// item carries a single expiry date and merging them would throw one away. The
// minimum stock, however, belongs to the product: two batches of four and two
// tins are six tins in the cupboard, not two shortages.
//
// So items sharing a barcode are judged on their combined quantity and reported
// once. IMPORTANT: on those rows Quantity holds the product's total across all
// its batches rather than the row's own stock, because that total is what "how
// far below the minimum am I" and the shopping list are asking about. Items
// without a barcode cannot be grouped and keep their own quantity.
func (e *ItemsRepository) QueryLowStock(ctx context.Context, gid uuid.UUID) ([]ItemSummary, error) {
	tracked, err := mapItemsSummaryErr(e.db.Item.Query().Where(
		item.HasGroupWith(group.ID(gid)),
		item.Archived(false),
		item.MinStockGT(0),
	).Order(
		ent.Asc(item.FieldName),
	).WithLabel().WithLocation().All(ctx))
	if err != nil {
		return nil, err
	}
	if len(tracked) == 0 {
		return []ItemSummary{}, nil
	}

	totals, err := e.stockByBarcode(ctx, gid, tracked)
	if err != nil {
		return nil, err
	}

	low := make([]ItemSummary, 0, len(tracked))
	reported := make(map[string]bool, len(tracked))

	for _, summary := range tracked {
		// A barcode is reported once, with the product's whole stock. The batch
		// standing in for it is the one that runs out first, so the row points
		// at what you will actually reach for rather than at whichever entry
		// happened to sort first.
		if summary.Barcode != "" {
			if reported[summary.Barcode] {
				continue
			}
			summary = soonestOf(tracked, summary.Barcode)
			if total, ok := totals[summary.Barcode]; ok {
				summary.Quantity = total
			}
		}

		if summary.Quantity > summary.MinStock {
			continue
		}

		if summary.Barcode != "" {
			reported[summary.Barcode] = true
		}
		low = append(low, summary)
	}

	return low, nil
}

// soonestOf returns the batch of a barcode with the nearest best-before date.
// Batches without one come last: a date is information, and its absence is no
// reason to reach for that tin first.
func soonestOf(items []ItemSummary, barcode string) ItemSummary {
	var best ItemSummary
	found := false

	for _, candidate := range items {
		if candidate.Barcode != barcode {
			continue
		}
		if !found || expiresBefore(candidate, best) {
			best, found = candidate, true
		}
	}

	return best
}

func expiresBefore(a, b ItemSummary) bool {
	left, right := a.ExpiryDate.Time(), b.ExpiryDate.Time()

	switch {
	case left.IsZero():
		return false
	case right.IsZero():
		return true
	default:
		return left.Before(right)
	}
}

// stockByBarcode totals the quantity held under each of the given items'
// barcodes, counting every batch in the group rather than only the ones that
// carry a minimum themselves.
func (e *ItemsRepository) stockByBarcode(ctx context.Context, gid uuid.UUID, of []ItemSummary) (map[string]int, error) {
	wanted := make([]string, 0, len(of))
	seen := make(map[string]bool, len(of))

	for _, summary := range of {
		if summary.Barcode == "" || seen[summary.Barcode] {
			continue
		}
		seen[summary.Barcode] = true
		wanted = append(wanted, summary.Barcode)
	}

	if len(wanted) == 0 {
		return map[string]int{}, nil
	}

	var rows []struct {
		Barcode  string `json:"barcode"`
		Quantity int    `json:"quantity"`
	}

	err := e.db.Item.Query().Where(
		item.HasGroupWith(group.ID(gid)),
		item.Archived(false),
		item.BarcodeIn(wanted...),
	).Select(item.FieldBarcode, item.FieldQuantity).Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}

	totals := make(map[string]int, len(wanted))
	for _, row := range rows {
		totals[row.Barcode] += row.Quantity
	}

	return totals, nil
}

// QueryByBarcode returns the non-archived items of a group carrying the given
// barcode. A barcode is not enforced to be unique - the same product may sit in
// two locations - so this returns every match and lets the caller decide.
func (e *ItemsRepository) QueryByBarcode(ctx context.Context, gid uuid.UUID, barcode string) ([]ItemSummary, error) {
	if barcode == "" {
		return []ItemSummary{}, nil
	}

	q := e.db.Item.Query().Where(
		item.HasGroupWith(group.ID(gid)),
		item.Archived(false),
		item.Barcode(barcode),
	).Order(
		ent.Asc(item.FieldName),
	).WithLabel().WithLocation()

	return mapItemsSummaryErr(q.All(ctx))
}
