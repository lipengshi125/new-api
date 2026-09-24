package code_sample_setting

import (
	"strings"
	"sync"

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

// CodeSampleSetting holds the call-sample overrides.
//
// Templates are site-wide, keyed by endpoint type and then by language.
// Models holds per-model overrides, keyed by model name, then endpoint type,
// then language. Per-model samples deliberately live here rather than on the
// models table: a sample is presentation, not model metadata, so editing one
// must never create or modify a 元信息 row. Anything left unset falls back to
// the templates and then to the built-in samples compiled into the frontend.
type CodeSampleSetting struct {
	Templates map[string]map[string]string            `json:"templates"`
	Models    map[string]map[string]map[string]string `json:"models"`
}

var codeSampleSetting = CodeSampleSetting{
	Templates: map[string]map[string]string{},
	Models:    map[string]map[string]map[string]string{},
}

// settingLock guards codeSampleSetting for readers. The ConfigManager writes it
// via reflection on option updates, so reads must not assume a stable map.
var settingLock sync.RWMutex

// MaxModelOverrides bounds how many models can carry their own samples. The
// whole set is serialized into a single options row and returned on the public
// pricing response, so it must not grow without limit.
const MaxModelOverrides = 500

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

func cloneByEndpoint(src map[string]map[string]string) map[string]map[string]string {
	result := make(map[string]map[string]string, len(src))
	for endpointType, byLang := range src {
		cloned := make(map[string]string, len(byLang))
		for lang, code := range byLang {
			cloned[lang] = code
		}
		result[endpointType] = cloned
	}
	return result
}

// GetTemplates returns a deep copy so callers cannot mutate the live setting.
func GetTemplates() map[string]map[string]string {
	settingLock.RLock()
	defer settingLock.RUnlock()
	return cloneByEndpoint(codeSampleSetting.Templates)
}

// GetTemplate returns the configured sample for an endpoint type and language.
func GetTemplate(endpointType, lang string) (string, bool) {
	settingLock.RLock()
	defer settingLock.RUnlock()
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

// GetModelSamples returns one model's overrides, or nil when it has none.
func GetModelSamples(modelName string) map[string]map[string]string {
	settingLock.RLock()
	defer settingLock.RUnlock()
	byEndpoint, ok := codeSampleSetting.Models[strings.TrimSpace(modelName)]
	if !ok || len(byEndpoint) == 0 {
		return nil
	}
	return cloneByEndpoint(byEndpoint)
}

// GetAllModelSamples returns a deep copy of every per-model override.
func GetAllModelSamples() map[string]map[string]map[string]string {
	settingLock.RLock()
	defer settingLock.RUnlock()
	result := make(map[string]map[string]map[string]string, len(codeSampleSetting.Models))
	for modelName, byEndpoint := range codeSampleSetting.Models {
		result[modelName] = cloneByEndpoint(byEndpoint)
	}
	return result
}

// BuildModelSamplesUpdate returns the full per-model override map with
// modelName's entry replaced by samples, ready to be persisted as one options
// row. Passing empty samples removes that model's entry entirely. The caller
// persists the result; this function does not mutate the live setting.
func BuildModelSamplesUpdate(modelName string, samples map[string]map[string]string) (map[string]map[string]map[string]string, error) {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return nil, &ValidationError{Reason: ReasonMissingModelName}
	}

	normalized, err := Normalize(samples)
	if err != nil {
		return nil, err
	}

	next := GetAllModelSamples()
	if len(normalized) == 0 {
		delete(next, modelName)
		return next, nil
	}
	if _, exists := next[modelName]; !exists && len(next) >= MaxModelOverrides {
		return nil, &ValidationError{Reason: ReasonTooManyModels}
	}
	next[modelName] = normalized
	return next, nil
}
