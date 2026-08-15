package utils

import (
	"context"

	"github.com/gin-gonic/gin"
)

// ginContextKey 用于在 context.Context 中存放 *gin.Context
type ginContextKey struct{}

// GetGinContext 从 Huma handler 的 ctx 中提取 *gin.Context，
// 以便读取 gin 中间件写入的值（如 userID）。
// 若 ctx 中不存在则返回 nil，调用方需自行判空。
func GetGinContext(ctx context.Context) *gin.Context {
	if c, ok := ctx.Value(ginContextKey{}).(*gin.Context); ok {
		return c
	}
	return nil
}

// InjectGinContext 将 *gin.Context 注入到请求的 context 中，供 Huma handler 使用。
func InjectGinContext(c *gin.Context) {
	ctx := context.WithValue(c.Request.Context(), ginContextKey{}, c)
	c.Request = c.Request.WithContext(ctx)
}
