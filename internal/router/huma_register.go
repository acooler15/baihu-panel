package router

// initAPIV1HumaRoutes 初始化 /api/v1 管理接口的 Huma 声明式注册。
//
// 阶段 2 按 controller 分批迁移。已迁移的 controller 通过各自的 RegisterAPI*Routes
// 方法将接口注册到 c.APIV1Huma。鉴权由 newHuma 中传入的 AuthRequired + AdminRequired
// 中间件统一处理。特殊接口（WS/SSE/文件流/代理）仍由 api_routes.go 中的 Gin 原生路由处理。
func initAPIV1HumaRoutes(c *Controllers) {
	if c.APIV1Huma == nil {
		return
	}

	api := c.APIV1Huma

	// 批次 1：标签管理
	c.Tag.RegisterAPITagRoutes(api)
	// 批次 2：仪表盘
	c.Dashboard.RegisterAPIDashboardRoutes(api)
	// 批次 3：通知管理
	c.Notification.RegisterAPINotificationRoutes(api)
	// 批次 4：应用日志
	c.AppLog.RegisterAPIAppLogRoutes(api)
	// 批次 5：脚本管理
	c.Script.RegisterAPIScriptRoutes(api)
	// 批次 6：环境变量
	c.Env.RegisterAPIEnvRoutes(api)
	// 批次 7：任务管理
	c.Task.RegisterAPITaskRoutes(api)
	// 批次 8：依赖管理
	c.Dependency.RegisterAPIDependencyRoutes(api)
	// 批次 9：Mise 环境
	c.Mise.RegisterAPIMiseRoutes(api)
	// 批次 10：Agent 管理
	c.Agent.RegisterAPIAgentRoutes(api)
	// 批次 11：任务执行
	c.Executor.RegisterAPIExecutorRoutes(api)
	// 批次 12：系统设置
	c.Settings.RegisterAPISettingsRoutes(api)
	// 批次 13：数据管理
	c.Data.RegisterAPIDataRoutes(api)
	// 批次 14：系统监控
	c.Monitor.RegisterAPIMonitorRoutes(api)
	// 批次 15：互联互通
	c.Interconnect.RegisterAPIInterconnectRoutes(api)
	// 批次 16：日志管理
	c.Log.RegisterAPILogRoutes(api)
	// 批次 17：终端
	c.Terminal.RegisterAPITerminalRoutes(api)
	// 批次 18：SystemWS（WebSocket 特殊接口，保留 Gin 原生处理，无需迁移）
	// 批次 19：WebUI 管理
	c.WebUI.RegisterAPIWebUIRoutes(api)
	// 批次 20：Auth（登录/OTP 等特殊接口，保留 Gin 原生处理，无需迁移）
	// 批次 21：文件管理
	c.File.RegisterAPIFileRoutes(api)
}
