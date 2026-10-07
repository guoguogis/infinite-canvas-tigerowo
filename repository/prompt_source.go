package repository

import (
	"errors"

	"github.com/tigerowo/infinite-canvas/model"
	"gorm.io/gorm"
)

// ListPromptSources 返回全部提示词来源，并附带各自的提示词条数。
func ListPromptSources() ([]model.PromptSource, error) {
	db, err := DB()
	if err != nil {
		return nil, err
	}
	items := []model.PromptSource{}
	if err := db.Order("built_in desc, name asc").Find(&items).Error; err != nil {
		return nil, err
	}
	counts := map[string]int{}
	type row struct {
		SourceID string
		Total    int
	}
	rows := []row{}
	if err := db.Model(&model.Prompt{}).
		Select("source_id, count(*) as total").
		Where("source_id <> ''").
		Group("source_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, item := range rows {
		counts[item.SourceID] = item.Total
	}
	for i := range items {
		items[i].PromptCount = counts[items[i].ID]
	}
	return items, nil
}

// FindPromptSource 按 ID 查询提示词来源。
func FindPromptSource(id string) (model.PromptSource, bool, error) {
	db, err := DB()
	if err != nil {
		return model.PromptSource{}, false, err
	}
	item := model.PromptSource{}
	err = db.Where("id = ?", id).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.PromptSource{}, false, nil
	}
	return item, err == nil, err
}

// SavePromptSource 保存提示词来源。
func SavePromptSource(item model.PromptSource) (model.PromptSource, error) {
	db, err := DB()
	if err != nil {
		return item, err
	}
	return item, db.Save(&item).Error
}

// DeletePromptSource 删除提示词来源，并清理它同步进来的提示词。
func DeletePromptSource(id string) error {
	db, err := DB()
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("source_id = ?", id).Delete(&model.Prompt{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&model.PromptSource{}).Error
	})
}

// ReplacePromptSourcePrompts 用最新同步结果替换某个来源的全部提示词。
func ReplacePromptSourcePrompts(sourceID string, items []model.Prompt) error {
	db, err := DB()
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("source_id = ?", sourceID).Delete(&model.Prompt{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		return tx.Create(&items).Error
	})
}

// CountPromptSourceItems 返回某个来源当前的提示词条数。
func CountPromptSourceItems(sourceID string) (int64, error) {
	db, err := DB()
	if err != nil {
		return 0, err
	}
	var total int64
	if err := db.Model(&model.Prompt{}).Where("source_id = ?", sourceID).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}
