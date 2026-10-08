package model

// Prompt 提示词记录。
type Prompt struct {
	ID        string   `json:"id" gorm:"primaryKey"`
	Title     string   `json:"title"`
	CoverURL  string   `json:"coverUrl"`
	Prompt    string   `json:"prompt"`
	Tags      []string `json:"tags" gorm:"serializer:json"`
	Category  string   `json:"category" gorm:"index"`
	SourceID  string   `json:"sourceId" gorm:"index"`
	VideoURL  string   `json:"videoUrl"`
	GithubURL string   `json:"githubUrl" gorm:"-"`
	Preview   string   `json:"preview"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
}

// PromptSourceKind 表示来源的数据获取方式。
type PromptSourceKind string

const (
	// PromptSourceKindRemote 从 URL 拉取提示词 JSON。
	PromptSourceKindRemote PromptSourceKind = "remote"
	// PromptSourceKindBuiltin 使用程序内置的提示词数据。
	PromptSourceKindBuiltin PromptSourceKind = "builtin"
	// PromptSourceKindManifest 先从 URL 拉取清单，再按清单里的分类文件逐个拉取并合并。
	PromptSourceKindManifest PromptSourceKind = "manifest"
)

// PromptSource 提示词来源，指向一个返回 JSON 数组的远程地址或程序内置数据。
type PromptSource struct {
	ID          string           `json:"id" gorm:"primaryKey"`
	Name        string           `json:"name"`
	URL         string           `json:"url"`
	Homepage    string           `json:"homepage"`
	Category    string           `json:"category"`
	Kind        PromptSourceKind `json:"kind"`
	Enabled     bool             `json:"enabled"`
	BuiltIn     bool             `json:"builtIn"`
	LastSyncAt  string           `json:"lastSyncAt"`
	LastError   string           `json:"lastError"`
	PromptCount int              `json:"promptCount" gorm:"-"`
}

// PromptList 提示词分页结果。
type PromptList struct {
	Items      []Prompt `json:"items"`
	Tags       []string `json:"tags"`
	Categories []string `json:"categories"`
	Total      int      `json:"total"`
}

// PromptCategory 提示词分类。
type PromptCategory struct {
	Category    string `json:"category" gorm:"primaryKey"`
	Name        string `json:"name"`
	Description string `json:"description"`
	GithubURL   string `json:"githubUrl"`
	Remote      bool   `json:"remote"`
	// SourceID 非空表示该分类来自提示词来源。
	SourceID  string `json:"sourceId"`
	UpdatedAt string `json:"updatedAt"`
	// PromptCount 是该分类下的提示词条数，仅接口返回时填充。
	PromptCount int `json:"promptCount" gorm:"-"`
}
