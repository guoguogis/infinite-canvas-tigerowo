package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/tigerowo/infinite-canvas/model"
	"github.com/tigerowo/infinite-canvas/service"
)

// miniMaxVideoResolutionStyle 描述同一协议下各模型的分辨率写法差异。
type miniMaxVideoResolutionStyle int

const (
	// miniMaxResolutionLower 用小写分辨率，如 480p / 720p / 1080p。
	miniMaxResolutionLower miniMaxVideoResolutionStyle = iota
	// miniMaxResolutionUpper 用大写分辨率，如 480P / 512P / 768P。
	miniMaxResolutionUpper
)

// miniMaxVideoResolutions 是协议内的模型差异表；未列出的模型沿用通用小写写法。
var miniMaxVideoResolutions = map[string]miniMaxVideoResolutionStyle{
	"MiniMax-H3": miniMaxResolutionUpper,
}

// prepareMiniMaxVideoRequest 把项目内的视频请求体转成协议要求的格式：
// 提示词放进 content，时长用 duration，画幅用 ratio，分辨率用 resolution。
// 该协议下所有模型共用同一套转换，模型差异只体现在 miniMaxVideoResolutions。
func prepareMiniMaxVideoRequest(input aiProtocolRequest) (aiProtocolRequest, bool, error) {
	if !service.IsMiniMaxChannel(input.channel) || input.endpoint != "/videos" {
		return input, false, nil
	}
	payload, err := miniMaxVideoPayload(input)
	if err != nil {
		return input, true, err
	}
	body, err := buildMiniMaxVideoBody(payload, input.modelName)
	if err != nil {
		return input, true, err
	}
	input.failureLabel = "MiniMax"
	input.body = body
	input.contentType = "application/json"
	return input, true, nil
}

// miniMaxVideoPayload 取请求参数，画布带参考图时发的是 multipart，需要单独解析。
func miniMaxVideoPayload(input aiProtocolRequest) (map[string]any, error) {
	if !strings.HasPrefix(input.contentType, "multipart/form-data") {
		payload := map[string]any{}
		if err := json.Unmarshal(input.body, &payload); err != nil {
			return nil, errors.New("视频请求参数格式错误")
		}
		return payload, nil
	}
	_, params, err := mime.ParseMediaType(input.contentType)
	if err != nil {
		return nil, errors.New("视频请求参数格式错误")
	}
	form, err := multipart.NewReader(bytes.NewReader(input.body), params["boundary"]).ReadForm(256 << 20)
	if err != nil {
		return nil, errors.New("视频请求参数格式错误")
	}
	defer form.RemoveAll()
	payload := map[string]any{}
	for key, values := range form.Value {
		if len(values) > 0 {
			payload[key] = values[0]
		}
	}
	if images := miniMaxReferenceImages(form); len(images) > 0 {
		payload["images"] = images
	}
	return payload, nil
}

// miniMaxReferenceImages 收集参考图并标注协议要求的角色。
func miniMaxReferenceImages(form *multipart.Form) []any {
	images := []any{}
	seen := map[string]bool{}
	appendImage := func(key string, value string) {
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		images = append(images, map[string]any{
			"type":      "image_url",
			"image_url": map[string]any{"url": value},
			"role":      miniMaxImageRole(key),
		})
	}
	for _, field := range form.Value {
		for _, value := range field {
			if isMiniMaxImageValue(value) {
				appendImage("input_reference[]", value)
			}
		}
	}
	for key, values := range form.Value {
		if !strings.Contains(strings.ToLower(key), "image") {
			continue
		}
		for _, value := range values {
			if isMiniMaxImageValue(value) {
				appendImage(key, value)
			}
		}
	}
	for key, headers := range form.File {
		for _, header := range headers {
			if value := miniMaxUploadedImage(header); value != "" {
				appendImage(key, value)
			}
		}
	}
	return images
}

func isMiniMaxImageValue(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "data:image/")
}

func miniMaxUploadedImage(header *multipart.FileHeader) string {
	if header.Size == 0 || header.Size > 30<<20 {
		return ""
	}
	file, err := header.Open()
	if err != nil {
		return ""
	}
	data, err := io.ReadAll(file)
	file.Close()
	if err != nil || len(data) == 0 {
		return ""
	}
	mimeType := strings.ToLower(strings.TrimSpace(strings.Split(header.Header.Get("Content-Type"), ";")[0]))
	if !strings.HasPrefix(mimeType, "image/") {
		mimeType = strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(data), ";")[0]))
	}
	if !strings.HasPrefix(mimeType, "image/") {
		return ""
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// miniMaxImageRole 把项目内的图片字段映射成协议要求的角色。
func miniMaxImageRole(key string) string {
	lower := strings.ToLower(strings.TrimSpace(key))
	switch {
	case strings.Contains(lower, "last"):
		return "last_frame"
	case strings.Contains(lower, "first"):
		return "first_frame"
	default:
		return "reference_image"
	}
}

// buildMiniMaxVideoBody 生成协议请求体。直连时前端已经按协议拼好了 content（含首尾帧与参考素材的角色），
// 这种情况直接沿用；只有项目内部格式（prompt + images）才需要在这里拼装。
func buildMiniMaxVideoBody(payload map[string]any, modelName string) ([]byte, error) {
	model := strings.TrimSpace(firstNonEmpty(modelName, toStringSafe(payload["model"])))
	content, _ := payload["content"].([]any)
	if len(content) == 0 {
		if text := strings.TrimSpace(toStringSafe(payload["prompt"])); text != "" {
			content = append(content, map[string]any{"type": "text", "text": text})
		}
		if images, ok := payload["images"].([]any); ok {
			content = append(content, images...)
		}
	}

	result := map[string]any{}
	if model != "" {
		result["model"] = model
	}
	if seconds := miniMaxVideoSeconds(payload); seconds != "" {
		result["duration"] = seconds
	}
	if ratio := miniMaxVideoRatio(payload); ratio != "" {
		result["ratio"] = ratio
	}
	if resolution := miniMaxVideoResolution(payload, model); resolution != "" {
		result["resolution"] = resolution
	}
	if len(content) > 0 {
		result["content"] = content
	}
	return json.Marshal(result)
}

// miniMaxVideoSeconds 取时长，只保留 1-15 的整数（更细的下限交给上游报错）。
func miniMaxVideoSeconds(payload map[string]any) string {
	value := strings.TrimSpace(firstNonEmpty(toStringSafe(payload["duration"]), toStringSafe(payload["seconds"])))
	value = strings.TrimSuffix(strings.ToLower(value), "s")
	if value == "" {
		return ""
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 && seconds <= 15 {
		return strconv.Itoa(seconds)
	}
	return ""
}

// miniMaxVideoResolution 统一分辨率字段名，并按模型差异表决定大小写。
func miniMaxVideoResolution(payload map[string]any, model string) string {
	value := strings.TrimSpace(firstNonEmpty(toStringSafe(payload["resolution"]), toStringSafe(payload["resolution_name"]), toStringSafe(payload["vquality"])))
	if value == "" {
		return ""
	}
	if miniMaxVideoResolutions[strings.TrimSpace(model)] == miniMaxResolutionUpper {
		return strings.ToUpper(value)
	}
	return strings.ToLower(value)
}

// miniMaxVideoRatio 取画幅，接受 宽:高 形式或协议允许的 adaptive（首尾帧模式必须用它）。
func miniMaxVideoRatio(payload map[string]any) string {
	value := strings.TrimSpace(firstNonEmpty(toStringSafe(payload["ratio"]), toStringSafe(payload["size"])))
	if strings.EqualFold(value, "adaptive") {
		return "adaptive"
	}
	if strings.Contains(value, ":") {
		return value
	}
	return ""
}

// prepareMiniMaxVideoRequest 把 metaso 的查询响应解包：上游统一用 task 包裹任务对象，
// 未解包时状态与视频地址都读不到。
func prepareMiniMaxVideoResponse(payload []byte, request *http.Request, channel model.ModelChannel) ([]byte, bool) {
	if !service.IsMiniMaxChannel(channel) || !strings.Contains(request.URL.Path, "/v2/query/video_generation/") {
		return nil, false
	}
	return transformMiniMaxVideoTaskResponse(payload)
}

func transformMiniMaxVideoTaskResponse(payload []byte) ([]byte, bool) {
	var root struct {
		Task map[string]any `json:"task"`
	}
	if json.Unmarshal(payload, &root) != nil || root.Task == nil {
		return nil, false
	}
	root.Task["task_id"] = readStringPath(root.Task, "id")
	root.Task["video_url"] = readStringPath(root.Task, "content.url")
	root.Task["size"] = readStringPath(root.Task, "resolution")
	transformed, err := json.Marshal(root.Task)
	return transformed, err == nil
}
