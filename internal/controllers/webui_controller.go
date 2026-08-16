package controllers

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/utils"
	"github.com/gin-gonic/gin"
)

type WebUIController struct {
	webuiService *services.WebUIService
}

func NewWebUIController(webuiService *services.WebUIService) *WebUIController {
	return &WebUIController{
		webuiService: webuiService,
	}
}

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
