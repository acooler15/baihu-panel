package controllers

import (
	"context"
	"net/http"

	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/models/vo"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/services/relation"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

// ===========================================================================
// OpenAPI (Bearer Token) 接口 —— 环境变量
// ===========================================================================

// OACreateEnvVarInput 创建环境变量（OpenAPI）
type OACreateEnvVarInput struct {
	Body vo.EnvCreateReq
}

// OACreateEnvVarOutput 创建环境变量（OpenAPI）
type OACreateEnvVarOutput struct {
	Body utils.HumaResponse[*vo.EnvVO]
}

// OACreateEnvVar 创建环境变量
func (ec *EnvController) OACreateEnvVar(ctx context.Context, input *OACreateEnvVarInput) (*OACreateEnvVarOutput, error) {
	c := utils.GetGinContext(ctx)
	var userID string
	if c != nil {
		userID = c.GetString("userID")
	}

	req := input.Body

	if req.Type == "" {
		req.Type = constant.EnvTypeNormal
	}

	hidden := true
	if req.Hidden != nil {
		hidden = *req.Hidden
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	envVar := ec.envService.CreateEnvVar(req.Name, req.Value, req.Remark, req.Type, hidden, enabled, userID)
	if envVar != nil {
		relation.DataRelation.SaveTags(envVar.ID, constant.RelationTypeEnvTag, req.Tags)
		envVar.Tags = req.Tags
	}

	services.GetAgentWSManager().BroadcastTasksToAll()

	return &OACreateEnvVarOutput{
		Body: utils.HumaResponse[*vo.EnvVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToEnvVO(envVar),
		},
	}, nil
}

// OAGetEnvVarsInput 获取环境变量列表（OpenAPI）
type OAGetEnvVarsInput struct {
	Name     string `query:"name" description:"按名称模糊查询"`
	Type     string `query:"type" description:"按类型筛选"`
	Tags     string `query:"tags" description:"按标签筛选"`
	Page     int    `query:"page" default:"1" description:"页码"`
	PageSize int    `query:"page_size" default:"10" description:"每页数量"`
}

// OAGetEnvVarsOutput 获取环境变量列表（OpenAPI）
type OAGetEnvVarsOutput struct {
	Body utils.HumaResponse[utils.HumaPagination[[]*vo.EnvVO]]
}

// OAGetEnvVars 获取环境变量列表
func (ec *EnvController) OAGetEnvVars(ctx context.Context, input *OAGetEnvVarsInput) (*OAGetEnvVarsOutput, error) {
	c := utils.GetGinContext(ctx)
	var userID string
	if c != nil {
		userID = c.GetString("userID")
	}

	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 10
	}

	envVars, total := ec.envService.GetEnvVarsWithPagination(userID, input.Name, input.Type, input.Tags, page, pageSize)

	return &OAGetEnvVarsOutput{
		Body: utils.HumaResponse[utils.HumaPagination[[]*vo.EnvVO]]{
			Code: 200,
			Msg:  "success",
			Data: utils.HumaPagination[[]*vo.EnvVO]{
				Data:     vo.ToEnvVOListFromModels(envVars),
				Total:    total,
				Page:     page,
				PageSize: pageSize,
			},
		},
	}, nil
}

// OAGetAllEnvVarsOutput 获取所有环境变量（OpenAPI）
type OAGetAllEnvVarsOutput struct {
	Body utils.HumaResponse[[]*vo.EnvVO]
}

// OAGetAllEnvVars 获取所有环境变量
func (ec *EnvController) OAGetAllEnvVars(ctx context.Context, input *struct{}) (*OAGetAllEnvVarsOutput, error) {
	c := utils.GetGinContext(ctx)
	var userID string
	if c != nil {
		userID = c.GetString("userID")
	}

	envVars := ec.envService.GetEnvVarsByUserID(userID)

	return &OAGetAllEnvVarsOutput{
		Body: utils.HumaResponse[[]*vo.EnvVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToEnvVOListFromModels(envVars),
		},
	}, nil
}

// OAGetEnvVarInput 获取环境变量详情（OpenAPI）
type OAGetEnvVarInput struct {
	ID string `path:"id" description:"环境变量ID"`
}

// OAGetEnvVarOutput 获取环境变量详情（OpenAPI）
type OAGetEnvVarOutput struct {
	Body utils.HumaResponse[*vo.EnvVO]
}

// OAGetEnvVar 获取环境变量详情
func (ec *EnvController) OAGetEnvVar(ctx context.Context, input *OAGetEnvVarInput) (*OAGetEnvVarOutput, error) {
	id := input.ID
	if id == "" {
		return nil, utils.HumaBadRequest("无效的环境变量ID")
	}

	envVar := ec.envService.GetEnvVarByID(id)
	if envVar == nil {
		return nil, utils.HumaNotFound("环境变量不存在")
	}

	return &OAGetEnvVarOutput{
		Body: utils.HumaResponse[*vo.EnvVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToEnvVO(envVar),
		},
	}, nil
}

// OAGetAssociatedTasksInput 获取关联任务（OpenAPI）
type OAGetAssociatedTasksInput struct {
	ID string `path:"id" description:"环境变量ID"`
}

// OAGetAssociatedTasksOutput 获取关联任务（OpenAPI）
type OAGetAssociatedTasksOutput struct {
	Body utils.HumaResponse[[]*vo.TaskVO]
}

// OAGetAssociatedTasks 获取关联任务
func (ec *EnvController) OAGetAssociatedTasks(ctx context.Context, input *OAGetAssociatedTasksInput) (*OAGetAssociatedTasksOutput, error) {
	id := input.ID
	if id == "" {
		return nil, utils.HumaBadRequest("无效的环境变量ID")
	}

	tasks := ec.envService.GetAssociatedTasks(id)

	return &OAGetAssociatedTasksOutput{
		Body: utils.HumaResponse[[]*vo.TaskVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToTaskVOListFromModels(tasks),
		},
	}, nil
}

// OAUpdateEnvVarInput 更新环境变量（OpenAPI）
type OAUpdateEnvVarInput struct {
	ID   string          `path:"id" description:"环境变量ID"`
	Body vo.EnvUpdateReq
}

// OAUpdateEnvVarOutput 更新环境变量（OpenAPI）
type OAUpdateEnvVarOutput struct {
	Body utils.HumaResponse[*vo.EnvVO]
}

// OAUpdateEnvVar 更新环境变量
func (ec *EnvController) OAUpdateEnvVar(ctx context.Context, input *OAUpdateEnvVarInput) (*OAUpdateEnvVarOutput, error) {
	id := input.ID
	if id == "" {
		return nil, utils.HumaBadRequest("无效的环境变量ID")
	}

	req := input.Body

	if req.Type == "" {
		req.Type = constant.EnvTypeNormal
	}

	existing := ec.envService.GetEnvVarByID(id)
	if existing == nil {
		return nil, utils.HumaNotFound("环境变量不存在")
	}

	hidden := existing.Hidden
	if req.Hidden != nil {
		hidden = req.Hidden
	}

	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = req.Enabled
	}

	envVar := ec.envService.UpdateEnvVar(id, req.Name, req.Value, req.Remark, req.Type, utils.DerefBool(hidden, true), utils.DerefBool(enabled, true))
	if envVar == nil {
		return nil, utils.HumaNotFound("环境变量不存在")
	}

	relation.DataRelation.SaveTags(envVar.ID, constant.RelationTypeEnvTag, req.Tags)
	envVar.Tags = req.Tags

	services.GetAgentWSManager().BroadcastTasksToAll()

	return &OAUpdateEnvVarOutput{
		Body: utils.HumaResponse[*vo.EnvVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToEnvVO(envVar),
		},
	}, nil
}

// OADeleteEnvVarInput 删除环境变量（OpenAPI）
type OADeleteEnvVarInput struct {
	ID    string `path:"id" description:"环境变量ID"`
	Force bool   `query:"force" description:"强制删除（忽略任务关联）"`
}

// OADeleteEnvVarOutput 删除环境变量（OpenAPI）
type OADeleteEnvVarOutput struct {
	Body utils.HumaResponse[any]
}

// OADeleteEnvVar 删除环境变量
func (ec *EnvController) OADeleteEnvVar(ctx context.Context, input *OADeleteEnvVarInput) (*OADeleteEnvVarOutput, error) {
	id := input.ID
	if id == "" {
		return nil, utils.HumaBadRequest("无效的环境变量ID")
	}

	success, associatedTasks := ec.envService.DeleteEnvVar(id, input.Force)

	if len(associatedTasks) > 0 {
		// 保持与旧 Gin 行为一致：HTTP 200，body 中 code=409 并携带关联任务数据
		return &OADeleteEnvVarOutput{
			Body: utils.HumaResponse[any]{
				Code: 409,
				Msg:  "该环境变量已被任务引用，请先在任务中删除引用或选择强制删除",
				Data: vo.ToTaskVOListFromModels(associatedTasks),
			},
		}, nil
	}

	if !success {
		return nil, utils.HumaNotFound("环境变量不存在或删除失败")
	}

	services.GetAgentWSManager().BroadcastTasksToAll()

	return &OADeleteEnvVarOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "删除成功",
		},
	}, nil
}

// RegisterOpenAPIEnvRoutes 注册 OpenAPI 环境变量相关 Huma 路由
func (ec *EnvController) RegisterOpenAPIEnvRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/env",
		OperationID: "openapiCreateEnvVar",
		Summary:     "创建环境变量",
		Description: "创建一个新的环境变量",
		Tags:        []string{"环境变量"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, ec.OACreateEnvVar)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/env",
		OperationID: "openapiGetEnvVars",
		Summary:     "获取环境变量列表",
		Description: "分页获取环境变量列表，支持按名称筛选",
		Tags:        []string{"环境变量"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, ec.OAGetEnvVars)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/env/all",
		OperationID: "openapiGetAllEnvVars",
		Summary:     "获取所有环境变量",
		Description: "获取当前用户的所有环境变量（不分页）",
		Tags:        []string{"环境变量"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, ec.OAGetAllEnvVars)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/env/{id}",
		OperationID: "openapiGetEnvVar",
		Summary:     "获取环境变量详情",
		Description: "根据 ID 获取环境变量详情",
		Tags:        []string{"环境变量"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, ec.OAGetEnvVar)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/env/{id}/tasks",
		OperationID: "openapiGetAssociatedTasks",
		Summary:     "获取关联任务",
		Description: "获取引用了该环境变量的任务列表",
		Tags:        []string{"环境变量"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, ec.OAGetAssociatedTasks)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPut,
		Path:        "/env/{id}",
		OperationID: "openapiUpdateEnvVar",
		Summary:     "更新环境变量",
		Description: "根据 ID 更新环境变量信息",
		Tags:        []string{"环境变量"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, ec.OAUpdateEnvVar)

	huma.Register(api, huma.Operation{
		Method:      http.MethodDelete,
		Path:        "/env/{id}",
		OperationID: "openapiDeleteEnvVar",
		Summary:     "删除环境变量",
		Description: "根据 ID 删除环境变量",
		Tags:        []string{"环境变量"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, ec.OADeleteEnvVar)
}
