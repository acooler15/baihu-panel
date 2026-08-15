package controllers

import (
	"context"
	"net/http"

	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

// ===========================================================================
// /api/v1 管理接口 —— Mise 环境管理
// ===========================================================================

// TAMiseListOutput 获取语言列表
type TAMiseListOutput struct {
	Body utils.HumaResponse[[]services.MiseLanguage]
}

// TAMiseList 获取语言列表
func (c *MiseController) TAMiseList(ctx context.Context, input *struct{}) (*TAMiseListOutput, error) {
	langs, err := c.service.List()
	if err != nil {
		return nil, utils.HumaServerError("获取语言列表失败: " + err.Error())
	}

	return &TAMiseListOutput{
		Body: utils.HumaResponse[[]services.MiseLanguage]{
			Code: 200,
			Msg:  "success",
			Data: langs,
		},
	}, nil
}

// TAMiseSyncOutput 同步本地环境
type TAMiseSyncOutput struct {
	Body utils.HumaResponse[any]
}

// TAMiseSync 同步本地环境到数据库
func (c *MiseController) TAMiseSync(ctx context.Context, input *struct{}) (*TAMiseSyncOutput, error) {
	if err := c.service.Sync(); err != nil {
		return nil, utils.HumaServerError("同步本地环境失败: " + err.Error())
	}

	return &TAMiseSyncOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "success",
		},
	}, nil
}

// TAMisePluginsOutput 获取可用插件列表
type TAMisePluginsOutput struct {
	Body utils.HumaResponse[[]string]
}

// TAMisePlugins 获取可用插件列表
func (c *MiseController) TAMisePlugins(ctx context.Context, input *struct{}) (*TAMisePluginsOutput, error) {
	plugins, err := c.service.Plugins()
	if err != nil {
		return nil, utils.HumaServerError("获取插件列表失败: " + err.Error())
	}

	return &TAMisePluginsOutput{
		Body: utils.HumaResponse[[]string]{
			Code: 200,
			Msg:  "success",
			Data: plugins,
		},
	}, nil
}

// TAMiseVersionsInput 获取指定插件的可用版本列表
type TAMiseVersionsInput struct {
	Plugin string `query:"plugin" description:"插件名"`
}

// TAMiseVersionsOutput 获取指定插件的可用版本列表
type TAMiseVersionsOutput struct {
	Body utils.HumaResponse[[]string]
}

// TAMiseVersions 获取指定插件的可用版本列表
func (c *MiseController) TAMiseVersions(ctx context.Context, input *TAMiseVersionsInput) (*TAMiseVersionsOutput, error) {
	if input.Plugin == "" {
		return nil, utils.HumaBadRequest("参数 plugin 不能为空")
	}

	versions, err := c.service.Versions(input.Plugin)
	if err != nil {
		return nil, utils.HumaServerError("获取版本列表失败: " + err.Error())
	}

	return &TAMiseVersionsOutput{
		Body: utils.HumaResponse[[]string]{
			Code: 200,
			Msg:  "success",
			Data: versions,
		},
	}, nil
}

// TAMiseVerifyCommandInput 获取验证命令
type TAMiseVerifyCommandInput struct {
	Plugin  string `query:"plugin" description:"插件名"`
	Version string `query:"version" description:"版本"`
}

// TAMiseVerifyCommandOutput 获取验证命令
type TAMiseVerifyCommandOutput struct {
	Body utils.HumaResponse[struct {
		Command string `json:"command"`
	}]
}

// TAMiseVerifyCommand 获取验证命令
func (c *MiseController) TAMiseVerifyCommand(ctx context.Context, input *TAMiseVerifyCommandInput) (*TAMiseVerifyCommandOutput, error) {
	if input.Plugin == "" {
		return nil, utils.HumaBadRequest("参数 plugin 不能为空")
	}

	cmd, err := c.service.GetVerifyCommand(input.Plugin, input.Version)
	if err != nil {
		return nil, utils.HumaServerError("获取验证命令失败: " + err.Error())
	}

	return &TAMiseVerifyCommandOutput{
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

// TAMiseUseGlobalInput 设置全局默认版本
type TAMiseUseGlobalInput struct {
	Body struct {
		Plugin  string `json:"plugin" description:"插件名"`
		Version string `json:"version" description:"版本"`
	}
}

// TAMiseUseGlobalOutput 设置全局默认版本
type TAMiseUseGlobalOutput struct {
	Body utils.HumaResponse[any]
}

// TAMiseUseGlobal 设置全局默认版本
func (c *MiseController) TAMiseUseGlobal(ctx context.Context, input *TAMiseUseGlobalInput) (*TAMiseUseGlobalOutput, error) {
	req := input.Body
	if req.Plugin == "" || req.Version == "" {
		return nil, utils.HumaBadRequest("参数 plugin 和 version 不能为空")
	}

	if err := c.service.UseGlobal(req.Plugin, req.Version); err != nil {
		return nil, utils.HumaServerError("设置全局版本失败: " + err.Error())
	}

	return &TAMiseUseGlobalOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "success",
		},
	}, nil
}

// TAMiseUnsetGlobalInput 取消全局默认版本
type TAMiseUnsetGlobalInput struct {
	Body struct {
		Plugin  string `json:"plugin" description:"插件名"`
		Version string `json:"version" description:"版本"`
	}
}

// TAMiseUnsetGlobalOutput 取消全局默认版本
type TAMiseUnsetGlobalOutput struct {
	Body utils.HumaResponse[any]
}

// TAMiseUnsetGlobal 取消全局默认版本
func (c *MiseController) TAMiseUnsetGlobal(ctx context.Context, input *TAMiseUnsetGlobalInput) (*TAMiseUnsetGlobalOutput, error) {
	req := input.Body
	if req.Plugin == "" {
		return nil, utils.HumaBadRequest("参数 plugin 不能为空")
	}

	if err := c.service.UnsetGlobal(req.Plugin, req.Version); err != nil {
		return nil, utils.HumaServerError("取消全局版本失败: " + err.Error())
	}

	return &TAMiseUnsetGlobalOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "success",
		},
	}, nil
}

// TAMiseEnvsOutput 获取全局环境变量
type TAMiseEnvsOutput struct {
	Body utils.HumaResponse[map[string]string]
}

// TAMiseEnvs 获取全局环境变量
func (c *MiseController) TAMiseEnvs(ctx context.Context, input *struct{}) (*TAMiseEnvsOutput, error) {
	envs, err := c.service.Envs()
	if err != nil {
		return nil, utils.HumaServerError("获取全局环境变量失败: " + err.Error())
	}

	return &TAMiseEnvsOutput{
		Body: utils.HumaResponse[map[string]string]{
			Code: 200,
			Msg:  "success",
			Data: envs,
		},
	}, nil
}

// TAMiseSetEnvInput 设置全局环境变量
type TAMiseSetEnvInput struct {
	Body struct {
		Key   string `json:"key" description:"环境变量名"`
		Value string `json:"value" description:"环境变量值"`
	}
}

// TAMiseSetEnvOutput 设置全局环境变量
type TAMiseSetEnvOutput struct {
	Body utils.HumaResponse[any]
}

// TAMiseSetEnv 设置全局环境变量
func (c *MiseController) TAMiseSetEnv(ctx context.Context, input *TAMiseSetEnvInput) (*TAMiseSetEnvOutput, error) {
	req := input.Body
	if req.Key == "" {
		return nil, utils.HumaBadRequest("参数 key 不能为空")
	}

	if err := c.service.SetEnv(req.Key, req.Value); err != nil {
		return nil, utils.HumaServerError("设置环境变量失败: " + err.Error())
	}

	return &TAMiseSetEnvOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "success",
		},
	}, nil
}

// TAMiseUnsetEnvInput 取消全局环境变量
type TAMiseUnsetEnvInput struct {
	Key string `query:"key" description:"环境变量名"`
}

// TAMiseUnsetEnvOutput 取消全局环境变量
type TAMiseUnsetEnvOutput struct {
	Body utils.HumaResponse[any]
}

// TAMiseUnsetEnv 取消全局环境变量
func (c *MiseController) TAMiseUnsetEnv(ctx context.Context, input *TAMiseUnsetEnvInput) (*TAMiseUnsetEnvOutput, error) {
	if input.Key == "" {
		return nil, utils.HumaBadRequest("参数 key 不能为空")
	}

	if err := c.service.UnsetEnv(input.Key); err != nil {
		return nil, utils.HumaServerError("取消环境变量失败: " + err.Error())
	}

	return &TAMiseUnsetEnvOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "success",
		},
	}, nil
}

// RegisterAPIMiseRoutes 注册 /api/v1 Mise 环境管理 Huma 路由
func (c *MiseController) RegisterAPIMiseRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/mise/ls",
		OperationID: "apiMiseList",
		Summary:     "获取语言列表",
		Description: "获取已安装的语言环境列表",
		Tags:        []string{"Mise 环境"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAMiseList)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/mise/sync",
		OperationID: "apiMiseSync",
		Summary:     "同步本地环境",
		Description: "同步本地环境到数据库",
		Tags:        []string{"Mise 环境"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAMiseSync)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/mise/plugins",
		OperationID: "apiMisePlugins",
		Summary:     "获取可用插件列表",
		Description: "获取可用的插件列表",
		Tags:        []string{"Mise 环境"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAMisePlugins)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/mise/versions",
		OperationID: "apiMiseVersions",
		Summary:     "获取版本列表",
		Description: "获取指定插件的可用版本列表",
		Tags:        []string{"Mise 环境"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAMiseVersions)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/mise/verify-cmd",
		OperationID: "apiMiseVerifyCommand",
		Summary:     "获取验证命令",
		Description: "获取指定插件版本的验证命令",
		Tags:        []string{"Mise 环境"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAMiseVerifyCommand)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/mise/use-global",
		OperationID: "apiMiseUseGlobal",
		Summary:     "设置全局默认版本",
		Description: "设置指定插件的全局默认版本",
		Tags:        []string{"Mise 环境"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAMiseUseGlobal)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/mise/unset-global",
		OperationID: "apiMiseUnsetGlobal",
		Summary:     "取消全局默认版本",
		Description: "取消指定插件的全局默认版本",
		Tags:        []string{"Mise 环境"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAMiseUnsetGlobal)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/mise/envs",
		OperationID: "apiMiseEnvs",
		Summary:     "获取全局环境变量",
		Description: "获取 mise 全局环境变量",
		Tags:        []string{"Mise 环境"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAMiseEnvs)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/mise/envs",
		OperationID: "apiMiseSetEnv",
		Summary:     "设置全局环境变量",
		Description: "设置 mise 全局环境变量",
		Tags:        []string{"Mise 环境"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAMiseSetEnv)

	huma.Register(api, huma.Operation{
		Method:      http.MethodDelete,
		Path:        "/mise/envs",
		OperationID: "apiMiseUnsetEnv",
		Summary:     "取消全局环境变量",
		Description: "取消 mise 全局环境变量",
		Tags:        []string{"Mise 环境"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAMiseUnsetEnv)
}
