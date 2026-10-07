package handler

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/tigerowo/infinite-canvas/model"
	"github.com/tigerowo/infinite-canvas/service"
)

// 豆包语音合成走独立的语音服务域名，请求体是 req_params，音色与格式放在 audio_params；
// 响应是逐行 JSON，音频以 base64 分片返回，最后一行的 code 为结束标记。
// TokenPlan 与后付费只在 baseUrl 上区分：分别是 .../api/v3/plan 与 .../api/v3，协议内路径相同。
const (
	doubaoTTsPath         = "/tts/unidirectional"
	doubaoTTsSampleRate   = 24000
	doubaoTTsStreamDone   = 20000000
	doubaoTTsMaxLineBytes = 16 << 20
)

// prepareDoubaoTTsRequest 把项目内的音频请求体转成协议要求的 req_params。
func prepareDoubaoTTsRequest(input aiProtocolRequest) (aiProtocolRequest, bool, error) {
	if !service.IsDoubaoTTsChannel(input.channel) || input.endpoint != "/audio/speech" {
		return input, false, nil
	}
	var payload struct {
		Input          string `json:"input"`
		Voice          string `json:"voice"`
		ResponseFormat string `json:"response_format"`
	}
	if json.Unmarshal(input.body, &payload) != nil {
		return input, true, errors.New("音频请求参数格式错误")
	}
	text := strings.TrimSpace(payload.Input)
	if text == "" {
		return input, true, errors.New("缺少要合成的文本")
	}
	speaker := service.DoubaoTTsSpeaker(payload.Voice)
	body, err := json.Marshal(map[string]any{
		"req_params": map[string]any{
			"text":    text,
			"speaker": speaker,
			"audio_params": map[string]any{
				"format":      doubaoTTsAudioFormat(payload.ResponseFormat),
				"sample_rate": doubaoTTsSampleRate,
			},
		},
	})
	if err != nil {
		return input, true, err
	}
	input.failureLabel = "豆包语音合成"
	input.body = body
	input.contentType = "application/json"
	input.headers = map[string]string{
		"X-Api-Resource-Id":                     service.DoubaoTTsResourceID(speaker),
		"X-Control-Require-Usage-Tokens-Return": "*",
	}
	return input, true, nil
}

func doubaoTTsAudioFormat(value string) string {
	format := strings.ToLower(strings.TrimSpace(value))
	switch format {
	case "mp3", "wav", "pcm", "ogg_opus":
		return format
	case "opus":
		return "ogg_opus"
	}
	return "mp3"
}

// copyDoubaoTTsResponse 把逐行 JSON 的音频分片拼成完整音频再返回，前端与画布音频任务都按音频字节处理。
func copyDoubaoTTsResponse(w http.ResponseWriter, response *http.Response, request *http.Request, channel model.ModelChannel, context aiLogContext, onFailure func()) bool {
	if !service.IsDoubaoTTsChannel(channel) || !strings.Contains(request.URL.Path, doubaoTTsPath) {
		return false
	}
	audio, err := readDoubaoTTsAudio(response.Body)
	if err != nil {
		if onFailure != nil {
			onFailure()
		}
		saveAIProxyLog(context, http.StatusBadGateway, "", err.Error())
		Fail(w, err.Error())
		return true
	}
	w.Header().Set("Content-Type", doubaoTTsContentType(audio))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(audio)
	saveAIProxyLog(context, http.StatusOK, "[binary audio]", "")
	return true
}

func readDoubaoTTsAudio(body io.Reader) ([]byte, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), doubaoTTsMaxLineBytes)
	audio := []byte{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var chunk struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Msg     string `json:"msg"`
			Data    string `json:"data"`
		}
		if json.Unmarshal([]byte(line), &chunk) != nil {
			return nil, errors.New("豆包语音合成返回了无法解析的数据")
		}
		if chunk.Code == doubaoTTsStreamDone {
			break
		}
		if chunk.Code != 0 {
			return nil, errors.New(firstNonEmpty(chunk.Message, chunk.Msg, "豆包语音合成失败"))
		}
		if chunk.Data == "" {
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(chunk.Data)
		if err != nil {
			return nil, errors.New("豆包语音合成返回的音频数据无法解码")
		}
		audio = append(audio, decoded...)
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.New("读取豆包语音合成响应失败")
	}
	if len(audio) == 0 {
		return nil, errors.New("豆包语音合成没有返回音频数据")
	}
	return audio, nil
}

func doubaoTTsContentType(audio []byte) string {
	detected := strings.TrimSpace(strings.Split(http.DetectContentType(audio), ";")[0])
	if strings.HasPrefix(detected, "audio/") {
		return detected
	}
	return "audio/mpeg"
}
