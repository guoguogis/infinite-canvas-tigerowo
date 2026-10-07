package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tigerowo/infinite-canvas/model"
	"github.com/tigerowo/infinite-canvas/service"
)

func doubaoTTsTestRequest(body string) aiProtocolRequest {
	return aiProtocolRequest{
		mode: aiProtocolProxyRequest, endpoint: "/audio/speech", modelName: "doubao-seed-tts-2.0",
		channel: model.ModelChannel{Protocol: "doubao-tts"}, contentType: "application/json", body: []byte(body),
	}
}

func TestPrepareDoubaoTTsRequest(t *testing.T) {
	prepared, handled, err := prepareDoubaoTTsRequest(doubaoTTsTestRequest(`{"model":"doubao-seed-tts-2.0","input":"你好","voice":"alloy","response_format":"mp3","speed":1}`))
	if err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	if prepared.headers["X-Api-Resource-Id"] != "seed-tts-2.0" {
		t.Fatalf("resource id header: %v", prepared.headers)
	}
	want := `{"req_params":{"audio_params":{"format":"mp3","sample_rate":24000},"speaker":"zh_female_shaoergushi_uranus_bigtts","text":"你好"}}`
	assertProtocolJSONValue(t, json.RawMessage(prepared.body), want)

	// OpenAI 音色名对豆包无意义，显式豆包音色则原样透传。
	custom, _, err := prepareDoubaoTTsRequest(doubaoTTsTestRequest(`{"input":"hi","voice":"zh_male_wenhao_mars_bigtts","response_format":"wav"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(custom.body, []byte(`"speaker":"zh_male_wenhao_mars_bigtts"`)) || !bytes.Contains(custom.body, []byte(`"format":"wav"`)) {
		t.Fatalf("custom voice body: %s", custom.body)
	}
	if custom.headers["X-Api-Resource-Id"] != "volc.service_type.10029" {
		t.Fatalf("1.0 voice resource id: %v", custom.headers)
	}

	// 私有复刻音色需要换成复刻资源 ID，否则上游会返回空音频。
	clone, _, err := prepareDoubaoTTsRequest(doubaoTTsTestRequest(`{"input":"hi","voice":"S_AbCdEf123"}`))
	if err != nil {
		t.Fatal(err)
	}
	if clone.headers["X-Api-Resource-Id"] != "seed-icl-2.0" || !bytes.Contains(clone.body, []byte(`"speaker":"S_AbCdEf123"`)) {
		t.Fatalf("clone voice: headers=%v body=%s", clone.headers, clone.body)
	}

	// 其他协议与其他端点不拦截。
	if _, handled, _ := prepareDoubaoTTsRequest(aiProtocolRequest{channel: model.ModelChannel{Protocol: "ark"}, endpoint: "/audio/speech", body: []byte(`{}`)}); handled {
		t.Fatal("ark channel should not be handled")
	}
	empty := doubaoTTsTestRequest(`{"input":"   "}`)
	if _, handled, err := prepareDoubaoTTsRequest(empty); !handled || err == nil {
		t.Fatalf("empty text should fail: handled=%v err=%v", handled, err)
	}
}

func TestDoubaoTTsUpstreamURL(t *testing.T) {
	// TokenPlan 与后付费只在 baseUrl 上区分，协议内路径一致。
	for _, test := range []struct{ name, baseURL, want string }{
		{"token plan", "https://openspeech.bytedance.com/api/v3/plan", "https://openspeech.bytedance.com/api/v3/plan/tts/unidirectional"},
		{"pay as you go", "https://openspeech.bytedance.com/api/v3", "https://openspeech.bytedance.com/api/v3/tts/unidirectional"},
		{"trailing slash normalized", " https://openspeech.bytedance.com/api/v3/plan/ ", "https://openspeech.bytedance.com/api/v3/plan/tts/unidirectional"},
	} {
		channel := model.ModelChannel{Protocol: "doubao-tts", BaseURL: test.baseURL, APIKey: "speech-key"}
		path := resolveAIProxyPath(channel, "doubao-seed-tts-2.0", "/audio/speech")
		if path != "/tts/unidirectional" {
			t.Fatalf("%s: resolved path %s", test.name, path)
		}
		if url := service.BuildModelChannelURL(channel, path); url != test.want {
			t.Fatalf("%s: upstream url %s, want %s", test.name, url, test.want)
		}
	}
}

func TestReadDoubaoTTsAudio(t *testing.T) {
	first := base64.StdEncoding.EncodeToString([]byte("AAA"))
	second := base64.StdEncoding.EncodeToString([]byte("BBB"))
	stream := strings.Join([]string{
		`{"code":0,"data":"` + first + `"}`,
		``,
		`{"code":0,"data":"` + second + `"}`,
		`{"code":20000000,"data":""}`,
	}, "\n")
	audio, err := readDoubaoTTsAudio(strings.NewReader(stream))
	if err != nil || string(audio) != "AAABBB" {
		t.Fatalf("audio=%q err=%v", audio, err)
	}

	if _, err := readDoubaoTTsAudio(strings.NewReader(`{"code":45000000,"message":"音色不存在"}`)); err == nil || !strings.Contains(err.Error(), "音色不存在") {
		t.Fatalf("upstream error not surfaced: %v", err)
	}
	if _, err := readDoubaoTTsAudio(strings.NewReader(`{"code":20000000}`)); err == nil {
		t.Fatal("empty audio should fail")
	}
}
