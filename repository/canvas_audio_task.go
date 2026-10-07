package repository

import "github.com/tigerowo/infinite-canvas/model"

func SaveCanvasAudioTask(task model.CanvasAudioTask) (model.CanvasAudioTask, error) {
	db, err := DB()
	if err != nil {
		return task, err
	}
	return task, db.Save(&task).Error
}

func UpdateCanvasAudioTask(task model.CanvasAudioTask) (model.CanvasAudioTask, error) {
	db, err := DB()
	if err != nil {
		return task, err
	}

	return task, db.Model(&model.CanvasAudioTask{}).
		Where("user_id = ? AND id = ?", task.UserID, task.ID).
		Select("*").
		Updates(&task).Error
}

func GetUserCanvasAudioTask(userID string, id string) (model.CanvasAudioTask, bool, error) {
	db, err := DB()
	if err != nil {
		return model.CanvasAudioTask{}, false, err
	}
	var task model.CanvasAudioTask
	err = db.First(&task, "user_id = ? AND id = ?", userID, id).Error
	if err != nil {
		return model.CanvasAudioTask{}, false, nil
	}
	return task, true, nil
}

// ListActiveCanvasAudioTasks 列出执行中的音频任务，userID 为空时返回全部用户。
func ListActiveCanvasAudioTasks(userID string, limit int) ([]model.CanvasAudioTask, error) {
	db, err := DB()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	var tasks []model.CanvasAudioTask
	query := db.Where("status IN ?", []string{"queued", "processing", "running", "in_progress"})
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	err = query.Order("created_at DESC").Limit(limit).Find(&tasks).Error
	return tasks, err
}
