package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/tunnel"
	"github.com/engigu/baihu-panel/internal/utils"
	"github.com/gin-gonic/gin"

	"github.com/danielgtaylor/huma/v2"
)

type InterconnectController struct {
	interconnectService *services.InterconnectService
	httpClient          *http.Client
}

func NewInterconnectController(interconnectService *services.InterconnectService) *InterconnectController {
	return &InterconnectController{
		interconnectService: interconnectService,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ===========================================================================
// Gin 原生 handler（由 api_routes.go 保留引用）
// ===========================================================================

// HandleTunnel 接受子节点 WebSocket 连接请求
func (ic *InterconnectController) HandleTunnel(c *gin.Context) {
	tunnel.HandleTunnel(c)
}

// ProxyRequest 代理转发请求至目标节点
func (ic *InterconnectController) ProxyRequest(c *gin.Context) {
	nodeID := c.Param("node_id")
	path := c.Param("path")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, utils.Response{Code: 400, Msg: "Node ID required"})
		return
	}

	node, err := ic.interconnectService.GetNodeByID(nodeID)
	if err != nil {
		c.JSON(http.StatusNotFound, utils.Response{Code: 404, Msg: "Node not found"})
		return
	}

	if strings.HasPrefix(node.URL, "tunnel://") {
		// 走 WebSocket 逆向隧道 (基于 Yamux 流式多路复用)
		err := tunnel.ProxyHTTP(nodeID, c, path)
		if err != nil {
			c.JSON(http.StatusInternalServerError, utils.Response{Code: 500, Msg: "Tunnel request failed: " + err.Error()})
		}
		return
	}

	// 走普通 HTTP 直连
	// Construct the target URL
	targetURL := strings.TrimRight(node.URL, "/") + path
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}

	req, err := http.NewRequest(c.Request.Method, targetURL, c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.Response{Code: 500, Msg: "Failed to create proxy request"})
		return
	}

	// Copy headers
	req.Header = c.Request.Header.Clone()

	// If the node token exists, append it as Bearer Auth
	if node.Token != "" {
		req.Header.Set("Authorization", "Bearer "+node.Token)
	}

	resp, err := ic.httpClient.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.Response{Code: 500, Msg: "Failed to connect to target node: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		for _, vv := range v {
			c.Writer.Header().Add(k, vv)
		}
	}
	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}

// getClientAndURL 辅助方法：根据节点类型决定走直连还是隧道，并返回对应的 Client 和完整 URL
func (ic *InterconnectController) getClientAndURL(node *models.InterconnectNode, path string) (*http.Client, string, error) {
	if strings.HasPrefix(node.URL, "tunnel://") {
		sess := tunnel.GetSession(node.ID)
		if sess == nil {
			return nil, "", net.ErrClosed
		}
		transport := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return sess.Session.Open()
			},
		}
		client := &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		}
		return client, "http://tunnel.local" + path, nil
	}

	targetURL := strings.TrimRight(node.URL, "/") + path
	return ic.httpClient, targetURL, nil
}

// ===========================================================================
// 互联互通业务方法（Huma）
// ===========================================================================

// InterconnectNodeBody 互联节点参数
type InterconnectNodeBody struct {
	Name   string `json:"name" description:"节点名称"`
	URL    string `json:"url" description:"节点地址"`
	Token  string `json:"token" description:"节点 Token"`
	Remark string `json:"remark" description:"备注"`
}

// GetNodesOutput 获取互联节点列表
type GetNodesOutput struct {
	Body utils.HumaResponse[[]*models.InterconnectNode]
}

// GetNodes 获取互联节点列表
func (ic *InterconnectController) GetNodes(ctx context.Context, input *struct{}) (*GetNodesOutput, error) {
	nodes, err := ic.interconnectService.GetNodes()
	if err != nil {
		return nil, utils.HumaServerError("获取互联节点失败")
	}

	return &GetNodesOutput{
		Body: utils.HumaResponse[[]*models.InterconnectNode]{
			Code: 200,
			Msg:  "success",
			Data: nodes,
		},
	}, nil
}

// CreateNodeInput 创建互联节点
type CreateNodeInput struct {
	Body InterconnectNodeBody
}

// CreateNodeOutput 创建互联节点
type CreateNodeOutput struct {
	Body utils.HumaResponse[*models.InterconnectNode]
}

// CreateNode 创建互联节点
func (ic *InterconnectController) CreateNode(ctx context.Context, input *CreateNodeInput) (*CreateNodeOutput, error) {
	req := input.Body
	if req.Name == "" || req.Token == "" {
		return nil, utils.HumaBadRequest("参数错误")
	}

	node, err := ic.interconnectService.CreateNode(req.Name, req.URL, req.Token, req.Remark)
	if err != nil {
		return nil, utils.HumaServerError("创建互联节点失败")
	}

	return &CreateNodeOutput{
		Body: utils.HumaResponse[*models.InterconnectNode]{
			Code: 200,
			Msg:  "success",
			Data: node,
		},
	}, nil
}

// UpdateNodeInput 更新互联节点
type UpdateNodeInput struct {
	ID   string `path:"id" description:"节点ID"`
	Body InterconnectNodeBody
}

// UpdateNodeOutput 更新互联节点
type UpdateNodeOutput struct {
	Body utils.HumaResponse[*models.InterconnectNode]
}

// UpdateNode 更新互联节点
func (ic *InterconnectController) UpdateNode(ctx context.Context, input *UpdateNodeInput) (*UpdateNodeOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的节点ID")
	}

	req := input.Body
	if req.Name == "" || req.Token == "" {
		return nil, utils.HumaBadRequest("参数错误")
	}

	node, err := ic.interconnectService.UpdateNode(input.ID, req.Name, req.URL, req.Token, req.Remark)
	if err != nil {
		return nil, utils.HumaServerError("更新互联节点失败")
	}

	return &UpdateNodeOutput{
		Body: utils.HumaResponse[*models.InterconnectNode]{
			Code: 200,
			Msg:  "success",
			Data: node,
		},
	}, nil
}

// DeleteNodeInput 删除互联节点
type DeleteNodeInput struct {
	ID string `path:"id" description:"节点ID"`
}

// DeleteNodeOutput 删除互联节点
type DeleteNodeOutput struct {
	Body utils.HumaResponse[any]
}

// DeleteNode 删除互联节点
func (ic *InterconnectController) DeleteNode(ctx context.Context, input *DeleteNodeInput) (*DeleteNodeOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的节点ID")
	}

	err := ic.interconnectService.DeleteNode(input.ID)
	if err != nil {
		return nil, utils.HumaServerError("删除互联节点失败")
	}

	return &DeleteNodeOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "success",
		},
	}, nil
}

// GetNodeStatusInput 获取单个子节点状态
type GetNodeStatusInput struct {
	ID string `path:"id" description:"节点ID"`
}

// GetNodeStatusOutput 获取单个子节点状态
type GetNodeStatusOutput struct {
	Body utils.HumaResponse[interface{}]
}

// GetNodeStatus 获取单个子节点的状态
func (ic *InterconnectController) GetNodeStatus(ctx context.Context, input *GetNodeStatusInput) (*GetNodeStatusOutput, error) {
	id := input.ID
	node, err := ic.interconnectService.GetNodeByID(id)
	if err != nil {
		return nil, utils.HumaNotFound("节点不存在")
	}

	// 针对反向隧道节点状态检测的特判
	if strings.HasPrefix(node.URL, "tunnel://") {
		sess := tunnel.GetSession(node.ID)
		if sess == nil {
			return &GetNodeStatusOutput{
				Body: utils.HumaResponse[interface{}]{
					Code: 500,
					Msg:  "节点离线或反向隧道未建立",
				},
			}, nil
		}

		transport := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return sess.Session.Open()
			},
		}
		client := &http.Client{
			Transport: transport,
			Timeout:   5 * time.Second,
		}

		req, err := http.NewRequest("GET", "http://tunnel.local/api/v1/monitor", nil)
		if err != nil {
			return nil, utils.HumaServerError("构建检测请求失败")
		}
		req.Header.Set("Authorization", "Bearer "+node.Token)

		resp, err := client.Do(req)
		if err != nil {
			return &GetNodeStatusOutput{
				Body: utils.HumaResponse[interface{}]{
					Code: 500,
					Msg:  "与子节点逆向连接通讯失败",
				},
			}, nil
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			return &GetNodeStatusOutput{
				Body: utils.HumaResponse[interface{}]{
					Code: 500,
					Msg:  "子节点检测异常",
					Data: string(body),
				},
			}, nil
		}

		var jsonResp map[string]interface{}
		if err := json.Unmarshal(body, &jsonResp); err != nil {
			return nil, utils.HumaServerError("解析节点检测数据失败")
		}

		if dataMap, ok := jsonResp["data"].(map[string]interface{}); ok {
			dataMap["tunnel_connected"] = true
			dataMap["tunnel_url"] = node.URL
			if hostMap, ok := dataMap["host"].(map[string]interface{}); ok {
				hostMap["tx_bytes"] = node.Metrics.TxBytes
				hostMap["rx_bytes"] = node.Metrics.RxBytes
			}
		}

		return &GetNodeStatusOutput{
			Body: utils.HumaResponse[interface{}]{
				Code: 200,
				Msg:  "success",
				Data: jsonResp["data"],
			},
		}, nil
	}

	apiURL := strings.TrimRight(node.URL, "/") + "/api/v1/monitor"
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, utils.HumaServerError("构建请求失败")
	}
	req.Header.Set("Authorization", "Bearer "+node.Token)

	resp, err := ic.httpClient.Do(req)
	if err != nil {
		return &GetNodeStatusOutput{
			Body: utils.HumaResponse[interface{}]{
				Code: 500,
				Msg:  "节点离线或网络不可达",
			},
		}, nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return &GetNodeStatusOutput{
			Body: utils.HumaResponse[interface{}]{
				Code: 500,
				Msg:  "节点返回异常",
				Data: string(body),
			},
		}, nil
	}

	var jsonResp map[string]interface{}
	if err := json.Unmarshal(body, &jsonResp); err != nil {
		return nil, utils.HumaServerError("解析节点响应失败")
	}

	return &GetNodeStatusOutput{
		Body: utils.HumaResponse[interface{}]{
			Code: 200,
			Msg:  "success",
			Data: jsonResp["data"],
		},
	}, nil
}

// SyncScriptInput 同步脚本
type SyncScriptInput struct {
	Body struct {
		NodeIDs  []string `json:"node_ids" description:"节点 ID 列表"`
		Filename string   `json:"filename" description:"脚本文件名"`
		Content  string   `json:"content" description:"脚本内容"`
	}
}

// SyncScriptOutput 同步脚本
type SyncScriptOutput struct {
	Body utils.HumaResponse[[]map[string]interface{}]
}

// SyncScript 将脚本同步到指定的节点列表
func (ic *InterconnectController) SyncScript(ctx context.Context, input *SyncScriptInput) (*SyncScriptOutput, error) {
	req := input.Body

	results := make([]map[string]interface{}, 0)

	for _, nodeID := range req.NodeIDs {
		node, err := ic.interconnectService.GetNodeByID(nodeID)
		if err != nil {
			results = append(results, map[string]interface{}{"node_id": nodeID, "success": false, "msg": "节点不存在"})
			continue
		}

		client, apiURL, err := ic.getClientAndURL(node, "/api/v1/scripts/save")
		if err != nil {
			results = append(results, map[string]interface{}{"node_id": nodeID, "success": false, "msg": "反向隧道未连接"})
			continue
		}

		payload := map[string]interface{}{
			"filename": req.Filename,
			"content":  req.Content,
		}
		payloadBytes, _ := json.Marshal(payload)

		httpReq, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
		if err != nil {
			results = append(results, map[string]interface{}{"node_id": nodeID, "success": false, "msg": "构建请求失败"})
			continue
		}
		httpReq.Header.Set("Authorization", "Bearer "+node.Token)
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(httpReq)
		if err != nil || resp.StatusCode != 200 {
			results = append(results, map[string]interface{}{"node_id": nodeID, "success": false, "msg": "同步请求失败或超时"})
		} else {
			results = append(results, map[string]interface{}{"node_id": nodeID, "success": true, "msg": "同步成功"})
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	return &SyncScriptOutput{
		Body: utils.HumaResponse[[]map[string]interface{}]{
			Code: 200,
			Msg:  "success",
			Data: results,
		},
	}, nil
}

// SyncEnvInput 同步环境变量
type SyncEnvInput struct {
	Body struct {
		NodeIDs []string `json:"node_ids" description:"节点 ID 列表"`
		Envs    []struct {
			ID string `json:"id" description:"环境变量 ID"`
		} `json:"envs" description:"环境变量列表"`
	}
}

// SyncEnvOutput 同步环境变量
type SyncEnvOutput struct {
	Body utils.HumaResponse[[]map[string]interface{}]
}

// SyncEnv 将环境变量同步到指定的节点列表
func (ic *InterconnectController) SyncEnv(ctx context.Context, input *SyncEnvInput) (*SyncEnvOutput, error) {
	req := input.Body

	var envIDs []string
	for _, e := range req.Envs {
		envIDs = append(envIDs, e.ID)
	}

	dataService := services.NewDataService()
	exportData := dataService.ExportBusinessData(nil, envIDs)

	results := make([]map[string]interface{}, 0)

	for _, nodeID := range req.NodeIDs {
		node, err := ic.interconnectService.GetNodeByID(nodeID)
		if err != nil {
			results = append(results, map[string]interface{}{"node_id": nodeID, "success": false, "msg": "节点不存在"})
			continue
		}

		client, apiURL, err := ic.getClientAndURL(node, "/api/v1/system/import")
		if err != nil {
			results = append(results, map[string]interface{}{"node_id": nodeID, "success": false, "msg": "反向隧道未连接"})
			continue
		}

		payloadBytes, _ := json.Marshal(exportData)

		httpReq, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
		if err != nil {
			results = append(results, map[string]interface{}{"node_id": nodeID, "success": false, "msg": "构建请求失败"})
			continue
		}
		httpReq.Header.Set("Authorization", "Bearer "+node.Token)
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(httpReq)
		if err != nil || resp.StatusCode != 200 {
			msg := "同步失败"
			if err != nil {
				msg = err.Error()
			}
			results = append(results, map[string]interface{}{"node_id": nodeID, "success": false, "msg": msg})
		} else {
			results = append(results, map[string]interface{}{"node_id": nodeID, "success": true, "msg": "同步成功"})
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	return &SyncEnvOutput{
		Body: utils.HumaResponse[[]map[string]interface{}]{
			Code: 200,
			Msg:  "success",
			Data: results,
		},
	}, nil
}

// SyncTaskInput 同步任务
type SyncTaskInput struct {
	Body struct {
		NodeIDs []string `json:"node_ids" description:"节点 ID 列表"`
		Tasks   []struct {
			ID string `json:"id" description:"任务 ID"`
		} `json:"tasks" description:"任务列表"`
	}
}

// SyncTaskOutput 同步任务
type SyncTaskOutput struct {
	Body utils.HumaResponse[[]map[string]interface{}]
}

// SyncTask 将任务同步到指定的节点列表
func (ic *InterconnectController) SyncTask(ctx context.Context, input *SyncTaskInput) (*SyncTaskOutput, error) {
	req := input.Body

	var taskIDs []string
	for _, t := range req.Tasks {
		taskIDs = append(taskIDs, t.ID)
	}

	dataService := services.NewDataService()
	exportData := dataService.ExportBusinessData(taskIDs, nil)

	results := make([]map[string]interface{}, 0)

	for _, nodeID := range req.NodeIDs {
		node, err := ic.interconnectService.GetNodeByID(nodeID)
		if err != nil {
			results = append(results, map[string]interface{}{"node_id": nodeID, "success": false, "msg": "节点不存在"})
			continue
		}

		client, apiURL, err := ic.getClientAndURL(node, "/api/v1/system/import")
		if err != nil {
			results = append(results, map[string]interface{}{"node_id": nodeID, "success": false, "msg": "反向隧道未连接"})
			continue
		}

		payloadBytes, _ := json.Marshal(exportData)

		httpReq, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
		if err != nil {
			results = append(results, map[string]interface{}{"node_id": nodeID, "success": false, "msg": "构建请求失败"})
			continue
		}
		httpReq.Header.Set("Authorization", "Bearer "+node.Token)
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(httpReq)
		if err != nil || resp.StatusCode != 200 {
			msg := "同步失败"
			if err != nil {
				msg = err.Error()
			}
			results = append(results, map[string]interface{}{"node_id": nodeID, "success": false, "msg": msg})
		} else {
			results = append(results, map[string]interface{}{"node_id": nodeID, "success": true, "msg": "同步成功"})
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	return &SyncTaskOutput{
		Body: utils.HumaResponse[[]map[string]interface{}]{
			Code: 200,
			Msg:  "success",
			Data: results,
		},
	}, nil
}

// GetChildStatusOutput 获取本机作为子节点的连接状态
type GetChildStatusOutput struct {
	Body utils.HumaResponse[struct {
		ParentURL   string `json:"parent_url"`
		ParentToken string `json:"parent_token"`
		Connected   bool   `json:"connected"`
		TunnelURL   string `json:"tunnel_url"`
		TxBytes     uint64 `json:"tx_bytes"`
		RxBytes     uint64 `json:"rx_bytes"`
	}]
}

// GetChildStatus 获取本机作为子节点的连接状态
func (ic *InterconnectController) GetChildStatus(ctx context.Context, input *struct{}) (*GetChildStatusOutput, error) {
	settingsSvc := services.NewSettingsService()
	parentURL := settingsSvc.Get(constant.SectionInterconnect, constant.KeyInterconnectParentURL)
	parentToken := settingsSvc.Get(constant.SectionInterconnect, constant.KeyInterconnectParentToken)

	connected := tunnel.IsTunnelConnected()
	tunnelURL := tunnel.GetLocalTunnelURL()

	return &GetChildStatusOutput{
		Body: utils.HumaResponse[struct {
			ParentURL   string `json:"parent_url"`
			ParentToken string `json:"parent_token"`
			Connected   bool   `json:"connected"`
			TunnelURL   string `json:"tunnel_url"`
			TxBytes     uint64 `json:"tx_bytes"`
			RxBytes     uint64 `json:"rx_bytes"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				ParentURL   string `json:"parent_url"`
				ParentToken string `json:"parent_token"`
				Connected   bool   `json:"connected"`
				TunnelURL   string `json:"tunnel_url"`
				TxBytes     uint64 `json:"tx_bytes"`
				RxBytes     uint64 `json:"rx_bytes"`
			}{
				ParentURL:   parentURL,
				ParentToken: parentToken,
				Connected:   connected,
				TunnelURL:   tunnelURL,
				TxBytes:     tunnel.GetTxBytes(),
				RxBytes:     tunnel.GetRxBytes(),
			},
		},
	}, nil
}

// ProxyRequestHumaInput 节点请求代理输入
type ProxyRequestHumaInput struct {
	NodeID string `path:"node_id" description:"节点 ID"`
	Path   string `path:"path" description:"代理路径"`
}

// ProxyRequestHumaOutput 节点请求代理输出（实际响应由 ProxyRequest 透传，此处为空体）
type ProxyRequestHumaOutput struct {
	Body struct{}
}

// ProxyRequestHuma 节点请求代理（转发至目标节点）。
// OpenAPI 不支持 ANY 方法，因此为 GET/POST/PUT/DELETE/PATCH 各注册一个 Operation，
// 实际处理统一复用 Gin 原生 ProxyRequest 逻辑（通过注入的 *gin.Context）。
// 注意：响应体由 ProxyRequest 直接写入 gin response writer，handler 返回 nil 输出，
// 以避免 huma 再次序列化 body。
func (ic *InterconnectController) ProxyRequestHuma(ctx context.Context, input *ProxyRequestHumaInput) (*ProxyRequestHumaOutput, error) {
	gc := utils.GetGinContext(ctx)
	if gc == nil {
		return nil, utils.HumaServerError("无法获取请求上下文")
	}
	// 代理请求直接透传，无需返回结构化 JSON（由 ProxyRequest 写响应）。
	// gin 路由已注册 :node_id 与 :path 参数，ProxyRequest 通过 c.Param 读取。
	ic.ProxyRequest(gc)
	return nil, nil
}

// ===========================================================================
// 路由注册
// ===========================================================================

// RegisterAPIInterconnectRoutes 注册 /api/v1 互联互通 Huma 路由
func (ic *InterconnectController) RegisterAPIInterconnectRoutes(api huma.API) {
	security := []map[string][]string{{"CookieAuth": {}}}
	tag := []string{"互联互通"}

	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/interconnect/nodes", OperationID: "GetNodes", Summary: "获取互联节点列表", Description: "获取互联节点列表", Tags: tag, Security: security}, ic.GetNodes)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/interconnect/nodes", OperationID: "CreateNode", Summary: "创建互联节点", Description: "创建互联节点", Tags: tag, Security: security}, ic.CreateNode)
	huma.Register(api, huma.Operation{Method: http.MethodPut, Path: "/interconnect/nodes/{id}", OperationID: "UpdateNode", Summary: "更新互联节点", Description: "更新互联节点", Tags: tag, Security: security}, ic.UpdateNode)
	huma.Register(api, huma.Operation{Method: http.MethodDelete, Path: "/interconnect/nodes/{id}", OperationID: "DeleteNode", Summary: "删除互联节点", Description: "删除互联节点", Tags: tag, Security: security}, ic.DeleteNode)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/interconnect/nodes/{id}/status", OperationID: "GetNodeStatus", Summary: "获取子节点状态", Description: "获取单个子节点的状态", Tags: tag, Security: security}, ic.GetNodeStatus)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/interconnect/sync/script", OperationID: "SyncScript", Summary: "同步脚本", Description: "将脚本同步到指定的节点列表", Tags: tag, Security: security}, ic.SyncScript)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/interconnect/sync/env", OperationID: "SyncEnv", Summary: "同步环境变量", Description: "将环境变量同步到指定的节点列表", Tags: tag, Security: security}, ic.SyncEnv)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/interconnect/sync/task", OperationID: "SyncTask", Summary: "同步任务", Description: "将任务同步到指定的节点列表", Tags: tag, Security: security}, ic.SyncTask)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/interconnect/child/status", OperationID: "GetChildStatus", Summary: "获取本机子节点连接状态", Description: "获取本机作为子节点的连接状态", Tags: tag, Security: security}, ic.GetChildStatus)

	// 节点请求代理（OpenAPI 不支持 ANY，展开为 5 个标准方法）
	proxyOps := []struct {
		method      string
		operationID string
	}{
		{http.MethodGet, "ProxyGet"},
		{http.MethodPost, "ProxyPost"},
		{http.MethodPut, "ProxyPut"},
		{http.MethodDelete, "ProxyDelete"},
		{http.MethodPatch, "ProxyPatch"},
	}
	for _, pop := range proxyOps {
		ic.registerProxyRoute(api, pop.method, pop.operationID, security, tag)
	}

	// 子节点监控数据上报（无 CookieAuth，使用内部 Bearer Token，selector 中放行）
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/interconnect/report", OperationID: "ReportMonitorData", Summary: "上报子节点监控数据", Description: "子节点通过 `Authorization: Bearer <token>` 上报系统监控数据。", Tags: tag}, ic.ReportMonitorDataHuma)
}

// ===========================================================================
// 子节点监控数据上报（Huma，迁移自 Gin 原生 ReportMonitorData）
// ===========================================================================

// ReportMonitorDataHumaInput 上报监控数据请求
type ReportMonitorDataHumaInput struct {
	Body models.NodeMetrics
}

// ReportMonitorDataHumaOutput 上报监控数据结果
type ReportMonitorDataHumaOutput struct {
	Body utils.HumaResponse[struct {
		TunnelURL string `json:"tunnel_url,omitempty"`
	}]
}

// ReportMonitorDataHuma 接收子节点上报的监控数据
func (ic *InterconnectController) ReportMonitorDataHuma(ctx context.Context, input *ReportMonitorDataHumaInput) (*ReportMonitorDataHumaOutput, error) {
	req := input.Body

	gc := utils.GetGinContext(ctx)
	authHeader := ""
	if gc != nil {
		authHeader = gc.GetHeader("Authorization")
	}
	if authHeader == "" {
		return nil, utils.HumaUnauthorized("missing authorization")
	}
	tokenStr := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))

	node, err := ic.interconnectService.GetNodeByToken(tokenStr)
	if err != nil {
		return nil, utils.HumaUnauthorized("invalid token")
	}

	if err := ic.interconnectService.UpdateNodeMonitorData(node.ID, req); err != nil {
		return nil, utils.HumaServerError("更新节点数据失败")
	}

	return &ReportMonitorDataHumaOutput{
		Body: utils.HumaResponse[struct {
			TunnelURL string `json:"tunnel_url,omitempty"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				TunnelURL string `json:"tunnel_url,omitempty"`
			}{
				TunnelURL: node.URL,
			},
		},
	}, nil
}

// registerProxyRoute 注册单个方法的代理路由
func (ic *InterconnectController) registerProxyRoute(api huma.API, method, operationID string, security []map[string][]string, tag []string) {
	huma.Register(api, huma.Operation{
		Method:      method,
		Path:        "/interconnect/proxy/{node_id}/{path}",
		OperationID: operationID,
		Summary:     "节点请求代理",
		Description: "将请求代理转发至指定互联节点（tunnel:// 走 Yamux 隧道，否则 HTTP 直连）。",
		Tags:        tag,
		Security:    security,
	}, ic.ProxyRequestHuma)
}
