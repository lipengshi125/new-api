// Package image implements the async image-task adaptor backing
// POST /v1/images and GET /v1/images/{task_id}.
//
// The upstream contract mirrors the async video flow: a submit call returns a
// task id, and a fetch call reports status until the task reaches a terminal
// state. Synchronous image endpoints (/v1/images/generations, /v1/images/edits)
// are unrelated and keep their existing non-task relay path.
package image

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// submitResponse models the upstream submit/fetch payload. Upstreams differ on
// where the id lives, so several spellings are accepted.
type submitResponse struct {
	ID     string `json:"id"`
	TaskID string `json:"task_id,omitempty"`
	Object string `json:"object,omitempty"`
	Model  string `json:"model,omitempty"`
	Status string `json:"status,omitempty"`

	Progress  int     `json:"progress,omitempty"`
	CreatedAt int64   `json:"created_at,omitempty"`
	Size      string  `json:"size,omitempty"`
	Data      []datum `json:"data,omitempty"`

	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

type datum struct {
	URL           string `json:"url,omitempty"`
	B64JSON       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	return relaycommon.ValidateImageTaskRequest(c, info)
}

// EstimateBilling charges per generated image: n images cost n times the base
// price. ValidateImageTaskRequest has already bounded n by dto.MaxImageN.
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil || req.N == nil || *req.N <= 1 {
		return nil
	}
	return map[string]float64{"n": float64(*req.N)}
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/v1/images", a.baseURL), nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	return nil
}

// BuildRequestBody forwards the client body with the model replaced by the
// mapped upstream name.
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, errors.Wrap(err, "get_request_body_failed")
	}
	cachedBody, err := storage.Bytes()
	if err != nil {
		return nil, errors.Wrap(err, "read_body_bytes_failed")
	}

	var bodyMap map[string]any
	if err := common.Unmarshal(cachedBody, &bodyMap); err != nil {
		return bytes.NewReader(cachedBody), nil
	}
	bodyMap["model"] = info.UpstreamModelName
	newBody, err := common.Marshal(bodyMap)
	if err != nil {
		return bytes.NewReader(cachedBody), nil
	}
	return bytes.NewReader(newBody), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse parses the submit response and echoes it to the client with the
// upstream id swapped for our public task id.
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	var parsed submitResponse
	if err := common.Unmarshal(responseBody, &parsed); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	upstreamID := parsed.ID
	if upstreamID == "" {
		upstreamID = parsed.TaskID
	}
	if upstreamID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task id is empty in upstream response"), "invalid_response", http.StatusInternalServerError)
		return
	}

	out := dto.NewOpenAIImageTask()
	out.ID = info.PublicTaskID
	out.Model = info.OriginModelName
	out.Progress = parsed.Progress
	out.CreatedAt = parsed.CreatedAt
	out.Size = parsed.Size
	if status := mapUpstreamStatus(parsed.Status); status != "" {
		out.Status = status
	}
	c.JSON(http.StatusOK, out)

	return upstreamID, responseBody, nil
}

// FetchTask queries upstream task status. The polling loop supplies task_id.
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("invalid task_id")
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/v1/images/%s", baseUrl, taskID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string { return ModelList }

func (a *TaskAdaptor) GetChannelName() string { return ChannelName }

// ParseTaskResult maps the upstream fetch payload onto the polling loop's
// TaskInfo. An unrecognized or absent status keeps the task in progress rather
// than failing it, so a premature poll does not refund a running task.
func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var parsed submitResponse
	if err := common.Unmarshal(respBody, &parsed); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	info := &relaycommon.TaskInfo{Code: 0}
	switch mapUpstreamStatus(parsed.Status) {
	case dto.ImageTaskStatusQueued:
		info.Status = model.TaskStatusQueued
	case dto.ImageTaskStatusCompleted:
		info.Status = model.TaskStatusSuccess
		info.Url = firstImageURL(parsed.Data)
	case dto.ImageTaskStatusFailed:
		info.Status = model.TaskStatusFailure
		if parsed.Error != nil && parsed.Error.Message != "" {
			info.Reason = parsed.Error.Message
		} else {
			info.Reason = "task failed"
		}
	default:
		info.Status = model.TaskStatusInProgress
	}

	if parsed.Progress > 0 && parsed.Progress < 100 {
		info.Progress = fmt.Sprintf("%d%%", parsed.Progress)
	}
	return info, nil
}

// ConvertToOpenAIImageTask renders a stored task for GET /v1/images/{id}.
func (a *TaskAdaptor) ConvertToOpenAIImageTask(task *model.Task) ([]byte, error) {
	out := dto.NewOpenAIImageTask()
	out.ID = task.TaskID
	out.Model = task.Properties.OriginModelName
	out.Status = task.Status.ToImageTaskStatus()
	out.CreatedAt = task.SubmitTime
	out.Progress = parseProgress(task.Progress)

	if task.Status == model.TaskStatusSuccess {
		out.CompletedAt = task.FinishTime
		// Prefer the image list the upstream reported; fall back to the single
		// stored result URL when the payload is unavailable or unparseable.
		var stored submitResponse
		if err := common.Unmarshal(task.Data, &stored); err == nil && len(stored.Data) > 0 {
			out.Size = stored.Size
			for _, d := range stored.Data {
				out.Data = append(out.Data, dto.OpenAIImageTaskDatum{
					URL:           d.URL,
					B64JSON:       d.B64JSON,
					RevisedPrompt: d.RevisedPrompt,
				})
			}
		} else if url := task.GetResultURL(); url != "" {
			out.Data = []dto.OpenAIImageTaskDatum{{URL: url}}
		}
	}

	if task.Status == model.TaskStatusFailure {
		out.CompletedAt = task.FinishTime
		out.Error = &dto.OpenAIImageTaskError{
			Message: task.FailReason,
			Code:    "task_failed",
		}
	}

	return common.Marshal(out)
}

// mapUpstreamStatus normalizes the status spellings seen across upstreams.
// An empty result means "unrecognized" and callers keep polling.
func mapUpstreamStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued", "pending", "not_start", "submitted":
		return dto.ImageTaskStatusQueued
	case "processing", "in_progress", "running":
		return dto.ImageTaskStatusInProgress
	case "completed", "succeeded", "success":
		return dto.ImageTaskStatusCompleted
	case "failed", "failure", "cancelled", "canceled", "error":
		return dto.ImageTaskStatusFailed
	}
	return ""
}

func firstImageURL(data []datum) string {
	for _, d := range data {
		if d.URL != "" {
			return d.URL
		}
	}
	return ""
}

func parseProgress(progress string) int {
	trimmed := strings.TrimSuffix(strings.TrimSpace(progress), "%")
	if trimmed == "" {
		return 0
	}
	var value int
	if _, err := fmt.Sscanf(trimmed, "%d", &value); err != nil {
		return 0
	}
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}
