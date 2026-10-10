package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetModelAPIDocs(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.Where("1 = 1").Delete(&ModelAPIDoc{}).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Where("1 = 1").Delete(&ModelAPIDoc{}).Error)
	})
}

// Saving twice must overwrite rather than fail on the primary key, since the
// admin edits a model's document in place.
func TestSaveModelAPIDocUpsertsByModelName(t *testing.T) {
	resetModelAPIDocs(t)

	require.NoError(t, SaveModelAPIDoc("kling-v3", "# v1"))
	require.NoError(t, SaveModelAPIDoc(" kling-v3 ", "# v2"))

	doc, ok, err := GetModelAPIDoc("kling-v3")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "# v2", doc.Content)

	var count int64
	require.NoError(t, DB.Model(&ModelAPIDoc{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

// Clearing the document is how the admin restores the auto-generated panel.
func TestSaveModelAPIDocBlankContentDeletes(t *testing.T) {
	resetModelAPIDocs(t)

	require.NoError(t, SaveModelAPIDoc("kling-v3", "# doc"))
	require.NoError(t, SaveModelAPIDoc("kling-v3", "  \n\t"))

	_, ok, err := GetModelAPIDoc("kling-v3")
	require.NoError(t, err)
	assert.False(t, ok)
}

// The bound keeps the document inside a MySQL TEXT column on every database.
func TestSaveModelAPIDocRejectsOversizedContent(t *testing.T) {
	resetModelAPIDocs(t)

	require.NoError(t, SaveModelAPIDoc("at-limit", strings.Repeat("x", MaxModelAPIDocBytes)))

	err := SaveModelAPIDoc("too-big", strings.Repeat("x", MaxModelAPIDocBytes+1))
	require.ErrorIs(t, err, ErrModelAPIDocTooLarge)
	_, ok, err := GetModelAPIDoc("too-big")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestSaveModelAPIDocRequiresModelName(t *testing.T) {
	resetModelAPIDocs(t)

	require.Error(t, SaveModelAPIDoc("  ", "# doc"))
}

// The marketplace decides between the custom document and the generated panel
// from this flag, so it must track the saved document across cache refreshes.
func TestPricingFlagsModelsWithAPIDoc(t *testing.T) {
	resetPricingEndpointTestTables(t)
	resetModelAPIDocs(t)

	insertPricingEndpointChannel(t, 101, constant.ChannelTypeOpenAI, dto.ChannelOtherSettings{})
	insertPricingEndpointAbility(t, 101, "kling-v3")
	insertPricingEndpointAbility(t, 101, "gpt-4o")
	require.NoError(t, SaveModelAPIDoc("kling-v3", "# doc"))
	InitChannelCache()
	InvalidatePricingCache()

	hasDoc := pricingAPIDocFlags(GetPricing())
	assert.True(t, hasDoc["kling-v3"])
	assert.False(t, hasDoc["gpt-4o"])

	require.NoError(t, SaveModelAPIDoc("kling-v3", ""))
	InvalidatePricingCache()
	assert.False(t, pricingAPIDocFlags(GetPricing())["kling-v3"])
}

func pricingAPIDocFlags(pricings []Pricing) map[string]bool {
	flags := make(map[string]bool, len(pricings))
	for _, pricing := range pricings {
		flags[pricing.ModelName] = pricing.HasAPIDoc
	}
	return flags
}

func TestGetModelAPIDocNameSet(t *testing.T) {
	resetModelAPIDocs(t)

	require.NoError(t, SaveModelAPIDoc("kling-v3", "# doc"))
	require.NoError(t, SaveModelAPIDoc("image-2.5-sunburst", "# doc"))

	names, err := GetModelAPIDocNameSet()
	require.NoError(t, err)
	assert.Len(t, names, 2)
	assert.Contains(t, names, "kling-v3")
	assert.Contains(t, names, "image-2.5-sunburst")
}
