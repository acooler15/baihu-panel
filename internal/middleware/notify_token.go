package middleware

import (
	"net/http"
	"strings"

	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/utils"
	"github.com/gin-gonic/gin"
)

// NotifyTokenAuth 通知 Token 认证中间件
func NotifyTokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("notify-token")
		if token == "" {
			c.JSON(http.StatusUnauthorized, utils.Response{Code: 401, Msg: "缺少通知 Token"})
			c.Abort()
			return
		}

		// 从 settings 表读取配置的通知 Token
		settingsService := services.NewSettingsService()
		savedToken := settingsService.Get(constant.SectionNotify, constant.KeyNotifyToken)

		if savedToken == "" {
			c.JSON(http.StatusUnauthorized, utils.Response{Code: 401, Msg: "通知 Token 未配置"})
			c.Abort()
			return
		}

		if !strings.EqualFold(token, savedToken) {
			c.JSON(http.StatusUnauthorized, utils.Response{Code: 401, Msg: "通知 Token 无效"})
			c.Abort()
			return
		}

		c.Next()
	}
}
