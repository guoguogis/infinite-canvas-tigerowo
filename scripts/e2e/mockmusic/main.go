// 本地 mock：豆包音乐上游（火山 API Gateway Action 风格）+ 一个 S3 兼容对象存储。
//
// 用途：在没有真实音乐模型 API Key 的情况下，端到端验证「音乐生成任务」链路。
//
// 用法：
//
//	go run ./scripts/e2e/mockmusic -music-addr 127.0.0.1:18081 -s3-addr 127.0.0.1:18082 -s3-dir <临时目录>
//
// 音乐接口（POST /?Action=<Action>&Version=2024-08-12）：
//
//	GenSongForTime / GenBGMForTime -> {"Result":{"TaskID":"..."}}
//	QuerySong                      -> 前 2 次 {"Result":{"Status":1}}，第 3 次起 Status 2 + SongDetail.AudioUrl
//	GenLyrics                      -> {"Result":{"Lyrics":"..."}}
//
// 触发失败分支：
//
//	Prompt 含 __FAIL_SUBMIT__ -> GenSongForTime 返回 HTTP 400（上游没接手）
//	Prompt 含 __FAIL_QUERY__  -> QuerySong 返回 Status 3 + FailureReason
package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	musicAddr = flag.String("music-addr", "127.0.0.1:18081", "音乐上游 mock 监听地址")
	s3Addr    = flag.String("s3-addr", "127.0.0.1:18082", "S3 mock 监听地址")
	s3Dir     = flag.String("s3-dir", "", "S3 mock 落盘目录，默认用系统临时目录")
	apiKey    = flag.String("api-key", "mock-music-key", "期望的 Bearer token")
	service   = flag.String("service-name", "imagination", "期望的 ServiceName 头")
)

type taskState struct {
	queries int
	failAt  string // "" | "submit" | "query"
}

type store struct {
	mu    sync.Mutex
	tasks map[string]*taskState
	seq   int
	dir   string
}

func main() {
	flag.Parse()
	dir := *s3Dir
	if dir == "" {
		dir, _ = os.MkdirTemp("", "mock-s3-")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatal(err)
	}
	s := &store{tasks: map[string]*taskState{}, dir: dir}
	log.Printf("mock 音乐上游: http://%s (apiKey=%s serviceName=%s)", *musicAddr, *apiKey, *service)
	log.Printf("mock S3 存储:  http://%s (落盘目录 %s)", *s3Addr, dir)

	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", s.handleMusic)
		log.Fatal(http.ListenAndServe(*musicAddr, logRequests("music", mux)))
	}()
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleS3)
	log.Fatal(http.ListenAndServe(*s3Addr, logRequests("s3", mux)))
}

func logRequests(tag string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %s %s%s", tag, r.Method, r.URL.Path, rawQuery(r))
		next.ServeHTTP(w, r)
	})
}

func rawQuery(r *http.Request) string {
	if r.URL.RawQuery == "" {
		return ""
	}
	return "?" + r.URL.RawQuery
}

// ---------- 音乐上游 ----------

func (s *store) handleMusic(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/audio/mock.wav" {
		writeWAV(w)
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ResponseMetadata": map[string]any{"Error": map[string]any{"Message": "只支持 POST"}}})
		return
	}
	if got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "); got != *apiKey {
		log.Printf("[music] 鉴权失败 Authorization=%q", r.Header.Get("Authorization"))
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ResponseMetadata": map[string]any{"Error": map[string]any{"Message": "mock: Bearer 鉴权不匹配"}}})
		return
	}
	if got := r.Header.Get("ServiceName"); got != *service {
		log.Printf("[music] ServiceName 不匹配 %q", got)
		writeJSON(w, http.StatusForbidden, map[string]any{"ResponseMetadata": map[string]any{"Error": map[string]any{"Message": "mock: ServiceName 不匹配"}}})
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	action := r.URL.Query().Get("Action")
	version := r.URL.Query().Get("Version")
	log.Printf("[music] Action=%s Version=%s body=%s", action, version, string(body))
	if version != "2024-08-12" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ResponseMetadata": map[string]any{"Error": map[string]any{"Message": "mock: Version 不是 2024-08-12"}}})
		return
	}
	var payload map[string]any
	_ = json.Unmarshal(body, &payload)

	switch action {
	case "GenSongForTime", "GenBGMForTime":
		text := fmt.Sprint(payload["Prompt"])
		if strings.Contains(text, "__FAIL_SUBMIT__") {
			log.Printf("[music] 按要求返回提交失败")
			writeJSON(w, http.StatusBadRequest, map[string]any{"ResponseMetadata": map[string]any{"Error": map[string]any{"Message": "mock: 上游拒绝创建任务"}}})
			return
		}
		s.mu.Lock()
		s.seq++
		taskID := fmt.Sprintf("mock-song-%d", s.seq)
		failAt := ""
		if strings.Contains(text, "__FAIL_QUERY__") {
			failAt = "query"
		}
		s.tasks[taskID] = &taskState{failAt: failAt}
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"ResponseMetadata": map[string]any{"RequestId": "mock-req"}, "Result": map[string]any{"TaskID": taskID}})
	case "QuerySong":
		taskID := fmt.Sprint(payload["TaskID"])
		s.mu.Lock()
		task := s.tasks[taskID]
		if task == nil {
			task = &taskState{}
			s.tasks[taskID] = task
		}
		task.queries++
		queries := task.queries
		failAt := task.failAt
		s.mu.Unlock()
		log.Printf("[music] QuerySong task=%s 第 %d 次查询 failAt=%q", taskID, queries, failAt)
		if failAt == "query" {
			writeJSON(w, http.StatusOK, map[string]any{
				"ResponseMetadata": map[string]any{"RequestId": "mock-req"},
				"Result":           map[string]any{"TaskID": taskID, "Status": 3, "FailureReason": map[string]any{"Msg": "mock: 音乐生成失败（内容审核未通过）", "Code": 1001}},
			})
			return
		}
		if queries <= 2 {
			writeJSON(w, http.StatusOK, map[string]any{"ResponseMetadata": map[string]any{"RequestId": "mock-req"}, "Result": map[string]any{"TaskID": taskID, "Status": 1, "Progress": 30}})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ResponseMetadata": map[string]any{"RequestId": "mock-req"},
			"Result": map[string]any{
				"TaskID": taskID,
				"Status": 2,
				"SongDetail": map[string]any{
					"AudioUrl":  fmt.Sprintf("http://%s/audio/mock.wav", *musicAddr),
					"Duration":  12,
					"Lyrics":    "mock 歌词第一行\nmock 歌词第二行",
					"VodFormat": "wav",
				},
			},
		})
	case "GenLyrics":
		writeJSON(w, http.StatusOK, map[string]any{"ResponseMetadata": map[string]any{"RequestId": "mock-req"}, "Result": map[string]any{"Lyrics": "mock 生成的歌词\n来自本地 mock 上游"}})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ResponseMetadata": map[string]any{"Error": map[string]any{"Message": "mock: 未知 Action " + action}}})
	}
}

// writeWAV 输出 2 秒 8kHz 16bit 单声道静音 wav（不依赖任何外部文件）。
func writeWAV(w http.ResponseWriter) {
	const (
		sampleRate = 8000
		seconds    = 2
		channels   = 1
		bits       = 16
	)
	dataSize := sampleRate * seconds * channels * bits / 8
	buf := make([]byte, 44+dataSize)
	copy(buf[0:], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:], uint32(36+dataSize))
	copy(buf[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(buf[16:], 16)
	binary.LittleEndian.PutUint16(buf[20:], 1)
	binary.LittleEndian.PutUint16(buf[22:], channels)
	binary.LittleEndian.PutUint32(buf[24:], sampleRate)
	binary.LittleEndian.PutUint32(buf[28:], sampleRate*channels*bits/8)
	binary.LittleEndian.PutUint16(buf[32:], channels*bits/8)
	binary.LittleEndian.PutUint16(buf[34:], bits)
	copy(buf[36:], "data")
	binary.LittleEndian.PutUint32(buf[40:], uint32(dataSize))
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Content-Length", fmt.Sprint(len(buf)))
	_, _ = w.Write(buf)
}

// ---------- S3 mock ----------

// handleS3 接受 path style 的 /<bucket>/<key> 请求，忽略 SigV4 签名（只校验 Authorization 存在）。
func (s *store) handleS3(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") == "" {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "mock S3: 缺少 Authorization"})
		return
	}
	key := strings.TrimPrefix(r.URL.Path, "/")
	if key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "mock S3: 缺少 bucket/key"})
		return
	}
	target := filepath.Join(s.dir, filepath.FromSlash(strings.ReplaceAll(key, "..", "_")))
	switch r.Method {
	case http.MethodPut, http.MethodPost:
		data, err := io.ReadAll(io.LimitReader(r.Body, 128<<20))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		log.Printf("[s3] 已写入 %s (%d 字节)", key, len(data))
		w.WriteHeader(http.StatusOK)
	case http.MethodGet, http.MethodHead:
		data, err := os.ReadFile(target)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "NoSuchKey"})
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		if r.Method == http.MethodGet {
			_, _ = w.Write(data)
		}
	case http.MethodDelete:
		_ = os.Remove(target)
		log.Printf("[s3] 已删除 %s", key)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "mock S3: 不支持 " + r.Method})
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	payload, _ := json.Marshal(value)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(payload)
}
