package repository

import (
	"strings"

	"github.com/tigerowo/infinite-canvas/model"
)

var musicTaskActiveStatuses = []string{"queued", "in_progress", "processing", "running"}

func SaveMusicTask(task model.MusicTask) (model.MusicTask, error) {
	db, err := DB()
	if err != nil {
		return task, err
	}
	return task, db.Save(&task).Error
}

func GetMusicTask(id string) (model.MusicTask, bool, error) {
	db, err := DB()
	if err != nil {
		return model.MusicTask{}, false, err
	}
	var task model.MusicTask
	if err := db.First(&task, "id = ?", id).Error; err != nil {
		return model.MusicTask{}, false, nil
	}
	return task, true, nil
}

func GetUserMusicTask(userID string, id string) (model.MusicTask, bool, error) {
	db, err := DB()
	if err != nil {
		return model.MusicTask{}, false, err
	}
	var task model.MusicTask
	if err := db.First(&task, "user_id = ? AND (id = ? OR client_task_id = ? OR upstream_task_id = ?)", userID, id, id, id).Error; err != nil {
		return model.MusicTask{}, false, nil
	}
	return task, true, nil
}

// ListUserMusicTasks 按用户分页列出音乐任务，status 为空时返回全部状态。
func ListUserMusicTasks(userID string, status string, q model.Query) ([]model.MusicTask, int64, error) {
	db, err := DB()
	if err != nil {
		return nil, 0, err
	}
	q.Normalize()
	query := db.Model(&model.MusicTask{}).Where("user_id = ?", userID)
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(status))
	}
	if keyword := strings.TrimSpace(q.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("title LIKE ? OR prompt LIKE ? OR lyrics LIKE ?", like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	tasks := make([]model.MusicTask, 0)
	err = query.Order("created_at DESC").Offset(q.Offset()).Limit(q.PageSize).Find(&tasks).Error
	return tasks, total, err
}

func DeleteUserMusicTask(userID string, id string) error {
	db, err := DB()
	if err != nil {
		return err
	}
	return db.Where("user_id = ? AND (id = ? OR client_task_id = ? OR upstream_task_id = ?)", userID, id, id, id).Delete(&model.MusicTask{}).Error
}

// ListDueMusicTasks 列出等待轮询的音乐任务。
func ListDueMusicTasks(limit int) ([]model.MusicTask, error) {
	db, err := DB()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	var tasks []model.MusicTask
	err = db.Where("status IN ?", musicTaskActiveStatuses).
		Order("created_at ASC").
		Limit(limit).
		Find(&tasks).Error
	return tasks, err
}

// ListActiveMusicTasks 列出执行中的音乐任务，userID 为空时返回全部用户。
func ListActiveMusicTasks(userID string, limit int) ([]model.MusicTask, error) {
	db, err := DB()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	query := db.Where("status IN ?", musicTaskActiveStatuses)
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	var tasks []model.MusicTask
	err = query.Order("created_at DESC").Limit(limit).Find(&tasks).Error
	return tasks, err
}
