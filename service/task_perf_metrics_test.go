package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// terminalFetchAdaptor answers every fetch with a fixed upstream status.
type terminalFetchAdaptor struct {
	status model.TaskStatus
}

func (a *terminalFetchAdaptor) Init(_ *relaycommon.RelayInfo) {}

func (a *terminalFetchAdaptor) FetchTask(_ string, _ string, body map[string]any, _ string) (*http.Response, error) {
	taskID, _ := body["task_id"].(string)
	responseBody, err := common.Marshal(dto.TaskResponse[model.Task]{
		Code: dto.TaskSuccessCode,
		Data: model.Task{TaskID: taskID, Status: a.status, Progress: "100%"},
	})
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(responseBody)),
	}, nil
}

func (a *terminalFetchAdaptor) ParseTaskResult([]byte) (*relaycommon.TaskInfo, error) {
	return &relaycommon.TaskInfo{Status: string(a.status)}, nil
}

func (a *terminalFetchAdaptor) AdjustBillingOnComplete(_ *model.Task, _ *relaycommon.TaskInfo) int {
	return 0
}

// taskSuccessRate reads the marketplace success rate and request count back
// through the public query API.
func taskSuccessRate(t *testing.T, modelName string) (rate float64, groups int) {
	t.Helper()
	// Query merges persisted buckets with in-memory ones, so the table must exist.
	require.NoError(t, model.DB.AutoMigrate(&model.PerfMetric{}))
	result, err := perfmetrics.Query(perfmetrics.QueryParams{Model: modelName, Hours: 1})
	require.NoError(t, err)
	if len(result.Groups) == 0 {
		return 0, 0
	}
	return result.Groups[0].SuccessRate, len(result.Groups)
}

// taskRequestCount sums the requests recorded for a model across its series.
func taskRequestCount(t *testing.T, modelName string) int64 {
	t.Helper()
	return perfmetrics.RequestCountForTest(modelName)
}

// Async tasks never reach the synchronous quota paths that feed the
// marketplace metrics, which left every async model at 0% success. The poller
// must record exactly one sample when it moves a task to a terminal state.
func TestPollingRecordsAsyncTaskPerformanceOnce(t *testing.T) {
	cases := []struct {
		name        string
		modelName   string
		status      model.TaskStatus
		wantSuccess float64
	}{
		{name: "success counts as a successful request", modelName: "perf-async-success", status: model.TaskStatusSuccess, wantSuccess: 100},
		{name: "failure counts as a failed request", modelName: "perf-async-failure", status: model.TaskStatusFailure, wantSuccess: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			truncate(t)
			const channelID = 7101
			seedChannel(t, channelID)
			ch, err := model.GetChannelById(channelID, true)
			require.NoError(t, err)

			task := &model.Task{
				TaskID:     "task_" + tc.modelName,
				UserId:     1,
				ChannelId:  channelID,
				Group:      "default",
				Status:     model.TaskStatusInProgress,
				Progress:   "30%",
				SubmitTime: time.Now().Unix() - 60,
				Properties: model.Properties{OriginModelName: tc.modelName},
				PrivateData: model.TaskPrivateData{
					UpstreamTaskID: "upstream_" + tc.modelName,
				},
			}
			require.NoError(t, model.DB.Create(task).Error)

			adaptor := &terminalFetchAdaptor{status: tc.status}
			taskM := map[string]*model.Task{task.PrivateData.UpstreamTaskID: task}
			require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, ch, task.PrivateData.UpstreamTaskID, taskM))

			rate, groups := taskSuccessRate(t, tc.modelName)
			require.Equal(t, 1, groups)
			assert.Equal(t, tc.wantSuccess, rate)

			// A second poll of an already-terminal task must not add a sample.
			reloaded, exists, err := model.GetByOnlyTaskId(task.TaskID)
			require.NoError(t, err)
			require.True(t, exists)
			taskM[task.PrivateData.UpstreamTaskID] = reloaded
			require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, ch, task.PrivateData.UpstreamTaskID, taskM))

			// For the failure case a duplicate sample would leave the rate at
			// 0%, so assert the bucket count directly instead of the rate.
			assert.Equal(t, int64(1), taskRequestCount(t, tc.modelName))
		})
	}
}
