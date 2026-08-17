package controllers

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/utils"
	"github.com/gin-gonic/gin"

	"github.com/danielgtaylor/huma/v2"
)

type WebUIController struct {
	webuiService *services.WebUIService
}

func NewWebUIController(webuiService *services.WebUIService) *WebUIController {
	return &WebUIController{
		webuiService: webuiService,
	}
}

// ===========================================================================
// Gin 原生 handler（由 api_routes.go 保留引用）
// ===========================================================================

// UploadWebUI 上传新WebUI
func (c *WebUIController) UploadWebUI(ctx *gin.Context) {
	file, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.Response{Code: 400, Msg: "获取上传文件失败"})
		return
	}

	// 临时保存上传的文件到挂载目录，避免 /tmp 跨分区移动或权限问题
	tmpDir := filepath.Join(constant.DataDir, "tmp")
	os.MkdirAll(tmpDir, 0755)
	tmpFile := filepath.Join(tmpDir, file.Filename)

	if err := ctx.SaveUploadedFile(file, tmpFile); err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.Response{Code: 500, Msg: "保存临时文件失败"})
		return
	}
	defer os.Remove(tmpFile) // 自动清理临时文件

	webuiName, err := c.webuiService.ExtractWebUI(tmpFile)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.Response{Code: 400, Msg: err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, utils.Response{Code: 200, Msg: "success", Data: gin.H{"message": "WebUI上传成功", "webui": webuiName}})
}

// ===========================================================================
// WebUI 管理业务方法（Huma）
// ===========================================================================

// GetWebUIsOutput 获取所有 WebUI
type GetWebUIsOutput struct {
	Body utils.HumaResponse[[]services.WebUIManifest]
}

// GetWebUIs 获取所有 WebUI
func (c *WebUIController) GetWebUIs(ctx context.Context, input *struct{}) (*GetWebUIsOutput, error) {
	webuis, err := c.webuiService.GetWebUIs()
	if err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &GetWebUIsOutput{
		Body: utils.HumaResponse[[]services.WebUIManifest]{
			Code: 200,
			Msg:  "success",
			Data: webuis,
		},
	}, nil
}

// SetActiveWebUIInput 切换活动 WebUI
type SetActiveWebUIInput struct {
	Body struct {
		Name string `json:"name" description:"WebUI 名称"`
	}
}

// SetActiveWebUIOutput 切换活动 WebUI
type SetActiveWebUIOutput struct {
	Body utils.HumaResponse[any]
}

// SetActiveWebUI 切换活动 WebUI
func (c *WebUIController) SetActiveWebUI(ctx context.Context, input *SetActiveWebUIInput) (*SetActiveWebUIOutput, error) {
	if input.Body.Name == "" {
		return nil, utils.HumaBadRequest("无效的请求参数")
	}

	if err := c.webuiService.SetActiveWebUI(input.Body.Name); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &SetActiveWebUIOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "WebUI已切换成功，部分页面可能需要刷新",
		},
	}, nil
}

// DeleteWebUIInput 删除自定义 WebUI
type DeleteWebUIInput struct {
	Name string `path:"name" description:"WebUI 名称"`
}

// DeleteWebUIOutput 删除自定义 WebUI
type DeleteWebUIOutput struct {
	Body utils.HumaResponse[any]
}

// DeleteWebUI 删除自定义 WebUI
func (c *WebUIController) DeleteWebUI(ctx context.Context, input *DeleteWebUIInput) (*DeleteWebUIOutput, error) {
	if input.Name == "" {
		return nil, utils.HumaBadRequest("未提供WebUI名称")
	}

	if err := c.webuiService.DeleteWebUI(input.Name); err != nil {
		return nil, utils.HumaBadRequest(err.Error())
	}

	return &DeleteWebUIOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "WebUI已删除",
		},
	}, nil
}

// UploadWebUIHumaInput 上传 WebUI
type UploadWebUIHumaInput struct {
	RawBody huma.MultipartFormFiles[struct {
		File huma.FormFile `form:"file" required:"true" description:"WebUI 压缩包"`
	}]
}

// UploadWebUIHumaOutput 上传 WebUI 结果
type UploadWebUIHumaOutput struct {
	Body utils.HumaResponse[struct {
		WebUI string `json:"webui"`
	}]
}

// UploadWebUIHuma 上传自定义 WebUI
func (c *WebUIController) UploadWebUIHuma(ctx context.Context, input *UploadWebUIHumaInput) (*UploadWebUIHumaOutput, error) {
	data := input.RawBody.Data()
	if !data.File.IsSet {
		return nil, utils.HumaBadRequest("获取上传文件失败")
	}

	// 临时保存上传的文件到挂载目录，避免 /tmp 跨分区移动或权限问题
	tmpDir := filepath.Join(constant.DataDir, "tmp")
	os.MkdirAll(tmpDir, 0755)
	tmpFile := filepath.Join(tmpDir, filepath.Base(data.File.Filename))

	dst, err := os.Create(tmpFile)
	if err != nil {
		return nil, utils.HumaServerError("保存临时文件失败")
	}
	if _, err := io.Copy(dst, data.File); err != nil {
		dst.Close()
		os.Remove(tmpFile)
		return nil, utils.HumaServerError("保存临时文件失败")
	}
	dst.Close()
	defer os.Remove(tmpFile) // 自动清理临时文件

	webuiName, err := c.webuiService.ExtractWebUI(tmpFile)
	if err != nil {
		return nil, utils.HumaBadRequest(err.Error())
	}

	return &UploadWebUIHumaOutput{
		Body: utils.HumaResponse[struct {
			WebUI string `json:"webui"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				WebUI string `json:"webui"`
			}{
				WebUI: webuiName,
			},
		},
	}, nil
}

// ===========================================================================
// 路由注册
// ===========================================================================

// RegisterAPIWebUIRoutes 注册 /api/v1 WebUI 管理 Huma 路由
func (c *WebUIController) RegisterAPIWebUIRoutes(api huma.API) {
	security := []map[string][]string{{"CookieAuth": {}}}
	tag := []string{"WebUI 管理"}

	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/webui", OperationID: "GetWebUIs", Summary: "获取所有 WebUI", Description: "获取所有可用的 WebUI 列表", Tags: tag, Security: security}, c.GetWebUIs)
	huma.Register(api, huma.Operation{Method: http.MethodPut, Path: "/webui/active", OperationID: "SetActiveWebUI", Summary: "切换活动 WebUI", Description: "切换当前活动的 WebUI", Tags: tag, Security: security}, c.SetActiveWebUI)
	huma.Register(api, huma.Operation{Method: http.MethodDelete, Path: "/webui/{name}", OperationID: "DeleteWebUI", Summary: "删除自定义 WebUI", Description: "删除指定的自定义 WebUI", Tags: tag, Security: security}, c.DeleteWebUI)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/webui/upload", OperationID: "UploadWebUI", Summary: "上传自定义 WebUI", Description: "以 multipart/form-data 上传 WebUI 压缩包并解压部署。", Tags: tag, Security: security}, c.UploadWebUIHuma)
}
