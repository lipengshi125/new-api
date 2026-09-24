package model

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/code_sample_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Call samples must never appear as model metadata: editing a sample for a
// model that has no 元信息 row must not create one, and must not modify an
// existing row. This is the contract that keeps the models table clean.
func TestUpdatePricingReadsSamplesWithoutTouchingModelRows(t *testing.T) {
	require.NoError(t, DB.Exec("DELETE FROM models").Error)
	require.NoError(t, DB.Exec("DELETE FROM abilities").Error)
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	t.Cleanup(func() {
		DB.Exec("DELETE FROM models")
		DB.Exec("DELETE FROM abilities")
		DB.Exec("DELETE FROM channels")
		code_sample_setting.SetModelSamplesForTest(nil)
		InvalidatePricingCache()
	})

	require.NoError(t, DB.Create(&Channel{Id: 8101, Type: 1, Status: 1,
		Models: "sample-only-model", Group: "default"}).Error)
	require.NoError(t, DB.Create(&Ability{ChannelId: 8101, Model: "sample-only-model",
		Group: "default", Enabled: true}).Error)

	samples := map[string]map[string]string{"openai": {"curl": "curl https://example.test"}}
	code_sample_setting.SetModelSamplesForTest(map[string]map[string]map[string]string{
		"sample-only-model": samples,
	})

	InvalidatePricingCache()
	pricing := GetPricing()

	var found *Pricing
	for i := range pricing {
		if pricing[i].ModelName == "sample-only-model" {
			found = &pricing[i]
		}
	}
	require.NotNil(t, found, "model with samples but no metadata row must still be priced")
	assert.Equal(t, samples, found.CodeSamples)

	var metaCount int64
	require.NoError(t, DB.Model(&Model{}).Count(&metaCount).Error)
	assert.Zero(t, metaCount, "storing a call sample must not create a 元信息 row")
}

// A model that does have metadata keeps it, and the sample is layered on top
// without the two interfering.
func TestUpdatePricingKeepsMetadataAlongsideSamples(t *testing.T) {
	require.NoError(t, DB.Exec("DELETE FROM models").Error)
	require.NoError(t, DB.Exec("DELETE FROM abilities").Error)
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	t.Cleanup(func() {
		DB.Exec("DELETE FROM models")
		DB.Exec("DELETE FROM abilities")
		DB.Exec("DELETE FROM channels")
		code_sample_setting.SetModelSamplesForTest(nil)
		InvalidatePricingCache()
	})

	require.NoError(t, DB.Create(&Channel{Id: 8102, Type: 1, Status: 1,
		Models: "documented-model", Group: "default"}).Error)
	require.NoError(t, DB.Create(&Ability{ChannelId: 8102, Model: "documented-model",
		Group: "default", Enabled: true}).Error)
	meta := Model{ModelName: "documented-model", Description: "keep me",
		Tags: "chat", Status: 1, SyncOfficial: 1}
	require.NoError(t, meta.Insert())

	code_sample_setting.SetModelSamplesForTest(map[string]map[string]map[string]string{
		"documented-model": {"openai": {"python": "from openai import OpenAI"}},
	})

	InvalidatePricingCache()
	pricing := GetPricing()

	var found *Pricing
	for i := range pricing {
		if pricing[i].ModelName == "documented-model" {
			found = &pricing[i]
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, "keep me", found.Description)
	assert.Equal(t, "chat", found.Tags)
	assert.Equal(t, "from openai import OpenAI", found.CodeSamples["openai"]["python"])

	var reloaded Model
	require.NoError(t, DB.Where("model_name = ?", "documented-model").First(&reloaded).Error)
	assert.Equal(t, "keep me", reloaded.Description)
	assert.Equal(t, meta.Id, reloaded.Id)
}
