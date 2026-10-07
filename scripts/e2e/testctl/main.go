// 端到端功能测试夹具工具：为「我的任务」相关测试插入/清理执行中任务数据。
//
// 用法：
//
//	go run ./scripts/e2e/testctl -mode seed -admin-id <管理员用户id> -user-id <普通用户id>
//	go run ./scripts/e2e/testctl -mode stats
//	go run ./scripts/e2e/testctl -mode clean
//
// seed 会为两个账号各插入一条执行中视频任务与一条执行中图片任务，并输出 id 供测试脚本断言。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/tigerowo/infinite-canvas/config"
	"github.com/tigerowo/infinite-canvas/model"
	"github.com/tigerowo/infinite-canvas/repository"
	"gorm.io/gorm"
)

const fixturePrefix = "test-fixture-"

type seedResult struct {
	AdminVideoID string `json:"adminVideoId"`
	UserVideoID  string `json:"userVideoId"`
	AdminImageID string `json:"adminImageId"`
	UserImageID  string `json:"userImageId"`
}

type statsRow struct {
	Username      string `json:"username"`
	UserID        string `json:"userId"`
	CanvasProject int64  `json:"canvasProjects"`
	VideoLog      int64  `json:"videoGenerationLogs"`
	ImageLog      int64  `json:"imageGenerationLogs"`
	VideoTask     int64  `json:"videoTasks"`
	ImageTask     int64  `json:"canvasImageTasks"`
}

func main() {
	mode := flag.String("mode", "stats", "seed | clean | stats")
	adminID := flag.String("admin-id", "", "管理员用户 id（seed 必填）")
	userID := flag.String("user-id", "", "普通用户 id（seed 必填）")
	flag.Parse()

	if err := config.Load(); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	db, err := repository.DB()
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}

	switch *mode {
	case "seed":
		if *adminID == "" || *userID == "" {
			log.Fatal("seed 需要 -admin-id 与 -user-id")
		}
		result, err := seed(db, *adminID, *userID)
		if err != nil {
			log.Fatalf("插入夹具失败: %v", err)
		}
		emit(result)
	case "clean":
		counts, err := clean(db)
		if err != nil {
			log.Fatalf("清理夹具失败: %v", err)
		}
		emit(counts)
	case "stats":
		emit(stats(db))
	default:
		log.Fatalf("未知模式: %s", *mode)
	}
}

func emit(value any) {
	out, err := json.Marshal(value)
	if err != nil {
		log.Fatalf("序列化结果失败: %v", err)
	}
	fmt.Println(string(out))
}

func seed(db *gorm.DB, adminID string, userID string) (seedResult, error) {
	displayNames := map[string]string{}
	for _, id := range []string{adminID, userID} {
		var user model.User
		if err := db.First(&user, "id = ?", id).Error; err != nil {
			return seedResult{}, err
		}
		displayNames[id] = firstNonEmpty(user.DisplayName, user.Username)
	}
	current := time.Now().UTC().Format(time.RFC3339Nano)
	result := seedResult{
		AdminVideoID: fixturePrefix + "video-admin",
		UserVideoID:  fixturePrefix + "video-user",
		AdminImageID: fixturePrefix + "image-admin",
		UserImageID:  fixturePrefix + "image-user",
	}
	// 不设置 WorkflowRef：视频任务轮询器会因为渠道不存在而轮询失败，但状态保持 processing，
	// 正好模拟「仍在执行中」的任务，且不会消耗算力点。
	videoTasks := []model.VideoTask{
		{ID: result.AdminVideoID, UserID: adminID, UserDisplayName: displayNames[adminID], Model: "fixture-video-model", ChannelID: "fixture-channel", ChannelName: "夹具渠道", Source: "video-workbench", Status: "processing", Progress: 42, Seconds: "5", Size: "1280x720", CreatedAt: current, UpdatedAt: current},
		{ID: result.UserVideoID, UserID: userID, UserDisplayName: displayNames[userID], Model: "fixture-video-model", ChannelID: "fixture-channel", ChannelName: "夹具渠道", Source: "video-workbench", Status: "processing", Progress: 42, Seconds: "5", Size: "1280x720", CreatedAt: current, UpdatedAt: current},
	}
	for _, task := range videoTasks {
		if _, err := repository.SaveVideoTask(task); err != nil {
			return seedResult{}, err
		}
	}
	imageTasks := []model.CanvasImageTask{
		{ID: result.AdminImageID, UserID: adminID, UserDisplayName: displayNames[adminID], Source: "image-workbench", Model: "fixture-image-model", Prompt: "夹具图片提示词（管理员）", Status: "processing", Progress: 17, CreatedAt: current, UpdatedAt: current},
		{ID: result.UserImageID, UserID: userID, UserDisplayName: displayNames[userID], Source: "image-workbench", Model: "fixture-image-model", Prompt: "夹具图片提示词（普通用户）", Status: "processing", Progress: 17, CreatedAt: current, UpdatedAt: current},
	}
	for _, task := range imageTasks {
		if _, err := repository.SaveCanvasImageTask(task); err != nil {
			return seedResult{}, err
		}
	}
	return result, nil
}

func clean(db *gorm.DB) (map[string]int64, error) {
	counts := map[string]int64{}
	targets := []struct {
		name  string
		table any
	}{
		{"video_tasks", &model.VideoTask{}},
		{"canvas_image_tasks", &model.CanvasImageTask{}},
		{"canvas_projects", &model.CanvasProject{}},
		{"video_generation_logs", &model.VideoGenerationLog{}},
		{"image_generation_logs", &model.ImageGenerationLog{}},
	}
	for _, target := range targets {
		result := db.Where("id LIKE ?", fixturePrefix+"%").Delete(target.table)
		if result.Error != nil {
			return nil, result.Error
		}
		counts[target.name] = result.RowsAffected
	}
	return counts, nil
}

func stats(db *gorm.DB) []statsRow {
	var users []model.User
	if err := db.Order("username ASC").Find(&users).Error; err != nil {
		log.Fatalf("读取用户失败: %v", err)
	}
	rows := make([]statsRow, 0, len(users))
	for _, user := range users {
		row := statsRow{Username: user.Username, UserID: user.ID}
		db.Model(&model.CanvasProject{}).Where("user_id = ? AND deleted_at = ''", user.ID).Count(&row.CanvasProject)
		db.Model(&model.VideoGenerationLog{}).Where("user_id = ? AND deleted_at = ''", user.ID).Count(&row.VideoLog)
		db.Model(&model.ImageGenerationLog{}).Where("user_id = ? AND deleted_at = ''", user.ID).Count(&row.ImageLog)
		db.Model(&model.VideoTask{}).Where("user_id = ?", user.ID).Count(&row.VideoTask)
		db.Model(&model.CanvasImageTask{}).Where("user_id = ?", user.ID).Count(&row.ImageTask)
		rows = append(rows, row)
	}
	return rows
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
