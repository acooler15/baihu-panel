package controllers

import (
	"context"
	"net/http"

	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

// ===========================================================================
// /api/v1 管理接口 —— 通知管理
// ===========================================================================

// TASaveChannelInput 保存/更新渠道
type TASaveChannelInput struct {
	Body services.NotifyChannel
}

// TASaveChannelOutput 保存/更新渠道
type TASaveChannelOutput struct {
	Body utils.HumaResponse[any]
}

// TASaveChannel 保存/更新渠道
func (nc *NotificationController) TASaveChannel(ctx context.Context, input *TASaveChannelInput) (*TASaveChannelOutput, error) {
	req := input.Body
	if req.Name == "" || req.Type == "" {
		return nil, utils.HumaBadRequest("渠道名称和类型不能为空")
	}

	if err := nc.notifyService.SaveChannel(req); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &TASaveChannelOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "保存成功",
		},
	}, nil
}

// TADeleteChannelInput 删除渠道
type TADeleteChannelInput struct {
	ID string `path:"id" description:"渠道ID"`
}

// TADeleteChannelOutput 删除渠道
type TADeleteChannelOutput struct {
	Body utils.HumaResponse[any]
}

// TADeleteChannel 删除渠道
func (nc *NotificationController) TADeleteChannel(ctx context.Context, input *TADeleteChannelInput) (*TADeleteChannelOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("缺少渠道ID")
	}

	if err := nc.notifyService.DeleteChannel(input.ID); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &TADeleteChannelOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "删除成功",
		},
	}, nil
}

// TATestChannelInput 测试渠道
type TATestChannelInput struct {
	Body services.NotifyChannel
}

// TATestChannelOutput 测试渠道
type TATestChannelOutput struct {
	Body utils.HumaResponse[*services.NotifyResult]
}

// TATestChannel 测试渠道
func (nc *NotificationController) TATestChannel(ctx context.Context, input *TATestChannelInput) (*TATestChannelOutput, error) {
	result := nc.notifyService.SendToChannel(input.Body, &services.NotifyMessage{
		Title: "🔔 白虎面板测试通知",
		Text:  "如果你看到这条消息，说明通知渠道配置正确！",
	})

	return &TATestChannelOutput{
		Body: utils.HumaResponse[*services.NotifyResult]{
			Code: 200,
			Msg:  "success",
			Data: result,
		},
	}, nil
}

// TASaveBindingInput 保存事件绑定
type TASaveBindingInput struct {
	Body struct {
		ID     string         `json:"id" description:"绑定ID，空则新建"`
		Type   string         `json:"type" description:"绑定类型"`
		Event  string         `json:"event" description:"事件类型"`
		WayID  string         `json:"way_id" description:"渠道ID"`
		DataID string         `json:"data_id" description:"关联ID"`
		Extra  models.BigText `json:"extra" description:"额外配置"`
	}
}

// TASaveBindingOutput 保存事件绑定
type TASaveBindingOutput struct {
	Body utils.HumaResponse[*models.NotifyBinding]
}

// TASaveBinding 保存事件绑定
func (nc *NotificationController) TASaveBinding(ctx context.Context, input *TASaveBindingInput) (*TASaveBindingOutput, error) {
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

	return &TASaveBindingOutput{
		Body: utils.HumaResponse[*models.NotifyBinding]{
			Code: 200,
			Msg:  "success",
			Data: binding,
		},
	}, nil
}

// TADeleteBindingInput 删除事件绑定
type TADeleteBindingInput struct {
	ID string `path:"id" description:"绑定ID"`
}

// TADeleteBindingOutput 删除事件绑定
type TADeleteBindingOutput struct {
	Body utils.HumaResponse[any]
}

// TADeleteBinding 删除事件绑定
func (nc *NotificationController) TADeleteBinding(ctx context.Context, input *TADeleteBindingInput) (*TADeleteBindingOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("缺少绑定ID")
	}

	if err := nc.notifyService.DeleteBinding(input.ID); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &TADeleteBindingOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "删除成功",
		},
	}, nil
}

// TABatchSaveBindingsInput 批量保存事件绑定
type TABatchSaveBindingsInput struct {
	Body struct {
		Type     string                 `json:"type" description:"绑定类型"`
		DataID   string                 `json:"data_id" description:"关联ID"`
		Bindings []models.NotifyBinding `json:"bindings" description:"绑定列表"`
	}
}

// TABatchSaveBindingsOutput 批量保存事件绑定
type TABatchSaveBindingsOutput struct {
	Body utils.HumaResponse[any]
}

// TABatchSaveBindings 批量保存事件绑定
func (nc *NotificationController) TABatchSaveBindings(ctx context.Context, input *TABatchSaveBindingsInput) (*TABatchSaveBindingsOutput, error) {
	req := input.Body
	if req.Type == "" {
		return nil, utils.HumaBadRequest("类型不能为空")
	}

	if err := nc.notifyService.BatchSaveBindings(req.Type, req.DataID, req.Bindings); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &TABatchSaveBindingsOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "保存成功",
		},
	}, nil
}

// TASendNotificationInput 发送通知
type TASendNotificationInput struct {
	Body struct {
		ChannelID string `json:"channel_id" description:"渠道ID"`
		Title     string `json:"title" description:"标题"`
		Text      string `json:"text" description:"内容"`
	}
}

// TASendNotificationOutput 发送通知
type TASendNotificationOutput struct {
	Body utils.HumaResponse[*services.NotifyResult]
}

// TASendNotification 发送通知
func (nc *NotificationController) TASendNotification(ctx context.Context, input *TASendNotificationInput) (*TASendNotificationOutput, error) {
	req := input.Body
	if req.ChannelID == "" || req.Title == "" {
		return nil, utils.HumaBadRequest("channel_id 和 title 不能为空")
	}

	result := nc.notifyService.SendByChannelID(req.ChannelID, &services.NotifyMessage{
		Title: req.Title,
		Text:  req.Text,
	})

	return &TASendNotificationOutput{
		Body: utils.HumaResponse[*services.NotifyResult]{
			Code: 200,
			Msg:  "success",
			Data: result,
		},
	}, nil
}

// RegisterAPINotificationRoutes 注册 /api/v1 通知管理 Huma 路由
func (nc *NotificationController) RegisterAPINotificationRoutes(api huma.API) {
	// 渠道类型（只读）
	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/notify/types",
		OperationID: "apiGetChannelTypes",
		Summary:     "获取支持的渠道类型",
		Description: "获取支持的通知渠道类型与事件类型",
		Tags:        []string{"通知管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, nc.TAGetChannelTypes)

	// 渠道列表
	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/notify/channels",
		OperationID: "apiGetChannels",
		Summary:     "获取所有渠道",
		Description: "获取所有通知渠道",
		Tags:        []string{"通知管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, nc.TAGetChannels)

	// 保存/更新渠道
	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/notify/channels",
		OperationID: "apiSaveChannel",
		Summary:     "保存/更新渠道",
		Description: "保存或更新通知渠道",
		Tags:        []string{"通知管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, nc.TASaveChannel)

	// 删除渠道
	huma.Register(api, huma.Operation{
		Method:      http.MethodDelete,
		Path:        "/notify/channels/{id}",
		OperationID: "apiDeleteChannel",
		Summary:     "删除渠道",
		Description: "删除指定通知渠道",
		Tags:        []string{"通知管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, nc.TADeleteChannel)

	// 测试渠道
	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/notify/channels/test",
		OperationID: "apiTestChannel",
		Summary:     "测试渠道",
		Description: "向指定渠道发送测试通知",
		Tags:        []string{"通知管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, nc.TATestChannel)

	// 绑定列表
	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/notify/bindings",
		OperationID: "apiGetBindings",
		Summary:     "获取事件绑定列表",
		Description: "获取所有事件绑定列表",
		Tags:        []string{"通知管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, nc.TAGetBindings)

	// 保存绑定
	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/notify/bindings",
		OperationID: "apiSaveBinding",
		Summary:     "保存事件绑定",
		Description: "保存或更新事件绑定",
		Tags:        []string{"通知管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, nc.TASaveBinding)

	// 批量保存绑定
	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/notify/bindings/batch",
		OperationID: "apiBatchSaveBindings",
		Summary:     "批量保存事件绑定",
		Description: "批量保存事件绑定",
		Tags:        []string{"通知管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, nc.TABatchSaveBindings)

	// 删除绑定
	huma.Register(api, huma.Operation{
		Method:      http.MethodDelete,
		Path:        "/notify/bindings/{id}",
		OperationID: "apiDeleteBinding",
		Summary:     "删除事件绑定",
		Description: "删除指定事件绑定",
		Tags:        []string{"通知管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, nc.TADeleteBinding)
}

// TAGetChannelTypesOutput 获取支持的渠道类型
type TAGetChannelTypesOutput struct {
	Body utils.HumaResponse[struct {
		ChannelTypes []map[string]string `json:"channel_types"`
		EventTypes   []map[string]string `json:"event_types"`
	}]
}

// TAGetChannelTypes 获取支持的渠道类型
func (nc *NotificationController) TAGetChannelTypes(ctx context.Context, input *struct{}) (*TAGetChannelTypesOutput, error) {
	return &TAGetChannelTypesOutput{
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

// TAGetChannelsOutput 获取所有渠道
type TAGetChannelsOutput struct {
	Body utils.HumaResponse[[]services.NotifyChannel]
}

// TAGetChannels 获取所有渠道
func (nc *NotificationController) TAGetChannels(ctx context.Context, input *struct{}) (*TAGetChannelsOutput, error) {
	channels := nc.notifyService.GetChannels()
	return &TAGetChannelsOutput{
		Body: utils.HumaResponse[[]services.NotifyChannel]{
			Code: 200,
			Msg:  "success",
			Data: channels,
		},
	}, nil
}

// TAGetBindingsOutput 获取事件绑定列表
type TAGetBindingsOutput struct {
	Body utils.HumaResponse[[]models.NotifyBinding]
}

// TAGetBindings 获取事件绑定列表
func (nc *NotificationController) TAGetBindings(ctx context.Context, input *struct{}) (*TAGetBindingsOutput, error) {
	bindings := nc.notifyService.GetBindings()
	return &TAGetBindingsOutput{
		Body: utils.HumaResponse[[]models.NotifyBinding]{
			Code: 200,
			Msg:  "success",
			Data: bindings,
		},
	}, nil
}
