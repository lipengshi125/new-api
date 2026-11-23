package taskcommon

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// upstreamRelayPayload is what an upstream relay returns for a task: its own
// task id, its model name and result URLs on its own hosts.
const upstreamRelayPayload = `{"id":"ob_upstream","task_id":"ob_upstream","model":"image-2.5-sunburst",` +
	`"status":"completed","progress":100,"video_url":"https://hk.relay.example/v1/videos/vid-1/content",` +
	`"data":[{"url":"https://hk.relay.example/v1/videos/vid-1/content","revised_prompt":"p"}],` +
	`"metadata":{"url":"https://cdn.upstream.example/a.png"}}`

// TestPublicVideoResult pins what a caller sees for a task: never an upstream
// id, model or URL, and the local content URL only once the task succeeded.
func TestPublicVideoResult(t *testing.T) {
	system_setting.ServerAddress = "https://www.mine.example"
	tests := []struct {
		name    string
		status  model.TaskStatus
		wantURL string
	}{
		{"queued task exposes no url", model.TaskStatusQueued, ""},
		{"in-progress task exposes no url", model.TaskStatusInProgress, ""},
		{"failed task exposes no url", model.TaskStatusFailure, ""},
		{"succeeded task exposes local content url", model.TaskStatusSuccess, "https://www.mine.example/v1/videos/task_pub/content"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &model.Task{TaskID: "task_pub", Status: tt.status}
			task.Properties.OriginModelName = "gpt-image-2.5"

			raw, err := PublicVideoResult(json.RawMessage(upstreamRelayPayload), task)
			require.NoError(t, err)

			body := string(raw)
			assert.NotContains(t, body, "ob_upstream")
			assert.NotContains(t, body, "image-2.5-sunburst")
			assert.NotContains(t, body, "hk.relay.example")
			assert.NotContains(t, body, "cdn.upstream.example")
			assert.Equal(t, "task_pub", gjson.Get(body, "id").String())
			assert.Equal(t, "task_pub", gjson.Get(body, "task_id").String())
			assert.Equal(t, "gpt-image-2.5", gjson.Get(body, "model").String())
			assert.Equal(t, "p", gjson.Get(body, "data.0.revised_prompt").String())
			assert.Equal(t, tt.wantURL, gjson.Get(body, "metadata.url").String())
		})
	}
}

// TestIsOwnProxyURL pins which URLs the content proxy refuses to fetch: only
// ones that would request this service itself.
func TestIsOwnProxyURL(t *testing.T) {
	system_setting.ServerAddress = "https://www.mine.example"
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"own content url for this task", "https://www.mine.example/v1/videos/task_pub/content", true},
		{"this task content path on any host", "http://localhost:3000/v1/images/task_pub/content", true},
		{"other url on own host", "https://WWW.mine.example/v1/images/task_other/content", true},
		{"upstream relay public content url", "https://hk.relay.example/v1/videos/vid-1/content", false},
		{"upstream cdn url", "https://cdn.upstream.example/a.png", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsOwnProxyURL(tt.url, "task_pub"))
		})
	}
}
