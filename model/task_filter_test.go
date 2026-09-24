/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedFilterTasks 建立一组跨用户、跨模型、跨令牌的任务，用于校验筛选条件。
func seedFilterTasks(t *testing.T) {
	t.Helper()
	truncateTables(t)

	insertTask(t, &Task{
		TaskID:     "task-a",
		UserId:     1,
		Platform:   constant.TaskPlatformSuno,
		ModelName:  "suno_music",
		TokenName:  "prod-key",
		SubmitTime: 100,
	})
	insertTask(t, &Task{
		TaskID:     "task-b",
		UserId:     1,
		Platform:   constant.TaskPlatformImage,
		ModelName:  "kling-v1",
		TokenName:  "dev-key",
		SubmitTime: 200,
	})
	insertTask(t, &Task{
		TaskID:     "task-c",
		UserId:     2,
		Platform:   constant.TaskPlatformImage,
		ModelName:  "kling-v1",
		TokenName:  "prod-key",
		SubmitTime: 300,
	})
}

func taskIDsOf(tasks []*Task) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.TaskID)
	}
	return ids
}

// 管理员任务日志按模型名/令牌名筛选，且列表与计数必须给出一致的结果集。
func TestTaskGetAllTasks_FilterByModelAndTokenName(t *testing.T) {
	seedFilterTasks(t)

	cases := []struct {
		name     string
		params   SyncTaskQueryParams
		expected []string
	}{
		{
			name:     "exact model name",
			params:   SyncTaskQueryParams{ModelName: "kling-v1"},
			expected: []string{"task-c", "task-b"},
		},
		{
			name:     "exact token name",
			params:   SyncTaskQueryParams{TokenName: "prod-key"},
			expected: []string{"task-c", "task-a"},
		},
		{
			name:     "model and token combined",
			params:   SyncTaskQueryParams{ModelName: "kling-v1", TokenName: "prod-key"},
			expected: []string{"task-c"},
		},
		{
			name:     "wildcard model name",
			params:   SyncTaskQueryParams{ModelName: "kling%"},
			expected: []string{"task-c", "task-b"},
		},
		{
			name:     "no match",
			params:   SyncTaskQueryParams{ModelName: "gpt-4o"},
			expected: []string{},
		},
		{
			name:     "empty filters return all",
			params:   SyncTaskQueryParams{},
			expected: []string{"task-c", "task-b", "task-a"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tasks := TaskGetAllTasks(0, 100, tc.params)
			assert.Equal(t, tc.expected, taskIDsOf(tasks))
			assert.Equal(t, int64(len(tc.expected)), TaskCountAllTasks(tc.params),
				"count must agree with the listing or pagination breaks")
		})
	}
}

// 普通用户视图的筛选不能越过 user_id 边界。
func TestTaskGetAllUserTask_FilterStaysScopedToUser(t *testing.T) {
	seedFilterTasks(t)

	params := SyncTaskQueryParams{TokenName: "prod-key"}
	tasks := TaskGetAllUserTask(1, 0, 100, params)
	assert.Equal(t, []string{"task-a"}, taskIDsOf(tasks))
	assert.Equal(t, int64(1), TaskCountAllUserTask(1, params))
}

// % 之外的 LIKE 元字符必须按字面量匹配，避免 _ 变成单字符通配符。
func TestTaskGetAllTasks_UnderscoreIsLiteral(t *testing.T) {
	seedFilterTasks(t)

	params := SyncTaskQueryParams{ModelName: "suno_%"}
	assert.Equal(t, []string{"task-a"}, taskIDsOf(TaskGetAllTasks(0, 100, params)))

	// suno_music 里的 _ 是字面量，"sunoXmusic" 式的模式不应命中
	noMatch := SyncTaskQueryParams{ModelName: "suno%music%extra"}
	assert.Empty(t, TaskGetAllTasks(0, 100, noMatch))
}

// 回填把 JSON 列里的模型名/令牌名提升到可筛选列，并且可重复运行。
func TestBackfillTaskModelAndTokenNames(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&Option{}))
	t.Cleanup(func() { DB.Exec("DELETE FROM options") })
	DB.Exec("DELETE FROM options")

	token := &Token{Name: "legacy-token", UserId: 7, Key: "k-legacy"}
	require.NoError(t, DB.Create(token).Error)

	legacy := &Task{
		TaskID:     "legacy",
		UserId:     7,
		Platform:   constant.TaskPlatformImage,
		Properties: Properties{OriginModelName: "kling-v1"},
		SubmitTime: 100,
	}
	legacy.PrivateData.TokenId = token.Id
	insertTask(t, legacy)

	// 只有上游模型名的历史行应回退到 upstream_model_name
	upstreamOnly := &Task{
		TaskID:     "upstream-only",
		UserId:     7,
		Platform:   constant.TaskPlatformImage,
		Properties: Properties{UpstreamModelName: "kling-upstream"},
		SubmitTime: 200,
	}
	insertTask(t, upstreamOnly)

	backfillTaskModelAndTokenNames()

	var got Task
	require.NoError(t, DB.Where("task_id = ?", "legacy").First(&got).Error)
	assert.Equal(t, "kling-v1", got.ModelName)
	assert.Equal(t, "legacy-token", got.TokenName)

	var upstream Task
	require.NoError(t, DB.Where("task_id = ?", "upstream-only").First(&upstream).Error)
	assert.Equal(t, "kling-upstream", upstream.ModelName)
	assert.Empty(t, upstream.TokenName, "no token id means no token name")

	// 回填后即可按模型筛选到历史任务
	assert.Equal(t, []string{"legacy"},
		taskIDsOf(TaskGetAllTasks(0, 100, SyncTaskQueryParams{ModelName: "kling-v1"})))

	// 第二次运行必须是空操作：标记已写入，且不会覆盖既有值
	require.NoError(t, DB.Model(&Task{}).Where("task_id = ?", "legacy").
		Update("model_name", "manually-corrected").Error)
	backfillTaskModelAndTokenNames()
	require.NoError(t, DB.Where("task_id = ?", "legacy").First(&got).Error)
	assert.Equal(t, "manually-corrected", got.ModelName,
		"backfill must not re-run once the completion marker exists")
}
