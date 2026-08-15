package controllers

import (
	"context"
	"net/http"

	"github.com/engigu/baihu-panel/internal/models/vo"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

// ===========================================================================
// /api/v1 管理接口 —— 脚本管理
// ===========================================================================

// TACreateScriptInput 创建脚本
type TACreateScriptInput struct {
	Body struct {
		Name    string `json:"name" description:"脚本名称"`
		Content string `json:"content" description:"脚本内容"`
	}
}

// TACreateScriptOutput 创建脚本
type TACreateScriptOutput struct {
	Body utils.HumaResponse[*vo.ScriptVO]
}

// TACreateScript 创建脚本
func (sc *ScriptController) TACreateScript(ctx context.Context, input *TACreateScriptInput) (*TACreateScriptOutput, error) {
	userID := ""
	if c := utils.GetGinContext(ctx); c != nil {
		userID = c.GetString("userID")
	}

	req := input.Body
	if req.Name == "" || req.Content == "" {
		return nil, utils.HumaBadRequest("脚本名称和内容不能为空")
	}

	script := sc.scriptService.CreateScript(req.Name, req.Content, userID)

	return &TACreateScriptOutput{
		Body: utils.HumaResponse[*vo.ScriptVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToScriptVO(script),
		},
	}, nil
}

// TAGetScriptsOutput 获取脚本列表
type TAGetScriptsOutput struct {
	Body utils.HumaResponse[[]*vo.ScriptVO]
}

// TAGetScripts 获取脚本列表
func (sc *ScriptController) TAGetScripts(ctx context.Context, input *struct{}) (*TAGetScriptsOutput, error) {
	userID := ""
	if c := utils.GetGinContext(ctx); c != nil {
		userID = c.GetString("userID")
	}

	scripts := sc.scriptService.GetScriptsByUserID(userID)
	vos := vo.ToScriptVOListFromModels(scripts)
	for i := range vos {
		vos[i].Content = "" // 列表不返回内容
	}

	return &TAGetScriptsOutput{
		Body: utils.HumaResponse[[]*vo.ScriptVO]{
			Code: 200,
			Msg:  "success",
			Data: vos,
		},
	}, nil
}

// TAGetScriptInput 获取脚本详情
type TAGetScriptInput struct {
	ID string `path:"id" description:"脚本ID"`
}

// TAGetScriptOutput 获取脚本详情
type TAGetScriptOutput struct {
	Body utils.HumaResponse[*vo.ScriptVO]
}

// TAGetScript 获取脚本详情
func (sc *ScriptController) TAGetScript(ctx context.Context, input *TAGetScriptInput) (*TAGetScriptOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的脚本ID")
	}

	script := sc.scriptService.GetScriptByID(input.ID)
	if script == nil {
		return nil, utils.HumaNotFound("脚本不存在")
	}

	return &TAGetScriptOutput{
		Body: utils.HumaResponse[*vo.ScriptVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToScriptVO(script),
		},
	}, nil
}

// TAUpdateScriptInput 更新脚本
type TAUpdateScriptInput struct {
	ID   string `path:"id" description:"脚本ID"`
	Body struct {
		Name    string `json:"name" description:"脚本名称"`
		Content string `json:"content" description:"脚本内容"`
	}
}

// TAUpdateScriptOutput 更新脚本
type TAUpdateScriptOutput struct {
	Body utils.HumaResponse[*vo.ScriptVO]
}

// TAUpdateScript 更新脚本
func (sc *ScriptController) TAUpdateScript(ctx context.Context, input *TAUpdateScriptInput) (*TAUpdateScriptOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的脚本ID")
	}

	script := sc.scriptService.UpdateScript(input.ID, input.Body.Name, input.Body.Content)
	if script == nil {
		return nil, utils.HumaNotFound("脚本不存在")
	}

	return &TAUpdateScriptOutput{
		Body: utils.HumaResponse[*vo.ScriptVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToScriptVO(script),
		},
	}, nil
}

// TADeleteScriptInput 删除脚本
type TADeleteScriptInput struct {
	ID string `path:"id" description:"脚本ID"`
}

// TADeleteScriptOutput 删除脚本
type TADeleteScriptOutput struct {
	Body utils.HumaResponse[any]
}

// TADeleteScript 删除脚本
func (sc *ScriptController) TADeleteScript(ctx context.Context, input *TADeleteScriptInput) (*TADeleteScriptOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的脚本ID")
	}

	success := sc.scriptService.DeleteScript(input.ID)
	if !success {
		return nil, utils.HumaNotFound("脚本不存在")
	}

	return &TADeleteScriptOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "删除成功",
		},
	}, nil
}

// RegisterAPIScriptRoutes 注册 /api/v1 脚本管理 Huma 路由
func (sc *ScriptController) RegisterAPIScriptRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/scripts",
		OperationID: "apiCreateScript",
		Summary:     "创建脚本",
		Description: "创建一个新的脚本",
		Tags:        []string{"脚本管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, sc.TACreateScript)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/scripts",
		OperationID: "apiGetScripts",
		Summary:     "获取脚本列表",
		Description: "获取当前用户的脚本列表（不包含内容）",
		Tags:        []string{"脚本管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, sc.TAGetScripts)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/scripts/{id}",
		OperationID: "apiGetScript",
		Summary:     "获取脚本详情",
		Description: "根据 ID 获取脚本详情",
		Tags:        []string{"脚本管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, sc.TAGetScript)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPut,
		Path:        "/scripts/{id}",
		OperationID: "apiUpdateScript",
		Summary:     "更新脚本",
		Description: "根据 ID 更新脚本",
		Tags:        []string{"脚本管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, sc.TAUpdateScript)

	huma.Register(api, huma.Operation{
		Method:      http.MethodDelete,
		Path:        "/scripts/{id}",
		OperationID: "apiDeleteScript",
		Summary:     "删除脚本",
		Description: "根据 ID 删除脚本",
		Tags:        []string{"脚本管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, sc.TADeleteScript)
}
