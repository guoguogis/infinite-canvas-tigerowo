package model

// MusicTask 音乐生成任务。
// Mode 取值 song（歌曲）或 instrumental（纯音乐），对应上游两套不同的生成接口。
type MusicTask struct {
	ID              string   `json:"id" gorm:"primaryKey"`
	UserID          string   `json:"userId" gorm:"index"`
	UserDisplayName string   `json:"userDisplayName"`
	ClientTaskID    string   `json:"clientTaskId" gorm:"index"`
	Source          string   `json:"source" gorm:"index"`
	SourceID        string   `json:"source_id" gorm:"index"`
	Mode            string   `json:"mode"`
	Title           string   `json:"title"`
	Prompt          string   `json:"prompt" gorm:"type:text"`
	Lyrics          string   `json:"lyrics" gorm:"type:text"`
	StyleTags       []string `json:"styleTags" gorm:"serializer:json"`
	Duration        int      `json:"duration"`
	Language        string   `json:"language"`
	VocalGender     string   `json:"vocalGender"`
	KeyMode         string   `json:"keyMode"`
	Tempo           int      `json:"tempo"`
	Model           string   `json:"model" gorm:"index"`
	ChannelID       string   `json:"channelId" gorm:"index"`
	UserChannelID   string   `json:"userChannelId" gorm:"index"`
	ChannelName     string   `json:"channelName"`
	Protocol        string   `json:"protocol"`
	Credits         float64  `json:"credits" gorm:"type:decimal(20,2)"`
	Status          string   `json:"status" gorm:"index:idx_music_tasks_status_created_at,priority:1"`
	Progress        int      `json:"progress"`
	AudioURL        string   `json:"audioUrl" gorm:"type:text"`
	StorageKey      string   `json:"storageKey"`
	MimeType        string   `json:"mimeType"`
	Bytes           int64    `json:"bytes"`
	DurationMs      int      `json:"durationMs"`
	UpstreamTaskID  string   `json:"upstreamTaskId" gorm:"index"`
	UpstreamModel   string   `json:"upstreamModel"`
	RequestBody     string   `json:"requestBody" gorm:"type:text"`
	ResponseBody    string   `json:"responseBody" gorm:"type:text"`
	LastResponse    string   `json:"lastResponse" gorm:"type:text"`
	Error           string   `json:"error" gorm:"type:text"`
	ErrorDetail     string   `json:"errorDetail" gorm:"type:text"`
	CreatedAt       string   `json:"createdAt" gorm:"index;index:idx_music_tasks_status_created_at,priority:2"`
	UpdatedAt       string   `json:"updatedAt" gorm:"index"`
	StartedAt       string   `json:"startedAt"`
	CompletedAt     string   `json:"completedAt"`
	LastPolledAt    string   `json:"lastPolledAt" gorm:"index"`
}
