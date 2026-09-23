package code_sample_setting

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Normalize is the single validation boundary for admin-authored samples: it
// guards both the per-model override endpoint and the stored global templates.
func TestNormalize(t *testing.T) {
	t.Run("rejects unknown endpoint type", func(t *testing.T) {
		_, err := Normalize(map[string]map[string]string{
			"totally-made-up": {"curl": "curl ..."},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported endpoint type")
	})

	t.Run("rejects unknown language", func(t *testing.T) {
		_, err := Normalize(map[string]map[string]string{
			"openai": {"cobol": "DISPLAY 'HI'"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported language")
	})

	t.Run("rejects oversized sample", func(t *testing.T) {
		_, err := Normalize(map[string]map[string]string{
			"openai": {"curl": strings.Repeat("x", MaxTemplateBytes+1)},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds")
	})

	t.Run("accepts sample at the size limit", func(t *testing.T) {
		atLimit := strings.Repeat("x", MaxTemplateBytes)
		normalized, err := Normalize(map[string]map[string]string{
			"openai": {"curl": atLimit},
		})
		require.NoError(t, err)
		assert.Equal(t, atLimit, normalized["openai"]["curl"])
	})

	t.Run("drops blank samples and empties", func(t *testing.T) {
		normalized, err := Normalize(map[string]map[string]string{
			"openai":    {"curl": "   ", "python": "from openai import OpenAI"},
			"anthropic": {"curl": ""},
		})
		require.NoError(t, err)
		assert.Equal(t, map[string]map[string]string{
			"openai": {"python": "from openai import OpenAI"},
		}, normalized)
	})

	t.Run("accepts every supported endpoint type and language", func(t *testing.T) {
		endpointTypes := []string{
			"openai", "openai-response", "openai-response-compact", "anthropic",
			"gemini", "jina-rerank", "image-generation", "embeddings", "openai-video",
		}
		input := make(map[string]map[string]string, len(endpointTypes))
		for _, endpointType := range endpointTypes {
			byLang := make(map[string]string, len(SupportedLanguages))
			for _, lang := range SupportedLanguages {
				byLang[lang] = "sample"
			}
			input[endpointType] = byLang
		}

		normalized, err := Normalize(input)
		require.NoError(t, err)
		assert.Len(t, normalized, len(endpointTypes))
		for _, endpointType := range endpointTypes {
			assert.Len(t, normalized[endpointType], len(SupportedLanguages))
		}
	})
}

func TestGetTemplatesReturnsDeepCopy(t *testing.T) {
	original := codeSampleSetting.Templates
	t.Cleanup(func() { codeSampleSetting.Templates = original })

	codeSampleSetting.Templates = map[string]map[string]string{
		"openai": {"curl": "curl https://example.test"},
	}

	snapshot := GetTemplates()
	snapshot["openai"]["curl"] = "mutated"
	snapshot["anthropic"] = map[string]string{"curl": "added"}

	assert.Equal(t, "curl https://example.test", codeSampleSetting.Templates["openai"]["curl"])
	assert.NotContains(t, codeSampleSetting.Templates, "anthropic")
}

func TestGetTemplate(t *testing.T) {
	original := codeSampleSetting.Templates
	t.Cleanup(func() { codeSampleSetting.Templates = original })

	codeSampleSetting.Templates = map[string]map[string]string{
		"openai": {"curl": "curl https://example.test", "python": "   "},
	}

	code, ok := GetTemplate("openai", "curl")
	assert.True(t, ok)
	assert.Equal(t, "curl https://example.test", code)

	_, ok = GetTemplate("openai", "python")
	assert.False(t, ok, "whitespace-only template should not count as configured")

	_, ok = GetTemplate("openai", "typescript")
	assert.False(t, ok)

	_, ok = GetTemplate("gemini", "curl")
	assert.False(t, ok)
}
