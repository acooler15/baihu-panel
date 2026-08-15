package router

// initOpenAPIV1Routes 初始化 OpenAPI v1 路由（Huma 声明式注册）。
//
// 说明：OpenAPI v1 路由通过 Huma 实例 c.Open2APIV1Huma 注册，
// 鉴权由 newHuma 中传入的 OpenapiRequired 中间件统一处理。
func initOpenAPIV1Routes(c *Controllers) {
	if c.Open2APIV1Huma == nil {
		return
	}

	api := c.Open2APIV1Huma
	// 任务相关接口
	c.Task.RegisterOpenAPITaskRoutes(api)
	// 环境变量相关接口
	c.Env.RegisterOpenAPIEnvRoutes(api)
	// 脚本相关接口
	c.Script.RegisterOpenAPIScriptRoutes(api)
	// 日志相关接口
	c.Log.RegisterOpenAPILogRoutes(api)
	// 任务执行相关接口
	c.Executor.RegisterOpenAPIExecutorRoutes(api)
}
