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

type LogController struct{}

func NewLogController() *LogController {
	return &LogController{}
}

// ===========================================================================
// 日志管理业务方法
// ===========================================================================

// GetLogsInput 获取任务日志列表
type GetLogsInput struct {
	TaskID   string `query:"task_id" description:"任务 ID"`
	TaskName string `query:"task_name" description:"任务名称模糊搜索"`
	Status   string `query:"status" description:"日志状态"`
	Page     int    `query:"page" default:"1" description:"页码"`
	PageSize int    `query:"page_size" default:"10" description:"每页数量"`
}

// GetLogsOutput 获取任务日志列表
type GetLogsOutput struct {
	Body utils.HumaResponse[utils.HumaPagination[[]vo.TaskLogVO]]
}

// GetLogs 获取任务日志列表
func (lc *LogController) GetLogs(ctx context.Context, input *GetLogsInput) (*GetLogsOutput, error) {
	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 10
	}

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
			return &GetLogsOutput{
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
	query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs)

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

	return &GetLogsOutput{
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

// GetLogDetailInput 获取日志详情
type GetLogDetailInput struct {
	ID string `path:"id" description:"日志ID"`
}

// GetLogDetailOutput 获取日志详情
type GetLogDetailOutput struct {
	Body utils.HumaResponse[*vo.TaskLogVO]
}

// GetLogDetail 获取日志详情
func (lc *LogController) GetLogDetail(ctx context.Context, input *GetLogDetailInput) (*GetLogDetailOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的日志ID")
	}

	var log models.TaskLog
	res := database.DB.Where("id = ?", input.ID).Limit(1).Find(&log)
	if res.Error != nil || res.RowsAffected == 0 {
		return nil, utils.HumaNotFound("日志不存在")
	}

	return &GetLogDetailOutput{
		Body: utils.HumaResponse[*vo.TaskLogVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToTaskLogVO(&log),
		},
	}, nil
}

// ClearLogsInput 清空日志
type ClearLogsInput struct {
	Body struct {
		TaskID *string `json:"task_id" description:"任务 ID，留空则清空全部"`
	}
}

// ClearLogsOutput 清空日志
type ClearLogsOutput struct {
	Body utils.HumaResponse[any]
}

// ClearLogs 清空日志
func (lc *LogController) ClearLogs(ctx context.Context, input *ClearLogsInput) (*ClearLogsOutput, error) {
	query := database.DB.Model(&models.TaskLog{})
	if input.Body.TaskID != nil && *input.Body.TaskID != "" {
		query = query.Where("task_id = ?", *input.Body.TaskID)
	} else {
		query = query.Where("1 = 1") // 允许无 GORM 安全保护地清空全部
	}

	if err := query.Delete(&models.TaskLog{}).Error; err != nil {
		return nil, utils.HumaServerError("清空日志失败")
	}

	return &ClearLogsOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "日志清空成功",
		},
	}, nil
}

// DeleteLogInput 删除日志
type DeleteLogInput struct {
	ID string `path:"id" description:"日志ID"`
}

// DeleteLogOutput 删除日志
type DeleteLogOutput struct {
	Body utils.HumaResponse[any]
}

// DeleteLog 删除日志
func (lc *LogController) DeleteLog(ctx context.Context, input *DeleteLogInput) (*DeleteLogOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的日志ID")
	}

	if err := database.DB.Where("id = ?", input.ID).Delete(&models.TaskLog{}).Error; err != nil {
		return nil, utils.HumaServerError("删除日志失败")
	}

	return &DeleteLogOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "日志已删除",
		},
	}, nil
}

// ===========================================================================
// 路由注册
// ===========================================================================

// registerLogRoutes 注册日志管理共用路由（2 条）。
// security 直接作为 OpenAPI 文档的 Security 声明。
func (lc *LogController) registerLogRoutes(api huma.API, security []map[string][]string) {
	tag := []string{"日志管理"}

	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/logs", OperationID: "GetLogs", Summary: "获取任务日志列表", Description: "分页获取任务日志列表，支持任务 ID、名称、状态筛选", Tags: tag, Security: security}, lc.GetLogs)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/logs/{id}", OperationID: "GetLogDetail", Summary: "获取日志详情", Description: "根据 ID 获取日志详情", Tags: tag, Security: security}, lc.GetLogDetail)
}

// RegisterAPILogRoutes 注册 /api/v1 日志管理 Huma 路由（CookieAuth）
// 含独有接口：ClearLogs、DeleteLog
func (lc *LogController) RegisterAPILogRoutes(api huma.API) {
	security := []map[string][]string{{"CookieAuth": {}}}
	tag := []string{"日志管理"}

	// 独有接口
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/logs/clear", OperationID: "ClearLogs", Summary: "清空日志", Description: "清空任务日志，可按任务 ID 清空或清空全部", Tags: tag, Security: security}, lc.ClearLogs)
	huma.Register(api, huma.Operation{Method: http.MethodDelete, Path: "/logs/{id}", OperationID: "DeleteLog", Summary: "删除日志", Description: "根据 ID 删除日志", Tags: tag, Security: security}, lc.DeleteLog)

	// 共用接口
	lc.registerLogRoutes(api, security)
}

// RegisterOpenAPILogRoutes 注册 OpenAPI 日志管理 Huma 路由（BearerAuth，子集）
func (lc *LogController) RegisterOpenAPILogRoutes(api huma.API) {
	lc.registerLogRoutes(api, []map[string][]string{{"BearerAuth": {}}})
}
