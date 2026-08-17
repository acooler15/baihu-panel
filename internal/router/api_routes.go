package router

import (
	"github.com/engigu/baihu-panel/internal/middleware"
	"github.com/gin-gonic/gin"
)

func initPublicAPIRoutes(api *gin.RouterGroup, c *Controllers) {
	// Health check (无需认证)
	api.GET("/ping", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{"message": "pong"})
	})

	// api.GET("/debug/goroutines", func(ctx *gin.Context) {
	// 	buf := make([]byte, 1024*1024)
	// 	n := runtime.Stack(buf, true)
	// 	ctx.Data(200, "text/plain; charset=utf-8", buf[:n])
	// })

	// 说明：认证接口（/auth/login、/auth/login/otp、/auth/logout）已迁移至 Huma，
	// 公开站点设置（/settings/public）、子节点上报（/interconnect/report）、
	// 内部接口（/internal/*）、Agent 外部接口（/api/agent/*）均已迁移至 Huma 注册。

	// 隧道模式 (被控端反向连入，使用独立 Token 做 WebSocket 鉴权，保留 Gin 原生处理)
	api.GET("/interconnect/tunnel", c.Interconnect.HandleTunnel)
}

func initAuthorizedAPIRoutes(api *gin.RouterGroup, c *Controllers) {
	authorized := api.Group("")
	authorized.Use(middleware.AuthRequired())
	{
		// 说明：所有非 WebSocket 接口均已迁移至 Huma 注册（见各 controller 的 RegisterAPI*Routes）。
		// 此处仅保留 WebSocket 特殊接口（终端 / 系统事件）。

		// 以下管理接口需要管理员权限
		adminOnly := authorized.Group("")
		adminOnly.Use(middleware.AdminRequired())
		{
			registerTerminalWSRoutes(adminOnly, c)
			registerSystemWSRoutes(adminOnly, c)
		}
	}

}

func registerTerminalWSRoutes(g *gin.RouterGroup, c *Controllers) {
	// 终端 WebSocket 保留 Gin 原生处理
	g.GET("/terminal/ws", c.Terminal.HandleWebSocket)
}

func registerSystemWSRoutes(g *gin.RouterGroup, c *Controllers) {
	g.GET("/ws/events", c.SystemWS.HandleEvents)
}

func initAgentAPIRoutes(root *gin.RouterGroup, c *Controllers) {
	// 说明：/api/agent/heartbeat、/tasks、/report、/download 已迁移至 AgentHuma 实例注册。
	// 此处仅保留 Agent WebSocket 长连接（供任务下发、结果上报与实时日志推送）。
	agentAPI := root.Group("/api/agent")
	{
		agentAPI.GET("/ws", c.Agent.WSConnect) // WebSocket 连接
	}
}
