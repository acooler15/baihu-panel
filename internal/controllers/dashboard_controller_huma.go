package controllers

import (
	"context"
	"net/http"
	"sort"
	"time"

	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/database"
	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

// ===========================================================================
// /api/v1 管理接口 —— 仪表盘
// ===========================================================================

// TAGetStatsOutput 获取仪表盘统计
type TAGetStatsOutput struct {
	Body utils.HumaResponse[StatsResponse]
}

// TAGetStats 获取仪表盘统计
func (dc *DashboardController) TAGetStats(ctx context.Context, input *struct{}) (*TAGetStatsOutput, error) {
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

	return &TAGetStatsOutput{
		Body: utils.HumaResponse[StatsResponse]{
			Code: 200,
			Msg:  "success",
			Data: stats,
		},
	}, nil
}

// TAGetSentenceOutput 获取随机古诗词
type TAGetSentenceOutput struct {
	Body utils.HumaResponse[struct {
		Sentence string `json:"sentence"`
	}]
}

// TAGetSentence 获取随机古诗词
func (dc *DashboardController) TAGetSentence(ctx context.Context, input *struct{}) (*TAGetSentenceOutput, error) {
	return &TAGetSentenceOutput{
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

// TAGetSendStatsInput 获取发送统计
type TAGetSendStatsInput struct {
	Days int `query:"days" default:"30" description:"统计天数，1-90"`
}

// TAGetSendStatsOutput 获取发送统计
type TAGetSendStatsOutput struct {
	Body utils.HumaResponse[[]DailyStats]
}

// TAGetSendStats 获取发送统计
func (dc *DashboardController) TAGetSendStats(ctx context.Context, input *TAGetSendStatsInput) (*TAGetSendStatsOutput, error) {
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

	return &TAGetSendStatsOutput{
		Body: utils.HumaResponse[[]DailyStats]{
			Code: 200,
			Msg:  "success",
			Data: result,
		},
	}, nil
}

// TAGetTaskStatsInput 获取任务执行占比
type TAGetTaskStatsInput struct {
	Days int `query:"days" default:"30" description:"统计天数，1-90"`
}

// TAGetTaskStatsOutput 获取任务执行占比
type TAGetTaskStatsOutput struct {
	Body utils.HumaResponse[[]TaskStats]
}

// TAGetTaskStats 获取任务执行占比
func (dc *DashboardController) TAGetTaskStats(ctx context.Context, input *TAGetTaskStatsInput) (*TAGetTaskStatsOutput, error) {
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

	return &TAGetTaskStatsOutput{
		Body: utils.HumaResponse[[]TaskStats]{
			Code: 200,
			Msg:  "success",
			Data: stats,
		},
	}, nil
}

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
	}, dc.TAGetStats)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/dashboard/sentence",
		OperationID: "apiGetSentence",
		Summary:     "获取随机古诗词",
		Description: "获取一句随机古诗词",
		Tags:        []string{"仪表盘"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, dc.TAGetSentence)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/dashboard/sendstats",
		OperationID: "apiGetSendStats",
		Summary:     "获取发送统计",
		Description: "获取每日发送统计，默认最近 30 天",
		Tags:        []string{"仪表盘"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, dc.TAGetSendStats)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/dashboard/taskstats",
		OperationID: "apiGetTaskStats",
		Summary:     "获取任务执行占比",
		Description: "获取任务执行统计，默认最近 30 天",
		Tags:        []string{"仪表盘"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, dc.TAGetTaskStats)
}
