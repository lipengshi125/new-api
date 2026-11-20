package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clearTimelineTables(t *testing.T) {
	t.Helper()
	wipe := func() {
		DB.Exec("DELETE FROM model_timelines")
		DB.Exec("DELETE FROM system_announcements")
		DB.Exec("DELETE FROM abilities")
		DB.Exec("DELETE FROM channels")
		DB.Exec("DELETE FROM options WHERE `key` = ?", timelineBaselineOptionKey)
	}
	// 前后都清一次：其他测试可能留下同 id 的渠道
	wipe()
	t.Cleanup(wipe)
	// 默认认为基线已完成，这样各用例聚焦于真实的上下线事件
	markBaselineDone(t)
}

func markBaselineDone(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.Save(&Option{Key: timelineBaselineOptionKey, Value: "true"}).Error)
}

// 升级后首次运行：存量模型只建立基线，不产生公告
func TestModelTimeline_BaselineSeedsWithoutAnnouncement(t *testing.T) {
	clearTimelineTables(t)
	DB.Exec("DELETE FROM options WHERE `key` = ?", timelineBaselineOptionKey)

	channel := newTestChannel(t, 20, "legacy-a,legacy-b", "default", common.ChannelStatusEnabled)
	require.NoError(t, channel.AddAbilities(nil))

	require.NoError(t, ReconcileModelTimelines())

	assert.Empty(t, announcementsOfType(t, "model_online"), "基线初始化不应发公告")
	assert.Empty(t, announcementsOfType(t, "model_offline"))
	assert.NotNil(t, mustTimeline(t, "legacy-a"))
	assert.NotNil(t, mustTimeline(t, "legacy-b"))

	// 基线标记写入后，后续新模型才会真正发公告
	channel2 := newTestChannel(t, 21, "fresh-model", "default", common.ChannelStatusEnabled)
	require.NoError(t, channel2.AddAbilities(nil))
	require.NoError(t, ReconcileModelTimelines())

	online := announcementsOfType(t, "model_online")
	require.Len(t, online, 1)
	assert.Equal(t, "模型 fresh-model 已上线", online[0].Content)
}

// 新建渠道走 AddAbilities（BatchInsertChannels 路径），应即时生成上线公告
func TestModelTimeline_BatchInsertChannelAnnounces(t *testing.T) {
	clearTimelineTables(t)

	priority := int64(0)
	weight := uint(0)
	channels := []Channel{{
		Id:          22,
		Type:        1,
		Key:         "k",
		Name:        "batch",
		Status:      common.ChannelStatusEnabled,
		Models:      "brand-new-model",
		Group:       "default",
		Priority:    &priority,
		Weight:      &weight,
		CreatedTime: common.GetTimestamp(),
	}}
	require.NoError(t, BatchInsertChannels(channels))

	online := announcementsOfType(t, "model_online")
	require.Len(t, online, 1, "新建渠道应即时发出上线公告")
	assert.Equal(t, "模型 brand-new-model 已上线", online[0].Content)
	assert.NotNil(t, mustTimeline(t, "brand-new-model"))
}

// newTestChannel 构造一个用于 UpdateAbilities 的渠道，并写入 channels 表
func newTestChannel(t *testing.T, id int, models string, group string, status int) *Channel {
	t.Helper()
	priority := int64(0)
	weight := uint(0)
	channel := &Channel{
		Id:          id,
		Type:        1,
		Key:         "test-key",
		Name:        "test-channel",
		Status:      status,
		Models:      models,
		Group:       group,
		Priority:    &priority,
		Weight:      &weight,
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(channel).Error)
	return channel
}

func announcementsOfType(t *testing.T, announcementType string) []*SystemAnnouncement {
	t.Helper()
	var list []*SystemAnnouncement
	require.NoError(t, DB.Where("type = ?", announcementType).Order("id").Find(&list).Error)
	return list
}

func TestModelTimeline_FirstOnlineCreatesAnnouncement(t *testing.T) {
	clearTimelineTables(t)

	channel := newTestChannel(t, 1, "gpt-4o", "default", common.ChannelStatusEnabled)
	require.NoError(t, channel.UpdateAbilities(nil))

	timeline, err := GetModelTimeline("gpt-4o")
	require.NoError(t, err)
	require.NotNil(t, timeline, "首次上线应写入时间线")
	assert.True(t, timeline.IsActive)
	assert.Greater(t, timeline.FirstEnabled, int64(0))

	online := announcementsOfType(t, "model_online")
	require.Len(t, online, 1)
	assert.Equal(t, "gpt-4o", online[0].Model)
	assert.False(t, online[0].Editable, "自动生成的公告不可编辑")
	assert.Equal(t, "模型 gpt-4o 已上线", online[0].Content)
}

// 一个渠道配置多个分组时，同一模型会产生多行 abilities。
// 这是曾经导致 count==1 永不成立、公告不触发的场景。
func TestModelTimeline_MultiGroupChannelStillAnnounces(t *testing.T) {
	clearTimelineTables(t)

	channel := newTestChannel(t, 2, "claude-3-5", "default,vip", common.ChannelStatusEnabled)
	require.NoError(t, channel.UpdateAbilities(nil))

	var abilityRows int64
	require.NoError(t, DB.Model(&Ability{}).Where("model = ?", "claude-3-5").Count(&abilityRows).Error)
	require.Equal(t, int64(2), abilityRows, "两个分组应产生两行 abilities")

	online := announcementsOfType(t, "model_online")
	require.Len(t, online, 1, "多分组不应影响上线公告的触发")
	assert.Equal(t, "模型 claude-3-5 已上线", online[0].Content)
}

// 第二个渠道启用同一模型时，模型本来就在广场里，不应重复公告
func TestModelTimeline_SecondChannelDoesNotDuplicateAnnouncement(t *testing.T) {
	clearTimelineTables(t)

	first := newTestChannel(t, 3, "gpt-4o-mini", "default", common.ChannelStatusEnabled)
	require.NoError(t, first.UpdateAbilities(nil))

	second := newTestChannel(t, 4, "gpt-4o-mini", "default", common.ChannelStatusEnabled)
	require.NoError(t, second.UpdateAbilities(nil))

	online := announcementsOfType(t, "model_online")
	assert.Len(t, online, 1, "模型已可见，第二个渠道不应再发上线公告")

	timeline, err := GetModelTimeline("gpt-4o-mini")
	require.NoError(t, err)
	require.NotNil(t, timeline)
	assert.True(t, timeline.IsActive)
}

func TestModelTimeline_DisableOneOfTwoChannelsKeepsModelActive(t *testing.T) {
	clearTimelineTables(t)

	a := newTestChannel(t, 5, "gemini-pro", "default", common.ChannelStatusEnabled)
	require.NoError(t, a.UpdateAbilities(nil))
	b := newTestChannel(t, 6, "gemini-pro", "default", common.ChannelStatusEnabled)
	require.NoError(t, b.UpdateAbilities(nil))

	// 禁用其中一个渠道
	b.Status = common.ChannelStatusManuallyDisabled
	require.NoError(t, b.Update())

	assert.Empty(t, announcementsOfType(t, "model_offline"), "仍有渠道启用时不应下线")

	timeline, err := GetModelTimeline("gemini-pro")
	require.NoError(t, err)
	require.NotNil(t, timeline)
	assert.True(t, timeline.IsActive)
}

func TestModelTimeline_FullOfflineThenReonline(t *testing.T) {
	clearTimelineTables(t)

	channel := newTestChannel(t, 7, "qwen-max", "default", common.ChannelStatusEnabled)
	require.NoError(t, channel.UpdateAbilities(nil))
	firstEnabled := mustTimeline(t, "qwen-max").FirstEnabled

	// 全部禁用 -> 下线
	channel.Status = common.ChannelStatusManuallyDisabled
	require.NoError(t, channel.Update())

	offline := announcementsOfType(t, "model_offline")
	require.Len(t, offline, 1)
	assert.Equal(t, "模型 qwen-max 已下线", offline[0].Content)
	assert.False(t, offline[0].Editable)

	timeline := mustTimeline(t, "qwen-max")
	assert.False(t, timeline.IsActive)
	assert.Greater(t, timeline.LastDisabled, int64(0))
	assert.Equal(t, firstEnabled, timeline.FirstEnabled, "首次上线时间不应被覆盖")

	// 重新启用 -> 重新上线
	channel.Status = common.ChannelStatusEnabled
	require.NoError(t, channel.Update())

	online := announcementsOfType(t, "model_online")
	require.Len(t, online, 2, "应包含首次上线与重新上线两条")
	assert.Equal(t, "模型 qwen-max 已上线", online[0].Content)
	assert.Equal(t, "模型 qwen-max 已重新上线", online[1].Content)

	timeline = mustTimeline(t, "qwen-max")
	assert.True(t, timeline.IsActive)
	assert.Equal(t, firstEnabled, timeline.FirstEnabled, "首次上线时间仍然保留")
}

// 历史数据没有时间线记录时，下线不应凭空刷出公告
func TestModelTimeline_OfflineWithoutPriorRecordIsSilent(t *testing.T) {
	clearTimelineTables(t)

	// 直接构造已启用的 ability，不经过 UpdateAbilities，因此没有时间线记录
	channel := newTestChannel(t, 8, "legacy-model", "default", common.ChannelStatusEnabled)
	require.NoError(t, DB.Create(&Ability{
		Group:     "default",
		Model:     "legacy-model",
		ChannelId: channel.Id,
		Enabled:   true,
	}).Error)

	channel.Status = common.ChannelStatusManuallyDisabled
	require.NoError(t, channel.Update())

	assert.Empty(t, announcementsOfType(t, "model_offline"))

	timeline, err := GetModelTimeline("legacy-model")
	require.NoError(t, err)
	assert.Nil(t, timeline, "无上线记录时不应创建时间线")
}

func mustTimeline(t *testing.T, modelName string) *ModelTimeline {
	t.Helper()
	timeline, err := GetModelTimeline(modelName)
	require.NoError(t, err)
	require.NotNil(t, timeline)
	return timeline
}
