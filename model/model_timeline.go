package model

import (
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
