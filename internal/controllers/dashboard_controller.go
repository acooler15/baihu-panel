package controllers

import (
	"github.com/engigu/baihu-panel/internal/services/tasks"
)

type DashboardController struct {
	executorService *tasks.ExecutorService
}

// StatsResponse 首页统计数据
type StatsResponse struct {
	Tasks      int64 `json:"tasks"`
	TodayExecs int64 `json:"today_execs"`
	Envs       int64 `json:"envs"`
	Logs       int64 `json:"logs"`
	Scheduled  int   `json:"scheduled"`
	Running    int   `json:"running"`
}

// DailyStats 每日统计数据
type DailyStats struct {
	Day     string `json:"day"`
	Total   int    `json:"total"`
	Success int    `json:"success"`
	Failed  int    `json:"failed"`
}

// TaskStats 任务执行统计
type TaskStats struct {
	TaskID   string `json:"task_id"`
	TaskName string `json:"task_name"`
	Count    int    `json:"count"`
}

func NewDashboardController(executorService *tasks.ExecutorService) *DashboardController {
	return &DashboardController{
		executorService: executorService,
	}
}
