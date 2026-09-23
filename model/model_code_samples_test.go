package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Model.Update() uses an explicit Select(...) allow-list, so a newly added
// column is silently dropped unless it is listed there. This asserts that
// code_samples actually survives a round trip through Update().
func TestModelUpdatePersistsCodeSamples(t *testing.T) {
	require.NoError(t, DB.Exec("DELETE FROM models").Error)
	t.Cleanup(func() { DB.Exec("DELETE FROM models") })

	m := Model{ModelName: "gpt-5-pro", Status: 1, SyncOfficial: 1}
	require.NoError(t, m.Insert())

	const samples = `{"openai":{"curl":"curl https://example.test/v1/chat/completions"}}`
	m.CodeSamples = samples
	m.Description = "updated"
	require.NoError(t, m.Update())

	var reloaded Model
	require.NoError(t, DB.Where("model_name = ?", "gpt-5-pro").First(&reloaded).Error)
	assert.Equal(t, samples, reloaded.CodeSamples)
	assert.Equal(t, "updated", reloaded.Description)
}

func TestUpsertModelCodeSamples(t *testing.T) {
	const samples = `{"anthropic":{"python":"import anthropic"}}`

	t.Run("creates row when model has no metadata yet", func(t *testing.T) {
		require.NoError(t, DB.Exec("DELETE FROM models").Error)
		t.Cleanup(func() { DB.Exec("DELETE FROM models") })

		require.NoError(t, UpsertModelCodeSamples("claude-sonnet-5", samples))

		var created Model
		require.NoError(t, DB.Where("model_name = ?", "claude-sonnet-5").First(&created).Error)
		assert.Equal(t, samples, created.CodeSamples)
		assert.Equal(t, NameRuleExact, created.NameRule)
		assert.Equal(t, 1, created.Status)
	})

	t.Run("updates existing row without touching other fields", func(t *testing.T) {
		require.NoError(t, DB.Exec("DELETE FROM models").Error)
		t.Cleanup(func() { DB.Exec("DELETE FROM models") })

		existing := Model{
			ModelName:   "claude-sonnet-5",
			Description: "keep me",
			Tags:        "chat",
			Status:      1,
		}
		require.NoError(t, existing.Insert())

		require.NoError(t, UpsertModelCodeSamples("claude-sonnet-5", samples))

		var reloaded Model
		require.NoError(t, DB.Where("model_name = ?", "claude-sonnet-5").First(&reloaded).Error)
		assert.Equal(t, existing.Id, reloaded.Id)
		assert.Equal(t, samples, reloaded.CodeSamples)
		assert.Equal(t, "keep me", reloaded.Description)
		assert.Equal(t, "chat", reloaded.Tags)
	})

	t.Run("ignores non-exact name rule rows so shared rules are not retargeted", func(t *testing.T) {
		require.NoError(t, DB.Exec("DELETE FROM models").Error)
		t.Cleanup(func() { DB.Exec("DELETE FROM models") })

		prefixRule := Model{ModelName: "claude-", NameRule: NameRulePrefix, Status: 1}
		require.NoError(t, prefixRule.Insert())

		require.NoError(t, UpsertModelCodeSamples("claude-", samples))

		// The prefix row backs many model names; the override must land on a
		// new exact-match row instead of the shared rule.
		var reloadedRule Model
		require.NoError(t, DB.Where("id = ?", prefixRule.Id).First(&reloadedRule).Error)
		assert.Empty(t, reloadedRule.CodeSamples)

		var exact Model
		require.NoError(t, DB.Where("model_name = ? AND name_rule = ?", "claude-", NameRuleExact).First(&exact).Error)
		assert.Equal(t, samples, exact.CodeSamples)
	})

	t.Run("clears override with empty payload", func(t *testing.T) {
		require.NoError(t, DB.Exec("DELETE FROM models").Error)
		t.Cleanup(func() { DB.Exec("DELETE FROM models") })

		require.NoError(t, UpsertModelCodeSamples("gpt-5-pro", samples))
		require.NoError(t, UpsertModelCodeSamples("gpt-5-pro", ""))

		var reloaded Model
		require.NoError(t, DB.Where("model_name = ?", "gpt-5-pro").First(&reloaded).Error)
		assert.Empty(t, reloaded.CodeSamples)
	})

	t.Run("rejects blank model name", func(t *testing.T) {
		assert.Error(t, UpsertModelCodeSamples("   ", samples))
	})
}

func TestParseModelCodeSamples(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected map[string]map[string]string
	}{
		{name: "empty string", raw: "", expected: nil},
		{name: "whitespace only", raw: "   ", expected: nil},
		{name: "malformed json", raw: `{"openai":`, expected: nil},
		{name: "wrong shape", raw: `{"openai":"curl ..."}`, expected: nil},
		{
			name:     "unknown endpoint type is rejected",
			raw:      `{"not-a-provider":{"curl":"curl ..."}}`,
			expected: nil,
		},
		{
			name:     "unknown language is rejected",
			raw:      `{"openai":{"cobol":"DISPLAY 'HI'"}}`,
			expected: nil,
		},
		{
			name:     "blank sample is dropped",
			raw:      `{"openai":{"curl":"   "}}`,
			expected: nil,
		},
		{
			name: "valid samples pass through",
			raw:  `{"openai":{"curl":"curl https://example.test","python":"from openai import OpenAI"}}`,
			expected: map[string]map[string]string{
				"openai": {
					"curl":   "curl https://example.test",
					"python": "from openai import OpenAI",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, parseModelCodeSamples(tc.raw))
		})
	}
}
