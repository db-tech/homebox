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

// The two keys are independent: setting one must not disturb the other, or
// configuring voice entry would switch the meal suggestions off.
func TestGroupRepository_TheTwoKeysDoNotInterfere(t *testing.T) {
	ctx := context.Background()

	group, err := tRepos.Groups.GroupCreate(ctx, "__test__.two_keys")
	require.NoError(t, err)
	t.Cleanup(func() { _ = tRepos.Groups.db.Group.DeleteOneID(group.ID).Exec(ctx) })

	base := GroupUpdate{Name: group.Name, Currency: group.Currency}

	recipes := "sk-ant-recipes"
	withRecipes := base
	withRecipes.RecipesAPIKey = &recipes
	_, err = tRepos.Groups.GroupUpdate(ctx, group.ID, withRecipes)
	require.NoError(t, err)

	voice := "sk-openai-voice"
	withVoice := base
	withVoice.VoiceAPIKey = &voice
	updated, err := tRepos.Groups.GroupUpdate(ctx, group.ID, withVoice)
	require.NoError(t, err)

	assert.True(t, updated.HasRecipesAPIKey, "setting the voice key must leave the other one alone")
	assert.True(t, updated.HasVoiceAPIKey)

	storedVoice, err := tRepos.Groups.VoiceAPIKey(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, voice, storedVoice)

	storedRecipes, err := tRepos.Groups.RecipesAPIKey(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, recipes, storedRecipes)

	// Clearing one leaves the other.
	blank := ""
	cleared := base
	cleared.VoiceAPIKey = &blank
	afterClear, err := tRepos.Groups.GroupUpdate(ctx, group.ID, cleared)
	require.NoError(t, err)

	assert.False(t, afterClear.HasVoiceAPIKey)
	assert.True(t, afterClear.HasRecipesAPIKey)
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
