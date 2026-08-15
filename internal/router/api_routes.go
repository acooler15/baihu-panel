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

	// Authentication routes (无需认证)
	auth := api.Group("/auth")
	{
		auth.POST("/login", c.Auth.Login)
		auth.POST("/login/otp", c.Auth.VerifyOTP)
		auth.POST("/logout", c.Auth.Logout)
		// auth.POST("/register", c.Auth.Register)
	}

	// 公开的站点设置（无需认证）
	api.GET("/settings/public", c.Settings.GetPublicSiteSettings)

	// 隧道模式 (被控端反向连入，使用独立 Token 做 WebSocket 鉴权)
	api.GET("/interconnect/tunnel", c.Interconnect.HandleTunnel)
	// 子节点主动上报监控数据 (无中间件鉴权，内部鉴权)
	api.POST("/interconnect/report", c.Interconnect.ReportMonitorData)

	// 内部使用的 API（仅限本地调用，无需 Bearer 认证）
	internalAPI := api.Group("/internal")
	internalAPI.Use(middleware.LocalhostOnly())
	{
		internalAPI.POST("/tasks/sync-repo-status", c.Task.SyncRepoTasks)
		internalAPI.POST("/tasks/execute/:id", c.Executor.ExecuteTask)
		internalAPI.POST("/tasks/toggle/:id", c.Task.ToggleTask)
	}
}

func initAuthorizedAPIRoutes(api *gin.RouterGroup, c *Controllers) {
	authorized := api.Group("")
	authorized.Use(middleware.AuthRequired())
	{
		// 获取当前用户 (普通用户即可访问)
		authorized.GET("/auth/me", c.Auth.GetCurrentUser)

		// OTP 两步验证管理 (普通用户即可访问，非 adminOnly)
		otp := authorized.Group("/auth/otp")
		{
			otp.GET("/status", c.Auth.GetOTPStatus)
			otp.POST("/generate", c.Auth.GenerateOTP)
			otp.POST("/enable", c.Auth.EnableOTP)
			otp.POST("/disable", c.Auth.DisableOTP)
		}

		// 以下管理接口需要管理员权限
		adminOnly := authorized.Group("")
		adminOnly.Use(middleware.AdminRequired())
		{
			registerFileSpecialRoutes(adminOnly, c)
			registerLogSSERoutes(adminOnly, c)
			registerTerminalWSRoutes(adminOnly, c)
			registerSettingsSpecialRoutes(adminOnly, c)
			registerAgentDownloadRoutes(adminOnly, c)
			registerSystemWSRoutes(adminOnly, c)
			registerWebUIUploadRoutes(adminOnly, c)
			registerMonitorSSERoutes(adminOnly, c)
			registerInterconnectProxyRoutes(adminOnly, c)
		}
	}

	// 通知发送 API（使用通知 Token 认证，供脚本调用）
	notifyAPI := api.Group("/notify")
	notifyAPI.Use(middleware.NotifyTokenAuth())
	{
		notifyAPI.POST("/send", c.Notification.SendNotification)
	}
}

func registerFileSpecialRoutes(g *gin.RouterGroup, c *Controllers) {
	// 文件流/上传接口保留 Gin 原生处理
	files := g.Group("/files")
	{
		files.GET("/download", c.File.DownloadFile)
		files.GET("/download-zip", c.File.DownloadZip)
		files.POST("/upload", c.File.UploadArchive)
		files.POST("/uploadfiles", c.File.UploadFiles)
	}
}

func registerLogSSERoutes(g *gin.RouterGroup, c *Controllers) {
	// 日志 SSE 保留 Gin 原生处理
	logs := g.Group("/logs")
	{
		logs.GET("/sse", c.LogSSE.StreamLog)
	}
}

func registerTerminalWSRoutes(g *gin.RouterGroup, c *Controllers) {
	// 终端 WebSocket 保留 Gin 原生处理
	g.GET("/terminal/ws", c.Terminal.HandleWebSocket)
}

func registerSettingsSpecialRoutes(g *gin.RouterGroup, c *Controllers) {
	// 文件流/上传接口保留 Gin 原生处理
	settings := g.Group("/settings")
	{
		settings.GET("/backup/download", c.Settings.DownloadBackup)
		settings.POST("/restore", c.Settings.RestoreBackup)
	}
}

func registerAgentDownloadRoutes(g *gin.RouterGroup, c *Controllers) {
	// Agent API（供前端调用，保持在 v1 下）
	// 文件下载（二进制流）保留 Gin 原生处理
	agentAPIv1 := g.Group("/agent")
	{
		agentAPIv1.GET("/download", c.Agent.Download)
	}
}

func registerSystemWSRoutes(g *gin.RouterGroup, c *Controllers) {
	g.GET("/ws/events", c.SystemWS.HandleEvents)
}

func registerMonitorSSERoutes(g *gin.RouterGroup, c *Controllers) {
	// SSE 保留 Gin 原生处理
	monitor := g.Group("/monitor")
	{
		monitor.GET("/sse", c.Monitor.MonitorSSE)
	}
}

func initAgentAPIRoutes(root *gin.RouterGroup, c *Controllers) {
	// Agent API（供远程 Agent 调用，不使用 /v1 版本号）
	agentAPI := root.Group("/api/agent")
	{
		agentAPI.POST("/heartbeat", c.Agent.Heartbeat)
		agentAPI.GET("/tasks", c.Agent.GetTasks)
		agentAPI.POST("/report", c.Agent.ReportResult)
		agentAPI.GET("/download", c.Agent.Download) // 也在这里注册，兼容 Agent 调用
		agentAPI.GET("/ws", c.Agent.WSConnect)      // WebSocket 连接
	}
}

func registerWebUIUploadRoutes(g *gin.RouterGroup, c *Controllers) {
	// multipart 文件上传保留 Gin 原生处理
	webuiGroup := g.Group("/webui")
	{
		webuiGroup.POST("/upload", c.WebUI.UploadWebUI)
	}
}

func registerInterconnectProxyRoutes(g *gin.RouterGroup, c *Controllers) {
	// 代理模式 (面板穿越) 动态路由保留 Gin 原生处理
	interconnect := g.Group("/interconnect")
	{
		interconnect.Any("/proxy/:node_id/*path", c.Interconnect.ProxyRequest)
	}
}





