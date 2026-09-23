package image

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
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

// TestConvertToOpenAIImageTask verifies GET /v1/images/{id} reports the public
// task id (never the upstream id) and surfaces the stored image list.
func TestConvertToOpenAIImageTask(t *testing.T) {
	adaptor := &TaskAdaptor{}

	t.Run("success exposes public id and images", func(t *testing.T) {
		task := &model.Task{
			TaskID:     "task_public123",
			Status:     model.TaskStatusSuccess,
			Progress:   "100%",
			SubmitTime: 1000,
			FinishTime: 1200,
			Data:       json.RawMessage(`{"id":"upstream_secret","size":"1024x1024","data":[{"url":"https://cdn.example.com/a.png","revised_prompt":"a green cat"},{"b64_json":"aGk="}]}`),
		}
		task.Properties.OriginModelName = "image-2.5-sunburst"

		raw, err := adaptor.ConvertToOpenAIImageTask(task)
		require.NoError(t, err)

		var got dto.OpenAIImageTask
		require.NoError(t, common.Unmarshal(raw, &got))

		assert.Equal(t, "task_public123", got.ID)
		assert.NotContains(t, string(raw), "upstream_secret")
		assert.Equal(t, "image-2.5-sunburst", got.Model)
		assert.Equal(t, dto.ImageTaskStatusCompleted, got.Status)
		assert.Equal(t, "1024x1024", got.Size)
		assert.Equal(t, int64(1200), got.CompletedAt)
		assert.Equal(t, 100, got.Progress)
		require.Len(t, got.Data, 2)
		assert.Equal(t, "https://cdn.example.com/a.png", got.Data[0].URL)
		assert.Equal(t, "a green cat", got.Data[0].RevisedPrompt)
		assert.Equal(t, "aGk=", got.Data[1].B64JSON)
	})

	t.Run("success falls back to stored result url", func(t *testing.T) {
		task := &model.Task{
			TaskID: "task_fallback",
			Status: model.TaskStatusSuccess,
			Data:   json.RawMessage(`not-json`),
		}
		task.PrivateData.ResultURL = "https://cdn.example.com/fallback.png"

		raw, err := adaptor.ConvertToOpenAIImageTask(task)
		require.NoError(t, err)

		var got dto.OpenAIImageTask
		require.NoError(t, common.Unmarshal(raw, &got))
		require.Len(t, got.Data, 1)
		assert.Equal(t, "https://cdn.example.com/fallback.png", got.Data[0].URL)
	})

	t.Run("failure reports error and no data", func(t *testing.T) {
		task := &model.Task{
			TaskID:     "task_failed",
			Status:     model.TaskStatusFailure,
			FailReason: "upstream rejected the prompt",
			FinishTime: 1300,
		}

		raw, err := adaptor.ConvertToOpenAIImageTask(task)
		require.NoError(t, err)

		var got dto.OpenAIImageTask
		require.NoError(t, common.Unmarshal(raw, &got))
		assert.Equal(t, dto.ImageTaskStatusFailed, got.Status)
		assert.Empty(t, got.Data)
		require.NotNil(t, got.Error)
		assert.Equal(t, "upstream rejected the prompt", got.Error.Message)
	})

	t.Run("in-flight task reports no data", func(t *testing.T) {
		task := &model.Task{
			TaskID:   "task_running",
			Status:   model.TaskStatusInProgress,
			Progress: "30%",
		}

		raw, err := adaptor.ConvertToOpenAIImageTask(task)
		require.NoError(t, err)

		var got dto.OpenAIImageTask
		require.NoError(t, common.Unmarshal(raw, &got))
		assert.Equal(t, dto.ImageTaskStatusInProgress, got.Status)
		assert.Equal(t, 30, got.Progress)
		assert.Empty(t, got.Data)
		assert.Nil(t, got.Error)
	})
}
