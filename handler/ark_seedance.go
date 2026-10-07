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

	"github.com/tigerowo/infinite-canvas/service"
)

func prepareArkSeedanceRequest(input aiProtocolRequest) (aiProtocolRequest, bool, error) {
	if !service.IsArkChannel(input.channel) || input.endpoint != "/videos" {
		return input, false, nil
	}
	input.failureLabel = "火山方舟"
	body, err := normalizeArkSeedanceVideoBody(input.body, input.modelName)
	if err != nil {
		return input, true, err
	}
	input.body = body
	input.contentType = "application/json"
	return input, true, nil
}

// arkImageContentTypes 是图片请求允许的参考图格式。
var arkImageContentTypes = map[string]string{
	"image/jpeg": "jpeg",
	"image/png":  "png",
	"image/webp": "webp",
	"image/bmp":  "bmp",
	"image/tiff": "tiff",
	"image/gif":  "gif",
	"image/heic": "heic",
	"image/heif": "heif",
}

// prepareArkImageRequest 把 OpenAI 风格的图生图请求转成方舟图片接口格式：
// 方舟没有 /images/edits，参考图要放在 /images/generations 的 JSON image 字段里。
func prepareArkImageRequest(input aiProtocolRequest) (aiProtocolRequest, bool, error) {
	if !service.IsArkChannel(input.channel) || input.endpoint != "/images/edits" {
		return input, false, nil
	}
	if !strings.HasPrefix(input.contentType, "multipart/form-data") {
		return input, false, nil
	}
	input.failureLabel = "火山方舟"
	body, err := normalizeArkImageBody(input.body, input.contentType, input.modelName)
	if err != nil {
		return input, true, err
	}
	input.body = body
	input.contentType = "application/json"
	return input, true, nil
}

func normalizeArkImageBody(body []byte, contentType string, modelName string) ([]byte, error) {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, err
	}
	form, err := multipart.NewReader(bytes.NewReader(body), params["boundary"]).ReadForm(256 << 20)
	if err != nil {
		return nil, err
	}
	defer form.RemoveAll()

	payload := map[string]any{
		"model":           firstNonEmpty(strings.TrimSpace(modelName), strings.TrimSpace(firstFormValue(form, "model"))),
		"prompt":          strings.TrimSpace(firstFormValue(form, "prompt")),
		"response_format": "url",
	}
	if references, err := arkReferenceDataURIs(form); err != nil {
		return nil, err
	} else if len(references) > 0 {
		payload["image"] = references
	}
	if size := strings.TrimSpace(firstFormValue(form, "size")); arkImageSize(size) != "" {
		payload["size"] = arkImageSize(size)
	}
	if strings.EqualFold(strings.TrimSpace(firstFormValue(form, "response_format")), "b64_json") {
		payload["response_format"] = "b64_json"
	}
	return json.Marshal(payload)
}

func firstFormValue(form *multipart.Form, key string) string {
	if values := form.Value[key]; len(values) > 0 {
		return values[0]
	}
	return ""
}

func arkImageContentType(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
}

// arkReferenceDataURIs 把上传的参考图转成方舟接受的 data URI，单张上限 30MB。
func arkReferenceDataURIs(form *multipart.Form) ([]string, error) {
	references := []string{}
	for _, headers := range form.File {
		for _, header := range headers {
			if header.Size > 30<<20 {
				return nil, errors.New("参考图不能超过 30MB")
			}
			file, err := header.Open()
			if err != nil {
				return nil, err
			}
			data, err := io.ReadAll(file)
			file.Close()
			if err != nil {
				return nil, err
			}
			if len(data) == 0 {
				continue
			}
			imageType := arkImageContentType(header.Header.Get("Content-Type"))
			if _, ok := arkImageContentTypes[imageType]; !ok {
				imageType = arkImageContentType(http.DetectContentType(data))
			}
			if _, ok := arkImageContentTypes[imageType]; !ok {
				return nil, errors.New("参考图格式不支持")
			}
			references = append(references, "data:"+imageType+";base64,"+base64.StdEncoding.EncodeToString(data))
		}
	}
	return references, nil
}

// arkImageSize 只放行方舟接受的尺寸：预设档位或宽高都为 16 倍数的显式像素。
func arkImageSize(value string) string {
	value = strings.TrimSpace(value)
	switch strings.ToUpper(value) {
	case "1K", "2K", "3K", "4K":
		return strings.ToUpper(value)
	}
	parts := strings.Split(strings.ToLower(value), "x")
	if len(parts) != 2 {
		return ""
	}
	width, widthErr := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, heightErr := strconv.Atoi(strings.TrimSpace(parts[1]))
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return ""
	}
	if width%16 != 0 || height%16 != 0 {
		return ""
	}
	return strconv.Itoa(width) + "x" + strconv.Itoa(height)
}

func normalizeArkSeedanceVideoBody(body []byte, modelName string) ([]byte, error) {
	payload := map[string]any{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	content := []any{map[string]any{"type": "text", "text": strings.TrimSpace(toStringSafe(payload["prompt"]))}}
	for _, url := range readStringSlice(payload["input_reference[]"]) {
		content = append(content, arkSeedanceReference("image", url, "reference_image"))
	}
	if url := strings.TrimSpace(toStringSafe(payload["first_frame_url"])); url != "" {
		content = append(content, arkSeedanceReference("image", url, "first_frame"))
	}
	if url := strings.TrimSpace(toStringSafe(payload["last_frame_url"])); url != "" {
		content = append(content, arkSeedanceReference("image", url, "last_frame"))
	}
	for _, url := range readStringSlice(payload["video_reference[]"]) {
		content = append(content, arkSeedanceReference("video", url, "reference_video"))
	}
	for _, url := range readStringSlice(payload["audio_reference[]"]) {
		content = append(content, arkSeedanceReference("audio", url, "reference_audio"))
	}
	result := map[string]any{
		"model":   firstNonEmpty(strings.TrimSpace(modelName), strings.TrimSpace(toStringSafe(payload["model"]))),
		"content": content,
	}
	if _, ok := payload["seconds"]; ok {
		result["duration"] = readIntPath(payload, "seconds")
	}
	if ratio := strings.TrimSpace(toStringSafe(payload["size"])); ratio != "" {
		result["ratio"] = ratio
	}
	if resolution := strings.TrimSpace(toStringSafe(payload["resolution_name"])); resolution != "" {
		result["resolution"] = resolution
	}
	if value, ok := payload["video_generate_audio"]; ok {
		result["generate_audio"] = boolLike(value)
	}
	if value, ok := payload["video_watermark"]; ok {
		result["watermark"] = boolLike(value)
	}
	return json.Marshal(result)
}

func arkSeedanceReference(kind string, url string, role string) map[string]any {
	field := kind + "_url"
	return map[string]any{
		"type": field,
		field:  map[string]any{"url": strings.TrimSpace(url)},
		"role": role,
	}
}
