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

	"github.com/danielgtaylor/huma/v2"
)

// ===========================================================================
// /api/v1 管理接口 —— 互联互通
// ===========================================================================

// TAInterconnectNodeBody 互联节点参数
type TAInterconnectNodeBody struct {
	Name   string `json:"name" description:"节点名称"`
	URL    string `json:"url" description:"节点地址"`
	Token  string `json:"token" description:"节点 Token"`
	Remark string `json:"remark" description:"备注"`
}

// TAGetNodesOutput 获取互联节点列表
type TAGetNodesOutput struct {
	Body utils.HumaResponse[[]*models.InterconnectNode]
}

// TAGetNodes 获取互联节点列表
func (ic *InterconnectController) TAGetNodes(ctx context.Context, input *struct{}) (*TAGetNodesOutput, error) {
	nodes, err := ic.interconnectService.GetNodes()
	if err != nil {
		return nil, utils.HumaServerError("获取互联节点失败")
	}

	return &TAGetNodesOutput{
		Body: utils.HumaResponse[[]*models.InterconnectNode]{
			Code: 200,
			Msg:  "success",
			Data: nodes,
		},
	}, nil
}

// TACreateNodeInput 创建互联节点
type TACreateNodeInput struct {
	Body TAInterconnectNodeBody
}

// TACreateNodeOutput 创建互联节点
type TACreateNodeOutput struct {
	Body utils.HumaResponse[*models.InterconnectNode]
}

// TACreateNode 创建互联节点
func (ic *InterconnectController) TACreateNode(ctx context.Context, input *TACreateNodeInput) (*TACreateNodeOutput, error) {
	req := input.Body
	if req.Name == "" || req.Token == "" {
		return nil, utils.HumaBadRequest("参数错误")
	}

	node, err := ic.interconnectService.CreateNode(req.Name, req.URL, req.Token, req.Remark)
	if err != nil {
		return nil, utils.HumaServerError("创建互联节点失败")
	}

	return &TACreateNodeOutput{
		Body: utils.HumaResponse[*models.InterconnectNode]{
			Code: 200,
			Msg:  "success",
			Data: node,
		},
	}, nil
}

// TAUpdateNodeInput 更新互联节点
type TAUpdateNodeInput struct {
	ID   string `path:"id" description:"节点ID"`
	Body TAInterconnectNodeBody
}

// TAUpdateNodeOutput 更新互联节点
type TAUpdateNodeOutput struct {
	Body utils.HumaResponse[*models.InterconnectNode]
}

// TAUpdateNode 更新互联节点
func (ic *InterconnectController) TAUpdateNode(ctx context.Context, input *TAUpdateNodeInput) (*TAUpdateNodeOutput, error) {
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

	return &TAUpdateNodeOutput{
		Body: utils.HumaResponse[*models.InterconnectNode]{
			Code: 200,
			Msg:  "success",
			Data: node,
		},
	}, nil
}

// TADeleteNodeInput 删除互联节点
type TADeleteNodeInput struct {
	ID string `path:"id" description:"节点ID"`
}

// TADeleteNodeOutput 删除互联节点
type TADeleteNodeOutput struct {
	Body utils.HumaResponse[any]
}

// TADeleteNode 删除互联节点
func (ic *InterconnectController) TADeleteNode(ctx context.Context, input *TADeleteNodeInput) (*TADeleteNodeOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的节点ID")
	}

	err := ic.interconnectService.DeleteNode(input.ID)
	if err != nil {
		return nil, utils.HumaServerError("删除互联节点失败")
	}

	return &TADeleteNodeOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "success",
		},
	}, nil
}

// TAGetNodeStatusInput 获取单个子节点状态
type TAGetNodeStatusInput struct {
	ID string `path:"id" description:"节点ID"`
}

// TAGetNodeStatusOutput 获取单个子节点状态
type TAGetNodeStatusOutput struct {
	Body utils.HumaResponse[interface{}]
}

// TAGetNodeStatus 获取单个子节点的状态
func (ic *InterconnectController) TAGetNodeStatus(ctx context.Context, input *TAGetNodeStatusInput) (*TAGetNodeStatusOutput, error) {
	id := input.ID
	node, err := ic.interconnectService.GetNodeByID(id)
	if err != nil {
		return nil, utils.HumaNotFound("节点不存在")
	}

	// 针对反向隧道节点状态检测的特判
	if strings.HasPrefix(node.URL, "tunnel://") {
		sess := tunnel.GetSession(node.ID)
		if sess == nil {
			return &TAGetNodeStatusOutput{
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
			return &TAGetNodeStatusOutput{
				Body: utils.HumaResponse[interface{}]{
					Code: 500,
					Msg:  "与子节点逆向连接通讯失败",
				},
			}, nil
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			return &TAGetNodeStatusOutput{
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

		return &TAGetNodeStatusOutput{
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
		return &TAGetNodeStatusOutput{
			Body: utils.HumaResponse[interface{}]{
				Code: 500,
				Msg:  "节点离线或网络不可达",
			},
		}, nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return &TAGetNodeStatusOutput{
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

	return &TAGetNodeStatusOutput{
		Body: utils.HumaResponse[interface{}]{
			Code: 200,
			Msg:  "success",
			Data: jsonResp["data"],
		},
	}, nil
}

// TASyncScriptInput 同步脚本
type TASyncScriptInput struct {
	Body struct {
		NodeIDs  []string `json:"node_ids" description:"节点 ID 列表"`
		Filename string   `json:"filename" description:"脚本文件名"`
		Content  string   `json:"content" description:"脚本内容"`
	}
}

// TASyncScriptOutput 同步脚本
type TASyncScriptOutput struct {
	Body utils.HumaResponse[[]map[string]interface{}]
}

// TASyncScript 将脚本同步到指定的节点列表
func (ic *InterconnectController) TASyncScript(ctx context.Context, input *TASyncScriptInput) (*TASyncScriptOutput, error) {
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

	return &TASyncScriptOutput{
		Body: utils.HumaResponse[[]map[string]interface{}]{
			Code: 200,
			Msg:  "success",
			Data: results,
		},
	}, nil
}

// TASyncEnvInput 同步环境变量
type TASyncEnvInput struct {
	Body struct {
		NodeIDs []string `json:"node_ids" description:"节点 ID 列表"`
		Envs    []struct {
			ID string `json:"id" description:"环境变量 ID"`
		} `json:"envs" description:"环境变量列表"`
	}
}

// TASyncEnvOutput 同步环境变量
type TASyncEnvOutput struct {
	Body utils.HumaResponse[[]map[string]interface{}]
}

// TASyncEnv 将环境变量同步到指定的节点列表
func (ic *InterconnectController) TASyncEnv(ctx context.Context, input *TASyncEnvInput) (*TASyncEnvOutput, error) {
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

	return &TASyncEnvOutput{
		Body: utils.HumaResponse[[]map[string]interface{}]{
			Code: 200,
			Msg:  "success",
			Data: results,
		},
	}, nil
}

// TASyncTaskInput 同步任务
type TASyncTaskInput struct {
	Body struct {
		NodeIDs []string `json:"node_ids" description:"节点 ID 列表"`
		Tasks   []struct {
			ID string `json:"id" description:"任务 ID"`
		} `json:"tasks" description:"任务列表"`
	}
}

// TASyncTaskOutput 同步任务
type TASyncTaskOutput struct {
	Body utils.HumaResponse[[]map[string]interface{}]
}

// TASyncTask 将任务同步到指定的节点列表
func (ic *InterconnectController) TASyncTask(ctx context.Context, input *TASyncTaskInput) (*TASyncTaskOutput, error) {
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

	return &TASyncTaskOutput{
		Body: utils.HumaResponse[[]map[string]interface{}]{
			Code: 200,
			Msg:  "success",
			Data: results,
		},
	}, nil
}

// TAGetChildStatusOutput 获取本机作为子节点的连接状态
type TAGetChildStatusOutput struct {
	Body utils.HumaResponse[struct {
		ParentURL   string `json:"parent_url"`
		ParentToken string `json:"parent_token"`
		Connected   bool   `json:"connected"`
		TunnelURL   string `json:"tunnel_url"`
		TxBytes     uint64 `json:"tx_bytes"`
		RxBytes     uint64 `json:"rx_bytes"`
	}]
}

// TAGetChildStatus 获取本机作为子节点的连接状态
func (ic *InterconnectController) TAGetChildStatus(ctx context.Context, input *struct{}) (*TAGetChildStatusOutput, error) {
	settingsSvc := services.NewSettingsService()
	parentURL := settingsSvc.Get(constant.SectionInterconnect, constant.KeyInterconnectParentURL)
	parentToken := settingsSvc.Get(constant.SectionInterconnect, constant.KeyInterconnectParentToken)

	connected := tunnel.IsTunnelConnected()
	tunnelURL := tunnel.GetLocalTunnelURL()

	return &TAGetChildStatusOutput{
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

// RegisterAPIInterconnectRoutes 注册 /api/v1 互联互通 Huma 路由
func (ic *InterconnectController) RegisterAPIInterconnectRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/interconnect/nodes",
		OperationID: "apiInterconnectGetNodes",
		Summary:     "获取互联节点列表",
		Description: "获取互联节点列表",
		Tags:        []string{"互联互通"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ic.TAGetNodes)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/interconnect/nodes",
		OperationID: "apiInterconnectCreateNode",
		Summary:     "创建互联节点",
		Description: "创建互联节点",
		Tags:        []string{"互联互通"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ic.TACreateNode)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPut,
		Path:        "/interconnect/nodes/{id}",
		OperationID: "apiInterconnectUpdateNode",
		Summary:     "更新互联节点",
		Description: "更新互联节点",
		Tags:        []string{"互联互通"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ic.TAUpdateNode)

	huma.Register(api, huma.Operation{
		Method:      http.MethodDelete,
		Path:        "/interconnect/nodes/{id}",
		OperationID: "apiInterconnectDeleteNode",
		Summary:     "删除互联节点",
		Description: "删除互联节点",
		Tags:        []string{"互联互通"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ic.TADeleteNode)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/interconnect/nodes/{id}/status",
		OperationID: "apiInterconnectGetNodeStatus",
		Summary:     "获取子节点状态",
		Description: "获取单个子节点的状态",
		Tags:        []string{"互联互通"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ic.TAGetNodeStatus)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/interconnect/sync/script",
		OperationID: "apiInterconnectSyncScript",
		Summary:     "同步脚本",
		Description: "将脚本同步到指定的节点列表",
		Tags:        []string{"互联互通"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ic.TASyncScript)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/interconnect/sync/env",
		OperationID: "apiInterconnectSyncEnv",
		Summary:     "同步环境变量",
		Description: "将环境变量同步到指定的节点列表",
		Tags:        []string{"互联互通"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ic.TASyncEnv)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/interconnect/sync/task",
		OperationID: "apiInterconnectSyncTask",
		Summary:     "同步任务",
		Description: "将任务同步到指定的节点列表",
		Tags:        []string{"互联互通"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ic.TASyncTask)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/interconnect/child/status",
		OperationID: "apiInterconnectGetChildStatus",
		Summary:     "获取本机子节点连接状态",
		Description: "获取本机作为子节点的连接状态",
		Tags:        []string{"互联互通"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ic.TAGetChildStatus)
}
