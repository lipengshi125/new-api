package code_sample_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/config"
)

// MaxTemplateBytes bounds a single stored sample. Samples are admin-authored
// snippets shown on the model pricing page, so a few kilobytes is generous;
// the bound keeps the options row (and the public pricing response) from being
// used as bulk storage.
const MaxTemplateBytes = 8 * 1024

// SupportedLanguages lists the sample languages the pricing page renders. The
// frontend tab set is derived from the same list, so both sides stay aligned.
var SupportedLanguages = []string{"curl", "python", "typescript", "javascript"}

// CodeSampleSetting holds the site-wide call-sample templates, keyed by
// endpoint type and then by language. A model may override any entry through
// its own models.code_samples column; anything left unset falls back to the
// built-in samples compiled into the frontend.
type CodeSampleSetting struct {
	Templates map[string]map[string]string `json:"templates"`
}

var codeSampleSetting = CodeSampleSetting{
	Templates: map[string]map[string]string{},
}

func init() {
	config.GlobalConfig.Register("code_sample_setting", &codeSampleSetting)
}

// IsSupportedEndpointType reports whether the endpoint type is one the pricing
// page can render samples for.
func IsSupportedEndpointType(endpointType string) bool {
	switch constant.EndpointType(endpointType) {
	case constant.EndpointTypeOpenAI,
		constant.EndpointTypeOpenAIResponse,
		constant.EndpointTypeOpenAIResponseCompact,
		constant.EndpointTypeAnthropic,
		constant.EndpointTypeGemini,
		constant.EndpointTypeJinaRerank,
		constant.EndpointTypeImageGeneration,
		constant.EndpointTypeEmbeddings,
		constant.EndpointTypeOpenAIVideo:
		return true
	default:
		return false
	}
}

// IsSupportedLanguage reports whether the language is one of the sample tabs.
func IsSupportedLanguage(lang string) bool {
	for _, supported := range SupportedLanguages {
		if lang == supported {
			return true
		}
	}
	return false
}

// Normalize drops unknown endpoint types and languages, trims whitespace-only
// samples, and rejects oversized entries. It returns a cleaned copy plus the
// first validation error encountered, so callers can reject a bad payload
// outright rather than silently storing part of it.
func Normalize(templates map[string]map[string]string) (map[string]map[string]string, error) {
	normalized := make(map[string]map[string]string, len(templates))
	for endpointType, byLang := range templates {
		if !IsSupportedEndpointType(endpointType) {
			return nil, &ValidationError{Reason: ReasonUnknownEndpointType, EndpointType: endpointType}
		}
		cleaned := make(map[string]string, len(byLang))
		for lang, code := range byLang {
			if !IsSupportedLanguage(lang) {
				return nil, &ValidationError{Reason: ReasonUnknownLanguage, EndpointType: endpointType, Language: lang}
			}
			if len(code) > MaxTemplateBytes {
				return nil, &ValidationError{Reason: ReasonTooLarge, EndpointType: endpointType, Language: lang}
			}
			if strings.TrimSpace(code) == "" {
				continue
			}
			cleaned[lang] = code
		}
		if len(cleaned) > 0 {
			normalized[endpointType] = cleaned
		}
	}
	return normalized, nil
}

// GetTemplates returns a deep copy so callers cannot mutate the live setting.
func GetTemplates() map[string]map[string]string {
	result := make(map[string]map[string]string, len(codeSampleSetting.Templates))
	for endpointType, byLang := range codeSampleSetting.Templates {
		cloned := make(map[string]string, len(byLang))
		for lang, code := range byLang {
			cloned[lang] = code
		}
		result[endpointType] = cloned
	}
	return result
}

// GetTemplate returns the configured sample for an endpoint type and language.
func GetTemplate(endpointType, lang string) (string, bool) {
	byLang, ok := codeSampleSetting.Templates[endpointType]
	if !ok {
		return "", false
	}
	code, ok := byLang[lang]
	if !ok || strings.TrimSpace(code) == "" {
		return "", false
	}
	return code, true
}
