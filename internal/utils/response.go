package utils

// Response 通用 JSON 响应结构
// 注意：所有接口已迁移至 Huma，此类型仅保留给仍使用 Gin 的特殊接口（WebSocket/SSE/文件流/Auth）做内联 JSON 响应。
type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}
