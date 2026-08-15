package controllers

import (
	"context"
	"net/http"

	"github.com/engigu/baihu-panel/internal/database"
	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/models/vo"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

// ===========================================================================
// OpenAPI (Bearer Token) 接口 —— 日志管理
// ===========================================================================

// OAGetLogsInput 获取任务日志列表（OpenAPI）
type OAGetLogsInput struct {
	TaskID   string `query:"task_id" description:"任务 ID"`
	TaskName string `query:"task_name" description:"任务名称"`
	Status   string `query:"status" description:"状态"`
	Page     int    `query:"page" default:"1" description:"页码"`
	PageSize int    `query:"page_size" default:"10" description:"每页数量"`
}

// OAGetLogsOutput 获取任务日志列表（OpenAPI）
type OAGetLogsOutput struct {
	Body utils.HumaResponse[utils.HumaPagination[[]vo.TaskLogVO]]
}

// OAGetLogs 获取任务日志列表
func (lc *LogController) OAGetLogs(ctx context.Context, input *OAGetLogsInput) (*OAGetLogsOutput, error) {
	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 10
	}
	p := utils.Pagination{Page: page, PageSize: pageSize}

	taskID := input.TaskID
	taskName := input.TaskName
	status := input.Status

	var logs []models.TaskLog
	var total int64

	query := database.DB.Model(&models.TaskLog{})
	if taskID != "" {
		query = query.Where("task_id = ?", taskID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	// 按任务名称过滤
	if taskName != "" {
		var taskIDs []string
		database.DB.Model(&models.Task{}).Where("name LIKE ?", "%"+taskName+"%").Pluck("id", &taskIDs)
		if len(taskIDs) > 0 {
			query = query.Where("task_id IN ?", taskIDs)
		} else {
			return &OAGetLogsOutput{
				Body: utils.HumaResponse[utils.HumaPagination[[]vo.TaskLogVO]]{
					Code: 200,
					Msg:  "success",
					Data: utils.HumaPagination[[]vo.TaskLogVO]{
						Data:     []vo.TaskLogVO{},
						Total:    0,
						Page:     page,
						PageSize: pageSize,
					},
				},
			}, nil
		}
	}

	query.Count(&total)
	query.Order("id DESC").Offset(p.Offset()).Limit(p.PageSize).Find(&logs)

	taskIDList := make([]string, 0)
	for _, log := range logs {
		taskIDList = append(taskIDList, log.TaskID)
	}

	var tasks []models.Task
	database.DB.Where("id IN ?", taskIDList).Find(&tasks)
	taskMap := make(map[string]models.Task)
	for _, t := range tasks {
		taskMap[t.ID] = t
	}

	result := make([]vo.TaskLogVO, len(logs))
	for i, log := range logs {
		task := taskMap[log.TaskID]
		taskType := task.Type
		if taskType == "" {
			taskType = "task"
		}
		result[i] = vo.TaskLogVO{
			ID:        log.ID,
			TaskID:    log.TaskID,
			TaskName:  task.Name,
			TaskType:  taskType,
			AgentID:   log.AgentID,
			Command:   string(log.Command),
			Status:    log.Status,
			Duration:  log.Duration,
			StartTime: log.StartTime,
			EndTime:   log.EndTime,
			CreatedAt: log.CreatedAt,
		}
	}

	return &OAGetLogsOutput{
		Body: utils.HumaResponse[utils.HumaPagination[[]vo.TaskLogVO]]{
			Code: 200,
			Msg:  "success",
			Data: utils.HumaPagination[[]vo.TaskLogVO]{
				Data:     result,
				Total:    total,
				Page:     page,
				PageSize: pageSize,
			},
		},
	}, nil
}

// OAGetLogDetailInput 获取日志详情（OpenAPI）
type OAGetLogDetailInput struct {
	ID string `path:"id" description:"日志ID"`
}

// OAGetLogDetailOutput 获取日志详情（OpenAPI）
type OAGetLogDetailOutput struct {
	Body utils.HumaResponse[*vo.TaskLogVO]
}

// OAGetLogDetail 获取日志详情
func (lc *LogController) OAGetLogDetail(ctx context.Context, input *OAGetLogDetailInput) (*OAGetLogDetailOutput, error) {
	id := input.ID
	if id == "" {
		return nil, utils.HumaBadRequest("无效的日志ID")
	}

	var log models.TaskLog
	res := database.DB.Where("id = ?", id).Limit(1).Find(&log)
	if res.Error != nil || res.RowsAffected == 0 {
		return nil, utils.HumaNotFound("日志不存在")
	}

	return &OAGetLogDetailOutput{
		Body: utils.HumaResponse[*vo.TaskLogVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToTaskLogVO(&log),
		},
	}, nil
}

// RegisterOpenAPILogRoutes 注册 OpenAPI 日志相关 Huma 路由
func (lc *LogController) RegisterOpenAPILogRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/logs",
		OperationID: "openapiGetLogs",
		Summary:     "获取任务日志列表",
		Description: "分页获取任务日志列表，支持按任务 ID、任务名称、状态筛选",
		Tags:        []string{"日志管理"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, lc.OAGetLogs)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/logs/{id}",
		OperationID: "openapiGetLogDetail",
		Summary:     "获取日志详情",
		Description: "根据 ID 获取任务日志详细内容（包含输出）",
		Tags:        []string{"日志管理"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, lc.OAGetLogDetail)
}
