package service

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tigerowo/infinite-canvas/model"
	"github.com/tigerowo/infinite-canvas/repository"
)

const (
	// musicTaskPollInterval 是轮询上游的间隔，火山引擎建议 10 秒。
	musicTaskPollInterval = 10 * time.Second
	musicTaskPollBatch    = 50
	// musicTaskAudioLimit 是单次转存的音频体积上限。
	musicTaskAudioLimit = 64 << 20
)

var (
	musicTaskPollerOnce  sync.Once
	musicTaskPollWake    = make(chan struct{}, 1)
	musicTaskRunningMu   sync.Mutex
	musicTaskRunning     bool
	musicTaskWakePending bool
)

type MusicTaskCreateInput struct {
	UserID          string
	UserDisplayName string
	ClientTaskID    string
	Source          string
	SourceID        string
	Mode            string
	Title           string
	Prompt          string
	Lyrics          string
	StyleTags       []string
	Duration        int
	Language        string
	VocalGender     string
	KeyMode         string
	Tempo           int
	Model           string
	ChannelID       string
	UserChannelID   string
	ChannelName     string
	Protocol        string
	Status          string
	Progress        int
	Credits         float64
	BillingName     string
	BillingPath     string
}

// MusicTaskPollUpdate 是一次上游调用后要写回任务的字段。
type MusicTaskPollUpdate struct {
	Status         string
	Progress       int
	UpstreamTaskID string
	UpstreamModel  string
	StorageKey     string
	MimeType       string
	Bytes          int64
	AudioURL       string
	DurationMs     int
	Lyrics         string
	RequestBody    string
	ResponseBody   string
	Error          string
	ErrorDetail    string
}

type MusicTaskList struct {
	Items    []map[string]any `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
}

// CreateMusicTask 建一条排队中的音乐任务并扣除算力点。上游调用由后台轮询器完成。
func CreateMusicTask(input MusicTaskCreateInput) (model.MusicTask, error) {
	current := now()
	status := NormalizeMusicTaskStatus(input.Status)
	if status == "" {
		status = "queued"
	}
	task := model.MusicTask{
		ID:              firstNonEmpty(input.ClientTaskID, "music-task-"+uuid.NewString()),
		UserID:          strings.TrimSpace(input.UserID),
		UserDisplayName: strings.TrimSpace(input.UserDisplayName),
		ClientTaskID:    strings.TrimSpace(input.ClientTaskID),
		Source:          normalizeMusicTaskSource(input.Source),
		SourceID:        strings.TrimSpace(input.SourceID),
		Mode:            normalizeMusicTaskMode(input.Mode),
		Title:           strings.TrimSpace(input.Title),
		Prompt:          strings.TrimSpace(input.Prompt),
		Lyrics:          input.Lyrics,
		StyleTags:       input.StyleTags,
		Duration:        input.Duration,
		Language:        strings.TrimSpace(input.Language),
		VocalGender:     strings.TrimSpace(input.VocalGender),
		KeyMode:         strings.TrimSpace(input.KeyMode),
		Tempo:           input.Tempo,
		Model:           strings.TrimSpace(input.Model),
		ChannelID:       strings.TrimSpace(input.ChannelID),
		UserChannelID:   strings.TrimSpace(input.UserChannelID),
		ChannelName:     strings.TrimSpace(input.ChannelName),
		Protocol:        strings.TrimSpace(input.Protocol),
		Credits:         normalizeCredits(input.Credits),
		Status:          status,
		Progress:        clampProgress(input.Progress),
		CreatedAt:       current,
		UpdatedAt:       current,
	}
	if IsCompletedMusicTaskStatus(task.Status) {
		task.Status = "completed"
		task.Progress = 100
		task.CompletedAt = current
	} else if IsFailedMusicTaskStatus(task.Status) || task.Error != "" {
		task.Status = "failed"
		task.CompletedAt = current
	}
	var saved model.MusicTask
	var err error
	if task.Credits > 0 {
		// 扣费与任务写入在同一个事务里完成。
		saved = task
		err = ConsumeUserCredits(saved.UserID, input.BillingName, saved.Credits, input.BillingPath, &saved)
	} else {
		saved, err = repository.SaveMusicTask(task)
	}
	if err == nil && !IsCompletedMusicTaskStatus(saved.Status) && !IsFailedMusicTaskStatus(saved.Status) {
		WakeMusicTaskPoller()
	}
	return saved, err
}

func GetUserMusicTask(userID string, id string) (model.MusicTask, bool, error) {
	return repository.GetUserMusicTask(strings.TrimSpace(userID), strings.TrimSpace(id))
}

func ListUserMusicTasks(userID string, status string, q model.Query) (MusicTaskList, error) {
	q.Normalize()
	tasks, total, err := repository.ListUserMusicTasks(strings.TrimSpace(userID), strings.TrimSpace(status), q)
	if err != nil {
		return MusicTaskList{}, err
	}
	items := make([]map[string]any, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, MusicTaskResponse(task))
	}
	return MusicTaskList{Items: items, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

func DeleteUserMusicTask(userID string, id string) error {
	return repository.DeleteUserMusicTask(strings.TrimSpace(userID), strings.TrimSpace(id))
}

// MusicTaskResponse 组装返回给前端的任务结构。
// 音频地址只暴露已转存到自有存储的地址，上游临时地址不外传。
func MusicTaskResponse(task model.MusicTask) map[string]any {
	result := map[string]any{
		"id":             task.ID,
		"object":         "music",
		"mode":           task.Mode,
		"title":          task.Title,
		"model":          task.Model,
		"channelId":      task.ChannelID,
		"userChannelId":  task.UserChannelID,
		"channelName":    task.ChannelName,
		"protocol":       task.Protocol,
		"source":         task.Source,
		"source_id":      task.SourceID,
		"status":         task.Status,
		"progress":       task.Progress,
		"prompt":         task.Prompt,
		"lyrics":         task.Lyrics,
		"styleTags":      task.StyleTags,
		"duration":       task.Duration,
		"language":       task.Language,
		"vocalGender":    task.VocalGender,
		"keyMode":        task.KeyMode,
		"tempo":          task.Tempo,
		"duration_ms":    task.DurationMs,
		"mime_type":      task.MimeType,
		"bytes":          task.Bytes,
		"credits":        task.Credits,
		"task_id":        firstNonEmpty(task.UpstreamTaskID, task.ID),
		"upstream_model": task.UpstreamModel,
		"created_at":     task.CreatedAt,
		"updated_at":     task.UpdatedAt,
		"started_at":     task.StartedAt,
		"completed_at":   task.CompletedAt,
		"createdAt":      task.CreatedAt,
		"updatedAt":      task.UpdatedAt,
	}
	if task.StorageKey != "" && task.AudioURL != "" {
		result["url"] = task.AudioURL
		result["audio_url"] = task.AudioURL
		result["storageKey"] = task.StorageKey
		result["data"] = []map[string]any{{"url": task.AudioURL}}
	}
	if IsFailedMusicTaskStatus(task.Status) && (task.Error != "" || task.ErrorDetail != "") {
		result["error"] = map[string]any{"message": firstNonEmpty(task.Error, task.ErrorDetail)}
		result["error_detail"] = task.ErrorDetail
	}
	return result
}

// StartMusicTaskPoller 启动音乐任务后台轮询器。
func StartMusicTaskPoller() {
	musicTaskPollerOnce.Do(func() {
		go runMusicTaskPoller()
	})
	WakeMusicTaskPoller()
}

func WakeMusicTaskPoller() {
	musicTaskRunningMu.Lock()
	if musicTaskRunning {
		musicTaskWakePending = true
		musicTaskRunningMu.Unlock()
		return
	}
	musicTaskRunning = true
	musicTaskWakePending = false
	musicTaskRunningMu.Unlock()
	select {
	case musicTaskPollWake <- struct{}{}:
	default:
		musicTaskRunningMu.Lock()
		musicTaskRunning = false
		musicTaskRunningMu.Unlock()
	}
}

func runMusicTaskPoller() {
	inFlight := sync.Map{}
	for range musicTaskPollWake {
		for {
			tasks, err := repository.ListDueMusicTasks(musicTaskPollBatch)
			if err != nil {
				log.Printf("list due music tasks failed err=%v", err)
				waitForNextMusicTaskPoll()
				continue
			}
			if len(tasks) == 0 {
				musicTaskRunningMu.Lock()
				if musicTaskWakePending {
					musicTaskWakePending = false
					musicTaskRunningMu.Unlock()
					continue
				}
				musicTaskRunning = false
				musicTaskRunningMu.Unlock()
				break
			}
			for _, task := range tasks {
				if _, loaded := inFlight.LoadOrStore(task.ID, true); loaded {
					continue
				}
				go func(task model.MusicTask) {
					defer inFlight.Delete(task.ID)
					update, err := PollMusicTaskFromUpstream(task)
					if err != nil {
						update = MusicTaskPollUpdate{Status: task.Status, ErrorDetail: err.Error()}
					}
					if err := UpdateMusicTaskFromPoll(task, update); err != nil {
						log.Printf("update music task failed id=%s err=%v", task.ID, err)
					}
				}(task)
			}
			waitForNextMusicTaskPoll()
		}
	}
}

func waitForNextMusicTaskPoll() {
	time.Sleep(musicTaskPollInterval)
}

func UpdateMusicTaskFromPoll(task model.MusicTask, update MusicTaskPollUpdate) error {
	current := now()
	task.Status = NormalizeMusicTaskStatus(firstNonEmpty(update.Status, task.Status))
	if task.Status == "" {
		task.Status = "processing"
	}
	if update.Progress > 0 || task.Progress == 0 {
		task.Progress = clampProgress(update.Progress)
	}
	if strings.TrimSpace(update.UpstreamTaskID) != "" {
		task.UpstreamTaskID = strings.TrimSpace(update.UpstreamTaskID)
	}
	if strings.TrimSpace(update.UpstreamModel) != "" {
		task.UpstreamModel = strings.TrimSpace(update.UpstreamModel)
	}
	if update.StorageKey != "" {
		task.StorageKey = update.StorageKey
	}
	if update.MimeType != "" {
		task.MimeType = update.MimeType
	}
	if update.Bytes > 0 {
		task.Bytes = update.Bytes
	}
	if update.AudioURL != "" {
		task.AudioURL = update.AudioURL
	}
	if update.DurationMs > 0 {
		task.DurationMs = update.DurationMs
	}
	if update.Lyrics != "" {
		task.Lyrics = update.Lyrics
	}
	if update.RequestBody != "" {
		task.RequestBody = update.RequestBody
	}
	if update.ResponseBody != "" {
		task.LastResponse = update.ResponseBody
	}
	if strings.TrimSpace(update.Error) != "" {
		task.Error = strings.TrimSpace(update.Error)
	}
	if strings.TrimSpace(update.ErrorDetail) != "" {
		task.ErrorDetail = strings.TrimSpace(update.ErrorDetail)
	}
	if task.StartedAt == "" && task.Status != "queued" {
		task.StartedAt = current
	}
	task.UpdatedAt = current
	task.LastPolledAt = musicTaskTime(time.Now())
	switch {
	case task.StorageKey != "" && task.AudioURL != "":
		task.Status = "completed"
		task.Progress = 100
		task.CompletedAt = current
		task.Error = ""
		task.ErrorDetail = ""
	case IsCompletedMusicTaskStatus(task.Status):
		// 上游说完成了却没有音频，按失败处理，避免前端一直等。
		task.Status = "failed"
		task.CompletedAt = current
		if task.Error == "" {
			task.Error = "上游没有返回音频地址"
		}
	case task.Error != "" || IsFailedMusicTaskStatus(task.Status):
		task.Status = "failed"
		task.CompletedAt = current
		refundUnsubmittedMusicTask(&task)
	}
	_, err := repository.SaveMusicTask(task)
	return err
}

// refundUnsubmittedMusicTask 在「上游还没接手就失败」时返还算力点。
// 上游已经接手过的任务即使失败也不返还：上游已经产生了消耗。
// 返还成功后把 Credits 置零，保证重复写回时不会二次返还。
func refundUnsubmittedMusicTask(task *model.MusicTask) {
	if task.UpstreamTaskID != "" || task.Credits <= 0 {
		return
	}
	if err := RefundUserCredits(task.UserID, task.Model, task.Credits, "music"); err != nil {
		log.Printf("refund music task credits failed: task=%s user=%s err=%v", task.ID, task.UserID, err)
		return
	}
	task.Credits = 0
}

// storeMusicTaskAudio 把上游音频转存到自有存储。
// 上游地址（火山/腾讯 12 小时到 7 天不等）会过期，因此任务里只记录自有存储地址。
func storeMusicTaskAudio(task model.MusicTask, channel model.ModelChannel, audio musicAudioPayload) (string, string, string, int64, error) {
	data, mimeType, err := readMusicAudioBytes(channel, audio)
	if err != nil {
		return "", "", "", 0, err
	}
	filename := "music-" + task.ID + extensionForContentType(mimeType)
	uploaded, err := UploadStorageObject(musicTaskContext(task), filename, mimeType, data)
	if err != nil {
		return "", "", "", 0, err
	}
	return uploaded.StorageKey, uploaded.URL, uploaded.MimeType, uploaded.Bytes, nil
}

// readMusicAudioBytes 取出上游音频字节：可能是直链、base64 或 hex。
func readMusicAudioBytes(channel model.ModelChannel, audio musicAudioPayload) ([]byte, string, error) {
	if audio.Hex != "" {
		data, err := hex.DecodeString(strings.TrimSpace(audio.Hex))
		if err != nil {
			return nil, "", errors.New("上游返回的音频数据不是合法的 hex")
		}
		return data, musicAudioMimeType(audio.Mime, ""), nil
	}
	if audio.Base64 != "" {
		data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(audio.Base64))
		if err != nil {
			return nil, "", errors.New("上游返回的音频数据不是合法的 base64")
		}
		return data, musicAudioMimeType(audio.Mime, ""), nil
	}
	if audio.URL == "" {
		return nil, "", errors.New("上游没有返回音频内容")
	}
	request, err := http.NewRequest(http.MethodGet, audio.URL, nil)
	if err != nil {
		return nil, "", errors.New("上游音频地址无效")
	}
	SetModelChannelAuthHeader(request, channel)
	response, err := HTTPClientForChannel(channel).Do(request)
	if err != nil {
		return nil, "", fmt.Errorf("下载上游音频失败：%w", err)
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return nil, "", fmt.Errorf("下载上游音频失败：HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, musicTaskAudioLimit))
	if err != nil {
		return nil, "", fmt.Errorf("读取上游音频失败：%w", err)
	}
	return data, musicAudioMimeType(audio.Mime, response.Header.Get("Content-Type")), nil
}

func musicAudioMimeType(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(strings.Split(value, ";")[0])
		if strings.HasPrefix(value, "audio/") {
			return value
		}
	}
	return "audio/mpeg"
}

// musicTaskContext 带上任务所属用户，供存储层判断可用范围。
func musicTaskContext(task model.MusicTask) context.Context {
	ctx := context.Background()
	if user, ok, err := repository.GetUserByID(task.UserID); err == nil && ok {
		ctx = WithUser(ctx, model.PublicUser(user))
	}
	return ctx
}

func NormalizeMusicTaskStatus(status string) string {
	return NormalizeVideoTaskStatus(status)
}

func IsCompletedMusicTaskStatus(status string) bool {
	return NormalizeMusicTaskStatus(status) == "completed"
}

func IsFailedMusicTaskStatus(status string) bool {
	return NormalizeMusicTaskStatus(status) == "failed"
}

func normalizeMusicTaskMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "instrumental", "bgm":
		return "instrumental"
	default:
		return "song"
	}
}

func normalizeMusicTaskSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "canvas":
		return "canvas"
	default:
		return "music-workbench"
	}
}

func musicTaskTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}
