package router

import (
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/engigu/baihu-panel/internal/utils"
	"github.com/gin-gonic/gin"
)

// newHuma 创建挂载到指定路径前缀的 Huma 实例。
//
// 说明：humagin.New 仅接受 *gin.Engine 作为挂载目标（路由注册在引擎根路径），
// 无法直接挂载到 gin.RouterGroup。为支持在 /api/v1 与 /open2api/v1 下各挂载
// 一个独立的 Huma 实例（双文档方案），这里通过自定义 adapter 在注册 Operation
// 时统一为路径追加前缀，从而实现按前缀隔离。
func newHuma(engine *gin.Engine, prefix, title, version, desc string) huma.API {
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
	}

	// 安装自定义错误 Transformer
	config.Transformers = append(config.Transformers, utils.HumaTransformer)

	return huma.NewAPI(config, &prefixAdapter{
		engine: engine,
		prefix: strings.TrimSuffix(prefix, "/"),
	})
}

// prefixAdapter 实现 huma.Adapter，在注册路由时自动为路径追加前缀，
// 使同一个 gin.Engine 下可以挂载多个独立的 Huma 实例。
type prefixAdapter struct {
	engine *gin.Engine
	prefix string
}

func (a *prefixAdapter) Handle(op *huma.Operation, handler func(huma.Context)) {
	// 将 {param} 转换为 gin 的 :param
	path := a.prefix + op.Path
	path = strings.ReplaceAll(path, "{", ":")
	path = strings.ReplaceAll(path, "}", "")

	a.engine.Handle(op.Method, path, func(c *gin.Context) {
		handler(humagin.NewContext(op, c))
	})
}

func (a *prefixAdapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// prefixAdapter 仅用于注册路由，实际请求由 gin.Engine 直接处理，
	// 因此这里不参与实际的 HTTP 处理。
	a.engine.ServeHTTP(w, r)
}
