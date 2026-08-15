package controllers

import (
	"context"
	"net/http"

	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

// ===========================================================================
// /api/v1 管理接口 —— WebUI 管理
// ===========================================================================

// TAWebUIGetWebUIsOutput 获取所有 WebUI
type TAWebUIGetWebUIsOutput struct {
	Body utils.HumaResponse[[]services.WebUIManifest]
}

// TAWebUIGetWebUIs 获取所有 WebUI
func (c *WebUIController) TAWebUIGetWebUIs(ctx context.Context, input *struct{}) (*TAWebUIGetWebUIsOutput, error) {
	webuis, err := c.webuiService.GetWebUIs()
	if err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &TAWebUIGetWebUIsOutput{
		Body: utils.HumaResponse[[]services.WebUIManifest]{
			Code: 200,
			Msg:  "success",
			Data: webuis,
		},
	}, nil
}

// TAWebUISetActiveInput 切换活动 WebUI
type TAWebUISetActiveInput struct {
	Body struct {
		Name string `json:"name" description:"WebUI 名称"`
	}
}

// TAWebUISetActiveOutput 切换活动 WebUI
type TAWebUISetActiveOutput struct {
	Body utils.HumaResponse[any]
}

// TAWebUISetActive 切换活动 WebUI
func (c *WebUIController) TAWebUISetActive(ctx context.Context, input *TAWebUISetActiveInput) (*TAWebUISetActiveOutput, error) {
	if input.Body.Name == "" {
		return nil, utils.HumaBadRequest("无效的请求参数")
	}

	if err := c.webuiService.SetActiveWebUI(input.Body.Name); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &TAWebUISetActiveOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "WebUI已切换成功，部分页面可能需要刷新",
		},
	}, nil
}

// TAWebUIDeleteInput 删除自定义 WebUI
type TAWebUIDeleteInput struct {
	Name string `path:"name" description:"WebUI 名称"`
}

// TAWebUIDeleteOutput 删除自定义 WebUI
type TAWebUIDeleteOutput struct {
	Body utils.HumaResponse[any]
}

// TAWebUIDelete 删除自定义 WebUI
func (c *WebUIController) TAWebUIDelete(ctx context.Context, input *TAWebUIDeleteInput) (*TAWebUIDeleteOutput, error) {
	if input.Name == "" {
		return nil, utils.HumaBadRequest("未提供WebUI名称")
	}

	if err := c.webuiService.DeleteWebUI(input.Name); err != nil {
		return nil, utils.HumaBadRequest(err.Error())
	}

	return &TAWebUIDeleteOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "WebUI已删除",
		},
	}, nil
}

// RegisterAPIWebUIRoutes 注册 /api/v1 WebUI 管理 Huma 路由
func (c *WebUIController) RegisterAPIWebUIRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/webui",
		OperationID: "apiGetWebUIs",
		Summary:     "获取所有 WebUI",
		Description: "获取所有可用的 WebUI 列表",
		Tags:        []string{"WebUI 管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAWebUIGetWebUIs)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPut,
		Path:        "/webui/active",
		OperationID: "apiSetActiveWebUI",
		Summary:     "切换活动 WebUI",
		Description: "切换当前活动的 WebUI",
		Tags:        []string{"WebUI 管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAWebUISetActive)

	huma.Register(api, huma.Operation{
		Method:      http.MethodDelete,
		Path:        "/webui/{name}",
		OperationID: "apiDeleteWebUI",
		Summary:     "删除自定义 WebUI",
		Description: "删除指定的自定义 WebUI",
		Tags:        []string{"WebUI 管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAWebUIDelete)
}

// RegisterAPIWebUIUploadRoutes 占位：上传接口由 Gin 原生处理（multipart 文件上传）
