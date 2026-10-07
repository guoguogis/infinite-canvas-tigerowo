package service

import (
	"sort"
	"strings"
	"time"

	"github.com/tigerowo/infinite-canvas/model"
	"github.com/tigerowo/infinite-canvas/repository"
)

func ListPrompts(q model.Query) (model.PromptList, error) {
	items, total, err := repository.ListPrompts(q)
	if err != nil {
		return model.PromptList{}, err
	}
	tags, err := repository.ListPromptTags(q)
	if err != nil {
		return model.PromptList{}, err
	}
	categories := promptCategoryCodes(ListAllPromptCategories())
	return model.PromptList{Items: items, Tags: tags, Categories: categories, Total: int(total)}, nil
}

func ListPromptCategories() []model.PromptCategory {
	categories, _ := repository.ListPromptCategories()
	return categories
}

// ListAllPromptCategories 返回提示词来源与内置分类合并后的分类列表。
// 内置视频来源排在来源最前，随后是其它来源，最后是内置分类。
func ListAllPromptCategories() []model.PromptCategory {
	items := ListPromptCategories()
	sources, err := repository.ListPromptSources()
	if err != nil {
		return items
	}
	builtin := map[string]bool{}
	merged := make([]model.PromptCategory, 0, len(sources)+len(items))
	for _, source := range sources {
		if !source.Enabled {
			continue
		}
		builtin[source.ID] = source.Kind == model.PromptSourceKindBuiltin
		merged = append(merged, promptSourceCategory(source))
	}
	sort.SliceStable(merged, func(i, j int) bool {
		return builtin[merged[i].Category] && !builtin[merged[j].Category]
	})
	return append(merged, items...)
}

func promptSourceCategory(source model.PromptSource) model.PromptCategory {
	return model.PromptCategory{
		Category:    source.ID,
		Name:        source.Name,
		Description: source.Name,
		GithubURL:   source.Homepage,
		SourceID:    source.ID,
	}
}

// IsPromptSourceCategory 判断分类是否来自提示词来源。
func IsPromptSourceCategory(category string) bool {
	category = strings.TrimSpace(category)
	if category == "" {
		return false
	}
	sources, err := repository.ListPromptSources()
	if err != nil {
		return false
	}
	for _, source := range sources {
		if source.ID == category {
			return true
		}
	}
	return false
}

func SavePrompt(item model.Prompt) (model.Prompt, error) {
	now := time.Now().Format(time.RFC3339)
	if item.Category == "" {
		item.Category = repository.PromptCategories()[0].Category
	}
	if item.ID == "" {
		item.ID = newID(item.Category)
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	item.GithubURL = ""
	// 分类来自内置分类表或提示词来源时保留，否则回落到默认分类。
	if category, ok := repository.PromptCategoryByCode(item.Category); ok {
		item.Category = category.Category
	} else if !IsPromptSourceCategory(item.Category) {
		item.Category = repository.PromptCategories()[0].Category
	}
	return repository.SavePrompt(item)
}

func DeletePrompt(id string) error {
	return repository.DeletePrompt(id)
}

func DeletePrompts(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return repository.DeletePrompts(ids)
}

func promptCategoryCodes(items []model.PromptCategory) []string {
	codes := []string{}
	for _, item := range items {
		if item.Category != "" {
			codes = append(codes, item.Category)
		}
	}
	return codes
}
