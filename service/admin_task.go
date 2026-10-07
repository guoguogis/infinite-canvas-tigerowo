package service

import (
	"context"
	"sort"
	"strings"

	"github.com/tigerowo/infinite-canvas/model"
	"github.com/tigerowo/infinite-canvas/repository"
)

const adminRunningTaskLimit = 300

// AdminRunningTask 管理员视角下的执行中任务。
type AdminRunningTask struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	UserID    string `json:"userId"`
	UserName  string `json:"userName"`
	Model     string `json:"model"`
	Status    string `json:"status"`
	Progress  int    `json:"progress"`
	Prompt    string `json:"prompt"`
	Source    string `json:"source"`
	SourceID  string `json:"sourceId"`
	NodeID    string `json:"nodeId"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// ListAdminRunningTasks 列出全部用户的执行中任务，userID 非空时只返回该用户。
func ListAdminRunningTasks(ctx context.Context, userID string) ([]AdminRunningTask, error) {
	user, ok := UserFromContext(ctx)
	if !ok || user.Role != model.UserRoleAdmin {
		return nil, safeMessageError{message: "未登录或权限不足"}
	}
	owner := strings.TrimSpace(userID)
	result := make([]AdminRunningTask, 0)

	videoTasks, err := repository.ListActiveVideoTasks(owner, adminRunningTaskLimit)
	if err != nil {
		return nil, err
	}
	for _, task := range videoTasks {
		result = append(result, AdminRunningTask{
			ID: task.ID, Kind: "video", UserID: task.UserID, UserName: task.UserDisplayName,
			Model: task.Model, Status: task.Status, Progress: task.Progress,
			Source: task.Source, SourceID: task.SourceID,
			CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt,
		})
	}

	imageTasks, err := repository.ListActiveCanvasImageTasks(owner, adminRunningTaskLimit)
	if err != nil {
		return nil, err
	}
	for _, task := range imageTasks {
		result = append(result, AdminRunningTask{
			ID: task.ID, Kind: "image", UserID: task.UserID, UserName: task.UserDisplayName,
			Model: task.Model, Status: task.Status, Progress: task.Progress, Prompt: task.Prompt,
			Source: task.Source, SourceID: task.SourceID, NodeID: task.NodeID,
			CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt,
		})
	}

	audioTasks, err := repository.ListActiveCanvasAudioTasks(owner, adminRunningTaskLimit)
	if err != nil {
		return nil, err
	}
	for _, task := range audioTasks {
		result = append(result, AdminRunningTask{
			ID: task.ID, Kind: "audio", UserID: task.UserID, UserName: task.UserDisplayName,
			Model: task.Model, Status: task.Status, Progress: task.Progress, Prompt: task.Prompt,
			Source: task.Source, SourceID: task.SourceID, NodeID: task.NodeID,
			CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt > result[j].CreatedAt
	})
	return result, nil
}
