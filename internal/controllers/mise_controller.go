package controllers

import (
	"context"
	"net/http"

	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

type MiseController struct {
	service *services.MiseService
}

func NewMiseController(service *services.MiseService) *MiseController {
	return &MiseController{
		service: service,
	}
}

// ===========================================================================
// Mise 环境管理业务方法
// ===========================================================================

// MiseListOutput 获取语言列表
type MiseListOutput struct {
	Body utils.HumaResponse[[]services.MiseLanguage]
}

// MiseList 获取语言列表
func (c *MiseController) MiseList(ctx context.Context, input *struct{}) (*MiseListOutput, error) {
	langs, err := c.service.List()
	if err != nil {
		return nil, utils.HumaServerError("获取语言列表失败: " + err.Error())
	}

	return &MiseListOutput{
		Body: utils.HumaResponse[[]services.MiseLanguage]{
			Code: 200,
			Msg:  "success",
			Data: langs,
		},
	}, nil
}

// MiseSyncOutput 同步本地环境
type MiseSyncOutput struct {
	Body utils.HumaResponse[any]
}

// MiseSync 同步本地环境到数据库
func (c *MiseController) MiseSync(ctx context.Context, input *struct{}) (*MiseSyncOutput, error) {
	if err := c.service.Sync(); err != nil {
		return nil, utils.HumaServerError("同步本地环境失败: " + err.Error())
	}

	return &MiseSyncOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "success",
		},
	}, nil
}

// MisePluginsOutput 获取可用插件列表
type MisePluginsOutput struct {
	Body utils.HumaResponse[[]string]
}

// MisePlugins 获取可用插件列表
func (c *MiseController) MisePlugins(ctx context.Context, input *struct{}) (*MisePluginsOutput, error) {
	plugins, err := c.service.Plugins()
	if err != nil {
		return nil, utils.HumaServerError("获取插件列表失败: " + err.Error())
	}

	return &MisePluginsOutput{
		Body: utils.HumaResponse[[]string]{
			Code: 200,
			Msg:  "success",
			Data: plugins,
		},
	}, nil
}

// MiseVersionsInput 获取指定插件的可用版本列表
type MiseVersionsInput struct {
	Plugin string `query:"plugin" description:"插件名"`
}

// MiseVersionsOutput 获取指定插件的可用版本列表
type MiseVersionsOutput struct {
	Body utils.HumaResponse[[]string]
}

// MiseVersions 获取指定插件的可用版本列表
func (c *MiseController) MiseVersions(ctx context.Context, input *MiseVersionsInput) (*MiseVersionsOutput, error) {
	if input.Plugin == "" {
		return nil, utils.HumaBadRequest("参数 plugin 不能为空")
	}

	versions, err := c.service.Versions(input.Plugin)
	if err != nil {
		return nil, utils.HumaServerError("获取版本列表失败: " + err.Error())
	}

	return &MiseVersionsOutput{
		Body: utils.HumaResponse[[]string]{
			Code: 200,
			Msg:  "success",
			Data: versions,
		},
	}, nil
}

// MiseVerifyCommandInput 获取验证命令
type MiseVerifyCommandInput struct {
	Plugin  string `query:"plugin" description:"插件名"`
	Version string `query:"version" description:"版本"`
}

// MiseVerifyCommandOutput 获取验证命令
type MiseVerifyCommandOutput struct {
	Body utils.HumaResponse[struct {
		Command string `json:"command"`
	}]
}

// MiseVerifyCommand 获取验证命令
func (c *MiseController) MiseVerifyCommand(ctx context.Context, input *MiseVerifyCommandInput) (*MiseVerifyCommandOutput, error) {
	if input.Plugin == "" {
		return nil, utils.HumaBadRequest("参数 plugin 不能为空")
	}

	cmd, err := c.service.GetVerifyCommand(input.Plugin, input.Version)
	if err != nil {
		return nil, utils.HumaServerError("获取验证命令失败: " + err.Error())
	}

	return &MiseVerifyCommandOutput{
		Body: utils.HumaResponse[struct {
			Command string `json:"command"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				Command string `json:"command"`
			}{Command: cmd},
		},
	}, nil
}

// MiseUseGlobalInput 设置全局默认版本
type MiseUseGlobalInput struct {
	Body struct {
		Plugin  string `json:"plugin" description:"插件名"`
		Version string `json:"version" description:"版本"`
	}
}

// MiseUseGlobalOutput 设置全局默认版本
type MiseUseGlobalOutput struct {
	Body utils.HumaResponse[any]
}

// MiseUseGlobal 设置全局默认版本
func (c *MiseController) MiseUseGlobal(ctx context.Context, input *MiseUseGlobalInput) (*MiseUseGlobalOutput, error) {
	req := input.Body
	if req.Plugin == "" || req.Version == "" {
		return nil, utils.HumaBadRequest("参数 plugin 和 version 不能为空")
	}

	if err := c.service.UseGlobal(req.Plugin, req.Version); err != nil {
		return nil, utils.HumaServerError("设置全局版本失败: " + err.Error())
	}

	return &MiseUseGlobalOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "success",
		},
	}, nil
}

// MiseUnsetGlobalInput 取消全局默认版本
type MiseUnsetGlobalInput struct {
	Body struct {
		Plugin  string `json:"plugin" description:"插件名"`
		Version string `json:"version" description:"版本"`
	}
}

// MiseUnsetGlobalOutput 取消全局默认版本
type MiseUnsetGlobalOutput struct {
	Body utils.HumaResponse[any]
}

// MiseUnsetGlobal 取消全局默认版本
func (c *MiseController) MiseUnsetGlobal(ctx context.Context, input *MiseUnsetGlobalInput) (*MiseUnsetGlobalOutput, error) {
	req := input.Body
	if req.Plugin == "" {
		return nil, utils.HumaBadRequest("参数 plugin 不能为空")
	}

	if err := c.service.UnsetGlobal(req.Plugin, req.Version); err != nil {
		return nil, utils.HumaServerError("取消全局版本失败: " + err.Error())
	}

	return &MiseUnsetGlobalOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "success",
		},
	}, nil
}

// MiseEnvsOutput 获取全局环境变量
type MiseEnvsOutput struct {
	Body utils.HumaResponse[map[string]string]
}

// MiseEnvs 获取全局环境变量
func (c *MiseController) MiseEnvs(ctx context.Context, input *struct{}) (*MiseEnvsOutput, error) {
	envs, err := c.service.Envs()
	if err != nil {
		return nil, utils.HumaServerError("获取全局环境变量失败: " + err.Error())
	}

	return &MiseEnvsOutput{
		Body: utils.HumaResponse[map[string]string]{
			Code: 200,
			Msg:  "success",
			Data: envs,
		},
	}, nil
}

// MiseSetEnvInput 设置全局环境变量
type MiseSetEnvInput struct {
	Body struct {
		Key   string `json:"key" description:"环境变量名"`
		Value string `json:"value" description:"环境变量值"`
	}
}

// MiseSetEnvOutput 设置全局环境变量
type MiseSetEnvOutput struct {
	Body utils.HumaResponse[any]
}

// MiseSetEnv 设置全局环境变量
func (c *MiseController) MiseSetEnv(ctx context.Context, input *MiseSetEnvInput) (*MiseSetEnvOutput, error) {
	req := input.Body
	if req.Key == "" {
		return nil, utils.HumaBadRequest("参数 key 不能为空")
	}

	if err := c.service.SetEnv(req.Key, req.Value); err != nil {
		return nil, utils.HumaServerError("设置环境变量失败: " + err.Error())
	}

	return &MiseSetEnvOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "success",
		},
	}, nil
}

// MiseUnsetEnvInput 取消全局环境变量
type MiseUnsetEnvInput struct {
	Key string `query:"key" description:"环境变量名"`
}

// MiseUnsetEnvOutput 取消全局环境变量
type MiseUnsetEnvOutput struct {
	Body utils.HumaResponse[any]
}

// MiseUnsetEnv 取消全局环境变量
func (c *MiseController) MiseUnsetEnv(ctx context.Context, input *MiseUnsetEnvInput) (*MiseUnsetEnvOutput, error) {
	if input.Key == "" {
		return nil, utils.HumaBadRequest("参数 key 不能为空")
	}

	if err := c.service.UnsetEnv(input.Key); err != nil {
		return nil, utils.HumaServerError("取消环境变量失败: " + err.Error())
	}

	return &MiseUnsetEnvOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "success",
		},
	}, nil
}

// ===========================================================================
// 路由注册
// ===========================================================================

// RegisterAPIMiseRoutes 注册 /api/v1 Mise 环境管理 Huma 路由
func (c *MiseController) RegisterAPIMiseRoutes(api huma.API) {
	security := []map[string][]string{{"CookieAuth": {}}}
	tag := []string{"Mise 环境"}

	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/mise/ls", OperationID: "MiseList", Summary: "获取语言列表", Description: "获取已安装的语言环境列表", Tags: tag, Security: security}, c.MiseList)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/mise/sync", OperationID: "MiseSync", Summary: "同步本地环境", Description: "同步本地环境到数据库", Tags: tag, Security: security}, c.MiseSync)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/mise/plugins", OperationID: "MisePlugins", Summary: "获取可用插件列表", Description: "获取可用的插件列表", Tags: tag, Security: security}, c.MisePlugins)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/mise/versions", OperationID: "MiseVersions", Summary: "获取版本列表", Description: "获取指定插件的可用版本列表", Tags: tag, Security: security}, c.MiseVersions)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/mise/verify-cmd", OperationID: "MiseVerifyCommand", Summary: "获取验证命令", Description: "获取指定插件版本的验证命令", Tags: tag, Security: security}, c.MiseVerifyCommand)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/mise/use-global", OperationID: "MiseUseGlobal", Summary: "设置全局默认版本", Description: "设置指定插件的全局默认版本", Tags: tag, Security: security}, c.MiseUseGlobal)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/mise/unset-global", OperationID: "MiseUnsetGlobal", Summary: "取消全局默认版本", Description: "取消指定插件的全局默认版本", Tags: tag, Security: security}, c.MiseUnsetGlobal)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/mise/envs", OperationID: "MiseEnvs", Summary: "获取全局环境变量", Description: "获取 mise 全局环境变量", Tags: tag, Security: security}, c.MiseEnvs)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/mise/envs", OperationID: "MiseSetEnv", Summary: "设置全局环境变量", Description: "设置 mise 全局环境变量", Tags: tag, Security: security}, c.MiseSetEnv)
	huma.Register(api, huma.Operation{Method: http.MethodDelete, Path: "/mise/envs", OperationID: "MiseUnsetEnv", Summary: "取消全局环境变量", Description: "取消 mise 全局环境变量", Tags: tag, Security: security}, c.MiseUnsetEnv)
}
