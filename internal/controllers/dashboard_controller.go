package controllers

import (
	"context"
	"net/http"
	"sort"
	"time"

	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/database"
	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/services/tasks"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
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

// ===========================================================================
// 仪表盘业务方法
// ===========================================================================

// GetStatsOutput 获取仪表盘统计
type GetStatsOutput struct {
	Body utils.HumaResponse[StatsResponse]
}

// GetStats 获取仪表盘统计
func (dc *DashboardController) GetStats(ctx context.Context, input *struct{}) (*GetStatsOutput, error) {
	var taskCount, envCount, logCount, todayExecs int64

	database.DB.Model(&models.Task{}).Count(&taskCount)
	database.DB.Model(&models.EnvironmentVariable{}).Count(&envCount)
	database.DB.Model(&models.TaskLog{}).Count(&logCount)

	// 今日执行总数
	today := time.Now().Format("2006-01-02")
	database.DB.Model(&models.SendStats{}).Where("day = ?", today).Select("COALESCE(SUM(num), 0)").Scan(&todayExecs)

	// 调度统计：本地调度 + Agent 调度
	localScheduled := dc.executorService.GetScheduledCount()

	var agentScheduled int64
	database.DB.Model(&models.Task{}).
		Where("agent_id IS NOT NULL AND enabled = ?", true).
		Count(&agentScheduled)

	totalScheduled := localScheduled + int(agentScheduled)

	// 正在运行
	running := dc.executorService.GetRunningCount()

	stats := StatsResponse{
		Tasks:      taskCount,
		TodayExecs: todayExecs,
		Envs:       envCount,
		Logs:       logCount,
		Scheduled:  totalScheduled,
		Running:    running,
	}

	return &GetStatsOutput{
		Body: utils.HumaResponse[StatsResponse]{
			Code: 200,
			Msg:  "success",
			Data: stats,
		},
	}, nil
}

// GetSentenceOutput 获取随机古诗词
type GetSentenceOutput struct {
	Body utils.HumaResponse[struct {
		Sentence string `json:"sentence"`
	}]
}

// GetSentence 获取随机古诗词
func (dc *DashboardController) GetSentence(ctx context.Context, input *struct{}) (*GetSentenceOutput, error) {
	return &GetSentenceOutput{
		Body: utils.HumaResponse[struct {
			Sentence string `json:"sentence"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				Sentence string `json:"sentence"`
			}{
				Sentence: constant.GetRandomSentence(),
			},
		},
	}, nil
}

// GetSendStatsInput 获取发送统计
type GetSendStatsInput struct {
	Days int `query:"days" default:"30" description:"统计天数，1-90"`
}

// GetSendStatsOutput 获取发送统计
type GetSendStatsOutput struct {
	Body utils.HumaResponse[[]DailyStats]
}

// GetSendStats 获取发送统计
func (dc *DashboardController) GetSendStats(ctx context.Context, input *GetSendStatsInput) (*GetSendStatsOutput, error) {
	days := input.Days
	if days <= 0 || days > 90 {
		days = 30
	}

	now := time.Now()
	startDay := now.AddDate(0, 0, -(days - 1)).Format("2006-01-02")

	var stats []models.SendStats
	database.DB.Where("day >= ?", startDay).Find(&stats)

	// 按日期聚合
	dayMap := make(map[string]*DailyStats)
	for _, s := range stats {
		if _, ok := dayMap[s.Day]; !ok {
			dayMap[s.Day] = &DailyStats{Day: s.Day}
		}
		ds := dayMap[s.Day]
		ds.Total += s.Num
		if s.Status == constant.TaskStatusSuccess {
			ds.Success += s.Num
		} else {
			ds.Failed += s.Num
		}
	}

	// 填充缺失的日期
	result := make([]DailyStats, 0, days)
	for i := days - 1; i >= 0; i-- {
		day := now.AddDate(0, 0, -i).Format("2006-01-02")
		if ds, ok := dayMap[day]; ok {
			result = append(result, *ds)
		} else {
			result = append(result, DailyStats{Day: day})
		}
	}

	// 按日期排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Day < result[j].Day
	})

	return &GetSendStatsOutput{
		Body: utils.HumaResponse[[]DailyStats]{
			Code: 200,
			Msg:  "success",
			Data: result,
		},
	}, nil
}

// GetTaskStatsInput 获取任务执行占比
type GetTaskStatsInput struct {
	Days int `query:"days" default:"30" description:"统计天数，1-90"`
}

// GetTaskStatsOutput 获取任务执行占比
type GetTaskStatsOutput struct {
	Body utils.HumaResponse[[]TaskStats]
}

// GetTaskStats 获取任务执行占比
func (dc *DashboardController) GetTaskStats(ctx context.Context, input *GetTaskStatsInput) (*GetTaskStatsOutput, error) {
	days := input.Days
	if days <= 0 || days > 90 {
		days = 30
	}

	now := time.Now()
	startDay := now.AddDate(0, 0, -(days - 1)).Format("2006-01-02")

	// 按 task_id 聚合统计
	var results []struct {
		TaskID string
		Total  int
	}
	database.DB.Model(&models.SendStats{}).
		Select("task_id, SUM(num) as total").
		Where("day >= ?", startDay).
		Group("task_id").
		Order("total DESC").
		Find(&results)

	// 获取任务名称
	taskIDs := make([]string, 0, len(results))
	for _, r := range results {
		taskIDs = append(taskIDs, r.TaskID)
	}

	var tasks []models.Task
	if len(taskIDs) > 0 {
		database.DB.Where("id IN ?", taskIDs).Find(&tasks)
	}
	taskNameMap := make(map[string]string)
	for _, t := range tasks {
		taskNameMap[t.ID] = t.Name
	}

	// 构建结果
	stats := make([]TaskStats, 0, len(results))
	for _, r := range results {
		name := taskNameMap[r.TaskID]
		if name == "" {
			name = "未知任务"
		}
		stats = append(stats, TaskStats{
			TaskID:   r.TaskID,
			TaskName: name,
			Count:    r.Total,
		})
	}

	return &GetTaskStatsOutput{
		Body: utils.HumaResponse[[]TaskStats]{
			Code: 200,
			Msg:  "success",
			Data: stats,
		},
	}, nil
}

// ===========================================================================
// /api/v1 管理接口 (CookieAuth) 路由注册
// ===========================================================================

// RegisterAPIDashboardRoutes 注册 /api/v1 仪表盘 Huma 路由
func (dc *DashboardController) RegisterAPIDashboardRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/dashboard/stats",
		OperationID: "apiGetStats",
		Summary:     "获取仪表盘统计",
		Description: "获取任务、环境变量、日志等数量统计及调度运行状态",
		Tags:        []string{"仪表盘"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, dc.GetStats)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/dashboard/sentence",
		OperationID: "apiGetSentence",
		Summary:     "获取随机古诗词",
		Description: "获取一句随机古诗词",
		Tags:        []string{"仪表盘"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, dc.GetSentence)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/dashboard/sendstats",
		OperationID: "apiGetSendStats",
		Summary:     "获取发送统计",
		Description: "获取每日发送统计，默认最近 30 天",
		Tags:        []string{"仪表盘"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, dc.GetSendStats)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/dashboard/taskstats",
		OperationID: "apiGetTaskStats",
		Summary:     "获取任务执行占比",
		Description: "获取任务执行统计，默认最近 30 天",
		Tags:        []string{"仪表盘"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, dc.GetTaskStats)
}
