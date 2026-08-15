package utils

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// HumaResponse 统一响应体，结构与原 utils.Response 一致
type HumaResponse[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data,omitempty"`
}

// HumaError 业务错误，实现 error 接口
type HumaError struct {
	Status int
	Code   int // 业务码，通常等于 Status
	Msg    string
}

func (e *HumaError) Error() string { return e.Msg }

// GetStatus 返回应返回给客户端的 HTTP 状态码。
func (e *HumaError) GetStatus() int {
	if e.Status == 0 {
		return e.Code
	}
	return e.Status
}

func NewHumaError(status int, msg string) *HumaError {
	return &HumaError{Status: status, Code: status, Msg: msg}
}

// HumaTransformer 将 HumaError 转换为 {code, msg} 响应。
//
// Huma v2 的 Transformer 签名：func(ctx Context, status string, v any) (any, error)。
// 当响应体 v 为 *HumaError 时，设置 HTTP 状态码并返回 {code, msg} 结构。
func HumaTransformer(ctx huma.Context, status string, v any) (any, error) {
	if he, ok := v.(*HumaError); ok {
		ctx.SetStatus(he.GetStatus())
		return &Response{
			Code: he.Code,
			Msg:  he.Msg,
		}, nil
	}
	// 默认透传
	return v, nil
}

// 便捷错误构造函数
func HumaBadRequest(msg string) *HumaError      { return NewHumaError(http.StatusBadRequest, msg) }
func HumaUnauthorized(msg string) *HumaError    { return NewHumaError(http.StatusUnauthorized, msg) }
func HumaForbidden(msg string) *HumaError       { return NewHumaError(http.StatusForbidden, msg) }
func HumaNotFound(msg string) *HumaError        { return NewHumaError(http.StatusNotFound, msg) }
func HumaConflict(msg string, data any) *HumaError {
	return NewHumaError(http.StatusConflict, msg)
}
func HumaTooManyRequests(msg string) *HumaError { return NewHumaError(http.StatusTooManyRequests, msg) }
func HumaServerError(msg string) *HumaError     { return NewHumaError(http.StatusInternalServerError, msg) }
