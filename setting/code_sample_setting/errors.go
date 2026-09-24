package code_sample_setting

import "fmt"

// Validation failure reasons for code-sample payloads.
const (
	ReasonUnknownEndpointType = "unknown_endpoint_type"
	ReasonUnknownLanguage     = "unknown_language"
	ReasonTooLarge            = "too_large"
	ReasonMissingModelName    = "missing_model_name"
	ReasonTooManyModels       = "too_many_models"
)

// ValidationError describes why a code-sample payload was rejected. Callers
// surface it to the admin UI so the offending entry is identifiable.
type ValidationError struct {
	Reason       string
	EndpointType string
	Language     string
}

func (e *ValidationError) Error() string {
	switch e.Reason {
	case ReasonUnknownEndpointType:
		return fmt.Sprintf("unsupported endpoint type: %s", e.EndpointType)
	case ReasonUnknownLanguage:
		return fmt.Sprintf("unsupported language %q for endpoint type %s", e.Language, e.EndpointType)
	case ReasonTooLarge:
		return fmt.Sprintf("code sample for %s/%s exceeds %d bytes", e.EndpointType, e.Language, MaxTemplateBytes)
	case ReasonMissingModelName:
		return "模型名称不能为空"
	case ReasonTooManyModels:
		return fmt.Sprintf("已达到 %d 个模型的调用示例上限", MaxModelOverrides)
	default:
		return "invalid code sample payload"
	}
}
