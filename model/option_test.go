package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// InitOptionMap must seed CustomRatios / CustomRatioNotes so GET /api/option/
// surfaces the keys even before any explicit save. Without the seed, the
// frontend read-back (getOptionValue only keeps keys present in defaults)
// silently drops them.
func TestInitOptionMapSeedsCustomRatios(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Option{}))

	InitOptionMap()

	for _, key := range []string{"CustomRatios", "CustomRatioNotes", "ModelBillingNotes"} {
		common.OptionMapRWMutex.RLock()
		value, ok := common.OptionMap[key]
		common.OptionMapRWMutex.RUnlock()

		require.True(t, ok, "%s must be seeded into OptionMap", key)
		assert.NotEmpty(t, value, "seeded %s JSON should be non-empty (e.g. \"{}\")", key)
	}
}
