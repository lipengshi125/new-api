package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// InitOptionMap must seed CustomRatios so GET /api/option/ surfaces the key
// even before any explicit save. Without the seed, the frontend read-back
// (getOptionValue only keeps keys present in defaults) silently drops it.
func TestInitOptionMapSeedsCustomRatios(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Option{}))

	InitOptionMap()

	common.OptionMapRWMutex.RLock()
	value, ok := common.OptionMap["CustomRatios"]
	common.OptionMapRWMutex.RUnlock()

	require.True(t, ok, "CustomRatios must be seeded into OptionMap")
	assert.NotEmpty(t, value, "seeded CustomRatios JSON should be non-empty (e.g. \"{}\")")
}
