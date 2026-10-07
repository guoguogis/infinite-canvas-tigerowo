package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/tigerowo/infinite-canvas/service"
)

type adminPromptSourceSyncRequest struct {
	ID string `json:"id"`
}

// AdminPromptSources 返回全部提示词来源。
func AdminPromptSources(w http.ResponseWriter, r *http.Request) {
	items, err := service.ListPromptSources()
	if err != nil {
		FailError(w, err)
		return
	}
	OK(w, items)
}

// AdminSavePromptSource 新增或更新一个提示词来源。
func AdminSavePromptSource(w http.ResponseWriter, r *http.Request) {
	var input service.PromptSourceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Fail(w, "提示词来源参数格式错误")
		return
	}
	items, err := service.SavePromptSource(input)
	if err != nil {
		Fail(w, err.Error())
		return
	}
	OK(w, items)
}

// AdminDeletePromptSource 删除一个非内置来源。
func AdminDeletePromptSource(w http.ResponseWriter, r *http.Request, id string) {
	items, err := service.DeletePromptSource(id)
	if err != nil {
		Fail(w, err.Error())
		return
	}
	OK(w, items)
}

// AdminSyncPromptSource 拉取单个来源的提示词。
func AdminSyncPromptSource(w http.ResponseWriter, r *http.Request) {
	var request adminPromptSourceSyncRequest
	_ = json.NewDecoder(r.Body).Decode(&request)
	if request.ID == "" {
		Fail(w, "缺少来源标识")
		return
	}
	log.Printf("sync prompt source start source=%s", request.ID)
	items, err := service.SyncPromptSource(request.ID)
	if err != nil {
		log.Printf("sync prompt source failed source=%s err=%v", request.ID, err)
		Fail(w, err.Error())
		return
	}
	log.Printf("sync prompt source done source=%s", request.ID)
	OK(w, items)
}

// AdminSyncAllPromptSources 同步全部启用的来源。
func AdminSyncAllPromptSources(w http.ResponseWriter, r *http.Request) {
	log.Printf("sync all prompt sources start")
	items, failures, err := service.SyncAllPromptSources()
	if err != nil {
		FailError(w, err)
		return
	}
	if len(failures) > 0 {
		log.Printf("sync all prompt sources partial failures=%v", failures)
	}
	OK(w, map[string]any{"sources": items, "failures": failures})
}
