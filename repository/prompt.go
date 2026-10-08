package repository

import (
	"errors"
	"time"

	"github.com/tigerowo/infinite-canvas/model"
	"gorm.io/gorm"
)

// PromptCategories 返回内置提示词分类的副本。
func PromptCategories() []model.PromptCategory {
	result := make([]model.PromptCategory, len(promptCategories))
	copy(result, promptCategories)
	return result
}

// CountPromptsByCategory 返回每个分类下的提示词条数。
func CountPromptsByCategory() (map[string]int, error) {
	db, err := DB()
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Category string
		Total    int
	}
	if err := db.Model(&model.Prompt{}).
		Select("category, count(*) as total").
		Group("category").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[string]int, len(rows))
	for _, row := range rows {
		counts[row.Category] = row.Total
	}
	return counts, nil
}

// PromptCategoryByCode 根据分类编码查找内置提示词分类。
func PromptCategoryByCode(category string) (model.PromptCategory, bool) {
	for _, item := range promptCategories {
		if item.Category == category {
			return item, true
		}
	}
	return model.PromptCategory{}, false
}

// ListPromptCategories 返回内置提示词分类。
func ListPromptCategories() ([]model.PromptCategory, error) {
	return PromptCategories(), nil
}

// ListPrompts 按查询条件返回提示词分页列表。
func ListPrompts(q model.Query) ([]model.Prompt, int64, error) {
	db, err := DB()
	if err != nil {
		return nil, 0, err
	}
	q.Normalize()
	tx := applyPromptFilters(db.Model(&model.Prompt{}), q)

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.Prompt
	if q.All {
		// SQLite 单次取数偏大时会随机失败（unable to open database file (14)），
		// 这里用游标分片取回全部数据，避免 OFFSET 和大结果集。
		if items, err = listAllPrompts(db, q, int(total)); err != nil {
			return nil, 0, err
		}
	} else if err := tx.Order("updated_at desc").Offset(q.Offset()).Limit(q.PageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}

	categories, _ := ListPromptCategories()
	githubURLs := map[string]string{}
	for _, item := range categories {
		githubURLs[item.Category] = item.GithubURL
	}
	for i := range items {
		items[i].GithubURL = githubURLs[items[i].Category]
	}
	return items, total, nil
}

// promptChunkSize 是单次取数的条数上限；SQLite 单次取数偏大时会随机报
// unable to open database file (14)，分片取小一些并配合重试更稳。
const promptChunkSize = 100

// promptChunkAttempts 是单片的取数尝试次数，用于抵消上面那个随机失败。
const promptChunkAttempts = 4

// listAllPrompts 用 (updated_at, id) 游标分批取回全部提示词，避免使用 OFFSET。
func listAllPrompts(db *gorm.DB, q model.Query, total int) ([]model.Prompt, error) {
	items := make([]model.Prompt, 0, total)
	cursor := promptCursor{}
	for {
		chunk, err := listPromptChunk(db, q, cursor)
		if err != nil {
			return nil, err
		}
		items = append(items, chunk...)
		if len(chunk) < promptChunkSize {
			return items, nil
		}
		last := chunk[len(chunk)-1]
		cursor = promptCursor{UpdatedAt: last.UpdatedAt, ID: last.ID}
	}
}

// listPromptChunk 取游标之后的一片数据，遇到随机失败会重试。
func listPromptChunk(db *gorm.DB, q model.Query, cursor promptCursor) ([]model.Prompt, error) {
	var lastErr error
	for attempt := 0; attempt < promptChunkAttempts; attempt++ {
		var chunk []model.Prompt
		chunkQuery := applyPromptFilters(db.Model(&model.Prompt{}), q)
		if condition, args := promptCursorCondition(cursor); condition != "" {
			chunkQuery = chunkQuery.Where(condition, args...)
		}
		err := chunkQuery.Order("updated_at desc, id desc").Limit(promptChunkSize).Find(&chunk).Error
		if err == nil {
			return chunk, nil
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	return nil, lastErr
}

type promptCursor struct {
	UpdatedAt string
	ID        string
}

// promptCursorCondition 返回游标之后的筛选条件；updated_at 为空串表示排在最末。
func promptCursorCondition(cursor promptCursor) (string, []any) {
	if cursor.UpdatedAt == "" && cursor.ID == "" {
		return "", nil
	}
	if cursor.UpdatedAt == "" {
		return "(updated_at = '' AND id < ?)", []any{cursor.ID}
	}
	return "(updated_at < ? OR (updated_at = ? AND id < ?) OR updated_at = '')", []any{cursor.UpdatedAt, cursor.UpdatedAt, cursor.ID}
}

// ListPromptTags 返回当前提示词查询条件下的全部标签。
func ListPromptTags(q model.Query) ([]string, error) {
	db, err := DB()
	if err != nil {
		return nil, err
	}
	q.Normalize()
	q.Tags = nil
	tx := applyPromptFilters(db.Model(&model.Prompt{}), q)

	var items []model.Prompt
	if err := tx.Select("tags").Find(&items).Error; err != nil {
		return nil, err
	}
	return promptTagsFromItems(items), nil
}

// SavePrompt 保存提示词，并在更新时保留原创建时间。
func SavePrompt(item model.Prompt) (model.Prompt, error) {
	db, err := DB()
	if err != nil {
		return item, err
	}
	if saved, ok, err := findPrompt(db, item.ID); err != nil {
		return item, err
	} else if ok && item.CreatedAt == "" {
		item.CreatedAt = saved.CreatedAt
	}
	item.GithubURL = ""
	return item, db.Save(&item).Error
}

// DeletePrompt 删除指定提示词。
func DeletePrompt(id string) error {
	db, err := DB()
	if err != nil {
		return err
	}
	return db.Delete(&model.Prompt{}, "id = ?", id).Error
}

// DeletePrompts 批量删除提示词。
func DeletePrompts(ids []string) error {
	db, err := DB()
	if err != nil {
		return err
	}
	return db.Delete(&model.Prompt{}, "id IN ?", ids).Error
}

// ReplacePromptCategory 用远程同步结果替换整个提示词分类。
func ReplacePromptCategory(category model.PromptCategory, items []model.Prompt) error {
	db, err := DB()
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("category = ?", category.Category).Delete(&model.Prompt{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		for i := range items {
			items[i].Category = category.Category
			items[i].GithubURL = ""
		}
		return tx.Create(&items).Error
	})
}

// applyPromptFilters 应用提示词列表的搜索条件。
func applyPromptFilters(tx *gorm.DB, q model.Query) *gorm.DB {
	if q.Keyword != "" {
		like := "%" + q.Keyword + "%"
		tx = tx.Where("title LIKE ? OR prompt LIKE ?", like, like)
	}
	if isActivePromptOption(q.Category) {
		tx = tx.Where("category = ?", q.Category)
	}
	return applyPromptTagsFilter(tx, q.Tags)
}

// findPrompt 根据 ID 查询提示词。
func findPrompt(db *gorm.DB, id string) (model.Prompt, bool, error) {
	item := model.Prompt{}
	err := db.Where("id = ?", id).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Prompt{}, false, nil
	}
	return item, err == nil, err
}

// applyPromptTagsFilter 应用 JSON 标签条件。
func applyPromptTagsFilter(tx *gorm.DB, tags []string) *gorm.DB {
	if len(tags) == 0 {
		return tx
	}
	condition := tx.Session(&gorm.Session{NewDB: true})
	for _, tag := range tags {
		condition = condition.Or(promptJSONTagsContains(tx), tag)
	}
	return tx.Where(condition)
}

func promptTagsFromItems(items []model.Prompt) []string {
	seen := map[string]bool{}
	tags := []string{}
	for _, item := range items {
		for _, tag := range item.Tags {
			if tag != "" && !seen[tag] {
				seen[tag] = true
				tags = append(tags, tag)
			}
		}
	}
	return tags
}

// promptJSONTagsContains 返回提示词 tags 的 JSON 包含条件。
func promptJSONTagsContains(tx *gorm.DB) string {
	switch tx.Dialector.Name() {
	case "mysql":
		return "JSON_CONTAINS(tags, JSON_QUOTE(?))"
	case "postgres":
		return "jsonb_exists(tags::jsonb, ?)"
	default:
		return "EXISTS (SELECT 1 FROM json_each(tags) WHERE value = ?)"
	}
}

// isActivePromptOption 判断提示词筛选项有效状态。
func isActivePromptOption(value string) bool {
	return value != "" && value != "全部" && value != "all"
}
