package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/logger"
	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/models/vo"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/services/tasks"
	"github.com/engigu/baihu-panel/internal/utils"
	"github.com/gin-gonic/gin"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gorilla/websocket"
)

var agentUpgrader = websocket.Upgrader{
	CheckOrigin: utils.CheckWSOrigin,
}

// AgentController Agent 控制器
type AgentController struct {
	agentService    *services.AgentService
	wsManager       *services.AgentWSManager
	settingsService *services.SettingsService
}

// NewAgentController 创建 Agent 控制器
func NewAgentController(settingsService *services.SettingsService) *AgentController {
	return &AgentController{
		agentService:    services.NewAgentService(),
		wsManager:       services.GetAgentWSManager(),
		settingsService: settingsService,
	}
}

// getActiveSchedulerConfig 获取 Agent 的实际调度配置（若为空或零值，则使用系统默认的 settings）
func (c *AgentController) getActiveSchedulerConfig(agent *models.Agent) map[string]interface{} {
	workerCount := agent.SchedulerConfig.WorkerCount
	queueSize := agent.SchedulerConfig.QueueSize
	rateInterval := int(agent.SchedulerConfig.RateInterval / time.Millisecond)
	strictQueue := agent.SchedulerConfig.StrictQueue

	// 如果未配置（WorkerCount <= 0），则使用全局系统设置
	if workerCount <= 0 {
		workerCount = getIntSetting(c.settingsService, constant.SectionScheduler, constant.KeyWorkerCount, 4)
		queueSize = getIntSetting(c.settingsService, constant.SectionScheduler, constant.KeyQueueSize, 100)
		rateInterval = getIntSetting(c.settingsService, constant.SectionScheduler, constant.KeyRateInterval, 200)
		strictQueue = false
	}

	return map[string]interface{}{
		"worker_count":  workerCount,
		"queue_size":    queueSize,
		"rate_interval": rateInterval,
		"strict_queue":  strictQueue,
	}
}

// ========== Agent API（供 Agent 调用）==========

// ========== Agent API（供 Agent 调用）==========

// getAgentToken 从请求头获取 Agent Token
func (c *AgentController) getAgentToken(ctx *gin.Context) string {
	auth := ctx.GetHeader("Authorization")
	if auth == "" {
		return ""
	}
	// Bearer <token>
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) == 2 && parts[0] == "Bearer" {
		return parts[1]
	}
	return auth
}

// ========== WebSocket ==========

// WSConnect Agent WebSocket 连接
func (c *AgentController) WSConnect(ctx *gin.Context) {
	// 添加 panic 恢复
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("[AgentWS] WSConnect panic: %v", r)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
		}
	}()

	ip := ctx.ClientIP()

	// 打印请求信息用于调试
	logger.Infof("[AgentWS] 收到连接请求: IP=%s, URL=%s", ip, ctx.Request.URL.String())

	// 检查 IP 限流
	if allowed, reason := c.wsManager.CheckRateLimit(ip); !allowed {
		logger.Warnf("[AgentWS] IP %s 被限流: %s", ip, reason)
		ctx.JSON(http.StatusTooManyRequests, gin.H{"error": reason})
		return
	}

	token := ctx.Query("token")
	if token == "" {
		c.wsManager.RecordConnectFail(ip)
		logger.Warnf("[AgentWS] 连接失败: 缺少 token, IP=%s", ip)
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "缺少 token"})
		return
	}

	machineID := ctx.Query("machine_id")
	logger.Infof("[AgentWS] Token: %s..., MachineID: %s...", token[:8], machineID[:16])

	isNewAgent := false

	// 先尝试用 token 查找已有 Agent
	agent := c.agentService.GetByToken(token)
	logger.Infof("[AgentWS] GetByToken 结果: agent=%v", agent != nil)

	// 如果没找到，尝试用令牌注册（会检查 machine_id 是否已存在）
	if agent == nil {
		logger.Infof("[AgentWS] 尝试注册新 Agent")
		var err error
		agent, isNewAgent, err = c.agentService.RegisterByToken(token, machineID, ip)
		if err != nil {
			c.wsManager.RecordConnectFail(ip)
			logger.Warnf("[AgentWS] 注册失败: %v, IP=%s, token=%s", err, ip, token[:8]+"...")
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		logger.Infof("[AgentWS] 注册成功: Agent #%s, isNew=%v", agent.ID, isNewAgent)
	}

	if !utils.DerefBool(agent.Enabled, true) {
		c.wsManager.RecordConnectFail(ip)
		logger.Warnf("[AgentWS] Agent #%s 已禁用, IP=%s", agent.ID, ip)
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Agent 已禁用"})
		return
	}

	logger.Infof("[AgentWS] 准备升级连接: Agent #%s, IP=%s", agent.ID, ip)
	conn, err := agentUpgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		logger.Errorf("[AgentWS] 升级连接失败: %v, Agent #%s, IP=%s", err, agent.ID, ip)
		return
	}
	conn.SetReadLimit(constant.MaxMessageSize)

	// 连接成功，重置失败计数
	c.wsManager.RecordConnectSuccess(ip)

	// 注册连接
	ac := c.wsManager.Register(agent.ID, conn, ip)

	// 更新 Agent 状态
	c.agentService.Heartbeat(token, ip, "", "", "", "", "")

	// 获取调度配置并发送连接成功消息（包含注册状态和调度配置）
	schedCfg := c.getActiveSchedulerConfig(agent)
	c.wsManager.SendToAgent(agent.ID, services.WSTypeConnected, map[string]interface{}{
		"agent_id":         agent.ID,
		"name":             agent.Name,
		"is_new_agent":     isNewAgent,
		"machine_id":       machineID,
		"scheduler_config": schedCfg,
	})

	logger.Infof("[AgentWS] Agent #%s 连接成功 (配置: %v)", agent.ID, schedCfg)

	// 启动读写协程
	go c.wsWritePump(ac)
	go c.wsReadPump(ac, agent)

	// 主动推送任务列表
	go c.wsManager.BroadcastTasks(agent.ID)
}

// wsReadPump 读取消息
func (c *AgentController) wsReadPump(ac *services.AgentConnection, agent *models.Agent) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("[AgentWS] Agent #%s wsReadPump panic: %v", agent.ID, r)
		}
		logger.Infof("[AgentWS] Agent #%s wsReadPump 退出", agent.ID)
		c.wsManager.Unregister(agent.ID, ac)
	}()

	// 检查连接是否有效（可能是旧连接被新连接替换）
	if ac == nil || ac.IsClosed() {
		return
	}

	ac.SetReadDeadline(time.Now().Add(90 * time.Second))
	// 注意：SetPongHandler 需要直接访问 Conn，但这里我们在连接建立后立即设置
	// 所以是安全的，因为此时连接还没有被其他 goroutine 关闭
	ac.Conn.SetPongHandler(func(string) error {
		ac.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	for {
		_, message, err := ac.ReadMessage()
		if err != nil {
			logger.Warnf("[AgentWS] Agent #%s 读取错误: %v", agent.ID, err)
			break
		}

		var msg services.WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		c.handleWSMessage(ac, agent, &msg)
	}
}

// wsWritePump 写入消息
func (c *AgentController) wsWritePump(ac *services.AgentConnection) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("[AgentWS] Agent #%s wsWritePump panic: %v", ac.AgentID, r)
		}
		logger.Infof("[AgentWS] Agent #%s wsWritePump 退出", ac.AgentID)
	}()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-ac.Send:
			if !ok {
				logger.Warnf("[AgentWS] Agent #%s Send channel 已关闭", ac.AgentID)
				return
			}
			if ac.IsClosed() {
				logger.Warnf("[AgentWS] Agent #%s 连接已关闭(write)", ac.AgentID)
				return
			}
			if err := ac.WriteMessage(message); err != nil {
				logger.Warnf("[AgentWS] Agent #%s 写入消息失败: %v", ac.AgentID, err)
				return
			}
		case <-ticker.C:
			if ac.IsClosed() {
				return
			}
			if err := ac.WritePing(); err != nil {
				logger.Warnf("[AgentWS] Agent #%s 发送 Ping 失败: %v", ac.AgentID, err)
				return
			}
		}
	}
}

// handleWSMessage 处理 WebSocket 消息
func (c *AgentController) handleWSMessage(ac *services.AgentConnection, agent *models.Agent, msg *services.WSMessage) {
	switch msg.Type {
	case services.WSTypeHeartbeat:
		c.handleHeartbeat(ac, agent, msg.Data)

	case services.WSTypeTaskResult:
		c.handleTaskResult(agent, msg.Data)

	case services.WSTypeTaskLog:
		c.handleTaskLog(agent, msg.Data)

	case services.WSTypeFetchTasks:
		c.handleFetchTasks(agent)

	case services.WSTypeTaskHeartbeat: // 任务心跳
		c.handleTaskHeartbeat(agent, msg.Data)
	}
}

// handleTaskHeartbeat 处理任务心跳
func (c *AgentController) handleTaskHeartbeat(_ *models.Agent, data json.RawMessage) {
	var req struct {
		LogID    string `json:"log_id"`
		Duration int64  `json:"duration"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		logger.Errorf("[AgentWS] 解析心跳消息失败: %v", err)
		return
	}
	if req.LogID != "" {
		logger.Infof("[AgentWS] 收到任务心跳: LogID=%s, Duration=%dms", req.LogID, req.Duration)
		c.agentService.UpdateTaskDuration(req.LogID, req.Duration)
	}
}

// handleFetchTasks 处理 Agent 请求任务列表
func (c *AgentController) handleFetchTasks(agent *models.Agent) {
	tasks := c.agentService.GetTasks(agent.ID)
	c.wsManager.SendToAgent(agent.ID, services.WSTypeTasks, map[string]interface{}{
		"tasks": tasks,
	})
	logger.Infof("[AgentWS] Agent #%s 请求任务列表，返回 %d 个任务", agent.ID, len(tasks))
}

// handleHeartbeat 处理心跳
func (c *AgentController) handleHeartbeat(ac *services.AgentConnection, agent *models.Agent, data json.RawMessage) {
	var req struct {
		Version    string `json:"version"`
		BuildTime  string `json:"build_time"`
		Hostname   string `json:"hostname"`
		OS         string `json:"os"`
		Arch       string `json:"arch"`
		AutoUpdate bool   `json:"auto_update"`
	}
	json.Unmarshal(data, &req)

	ac.UpdatePing()

	// 更新 Agent 信息（使用连接时保存的 IP）
	c.agentService.Heartbeat(agent.Token, ac.IP, req.Version, req.BuildTime, req.Hostname, req.OS, req.Arch)

	// 检查是否需要更新
	latestVersion := c.agentService.GetLatestVersion()
	needUpdate := c.agentService.CheckNeedUpdate(req.Version, req.BuildTime)
	forceUpdate := agent.ForceUpdate

	if forceUpdate && needUpdate {
		c.agentService.ClearForceUpdate(agent.ID)
	}

	// 发送心跳响应
	response := map[string]interface{}{
		"agent_id":       agent.ID,
		"name":           agent.Name,
		"need_update":    needUpdate,
		"force_update":   forceUpdate,
		"latest_version": latestVersion,
	}
	c.wsManager.SendToAgent(agent.ID, services.WSTypeHeartbeatAck, response)
}

// handleTaskResult 处理任务结果
func (c *AgentController) handleTaskResult(agent *models.Agent, data json.RawMessage) {
	var result models.AgentTaskResult
	if err := json.Unmarshal(data, &result); err != nil {
		return
	}

	result.AgentID = agent.ID
	c.agentService.ReportResult(&result)
}

// handleTaskLog 处理 Agent 发送的实时日志
func (c *AgentController) handleTaskLog(_ *models.Agent, data json.RawMessage) {
	var logMsg struct {
		LogID   string `json:"log_id"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(data, &logMsg); err != nil {
		logger.Errorf("[AgentWS] 解析日志消息失败: %v", err)
		return
	}

	tl := tasks.GetActiveLog(logMsg.LogID)
	if tl != nil {
		tl.Write([]byte(logMsg.Content))
	} else {
		logger.Warnf("[AgentWS] 收到任务日志 but could not find active TinyLog: LogID=%s, ContentSize=%d", logMsg.LogID, len(logMsg.Content))
	}
}

// NotifyTaskUpdate 通知 Agent 任务更新
func (c *AgentController) NotifyTaskUpdate(agentID string) {
	c.wsManager.BroadcastTasks(agentID)
}

// getIntSetting 辅助方法
func getIntSetting(s *services.SettingsService, section, key string, defaultVal int) int {
	val := s.Get(section, key)

	if val == "" {
		return defaultVal
	}
	if result, err := strconv.Atoi(val); err == nil {
		return result
	}
	return defaultVal
}

// ===========================================================================
// Agent 管理业务方法（Huma）
// ===========================================================================

// AgentListOutput 获取 Agent 列表
type AgentListOutput struct {
	Body utils.HumaResponse[[]*vo.AgentVO]
}

// AgentList 获取 Agent 列表
func (c *AgentController) AgentList(ctx context.Context, input *struct{}) (*AgentListOutput, error) {
	agents := c.agentService.List()

	return &AgentListOutput{
		Body: utils.HumaResponse[[]*vo.AgentVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToAgentVOListFromModels(agents),
		},
	}, nil
}

// AgentGetVersionOutput 获取 Agent 最新版本信息
type AgentGetVersionOutput struct {
	Body utils.HumaResponse[struct {
		Version   string              `json:"version"`
		Platforms []map[string]string `json:"platforms"`
	}]
}

// AgentGetVersion 获取 Agent 最新版本信息
func (c *AgentController) AgentGetVersion(ctx context.Context, input *struct{}) (*AgentGetVersionOutput, error) {
	version := c.agentService.GetLatestVersion()
	platforms := c.agentService.GetAvailablePlatforms()

	return &AgentGetVersionOutput{
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

// AgentUpdateInput 更新 Agent
type AgentUpdateInput struct {
	ID   string `path:"id" description:"Agent ID"`
	Body struct {
		Name            string                     `json:"name" description:"Agent 名称"`
		Description     string                     `json:"description" description:"描述"`
		Enabled         bool                       `json:"enabled" description:"是否启用"`
		SchedulerConfig *vo.AgentSchedulerConfigVO `json:"scheduler_config" description:"调度配置"`
	}
}

// AgentUpdateOutput 更新 Agent
type AgentUpdateOutput struct {
	Body utils.HumaResponse[any]
}

// AgentUpdate 更新 Agent
func (c *AgentController) AgentUpdate(ctx context.Context, input *AgentUpdateInput) (*AgentUpdateOutput, error) {
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

	return &AgentUpdateOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "更新成功",
		},
	}, nil
}

// AgentDeleteInput 删除 Agent
type AgentDeleteInput struct {
	ID string `path:"id" description:"Agent ID"`
}

// AgentDeleteOutput 删除 Agent
type AgentDeleteOutput struct {
	Body utils.HumaResponse[any]
}

// AgentDelete 删除 Agent
func (c *AgentController) AgentDelete(ctx context.Context, input *AgentDeleteInput) (*AgentDeleteOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的 ID")
	}

	if err := c.agentService.Delete(input.ID); err != nil {
		return nil, utils.HumaBadRequest(err.Error())
	}

	return &AgentDeleteOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "删除成功",
		},
	}, nil
}

// AgentRegenerateTokenInput 重新生成 Token
type AgentRegenerateTokenInput struct {
	ID string `path:"id" description:"Agent ID"`
}

// AgentRegenerateTokenOutput 重新生成 Token
type AgentRegenerateTokenOutput struct {
	Body utils.HumaResponse[struct {
		Token string `json:"token"`
	}]
}

// AgentRegenerateToken 重新生成 Token
func (c *AgentController) AgentRegenerateToken(ctx context.Context, input *AgentRegenerateTokenInput) (*AgentRegenerateTokenOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的 ID")
	}

	token, err := c.agentService.RegenerateToken(input.ID)
	if err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &AgentRegenerateTokenOutput{
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

// AgentForceUpdateInput 强制更新 Agent
type AgentForceUpdateInput struct {
	ID string `path:"id" description:"Agent ID"`
}

// AgentForceUpdateOutput 强制更新 Agent
type AgentForceUpdateOutput struct {
	Body utils.HumaResponse[any]
}

// AgentForceUpdate 强制更新 Agent
func (c *AgentController) AgentForceUpdate(ctx context.Context, input *AgentForceUpdateInput) (*AgentForceUpdateOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的 ID")
	}

	if err := c.agentService.SetForceUpdate(input.ID); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &AgentForceUpdateOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "已标记强制更新，Agent 下次心跳时将自动更新",
		},
	}, nil
}

// AgentListTokensOutput 获取令牌列表
type AgentListTokensOutput struct {
	Body utils.HumaResponse[[]*vo.AgentTokenVO]
}

// AgentListTokens 获取令牌列表
func (c *AgentController) AgentListTokens(ctx context.Context, input *struct{}) (*AgentListTokensOutput, error) {
	tokens := c.agentService.ListTokens()

	return &AgentListTokensOutput{
		Body: utils.HumaResponse[[]*vo.AgentTokenVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToAgentTokenVOListFromModels(tokens),
		},
	}, nil
}

// AgentCreateTokenInput 创建令牌
type AgentCreateTokenInput struct {
	Body struct {
		Remark    string `json:"remark" description:"备注"`
		MaxUses   int    `json:"max_uses" description:"最大使用次数"`
		ExpiresAt string `json:"expires_at" description:"过期时间，格式: 2006-01-02 15:04:05"`
	}
}

// AgentCreateTokenOutput 创建令牌
type AgentCreateTokenOutput struct {
	Body utils.HumaResponse[*vo.AgentTokenVO]
}

// AgentCreateToken 创建令牌
func (c *AgentController) AgentCreateToken(ctx context.Context, input *AgentCreateTokenInput) (*AgentCreateTokenOutput, error) {
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

	return &AgentCreateTokenOutput{
		Body: utils.HumaResponse[*vo.AgentTokenVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToAgentTokenVO(token),
		},
	}, nil
}

// AgentDeleteTokenInput 删除令牌
type AgentDeleteTokenInput struct {
	ID string `path:"id" description:"令牌ID"`
}

// AgentDeleteTokenOutput 删除令牌
type AgentDeleteTokenOutput struct {
	Body utils.HumaResponse[any]
}

// AgentDeleteToken 删除令牌
func (c *AgentController) AgentDeleteToken(ctx context.Context, input *AgentDeleteTokenInput) (*AgentDeleteTokenOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的 ID")
	}

	if err := c.agentService.DeleteToken(input.ID); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &AgentDeleteTokenOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "删除成功",
		},
	}, nil
}

// AgentDownloadInput 下载 Agent 程序
type AgentDownloadInput struct {
	OS   string `query:"os" default:"linux" description:"操作系统"`
	Arch string `query:"arch" default:"amd64" description:"CPU 架构"`
}

// AgentDownload 下载 Agent 程序（流式输出）
func (c *AgentController) AgentDownload(ctx context.Context, input *AgentDownloadInput) (*huma.StreamResponse, error) {
	osType := input.OS
	if osType == "" {
		osType = "linux"
	}
	arch := input.Arch
	if arch == "" {
		arch = "amd64"
	}

	data, filename, err := c.agentService.GetAgentBinary(osType, arch)
	if err != nil {
		return nil, utils.HumaNotFound(err.Error())
	}

	return &huma.StreamResponse{
		Body: func(hctx huma.Context) {
			hctx.SetHeader("Content-Disposition", `attachment; filename="`+filename+`"`)
			hctx.SetHeader("Content-Type", "application/gzip")
			hctx.SetHeader("Content-Length", strconv.Itoa(len(data)))
			io.Copy(hctx.BodyWriter(), bytes.NewReader(data))
		},
	}, nil
}

// ===========================================================================
// 路由注册
// ===========================================================================

// RegisterAPIAgentRoutes 注册 /api/v1 Agent 管理 Huma 路由
func (c *AgentController) RegisterAPIAgentRoutes(api huma.API) {
	security := []map[string][]string{{"CookieAuth": {}}}
	tag := []string{"Agent 管理"}

	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/agents", OperationID: "AgentList", Summary: "获取 Agent 列表", Description: "获取 Agent 列表", Tags: tag, Security: security}, c.AgentList)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/agents/version", OperationID: "AgentGetVersion", Summary: "获取 Agent 最新版本信息", Description: "获取 Agent 最新版本及可用平台", Tags: tag, Security: security}, c.AgentGetVersion)
	huma.Register(api, huma.Operation{Method: http.MethodPut, Path: "/agents/{id}", OperationID: "AgentUpdate", Summary: "更新 Agent", Description: "更新 Agent 的名称、描述、启用状态与调度配置", Tags: tag, Security: security}, c.AgentUpdate)
	huma.Register(api, huma.Operation{Method: http.MethodDelete, Path: "/agents/{id}", OperationID: "AgentDelete", Summary: "删除 Agent", Description: "删除指定的 Agent", Tags: tag, Security: security}, c.AgentDelete)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/agents/{id}/token", OperationID: "AgentRegenerateToken", Summary: "重新生成 Token", Description: "重新生成指定 Agent 的 Token", Tags: tag, Security: security}, c.AgentRegenerateToken)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/agents/{id}/update", OperationID: "AgentForceUpdate", Summary: "强制更新 Agent", Description: "标记指定 Agent 进行强制更新", Tags: tag, Security: security}, c.AgentForceUpdate)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/agents/tokens", OperationID: "AgentListTokens", Summary: "获取令牌列表", Description: "获取 Agent 接入令牌列表", Tags: tag, Security: security}, c.AgentListTokens)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/agents/tokens", OperationID: "AgentCreateToken", Summary: "创建令牌", Description: "创建 Agent 接入令牌", Tags: tag, Security: security}, c.AgentCreateToken)
	huma.Register(api, huma.Operation{Method: http.MethodDelete, Path: "/agents/tokens/{id}", OperationID: "AgentDeleteToken", Summary: "删除令牌", Description: "删除指定的 Agent 接入令牌", Tags: tag, Security: security}, c.AgentDeleteToken)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/agent/download", OperationID: "AgentDownload", Summary: "下载 Agent 程序", Description: "按 os（linux/darwin/windows）与 arch（amd64/arm64）查询参数下载对应平台的 Agent 二进制包。", Tags: tag, Security: security}, c.AgentDownload)
}

// ===========================================================================
// Agent 外部接口（供远程 Agent 调用，挂载于 /api/agent 前缀）
// ===========================================================================

// RegisterAPIAgentExternalRoutes 注册 /api/agent 供远程 Agent 调用的 Huma 路由。
// 鉴权由 handler 内部通过 `Authorization: Bearer <token>` 完成，selector 不套用任何业务中间件。
func (c *AgentController) RegisterAPIAgentExternalRoutes(api huma.API) {
	tag := []string{"Agent 外部接口"}

	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/agent/heartbeat", OperationID: "AgentHeartbeat", Summary: "Agent 心跳", Description: "Agent 周期性上报版本、主机等信息并获取更新状态。使用 `Authorization: Bearer <token>` 鉴权。", Tags: tag}, c.AgentHeartbeatHuma)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/agent/tasks", OperationID: "AgentGetTasks", Summary: "Agent 获取任务列表", Description: "Agent 拉取分配给自己的任务列表。使用 `Authorization: Bearer <token>` 鉴权。", Tags: tag}, c.AgentGetTasksHuma)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/agent/report", OperationID: "AgentReportResult", Summary: "Agent 上报执行结果", Description: "Agent 上报任务执行结果。使用 `Authorization: Bearer <token>` 鉴权。", Tags: tag}, c.AgentReportResultHuma)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/agent/download", OperationID: "AgentBinaryDownload", Summary: "下载 Agent 程序", Description: "下载 Agent 二进制包（无需鉴权，供 Agent 自动更新拉取）。", Tags: tag}, c.AgentDownload)
}

// AgentHeartbeatHumaInput Agent 心跳请求
type AgentHeartbeatHumaInput struct {
	Body struct {
		Version    string `json:"version"`
		BuildTime  string `json:"build_time"`
		Hostname   string `json:"hostname"`
		OS         string `json:"os"`
		Arch       string `json:"arch"`
		AutoUpdate bool   `json:"auto_update"`
	}
}

// AgentHeartbeatHumaOutput Agent 心跳结果
type AgentHeartbeatHumaOutput struct {
	Body utils.HumaResponse[struct {
		AgentID       string `json:"agent_id"`
		Name          string `json:"name"`
		NeedUpdate    bool   `json:"need_update"`
		ForceUpdate   bool   `json:"force_update"`
		LatestVersion string `json:"latest_version"`
	}]
}

// AgentHeartbeatHuma Agent 心跳
func (c *AgentController) AgentHeartbeatHuma(ctx context.Context, input *AgentHeartbeatHumaInput) (*AgentHeartbeatHumaOutput, error) {
	gc := utils.GetGinContext(ctx)
	var token string
	var ip string
	if gc != nil {
		token = c.getAgentToken(gc)
		ip = gc.ClientIP()
	}
	if token == "" {
		return nil, utils.HumaUnauthorized("缺少认证 Token")
	}

	req := input.Body
	agent, err := c.agentService.Heartbeat(token, ip, req.Version, req.BuildTime, req.Hostname, req.OS, req.Arch)
	if err != nil {
		return nil, utils.HumaUnauthorized(err.Error())
	}

	// 检查是否需要更新
	latestVersion := c.agentService.GetLatestVersion()
	needUpdate := c.agentService.CheckNeedUpdate(req.Version, req.BuildTime)
	forceUpdate := agent.ForceUpdate

	// 如果强制更新已触发，重置标志
	if forceUpdate && needUpdate {
		c.agentService.ClearForceUpdate(agent.ID)
	}

	return &AgentHeartbeatHumaOutput{
		Body: utils.HumaResponse[struct {
			AgentID       string `json:"agent_id"`
			Name          string `json:"name"`
			NeedUpdate    bool   `json:"need_update"`
			ForceUpdate   bool   `json:"force_update"`
			LatestVersion string `json:"latest_version"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				AgentID       string `json:"agent_id"`
				Name          string `json:"name"`
				NeedUpdate    bool   `json:"need_update"`
				ForceUpdate   bool   `json:"force_update"`
				LatestVersion string `json:"latest_version"`
			}{
				AgentID:       agent.ID,
				Name:          agent.Name,
				NeedUpdate:    needUpdate,
				ForceUpdate:   forceUpdate,
				LatestVersion: latestVersion,
			},
		},
	}, nil
}

// AgentGetTasksHumaOutput Agent 获取任务列表结果
type AgentGetTasksHumaOutput struct {
	Body utils.HumaResponse[struct {
		AgentID string             `json:"agent_id"`
		Tasks   []models.AgentTask `json:"tasks"`
	}]
}

// AgentGetTasksHuma Agent 获取任务列表
func (c *AgentController) AgentGetTasksHuma(ctx context.Context, input *struct{}) (*AgentGetTasksHumaOutput, error) {
	gc := utils.GetGinContext(ctx)
	var token string
	if gc != nil {
		token = c.getAgentToken(gc)
	}
	if token == "" {
		return nil, utils.HumaUnauthorized("缺少认证 Token")
	}

	// 先尝试通过 token 查找 Agent
	agent := c.agentService.GetByToken(token)

	// 如果找不到，尝试验证令牌并通过 machine_id 查找
	if agent == nil && gc != nil {
		machineID := gc.GetHeader("X-Machine-ID")
		if machineID != "" {
			// 验证令牌是否有效
			if _, err := c.agentService.ValidateToken(token); err == nil {
				// 令牌有效，尝试通过 machine_id 查找 Agent
				agent = c.agentService.GetByMachineID(machineID)
			}
		}
	}

	if agent == nil {
		return nil, utils.HumaUnauthorized("无效的 Token")
	}

	if !utils.DerefBool(agent.Enabled, true) {
		return nil, utils.HumaForbidden("Agent 已禁用")
	}

	tasks := c.agentService.GetTasks(agent.ID)
	return &AgentGetTasksHumaOutput{
		Body: utils.HumaResponse[struct {
			AgentID string             `json:"agent_id"`
			Tasks   []models.AgentTask `json:"tasks"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				AgentID string             `json:"agent_id"`
				Tasks   []models.AgentTask `json:"tasks"`
			}{
				AgentID: agent.ID,
				Tasks:   tasks,
			},
		},
	}, nil
}

// AgentReportResultHumaInput Agent 上报执行结果请求
type AgentReportResultHumaInput struct {
	Body models.AgentTaskResult
}

// AgentReportResultHumaOutput Agent 上报执行结果结果
type AgentReportResultHumaOutput struct {
	Body utils.HumaResponse[any]
}

// AgentReportResultHuma Agent 上报执行结果
func (c *AgentController) AgentReportResultHuma(ctx context.Context, input *AgentReportResultHumaInput) (*AgentReportResultHumaOutput, error) {
	gc := utils.GetGinContext(ctx)
	var token string
	if gc != nil {
		token = c.getAgentToken(gc)
	}
	if token == "" {
		return nil, utils.HumaUnauthorized("缺少认证 Token")
	}

	agent := c.agentService.GetByToken(token)
	if agent == nil {
		return nil, utils.HumaUnauthorized("无效的 Token")
	}

	if !utils.DerefBool(agent.Enabled, true) {
		return nil, utils.HumaForbidden("Agent 已禁用")
	}

	result := input.Body
	result.AgentID = agent.ID

	if err := c.agentService.ReportResult(&result); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &AgentReportResultHumaOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "上报成功",
		},
	}, nil
}
