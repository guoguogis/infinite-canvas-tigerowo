package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/tigerowo/infinite-canvas/service"
)

type musicTaskRequest struct {
	Mode          string   `json:"mode"`
	Title         string   `json:"title"`
	Prompt        string   `json:"prompt"`
	Lyrics        string   `json:"lyrics"`
	StyleTags     []string `json:"styleTags"`
	Duration      int      `json:"duration"`
	Language      string   `json:"language"`
	VocalGender   string   `json:"vocalGender"`
	KeyMode       string   `json:"keyMode"`
	Tempo         int      `json:"tempo"`
	Model         string   `json:"model"`
	ChannelID     string   `json:"channelId"`
	UserChannelID string   `json:"userChannelId"`
}

// CreateMusicTask 新建音乐生成任务。任务先落库为排队状态，上游调用交给后台轮询器。
func CreateMusicTask(w http.ResponseWriter, r *http.Request) {
	user, ok := service.UserFromContext(r.Context())
	if !ok {
		Fail(w, "未登录或权限不足")
		return
	}
	var request musicTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		Fail(w, "请求参数不正确")
		return
	}
	modelName := strings.TrimSpace(request.Model)
	if modelName == "" {
		Fail(w, "缺少模型名称")
		return
	}
	mode := strings.ToLower(strings.TrimSpace(request.Mode))
	instrumental := mode == "instrumental" || mode == "bgm"
	if strings.TrimSpace(request.Prompt) == "" && (instrumental || strings.TrimSpace(request.Lyrics) == "") {
		Fail(w, "请填写风格描述或歌词")
		return
	}
	channelID := firstNonEmpty(strings.TrimSpace(request.ChannelID), r.Header.Get("X-Model-Channel-ID"))
	userChannelID := firstNonEmpty(strings.TrimSpace(request.UserChannelID), r.Header.Get(userModelChannelHeader))
	if channelID == "" && userChannelID == "" {
		Fail(w, "缺少模型渠道")
		return
	}
	channel, resolvedUserChannelID, err := selectAIRequestChannel(user, modelName, channelID, userChannelID, true)
	if err != nil {
		log.Printf("music task select channel failed: model=%s err=%v", modelName, err)
		failAIChannelSelect(w, err, "AI 接口请求失败")
		return
	}
	if !service.SupportsMusicChannel(channel) {
		Fail(w, "该渠道不支持音乐生成")
		return
	}
	credits, err := service.ModelCost(modelName)
	if err != nil {
		log.Printf("music task read model cost failed: model=%s err=%v", modelName, err)
		Fail(w, "AI 接口请求失败")
		return
	}
	task, err := service.CreateMusicTask(service.MusicTaskCreateInput{
		UserID:          user.ID,
		UserDisplayName: firstNonEmpty(user.DisplayName, user.Username),
		ClientTaskID:    readClientMusicTaskID(r),
		Source:          readMusicTaskSource(r),
		SourceID:        readMusicTaskSourceID(r),
		Mode:            request.Mode,
		Title:           request.Title,
		Prompt:          request.Prompt,
		Lyrics:          request.Lyrics,
		StyleTags:       request.StyleTags,
		Duration:        request.Duration,
		Language:        request.Language,
		VocalGender:     request.VocalGender,
		KeyMode:         request.KeyMode,
		Tempo:           request.Tempo,
		Model:           modelName,
		ChannelID:       channel.ID,
		UserChannelID:   resolvedUserChannelID,
		ChannelName:     channel.Name,
		Protocol:        channel.Protocol,
		Credits:         credits,
		BillingName:     modelName,
		BillingPath:     "music",
	})
	if err != nil {
		log.Printf("create music task failed: user=%s model=%s err=%v", user.ID, modelName, err)
		FailError(w, err)
		return
	}
	OK(w, service.MusicTaskResponse(task))
}

// UserMusicTasks 列出当前用户的音乐任务（历史记录 + 未完成任务恢复）。
func UserMusicTasks(w http.ResponseWriter, r *http.Request) {
	user, ok := service.UserFromContext(r.Context())
	if !ok {
		Fail(w, "未登录或权限不足")
		return
	}
	tasks, err := service.ListUserMusicTasks(user.ID, r.URL.Query().Get("status"), parseQuery(r))
	if err != nil {
		log.Printf("list music tasks failed: user=%s err=%v", user.ID, err)
		Fail(w, "AI 接口请求失败")
		return
	}
	OK(w, tasks)
}

func GetMusicTask(w http.ResponseWriter, r *http.Request, id string) {
	user, ok := service.UserFromContext(r.Context())
	if !ok {
		Fail(w, "未登录或权限不足")
		return
	}
	task, found, err := service.GetUserMusicTask(user.ID, strings.TrimSpace(id))
	if err != nil {
		log.Printf("read music task failed: user=%s id=%s err=%v", user.ID, id, err)
		Fail(w, "AI 接口请求失败")
		return
	}
	if !found {
		Fail(w, "音乐任务不存在")
		return
	}
	OK(w, service.MusicTaskResponse(task))
}

func DeleteUserMusicTask(w http.ResponseWriter, r *http.Request, id string) {
	user, ok := service.UserFromContext(r.Context())
	if !ok {
		Fail(w, "未登录或权限不足")
		return
	}
	if strings.TrimSpace(id) == "" {
		Fail(w, "音乐任务不存在")
		return
	}
	if err := service.DeleteUserMusicTask(user.ID, id); err != nil {
		log.Printf("delete music task failed: user=%s id=%s err=%v", user.ID, id, err)
		Fail(w, "AI 接口请求失败")
		return
	}
	OK(w, map[string]any{"deleted": true})
}

// GenerateMusicLyrics 生成歌词，目前只有火山引擎渠道提供该接口。
func GenerateMusicLyrics(w http.ResponseWriter, r *http.Request) {
	user, ok := service.UserFromContext(r.Context())
	if !ok {
		Fail(w, "未登录或权限不足")
		return
	}
	var request musicTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		Fail(w, "请求参数不正确")
		return
	}
	modelName := strings.TrimSpace(request.Model)
	if modelName == "" {
		Fail(w, "缺少模型名称")
		return
	}
	if strings.TrimSpace(request.Prompt) == "" {
		Fail(w, "请填写歌词主题")
		return
	}
	channelID := firstNonEmpty(strings.TrimSpace(request.ChannelID), r.Header.Get("X-Model-Channel-ID"))
	userChannelID := firstNonEmpty(strings.TrimSpace(request.UserChannelID), r.Header.Get(userModelChannelHeader))
	if channelID == "" && userChannelID == "" {
		Fail(w, "缺少模型渠道")
		return
	}
	channel, resolvedUserChannelID, err := selectAIRequestChannel(user, modelName, channelID, userChannelID, true)
	if err != nil {
		log.Printf("music lyrics select channel failed: model=%s err=%v", modelName, err)
		failAIChannelSelect(w, err, "AI 接口请求失败")
		return
	}
	genre := ""
	if len(request.StyleTags) > 0 {
		genre = strings.TrimSpace(request.StyleTags[0])
	}
	lyrics, err := service.GenerateMusicLyrics(user.ID, service.MusicLyricsInput{
		Model:         modelName,
		Prompt:        request.Prompt,
		Genre:         genre,
		VocalGender:   request.VocalGender,
		ChannelID:     channel.ID,
		UserChannelID: resolvedUserChannelID,
	})
	if err != nil {
		log.Printf("generate music lyrics failed: user=%s model=%s err=%v", user.ID, modelName, err)
		Fail(w, err.Error())
		return
	}
	OK(w, map[string]any{"lyrics": lyrics})
}

func readClientMusicTaskID(r *http.Request) string {
	id := strings.TrimSpace(r.Header.Get("X-Client-Music-Task-ID"))
	if strings.HasPrefix(id, "client_music_task_") {
		return id
	}
	return ""
}

func readMusicTaskSource(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("X-Music-Task-Source"))
}

func readMusicTaskSourceID(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("X-Music-Task-Source-ID"))
}
