package controllers

import (
	"context"
	"net/http"

	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

// ===========================================================================
// /api/v1 管理接口 —— 应用日志
// ===========================================================================

// TAGetLogsInput 获取应用日志列表
type TAGetLogsInput struct {
	Category string `query:"category" description:"日志分类"`
	Status   string `query:"status" description:"日志状态"`
	Level    string `query:"level" description:"日志级别"`
	Keyword  string `query:"keyword" description:"关键字搜索"`
	Page     int    `query:"page" default:"1" description:"页码"`
	PageSize int    `query:"page_size" default:"10" description:"每页数量"`
}

// TAGetLogsOutput 获取应用日志列表
type TAGetLogsOutput struct {
	Body utils.HumaResponse[utils.HumaPagination[[]models.AppLog]]
}

// TAGetLogs 获取应用日志列表
func (ac *AppLogController) TAGetLogs(ctx context.Context, input *TAGetLogsInput) (*TAGetLogsOutput, error) {
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

	return &TAGetLogsOutput{
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

// TAMarkAsReadInput 标记已读
type TAMarkAsReadInput struct {
	Body struct {
		ID       string `json:"id" description:"日志ID"`
		Category string `json:"category" description:"日志分类"`
	}
}

// TAMarkAsReadOutput 标记已读
type TAMarkAsReadOutput struct {
	Body utils.HumaResponse[any]
}

// TAMarkAsRead 标记已读
func (ac *AppLogController) TAMarkAsRead(ctx context.Context, input *TAMarkAsReadInput) (*TAMarkAsReadOutput, error) {
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

	return &TAMarkAsReadOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "标记成功",
		},
	}, nil
}

// TAClearLogsInput 清理日志
type TAClearLogsInput struct {
	Body struct {
		Category string `json:"category" description:"日志分类"`
	}
}

// TAClearLogsOutput 清理日志
type TAClearLogsOutput struct {
	Body utils.HumaResponse[any]
}

// TAClearLogs 清理日志
func (ac *AppLogController) TAClearLogs(ctx context.Context, input *TAClearLogsInput) (*TAClearLogsOutput, error) {
	if err := ac.appLogService.Clear(input.Body.Category); err != nil {
		return nil, utils.HumaBadRequest(err.Error())
	}

	return &TAClearLogsOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "清理成功",
		},
	}, nil
}

// RegisterAPIAppLogRoutes 注册 /api/v1 应用日志 Huma 路由
func (ac *AppLogController) RegisterAPIAppLogRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/app-logs",
		OperationID: "apiGetAppLogs",
		Summary:     "获取应用日志列表",
		Description: "分页获取应用日志列表，支持分类、状态、级别、关键字筛选",
		Tags:        []string{"应用日志"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ac.TAGetLogs)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/app-logs/read",
		OperationID: "apiMarkAppLogsRead",
		Summary:     "标记已读",
		Description: "根据日志ID或分类标记已读",
		Tags:        []string{"应用日志"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ac.TAMarkAsRead)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/app-logs/clear",
		OperationID: "apiClearAppLogs",
		Summary:     "清理日志",
		Description: "清理应用日志，可按分类清理",
		Tags:        []string{"应用日志"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ac.TAClearLogs)
}
