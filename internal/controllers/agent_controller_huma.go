package controllers

import (
	"context"
	"net/http"
	"time"

	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/models/vo"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

// ===========================================================================
// /api/v1 管理接口 —— Agent 管理
// ===========================================================================

// TAAGentListOutput 获取 Agent 列表
type TAAGentListOutput struct {
	Body utils.HumaResponse[[]*vo.AgentVO]
}

// TAAGentList 获取 Agent 列表
func (c *AgentController) TAAGentList(ctx context.Context, input *struct{}) (*TAAGentListOutput, error) {
	agents := c.agentService.List()

	return &TAAGentListOutput{
		Body: utils.HumaResponse[[]*vo.AgentVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToAgentVOListFromModels(agents),
		},
	}, nil
}

// TAAGentGetVersionOutput 获取 Agent 最新版本信息
type TAAGentGetVersionOutput struct {
	Body utils.HumaResponse[struct {
		Version   string              `json:"version"`
		Platforms []map[string]string `json:"platforms"`
	}]
}

// TAAGentGetVersion 获取 Agent 最新版本信息
func (c *AgentController) TAAGentGetVersion(ctx context.Context, input *struct{}) (*TAAGentGetVersionOutput, error) {
	version := c.agentService.GetLatestVersion()
	platforms := c.agentService.GetAvailablePlatforms()

	return &TAAGentGetVersionOutput{
		Body: utils.HumaResponse[struct {
			Version   string              `json:"version"`
			Platforms []map[string]string `json:"platforms"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				Version   string              `json:"version"`
				Platforms []map[string]string `json:"platforms"`
			}{
				Version:   version,
				Platforms: platforms,
			},
		},
	}, nil
}

// TAAGentUpdateInput 更新 Agent
type TAAGentUpdateInput struct {
	ID   string `path:"id" description:"Agent ID"`
	Body struct {
		Name            string                     `json:"name" description:"Agent 名称"`
		Description     string                     `json:"description" description:"描述"`
		Enabled         bool                       `json:"enabled" description:"是否启用"`
		SchedulerConfig *vo.AgentSchedulerConfigVO `json:"scheduler_config" description:"调度配置"`
	}
}

// TAAGentUpdateOutput 更新 Agent
type TAAGentUpdateOutput struct {
	Body utils.HumaResponse[any]
}

// TAAGentUpdate 更新 Agent
func (c *AgentController) TAAGentUpdate(ctx context.Context, input *TAAGentUpdateInput) (*TAAGentUpdateOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的 ID")
	}

	req := input.Body

	// 获取旧状态
	oldAgent := c.agentService.GetByID(input.ID)
	if oldAgent == nil {
		return nil, utils.HumaNotFound("Agent 不存在")
	}
	wasEnabled := utils.DerefBool(oldAgent.Enabled, true)

	var schedulerConfig models.AgentSchedulerConfig
	if req.SchedulerConfig != nil {
		schedulerConfig.WorkerCount = req.SchedulerConfig.WorkerCount
		schedulerConfig.QueueSize = req.SchedulerConfig.QueueSize
		schedulerConfig.RateInterval = time.Duration(req.SchedulerConfig.RateInterval) * time.Millisecond
		schedulerConfig.Verbose = req.SchedulerConfig.Verbose
		schedulerConfig.StrictQueue = req.SchedulerConfig.StrictQueue
	}

	if err := c.agentService.Update(input.ID, req.Name, req.Description, req.Enabled, schedulerConfig); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	// 如果启用状态发生变化，通知 Agent
	if wasEnabled != req.Enabled {
		if req.Enabled {
			c.wsManager.SendToAgent(input.ID, services.WSTypeEnabled, map[string]interface{}{
				"message": "Agent 已启用",
			})
			c.wsManager.BroadcastTasks(input.ID)
		} else {
			c.wsManager.SendToAgent(input.ID, services.WSTypeDisabled, map[string]interface{}{
				"message": "Agent 已禁用",
			})
		}
	}

	// 推送最新的调度配置给 Agent (如果 Agent 在线)
	if req.Enabled {
		updatedAgent := c.agentService.GetByID(input.ID)
		if updatedAgent != nil {
			c.wsManager.SendToAgent(input.ID, services.WSTypeConnected, map[string]interface{}{
				"agent_id":         input.ID,
				"name":             req.Name,
				"scheduler_config": c.getActiveSchedulerConfig(updatedAgent),
			})
		}
	}

	return &TAAGentUpdateOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "更新成功",
		},
	}, nil
}

// TAAGentDeleteInput 删除 Agent
type TAAGentDeleteInput struct {
	ID string `path:"id" description:"Agent ID"`
}

// TAAGentDeleteOutput 删除 Agent
type TAAGentDeleteOutput struct {
	Body utils.HumaResponse[any]
}

// TAAGentDelete 删除 Agent
func (c *AgentController) TAAGentDelete(ctx context.Context, input *TAAGentDeleteInput) (*TAAGentDeleteOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的 ID")
	}

	if err := c.agentService.Delete(input.ID); err != nil {
		return nil, utils.HumaBadRequest(err.Error())
	}

	return &TAAGentDeleteOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "删除成功",
		},
	}, nil
}

// TAAGentRegenerateTokenInput 重新生成 Token
type TAAGentRegenerateTokenInput struct {
	ID string `path:"id" description:"Agent ID"`
}

// TAAGentRegenerateTokenOutput 重新生成 Token
type TAAGentRegenerateTokenOutput struct {
	Body utils.HumaResponse[struct {
		Token string `json:"token"`
	}]
}

// TAAGentRegenerateToken 重新生成 Token
func (c *AgentController) TAAGentRegenerateToken(ctx context.Context, input *TAAGentRegenerateTokenInput) (*TAAGentRegenerateTokenOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的 ID")
	}

	token, err := c.agentService.RegenerateToken(input.ID)
	if err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &TAAGentRegenerateTokenOutput{
		Body: utils.HumaResponse[struct {
			Token string `json:"token"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				Token string `json:"token"`
			}{Token: token},
		},
	}, nil
}

// TAAGentForceUpdateInput 强制更新 Agent
type TAAGentForceUpdateInput struct {
	ID string `path:"id" description:"Agent ID"`
}

// TAAGentForceUpdateOutput 强制更新 Agent
type TAAGentForceUpdateOutput struct {
	Body utils.HumaResponse[any]
}

// TAAGentForceUpdate 强制更新 Agent
func (c *AgentController) TAAGentForceUpdate(ctx context.Context, input *TAAGentForceUpdateInput) (*TAAGentForceUpdateOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的 ID")
	}

	if err := c.agentService.SetForceUpdate(input.ID); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &TAAGentForceUpdateOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "已标记强制更新，Agent 下次心跳时将自动更新",
		},
	}, nil
}

// TAAGentListTokensOutput 获取令牌列表
type TAAGentListTokensOutput struct {
	Body utils.HumaResponse[[]*vo.AgentTokenVO]
}

// TAAGentListTokens 获取令牌列表
func (c *AgentController) TAAGentListTokens(ctx context.Context, input *struct{}) (*TAAGentListTokensOutput, error) {
	tokens := c.agentService.ListTokens()

	return &TAAGentListTokensOutput{
		Body: utils.HumaResponse[[]*vo.AgentTokenVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToAgentTokenVOListFromModels(tokens),
		},
	}, nil
}

// TAAGentCreateTokenInput 创建令牌
type TAAGentCreateTokenInput struct {
	Body struct {
		Remark    string `json:"remark" description:"备注"`
		MaxUses   int    `json:"max_uses" description:"最大使用次数"`
		ExpiresAt string `json:"expires_at" description:"过期时间，格式: 2006-01-02 15:04:05"`
	}
}

// TAAGentCreateTokenOutput 创建令牌
type TAAGentCreateTokenOutput struct {
	Body utils.HumaResponse[*vo.AgentTokenVO]
}

// TAAGentCreateToken 创建令牌
func (c *AgentController) TAAGentCreateToken(ctx context.Context, input *TAAGentCreateTokenInput) (*TAAGentCreateTokenOutput, error) {
	req := input.Body

	var expiresAt *time.Time
	if req.ExpiresAt != "" {
		t, err := time.ParseInLocation("2006-01-02 15:04:05", req.ExpiresAt, time.Local)
		if err != nil {
			return nil, utils.HumaBadRequest("过期时间格式错误")
		}
		expiresAt = &t
	}

	token, err := c.agentService.CreateToken(req.Remark, req.MaxUses, expiresAt)
	if err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &TAAGentCreateTokenOutput{
		Body: utils.HumaResponse[*vo.AgentTokenVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToAgentTokenVO(token),
		},
	}, nil
}

// TAAGentDeleteTokenInput 删除令牌
type TAAGentDeleteTokenInput struct {
	ID string `path:"id" description:"令牌ID"`
}

// TAAGentDeleteTokenOutput 删除令牌
type TAAGentDeleteTokenOutput struct {
	Body utils.HumaResponse[any]
}

// TAAGentDeleteToken 删除令牌
func (c *AgentController) TAAGentDeleteToken(ctx context.Context, input *TAAGentDeleteTokenInput) (*TAAGentDeleteTokenOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的 ID")
	}

	if err := c.agentService.DeleteToken(input.ID); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &TAAGentDeleteTokenOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "删除成功",
		},
	}, nil
}

// RegisterAPIAgentRoutes 注册 /api/v1 Agent 管理 Huma 路由
func (c *AgentController) RegisterAPIAgentRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/agents",
		OperationID: "apiAgentList",
		Summary:     "获取 Agent 列表",
		Description: "获取 Agent 列表",
		Tags:        []string{"Agent 管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAAGentList)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/agents/version",
		OperationID: "apiAgentGetVersion",
		Summary:     "获取 Agent 最新版本信息",
		Description: "获取 Agent 最新版本及可用平台",
		Tags:        []string{"Agent 管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAAGentGetVersion)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPut,
		Path:        "/agents/{id}",
		OperationID: "apiAgentUpdate",
		Summary:     "更新 Agent",
		Description: "更新 Agent 的名称、描述、启用状态与调度配置",
		Tags:        []string{"Agent 管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAAGentUpdate)

	huma.Register(api, huma.Operation{
		Method:      http.MethodDelete,
		Path:        "/agents/{id}",
		OperationID: "apiAgentDelete",
		Summary:     "删除 Agent",
		Description: "删除指定的 Agent",
		Tags:        []string{"Agent 管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAAGentDelete)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/agents/{id}/token",
		OperationID: "apiAgentRegenerateToken",
		Summary:     "重新生成 Token",
		Description: "重新生成指定 Agent 的 Token",
		Tags:        []string{"Agent 管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAAGentRegenerateToken)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/agents/{id}/update",
		OperationID: "apiAgentForceUpdate",
		Summary:     "强制更新 Agent",
		Description: "标记指定 Agent 进行强制更新",
		Tags:        []string{"Agent 管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAAGentForceUpdate)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/agents/tokens",
		OperationID: "apiAgentListTokens",
		Summary:     "获取令牌列表",
		Description: "获取 Agent 接入令牌列表",
		Tags:        []string{"Agent 管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAAGentListTokens)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/agents/tokens",
		OperationID: "apiAgentCreateToken",
		Summary:     "创建令牌",
		Description: "创建 Agent 接入令牌",
		Tags:        []string{"Agent 管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAAGentCreateToken)

	huma.Register(api, huma.Operation{
		Method:      http.MethodDelete,
		Path:        "/agents/tokens/{id}",
		OperationID: "apiAgentDeleteToken",
		Summary:     "删除令牌",
		Description: "删除指定的 Agent 接入令牌",
		Tags:        []string{"Agent 管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAAGentDeleteToken)
}
