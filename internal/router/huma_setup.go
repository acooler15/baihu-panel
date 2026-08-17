package router

import (
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/engigu/baihu-panel/internal/utils"
	"github.com/gin-gonic/gin"
)

// mwSelector 根据 Operation 决定套用哪些 gin 中间件。
// /api/v1 实例内存在多种鉴权级别（普通用户 / 管理员 / NotifyToken），
// 通过该函数按 op.Path 返回对应的 gin handler 链。
type mwSelector func(op huma.Operation) []gin.HandlerFunc

// newHuma 创建挂载到指定路径前缀的 Huma 实例。
//
// 说明：humagin.New 仅接受 *gin.Engine 作为挂载目标（路由注册在引擎根路径），
// 无法直接挂载到 gin.RouterGroup。为支持在 /api/v1 与 /open2api/v1 下各挂载
// 一个独立的 Huma 实例（双文档方案），这里通过自定义 adapter 在注册 Operation
// 时统一为路径追加前缀，从而实现按前缀隔离。
//
// selector 根据每个 Operation 决定该路由套用哪些 gin 中间件（如 OpenapiRequired、
// AuthRequired + AdminRequired 等鉴权链）。为 nil 时不套用任何业务中间件。
func newHuma(engine *gin.Engine, prefix, title, version, desc string, selector mwSelector) huma.API {
	config := huma.DefaultConfig(title, version)
	config.Info.Description = desc
	// 安全方案
	config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"BearerAuth": {
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
			Description:  "Type 'Bearer' followed by a space and the API token.",
		},
		"CookieAuth": {
			Type:        "apiKey",
			In:          "cookie",
			Name:        "bh_token",
			Description: "Session cookie set after login.",
		},
		"NotifyTokenAuth": {
			Type:        "apiKey",
			In:          "header",
			Name:        "notify-token",
			Description: "通知 Token，通过 `notify-token` 请求头传递（供脚本调用发送通知）。",
		},
	}

	// servers 配置：默认使用相对路径，使 OpenAPI 文档可跟随部署域名自动适配
	config.Servers = []*huma.Server{
		{
			URL:         "/",
			Description: "Baihu Panel API",
		},
	}

	// 安装自定义错误 Transformer
	config.Transformers = append(config.Transformers, utils.HumaTransformer)

	return huma.NewAPI(config, &prefixAdapter{
		engine:   engine,
		prefix:   strings.TrimSuffix(prefix, "/"),
		selector: selector,
	})
}

// prefixAdapter 实现 huma.Adapter，在注册路由时自动为路径追加前缀，
// 使同一个 gin.Engine 下可以挂载多个独立的 Huma 实例。
type prefixAdapter struct {
	engine   *gin.Engine
	prefix   string
	selector mwSelector
}

func (a *prefixAdapter) Handle(op *huma.Operation, handler func(huma.Context)) {
	// 将 {param} 转换为 gin 的 :param
	path := a.prefix + op.Path
	path = strings.ReplaceAll(path, "{", ":")
	path = strings.ReplaceAll(path, "}", "")

	// Huma 自动注册的 OpenAPI 规范 / 文档 / schema 路由不应套用业务鉴权中间件，
	// 否则文档无法被公开访问。通过路径特征识别并跳过中间件。
	var handlers []gin.HandlerFunc
	if !isHumaMetaPath(op.Path) && a.selector != nil {
		handlers = a.selector(*op)
	}

	a.engine.Handle(op.Method, path, append(handlers, func(c *gin.Context) {
		// 将 *gin.Context 注入到请求 context，供 Huma handler 读取中间件写入的值
		utils.InjectGinContext(c)
		handler(humagin.NewContext(op, c))
	})...)
}

// isHumaMetaPath 判断是否为 Huma 自动注册的元信息路由（OpenAPI 规范、文档、schema）。
// 这些路由由 huma.NewAPI 通过 adapter.Handle 注册，路径形如 /openapi.json、
// /docs、/schemas/{schema} 等，不应被业务鉴权中间件拦截。
func isHumaMetaPath(opPath string) bool {
	base := opPath
	if idx := strings.Index(base, "?"); idx >= 0 {
		base = base[:idx]
	}
	return strings.HasPrefix(base, "/openapi") ||
		strings.HasPrefix(base, "/docs") ||
		strings.HasPrefix(base, "/schemas")
}

func (a *prefixAdapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// prefixAdapter 仅用于注册路由，实际请求由 gin.Engine 直接处理，
	// 因此这里不参与实际的 HTTP 处理。
	a.engine.ServeHTTP(w, r)
}
