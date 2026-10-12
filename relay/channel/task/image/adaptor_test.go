package image

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseTaskResultStatusMapping verifies upstream status strings map to the
// polling loop's task states. The critical contract is that an unrecognized or
// empty status must stay IN_PROGRESS: treating it as failure would refund and
// abandon a task that is still running upstream.
func TestParseTaskResultStatusMapping(t *testing.T) {
	adaptor := &TaskAdaptor{}

	tests := []struct {
		name       string
		body       string
		wantStatus model.TaskStatus
		wantURL    string
		wantReason string
	}{
		{
			name:       "queued",
			body:       `{"id":"x","status":"queued"}`,
			wantStatus: model.TaskStatusQueued,
		},
		{
			name:       "pending maps to queued",
			body:       `{"id":"x","status":"pending"}`,
			wantStatus: model.TaskStatusQueued,
		},
		{
			name:       "processing",
			body:       `{"id":"x","status":"processing"}`,
			wantStatus: model.TaskStatusInProgress,
		},
		{
			name:       "completed exposes first url",
			body:       `{"id":"x","status":"completed","data":[{"url":"https://cdn.example.com/1.png"}]}`,
			wantStatus: model.TaskStatusSuccess,
			wantURL:    "https://cdn.example.com/1.png",
		},
		{
			name:       "completed exposes metadata url",
			body:       `{"id":"x","status":"completed","data":{"duration":6750000},"metadata":{"url":"https://cdn.example.com/1.jpg"}}`,
			wantStatus: model.TaskStatusSuccess,
			wantURL:    "https://cdn.example.com/1.jpg",
		},
		{
			name:       "succeeded alias",
			body:       `{"id":"x","status":"succeeded","data":[{"b64_json":"aGk="}]}`,
			wantStatus: model.TaskStatusSuccess,
		},
		{
			name:       "failed carries upstream message",
			body:       `{"id":"x","status":"failed","error":{"message":"content policy","code":"rejected"}}`,
			wantStatus: model.TaskStatusFailure,
			wantReason: "content policy",
		},
		{
			name:       "failed without message uses fallback",
			body:       `{"id":"x","status":"failed"}`,
			wantStatus: model.TaskStatusFailure,
			wantReason: "task failed",
		},
		{
			name:       "unknown status keeps polling",
			body:       `{"id":"x","status":"materializing"}`,
			wantStatus: model.TaskStatusInProgress,
		},
		{
			name:       "absent status keeps polling",
			body:       `{"id":"x"}`,
			wantStatus: model.TaskStatusInProgress,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := adaptor.ParseTaskResult([]byte(tt.body))
			require.NoError(t, err)
			assert.Equal(t, string(tt.wantStatus), info.Status)
			assert.Equal(t, tt.wantURL, info.Url)
			assert.Equal(t, tt.wantReason, info.Reason)
		})
	}
}

// TestConvertToOpenAIVideo verifies a stored image task is returned as the
// upstream payload (same passthrough as /v1/videos/{id}) with the public id,
// and that metadata.url is the real image URL, never the local content proxy.
func TestConvertToOpenAIVideo(t *testing.T) {
	adaptor := &TaskAdaptor{}

	t.Run("success passes upstream payload through with public id", func(t *testing.T) {
		task := &model.Task{
			TaskID:     "task_public123",
			Status:     model.TaskStatusSuccess,
			FinishTime: 1200,
			Data: json.RawMessage(`{"id":"upstream_secret","task_id":"upstream_secret","object":"video","model":"banana-2",` +
				`"status":"completed","progress":100,"created_at":1000,"completed_at":1200,"seconds":"5",` +
				`"data":{"duration":6750000},"metadata":{"url":"https://cdn.example.com/a.jpg"}}`),
		}
		task.PrivateData.ResultURL = "https://cdn.example.com/a.jpg"

		raw, err := adaptor.ConvertToOpenAIVideo(task)
		require.NoError(t, err)

		var got map[string]any
		require.NoError(t, common.Unmarshal(raw, &got))
		assert.NotContains(t, string(raw), "upstream_secret")
		assert.Equal(t, "task_public123", got["id"])
		assert.Equal(t, "task_public123", got["task_id"])
		assert.Equal(t, "banana-2", got["model"])
		assert.Equal(t, dto.VideoStatusCompleted, got["status"])
		assert.Equal(t, "5", got["seconds"])
		assert.Equal(t, map[string]any{"duration": float64(6750000)}, got["data"])
		assert.Equal(t, map[string]any{"url": "https://cdn.example.com/a.jpg"}, got["metadata"])
	})

	t.Run("openai image list fills metadata url", func(t *testing.T) {
		task := &model.Task{
			TaskID: "task_list",
			Status: model.TaskStatusSuccess,
			Data:   json.RawMessage(`{"status":"completed","data":[{"url":"https://cdn.example.com/a.png"},{"url":"https://cdn.example.com/b.png"}]}`),
		}

		raw, err := adaptor.ConvertToOpenAIVideo(task)
		require.NoError(t, err)

		var got dto.OpenAIVideo
		require.NoError(t, common.Unmarshal(raw, &got))
		assert.Equal(t, "https://cdn.example.com/a.png", got.Metadata["url"])
	})

	t.Run("local content proxy url is never exposed", func(t *testing.T) {
		task := &model.Task{
			TaskID: "task_proxy",
			Status: model.TaskStatusSuccess,
			Data:   json.RawMessage(`{"status":"completed"}`),
		}
		task.PrivateData.ResultURL = taskcommon.BuildProxyURL("task_proxy")

		raw, err := adaptor.ConvertToOpenAIVideo(task)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), "/content")
	})

	t.Run("success falls back to stored result url", func(t *testing.T) {
		task := &model.Task{
			TaskID: "task_fallback",
			Status: model.TaskStatusSuccess,
			Data:   json.RawMessage(`not-json`),
		}
		task.PrivateData.ResultURL = "https://cdn.example.com/fallback.png"

		raw, err := adaptor.ConvertToOpenAIVideo(task)
		require.NoError(t, err)

		var got dto.OpenAIVideo
		require.NoError(t, common.Unmarshal(raw, &got))
		assert.Equal(t, "https://cdn.example.com/fallback.png", got.Metadata["url"])
	})

	t.Run("failure reports error", func(t *testing.T) {
		task := &model.Task{
			TaskID:     "task_failed",
			Status:     model.TaskStatusFailure,
			FailReason: "upstream rejected the prompt",
			FinishTime: 1300,
		}

		raw, err := adaptor.ConvertToOpenAIVideo(task)
		require.NoError(t, err)

		var got dto.OpenAIVideo
		require.NoError(t, common.Unmarshal(raw, &got))
		assert.Equal(t, dto.VideoStatusFailed, got.Status)
		require.NotNil(t, got.Error)
		assert.Equal(t, "upstream rejected the prompt", got.Error.Message)
	})

	t.Run("in-flight task reports progress and no result", func(t *testing.T) {
		task := &model.Task{
			TaskID:   "task_running",
			Status:   model.TaskStatusInProgress,
			Progress: "30%",
		}

		raw, err := adaptor.ConvertToOpenAIVideo(task)
		require.NoError(t, err)

		var got dto.OpenAIVideo
		require.NoError(t, common.Unmarshal(raw, &got))
		assert.Equal(t, dto.VideoStatusInProgress, got.Status)
		assert.Equal(t, 30, got.Progress)
		assert.Empty(t, got.Metadata["url"])
		assert.Nil(t, got.Error)
	})
}

// TestResultURL pins the URL the content proxy fetches for an image task: the
// real upstream image URL, never the local content proxy, which would loop back
// to this service (and to an upstream /v1/videos/{id}/content that 404s).
func TestResultURL(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		resultURL string
		want      string
	}{
		{"metadata url wins", `{"metadata":{"url":"https://cdn.example.com/a.jpg"}}`, "https://other.example.com/b.jpg", "https://cdn.example.com/a.jpg"},
		{"openai image list", `{"data":[{"url":"https://cdn.example.com/a.png"}]}`, "", "https://cdn.example.com/a.png"},
		{"stored result url fallback", `{"status":"completed"}`, "https://cdn.example.com/c.png", "https://cdn.example.com/c.png"},
		{"local video proxy is rejected", `{"status":"completed"}`, "https://api.example.com/v1/videos/task_x/content", ""},
		{"local image proxy is rejected", `{"status":"completed"}`, "https://api.example.com/v1/images/task_x/content", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &model.Task{TaskID: "task_x", Data: json.RawMessage(tt.data)}
			task.PrivateData.ResultURL = tt.resultURL
			assert.Equal(t, tt.want, ResultURL(task))
		})
	}
}
