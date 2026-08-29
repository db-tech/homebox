package repo

import (
	"context"
	"time"

	"entgo.io/ent/dialect/sql"
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
// stock configured and whose quantity has fallen to or below it.
//
// A min_stock of 0 means "not tracked" and is always excluded.
func (e *ItemsRepository) QueryLowStock(ctx context.Context, gid uuid.UUID) ([]ItemSummary, error) {
	q := e.db.Item.Query().Where(
		item.HasGroupWith(group.ID(gid)),
		item.Archived(false),
		item.MinStockGT(0),
		// Column-to-column comparison: sql.LTE would bind the second argument as
		// a literal value rather than reading it as a column.
		func(s *sql.Selector) {
			s.Where(sql.ColumnsLTE(s.C(item.FieldQuantity), s.C(item.FieldMinStock)))
		},
	).Order(
		ent.Asc(item.FieldName),
	).WithLabel().WithLocation()

	return mapItemsSummaryErr(q.All(ctx))
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
