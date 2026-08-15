package main

import (
	"fmt"
	"os"

	"github.com/engigu/baihu-panel/internal/router"
)

// export-openapi 是一个不监听端口的独立工具：
// 仅创建双 Huma 实例并导出 OpenAPI JSON 文档（不依赖数据库连接）。
//
// 用法：
//
//	go run ./cmd/export-openapi [输出目录]
//
// 默认输出目录为 docs/public，生成：
//   - docs/public/api-openapi.json        (/api/v1 管理接口)
//   - docs/public/open2api-openapi.json   (/open2api/v1 开放接口)
func main() {
	outDir := "docs/public"
	if len(os.Args) > 1 && os.Args[1] != "" {
		outDir = os.Args[1]
	}

	if err := router.ExportOpenAPI(outDir); err != nil {
		fmt.Fprintf(os.Stderr, "export-openapi 失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("OpenAPI 文档导出完成。")
}
