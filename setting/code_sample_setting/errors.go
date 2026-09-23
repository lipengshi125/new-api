package code_sample_setting

import "fmt"

// Validation failure reasons for code-sample payloads.
const (
	ReasonUnknownEndpointType = "unknown_endpoint_type"
	ReasonUnknownLanguage     = "unknown_language"
	ReasonTooLarge            = "too_large"
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
	default:
		return "invalid code sample payload"
	}
}
