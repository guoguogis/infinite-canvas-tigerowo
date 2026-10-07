package service

import (
	"sort"
	"strings"

	"github.com/tigerowo/infinite-canvas/model"
)

// 豆包语音合成走独立的语音服务域名（openspeech），因此单独成一个协议；密钥可以用方舟 Agent Plan 的 key。
const (
	// DoubaoTTsDefaultResourceID 是 X-Api-Resource-Id 的兜底值。
	DoubaoTTsDefaultResourceID = "seed-tts-2.0"
	// DoubaoTTsDefaultSpeaker 是音色的兜底值。
	DoubaoTTsDefaultSpeaker = "zh_female_shaoergushi_uranus_bigtts"
)

// doubaoTTsModelNames 是协议内可选的语音合成模型名。
var doubaoTTsModelNames = []string{"doubao-seed-tts-2.0", "seed-tts-2.0"}

// openAIVoiceNames 是 OpenAI 音色取值，对豆包无意义，命中时需要回退到豆包默认音色。
var openAIVoiceNames = map[string]bool{
	"alloy": true, "ash": true, "ballad": true, "coral": true, "echo": true, "fable": true,
	"nova": true, "onyx": true, "sage": true, "shimmer": true, "verse": true, "marin": true, "cedar": true,
}

func IsDoubaoTTsChannel(channel model.ModelChannel) bool {
	return strings.EqualFold(strings.TrimSpace(channel.Protocol), ModelChannelProtocolDoubaoTTs)
}

// DoubaoTTsModels 返回语音合成模型名，供渠道配置拉取。
func DoubaoTTsModels() []string {
	names := append([]string(nil), doubaoTTsModelNames...)
	sort.Strings(names)
	return names
}

// DoubaoTTsSpeaker 取最终音色：显式音色（含私有复刻音色）优先，OpenAI 音色名与空值回退默认音色。
func DoubaoTTsSpeaker(voice string) string {
	speaker := strings.TrimSpace(voice)
	if speaker == "" || openAIVoiceNames[strings.ToLower(speaker)] {
		return DoubaoTTsDefaultSpeaker
	}
	return speaker
}

// DoubaoTTsResourceID 按音色 ID 推导 X-Api-Resource-Id。不同代次与复刻音色的资源 ID 不同，
// 传错时上游返回 HTTP 200 + code 55000000 的空音频。
func DoubaoTTsResourceID(voice string) string {
	speaker := strings.TrimSpace(voice)
	switch {
	case strings.HasPrefix(speaker, "S_"):
		// 个人声音复刻音色。
		return "seed-icl-2.0"
	case strings.Contains(speaker, "ICL_uranus_"):
		return "seed-icl-2.0"
	case strings.Contains(speaker, "uranus"):
		// 语音合成 2.0 标准音色。
		return "seed-tts-2.0"
	case strings.HasSuffix(speaker, "_bigtts"):
		// 1.0 代音色。
		return "volc.service_type.10029"
	}
	return DoubaoTTsDefaultResourceID
}
