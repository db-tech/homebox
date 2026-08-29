package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/schema/mixins"
)

// ConsumptionEntry records a single stock movement for an item, e.g. taking a
// can out of the pantry or restocking it after shopping.
type ConsumptionEntry struct {
	ent.Schema
}

func (ConsumptionEntry) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
	}
}

func (ConsumptionEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("item_id", "date"),
	}
}

func (ConsumptionEntry) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("item_id", uuid.UUID{}),
		field.Time("date"),
		// Always positive - the direction is carried by "type" so that reports
		// can sum consumed and restocked amounts independently.
		field.Int("amount").
			Positive(),
		field.Enum("type").
			Values("consume", "restock", "correction"),
		field.String("note").
			MaxLen(500).
			Optional(),
	}
}

func (ConsumptionEntry) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).
			Field("item_id").
			Ref("consumption_entries").
			Required().
			Unique(),
	}
}
