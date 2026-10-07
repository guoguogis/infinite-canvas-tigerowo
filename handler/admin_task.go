package handler

import (
	"log"
	"net/http"

	"github.com/tigerowo/infinite-canvas/service"
)

// AdminRunningTasks 管理员查看全部用户的执行中任务。
func AdminRunningTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := service.ListAdminRunningTasks(r.Context(), r.URL.Query().Get("userId"))
	if err != nil {
		log.Printf("list admin running tasks failed: err=%v", err)
		FailError(w, err)
		return
	}
	OK(w, tasks)
}
