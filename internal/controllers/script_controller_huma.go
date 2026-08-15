package controllers

import (
	"context"
	"net/http"

	"github.com/engigu/baihu-panel/internal/models/vo"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

// ===========================================================================
// OpenAPI (Bearer Token) 接口 —— 脚本管理
// ===========================================================================

// OACreateScriptInput 创建脚本（OpenAPI）
type OACreateScriptInput struct {
	Body vo.ScriptCreateReq
}

// OACreateScriptOutput 创建脚本（OpenAPI）
type OACreateScriptOutput struct {
	Body utils.HumaResponse[*vo.ScriptVO]
}

// OACreateScript 创建脚本
func (sc *ScriptController) OACreateScript(ctx context.Context, input *OACreateScriptInput) (*OACreateScriptOutput, error) {
	c := utils.GetGinContext(ctx)
	var userID string
	if c != nil {
		userID = c.GetString("userID")
	}

	req := input.Body

	script := sc.scriptService.CreateScript(req.Name, req.Content, userID)

	return &OACreateScriptOutput{
		Body: utils.HumaResponse[*vo.ScriptVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToScriptVO(script),
		},
	}, nil
}

// OAGetScriptsOutput 获取脚本列表（OpenAPI）
type OAGetScriptsOutput struct {
	Body utils.HumaResponse[[]*vo.ScriptVO]
}

// OAGetScripts 获取脚本列表
func (sc *ScriptController) OAGetScripts(ctx context.Context, input *struct{}) (*OAGetScriptsOutput, error) {
	c := utils.GetGinContext(ctx)
	var userID string
	if c != nil {
		userID = c.GetString("userID")
	}

	scripts := sc.scriptService.GetScriptsByUserID(userID)
	vos := vo.ToScriptVOListFromModels(scripts)
	for i := range vos {
		vos[i].Content = "" // 列表不返回内容
	}

	return &OAGetScriptsOutput{
		Body: utils.HumaResponse[[]*vo.ScriptVO]{
			Code: 200,
			Msg:  "success",
			Data: vos,
		},
	}, nil
}

// OAGetScriptInput 获取脚本详情（OpenAPI）
type OAGetScriptInput struct {
	ID string `path:"id" description:"脚本ID"`
}

// OAGetScriptOutput 获取脚本详情（OpenAPI）
type OAGetScriptOutput struct {
	Body utils.HumaResponse[*vo.ScriptVO]
}

// OAGetScript 获取脚本详情
func (sc *ScriptController) OAGetScript(ctx context.Context, input *OAGetScriptInput) (*OAGetScriptOutput, error) {
	id := input.ID
	if id == "" {
		return nil, utils.HumaBadRequest("无效的脚本ID")
	}

	script := sc.scriptService.GetScriptByID(id)
	if script == nil {
		return nil, utils.HumaNotFound("脚本不存在")
	}

	return &OAGetScriptOutput{
		Body: utils.HumaResponse[*vo.ScriptVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToScriptVO(script),
		},
	}, nil
}

// OAUpdateScriptInput 更新脚本（OpenAPI）
type OAUpdateScriptInput struct {
	ID   string            `path:"id" description:"脚本ID"`
	Body vo.ScriptUpdateReq
}

// OAUpdateScriptOutput 更新脚本（OpenAPI）
type OAUpdateScriptOutput struct {
	Body utils.HumaResponse[*vo.ScriptVO]
}

// OAUpdateScript 更新脚本
func (sc *ScriptController) OAUpdateScript(ctx context.Context, input *OAUpdateScriptInput) (*OAUpdateScriptOutput, error) {
	id := input.ID
	if id == "" {
		return nil, utils.HumaBadRequest("无效的脚本ID")
	}

	req := input.Body

	script := sc.scriptService.UpdateScript(id, req.Name, req.Content)
	if script == nil {
		return nil, utils.HumaNotFound("脚本不存在")
	}

	return &OAUpdateScriptOutput{
		Body: utils.HumaResponse[*vo.ScriptVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToScriptVO(script),
		},
	}, nil
}

// OADeleteScriptInput 删除脚本（OpenAPI）
type OADeleteScriptInput struct {
	ID string `path:"id" description:"脚本ID"`
}

// OADeleteScriptOutput 删除脚本（OpenAPI）
type OADeleteScriptOutput struct {
	Body utils.HumaResponse[any]
}

// OADeleteScript 删除脚本
func (sc *ScriptController) OADeleteScript(ctx context.Context, input *OADeleteScriptInput) (*OADeleteScriptOutput, error) {
	id := input.ID
	if id == "" {
		return nil, utils.HumaBadRequest("无效的脚本ID")
	}

	success := sc.scriptService.DeleteScript(id)
	if !success {
		return nil, utils.HumaNotFound("脚本不存在")
	}

	return &OADeleteScriptOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "删除成功",
		},
	}, nil
}

// RegisterOpenAPIScriptRoutes 注册 OpenAPI 脚本相关 Huma 路由
func (sc *ScriptController) RegisterOpenAPIScriptRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/scripts",
		OperationID: "openapiCreateScript",
		Summary:     "创建脚本",
		Description: "创建一个新的脚本",
		Tags:        []string{"脚本管理"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, sc.OACreateScript)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/scripts",
		OperationID: "openapiGetScripts",
		Summary:     "获取脚本列表",
		Description: "获取当前用户的所有脚本（内容字段为空）",
		Tags:        []string{"脚本管理"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, sc.OAGetScripts)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/scripts/{id}",
		OperationID: "openapiGetScript",
		Summary:     "获取脚本详情",
		Description: "根据 ID 获取脚本详情",
		Tags:        []string{"脚本管理"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, sc.OAGetScript)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPut,
		Path:        "/scripts/{id}",
		OperationID: "openapiUpdateScript",
		Summary:     "更新脚本",
		Description: "根据 ID 更新脚本信息",
		Tags:        []string{"脚本管理"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, sc.OAUpdateScript)

	huma.Register(api, huma.Operation{
		Method:      http.MethodDelete,
		Path:        "/scripts/{id}",
		OperationID: "openapiDeleteScript",
		Summary:     "删除脚本",
		Description: "根据 ID 删除脚本",
		Tags:        []string{"脚本管理"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, sc.OADeleteScript)
}
