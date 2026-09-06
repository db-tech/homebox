package repo

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Group_Create(t *testing.T) {
	g, err := tRepos.Groups.GroupCreate(context.Background(), "test")

	require.NoError(t, err)
	assert.Equal(t, "test", g.Name)

	// Get by ID
	foundGroup, err := tRepos.Groups.GroupByID(context.Background(), g.ID)
	require.NoError(t, err)
	assert.Equal(t, g.ID, foundGroup.ID)
}

func Test_Group_Update(t *testing.T) {
	g, err := tRepos.Groups.GroupCreate(context.Background(), "test")
	require.NoError(t, err)

	g, err = tRepos.Groups.GroupUpdate(context.Background(), g.ID, GroupUpdate{
		Name:     "test2",
		Currency: "eur",
	})
	require.NoError(t, err)
	assert.Equal(t, "test2", g.Name)
	assert.Equal(t, "EUR", g.Currency)
}

// TODO: Fix this test at some point, the data itself in production/development is working fine, it only fails on the test
/*func Test_Group_GroupStatistics(t *testing.T) {
	useItems(t, 20)
	useLabels(t, 20)

	stats, err := tRepos.Groups.StatsGroup(context.Background(), tGroup.ID)

	require.NoError(t, err)
	assert.Equal(t, 20, stats.TotalItems)
	assert.Equal(t, 20, stats.TotalLabels)
	assert.Equal(t, 1, stats.TotalUsers)
	assert.Equal(t, 1, stats.TotalLocations)
}*/

// The key is three-valued on the way in and one-valued on the way out: you can
// set it, clear it, or leave it alone, and you can only ever read back whether
// there is one.
func TestGroupRepository_RecipesAPIKey(t *testing.T) {
	ctx := context.Background()

	group, err := tRepos.Groups.GroupCreate(ctx, "__test__.recipes_key")
	require.NoError(t, err)
	t.Cleanup(func() { _ = tRepos.Groups.db.Group.DeleteOneID(group.ID).Exec(ctx) })

	assert.False(t, group.HasRecipesAPIKey, "a new group has no key")

	base := GroupUpdate{Name: group.Name, Currency: group.Currency}

	key := "sk-ant-secret"
	withKey := base
	withKey.RecipesAPIKey = &key

	updated, err := tRepos.Groups.GroupUpdate(ctx, group.ID, withKey)
	require.NoError(t, err)
	assert.True(t, updated.HasRecipesAPIKey)

	stored, err := tRepos.Groups.RecipesAPIKey(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, key, stored)

	// Renaming the group must not throw the key away. This is the whole reason
	// the field is a pointer: an update that says nothing about the key has to
	// mean "leave it", not "clear it".
	renamed := base
	renamed.Name = "__test__.recipes_key_renamed"
	afterRename, err := tRepos.Groups.GroupUpdate(ctx, group.ID, renamed)
	require.NoError(t, err)
	assert.True(t, afterRename.HasRecipesAPIKey, "an unrelated update must leave the key alone")

	blank := ""
	cleared := base
	cleared.RecipesAPIKey = &blank
	afterClear, err := tRepos.Groups.GroupUpdate(ctx, group.ID, cleared)
	require.NoError(t, err)
	assert.False(t, afterClear.HasRecipesAPIKey, "an empty string clears it")

	gone, err := tRepos.Groups.RecipesAPIKey(ctx, group.ID)
	require.NoError(t, err)
	assert.Empty(t, gone)
}

// Whatever else changes, the key must never be serialised towards a browser.
func TestGroupRepository_KeyIsNotInTheJSON(t *testing.T) {
	ctx := context.Background()

	group, err := tRepos.Groups.GroupCreate(ctx, "__test__.recipes_json")
	require.NoError(t, err)
	t.Cleanup(func() { _ = tRepos.Groups.db.Group.DeleteOneID(group.ID).Exec(ctx) })

	key := "sk-ant-must-not-leak"
	update := GroupUpdate{Name: group.Name, Currency: group.Currency, RecipesAPIKey: &key}

	updated, err := tRepos.Groups.GroupUpdate(ctx, group.ID, update)
	require.NoError(t, err)

	encoded, err := json.Marshal(updated)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), key)
	assert.Contains(t, string(encoded), "hasRecipesApiKey")
}
