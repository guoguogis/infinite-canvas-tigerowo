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
	if err := DeletePromptSourcePrompts(sourceID); err != nil {
		return err
	}
	return AppendPromptSourcePrompts(sourceID, items)
}

// DeletePromptSourcePrompts 清空某个来源已有的提示词。
func DeletePromptSourcePrompts(sourceID string) error {
	db, err := DB()
	if err != nil {
		return err
	}
	return db.Where("source_id = ?", sourceID).Delete(&model.Prompt{}).Error
}

// promptInsertBatchSize 是单条 INSERT 的行数上限。
// SQLite 单条语句的绑定参数有上限（默认 32766），prompts 有 11 列，500 行约 5500 个参数，留足余量。
const promptInsertBatchSize = 500

// AppendPromptSourcePrompts 追加写入某个来源的提示词。
// 按行数切块，每块用一条独立的 INSERT 语句写入：
// 既避免超出 SQLite 的单语句参数上限，也不使用 GORM 的 CreateInBatches —— 它会把所有批次包进同一个
// 显式事务，在本项目的 SQLite 配置下执行多条语句会报 unable to open database file。
func AppendPromptSourcePrompts(sourceID string, items []model.Prompt) error {
	if len(items) == 0 {
		return nil
	}
	db, err := DB()
	if err != nil {
		return err
	}
	for index := range items {
		items[index].SourceID = sourceID
	}
	for start := 0; start < len(items); start += promptInsertBatchSize {
		end := start + promptInsertBatchSize
		if end > len(items) {
			end = len(items)
		}
		chunk := items[start:end]
		if err := db.Create(&chunk).Error; err != nil {
			return err
		}
	}
	return nil
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
