package controllers

import (
	"net/http"

	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/utils"
	"github.com/gin-gonic/gin"
)

type NotificationController struct {
	notifyService *services.NotificationService
}

func NewNotificationController() *NotificationController {
	return &NotificationController{
		notifyService: services.NewNotificationService(),
	}
}

// SendNotification API 发送通知（供脚本调用）
func (nc *NotificationController) SendNotification(c *gin.Context) {
	var req struct {
		ChannelID string `json:"channel_id"`
		Title     string `json:"title"`
		Text      string `json:"text"`
		Content   string `json:"content"`
		Format    string `json:"format"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.Response{Code: 400, Msg: "参数错误"})
		return
	}

	if req.ChannelID == "" || req.Title == "" {
		c.JSON(http.StatusBadRequest, utils.Response{Code: 400, Msg: "channel_id 和 title 不能为空"})
		return
	}

	// 兼容旧版 text 字段：优先使用 content，为空则回退到 text
	body := req.Content
	if body == "" {
		body = req.Text
	}

	result := nc.notifyService.SendByChannelID(req.ChannelID, &services.NotifyMessage{
		Title:   req.Title,
		Content: body,
		Format:  req.Format,
	})

	c.JSON(http.StatusOK, utils.Response{Code: 200, Msg: "success", Data: result})
}
