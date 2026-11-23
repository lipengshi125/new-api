package taskcommon

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// UnmarshalMetadata converts a map[string]any metadata to a typed struct via JSON round-trip.
// This replaces the repeated pattern: json.Marshal(metadata) → json.Unmarshal(bytes, &target).
func UnmarshalMetadata(metadata map[string]any, target any) error {
	if metadata == nil {
		return nil
	}
	// Prevent metadata from overriding model fields to avoid billing bypass.
	delete(metadata, "model")
	metaBytes, err := common.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata failed: %w", err)
	}
	if err := common.Unmarshal(metaBytes, target); err != nil {
		return fmt.Errorf("unmarshal metadata failed: %w", err)
	}
	return nil
}

// DefaultString returns val if non-empty, otherwise fallback.
func DefaultString(val, fallback string) string {
	if val == "" {
		return fallback
	}
	return val
}

// DefaultInt returns val if non-zero, otherwise fallback.
func DefaultInt(val, fallback int) int {
	if val == 0 {
		return fallback
	}
	return val
}

// EncodeLocalTaskID encodes an upstream operation name to a URL-safe base64 string.
// Used by Gemini/Vertex to store upstream names as task IDs.
func EncodeLocalTaskID(name string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(name))
}

// DecodeLocalTaskID decodes a base64-encoded upstream operation name.
func DecodeLocalTaskID(id string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// BuildProxyURL constructs the video proxy URL using the public task ID.
// e.g., "https://your-server.com/v1/videos/task_xxxx/content"
func BuildProxyURL(taskID string) string {
	return fmt.Sprintf("%s/v1/videos/%s/content", system_setting.ServerAddress, taskID)
}

// IsOwnProxyURL reports whether u points back at this service: the task's own
// /content proxy path, or any URL on this server's host. The content proxy
// must never fetch such a URL, because the request would loop to itself.
// Content URLs on other hosts (e.g. an upstream relay's public /content) are
// legitimate fetch targets and are not matched.
func IsOwnProxyURL(u, taskID string) bool {
	u = strings.TrimSpace(u)
	if u == "" {
		return false
	}
	if taskID != "" && strings.Contains(u, "/"+taskID+"/content") {
		return true
	}
	target, err := url.Parse(u)
	if err != nil || target.Host == "" {
		return false
	}
	self, err := url.Parse(system_setting.ServerAddress)
	if err != nil || self.Host == "" {
		return false
	}
	return strings.EqualFold(target.Host, self.Host)
}

// PublicVideoResult rewrites a stored upstream task payload before it is shown
// to the caller: the public task id and the requested model replace upstream
// values, every upstream result URL is removed, and metadata.url is set to this
// service's content proxy only once the task has succeeded.
func PublicVideoResult(data []byte, task *model.Task) ([]byte, error) {
	var err error
	if data, err = sjson.SetBytes(data, "id", task.TaskID); err != nil {
		return nil, fmt.Errorf("set id failed: %w", err)
	}
	if data, err = sjson.SetBytes(data, "task_id", task.TaskID); err != nil {
		return nil, fmt.Errorf("set task_id failed: %w", err)
	}
	if data, err = sjson.SetBytes(data, "status", task.Status.ToVideoStatus()); err != nil {
		return nil, fmt.Errorf("set status failed: %w", err)
	}
	if originModel := task.Properties.OriginModelName; originModel != "" {
		if data, err = sjson.SetBytes(data, "model", originModel); err != nil {
			return nil, fmt.Errorf("set model failed: %w", err)
		}
	}

	for _, path := range []string{"metadata.url", "video_url", "image_url", "url", "data.url"} {
		if gjson.GetBytes(data, path).Exists() {
			if data, err = sjson.DeleteBytes(data, path); err != nil {
				return nil, fmt.Errorf("remove %s failed: %w", path, err)
			}
		}
	}
	if items := gjson.GetBytes(data, "data"); items.IsArray() {
		for i := range items.Array() {
			path := fmt.Sprintf("data.%d.url", i)
			if gjson.GetBytes(data, path).Exists() {
				if data, err = sjson.DeleteBytes(data, path); err != nil {
					return nil, fmt.Errorf("remove %s failed: %w", path, err)
				}
			}
		}
	}

	if task.Status == model.TaskStatusSuccess {
		if data, err = sjson.SetBytes(data, "metadata.url", BuildProxyURL(task.TaskID)); err != nil {
			return nil, fmt.Errorf("set metadata.url failed: %w", err)
		}
	}
	return data, nil
}

// Status-to-progress mapping constants for polling updates.
const (
	ProgressSubmitted  = "10%"
	ProgressQueued     = "20%"
	ProgressInProgress = "30%"
	ProgressComplete   = "100%"
)

// ---------------------------------------------------------------------------
// BaseBilling — embeddable no-op implementations for TaskAdaptor billing methods.
// Adaptors that do not need custom billing can embed this struct directly.
// ---------------------------------------------------------------------------

type BaseBilling struct{}

// EstimateBilling returns nil (no extra ratios; use base model price).
func (BaseBilling) EstimateBilling(_ *gin.Context, _ *relaycommon.RelayInfo) map[string]float64 {
	return nil
}

// AdjustBillingOnSubmit returns nil (no submit-time adjustment).
func (BaseBilling) AdjustBillingOnSubmit(_ *relaycommon.RelayInfo, _ []byte) map[string]float64 {
	return nil
}

// AdjustBillingOnComplete returns 0 (keep pre-charged amount).
func (BaseBilling) AdjustBillingOnComplete(_ *model.Task, _ *relaycommon.TaskInfo) int {
	return 0
}
