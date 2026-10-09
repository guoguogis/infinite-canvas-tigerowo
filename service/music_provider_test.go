package service

import (
	"strings"
	"testing"
)

func TestMusicAudioPayloadFrom(t *testing.T) {
	payload := musicAudioPayloadFrom("https://media.invalid/a.mp3")
	if payload.URL == "" {
		t.Fatalf("直链应识别为 URL，得到 %+v", payload)
	}
	payload = musicAudioPayloadFrom("data:audio/wav;base64,AQID")
	if payload.Base64 != "AQID" || payload.Mime != "audio/wav" {
		t.Fatalf("data URL 解析错误：%+v", payload)
	}
	payload = musicAudioPayloadFrom("00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff")
	if payload.Hex == "" {
		t.Fatalf("长 hex 应识别为 Hex，得到 %+v", payload)
	}
	payload = musicAudioPayloadFrom("QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVo=")
	if payload.Base64 == "" {
		t.Fatalf("非 hex 应落到 Base64，得到 %+v", payload)
	}
	if !musicAudioPayloadFrom("").Empty() {
		t.Fatal("空值应为 Empty")
	}
}

func TestMusicVolcParameterMapping(t *testing.T) {
	cases := map[string]string{"男声": "Male", "male": "Male", "女声": "Female", "合唱": ""}
	for input, want := range cases {
		if got := volcMusicGender(input); got != want {
			t.Errorf("volcMusicGender(%q) = %q，期望 %q", input, got, want)
		}
	}
	if got := volcMusicKeyMode("大调"); got != "Major" {
		t.Errorf("volcMusicKeyMode(大调) = %q", got)
	}
	if got := volcMusicKeyMode("Minor"); got != "Minor" {
		t.Errorf("volcMusicKeyMode(Minor) = %q", got)
	}
	if got := volcMusicLanguage("中文"); got != "Chinese" {
		t.Errorf("volcMusicLanguage(中文) = %q", got)
	}
	if got := volcMusicLanguage("English"); got != "English" {
		t.Errorf("volcMusicLanguage(English) = %q", got)
	}
}

func TestClampMusicSeconds(t *testing.T) {
	if got := clampSeconds(0, 30, 240); got != 0 {
		t.Errorf("未填时长应保持 0，得到 %d", got)
	}
	if got := clampSeconds(300, 30, 240); got != 240 {
		t.Errorf("超出上限应夹到 240，得到 %d", got)
	}
	if got := clampSeconds(200, 30, 120); got != 120 {
		t.Errorf("纯音乐上限应夹到 120，得到 %d", got)
	}
	if got := clampSeconds(10, 30, 120); got != 30 {
		t.Errorf("低于下限应夹到 30，得到 %d", got)
	}
}

func TestMusicAudioMimeType(t *testing.T) {
	if got := musicAudioMimeType("", "audio/wav; charset=binary"); got != "audio/wav" {
		t.Errorf("应取到 audio/wav，得到 %q", got)
	}
	if got := musicAudioMimeType("audio/mpeg"); got != "audio/mpeg" {
		t.Errorf("得到 %q", got)
	}
	if got := musicAudioMimeType("", "application/octet-stream"); got != "audio/mpeg" {
		t.Errorf("非音频类型应回退 audio/mpeg，得到 %q", got)
	}
}

func TestMusicPathReadingForVolcPayload(t *testing.T) {
	payload := []byte(`{"Result":{"Status":2,"TaskID":"t-1","SongDetail":{"AudioUrl":"https://media.invalid/a.mp3","Duration":150}}}`)
	if got := musicIntPath(payload, "Result.Status"); got != 2 {
		t.Errorf("Result.Status = %d", got)
	}
	if got := musicStringPath(payload, "Result.TaskID"); got != "t-1" {
		t.Errorf("Result.TaskID = %q", got)
	}
	if got := musicStringPath(payload, "Result.SongDetail.AudioUrl"); got != "https://media.invalid/a.mp3" {
		t.Errorf("AudioUrl = %q", got)
	}
	if got := musicIntPath(payload, "Result.SongDetail.Duration"); got != 150 {
		t.Errorf("Duration = %d", got)
	}
	if got := musicStringPath(payload, "Result.Missing.Value"); got != "" {
		t.Errorf("缺失字段应返回空，得到 %q", got)
	}
	if got := musicErrorText([]byte(`{"ResponseMetadata":{"Error":{"Message":"invalid action"}}}`)); got != "invalid action" {
		t.Errorf("错误信息 = %q", got)
	}
}

func TestMusicChannelURL(t *testing.T) {
	tokenHubCases := map[string]string{
		"https://tokenhub.tencentmaas.com":                                  "https://tokenhub.tencentmaas.com/v1/wand/minimax-music/generation",
		"https://tokenhub.tencentmaas.com/":                                 "https://tokenhub.tencentmaas.com/v1/wand/minimax-music/generation",
		"https://tokenhub.tencentmaas.com/v1":                               "https://tokenhub.tencentmaas.com/v1/wand/minimax-music/generation",
		"https://tokenhub.tencentmaas.com/v1/wand/minimax-music/generation": "https://tokenhub.tencentmaas.com/v1/wand/minimax-music/generation",
		"": "",
	}
	for input, want := range tokenHubCases {
		if got := musicChannelURL(input, tokenHubMusicPath); got != want {
			t.Errorf("musicChannelURL(%q, TokenHub) = %q，期望 %q", input, got, want)
		}
	}
	// MiniMax 官方：只填域名或 /v1 时补齐默认路径；第三方中转已带路径（路径与官方无关）时原样使用。
	minimaxCases := map[string]string{
		"https://api.minimax.cn":                     "https://api.minimax.cn/v1/music_generation",
		"https://api.minimax.cn/":                    "https://api.minimax.cn/v1/music_generation",
		"https://api.minimax.cn/v1":                  "https://api.minimax.cn/v1/music_generation",
		"https://metaso.cn/api/minimax":              "https://metaso.cn/api/minimax",
		"https://relay.invalid/minimax/v1/music_gen": "https://relay.invalid/minimax/v1/music_gen",
	}
	for input, want := range minimaxCases {
		if got := musicChannelURL(input, minimaxMusicPath); got != want {
			t.Errorf("musicChannelURL(%q, MiniMax) = %q，期望 %q", input, got, want)
		}
	}
	if got := musicChannelURL("https://api.minimax.cn", minimaxLyricsPath); got != "https://api.minimax.cn/v1/lyrics_generation" {
		t.Errorf("MiniMax 歌词地址 = %q", got)
	}
}

func TestMusicSafeBody(t *testing.T) {
	if got := musicSafeBody([]byte("  {\"a\":1}  ")); got != "{\"a\":1}" {
		t.Errorf("得到 %q", got)
	}
	got := musicSafeBody([]byte(strings.Repeat("a", 3000)))
	if len(got) > 2020 || !strings.HasSuffix(got, "(truncated)") {
		t.Errorf("超长响应应被截断，长度 %d", len(got))
	}
}

func TestMusicTaskStatusNormalization(t *testing.T) {
	if !IsCompletedMusicTaskStatus("success") {
		t.Fatal("success 应归一为 completed")
	}
	if !IsFailedMusicTaskStatus("error") {
		t.Fatal("error 应归一为 failed")
	}
	if got := NormalizeMusicTaskStatus(""); got != "queued" {
		t.Fatalf("空状态应归一为 queued，得到 %q", got)
	}
}
