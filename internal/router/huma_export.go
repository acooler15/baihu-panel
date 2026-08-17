package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/controllers"
	"github.com/engigu/baihu-panel/internal/middleware"
	"github.com/gin-gonic/gin"
)

// ExportOpenAPI 生成双 Huma 实例的 OpenAPI 文档（/api/v1 与 /open2api/v1），
// 分别写入 outDir/api-openapi.json 与 outDir/open2api-openapi.json。
//
// 设计要点：
//   - 本函数不监听端口、不依赖数据库连接（仅生成 schema）。
//   - 路由注册使用零值控制器：huma.Register 只对 handler 做类型反射（Input/Output
//     结构体）以生成 OpenAPI 文档，不会真正调用 handler 方法，因此无需初始化服务层。
func ExportOpenAPI(outDir string) error {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()

	// /api/v1 管理接口（Cookie 鉴权）
	// 与 router.Setup 中的 APIV1Huma 保持一致，通过 selector 按 Operation 选择中间件
	apiHuma := newHuma(engine, "/api/v1", "Baihu Panel API", constant.Version,
		"内部管理 API。需通过登录后的 Cookie 会话进行鉴权。",
		func(op huma.Operation) []gin.HandlerFunc {
			p := op.Path
			switch {
			case p == "/auth/me", strings.HasPrefix(p, "/auth/otp/"):
				return []gin.HandlerFunc{middleware.AuthRequired()}
			case p == "/notify/send":
				return []gin.HandlerFunc{middleware.NotifyTokenAuth()}
			default:
				return []gin.HandlerFunc{middleware.AuthRequired(), middleware.AdminRequired()}
			}
		})

	// /open2api/v1 开放接口（Bearer Token 鉴权）
	open2apiHuma := newHuma(engine, "/open2api/v1", "Baihu Panel OpenAPI", constant.Version,
		"对外开放 API。需通过 Bearer Token 进行鉴权。",
		func(op huma.Operation) []gin.HandlerFunc {
			return []gin.HandlerFunc{middleware.OpenapiRequired()}
		})

	c := &Controllers{
		APIV1Huma:      apiHuma,
		Open2APIV1Huma: open2apiHuma,
		Auth:           &controllers.AuthController{},
		Task:           &controllers.TaskController{},
		Env:            &controllers.EnvController{},
		Script:         &controllers.ScriptController{},
		Executor:       &controllers.ExecutorController{},
		File:           &controllers.FileController{},
		Dashboard:      &controllers.DashboardController{},
		Log:            &controllers.LogController{},
		Settings:       &controllers.SettingsController{},
		Dependency:     &controllers.DependencyController{},
		Agent:          &controllers.AgentController{},
		Mise:           &controllers.MiseController{},
		Notification:   &controllers.NotificationController{},
		AppLog:         &controllers.AppLogController{},
		WebUI:          &controllers.WebUIController{},
		Monitor:        &controllers.MonitorController{},
		Interconnect:   &controllers.InterconnectController{},
		Data:           &controllers.DataController{},
		Tag:            &controllers.TagController{},
		Terminal:       &controllers.TerminalController{},
	}

	// 注册全部 Huma 路由（与 router.Setup 保持同一套注册入口）
	initOpenAPIV1Routes(c)
	initAPIV1HumaRoutes(c)

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	if err := writeOpenAPIDoc(filepath.Join(outDir, "api-openapi.json"), apiHuma.OpenAPI()); err != nil {
		return err
	}
	if err := writeOpenAPIDoc(filepath.Join(outDir, "open2api-openapi.json"), open2apiHuma.OpenAPI()); err != nil {
		return err
	}
	return nil
}

// writeOpenAPIDoc 将 OpenAPI 文档以缩进 JSON 形式写入文件。
func writeOpenAPIDoc(path string, doc *huma.OpenAPI) error {
	raw, err := doc.MarshalJSON()
	if err != nil {
		return fmt.Errorf("序列化 %s 失败: %w", path, err)
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return fmt.Errorf("格式化 %s 失败: %w", path, err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("写入 %s 失败: %w", path, err)
	}
	fmt.Printf("已导出 OpenAPI 文档: %s\n", path)
	return nil
}
