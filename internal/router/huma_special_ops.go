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
			Path:        "/api/v1/logs/sse",
			OperationID: "streamLogs",
			Tags:        []string{"日志"},
			Summary:     "实时日志流（SSE）",
			Description: "通过 Server-Sent Events 实时推送任务执行日志。连接建立后持续推送 `message` 事件，内容为 `{code, data, msg}`。客户端断开或出错时连接自动关闭。",
		},
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
			Path:        "/api/v1/monitor/sse",
			OperationID: "monitorSSE",
			Tags:        []string{"监控"},
			Summary:     "系统监控数据流（SSE）",
			Description: "通过 Server-Sent Events 每 5 秒推送一次系统监控数据（CPU/内存/磁盘/调度器状态）。事件格式为 `{code, data, msg}`。",
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
			Path:        "/api/v1/files/download",
			OperationID: "downloadFile",
			Tags:        []string{"文件管理"},
			Summary:     "下载单个文件",
			Description: "按 `path` 查询参数下载工作目录内的单个文件。",
		},
		{
			Method:      "GET",
			Path:        "/api/v1/files/download-zip",
			OperationID: "downloadFilesAsZip",
			Tags:        []string{"文件管理"},
			Summary:     "批量下载为 Zip",
			Description: "按 `path` 查询参数（可重复）批量下载多个文件/目录，打包为 Zip 流。",
		},
		{
			Method:      "POST",
			Path:        "/api/v1/files/upload",
			OperationID: "uploadArchive",
			Tags:        []string{"文件管理"},
			Summary:     "上传并解压归档",
			Description: "以 multipart/form-data 上传 zip/tar/gz/tgz 归档文件，并按 `path` 表单字段指定目录解压。",
		},
		{
			Method:      "POST",
			Path:        "/api/v1/files/uploadfiles",
			OperationID: "uploadFiles",
			Tags:        []string{"文件管理"},
			Summary:     "上传多个文件",
			Description: "以 multipart/form-data 上传多个文件（字段名 `files`，可选 `paths` 相对路径数组），支持保持文件夹结构。",
		},
		{
			Method:      "GET",
			Path:        "/api/v1/settings/backup/download",
			OperationID: "downloadBackup",
			Tags:        []string{"系统设置"},
			Summary:     "下载系统备份",
			Description: "下载最近一次生成的系统备份 Zip 文件，下载完成后自动清除备份记录与文件。",
		},
		{
			Method:      "POST",
			Path:        "/api/v1/settings/restore",
			OperationID: "restoreBackup",
			Tags:        []string{"系统设置"},
			Summary:     "恢复系统备份",
			Description: "以 multipart/form-data 上传备份 Zip 文件并执行恢复。",
		},
		{
			Method:      "GET",
			Path:        "/api/v1/agent/download",
			OperationID: "downloadAgent",
			Tags:        []string{"Agent"},
			Summary:     "下载 Agent 程序",
			Description: "按 `os`（linux/darwin/windows）与 `arch`（amd64/arm64）查询参数下载对应平台的 Agent 二进制包。",
		},
		{
			Method:      "POST",
			Path:        "/api/v1/webui/upload",
			OperationID: "uploadWebUI",
			Tags:        []string{"系统设置"},
			Summary:     "上传自定义 WebUI",
			Description: "以 multipart/form-data 上传 WebUI 压缩包并解压部署。",
		},
		{
			Method:      "POST",
			Path:        "/api/v1/notify/send",
			OperationID: "sendNotification",
			Tags:        []string{"通知"},
			Summary:     "发送通知",
			Description: "按 `channel_id` 与 `title` 发送一条通知（供脚本调用）。",
		},
		// /interconnect/proxy 在 Gin 中注册为 Any 方法（支持任意 HTTP 方法），
		// 而 Huma 的 OpenAPI 仅接受标准 HTTP 方法，因此展开为多个具体方法注册。
		{
			Method:      "GET",
			Path:        "/api/v1/interconnect/proxy/{node_id}/{path}",
			OperationID: "proxyNodeRequestGet",
			Tags:        []string{"互联互通"},
			Summary:     "节点请求代理",
			Description: "将请求代理转发至指定互联节点（`tunnel://` 走 Yamux 隧道，否则 HTTP 直连）。",
		},
		{
			Method:      "POST",
			Path:        "/api/v1/interconnect/proxy/{node_id}/{path}",
			OperationID: "proxyNodeRequestPost",
			Tags:        []string{"互联互通"},
			Summary:     "节点请求代理",
			Description: "将请求代理转发至指定互联节点（`tunnel://` 走 Yamux 隧道，否则 HTTP 直连）。",
		},
		{
			Method:      "PUT",
			Path:        "/api/v1/interconnect/proxy/{node_id}/{path}",
			OperationID: "proxyNodeRequestPut",
			Tags:        []string{"互联互通"},
			Summary:     "节点请求代理",
			Description: "将请求代理转发至指定互联节点（`tunnel://` 走 Yamux 隧道，否则 HTTP 直连）。",
		},
		{
			Method:      "DELETE",
			Path:        "/api/v1/interconnect/proxy/{node_id}/{path}",
			OperationID: "proxyNodeRequestDelete",
			Tags:        []string{"互联互通"},
			Summary:     "节点请求代理",
			Description: "将请求代理转发至指定互联节点（`tunnel://` 走 Yamux 隧道，否则 HTTP 直连）。",
		},
		{
			Method:      "PATCH",
			Path:        "/api/v1/interconnect/proxy/{node_id}/{path}",
			OperationID: "proxyNodeRequestPatch",
			Tags:        []string{"互联互通"},
			Summary:     "节点请求代理",
			Description: "将请求代理转发至指定互联节点（`tunnel://` 走 Yamux 隧道，否则 HTTP 直连）。",
		},
		{
			Method:      "GET",
			Path:        "/api/v1/interconnect/tunnel",
			OperationID: "interconnectTunnel",
			Tags:        []string{"互联互通"},
			Summary:     "子节点隧道连接（WebSocket）",
			Description: "接受子节点发起的 WebSocket 逆向隧道连接。",
		},
		{
			Method:      "POST",
			Path:        "/api/v1/interconnect/report",
			OperationID: "reportNodeMetrics",
			Tags:        []string{"互联互通"},
			Summary:     "子节点监控数据上报",
			Description: "子节点通过 `Authorization: Bearer <token>` 上报系统监控数据。",
		},
	}

	for _, op := range ops {
		doc.AddOperation(op)
	}
}
