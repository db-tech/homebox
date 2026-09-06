package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeChecklist(t *testing.T) {
	assert.Equal(t, []string{"radio", "torch"}, decodeChecklist(`["radio","torch"]`))
}

func TestDecodeChecklist_NothingStoredIsNothingTicked(t *testing.T) {
	for _, stored := range []string{"", "   "} {
		assert.Equal(t, []string{}, decodeChecklist(stored))
	}
}

func TestDecodeChecklist_RubbishDoesNotBreakThePage(t *testing.T) {
	// A preference that cannot be read is worth losing. Failing the request
	// would take the whole stockpile page down with it.
	for _, stored := range []string{"not json", "{}", "[1,2,3]", "["} {
		assert.Equal(t, []string{}, decodeChecklist(stored), "%q", stored)
	}
}

func TestEncodeChecklist_EmptyStoresNothing(t *testing.T) {
	encoded, err := encodeChecklist(nil)
	assert.NoError(t, err)
	assert.Empty(t, encoded, "an empty list should not be stored as \"[]\"")
}

func TestChecklistRoundTrip(t *testing.T) {
	ticked := []string{"torch", "radio", "powerbank"}

	encoded, err := encodeChecklist(ticked)
	assert.NoError(t, err)
	assert.Equal(t, ticked, decodeChecklist(encoded))
}

func TestClamp(t *testing.T) {
	assert.Equal(t, 1, clamp(0, 1, 50), "a household of nobody is a slip")
	assert.Equal(t, 1, clamp(-4, 1, 50))
	assert.Equal(t, 50, clamp(9999, 1, 50))
	assert.Equal(t, 4, clamp(4, 1, 50))
}
