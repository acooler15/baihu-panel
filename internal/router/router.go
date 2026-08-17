package router

import (
	"os"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/controllers"
	"github.com/engigu/baihu-panel/internal/middleware"
	"github.com/engigu/baihu-panel/internal/services"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
)

type Controllers struct {
	Task         *controllers.TaskController
	Auth         *controllers.AuthController
	Env          *controllers.EnvController
	Script       *controllers.ScriptController
	Executor     *controllers.ExecutorController
	File         *controllers.FileController
	Dashboard    *controllers.DashboardController
	Log          *controllers.LogController
	LogSSE       *controllers.LogSSEController
	Terminal     *controllers.TerminalController
	Settings     *controllers.SettingsController
	Dependency   *controllers.DependencyController
	Agent        *controllers.AgentController
	Mise         *controllers.MiseController
	Notification *controllers.NotificationController
	AppLog       *controllers.AppLogController
	SystemWS     *controllers.SystemWSController
	WebUI        *controllers.WebUIController
	Monitor      *controllers.MonitorController
	Interconnect *controllers.InterconnectController
	Data         *controllers.DataController
	Tag          *controllers.TagController

	// Huma 实例（双文档方案）
	// APIV1Huma 挂载于 /api/v1，Open2APIV1Huma 挂载于 /open2api/v1
	APIV1Huma      huma.API
	Open2APIV1Huma huma.API
	// AgentHuma 挂载于 /api 前缀（供远程 Agent 调用）
	AgentHuma huma.API
}

func Setup(c *Controllers) *gin.Engine {
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(middleware.GinLogger(), middleware.GinRecovery())
	router.Use(middleware.TravelProxyMiddleware())

	// 获取 URL 前缀
	cfg := services.GetConfig()
	urlPrefix := strings.TrimSuffix(cfg.Server.URLPrefix, "/")

	// 创建一个路由组，如果有前缀则使用前缀，否则使用根路径
	var root *gin.RouterGroup
	if urlPrefix != "" {
		root = router.Group(urlPrefix)
	} else {
		root = router.Group("")
	}

	// 按需绑定 Pprof 调试路由 (注册在 root 下以支持 URLPrefix)
	if cfg.Server.PprofEnabled {
		// pprof.RouteRegister 会在传入的路由组下注册 /debug/pprof 等路由
		pprof.RouteRegister(root)
	}

	// =========================================================================
	// 路由分类组装 (对应 Nginx 的 location 块分发)
	// =========================================================================

	// 1. [ location /assets ] 静态资源路由
	initStaticRoutes(root)

	// 3. [ location /api ] 内部 API 路由组
	apiV1 := root.Group("/api/v1")
	initPublicAPIRoutes(apiV1, c)     // 公开接口 (无需认证)
	initAuthorizedAPIRoutes(apiV1, c) // 授权接口 (需 JWT)

	// 3.1 Huma 实例（双文档方案）：挂载于 /api/v1
	// 阶段 2 迁移的管理接口均为管理员权限接口，统一套用 AuthRequired + AdminRequired 鉴权；
	// 阶段 5 起，APIV1Huma 通过 selector 按 Operation 路径选择鉴权中间件：
	//   - /auth/me、/auth/otp/*        → AuthRequired（普通用户）
	//   - /notify/send                 → NotifyTokenAuth（独立鉴权）
	//   - 其余管理接口 / 文件流 / 代理   → AuthRequired + AdminRequired
	// 公开接口（/auth/login 等）与 WebSocket 接口继续由 Gin 原生处理。
	// API 版本号与编译产物版本号保持一致（由 Makefile LDFLAGS 注入，默认 "dev"）
	c.APIV1Huma = newHuma(router, "/api/v1", "Baihu Panel API", constant.Version,
		"内部管理 API。需通过登录后的 Cookie 会话进行鉴权。",
		func(op huma.Operation) []gin.HandlerFunc {
			p := op.Path
			switch {
			// 公开接口：无需鉴权
			case p == "/auth/login", p == "/auth/login/otp", p == "/auth/logout",
				p == "/settings/public", p == "/interconnect/report":
				return nil

			// 普通用户：仅 AuthRequired
			case p == "/auth/me", strings.HasPrefix(p, "/auth/otp/"):
				return []gin.HandlerFunc{middleware.AuthRequired()}

			// NotifyTokenAuth 独立鉴权
			case p == "/notify/send":
				return []gin.HandlerFunc{middleware.NotifyTokenAuth()}

			// 内部接口：仅本地回环 + 内部凭证
			case strings.HasPrefix(p, "/internal/"):
				return []gin.HandlerFunc{middleware.LocalhostOnly()}

			// 其余：AuthRequired + AdminRequired（含已迁移业务接口、SSE、文件流、代理）
			default:
				return []gin.HandlerFunc{middleware.AuthRequired(), middleware.AdminRequired()}
			}
		})

	// Agent 外部接口实例：挂载于 /api/agent 前缀（供远程 Agent 调用）。
	// 鉴权由各 handler 内部通过 `Authorization: Bearer <token>` 完成，此处不套用任何业务中间件。
	c.AgentHuma = newHuma(router, "/api", "Baihu Panel Agent API", constant.Version,
		"供远程 Agent 调用的外部接口。通过 `Authorization: Bearer <token>` 进行鉴权。", nil)

	// 4.1 Huma 实例（双文档方案）：挂载于 /open2api/v1，注册的 Huma 路由统一走 OpenapiRequired 鉴权
	// 注意：必须在 initOpenAPIV1Routes 之前创建，否则注册时实例仍为 nil
	c.Open2APIV1Huma = newHuma(router, "/open2api/v1", "Baihu Panel OpenAPI", constant.Version,
		"对外开放 API。需通过 Bearer Token 进行鉴权。",
		func(op huma.Operation) []gin.HandlerFunc {
			return []gin.HandlerFunc{middleware.OpenapiRequired()}
		})

	// 4. [ location /api/agent ] Agent 相关 API 路由组
	initAgentAPIRoutes(root, c)
	initOpenAPIV1Routes(c)
	// /api/v1 管理接口的 Huma 声明式注册（阶段 2 分批迁移）
	initAPIV1HumaRoutes(c)

	// =========================================================================
	// [ location / ] 全局 404 兜底与 SPA 渲染
	// 对应 Nginx: try_files $uri $uri/ /index.html;
	// =========================================================================
	router.NoRoute(func(ctx *gin.Context) {
		path := ctx.Request.URL.Path

		// 如果配置了前缀，只处理带前缀的路径
		if urlPrefix != "" && !strings.HasPrefix(path, urlPrefix) {
			ctx.Status(404)
			return
		}

		// 解析实际的相对路径
		relPath := strings.TrimPrefix(path, urlPrefix)
		if !strings.HasPrefix(relPath, "/") {
			relPath = "/" + relPath
		}

		// 拦截器：不该返回 index.html 的情况
		// 如果该请求被识别为 API 请求、静态资源请求，或者是带有明确文件后缀（如 .js / .css / .png）的物理文件请求
		// 都不应该返回 SPA 页面（会报前端 MIME 类型错误），而是直接掐断返回 404
		hasAnyExt := false
		if idx := strings.LastIndex(relPath, "."); idx > 0 && len(relPath)-idx < 6 {
			// 简单判断是否有后缀（如 .js, .css）
			hasAnyExt = true
		}

		if strings.HasPrefix(relPath, "/api/") || strings.HasPrefix(relPath, "/assets/") || strings.HasPrefix(relPath, "/debug/") || hasAnyExt {
			ctx.String(404, "404 Not Found")
			return
		}

		// 其他所有有效的前端页面路径（如 /tasks, /settings），都返回 index.html 交给 vue-router 处理
		serveSPA(ctx, urlPrefix, 200)
	})

	return router
}
