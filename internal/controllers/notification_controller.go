package controllers

import (
	"context"
	"net/http"

	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/utils"
	"github.com/gin-gonic/gin"

	"github.com/danielgtaylor/huma/v2"
)

type NotificationController struct {
	notifyService *services.NotificationService
}

func NewNotificationController() *NotificationController {
	return &NotificationController{
		notifyService: services.NewNotificationService(),
	}
}

// ===========================================================================
// Gin 原生 handler（由 api_routes.go 保留引用）
// ===========================================================================

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

// ===========================================================================
// 通知管理业务方法（Huma）
// ===========================================================================

// GetChannelTypesOutput 获取支持的渠道类型
type GetChannelTypesOutput struct {
	Body utils.HumaResponse[struct {
		ChannelTypes []map[string]string `json:"channel_types"`
		EventTypes   []map[string]string `json:"event_types"`
	}]
}

// GetChannelTypes 获取支持的渠道类型
func (nc *NotificationController) GetChannelTypes(ctx context.Context, input *struct{}) (*GetChannelTypesOutput, error) {
	return &GetChannelTypesOutput{
		Body: utils.HumaResponse[struct {
			ChannelTypes []map[string]string `json:"channel_types"`
			EventTypes   []map[string]string `json:"event_types"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				ChannelTypes []map[string]string `json:"channel_types"`
				EventTypes   []map[string]string `json:"event_types"`
			}{
				ChannelTypes: services.SupportedChannelTypes,
				EventTypes:   services.SupportedEvents,
			},
		},
	}, nil
}

// GetChannelsOutput 获取所有渠道
type GetChannelsOutput struct {
	Body utils.HumaResponse[[]services.NotifyChannel]
}

// GetChannels 获取所有渠道
func (nc *NotificationController) GetChannels(ctx context.Context, input *struct{}) (*GetChannelsOutput, error) {
	channels := nc.notifyService.GetChannels()
	return &GetChannelsOutput{
		Body: utils.HumaResponse[[]services.NotifyChannel]{
			Code: 200,
			Msg:  "success",
			Data: channels,
		},
	}, nil
}

// SaveChannelInput 保存/更新渠道
type SaveChannelInput struct {
	Body services.NotifyChannel
}

// SaveChannelOutput 保存/更新渠道
type SaveChannelOutput struct {
	Body utils.HumaResponse[any]
}

// SaveChannel 保存/更新渠道
func (nc *NotificationController) SaveChannel(ctx context.Context, input *SaveChannelInput) (*SaveChannelOutput, error) {
	req := input.Body
	if req.Name == "" || req.Type == "" {
		return nil, utils.HumaBadRequest("渠道名称和类型不能为空")
	}

	if err := nc.notifyService.SaveChannel(req); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &SaveChannelOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "保存成功",
		},
	}, nil
}

// DeleteChannelInput 删除渠道
type DeleteChannelInput struct {
	ID string `path:"id" description:"渠道ID"`
}

// DeleteChannelOutput 删除渠道
type DeleteChannelOutput struct {
	Body utils.HumaResponse[any]
}

// DeleteChannel 删除渠道
func (nc *NotificationController) DeleteChannel(ctx context.Context, input *DeleteChannelInput) (*DeleteChannelOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("缺少渠道ID")
	}

	if err := nc.notifyService.DeleteChannel(input.ID); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &DeleteChannelOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "删除成功",
		},
	}, nil
}

// TestChannelInput 测试渠道
type TestChannelInput struct {
	Body services.NotifyChannel
}

// TestChannelOutput 测试渠道
type TestChannelOutput struct {
	Body utils.HumaResponse[*services.NotifyResult]
}

// TestChannel 测试渠道
func (nc *NotificationController) TestChannel(ctx context.Context, input *TestChannelInput) (*TestChannelOutput, error) {
	result := nc.notifyService.SendToChannel(input.Body, &services.NotifyMessage{
		Title:   "🔔 白虎面板测试通知",
		Content: "如果你看到这条消息，说明通知渠道配置正确！",
	})

	return &TestChannelOutput{
		Body: utils.HumaResponse[*services.NotifyResult]{
			Code: 200,
			Msg:  "success",
			Data: result,
		},
	}, nil
}

// GetBindingsOutput 获取事件绑定列表
type GetBindingsOutput struct {
	Body utils.HumaResponse[[]models.NotifyBinding]
}

// GetBindings 获取事件绑定列表
func (nc *NotificationController) GetBindings(ctx context.Context, input *struct{}) (*GetBindingsOutput, error) {
	bindings := nc.notifyService.GetBindings()
	return &GetBindingsOutput{
		Body: utils.HumaResponse[[]models.NotifyBinding]{
			Code: 200,
			Msg:  "success",
			Data: bindings,
		},
	}, nil
}

// SaveBindingInput 保存事件绑定
type SaveBindingInput struct {
	Body struct {
		ID     string         `json:"id" description:"绑定ID，空则新建"`
		Type   string         `json:"type" description:"绑定类型"`
		Event  string         `json:"event" description:"事件类型"`
		WayID  string         `json:"way_id" description:"渠道ID"`
		DataID string         `json:"data_id" description:"关联ID"`
		Extra  models.BigText `json:"extra" description:"额外配置"`
	}
}

// SaveBindingOutput 保存事件绑定
type SaveBindingOutput struct {
	Body utils.HumaResponse[*models.NotifyBinding]
}

// SaveBinding 保存事件绑定
func (nc *NotificationController) SaveBinding(ctx context.Context, input *SaveBindingInput) (*SaveBindingOutput, error) {
	req := input.Body
	if req.Type == "" || req.Event == "" || req.WayID == "" {
		return nil, utils.HumaBadRequest("类型、事件和渠道ID不能为空")
	}

	binding := &models.NotifyBinding{
		ID:     req.ID,
		Type:   req.Type,
		Event:  req.Event,
		WayID:  req.WayID,
		DataID: req.DataID,
		Extra:  req.Extra,
	}

	if err := nc.notifyService.SaveBinding(binding); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &SaveBindingOutput{
		Body: utils.HumaResponse[*models.NotifyBinding]{
			Code: 200,
			Msg:  "success",
			Data: binding,
		},
	}, nil
}

// BatchSaveBindingsInput 批量保存事件绑定
type BatchSaveBindingsInput struct {
	Body struct {
		Type     string                 `json:"type" description:"绑定类型"`
		DataID   string                 `json:"data_id" description:"关联ID"`
		Bindings []models.NotifyBinding `json:"bindings" description:"绑定列表"`
	}
}

// BatchSaveBindingsOutput 批量保存事件绑定
type BatchSaveBindingsOutput struct {
	Body utils.HumaResponse[any]
}

// BatchSaveBindings 批量保存事件绑定
func (nc *NotificationController) BatchSaveBindings(ctx context.Context, input *BatchSaveBindingsInput) (*BatchSaveBindingsOutput, error) {
	req := input.Body
	if req.Type == "" {
		return nil, utils.HumaBadRequest("类型不能为空")
	}

	if err := nc.notifyService.BatchSaveBindings(req.Type, req.DataID, req.Bindings); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &BatchSaveBindingsOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "保存成功",
		},
	}, nil
}

// DeleteBindingInput 删除事件绑定
type DeleteBindingInput struct {
	ID string `path:"id" description:"绑定ID"`
}

// DeleteBindingOutput 删除事件绑定
type DeleteBindingOutput struct {
	Body utils.HumaResponse[any]
}

// DeleteBinding 删除事件绑定
func (nc *NotificationController) DeleteBinding(ctx context.Context, input *DeleteBindingInput) (*DeleteBindingOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("缺少绑定ID")
	}

	if err := nc.notifyService.DeleteBinding(input.ID); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &DeleteBindingOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "删除成功",
		},
	}, nil
}

// SendNotifyInput 发送通知
type SendNotifyInput struct {
	Body struct {
		ChannelID string `json:"channel_id" description:"渠道ID"`
		Title     string `json:"title" description:"标题"`
		Text      string `json:"text" description:"内容(兼容旧版，优先使用 content)"`
		Content   string `json:"content" description:"内容"`
	}
}

// SendNotifyOutput 发送通知
type SendNotifyOutput struct {
	Body utils.HumaResponse[*services.NotifyResult]
}

// SendNotify 发送通知
func (nc *NotificationController) SendNotify(ctx context.Context, input *SendNotifyInput) (*SendNotifyOutput, error) {
	req := input.Body
	if req.ChannelID == "" || req.Title == "" {
		return nil, utils.HumaBadRequest("channel_id 和 title 不能为空")
	}

	// 兼容旧版 text 字段：优先使用 content，为空则回退到 text
	body := req.Content
	if body == "" {
		body = req.Text
	}

	result := nc.notifyService.SendByChannelID(req.ChannelID, &services.NotifyMessage{
		Title:   req.Title,
		Content: body,
	})

	return &SendNotifyOutput{
		Body: utils.HumaResponse[*services.NotifyResult]{
			Code: 200,
			Msg:  "success",
			Data: result,
		},
	}, nil
}

// ===========================================================================
// 路由注册
// ===========================================================================

// RegisterAPINotificationRoutes 注册 /api/v1 通知管理 Huma 路由
func (nc *NotificationController) RegisterAPINotificationRoutes(api huma.API) {
	security := []map[string][]string{{"CookieAuth": {}}}
	notifySecurity := []map[string][]string{{"NotifyTokenAuth": {}}}
	tag := []string{"通知管理"}
	notifyTag := []string{"通知"}

	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/notify/types", OperationID: "GetChannelTypes", Summary: "获取支持的渠道类型", Description: "获取支持的通知渠道类型与事件类型", Tags: tag, Security: security}, nc.GetChannelTypes)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/notify/channels", OperationID: "GetChannels", Summary: "获取所有渠道", Description: "获取所有通知渠道", Tags: tag, Security: security}, nc.GetChannels)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/notify/channels", OperationID: "SaveChannel", Summary: "保存/更新渠道", Description: "保存或更新通知渠道", Tags: tag, Security: security}, nc.SaveChannel)
	huma.Register(api, huma.Operation{Method: http.MethodDelete, Path: "/notify/channels/{id}", OperationID: "DeleteChannel", Summary: "删除渠道", Description: "删除指定通知渠道", Tags: tag, Security: security}, nc.DeleteChannel)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/notify/channels/test", OperationID: "TestChannel", Summary: "测试渠道", Description: "向指定渠道发送测试通知", Tags: tag, Security: security}, nc.TestChannel)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/notify/bindings", OperationID: "GetBindings", Summary: "获取事件绑定列表", Description: "获取所有事件绑定列表", Tags: tag, Security: security}, nc.GetBindings)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/notify/bindings", OperationID: "SaveBinding", Summary: "保存事件绑定", Description: "保存或更新事件绑定", Tags: tag, Security: security}, nc.SaveBinding)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/notify/bindings/batch", OperationID: "BatchSaveBindings", Summary: "批量保存事件绑定", Description: "批量保存事件绑定", Tags: tag, Security: security}, nc.BatchSaveBindings)
	huma.Register(api, huma.Operation{Method: http.MethodDelete, Path: "/notify/bindings/{id}", OperationID: "DeleteBinding", Summary: "删除事件绑定", Description: "删除指定事件绑定", Tags: tag, Security: security}, nc.DeleteBinding)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/notify/send", OperationID: "SendNotify", Summary: "发送通知", Description: "按 `channel_id` 与 `title` 发送一条通知（供脚本调用）。需通过 `notify-token` 请求头鉴权。", Tags: notifyTag, Security: notifySecurity}, nc.SendNotify)
}
