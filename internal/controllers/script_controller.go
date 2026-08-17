package controllers

import (
	"context"
	"net/http"

	"github.com/engigu/baihu-panel/internal/models/vo"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

type ScriptController struct {
	scriptService *services.ScriptService
}

func NewScriptController(scriptService *services.ScriptService) *ScriptController {
	return &ScriptController{scriptService: scriptService}
}

// ===========================================================================
// 脚本管理业务方法
// ===========================================================================

// CreateScriptInput 创建脚本
type CreateScriptInput struct {
	Body struct {
		Name    string `json:"name" description:"脚本名称"`
		Content string `json:"content" description:"脚本内容"`
	}
}

// CreateScriptOutput 创建脚本
type CreateScriptOutput struct {
	Body utils.HumaResponse[*vo.ScriptVO]
}

// CreateScript 创建脚本
func (sc *ScriptController) CreateScript(ctx context.Context, input *CreateScriptInput) (*CreateScriptOutput, error) {
	userID := ""
	if c := utils.GetGinContext(ctx); c != nil {
		userID = c.GetString("userID")
	}

	req := input.Body
	if req.Name == "" || req.Content == "" {
		return nil, utils.HumaBadRequest("脚本名称和内容不能为空")
	}

	script := sc.scriptService.CreateScript(req.Name, req.Content, userID)

	return &CreateScriptOutput{
		Body: utils.HumaResponse[*vo.ScriptVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToScriptVO(script),
		},
	}, nil
}

// GetScriptsOutput 获取脚本列表
type GetScriptsOutput struct {
	Body utils.HumaResponse[[]*vo.ScriptVO]
}

// GetScripts 获取脚本列表
func (sc *ScriptController) GetScripts(ctx context.Context, input *struct{}) (*GetScriptsOutput, error) {
	userID := ""
	if c := utils.GetGinContext(ctx); c != nil {
		userID = c.GetString("userID")
	}

	scripts := sc.scriptService.GetScriptsByUserID(userID)
	vos := vo.ToScriptVOListFromModels(scripts)
	for i := range vos {
		vos[i].Content = "" // 列表不返回内容
	}

	return &GetScriptsOutput{
		Body: utils.HumaResponse[[]*vo.ScriptVO]{
			Code: 200,
			Msg:  "success",
			Data: vos,
		},
	}, nil
}

// GetScriptInput 获取脚本详情
type GetScriptInput struct {
	ID string `path:"id" description:"脚本ID"`
}

// GetScriptOutput 获取脚本详情
type GetScriptOutput struct {
	Body utils.HumaResponse[*vo.ScriptVO]
}

// GetScript 获取脚本详情
func (sc *ScriptController) GetScript(ctx context.Context, input *GetScriptInput) (*GetScriptOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的脚本ID")
	}

	script := sc.scriptService.GetScriptByID(input.ID)
	if script == nil {
		return nil, utils.HumaNotFound("脚本不存在")
	}

	return &GetScriptOutput{
		Body: utils.HumaResponse[*vo.ScriptVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToScriptVO(script),
		},
	}, nil
}

// UpdateScriptInput 更新脚本
type UpdateScriptInput struct {
	ID   string `path:"id" description:"脚本ID"`
	Body struct {
		Name    string `json:"name" description:"脚本名称"`
		Content string `json:"content" description:"脚本内容"`
	}
}

// UpdateScriptOutput 更新脚本
type UpdateScriptOutput struct {
	Body utils.HumaResponse[*vo.ScriptVO]
}

// UpdateScript 更新脚本
func (sc *ScriptController) UpdateScript(ctx context.Context, input *UpdateScriptInput) (*UpdateScriptOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的脚本ID")
	}

	script := sc.scriptService.UpdateScript(input.ID, input.Body.Name, input.Body.Content)
	if script == nil {
		return nil, utils.HumaNotFound("脚本不存在")
	}

	return &UpdateScriptOutput{
		Body: utils.HumaResponse[*vo.ScriptVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToScriptVO(script),
		},
	}, nil
}

// DeleteScriptInput 删除脚本
type DeleteScriptInput struct {
	ID string `path:"id" description:"脚本ID"`
}

// DeleteScriptOutput 删除脚本
type DeleteScriptOutput struct {
	Body utils.HumaResponse[any]
}

// DeleteScript 删除脚本
func (sc *ScriptController) DeleteScript(ctx context.Context, input *DeleteScriptInput) (*DeleteScriptOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的脚本ID")
	}

	success := sc.scriptService.DeleteScript(input.ID)
	if !success {
		return nil, utils.HumaNotFound("脚本不存在")
	}

	return &DeleteScriptOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "删除成功",
		},
	}, nil
}

// ===========================================================================
// 路由注册
// ===========================================================================

// registerScriptRoutes 注册脚本管理共用路由（5 条）。
// security 直接作为 OpenAPI 文档的 Security 声明。
func (sc *ScriptController) registerScriptRoutes(api huma.API, security []map[string][]string) {
	tag := []string{"脚本管理"}

	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/scripts", OperationID: "CreateScript", Summary: "创建脚本", Description: "创建一个新的脚本", Tags: tag, Security: security}, sc.CreateScript)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/scripts", OperationID: "GetScripts", Summary: "获取脚本列表", Description: "获取当前用户的脚本列表（不包含内容）", Tags: tag, Security: security}, sc.GetScripts)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/scripts/{id}", OperationID: "GetScript", Summary: "获取脚本详情", Description: "根据 ID 获取脚本详情", Tags: tag, Security: security}, sc.GetScript)
	huma.Register(api, huma.Operation{Method: http.MethodPut, Path: "/scripts/{id}", OperationID: "UpdateScript", Summary: "更新脚本", Description: "根据 ID 更新脚本", Tags: tag, Security: security}, sc.UpdateScript)
	huma.Register(api, huma.Operation{Method: http.MethodDelete, Path: "/scripts/{id}", OperationID: "DeleteScript", Summary: "删除脚本", Description: "根据 ID 删除脚本", Tags: tag, Security: security}, sc.DeleteScript)
}

// RegisterAPIScriptRoutes 注册 /api/v1 脚本管理 Huma 路由（CookieAuth）
func (sc *ScriptController) RegisterAPIScriptRoutes(api huma.API) {
	sc.registerScriptRoutes(api, []map[string][]string{{"CookieAuth": {}}})
}

// RegisterOpenAPIScriptRoutes 注册 OpenAPI 脚本管理 Huma 路由（BearerAuth）
func (sc *ScriptController) RegisterOpenAPIScriptRoutes(api huma.API) {
	sc.registerScriptRoutes(api, []map[string][]string{{"BearerAuth": {}}})
}
