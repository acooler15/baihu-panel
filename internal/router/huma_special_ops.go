package router

import (
	"github.com/danielgtaylor/huma/v2"
)

// registerSpecialOperations 为保留 Gin 原生的特殊接口（WebSocket / SSE / 文件流 / 代理等）
// 手动补充 OpenAPI Operation 描述，使导出文档（/api/v1/openapi.json）覆盖全部公开接口。
//
// 注意：这些接口仍由 api_routes.go 中的 Gin 原生路由处理，此处仅补充文档，
// 不会重复注册实际路由。
func registerSpecialOperations(api huma.API) {
	if api == nil {
		return
	}
	doc := api.OpenAPI()
	if doc == nil {
		return
	}

	ops := []*huma.Operation{
		{
			Method:      "GET",
			Path:        "/api/v1/terminal/ws",
			OperationID: "openTerminal",
			Tags:        []string{"终端"},
			Summary:     "WebSocket 终端",
			Description: "建立 WebSocket 连接进入交互式终端。连接建立后服务端先发送 `__PTY_MODE__`（Unix/ConPTY）或 `__PIPE_MODE__`（Windows Pipe）标识。客户端可发送 `{\"type\":\"resize\",\"rows\":N,\"cols\":N}` 调整窗口大小，其余消息透传到 shell 进程。",
		},
		{
			Method:      "GET",
			Path:        "/api/v1/ws/events",
			OperationID: "systemEvents",
			Tags:        []string{"系统"},
			Summary:     "系统事件流（WebSocket）",
			Description: "建立 WebSocket 连接实时接收系统事件推送（任务执行结果、日志等）。",
		},
		{
			Method:      "GET",
			Path:        "/api/v1/agent/ws",
			OperationID: "agentWebSocket",
			Tags:        []string{"Agent"},
			Summary:     "Agent WebSocket 连接",
			Description: "远程 Agent 建立 WebSocket 长连接，用于任务下发、结果上报与实时日志推送。连接需要 `token` 与 `machine_id` 查询参数。",
		},
		{
			Method:      "GET",
			Path:        "/api/v1/interconnect/tunnel",
			OperationID: "interconnectTunnel",
			Tags:        []string{"互联互通"},
			Summary:     "子节点隧道连接（WebSocket）",
			Description: "接受子节点发起的 WebSocket 逆向隧道连接。",
		},
	}

	for _, op := range ops {
		doc.AddOperation(op)
	}
}
