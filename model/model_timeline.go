package model

import (
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// ModelTimeline 记录模型首次启用时间
type ModelTimeline struct {
	Model        string `json:"model" gorm:"type:varchar(255);primaryKey;autoIncrement:false"`
	FirstEnabled int64  `json:"first_enabled" gorm:"bigint;index"` // 首次在任何渠道启用的时间戳
	LastDisabled int64  `json:"last_disabled" gorm:"bigint"`       // 最后一次完全禁用的时间戳（所有渠道都禁用）
	IsActive     bool   `json:"is_active" gorm:"default:true"`     // 当前是否在至少一个渠道中启用
}

// SystemAnnouncement 系统公告时间线
type SystemAnnouncement struct {
	ID          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Type        string `json:"type" gorm:"type:varchar(50);index"` // 类型: model_online, model_offline, manual
	Model       string `json:"model" gorm:"type:varchar(255);index"`
	Content     string `json:"content" gorm:"type:text"`
	CreatedTime int64  `json:"created_time" gorm:"bigint;index"`
	Editable    bool   `json:"editable" gorm:"default:false"` // 是否可编辑（手动添加的才能编辑）
}

// timelineReconcileLock 串行化对账，避免并发写入重复公告。
// 对账是幂等的：状态未变化时不会产生任何写入。
var timelineReconcileLock sync.Mutex

// GetModelTimeline 获取模型时间线信息
func GetModelTimeline(model string) (*ModelTimeline, error) {
	timeline := &ModelTimeline{}
	err := DB.Where("model = ?", model).First(timeline).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return timeline, nil
}

// UpsertModelTimeline 创建或更新模型时间线
func UpsertModelTimeline(model string, isEnabled bool) error {
	now := common.GetTimestamp()

	timeline, err := GetModelTimeline(model)
	if err != nil {
		return err
	}

	if timeline == nil {
		// 首次记录
		if isEnabled {
			timeline = &ModelTimeline{
				Model:        model,
				FirstEnabled: now,
				IsActive:     true,
			}
			return DB.Create(timeline).Error
		}
		// 如果首次记录就是禁用，不创建记录
		return nil
	}

	// 更新现有记录
	updates := map[string]interface{}{
		"is_active": isEnabled,
	}

	if !isEnabled {
		updates["last_disabled"] = now
	}

	return DB.Model(&ModelTimeline{}).Where("model = ?", model).Updates(updates).Error
}

// getEnabledModelSet 返回当前至少在一个渠道启用的模型集合。
// 以 abilities 表为准：enabled=true 的行代表模型对用户可见（出现在模型广场）。
func getEnabledModelSet() (map[string]struct{}, error) {
	var models []string
	err := DB.Model(&Ability{}).
		Where("enabled = ?", true).
		Distinct("model").
		Pluck("model", &models).Error
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(models))
	for _, m := range models {
		m = strings.TrimSpace(m)
		if m != "" {
			set[m] = struct{}{}
		}
	}
	return set, nil
}

// ReconcileModelTimelines 将时间线状态与当前实际启用的模型集合对齐，
// 并为发生变化的模型写入上线/下线公告。
//
// 之所以用“对账”而不是在各个写入点逐条挂钩：模型的可见性会受新建渠道、
// 编辑渠道、批量导入、按标签启停、自动禁用、删除渠道等多条路径影响，
// 逐条挂钩很容易漏。对账只关心最终状态，覆盖所有路径且幂等。
func ReconcileModelTimelines() error {
	timelineReconcileLock.Lock()
	defer timelineReconcileLock.Unlock()

	enabled, err := getEnabledModelSet()
	if err != nil {
		return err
	}

	var timelines []*ModelTimeline
	if err := DB.Find(&timelines).Error; err != nil {
		return err
	}

	// 升级后的首次运行：只建立基线，不发公告。
	// 否则升级瞬间会给所有存量模型刷出一大批“已上线”。
	// 用 Option 标记区分“从未初始化”与“初始化后确实还没有任何模型”。
	if !isTimelineBaselineDone() {
		firstSeen := getEarliestChannelTimeByModel()
		now := common.GetTimestamp()
		for modelName := range enabled {
			// 用启用该模型的最早渠道创建时间作为上线时间的近似值，
			// 这样升级后老模型不会全部挤在“升级当天”。
			firstEnabled := firstSeen[modelName]
			if firstEnabled <= 0 {
				firstEnabled = now
			}
			if err := DB.Create(&ModelTimeline{
				Model:        modelName,
				FirstEnabled: firstEnabled,
				IsActive:     true,
			}).Error; err != nil {
				common.SysLog("failed to seed model timeline: model=" + modelName + ", error=" + err.Error())
			}
		}
		if err := DB.Save(&Option{Key: timelineBaselineOptionKey, Value: "true"}).Error; err != nil {
			common.SysLog("failed to persist model timeline baseline marker: " + err.Error())
		}
		return nil
	}

	timelineByModel := make(map[string]*ModelTimeline, len(timelines))
	for _, t := range timelines {
		timelineByModel[t.Model] = t
	}

	now := common.GetTimestamp()

	// 上线：现在可见但时间线里不是激活态
	for modelName := range enabled {
		timeline := timelineByModel[modelName]
		var content string
		switch {
		case timeline == nil:
			content = "模型 " + modelName + " 已上线"
		case !timeline.IsActive:
			content = "模型 " + modelName + " 已重新上线"
		default:
			// 已激活，无需处理
			continue
		}

		if timeline == nil {
			if err := DB.Create(&ModelTimeline{
				Model:        modelName,
				FirstEnabled: now,
				IsActive:     true,
			}).Error; err != nil {
				common.SysLog("failed to create model timeline: model=" + modelName + ", error=" + err.Error())
				continue
			}
		} else if err := DB.Model(&ModelTimeline{}).Where("model = ?", modelName).
			Update("is_active", true).Error; err != nil {
			common.SysLog("failed to update model timeline: model=" + modelName + ", error=" + err.Error())
			continue
		}

		if err := CreateSystemAnnouncement("model_online", modelName, content, false); err != nil {
			common.SysLog("failed to create online announcement: model=" + modelName + ", error=" + err.Error())
		}
	}

	// 下线：时间线里是激活态但现在不可见
	for _, timeline := range timelines {
		if !timeline.IsActive {
			continue
		}
		if _, ok := enabled[timeline.Model]; ok {
			continue
		}
		if err := DB.Model(&ModelTimeline{}).Where("model = ?", timeline.Model).
			Updates(map[string]interface{}{"is_active": false, "last_disabled": now}).Error; err != nil {
			common.SysLog("failed to update model timeline: model=" + timeline.Model + ", error=" + err.Error())
			continue
		}
		content := "模型 " + timeline.Model + " 已下线"
		if err := CreateSystemAnnouncement("model_offline", timeline.Model, content, false); err != nil {
			common.SysLog("failed to create offline announcement: model=" + timeline.Model + ", error=" + err.Error())
		}
	}

	return nil
}

// getEarliestChannelTimeByModel 返回每个模型最早出现的渠道创建时间。
// 用作存量模型上线时间的近似值。
func getEarliestChannelTimeByModel() map[string]int64 {
	type row struct {
		Model     string
		FirstSeen int64
	}
	var rows []row
	err := DB.Table("abilities").
		Select("abilities.model as model, MIN(channels.created_time) as first_seen").
		Joins("JOIN channels ON abilities.channel_id = channels.id").
		Where("abilities.enabled = ?", true).
		Group("abilities.model").
		Scan(&rows).Error
	if err != nil {
		common.SysLog("failed to query earliest channel time by model: " + err.Error())
		return nil
	}
	result := make(map[string]int64, len(rows))
	for _, r := range rows {
		result[r.Model] = r.FirstSeen
	}
	return result
}

// timelineBaselineOptionKey 标记时间线基线已初始化。
// 没有这个标记就无法区分“升级后首次运行”和“初始化完成但确实还没有任何模型”，
// 后者若被当成基线会吞掉新建模型的上线公告。
const timelineBaselineOptionKey = "ModelTimelineBaselineInitialized"

func isTimelineBaselineDone() bool {
	var opt Option
	err := DB.Where(commonKeyCol+" = ?", timelineBaselineOptionKey).First(&opt).Error
	return err == nil && opt.Value == "true"
}

// reconcileTimelinesSafe 供各写入路径调用：失败只记日志，不影响主流程。
func reconcileTimelinesSafe() {
	if err := ReconcileModelTimelines(); err != nil {
		common.SysLog("failed to reconcile model timelines: " + err.Error())
	}
}

// StartModelTimelineReconciler 周期对账，兜住所有未显式触发的变更路径。
func StartModelTimelineReconciler(frequencySeconds int) {
	if frequencySeconds <= 0 {
		frequencySeconds = 60
	}
	// 启动先跑一次，让历史数据建立基线
	if err := ReconcileModelTimelines(); err != nil {
		common.SysLog("model timeline reconcile failed: " + err.Error())
	}
	for {
		time.Sleep(time.Duration(frequencySeconds) * time.Second)
		if err := ReconcileModelTimelines(); err != nil {
			common.SysLog("model timeline reconcile failed: " + err.Error())
		}
	}
}

// GetAllModelTimelines 获取所有模型时间线
func GetAllModelTimelines() ([]*ModelTimeline, error) {
	var timelines []*ModelTimeline
	err := DB.Order("first_enabled DESC").Find(&timelines).Error
	return timelines, err
}

// CreateSystemAnnouncement 创建系统公告
func CreateSystemAnnouncement(announcementType, model, content string, editable bool) error {
	announcement := &SystemAnnouncement{
		Type:        announcementType,
		Model:       model,
		Content:     content,
		CreatedTime: common.GetTimestamp(),
		Editable:    editable,
	}
	return DB.Create(announcement).Error
}

// GetAllSystemAnnouncements 获取所有系统公告
func GetAllSystemAnnouncements(startIdx int, num int) ([]*SystemAnnouncement, error) {
	var announcements []*SystemAnnouncement
	err := DB.Order("created_time DESC").Limit(num).Offset(startIdx).Find(&announcements).Error
	return announcements, err
}

// GetRecentAutoAnnouncements 获取最近的自动公告（用于合并到状态接口）
func GetRecentAutoAnnouncements(limit int) ([]*SystemAnnouncement, error) {
	if limit <= 0 {
		limit = 20
	}
	var announcements []*SystemAnnouncement
	err := DB.Where("type IN ?", []string{"model_online", "model_offline"}).
		Order("created_time DESC").Limit(limit).Find(&announcements).Error
	return announcements, err
}

// GetSystemAnnouncementById 根据ID获取公告
func GetSystemAnnouncementById(id int) (*SystemAnnouncement, error) {
	announcement := &SystemAnnouncement{}
	err := DB.Where("id = ?", id).First(announcement).Error
	if err != nil {
		return nil, err
	}
	return announcement, nil
}

// UpdateSystemAnnouncement 更新系统公告（仅手动添加的可编辑）
func (announcement *SystemAnnouncement) Update() error {
	if !announcement.Editable {
		return gorm.ErrInvalidData
	}
	return DB.Model(announcement).Updates(announcement).Error
}

// DeleteSystemAnnouncement 删除系统公告
func DeleteSystemAnnouncement(id int) error {
	return DB.Where("id = ?", id).Delete(&SystemAnnouncement{}).Error
}

// CountSystemAnnouncements 统计公告总数
func CountSystemAnnouncements() (int64, error) {
	var count int64
	err := DB.Model(&SystemAnnouncement{}).Count(&count).Error
	return count, err
}
