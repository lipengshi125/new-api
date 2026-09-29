package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	commonRelay "github.com/QuantumNous/new-api/relay/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newSubmittedTaskRelayInfo builds the RelayInfo shape the task controller holds
// after a successful upstream submission. ChannelMeta and TaskRelayInfo are
// embedded pointers that GenRelayInfo always populates for task relays.
func newSubmittedTaskRelayInfo() *commonRelay.RelayInfo {
	info := &commonRelay.RelayInfo{
		UserId:          7,
		UsingGroup:      "default",
		OriginModelName: "image-2.5-sunburst",
		TaskRelayInfo:   &commonRelay.TaskRelayInfo{},
	}
	info.ChannelMeta = &commonRelay.ChannelMeta{ChannelId: 101}
	return info
}

// A task row is only created after the upstream accepted the submission, so it
// must never start out as NOT_START: that status survived until the first
// polling round wrote a real status back, so every freshly accepted task read
// as "not started" in the task log for a full polling interval.
func TestInitTaskStartsAsSubmitted(t *testing.T) {
	task := InitTask(constant.TaskPlatformImage, newSubmittedTaskRelayInfo())
	require.NotNil(t, task)

	assert.Equal(t, TaskStatus(TaskStatusSubmitted), task.Status)
	assert.NotEqual(t, TaskStatusNotStart, task.Status)
	// Progress still starts at 0%, which is what keeps the task inside the
	// unfinished-task polling window.
	assert.Equal(t, "0%", task.Progress)
}

// The public video/image API reports a freshly submitted task as queued, so the
// new starting status must not change what clients poll for.
func TestSubmittedTaskReportsQueuedToClients(t *testing.T) {
	task := InitTask(constant.TaskPlatformImage, newSubmittedTaskRelayInfo())
	require.NotNil(t, task)

	assert.Equal(t, "queued", task.Status.ToVideoStatus())
}

// Changing the starting status must not drop new tasks out of the polling
// query, which selects on progress and terminal statuses rather than on
// NOT_START.
func TestNewlySubmittedTaskIsPolled(t *testing.T) {
	require.NoError(t, DB.Where("1 = 1").Delete(&Task{}).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Where("1 = 1").Delete(&Task{}).Error)
	})

	task := InitTask(constant.TaskPlatformImage, newSubmittedTaskRelayInfo())
	task.PrivateData.UpstreamTaskID = "upstream-1"
	require.NoError(t, task.Insert())

	assert.True(t, HasUnfinishedSyncTasks())

	pending := GetAllUnFinishSyncTasks(100)
	require.Len(t, pending, 1)
	assert.Equal(t, task.TaskID, pending[0].TaskID)
	assert.Equal(t, TaskStatus(TaskStatusSubmitted), pending[0].Status)
}
