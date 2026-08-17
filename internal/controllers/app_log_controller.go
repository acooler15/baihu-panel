package controllers

import (
	"context"
	"net/http"

	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

type AppLogController struct {
	appLogService *services.AppLogService
}

func NewAppLogController() *AppLogController {
	return &AppLogController{
		appLogService: services.NewAppLogService(),
	}
}

// ===========================================================================
// 应用日志业务方法
// ===========================================================================

// GetAppLogsInput 获取应用日志列表
type GetAppLogsInput struct {
	Category string `query:"category" description:"日志分类"`
	Status   string `query:"status" description:"日志状态"`
	Level    string `query:"level" description:"日志级别"`
	Keyword  string `query:"keyword" description:"关键字搜索"`
	Page     int    `query:"page" default:"1" description:"页码"`
	PageSize int    `query:"page_size" default:"10" description:"每页数量"`
}

// GetAppLogsOutput 获取应用日志列表
type GetAppLogsOutput struct {
	Body utils.HumaResponse[utils.HumaPagination[[]models.AppLog]]
}

// GetAppLogs 获取应用日志列表
func (ac *AppLogController) GetAppLogs(ctx context.Context, input *GetAppLogsInput) (*GetAppLogsOutput, error) {
	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 10
	}

	logs, total, err := ac.appLogService.List(input.Category, input.Status, input.Level, page, pageSize, input.Keyword)
	if err != nil {
		return nil, utils.HumaBadRequest(err.Error())
	}

	return &GetAppLogsOutput{
		Body: utils.HumaResponse[utils.HumaPagination[[]models.AppLog]]{
			Code: 200,
			Msg:  "success",
			Data: utils.HumaPagination[[]models.AppLog]{
				Data:     logs,
				Total:    total,
				Page:     page,
				PageSize: pageSize,
			},
		},
	}, nil
}

// MarkAppLogsReadInput 标记已读
type MarkAppLogsReadInput struct {
	Body struct {
		ID       string `json:"id" description:"日志ID"`
		Category string `json:"category" description:"日志分类"`
	}
}

// MarkAppLogsReadOutput 标记已读
type MarkAppLogsReadOutput struct {
	Body utils.HumaResponse[any]
}

// MarkAppLogsRead 标记已读
func (ac *AppLogController) MarkAppLogsRead(ctx context.Context, input *MarkAppLogsReadInput) (*MarkAppLogsReadOutput, error) {
	req := input.Body
	if req.ID != "" {
		if err := ac.appLogService.MarkAsRead(req.ID); err != nil {
			return nil, utils.HumaBadRequest(err.Error())
		}
	} else if req.Category != "" {
		if err := ac.appLogService.MarkAllAsRead(req.Category); err != nil {
			return nil, utils.HumaBadRequest(err.Error())
		}
	} else {
		return nil, utils.HumaBadRequest("id 或 category 必须提供")
	}

	return &MarkAppLogsReadOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "标记成功",
		},
	}, nil
}

// ClearAppLogsInput 清理日志
type ClearAppLogsInput struct {
	Body struct {
		Category string `json:"category" description:"日志分类"`
	}
}

// ClearAppLogsOutput 清理日志
type ClearAppLogsOutput struct {
	Body utils.HumaResponse[any]
}

// ClearAppLogs 清理日志
func (ac *AppLogController) ClearAppLogs(ctx context.Context, input *ClearAppLogsInput) (*ClearAppLogsOutput, error) {
	if err := ac.appLogService.Clear(input.Body.Category); err != nil {
		return nil, utils.HumaBadRequest(err.Error())
	}

	return &ClearAppLogsOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "清理成功",
		},
	}, nil
}

// ===========================================================================
// 路由注册
// ===========================================================================

// RegisterAPIAppLogRoutes 注册 /api/v1 应用日志 Huma 路由
func (ac *AppLogController) RegisterAPIAppLogRoutes(api huma.API) {
	security := []map[string][]string{{"CookieAuth": {}}}
	tag := []string{"应用日志"}

	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/app-logs", OperationID: "GetAppLogs", Summary: "获取应用日志列表", Description: "分页获取应用日志列表，支持分类、状态、级别、关键字筛选", Tags: tag, Security: security}, ac.GetAppLogs)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/app-logs/read", OperationID: "MarkAppLogsRead", Summary: "标记已读", Description: "根据日志ID或分类标记已读", Tags: tag, Security: security}, ac.MarkAppLogsRead)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/app-logs/clear", OperationID: "ClearAppLogs", Summary: "清理日志", Description: "清理应用日志，可按分类清理", Tags: tag, Security: security}, ac.ClearAppLogs)
}
