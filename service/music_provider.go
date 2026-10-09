package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/tigerowo/infinite-canvas/model"
)

// 音乐模型渠道协议。音乐模型的能力值用 music，与 TTS 的 audio 分开。
const (
	ModelChannelProtocolVolcMusic     = "volc-music"
	ModelChannelProtocolTokenHubMusic = "tokenhub-music"
	ModelChannelProtocolMinimaxMusic  = "minimax-music"
)

const (
	// 火山引擎 AI 音乐生成（豆包音乐）走 API Gateway，Action 通过查询参数指定。
	volcMusicAPIVersion   = "2024-08-12"
	volcMusicServiceName  = "imagination"
	volcMusicSongVersion  = "v4.3"
	volcMusicBGMVersion   = "v5.0"
	volcMusicAudioFormat  = "mp3"
	volcMusicSongMinSec   = 30
	volcMusicSongMaxSec   = 240
	volcMusicBGMMinSec    = 30
	volcMusicBGMMaxSec    = 120
	volcMusicActionLyrics = "GenLyrics"
	volcMusicActionQuery  = "QuerySong"
	// 预付费资源包对应的 Action 是 GenSongV4 / GenBGM，这里默认走后付费，
	// 换预付费只需把下面两个常量改成 GenSongV4 / GenBGM。
	volcMusicActionSong = "GenSongForTime"
	volcMusicActionBGM  = "GenBGMForTime"
)

// musicAudioPayload 承载上游返回的音频，三者之一。
type musicAudioPayload struct {
	URL    string
	Base64 string
	Hex    string
	Mime   string
}

func (payload musicAudioPayload) Empty() bool {
	return payload.URL == "" && payload.Base64 == "" && payload.Hex == ""
}

// MusicTaskGenerateInput 是适配器需要的生成参数。
type MusicTaskGenerateInput struct {
	Model       string
	Mode        string
	Title       string
	Prompt      string
	Lyrics      string
	StyleTags   []string
	Duration    int
	Language    string
	VocalGender string
	KeyMode     string
	Tempo       int
}

type MusicLyricsInput struct {
	Model         string
	Prompt        string
	Genre         string
	Mood          string
	VocalGender   string
	ChannelID     string
	UserChannelID string
}

// MusicProviderResult 是一次上游调用的结果。
type MusicProviderResult struct {
	Progress       int
	Status         string
	UpstreamTaskID string
	UpstreamModel  string
	AudioURL       string
	AudioBase64    string
	AudioHex       string
	AudioMime      string
	DurationMs     int
	Lyrics         string
	RequestBody    string
	ResponseBody   string
	Error          string
	ErrorDetail    string
}

func (result MusicProviderResult) audio() musicAudioPayload {
	return musicAudioPayload{URL: result.AudioURL, Base64: result.AudioBase64, Hex: result.AudioHex, Mime: result.AudioMime}
}

// MusicProvider 是单家音乐渠道的协议适配。
// 同步返回的上游（腾讯云 TokenHub 一类）在 Submit 里就直接带出音频，无需轮询。
type MusicProvider interface {
	Submit(channel model.ModelChannel, input MusicTaskGenerateInput) (MusicProviderResult, error)
	Poll(channel model.ModelChannel, task model.MusicTask) (MusicProviderResult, error)
	Lyrics(channel model.ModelChannel, input MusicLyricsInput) (string, error)
}

func musicProviderForChannel(channel model.ModelChannel) (MusicProvider, error) {
	switch strings.ToLower(strings.TrimSpace(channel.Protocol)) {
	case ModelChannelProtocolVolcMusic:
		return volcMusicProvider{}, nil
	case ModelChannelProtocolTokenHubMusic:
		return tokenHubMusicProvider{}, nil
	case ModelChannelProtocolMinimaxMusic:
		return minimaxMusicProvider{}, nil
	default:
		return nil, fmt.Errorf("渠道 %s 的协议 %s 暂不支持音乐生成", channel.Name, channel.Protocol)
	}
}

// SupportsMusicChannel 判断渠道协议是否支持音乐生成。
func SupportsMusicChannel(channel model.ModelChannel) bool {
	_, err := musicProviderForChannel(channel)
	return err == nil
}

// PollMusicTaskFromUpstream 推进一条音乐任务：没有上游任务 ID 时先提交，之后轮询。
// 提交失败视为终态失败并把原因写进任务；轮询失败视为瞬时错误，留给下一轮重试。
func PollMusicTaskFromUpstream(task model.MusicTask) (MusicTaskPollUpdate, error) {
	channel, err := resolveMusicChannel(task.UserID, task.Model, task.UserChannelID, task.ChannelID)
	if err != nil {
		return MusicTaskPollUpdate{}, err
	}
	provider, err := musicProviderForChannel(channel)
	if err != nil {
		return MusicTaskPollUpdate{Status: "failed", Error: err.Error(), ErrorDetail: err.Error()}, nil
	}
	submitting := strings.TrimSpace(task.UpstreamTaskID) == ""
	var result MusicProviderResult
	if submitting {
		result, err = provider.Submit(channel, musicGenerateInputFromTask(task))
	} else {
		result, err = provider.Poll(channel, task)
	}
	if err != nil {
		if submitting {
			return MusicTaskPollUpdate{Status: "failed", Error: err.Error(), ErrorDetail: err.Error()}, nil
		}
		return MusicTaskPollUpdate{}, err
	}
	update := MusicTaskPollUpdate{
		Status:         result.Status,
		Progress:       result.Progress,
		UpstreamTaskID: result.UpstreamTaskID,
		UpstreamModel:  result.UpstreamModel,
		DurationMs:     result.DurationMs,
		Lyrics:         result.Lyrics,
		RequestBody:    result.RequestBody,
		ResponseBody:   result.ResponseBody,
		Error:          result.Error,
		ErrorDetail:    result.ErrorDetail,
	}
	audio := result.audio()
	if audio.Empty() {
		return update, nil
	}
	storageKey, audioURL, mimeType, size, err := storeMusicTaskAudio(task, channel, audio)
	if err != nil {
		return MusicTaskPollUpdate{
			Status:       "failed",
			ResponseBody: result.ResponseBody,
			Error:        "音频转存到自有存储失败",
			ErrorDetail:  err.Error(),
		}, nil
	}
	update.StorageKey = storageKey
	update.AudioURL = audioURL
	update.MimeType = mimeType
	update.Bytes = size
	if update.Status == "" || !IsCompletedMusicTaskStatus(update.Status) {
		update.Status = "completed"
		update.Progress = 100
	}
	return update, nil
}

// GenerateMusicLyrics 生成歌词，目前火山引擎与 MiniMax 官方提供独立歌词接口。
func GenerateMusicLyrics(userID string, input MusicLyricsInput) (string, error) {
	channel, err := resolveMusicChannel(userID, input.Model, input.UserChannelID, input.ChannelID)
	if err != nil {
		return "", err
	}
	provider, err := musicProviderForChannel(channel)
	if err != nil {
		return "", err
	}
	return provider.Lyrics(channel, input)
}

func musicGenerateInputFromTask(task model.MusicTask) MusicTaskGenerateInput {
	return MusicTaskGenerateInput{
		Model:       task.Model,
		Mode:        task.Mode,
		Title:       task.Title,
		Prompt:      task.Prompt,
		Lyrics:      task.Lyrics,
		StyleTags:   task.StyleTags,
		Duration:    task.Duration,
		Language:    task.Language,
		VocalGender: task.VocalGender,
		KeyMode:     task.KeyMode,
		Tempo:       task.Tempo,
	}
}

func resolveMusicChannel(userID string, modelName string, userChannelID string, channelID string) (model.ModelChannel, error) {
	if strings.TrimSpace(userChannelID) != "" {
		return SelectUserLocalModelChannelForModel(userID, modelName, userChannelID)
	}
	return SelectModelChannelForModel(modelName, channelID, false)
}

// volcMusicProvider 适配火山引擎 AI 音乐生成（豆包音乐）。
type volcMusicProvider struct{}

func (volcMusicProvider) Submit(channel model.ModelChannel, input MusicTaskGenerateInput) (MusicProviderResult, error) {
	var action string
	var body map[string]any
	if input.Mode == "instrumental" {
		action = volcMusicActionBGM
		body = map[string]any{
			"Text":    firstNonEmpty(input.Prompt, input.Title),
			"Version": volcMusicBGMVersion,
		}
		if duration := clampSeconds(input.Duration, volcMusicBGMMinSec, volcMusicBGMMaxSec); duration > 0 {
			body["Duration"] = duration
		}
	} else {
		action = volcMusicActionSong
		// 上游的 Mood / Scene / Instrument / Timbre 没有对应的任务字段，
		// 这些信息由用户在风格描述里自由填写，这里只映射结构化的字段。
		body = map[string]any{
			"ModelVersion": volcMusicSongVersion,
			"Prompt":       input.Prompt,
			"Lyrics":       input.Lyrics,
			"VodFormat":    volcMusicAudioFormat,
		}
		if duration := clampSeconds(input.Duration, volcMusicSongMinSec, volcMusicSongMaxSec); duration > 0 {
			body["Duration"] = duration
		}
		if len(input.StyleTags) > 0 {
			body["Genre"] = input.StyleTags[0]
		}
		if len(input.StyleTags) > 1 {
			extra := input.StyleTags[1:]
			if len(extra) > 2 {
				extra = extra[:2]
			}
			body["GenreExtra"] = strings.Join(extra, ",")
		}
		if gender := volcMusicGender(input.VocalGender); gender != "" {
			body["Gender"] = gender
		}
		if kmode := volcMusicKeyMode(input.KeyMode); kmode != "" {
			body["Kmode"] = kmode
		}
		if input.Tempo > 0 {
			body["Tempo"] = input.Tempo
		}
		if lang := volcMusicLanguage(input.Language); lang != "" {
			body["Lang"] = lang
		}
	}
	payload, err := volcMusicRequest(channel, action, body)
	if err != nil {
		return MusicProviderResult{}, err
	}
	taskID := musicStringPath(payload, "Result.TaskID")
	if taskID == "" {
		return MusicProviderResult{}, fmt.Errorf("豆包音乐没有返回任务 ID：%s", musicErrorText(payload))
	}
	version := volcMusicSongVersion
	if action == volcMusicActionBGM {
		version = volcMusicBGMVersion
	}
	return MusicProviderResult{
		Status:         "processing",
		Progress:       50,
		UpstreamTaskID: taskID,
		UpstreamModel:  "doubao-music/" + version,
		RequestBody:    musicRequestBody(body),
		ResponseBody:   string(payload),
	}, nil
}

func (volcMusicProvider) Poll(channel model.ModelChannel, task model.MusicTask) (MusicProviderResult, error) {
	payload, err := volcMusicRequest(channel, volcMusicActionQuery, map[string]any{"TaskID": task.UpstreamTaskID})
	if err != nil {
		return MusicProviderResult{}, err
	}
	result := MusicProviderResult{ResponseBody: string(payload), UpstreamModel: task.UpstreamModel}
	switch musicIntPath(payload, "Result.Status") {
	case 0:
		result.Status = "queued"
	case 1:
		result.Status = "processing"
		result.Progress = 50
	case 2:
		result.Status = "completed"
		result.Progress = 100
		result.AudioURL = musicStringPath(payload, "Result.SongDetail.AudioUrl")
		result.Lyrics = musicStringPath(payload, "Result.SongDetail.Lyrics")
		// 上游返回的 Duration 单位是秒，与请求参数一致。
		result.DurationMs = musicIntPath(payload, "Result.SongDetail.Duration") * 1000
		if result.AudioURL == "" {
			result.Status = "failed"
			result.Error = "豆包音乐返回完成但没有音频地址"
		}
	case 3:
		result.Status = "failed"
		result.Error = firstNonEmpty(
			musicStringPath(payload, "Result.FailureReason.Msg"),
			musicStringPath(payload, "Result.FailureReason.Message"),
			musicErrorText(payload),
			"豆包音乐生成失败",
		)
		result.ErrorDetail = string(payload)
	default:
		result.Status = "processing"
	}
	return result, nil
}

func (volcMusicProvider) Lyrics(channel model.ModelChannel, input MusicLyricsInput) (string, error) {
	body := map[string]any{
		"ModelVersion": volcMusicSongVersion,
		"Prompt":       input.Prompt,
	}
	if input.Genre != "" {
		body["Genre"] = input.Genre
	}
	if input.Mood != "" {
		body["Mood"] = input.Mood
	}
	if gender := volcMusicGender(input.VocalGender); gender != "" {
		body["Gender"] = gender
	}
	payload, err := volcMusicRequest(channel, volcMusicActionLyrics, body)
	if err != nil {
		return "", err
	}
	lyrics := musicStringPath(payload, "Result.Lyrics")
	if strings.TrimSpace(lyrics) == "" {
		return "", fmt.Errorf("豆包音乐没有返回歌词：%s", musicErrorText(payload))
	}
	return lyrics, nil
}

func volcMusicRequest(channel model.ModelChannel, action string, body map[string]any) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	address := strings.TrimRight(strings.TrimSpace(channel.BaseURL), "/") + "/?Action=" + action + "&Version=" + volcMusicAPIVersion
	request, err := http.NewRequest(http.MethodPost, address, bytes.NewReader(payload))
	if err != nil {
		return nil, errors.New("音乐渠道地址无效")
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	// 火山引擎走 API Gateway 的 Bearer 鉴权；AK/SK 的 HMAC-SHA256 模式未实现。
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(channel.APIKey))
	request.Header.Set("ServiceName", volcMusicServiceName)
	response, err := HTTPClientForChannel(channel).Do(request)
	if err != nil {
		return nil, fmt.Errorf("调用豆包音乐失败：%w", err)
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("调用豆包音乐失败：HTTP %d %s", response.StatusCode, musicErrorText(data))
	}
	return data, nil
}

// 音乐渠道的完整路径。渠道里可能只填域名，也可能填完整地址；
// 走第三方中转时路径和官方可能完全不同，此时按原样使用。
const (
	tokenHubMusicPath = "/v1/wand/minimax-music/generation"
	minimaxMusicPath  = "/v1/music_generation"
	minimaxLyricsPath = "/v1/lyrics_generation"
)

// 腾讯云 TokenHub 转售的是 MiniMax 形状的接口，与 MiniMax 官方的错误文案不同。
const (
	tokenHubMusicLabel = "腾讯云 TokenHub"
	minimaxMusicLabel  = "MiniMax"
)

// musicChannelURL 补全音乐渠道的请求地址：只填域名时拼上默认路径，
// 只填到 /v1 时补上默认路径的剩余部分，已带其它路径时原样使用。
func musicChannelURL(baseURL string, defaultPath string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return ""
	}
	host := base
	if parts := strings.SplitN(base, "://", 2); len(parts) == 2 {
		host = parts[1]
	}
	switch {
	case !strings.Contains(host, "/"):
		return base + defaultPath
	case strings.HasSuffix(strings.ToLower(base), "/v1"):
		return base + strings.TrimPrefix(defaultPath, "/v1")
	default:
		return base
	}
}

// musicSafeBody 截断响应体。TokenHub 会把整段音频以 hex 内联在响应里，
// 原样写进任务的 response_body 会单条几十 MB。
func musicSafeBody(payload []byte) string {
	text := strings.TrimSpace(string(payload))
	if len(text) > 2000 {
		return text[:2000] + "...(truncated)"
	}
	return text
}

// tokenHubMusicProvider 适配腾讯云 TokenHub 转售的音乐模型（MiniMax / Mureka）。
type tokenHubMusicProvider struct{}

func (tokenHubMusicProvider) Submit(channel model.ModelChannel, input MusicTaskGenerateInput) (MusicProviderResult, error) {
	return minimaxShapeMusicSubmit(channel, tokenHubMusicLabel, tokenHubMusicPath, input)
}

func (tokenHubMusicProvider) Poll(channel model.ModelChannel, task model.MusicTask) (MusicProviderResult, error) {
	// 腾讯云 TokenHub 是同步返回，正常流程不会走到轮询。
	return MusicProviderResult{}, errors.New("该音乐渠道同步返回结果，无需轮询")
}

func (tokenHubMusicProvider) Lyrics(channel model.ModelChannel, input MusicLyricsInput) (string, error) {
	return "", errors.New("该音乐渠道不支持歌词生成")
}

// minimaxMusicProvider 适配 MiniMax 官方音乐生成（POST /v1/music_generation）。
//
// 官方服务调整通知：自 2026 年 8 月 20 日起，付费接口（音乐生成、歌词生成）不再面向新用户提供服务，
// 历史付费用户可继续使用现有 API 服务；免费音乐生成接口（music-3.0-free / music-2.6-free /
// music-cover-free）停止服务。也就是说新账号可能拿不到权限，这是上游策略，不是本地渠道配置问题。
type minimaxMusicProvider struct{}

func (minimaxMusicProvider) Submit(channel model.ModelChannel, input MusicTaskGenerateInput) (MusicProviderResult, error) {
	// 官方接口与腾讯云 TokenHub 转售同构，共用同一套请求与解析。
	return minimaxShapeMusicSubmit(channel, minimaxMusicLabel, minimaxMusicPath, input)
}

func (minimaxMusicProvider) Poll(channel model.ModelChannel, task model.MusicTask) (MusicProviderResult, error) {
	// MiniMax 音乐生成同步返回结果，正常流程不会走到轮询。
	return MusicProviderResult{}, errors.New("该音乐渠道同步返回结果，无需轮询")
}

// Lyrics 走 MiniMax 官方歌词生成接口 POST /v1/lyrics_generation：
// mode 必填（write_full_song 写完整歌曲 / edit 编辑续写），歌词在响应顶层的 lyrics 字段。
func (minimaxMusicProvider) Lyrics(channel model.ModelChannel, input MusicLyricsInput) (string, error) {
	parts := make([]string, 0, 4)
	for _, value := range []string{input.Prompt, input.Genre, input.Mood, input.VocalGender} {
		if value = strings.TrimSpace(value); value != "" {
			parts = append(parts, value)
		}
	}
	body := map[string]any{"mode": "write_full_song", "prompt": strings.Join(parts, ", ")}
	payload, err := musicChannelRequest(channel, minimaxMusicLabel, minimaxLyricsPath, body)
	if err != nil {
		return "", err
	}
	if code := musicIntPath(payload, "base_resp.status_code"); code != 0 {
		message := firstNonEmpty(musicStringPath(payload, "base_resp.status_msg"), "MiniMax 歌词生成失败")
		return "", fmt.Errorf("%s（base_resp.status_code=%d）", message, code)
	}
	lyrics := musicStringPath(payload, "lyrics")
	if lyrics == "" {
		return "", fmt.Errorf("MiniMax 没有返回歌词：%s", musicErrorText(payload))
	}
	return lyrics, nil
}

// minimaxShapeMusicSubmit 处理 MiniMax 形状的音乐生成，官方接口与腾讯云 TokenHub 转售共用：
// 两者都同步返回、用 base_resp.status_code 表达业务错误、用 data.audio 承载音频、用 data.status 表示合成状态。
func minimaxShapeMusicSubmit(channel model.ModelChannel, label string, defaultPath string, input MusicTaskGenerateInput) (MusicProviderResult, error) {
	// output_format 用 url：实测腾讯云 TokenHub 在 hex 模式下不返回音频（data.audio 为空），
	// 而 url 模式正常。上游 url 有效期 24 小时，本服务拿到后立即转存自有存储，不存在过期风险。
	body := map[string]any{
		"model":           input.Model,
		"prompt":          input.Prompt,
		"is_instrumental": input.Mode == "instrumental",
		"output_format":   "url",
	}
	if input.Lyrics != "" {
		body["lyrics"] = input.Lyrics
	}
	payload, err := musicChannelRequest(channel, label, defaultPath, body)
	if err != nil {
		return MusicProviderResult{}, err
	}
	summary := musicSafeBody(payload)
	// 业务错误放在 body 的 base_resp.status_code 里：0 成功，1002 限流，1004/2049 鉴权失败，
	// 1008 余额不足，1026 敏感内容，2013 参数异常；非 0 视为失败并返还算力点。
	if code := musicIntPath(payload, "base_resp.status_code"); code != 0 {
		message := firstNonEmpty(musicStringPath(payload, "base_resp.status_msg"), label+" 返回错误")
		return MusicProviderResult{
			Status:       "failed",
			Error:        message,
			ErrorDetail:  fmt.Sprintf("%s（base_resp.status_code=%d）", message, code),
			RequestBody:  musicRequestBody(body),
			ResponseBody: summary,
		}, nil
	}
	// MiniMax 官方返回 data.audio（hex 或 url）；腾讯云文档只给出请求参数与计费口径，
	// 这里按同源形状读取，读不到就报错并把原始响应带上。
	audio := musicAudioPayloadFrom(firstNonEmpty(
		musicStringPath(payload, "data.audio"),
		musicStringPath(payload, "data.url"),
		musicStringPath(payload, "audio"),
		musicStringPath(payload, "url"),
	))
	if audio.Empty() {
		return MusicProviderResult{}, fmt.Errorf("%s 没有返回音频：%s", label, musicErrorText(payload))
	}
	// data.status：1 合成中、2 已完成。同步接口没有可轮询的任务 ID，
	// 非完成态不能置为成功，否则会白扣算力点。
	if status := musicIntPath(payload, "data.status"); status != 0 && status != 2 {
		return MusicProviderResult{
			Status:       "failed",
			Error:        fmt.Sprintf("%s 返回非完成状态（data.status=%d）", label, status),
			ErrorDetail:  musicErrorText(payload),
			RequestBody:  musicRequestBody(body),
			ResponseBody: summary,
		}, nil
	}
	result := MusicProviderResult{
		Status:       "completed",
		Progress:     100,
		RequestBody:  musicRequestBody(body),
		ResponseBody: summary,
	}
	result.AudioURL = audio.URL
	result.AudioBase64 = audio.Base64
	result.AudioHex = audio.Hex
	return result, nil
}

// musicChannelRequest 按渠道 BaseURL 推导出请求地址，发 JSON POST 并返回响应体。
func musicChannelRequest(channel model.ModelChannel, label string, defaultPath string, body map[string]any) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	address := musicChannelURL(channel.BaseURL, defaultPath)
	if address == "" {
		return nil, errors.New("音乐渠道地址无效")
	}
	request, err := http.NewRequest(http.MethodPost, address, bytes.NewReader(payload))
	if err != nil {
		return nil, errors.New("音乐渠道地址无效")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(channel.APIKey))
	response, err := HTTPClientForChannel(channel).Do(request)
	if err != nil {
		return nil, fmt.Errorf("调用%s失败：%w", label, err)
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 128<<20))
	if response.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("调用%s失败：HTTP %d %s", label, response.StatusCode, musicErrorText(data))
	}
	return data, nil
}

func volcMusicGender(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "female", "女声", "女":
		return "Female"
	case "male", "男声", "男":
		return "Male"
	default:
		return ""
	}
}

func volcMusicKeyMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "major", "大调":
		return "Major"
	case "minor", "小调":
		return "Minor"
	default:
		return ""
	}
}

func volcMusicLanguage(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "chinese", "中文", "zh":
		return "Chinese"
	case "english", "英文", "en":
		return "English"
	default:
		return ""
	}
}

func clampSeconds(value int, min int, max int) int {
	if value <= 0 {
		return 0
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// musicAudioPayloadFrom 识别上游音频字段：直链、data URL、hex 或 base64。
func musicAudioPayloadFrom(value string) musicAudioPayload {
	value = strings.TrimSpace(value)
	switch {
	case value == "":
		return musicAudioPayload{}
	case strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://"):
		return musicAudioPayload{URL: value}
	case strings.HasPrefix(value, "data:"):
		index := strings.Index(value, ";base64,")
		if index < 0 {
			return musicAudioPayload{}
		}
		return musicAudioPayload{Base64: value[index+len(";base64,"):], Mime: strings.TrimPrefix(value[:index], "data:")}
	case isMusicHexString(value):
		return musicAudioPayload{Hex: value}
	default:
		return musicAudioPayload{Base64: value}
	}
}

// isMusicHexString 判断是不是一段音频的 hex：必须是长且偶数的纯 hex 字符串。
func isMusicHexString(value string) bool {
	if len(value) < 64 || len(value)%2 != 0 {
		return false
	}
	for _, char := range value {
		if !strings.ContainsRune("0123456789abcdefABCDEF", char) {
			return false
		}
	}
	return true
}

// musicErrorText 从上游响应里尽量取出一条可读的错误信息。
func musicErrorText(payload []byte) string {
	message := firstNonEmpty(
		musicStringPath(payload, "ResponseMetadata.Error.Message"),
		musicStringPath(payload, "Result.FailureReason.Msg"),
		musicStringPath(payload, "base_resp.status_msg"),
		musicStringPath(payload, "error.message"),
		musicStringPath(payload, "error"),
		musicStringPath(payload, "message"),
		musicStringPath(payload, "msg"),
	)
	if message != "" {
		return message
	}
	text := strings.TrimSpace(string(payload))
	if len(text) > 300 {
		text = text[:300]
	}
	return text
}

func musicRequestBody(body map[string]any) string {
	payload, err := json.Marshal(body)
	if err != nil {
		return ""
	}
	return string(payload)
}

func musicStringPath(payload []byte, path string) string {
	value, ok := musicPathValue(payload, path)
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func musicIntPath(payload []byte, path string) int {
	value, ok := musicPathValue(payload, path)
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case json.Number:
		number, _ := typed.Int64()
		return int(number)
	case string:
		number, _ := strconv.Atoi(strings.TrimSpace(typed))
		return number
	default:
		return 0
	}
}

func musicPathValue(payload []byte, path string) (any, bool) {
	var root any
	if len(payload) == 0 || json.Unmarshal(payload, &root) != nil {
		return nil, false
	}
	current := root
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}
