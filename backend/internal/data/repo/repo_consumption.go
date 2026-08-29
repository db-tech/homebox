package repo

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/reporting/eventbus"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/consumptionentry"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/group"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/item"
)

// ConsumptionRepository records stock movements for pantry items. Recording an
// entry and adjusting the item quantity happen in one transaction so the log can
// never drift away from the stock it is supposed to explain.
type ConsumptionRepository struct {
	db  *ent.Client
	bus *eventbus.EventBus
}

// ErrInsufficientStock is returned when consuming more than is currently in stock.
var ErrInsufficientStock = errors.New("not enough stock to consume that amount")

// ErrItemNotFound is returned when the item does not exist in the caller's group.
var ErrItemNotFound = errors.New("item not found")

const (
	ConsumptionTypeConsume    = "consume"
	ConsumptionTypeRestock    = "restock"
	ConsumptionTypeCorrection = "correction"
)

type (
	ConsumptionCreate struct {
		// Amount is always positive; Type carries the direction.
		Amount int    `json:"amount" validate:"required,min=1"`
		Type   string `json:"type"   validate:"required,oneof=consume restock correction"`
		Note   string `json:"note"   validate:"max=500"`
		// Date defaults to now when zero.
		Date time.Time `json:"date"`
	}

	ConsumptionEntry struct {
		ID        uuid.UUID `json:"id"`
		ItemID    uuid.UUID `json:"itemId"`
		Date      time.Time `json:"date"`
		Amount    int       `json:"amount"`
		Type      string    `json:"type"`
		Note      string    `json:"note"`
		CreatedAt time.Time `json:"createdAt"`
	}

	// ConsumptionSummary aggregates the log for a single item over a period.
	ConsumptionSummary struct {
		ItemID         uuid.UUID `json:"itemId"`
		ItemName       string    `json:"itemName"`
		TotalConsumed  int       `json:"totalConsumed"`
		TotalRestocked int       `json:"totalRestocked"`
		Entries        int       `json:"entries"`
		// AveragePerWeek is the consumed amount projected onto a 7 day window
		// over the requested period. Useful to judge how long stock will last.
		AveragePerWeek float64 `json:"averagePerWeek"`
	}
)

var mapEachConsumptionEntry = mapTEachFunc(mapConsumptionEntry)

func mapConsumptionEntry(e *ent.ConsumptionEntry) ConsumptionEntry {
	return ConsumptionEntry{
		ID:        e.ID,
		ItemID:    e.ItemID,
		Date:      e.Date,
		Amount:    e.Amount,
		Type:      e.Type.String(),
		Note:      e.Note,
		CreatedAt: e.CreatedAt,
	}
}

func (r *ConsumptionRepository) publishMutationEvent(gid uuid.UUID) {
	if r.bus != nil {
		r.bus.Publish(eventbus.EventItemMutation, eventbus.GroupMutationEvent{GID: gid})
	}
}

// maxLockRetries bounds how often a stock movement is retried when the database
// reports a transient lock conflict. SQLite serialises writers, so two people
// scanning at the same moment can collide; retrying a handful of times turns
// that into a short wait instead of an error in the user's face.
const maxLockRetries = 5

// isTransientLockError reports whether err is SQLite telling us the database was
// busy, which is worth another attempt, as opposed to a real failure.
//
// This matches on the message rather than a driver error type on purpose: the
// server links modernc.org/sqlite through pkgs/cgofreesqlite while the tests run
// on mattn/go-sqlite3, and importing either one here would register a second
// driver under the name "sqlite3" and panic at startup. Both drivers surface
// SQLite's own wording for SQLITE_BUSY and SQLITE_LOCKED.
func isTransientLockError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "database table is locked")
}

// Create records a stock movement and applies it to the item quantity.
//
// consume    -> quantity decreases
// restock    -> quantity increases
// correction -> quantity is not touched; the entry only annotates the log
func (r *ConsumptionRepository) Create(ctx context.Context, gid, itemID uuid.UUID, data ConsumptionCreate) (ConsumptionEntry, error) {
	if data.Amount < 1 {
		return ConsumptionEntry{}, errors.New("amount must be at least 1")
	}

	switch data.Type {
	case ConsumptionTypeConsume, ConsumptionTypeRestock, ConsumptionTypeCorrection:
	default:
		return ConsumptionEntry{}, fmt.Errorf("unknown consumption type: %q", data.Type)
	}

	date := data.Date
	if date.IsZero() {
		date = time.Now()
	}

	var (
		entry ConsumptionEntry
		err   error
	)

	for attempt := 0; attempt < maxLockRetries; attempt++ {
		entry, err = r.createOnce(ctx, gid, itemID, data, date)
		if !isTransientLockError(err) {
			break
		}

		select {
		case <-ctx.Done():
			return ConsumptionEntry{}, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 5 * time.Millisecond):
		}
	}

	if err != nil {
		return ConsumptionEntry{}, err
	}

	r.publishMutationEvent(gid)

	return entry, nil
}

// createOnce is a single attempt at recording a movement.
//
// The quantity change is expressed as one conditional UPDATE rather than a read
// followed by a write. Reading the quantity and writing back a computed value
// would lose one of two concurrent movements, and on SQLite in WAL mode the
// read-to-write upgrade fails outright once another connection commits in
// between. Starting the transaction with the write keeps both problems away.
func (r *ConsumptionRepository) createOnce(ctx context.Context, gid, itemID uuid.UUID, data ConsumptionCreate, date time.Time) (ConsumptionEntry, error) {
	tx, err := r.db.Tx(ctx)
	if err != nil {
		return ConsumptionEntry{}, err
	}

	rollback := func(cause error) (ConsumptionEntry, error) {
		if rerr := tx.Rollback(); rerr != nil {
			return ConsumptionEntry{}, fmt.Errorf("%w (rollback failed: %w)", cause, rerr)
		}
		return ConsumptionEntry{}, cause
	}

	owned := func() (bool, error) {
		return tx.Item.Query().Where(
			item.ID(itemID),
			item.HasGroupWith(group.ID(gid)),
		).Exist(ctx)
	}

	switch data.Type {
	case ConsumptionTypeConsume:
		// The QuantityGTE predicate is the stock check: if it does not hold, no
		// row is updated and nothing was taken out.
		affected, err := tx.Item.Update().Where(
			item.ID(itemID),
			item.HasGroupWith(group.ID(gid)),
			item.QuantityGTE(data.Amount),
		).AddQuantity(-data.Amount).Save(ctx)
		if err != nil {
			return rollback(err)
		}
		if affected == 0 {
			// Either the stock is too low or the item is not ours. Tell them
			// apart so the caller can report something useful.
			exists, err := owned()
			if err != nil {
				return rollback(err)
			}
			if !exists {
				return rollback(ErrItemNotFound)
			}
			return rollback(ErrInsufficientStock)
		}

	case ConsumptionTypeRestock:
		affected, err := tx.Item.Update().Where(
			item.ID(itemID),
			item.HasGroupWith(group.ID(gid)),
		).AddQuantity(data.Amount).Save(ctx)
		if err != nil {
			return rollback(err)
		}
		if affected == 0 {
			return rollback(ErrItemNotFound)
		}

	case ConsumptionTypeCorrection:
		// Nothing to write to the item, so the ownership check is all there is.
		exists, err := owned()
		if err != nil {
			return rollback(err)
		}
		if !exists {
			return rollback(ErrItemNotFound)
		}
	}

	entry, err := tx.ConsumptionEntry.Create().
		SetItemID(itemID).
		SetDate(date).
		SetAmount(data.Amount).
		SetType(consumptionentry.Type(data.Type)).
		SetNote(data.Note).
		Save(ctx)
	if err != nil {
		return rollback(err)
	}

	if err := tx.Commit(); err != nil {
		return ConsumptionEntry{}, err
	}

	return mapConsumptionEntry(entry), nil
}

// GetByItem returns the log for one item, newest first.
func (r *ConsumptionRepository) GetByItem(ctx context.Context, gid, itemID uuid.UUID) ([]ConsumptionEntry, error) {
	entries, err := r.db.ConsumptionEntry.Query().
		Where(
			consumptionentry.ItemID(itemID),
			consumptionentry.HasItemWith(item.HasGroupWith(group.ID(gid))),
		).
		Order(consumptionentry.ByDate(sql.OrderDesc())).
		All(ctx)
	if err != nil {
		return nil, err
	}

	return mapEachConsumptionEntry(entries), nil
}

// Delete removes a single log entry. The item quantity is deliberately left
// alone - the entry is a historical record, and silently moving stock around
// when correcting a typo in the log would surprise the user.
func (r *ConsumptionRepository) Delete(ctx context.Context, gid, id uuid.UUID) error {
	_, err := r.db.ConsumptionEntry.Delete().
		Where(
			consumptionentry.ID(id),
			consumptionentry.HasItemWith(item.HasGroupWith(group.ID(gid))),
		).
		Exec(ctx)

	if err == nil {
		r.publishMutationEvent(gid)
	}

	return err
}

// GetStatistics aggregates consumption per item over the last "days" days,
// most-consumed first.
func (r *ConsumptionRepository) GetStatistics(ctx context.Context, gid uuid.UUID, days int) ([]ConsumptionSummary, error) {
	if days < 1 {
		days = 30
	}
	since := time.Now().AddDate(0, 0, -days)

	entries, err := r.db.ConsumptionEntry.Query().
		Where(
			consumptionentry.HasItemWith(item.HasGroupWith(group.ID(gid))),
			consumptionentry.DateGTE(since),
		).
		WithItem().
		All(ctx)
	if err != nil {
		return nil, err
	}

	byItem := map[uuid.UUID]*ConsumptionSummary{}
	for _, e := range entries {
		s, ok := byItem[e.ItemID]
		if !ok {
			s = &ConsumptionSummary{ItemID: e.ItemID}
			if e.Edges.Item != nil {
				s.ItemName = e.Edges.Item.Name
			}
			byItem[e.ItemID] = s
		}

		s.Entries++
		switch e.Type.String() {
		case ConsumptionTypeConsume:
			s.TotalConsumed += e.Amount
		case ConsumptionTypeRestock:
			s.TotalRestocked += e.Amount
		}
	}

	out := make([]ConsumptionSummary, 0, len(byItem))
	for _, s := range byItem {
		s.AveragePerWeek = float64(s.TotalConsumed) * 7 / float64(days)
		out = append(out, *s)
	}

	// Most consumed first, ties broken by name so the output is stable.
	sort.Slice(out, func(i, j int) bool {
		if out[i].TotalConsumed != out[j].TotalConsumed {
			return out[i].TotalConsumed > out[j].TotalConsumed
		}
		return out[i].ItemName < out[j].ItemName
	})

	return out, nil
}
