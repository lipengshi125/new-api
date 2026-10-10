package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MaxModelAPIDocBytes bounds one model's API document. MySQL TEXT holds 65535
// bytes, so the limit stays below it to keep the column portable across
// SQLite, MySQL and PostgreSQL.
const MaxModelAPIDocBytes = 60 * 1024

var ErrModelAPIDocTooLarge = fmt.Errorf("API 文档不能超过 %d KB", MaxModelAPIDocBytes/1024)

// ModelAPIDoc is an admin-authored Markdown document that replaces the
// auto-generated API panel on the model marketplace. It is keyed by model name
// rather than the models row: like call samples it is presentation, so a model
// without a 元信息 entry can still carry one. Docs live in their own table, not
// in options, so the public pricing response only carries a has_api_doc flag
// and each document is fetched on demand.
type ModelAPIDoc struct {
	ModelName   string `json:"model_name" gorm:"primaryKey;size:128"`
	Content     string `json:"content" gorm:"type:text"`
	UpdatedTime int64  `json:"updated_time" gorm:"bigint"`
}

func (ModelAPIDoc) TableName() string {
	return "model_api_docs"
}

// GetModelAPIDoc returns a model's document, or ok=false when it has none.
func GetModelAPIDoc(modelName string) (doc ModelAPIDoc, ok bool, err error) {
	err = DB.Where("model_name = ?", strings.TrimSpace(modelName)).First(&doc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ModelAPIDoc{}, false, nil
	}
	if err != nil {
		return ModelAPIDoc{}, false, err
	}
	return doc, true, nil
}

// SaveModelAPIDoc stores a model's document. Whitespace-only content removes
// it, which restores the auto-generated panel.
func SaveModelAPIDoc(modelName string, content string) error {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return errors.New("模型名称不能为空")
	}
	if len(content) > MaxModelAPIDocBytes {
		return ErrModelAPIDocTooLarge
	}
	if strings.TrimSpace(content) == "" {
		return DB.Where("model_name = ?", modelName).Delete(&ModelAPIDoc{}).Error
	}
	doc := ModelAPIDoc{
		ModelName:   modelName,
		Content:     content,
		UpdatedTime: time.Now().Unix(),
	}
	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "model_name"}},
		DoUpdates: clause.AssignmentColumns([]string{"content", "updated_time"}),
	}).Create(&doc).Error
}

// GetModelAPIDocNameSet returns the names of every model that has a document,
// so the pricing cache can flag them without loading any content.
func GetModelAPIDocNameSet() (map[string]struct{}, error) {
	var names []string
	if err := DB.Model(&ModelAPIDoc{}).Pluck("model_name", &names).Error; err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		set[name] = struct{}{}
	}
	return set, nil
}
