package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/schema/mixins"
)

// Group holds the schema definition for the Group entity.
type Group struct {
	ent.Schema
}

func (Group) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
	}
}

// Fields of the Home.
func (Group) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			MaxLen(255).
			NotEmpty(),
		field.String("currency").
			Default("usd"),
		// API key for the meal suggestions, kept per group so a household can
		// switch them on itself rather than needing the deployment edited.
		//
		// Sensitive keeps it out of String() and out of anything Ent marshals,
		// because it must never travel back to a browser. It is stored in plain
		// text, as notifier URLs already are: Homebox has no secret store, and
		// encrypting it with a key that sits in the same database would look
		// like protection without being any.
		field.String("recipes_api_key").
			Sensitive().
			MaxLen(255).
			Optional(),
		// Transcription key for voice entry. A second provider is needed because
		// the model that reads the transcript cannot take audio at all. Same
		// treatment as the other one: sensitive, plain text, never returned.
		field.String("voice_api_key").
			Sensitive().
			MaxLen(255).
			Optional(),
		// How many people the emergency stockpile has to feed, and for how
		// long. Both are household facts rather than per-user preferences, so
		// they live with the group.
		// Optional rather than defaulted so that adding them is a plain ALTER
		// on an existing database instead of a rebuild of the whole table.
		// Unset reads as zero, and the defaults - one person, ten days - are
		// applied when the figures are used.
		field.Int("household_size").Optional(),
		field.Int("emergency_days").Optional(),
		// Which items on the non-food checklist have been ticked off, as a JSON
		// array of ids. A checkbox per group is all this needs, and a table for
		// it would be a table for one column.
		field.Text("emergency_checklist").Optional(),
	}
}

// Edges of the Home.
func (Group) Edges() []ent.Edge {
	owned := func(name string, t any) ent.Edge {
		return edge.To(name, t).
			Annotations(entsql.Annotation{
				OnDelete: entsql.Cascade,
			})
	}

	return []ent.Edge{
		owned("users", User.Type),
		owned("locations", Location.Type),
		owned("items", Item.Type),
		owned("labels", Label.Type),
		owned("documents", Document.Type),
		owned("invitation_tokens", GroupInvitationToken.Type),
		owned("notifiers", Notifier.Type),
		// $scaffold_edge
	}
}

// GroupMixin when embedded in an ent.Schema, adds a reference to
// the Group entity.
type GroupMixin struct {
	ref   string
	field string
	mixin.Schema
}

func (g GroupMixin) Fields() []ent.Field {
	if g.field != "" {
		return []ent.Field{
			field.UUID(g.field, uuid.UUID{}),
		}
	}

	return nil
}

func (g GroupMixin) Edges() []ent.Edge {
	edge := edge.From("group", Group.Type).
		Ref(g.ref).
		Unique().
		Required()

	if g.field != "" {
		edge = edge.Field(g.field)
	}

	return []ent.Edge{edge}
}
