# 迁移执行方案：Gin → Huma+Gin

> 本文档为 AI 可执行的迁移方案，包含完整的背景、决策、步骤、代码规范与验收标准。
> AI 在执行时，应按阶段顺序推进，严格遵守每节的"执行约束"与"验收清单"。
>
> **编译验证环境约束**：若在 Windows 上进行编译验证，编译目标必须为 `linux/amd64`（项目部署目标为 Linux）。所有涉及 `go build` 的验证步骤，在 Windows 环境下均须使用交叉编译命令 `GOOS=linux GOARCH=amd64 go build ./...`，禁止直接执行 `go build ./...` 生成 Windows 二进制。

---

## 0. 背景与现状

### 0.1 项目技术栈

- 后端：Go 1.26 + Gin v1.12 + GORM
- 前端：Vue3 + fetch 封装（`web/src/api/index.ts`）
- 文档：swag 注解式（`@Summary` 等）+ `swag init` 生成 swagger.json，前端用 Scalar 渲染
- 鉴权：Cookie JWT（`/api`）+ Bearer Token（`/open2api`）+ Notify Token + 互联 Token

### 0.2 路由结构

```
gin.Engine
└── root (可选 URLPrefix)
    ├── /assets/*, /logo.svg, PWA 路由 (静态资源，Gin 原生)
    ├── /api/v1
    │   ├── /auth/* (公开)
    │   ├── /interconnect/tunnel, /report (公开)
    │   ├── /internal/* (LocalhostOnly)
    │   ├── /auth/me, /auth/otp/* (AuthRequired)
    │   ├── /dashboard, /tasks, /env, /scripts, /files, /logs, /terminal,
    │   │   /settings, /deps, /agents, /mise, /notify, /app-logs,
    │   │   /ws/events, /webui, /monitor, /interconnect, /system, /tags (AuthRequired + AdminRequired)
    │   └── /notify/send (NotifyTokenAuth)
    ├── /api/agent (Agent 通信: heartbeat/tasks/report/download/ws)
    └── /open2api/v1 (OpenapiRequired: tasks/env/scripts/logs/execute)
└── NoRoute → SPA index.html 兜底
```

### 0.3 当前响应体规范

文件：`internal/utils/response.go`

```go
type Response struct {
    Code int         `json:"code"`        // 业务码：200/400/401/403/404/409/429/500
    Msg  string      `json:"msg"`
    Data interface{} `json:"data,omitempty"`
}
```

- 所有响应 HTTP 状态码恒为 200，仅靠 `code` 字段区分业务状态
- 工具函数：`utils.Success(c, data)` / `utils.SuccessMsg(c, msg)` / `utils.Error(c, code, msg)` / `utils.BadRequest` / `utils.Unauthorized` / `utils.Forbidden` / `utils.NotFound` / `utils.ServerError`

### 0.4 当前文档机制

- `main.go` 顶部 swag 全局注解（`@title Baihu Panel API`，`@BasePath /open2api/v1`）
- 各 controller 方法上方 `@Summary`/`@Router`/`@Param`/`@Success` 注解
- `make swag` → `go run github.com/swaggo/swag/cmd/swag@latest init -g main.go -o ./docs/public --ot json,yaml`
- 产物：`docs/public/swagger.json` / `swagger.yaml`
- 前端渲染：`docs/guide/api.md` 使用 `@scalar/api-reference` 指向 `/baihu-panel/swagger.json`

### 0.5 前端调用约定

文件：`web/src/api/index.ts`

```ts
interface ApiResponse<T> { code: number; msg: string; data: T }
// fetch 后统一解析 json，判断 json.code !== 200 抛错，json.code === 401 跳登录
```

---

## 1. 迁移决策（已确认）

> **用户最终确认（2026-08-15）**：
> - 响应码方案：**方案 A**（HTTP 状态码 = 业务码）
> - 文档分离：**做法 1**（双 Huma 实例，两份 OpenAPI 文档）
> - 迁移方式：**渐进式**（阶段 0→1→2→3→4，分 controller 批次推进）
> - 特殊接口：**保留 Gin 原生处理**（WebSocket/SSE/文件流/代理等）
>
> 以下各项均为最终决策，执行过程中不再变更。

### 1.1 响应码方案：方案 A

- **HTTP 状态码 = 业务码**：成功 200，失败 400/401/403/404/409/429/500
- **响应体字段结构不变**：`{code, msg, data}` 三字段保留
- **前端兼容**：过渡期双判断 `!res.ok || json.code !== 200`，最终可简化为 `!res.ok`

> 理由：符合 OpenAPI 响应码规范，Huma 原生支持，响应体结构对前端透明。

### 1.2 文档分离：做法 1（双 Huma 实例）

- 创建 `apiHuma`（挂载 `/api/v1`）和 `open2apiHuma`（挂载 `/open2api/v1`）两个独立 Huma 实例
- 分别导出 `openapi.json`，前端 Scalar 分别渲染或合并展示
- 控制器业务逻辑共享，仅注册入口分离

### 1.3 迁移方式：渐进式

- 严格按阶段 0→1→2→3→4 顺序推进，不跳跃
- 每阶段完成后对照验收清单逐项确认，通过后方可进入下一阶段
- 阶段 2 内按 controller 分批次迁移，每批次（controller）独立提交，便于回滚
- 过渡期内新旧方法并存：新增 `*Huma` 方法与旧 `*gin.Context` 方法共存，该 controller 全部迁移完成并测试通过后再删除旧方法

### 1.4 特殊接口保留 Gin 原生

以下接口**不迁移**至 Huma 声明式注册，保留 `gin.RouterGroup` 原生处理：

| 接口 | 原因 |
|------|------|
| `GET /terminal/ws` | WebSocket 长连接 |
| `GET /api/agent/ws` | WebSocket |
| `GET /ws/events` | WebSocket |
| `GET /logs/sse` | SSE 流 |
| `GET /monitor/sse` | SSE 流 |
| `GET /interconnect/tunnel` | WebSocket 隧道 |
| `POST /interconnect/report` | 内部协议 |
| `Any /interconnect/proxy/*` | 反向代理透传 |
| `GET /files/download*` | 文件流响应 |
| `GET /agent/download` | 文件下载 |
| `POST /files/upload*` | multipart 上传 |
| `internal/*` 路由 | 仅本地调用 |
| `NoRoute` SPA 兜底 | 非 API |

这些接口在 OpenAPI 文档中通过手动补充 Operation 元数据描述（不注册 handler）。

---

## 2. 目标架构

### 2.1 依赖

```bash
go get github.com/danielgtaylor/huma/v2
```

> Huma v2 通过 `humagin` 适配器挂载到 `*gin.Engine` / `gin.RouterGroup`。
> 项目已间接依赖 `kin-openapi`（go.mod），无版本冲突。

### 2.2 目录结构变更

```
internal/
├── router/
│   ├── router.go              (修改: 创建双 Huma 实例并挂载)
│   ├── huma_setup.go          (新增: Huma 实例工厂、配置、Transformer)
│   ├── huma_register.go       (新增: Huma 路由注册入口)
│   ├── api_routes.go          (保留: Gin 原生特殊接口)
│   ├── openapi.go             (重写: Huma 注册 open2api 接口)
│   ├── register.go            (保留: 控制器依赖注入)
│   ├── static_routes.go       (保留)
│   └── events.go              (保留)
├── utils/
│   ├── response.go            (保留: 过渡期兼容)
│   └── huma_response.go       (新增: Huma 泛型响应模型 + Transformer)
├── controllers/
│   ├── *_controller.go        (保留旧 Gin 方法，过渡期)
│   └── *_controller_huma.go   (新增: Huma handler，或直接在原文件追加)
└── models/vo/
    └── *.go                   (补全 struct tag: json + description)
```

### 2.3 响应模型规范

文件：`internal/utils/huma_response.go`（新建）

```go
package utils

import (
    "context"
    "net/http"

    "github.com/danielgtaylor/huma/v2"
)

// HumaResponse 统一响应体，结构与原 utils.Response 一致
type HumaResponse[T any] struct {
    Code int    `json:"code"`
    Msg  string `json:"msg"`
    Data T      `json:"data,omitempty"`
}

// HumaError 业务错误，实现 error 接口
type HumaError struct {
    Status  int
    Code    int    // 业务码，通常等于 Status
    Msg     string
}

func (e *HumaError) Error() string { return e.Msg }

func NewHumaError(status int, msg string) *HumaError {
    return &HumaError{Status: status, Code: status, Msg: msg}
}

// HumaTransformer 将 HumaError 转换为 {code, msg} 响应
func HumaTransformer(ctx context.Context, status *int, v any) error {
    if he, ok := v.(*HumaError); ok {
        *status = he.Status
        return nil
    }
    // 默认透传
    return nil
}

// 便捷错误构造函数
func HumaBadRequest(msg string) *HumaError      { return NewHumaError(http.StatusBadRequest, msg) }
func HumaUnauthorized(msg string) *HumaError    { return NewHumaError(http.StatusUnauthorized, msg) }
func HumaForbidden(msg string) *HumaError       { return NewHumaError(http.StatusForbidden, msg) }
func HumaNotFound(msg string) *HumaError        { return NewHumaError(http.StatusNotFound, msg) }
func HumaConflict(msg string, data any) *HumaError { return NewHumaError(http.StatusConflict, msg) }
func HumaTooManyRequests(msg string) *HumaError { return NewHumaError(http.StatusTooManyRequests, msg) }
func HumaServerError(msg string) *HumaError     { return NewHumaError(http.StatusInternalServerError, msg) }
```

### 2.4 Huma 实例工厂

文件：`internal/router/huma_setup.go`（新建）

```go
package router

import (
    "github.com/danielgtaylor/huma/v2"
    "github.com/danielgtaylor/huma/v2/humagin"
    "github.com/gin-gonic/gin"
    "github.com/engigu/baihu-panel/internal/utils"
)

// newHuma 创建挂载到 gin.RouterGroup 的 Huma 实例
func newHuma(group *gin.RouterGroup, title, version, desc string) huma.API {
    config := huma.DefaultConfig(title, version)
    config.Info.Description = desc
    config.OpenAPI.OpenAPI = "3.1.0"  // 使用 OpenAPI 3.1

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

    return humagin.New(group, config)
}
```

### 2.5 Handler 编写规范

每个 Huma handler 遵循以下模式：

```go
// 1. 定义 Input（请求）
type CreateTaskInput struct {
    Body vo.TaskCreateReq
}

// 2. 定义 Output（响应）
type CreateTaskOutput struct {
    Body utils.HumaResponse[*vo.TaskVO]
}

// 3. 实现 handler（复用原有业务逻辑）
func (tc *TaskController) CreateTaskHuma(ctx context.Context, input *CreateTaskInput) (*CreateTaskOutput, error) {
    // === 以下为原 CreateTask 的业务逻辑，原样搬入 ===
    req := input.Body

    if req.Type != constant.TaskTypeRepo && req.Command == "" {
        return nil, utils.HumaBadRequest("命令不能为空")
    }
    // ... 其余校验与业务逻辑 ...

    task := tc.taskService.CreateTask(&param)
    // === 业务逻辑结束 ===

    return &CreateTaskOutput{
        Body: utils.HumaResponse[*vo.TaskVO]{
            Code: 200,
            Msg:  "success",
            Data: vo.ToTaskVO(task),
        },
    }, nil
}
```

### 2.6 路由注册规范

```go
// 在 huma_register.go 或对应 controller 的 Huma 注册函数中
huma.Register(apiHuma, huma.Operation{
    Method:      http.MethodPost,
    Path:        "/tasks",
    OperationID: "createTask",
    Summary:     "创建任务",
    Description: "创建一个新的任务",
    Tags:        []string{"任务管理"},
    Security:    []map[string][]string{{"CookieAuth": {}}},
}, tc.CreateTaskHuma)
```

---

## 3. 执行阶段

### 阶段 0：依赖引入与基础设施搭建

**目标**：引入 Huma，建立双 Huma 实例 + Gin 共存骨架，不改动任何现有接口行为。

#### 执行步骤

1. **添加依赖**
   ```bash
   go get github.com/danielgtaylor/huma/v2
   go mod tidy
   ```

2. **新建** `internal/utils/huma_response.go`：按 2.3 节内容创建。

3. **新建** `internal/router/huma_setup.go`：按 2.4 节内容创建 `newHuma` 工厂函数。

4. **修改** `internal/router/router.go` 的 `Setup` 函数：
   - 在创建 `apiV1 := root.Group("/api/v1")` 后，创建 `apiHuma := newHuma(apiV1, ...)`
   - 在创建 `open := root.Group("/open2api/v1")` 后，创建 `open2apiHuma := newHuma(open, ...)`
   - 将两个 Huma 实例通过 `Controllers` 结构或参数传递给注册函数
   - **暂不注册任何接口**，仅验证实例创建成功

5. **验证编译与启动**：`go build ./... && ./baihu-panel`（Windows 环境使用 `GOOS=linux GOARCH=amd64 go build ./...` 仅验证编译，运行时测试需在 Linux/WSL/Docker 中进行），确认服务正常启动。

#### 执行约束

- 不删除任何现有 Gin 路由注册
- 不修改任何 controller 方法
- 不修改前端代码

#### 验收清单

- [ ] `go build ./...` 通过（Windows 环境须使用 `GOOS=linux GOARCH=amd64 go build ./...`）
- [ ] 服务正常启动，`/api/v1/ping` 返回 `{"message":"pong"}`
- [ ] 所有现有接口行为不变
- [ ] `go.mod` 包含 `github.com/danielgtaylor/huma/v2`

---

### 阶段 1：迁移 `/open2api/v1`（外部 API）

**目标**：将 `/open2api/v1` 下 5 个模块迁移至 Huma，生成独立 OpenAPI 文档。

#### 迁移范围

| 模块 | 接口数 | 方法 |
|------|--------|------|
| tasks | 7 | POST ``, GET ``, GET `/:id`, PUT `/:id`, DELETE `/:id`, POST `/stop/:logID`, GET `/tags` |
| env | 8 | POST ``, GET ``, GET `/all`, GET `/:id`, GET `/:id/tasks`, PUT `/:id`, DELETE `/:id` |
| scripts | 5 | POST ``, GET ``, GET `/:id`, PUT `/:id`, DELETE `/:id` |
| logs | 2 | GET ``, GET `/:id` |
| execute | 2 | POST `/task/:id`, GET `/results` |

#### 执行步骤

1. **补全 VO struct tag**：为 `vo/task_vo.go`、`vo/env.go`（如不存在则新建）、`vo/script_vo.go` 中的结构体补充 `description` tag。
   ```go
   type TaskCreateReq struct {
       Name string `json:"name" binding:"required" description:"任务名称" example:"测试任务"`
       // ...
   }
   ```

2. **为每个 controller 创建 Huma handler**：
   - 优先选择直接在原 controller 文件中追加 `*Huma` 方法（如 `CreateTaskHuma`），避免文件爆炸
   - 若原文件过大，可新建 `*_huma.go` 文件
   - **复用原有业务逻辑**，仅替换：
     - `c.ShouldBindJSON(&req)` → 从 `input.Body` 获取
     - `c.Param("id")` → 从 `input.ID` 获取（需在 Input struct 定义 path param）
     - `c.Query("name")` → 从 `input.Name` 获取（需在 Input struct 定义 query param）
     - `utils.Success(c, data)` → `return &Output{Body: ...}, nil`
     - `utils.BadRequest(c, msg)` → `return nil, utils.HumaBadRequest(msg)`
     - `c.GetString("userID")` → 从 context 获取（需通过中间件注入或 huma context）

3. **重写** `internal/router/openapi.go`：
   - 删除原 `registerOpenAPITaskRoutes` 等 Gin 路由注册
   - 改为调用各 controller 的 `RegisterOpenAPIRoutes(open2apiHuma)` 方法
   - 每个 controller 提供一个 `RegisterOpenAPIRoutes(api huma.API)` 方法

4. **移除被迁移方法的 swag 注解**（`@Summary` 等），改由 Huma Operation 描述。

5. **context 传递用户信息**：
   - `OpenapiRequired` 中间件继续用 `c.Set("userID", ...)` 设置
   - Huma handler 中通过 `humaauth` 或手动从 gin context 提取：
     ```go
     // 在 Input 中定义，或通过 wrapper 获取
     userID := "" // 需从 context 提取，见下方"鉴权上下文传递"
     ```

#### 鉴权上下文传递方案

Huma handler 签名为 `func(ctx context.Context, input *Input) (*Output, error)`，无法直接访问 `*gin.Context`。解决方案：

```go
// 方案：通过 huma 中间件将 gin context 值注入到 ctx
import "github.com/gin-gonic/gin"

// 在 newHuma 中添加中间件
group.Use(func(c *gin.Context) {
    ctx := context.WithValue(c.Request.Context(), ginContextKey{}, c)
    c.Request = c.Request.WithContext(ctx)
    c.Next()
})

// handler 中提取
func getGinContext(ctx context.Context) *gin.Context {
    return ctx.Value(ginContextKey{}).(*gin.Context)
}

type ginContextKey struct{}

// 在 handler 中
func (tc *TaskController) SomeHuma(ctx context.Context, input *SomeInput) (*SomeOutput, error) {
    c := getGinContext(ctx)
    userID := c.GetString("userID")
    // ...
}
```

#### 执行约束

- 每个模块迁移后立即测试，确认接口行为不变
- 保留原 Gin 方法（如 `CreateTask`），新增 `CreateTaskHuma`，避免过渡期破坏
- 响应体字段顺序保持 `{code, msg, data}`

#### 验收清单

- [ ] `/open2api/v1/tasks` 等 5 模块接口功能正常
- [ ] `/open2api/v1/openapi.json`（或通过 huma API 获取）可访问，包含所有迁移接口
- [ ] 文档中 `securitySchemes` 包含 `BearerAuth`
- [ ] 所有响应体为 `{code, msg, data}` 格式
- [ ] HTTP 状态码与业务码一致（200/400/401/404/500）
- [ ] `go build ./...` 通过（Windows 环境须使用 `GOOS=linux GOARCH=amd64 go build ./...`）

---

### 阶段 2：迁移 `/api/v1` 管理接口

**目标**：将 `/api/v1` 下管理接口迁移至 Huma。工作量最大，按 controller 分批迁移。

#### 迁移顺序（按复杂度从低到高）

| 批次 | Controller | 接口数 | 复杂度 | 备注 | 状态 |
|------|------------|--------|--------|------|------|
| 1 | Tag | 4 | 低 | 纯 CRUD | ✅ 已迁移 |
| 2 | Dashboard | 4 | 低 | 只读查询 | ✅ 已迁移 |
| 3 | Notification | 10 | 低 | CRUD + 测试（`/notify/send` 使用 NotifyTokenAuth 保留 Gin） | ✅ 已迁移 |
| 4 | AppLog | 3 | 低 | CRUD | ✅ 已迁移 |
| 5 | Script | 5 | 低 | CRUD | ✅ 已迁移 |
| 6 | Env | 9 | 中 | 含分页、关联任务、409 冲突响应 | ✅ 已迁移 |
| 7 | Task | 12 | 中 | 含仓库任务、批量操作、cron 校验 | ✅ 已迁移 |
| 8 | Dependency | 13 | 中 | 含命令生成、安装 | ✅ 已迁移 |
| 9 | Mise | 10 | 中 | 含命令执行 | ✅ 已迁移 |
| 10 | Agent | 11 | 中 | 含 token 管理（`/agent/download` 保留 Gin） | ✅ 已迁移 |
| 11 | Executor | 3 | 中 | 含任务执行 | ✅ 已迁移 |
| 12 | Settings | 18 | 高 | 含备份恢复、通用配置（download/restore 保留 Gin） | ✅ 已迁移 |
| 13 | Data | 2 | 高 | 含导入导出 | ✅ 已迁移 |
| 14 | Monitor | 2 | 中 | **SSE 保留 Gin**，仅迁移 GET `/monitor` | ✅ 已迁移 |
| 15 | Interconnect | 9 | 高 | **tunnel/report/proxy 保留 Gin**，其余迁移 | ✅ 已迁移 |
| 16 | Log/LogSSE | 5 | 中 | **SSE 保留 Gin**，其余迁移 | ✅ 已迁移 |
| 17 | Terminal | 2 | 低 | **WS 保留 Gin**，迁移 cmds + execute | ✅ 已迁移 |
| 18 | SystemWS | 1 | 低 | **WS 保留 Gin**，不迁移 | ➖ 不迁移 |
| 19 | WebUI | 4 | 中 | 含文件上传，**upload 保留 Gin** | ✅ 已迁移 |
| 20 | Auth | 7 | 中 | 登录为公开路由、`/auth/me` 与 `/auth/otp/*` 为 AuthRequired（非 admin），与 `/api/v1` Huma 实例的 AdminRequired 不兼容，**保留 Gin** | ➖ 保留 Gin |
| 21 | File | 12 | 高 | **download/upload 保留 Gin**，其余迁移 | ✅ 已迁移 |

#### 每批次执行步骤（通用模板）

1. **阅读该 controller 的所有方法**，理解请求/响应结构
2. **补全 VO struct tag**（若未在阶段 1 补全）
3. **为每个方法创建 Huma handler**（按 2.5 节规范）
4. **创建该 controller 的 `RegisterAPIRoutes(api huma.API)` 方法**，注册所有 Huma 接口
5. **修改 `api_routes.go`**：删除已迁移接口的 Gin 注册，保留特殊接口（WS/SSE/文件流/代理）
6. **移除 swag 注解**
7. **测试该 controller 所有接口**

#### 分页接口规范

```go
// Input
type GetTasksInput struct {
    Name     string `query:"name" description:"任务名称"`
    AgentID  string `query:"agent_id" description:"Agent ID"`
    Tags     string `query:"tags" description:"标签"`
    Type     string `query:"type" description:"任务类型"`
    SortBy   string `query:"sort_by" description:"排序字段"`
    Order    string `query:"order" description:"排序方向"`
    Page     int    `query:"page" default:"1" description:"页码"`
    PageSize int    `query:"page_size" default:"10" description:"每页数量"`
}

// Output
type GetTasksOutput struct {
    Body utils.HumaResponse[utils.PaginationData[[]*vo.TaskVO]]
}
```

#### 409 冲突响应规范（如 EnvController.DeleteEnvVar）

```go
// 当环境变量被任务引用时
type DeleteEnvVarOutput struct {
    Body utils.HumaResponse[[]*vo.TaskVO]
}

func (ec *EnvController) DeleteEnvVarHuma(ctx context.Context, input *DeleteEnvVarInput) (*DeleteEnvVarOutput, error) {
    // ...
    if len(associatedTasks) > 0 {
        return &DeleteEnvVarOutput{
            Body: utils.HumaResponse[[]*vo.TaskVO]{
                Code: 409,
                Msg:  "该环境变量已被任务引用，请先在任务中删除引用或选择强制删除",
                Data: vo.ToTaskVOListFromModels(associatedTasks),
            },
        }, nil
        // 注意：这里返回 nil error，因为这是业务层面的 409，需要携带 data
        // HTTP 状态码需通过 huma 的方式设置
    }
    // ...
}
```

> **注意**：Huma 默认成功返回 200。若需返回 409 并携带响应体，需使用 `huma.WriteErr` 或自定义响应头。建议在 Output 中添加 `Status int` 字段并通过 Transformer 处理，或在 handler 中直接写入响应。

**替代方案**（推荐）：
```go
// 使用 huma.Resp 表示多种响应
type DeleteEnvVarOutput struct {
    Body    utils.HumaResponse[any]    // 200 成功
    Conflict utils.HumaResponse[[]*vo.TaskVO] // 409 冲突
}
```

#### 特殊接口保留清单

在 `api_routes.go` 中保留以下 Gin 原生路由注册：

```go
// 以下接口保留 Gin 原生处理，不迁移到 Huma
func initSpecialGinRoutes(api *gin.RouterGroup, c *Controllers) {
    // WebSocket
    api.GET("/terminal/ws", c.Terminal.HandleWebSocket)
    api.GET("/ws/events", c.SystemWS.HandleEvents)

    // SSE
    logs := api.Group("/logs")
    logs.GET("/sse", c.LogSSE.StreamLog)

    monitor := api.Group("/monitor")
    monitor.GET("/sse", c.Monitor.MonitorSSE)

    // 文件流
    files := api.Group("/files")
    files.GET("/download", c.File.DownloadFile)
    files.GET("/download-zip", c.File.DownloadZip)
    files.POST("/upload", c.File.UploadArchive)
    files.POST("/uploadfiles", c.File.UploadFiles)

    // Agent 相关（在 initAgentAPIRoutes 中保留）
    // interconnect/tunnel, report, proxy 保留
}
```

#### 执行约束

- 每个批次（controller）独立提交，便于回滚
- 保留原 Gin 方法直到该 controller 全部迁移完成并测试通过后，再删除
- 不修改 service 层代码
- 不修改 model 层代码（仅补全 vo 的 struct tag）

#### 验收清单（每批次）

- [ ] 该 controller 所有已迁移接口功能正常
- [ ] 该 controller 保留的 Gin 原生接口（WS/SSE/文件流）正常
- [ ] `go build ./...` 通过（Windows 环境须使用 `GOOS=linux GOARCH=amd64 go build ./...`）
- [ ] 无 swag 注解残留（已迁移部分）

#### 阶段 2 整体验收

- [ ] `/api/v1/openapi.json` 可访问，包含所有迁移接口
- [ ] 文档中 `securitySchemes` 包含 `CookieAuth`
- [ ] 所有特殊接口（WS/SSE/文件流/代理）正常工作
- [ ] 前端功能不受影响

---

### 阶段 3：前端适配与文档切换

**目标**：前端适配新响应码语义，切换文档生成方式。

#### 执行步骤

1. **修改前端 `request` 函数**（`web/src/api/index.ts`）：
   ```ts
   export async function request<T>(url: string, options?: RequestInit): Promise<T> {
     let res: Response
     try {
       res = await fetch(`${API_BASE_URL}${url}`, {
         ...options,
         credentials: 'include',
         headers: {
           'Content-Type': 'application/json',
           ...options?.headers
         }
       })
     } catch (err: any) {
       throw err
     }

     const json: ApiResponse<T> = await res.json()

     // 过渡期：双判断兼容
     if (res.status === 401 || json.code === 401) {
       window.location.href = BASE_URL + '/login'
       throw new Error(json.msg || '请先登录')
     }

     if (!res.ok || json.code !== 200) {
       throw new Error(json.msg || '请求失败')
     }

     return json.data
   }
   ```

2. **新增 OpenAPI 导出脚本** `cmd/export_openapi.go`（或 `scripts/export_openapi.go`）：
   ```go
   // 启动一个不监听端口的程序，仅创建 Huma 实例并导出 OpenAPI JSON
   // 输出到 docs/public/api-openapi.json 和 docs/public/open2api-openapi.json
   ```

3. **更新 Makefile**：
   ```makefile
   # 删除 swag 目标
   # 新增 openapi 目标
   openapi:
       @mkdir -p docs/public
       go run ./cmd/export-openapi
   ```

4. **更新文档渲染**（`docs/guide/api.md`）：
   - 修改 Scalar 配置，指向新的 JSON 路径
   - 或创建两个页面分别渲染 `/api` 和 `/open2api` 文档

5. **删除 swag 相关**：
   - 删除 `main.go` 顶部 swag 全局注解
   - `go mod tidy` 清理 `swaggo/swag` 依赖
   - 删除旧 `docs/public/swagger.json` / `swagger.yaml`

#### 执行约束

- 前端修改保持向后兼容（双判断）
- 文档导出脚本不依赖数据库连接（仅生成 schema）

#### 验收清单

- [ ] 前端所有功能正常（登录、任务管理、文件操作等）
- [ ] `make openapi` 生成两个 JSON 文件
- [ ] Scalar 可正常渲染新文档
- [ ] `go.mod` 不再包含 `swaggo/swag`
- [ ] `main.go` 无 swag 注解

---

### 阶段 4：清理与优化

**目标**：移除过渡代码，统一错误处理，完善文档。

#### 执行步骤

1. **删除旧 Gin 响应工具**：
   - 若所有 controller 已迁移，删除 `internal/utils/response.go` 中的 `Success`/`Error`/`BadRequest` 等
   - 保留 `HumaResponse` 和 `HumaError`

2. **删除旧 Gin controller 方法**：
   - 删除所有 `func (c *gin.Context)` 签名的旧方法
   - 删除 `api_routes.go` 中已无用的 `register*Routes` 函数

3. **统一错误处理**：
   - 利用 Huma 的 `Errors` 配置，定义统一错误响应模型
   - 确保所有 panic 通过 `GinRecovery` 中间件捕获并返回 500

4. **完善 OpenAPI 文档**：
   - 补充 `components.securitySchemes`
   - 为特殊接口（WS/SSE）手动补充 Operation 描述
   - 添加 `servers` 配置

5. **前端简化**（可选）：
   - 移除过渡期双判断，改为纯 `!res.ok` 判断

6. **更新** `CHANGELOG.md`

#### 验收清单

- [ ] 无 `utils.Success`/`utils.Error` 等 Gin 响应函数残留
- [ ] 无 `func(c *gin.Context)` 签名的旧 controller 方法残留
- [ ] 无 swag 注解残留
- [ ] `go build ./...` 通过（Windows 环境须使用 `GOOS=linux GOARCH=amd64 go build ./...`），无未使用导入
- [ ] `go vet ./...` 通过（Windows 环境同样适用，无需交叉编译参数）
- [ ] `CHANGELOG.md` 已更新

---

## 4. 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| Controller 签名重构量大 | 22 个文件，100+ 方法 | 阶段 2 分 controller 逐批迁移，每批独立提交 |
| 前端 401 判断逻辑变更 | 登录态失效处理 | 过渡期双判断 `!res.ok \|\| json.code !== 200` |
| `TravelProxyMiddleware` 依赖 `c.JSON` | 代理错误响应格式 | 该中间件作用于全局 gin.Engine，在 Huma handler 外包裹，不受影响 |
| Huma 对 `gin.H` 内联结构无法生成 schema | 文档缺失 data 字段结构 | 强制使用具名 struct（`vo.*`），消除所有 `gin.H` 返回 |
| `PaginationData` 泛型嵌套 | OpenAPI schema 复杂 | 用 Huma 泛型 `HumaResponse[PaginationData[T]]`，Huma v2 支持 |
| Cookie 鉴权与 OpenAPI securitySchemes | 文档无法体现 Cookie | 用 `CookieAuth (in: cookie)` 描述 |
| context 传递 gin context 值 | Huma handler 无法直接访问 `c.GetString("userID")` | 通过中间件将 `*gin.Context` 注入 `context.Context`，handler 中提取 |
| `DeleteEnvVar` 等需返回非 200 + 携带 data | Huma 默认成功仅 200 | 使用 `huma.WriteErr` 或自定义 Output 含多个响应字段 |

---

## 5. 工作量估算

| 阶段 | 预估工时 | 可并行 |
|------|----------|--------|
| 阶段 0 | 1-2 天 | 否（阻塞后续） |
| 阶段 1 | 3-5 天 | 是（与阶段 2 部分并行） |
| 阶段 2 | 5-8 天 | 是（按 controller 并行） |
| 阶段 3 | 2-3 天 | 是（前端与文档可并行） |
| 阶段 4 | 1-2 天 | 否 |
| **合计** | **12-20 天** | |

---

## 6. AI 执行指引

### 6.1 执行原则

1. **严格按阶段顺序**：阶段 0 → 1 → 2 → 3 → 4，不跳跃
2. **每阶段独立验收**：完成一阶段后对照验收清单逐项确认，再进入下一阶段
3. **小步提交**：阶段 2 内按 controller 批次提交，每批一个 commit
4. **不改 service/model 层**：仅补全 vo 的 struct tag，不改业务逻辑
5. **保留旧代码过渡**：新增 `*Huma` 方法与旧 `*gin.Context` 方法并存，全部迁移完成后再删除

### 6.2 单个 controller 迁移 checklist

对每个 controller，执行以下步骤：

- [ ] 1. 阅读该 controller 所有方法
- [ ] 2. 识别请求结构（body/query/path/header）
- [ ] 3. 识别响应结构（data 类型、是否有分页、是否有特殊状态码如 409）
- [ ] 4. 补全 VO struct tag（`description`）
- [ ] 5. 为每个方法创建 Huma handler（Input/Output/handler 函数）
- [ ] 6. 创建 `RegisterAPIRoutes(api huma.API)` 方法
- [ ] 7. 修改 `api_routes.go`，删除已迁移接口的 Gin 注册
- [ ] 8. 保留特殊接口（WS/SSE/文件流）的 Gin 注册
- [ ] 9. 移除 swag 注解
- [ ] 10. 编译验证 `go build ./...`（Windows 环境使用 `GOOS=linux GOARCH=amd64 go build ./...`）
- [ ] 11. 功能测试

### 6.3 常见模式参考

#### GET 列表 + 分页

```go
type GetListInput struct {
    Page     int    `query:"page" default:"1"`
    PageSize int    `query:"page_size" default:"10"`
    Name     string `query:"name"`
}
type GetListOutput struct {
    Body utils.HumaResponse[utils.PaginationData[[]*vo.SomeVO]]
}
func (ctrl *SomeController) GetListHuma(ctx context.Context, input *GetListInput) (*GetListOutput, error) {
    p := utils.Pagination{Page: input.Page, PageSize: input.PageSize}
    items, total := ctrl.svc.GetWithPagination(p.Page, p.PageSize, input.Name)
    return &GetListOutput{
        Body: utils.HumaResponse[utils.PaginationData[[]*vo.SomeVO]]{
            Code: 200, Msg: "success",
            Data: utils.PaginationData[[]*vo.SomeVO]{
                Data: vo.ToSomeVOList(items), Total: total,
                Page: p.Page, PageSize: p.PageSize,
            },
        },
    }, nil
}
```

#### GET 详情 by ID

```go
type GetByIDInput struct {
    ID string `path:"id" description:"ID"`
}
type GetByIDOutput struct {
    Body utils.HumaResponse[*vo.SomeVO]
}
```

#### POST 创建

```go
type CreateInput struct {
    Body vo.SomeCreateReq
}
type CreateOutput struct {
    Body utils.HumaResponse[*vo.SomeVO]
}
```

#### PUT 更新

```go
type UpdateInput struct {
    ID   string             `path:"id"`
    Body vo.SomeUpdateReq
}
type UpdateOutput struct {
    Body utils.HumaResponse[*vo.SomeVO]
}
```

#### DELETE

```go
type DeleteInput struct {
    ID string `path:"id"`
}
type DeleteOutput struct {
    Body utils.HumaResponse[any]
}
```

#### 需要用户上下文

```go
func (ctrl *SomeController) SomeHuma(ctx context.Context, input *SomeInput) (*SomeOutput, error) {
    c := getGinContext(ctx)  // 从 context 提取 *gin.Context
    userID := c.GetString("userID")
    // ...
}
```

### 6.4 验证命令

> **重要**：项目部署目标为 Linux。若在 Windows 上进行编译验证，必须使用交叉编译，目标平台为 `linux/amd64`，禁止生成 Windows 二进制。

#### Linux 环境验证

```bash
# 编译
go build ./...

# 静态检查
go vet ./...

# 启动服务测试
go run main.go serve

# 导出 OpenAPI 文档（阶段 3 后）
make openapi

# 检查文档
cat docs/public/api-openapi.json | python -m json.tool | head -50
cat docs/public/open2api-openapi.json | python -m json.tool | head -50
```

#### Windows 环境验证（交叉编译至 linux/amd64）

```powershell
# 编译（必须交叉编译至 linux/amd64）
$env:GOOS="linux"; $env:GOARCH="amd64"; go build ./...

# 静态检查（vet 不受 GOOS/GOARCH 影响，但建议保持一致）
$env:GOOS="linux"; $env:GOARCH="amd64"; go vet ./...

# 编译产出二进制（用于确认可产出 linux 二进制）
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o baihu-panel-linux-amd64 ./main.go

# 完成后还原环境变量（避免影响后续操作）
$env:GOOS=""; $env:GOARCH=""
```

> **注意**：Windows 环境下无法直接 `go run` 启动 linux 目标的服务进行运行时测试。运行时功能验证需在 Linux 环境（或 WSL/Docker）中进行。编译验证仅确认代码可正确构建为 linux/amd64 二进制。

#### Bash（Windows Git Bash / WSL）环境验证

```bash
# 编译（交叉编译至 linux/amd64）
GOOS=linux GOARCH=amd64 go build ./...

# 静态检查
GOOS=linux GOARCH=amd64 go vet ./...

# 编译产出二进制
GOOS=linux GOARCH=amd64 go build -o baihu-panel-linux-amd64 ./main.go
```

> WSL 环境下若 `uname -m` 显示 `x86_64`，可直接按 Linux 环境验证命令执行，无需交叉编译。

---

## 7. 附录

### 7.1 Huma 版本要求

- `github.com/danielgtaylor/huma/v2` v2.x（最新版）
- 需要 Go 1.21+（项目使用 Go 1.26，满足）

### 7.2 相关文件清单

**新增文件**：
- `internal/utils/huma_response.go`
- `internal/router/huma_setup.go`
- `internal/router/huma_register.go`（可选，或分散到各 controller）
- `cmd/export-openapi/main.go`（阶段 3）

**修改文件**：
- `internal/router/router.go`
- `internal/router/openapi.go`（重写）
- `internal/router/api_routes.go`（逐步删减）
- `internal/controllers/*.go`（追加 Huma handler）
- `internal/models/vo/*.go`（补全 struct tag）
- `web/src/api/index.ts`（阶段 3）
- `docs/guide/api.md`（阶段 3）
- `Makefile`（阶段 3）
- `main.go`（阶段 3，删除 swag 注解）
- `go.mod` / `go.sum`

**删除文件**（阶段 4）：
- `docs/public/swagger.json`
- `docs/public/swagger.yaml`

### 7.3 OpenAPI 版本

- 目标：OpenAPI 3.1.0
- Huma v2 默认生成 3.1.0
- 兼容 3.0.x 工具（Scalar、Swagger UI 等均支持 3.1）

### 7.4 安全方案配置

```json
{
  "components": {
    "securitySchemes": {
      "BearerAuth": {
        "type": "http",
        "scheme": "bearer",
        "bearerFormat": "JWT"
      },
      "CookieAuth": {
        "type": "apiKey",
        "in": "cookie",
        "name": "bh_token"
      },
      "NotifyTokenAuth": {
        "type": "apiKey",
        "in": "header",
        "name": "notify-token"
      }
    }
  }
}
```

- `/open2api` 接口使用 `BearerAuth`
- `/api/v1` 管理接口使用 `CookieAuth`
- `/api/v1/notify/send` 使用 `NotifyTokenAuth`
