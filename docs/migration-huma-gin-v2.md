# 迁移执行方案 v2：双 Huma 实例 + 双鉴权域

> v1 已完成 controller 业务接口的 Huma 化，但保留了过多 Gin 原生接口。
> v2 修订方向：**`/api/v1` 与 `/open2api/v1` 各一个 Huma 实例，各用各的 gin 鉴权中间件；
> 公开接口暂不纳入迁移范围，保留 Gin 原生处理。**

---

## 1. 核心原则

### 1.1 两个 Huma 实例，两个鉴权域

| Huma 实例 | 挂载前缀 | gin 鉴权中间件 | OpenAPI 文档 |
|-----------|----------|----------------|--------------|
| `APIV1Huma` | `/api/v1` | `AuthRequired` + `AdminRequired` | `api-openapi.json` |
| `Open2APIV1Huma` | `/open2api/v1` | `OpenapiRequired` | `open2api-openapi.json` |

- 鉴权**完全由 gin 中间件实现**，Huma 只负责路由注册与文档生成
- `prefixAdapter` 在 `Handle` 时把 gin 中间件 append 进 gin handler 链，Huma handler 不感知鉴权
- 两个实例互不干扰，各自生成独立 OpenAPI 文档

### 1.2 公开接口暂不迁移

以下接口**保留 Gin 原生处理，本次不纳入 v2 范围**：

```
/api/v1/ping
/api/v1/auth/login, /auth/login/otp, /auth/logout
/api/v1/settings/public
/api/v1/interconnect/tunnel    (WebSocket)
/api/v1/interconnect/report    (内部 Token，handler 内校验)
/api/v1/internal/*             (LocalhostOnly)
/api/agent/*                   (Agent 通信，独立鉴权域)
```

> 这些接口的 OpenAPI Operation 描述继续由 `huma_special_ops.go` 手动维护（或不维护，按需）。

### 1.3 v2 迁移范围

将 v1 保留在 Gin 的「`/api/v1` 鉴权接口」与「`/open2api/v1` 已迁移接口的同类补充」迁至对应 Huma 实例：

| 接口 | 当前归属 | v2 目标归属 | 鉴权 |
|------|----------|-------------|------|
| `/api/v1/auth/me`, `/auth/otp/*` | Gin (AuthRequired) | `APIV1Huma` | AuthRequired（普通用户） |
| `/api/v1/logs/sse`, `/monitor/sse` | Gin (Admin) | `APIV1Huma` (sse.Register) | AuthRequired+AdminRequired |
| `/api/v1/files/download*`, `/upload*` | Gin (Admin) | `APIV1Huma` | AuthRequired+AdminRequired |
| `/api/v1/settings/backup/download`, `/restore` | Gin (Admin) | `APIV1Huma` | AuthRequired+AdminRequired |
| `/api/v1/webui/upload` | Gin (Admin) | `APIV1Huma` | AuthRequired+AdminRequired |
| `/api/v1/agent/download` | Gin (Admin) | `APIV1Huma` | AuthRequired+AdminRequired |
| `/api/v1/notify/send` | Gin (NotifyTokenAuth) | `APIV1Huma` | NotifyTokenAuth |
| `/api/v1/interconnect/proxy/*` | Gin (Admin) | `APIV1Huma` (多方法) | AuthRequired+AdminRequired |

> `/api/agent/*`（heartbeat/tasks/report/download/ws）属于独立鉴权域，**不在 `/api/v1` 或 `/open2api/v1` 任一实例内**，本次不迁移。

---

## 2. 需要解决的关键问题

### 2.1 `/api/v1` 实例内存在两种鉴权级别

`APIV1Huma` 创建时统一传入 `AuthRequired + AdminRequired`，但 v2 要迁入的接口中有：

- **普通用户级**（仅需 `AuthRequired`）：`/auth/me`、`/auth/otp/*`
- **管理员级**（需 `AuthRequired + AdminRequired`）：其余所有接口
- **独立鉴权**（`NotifyTokenAuth`）：`/notify/send`

#### 解决方案：`prefixAdapter` 支持按 Operation 选择中间件

改造 `huma_setup.go`，让 `Handle` 时根据 `op.Path` 选择对应的 gin 中间件链：

```go
// mwSelector 根据Operation决定套用哪些 gin 中间件
type mwSelector func(op huma.Operation) []gin.HandlerFunc

func newHuma(engine *gin.Engine, prefix, title, version, desc string,
    selector mwSelector) huma.API {
    // ... 同 v1 ...
    return huma.NewAPI(config, &prefixAdapter{
        engine:   engine,
        prefix:   strings.TrimSuffix(prefix, "/"),
        selector: selector,
    })
}

func (a *prefixAdapter) Handle(op *huma.Operation, handler func(huma.Context)) {
    path := a.prefix + op.Path
    path = strings.ReplaceAll(path, "{", ":")
    path = strings.ReplaceAll(path, "}", "")

    var handlers []gin.HandlerFunc
    if !isHumaMetaPath(op.Path) && a.selector != nil {
        handlers = a.selector(op)
    }

    a.engine.Handle(op.Method, path, append(handlers, func(c *gin.Context) {
        utils.InjectGinContext(c)
        handler(humagin.NewContext(op, c))
    })...)
}
```

#### `/api/v1` 实例的 selector

```go
c.APIV1Huma = newHuma(router, "/api/v1", "Baihu Panel API", constant.Version,
    "内部管理 API。需通过登录后的 Cookie 会话进行鉴权。",
    func(op huma.Operation) []gin.HandlerFunc {
        p := op.Path
        switch {
        // 普通用户：仅 AuthRequired
        case p == "/auth/me", strings.HasPrefix(p, "/auth/otp/"):
            return []gin.HandlerFunc{middleware.AuthRequired()}

        // NotifyTokenAuth 独立鉴权
        case p == "/notify/send":
            return []gin.HandlerFunc{middleware.NotifyTokenAuth()}

        // 其余：AuthRequired + AdminRequired（含已迁移业务接口、SSE、文件流、代理）
        default:
            return []gin.HandlerFunc{middleware.AuthRequired(), middleware.AdminRequired()}
        }
    })
```

#### `/open2api/v1` 实例的 selector

```go
c.Open2APIV1Huma = newHuma(router, "/open2api/v1", "Baihu Panel OpenAPI", constant.Version,
    "对外开放 API。需通过 Bearer Token 进行鉴权。",
    func(op huma.Operation) []gin.HandlerFunc {
        return []gin.HandlerFunc{middleware.OpenapiRequired()}
    })
```

> `humagin` 自动注册的 `/openapi.json`、`/docs`、`/schemas/*` 经 `isHumaMetaPath` 跳过所有中间件。

---

## 3. 文件结构变更

```
internal/
├── router/
│   ├── router.go              (修改: newHuma 传入 selector)
│   ├── huma_setup.go          (修改: prefixAdapter.middleware → selector)
│   ├── huma_register.go       (修改: 新增 Auth/SSE/File/Proxy 注册调用)
│   ├── huma_special_ops.go    (精简: 删除已迁移接口的 Operation 描述)
│   ├── api_routes.go          (精简: 删除已迁移接口的 Gin 注册)
│   ├── huma_export.go         (修改: ExportOpenAPI 同步传入 selector)
│   ├── openapi.go             (不变)
│   └── register.go            (不变)
├── controllers/
│   ├── auth_controller.go     (追加 Huma handler: GetCurrentUser/OTP*)
│   ├── settings_controller.go  (追加 backup/download/restore Huma handler)
│   ├── file_controller.go     (追加 download/upload Huma handler)
│   ├── log_sse_controller.go   (重写为 sse.Register)
│   ├── monitor_controller.go  (追加 sse.Register)
│   ├── interconnect_controller.go (追加 proxy Huma handler)
│   └── notification_controller.go (追加 SendNotification Huma handler)
└── models/vo/
    ├── auth_vo.go             (新增: OTP 相关 VO)
    └── (其余 vo 补全 struct tag)
```

---

## 4. 执行阶段

### 阶段 5：基础设施改造

**目标**：让 `prefixAdapter` 支持按 Operation 选择 gin 中间件。

#### 执行步骤

1. **改造 `huma_setup.go`**：
   - `prefixAdapter.middleware []gin.HandlerFunc` → `selector mwSelector`
   - `newHuma` 签名：`middleware ...gin.HandlerFunc` → `selector mwSelector`
   - `Handle` 内调用 `selector(op)` 获取中间件链

2. **改造 `router.go`**：
   - `APIV1Huma` 创建时传入按 2.1 节的 selector
   - `Open2APIV1Huma` 创建时传入固定 `OpenapiRequired` selector

3. **改造 `huma_export.go`**：
   - `ExportOpenAPI` 内创建实例时同步传入 selector

4. **验证**：`go build ./... && go vet ./...` 通过，现有接口鉴权行为不变（selector 的 default 分支返回 `AuthRequired+AdminRequired`，与 v1 等价）

#### 验收清单

- [ ] `go build ./...` 通过
- [ ] `go vet ./...` 通过
- [ ] 现有 `/api/v1` 已迁移接口鉴权行为不变
- [ ] `/open2api/v1` 接口鉴权行为不变
- [ ] `make openapi` 仍可正常导出

---

### 阶段 6：迁移 Auth 普通用户接口

**目标**：将 `/api/v1/auth/me`、`/auth/otp/*` 迁入 `APIV1Huma`（普通用户鉴权）。

#### 迁移范围

| 接口 | 方法 | 鉴权 |
|------|------|------|
| `/auth/me` | GET | AuthRequired |
| `/auth/otp/status` | GET | AuthRequired |
| `/auth/otp/generate` | POST | AuthRequired |
| `/auth/otp/enable` | POST | AuthRequired |
| `/auth/otp/disable` | POST | AuthRequired |

#### 执行步骤

1. **新建 `internal/models/vo/auth_vo.go`**：
   - `OTPStatusResp`、`OTPGenerateResp` 等
   - 补全 `json` + `description` + `example` tag

2. **在 `auth_controller.go` 追加 Huma handler**：
   - `GetCurrentUserHuma`、`GetOTPStatusHuma`、`GenerateOTPHuma`、`EnableOTPHuma`、`DisableOTPHuma`
   - 复用原业务逻辑，`c.GetString("userID")` 改为 `utils.GetGinContext(ctx).GetString("userID")`

3. **新增 `AuthController.RegisterAPIAuthRoutes(api huma.API)`**：
   - 注册上述 5 个接口
   - Operation 的 `Security` 字段标注 `[{"CookieAuth": {}}]`

4. **在 `huma_register.go` 调用** `c.Auth.RegisterAPIAuthRoutes(api)`

5. **从 `api_routes.go` 删除** `initAuthorizedAPIRoutes` 中 `/auth/me`、`/auth/otp/*` 的 Gin 注册

6. **验证**：`/auth/me`、OTP 全流程功能正常

#### 验收清单

- [ ] 5 个接口功能正常
- [ ] OTP 启用/禁用流程正常
- [ ] `api-openapi.json` 包含这 5 个接口
- [ ] 文档中 `Security` 标注为 `CookieAuth`
- [ ] `go build ./...` 通过

---

### 阶段 7：迁移 SSE 接口

**目标**：用 Huma 原生 `sse.Register` 替换 Gin SSE。

#### 迁移范围

| 接口 | 方法 | 鉴权 | 事件类型 |
|------|------|------|----------|
| `/logs/sse` | GET | AuthRequired+AdminRequired | `message`（日志条目） |
| `/monitor/sse` | GET | AuthRequired+AdminRequired | `message`（监控数据） |

#### 执行步骤

1. **重写 `log_sse_controller.go`**：
   - 定义 SSE 事件 VO（如 `LogEventVO`）
   - 使用 `sse.Register` 注册：
     ```go
     sse.Register(api, huma.Operation{
         Method: http.MethodGet, Path: "/logs/sse",
         OperationID: "streamLogs", Summary: "实时日志流（SSE）",
         Security: []map[string][]string{{"CookieAuth": {}}},
     }, map[string]any{
         "message": vo.LogEventVO{},
     }, lc.StreamLogSSE)
     ```
   - `StreamLogSSE(ctx, input, send)`：复用原 `StreamLog` 的订阅逻辑，将每条日志通过 `send.Data(...)` 推送

2. **重写 `monitor_controller.go` 的 SSE 部分**：
   - 同上，`monitorSSE` 改为 `sse.Register`

3. **从 `api_routes.go` 删除** `registerLogSSERoutes`、`registerMonitorSSERoutes`

4. **从 `huma_special_ops.go` 删除** logs/sse、monitor/sse 的 Operation 描述

5. **验证**：前端日志流、监控流实时推送正常

#### 验收清单

- [ ] `/logs/sse` 推送正常，事件格式与原一致
- [ ] `/monitor/sse` 推送正常
- [ ] 断开连接后 goroutine 正确退出
- [ ] `api-openapi.json` 中 SSE 接口含 `text/event-stream` 响应描述
- [ ] `go build ./...` 通过

---

### 阶段 8：迁移文件流与代理接口

**目标**：文件下载/上传、备份恢复、WebUI 上传、Agent 下载、代理透传迁至 Huma。

#### 迁移范围

| 接口 | 方法 | 鉴权 | 技术点 |
|------|------|------|--------|
| `/files/download` | GET | Admin | `huma.StreamResponse` + `BodyWriter` |
| `/files/download-zip` | GET | Admin | 同上，Zip 流 |
| `/files/upload` | POST | Admin | `*huma.FormFile` |
| `/files/uploadfiles` | POST | Admin | `[]*huma.FormFile` |
| `/settings/backup/download` | GET | Admin | 文件流 |
| `/settings/restore` | POST | Admin | `*huma.FormFile` |
| `/webui/upload` | POST | Admin | `*huma.FormFile` |
| `/agent/download` | GET | Admin | 文件流 |
| `/interconnect/proxy/{node_id}/{path}` | 多方法 | Admin | handler 内复用原 ProxyRequest 逻辑 |

#### 执行步骤

1. **文件下载类**：Output 用 `*huma.StreamResponse`
   ```go
   func (fc *FileController) DownloadFileHuma(ctx context.Context, input *DownloadFileInput) (*huma.StreamResponse, error) {
       // ... 业务逻辑获取文件路径 ...
       return &huma.StreamResponse{
           Body: func(hctx huma.Context) {
               hctx.SetHeader("Content-Type", "application/octet-stream")
               hctx.SetHeader("Content-Disposition", `attachment; filename="..."`)
               io.Copy(hctx.BodyWriter(), file)
           },
       }, nil
   }
   ```

2. **文件上传类**：Input 用 `*huma.FormFile` / `[]*huma.FormFile`
   ```go
   type UploadArchiveInput struct {
       RawBody *huma.FormFile
       Path    string `form:"path"`
   }
   ```

3. **代理接口**：注册 GET/POST/PUT/DELETE/PATCH 5 个 Operation（OpenAPI 不支持 ANY），
   handler 内通过 `utils.GetGinContext(ctx)` 取 `*gin.Context`，复用原 `ProxyRequest` 逻辑

4. **从 `api_routes.go` 删除** `registerFileSpecialRoutes`、`registerSettingsSpecialRoutes`、
   `registerAgentDownloadRoutes`、`registerWebUIUploadRoutes`、`registerInterconnectProxyRoutes`

5. **从 `huma_special_ops.go` 删除** 这些接口的 Operation 描述

6. **验证**：文件下载/上传、备份恢复、WebUI 上传、Agent 下载、代理透传功能正常

#### 验收清单

- [ ] 所有文件流接口功能正常
- [ ] 大文件下载/上传不 OOM（流式处理）
- [ ] 代理接口支持 GET/POST/PUT/DELETE/PATCH
- [ ] `api-openapi.json` 包含这些接口，含正确的 multipart/form-data 描述
- [ ] `go build ./...` 通过

---

### 阶段 9：迁移 NotifyToken 接口

**目标**：将 `/api/v1/notify/send` 迁入 `APIV1Huma`（NotifyTokenAuth 鉴权）。

#### 执行步骤

1. **在 `notification_controller.go` 追加 `SendNotificationHuma`**

2. **注册时 selector 识别 `/notify/send` 套用 `NotifyTokenAuth`**（已在阶段 5 selector 中配置）

3. **从 `api_routes.go` 删除** `notifyAPI` 部分

4. **从 `huma_special_ops.go` 删除** `/notify/send` 的 Operation 描述

#### 验收清单

- [ ] `/notify/send` 功能正常
- [ ] 鉴权使用 `notify-token` header
- [ ] `go build ./...` 通过

---

### 阶段 10：清理与文档收尾

**目标**：删除过渡代码，精简特殊 Operation 描述，确保文档完整。

#### 执行步骤

1. **精简 `huma_special_ops.go`**：仅保留公开接口与 WebSocket 接口的 Operation（若需要文档覆盖公开接口）：
   - `/ping`、`/auth/login*`、`/logout`、`/settings/public`、`/interconnect/tunnel`、`/interconnect/report`、`/internal/*`
   - `/terminal/ws`、`/ws/events`、`/agent/ws`

2. **精简 `api_routes.go`**：仅保留公开接口与 4 个 WebSocket 路由

3. **删除 `internal/middleware/auth.go` 的 `SwaggerAuth`**（已无引用，死代码）

4. **清理 `internal/models/vo/task_vo.go` 的 `swaggertype:"string"` tag**（swag 已移除，Huma 不识别）

5. **全量文档覆盖验证**：
   ```bash
   make openapi
   python3 -c "
   import json
   d = json.load(open('docs/public/api-openapi.json'))
   paths = sorted(d.get('paths',{}).keys())
   print('api paths:', len(paths))
   for p in paths: print(' ', p)
   "
   ```

6. **更新 `CHANGELOG.md`**

#### 验收清单

- [ ] `api_routes.go` 仅含公开接口 + 4 个 WS 路由
- [ ] `huma_special_ops.go` 仅含公开接口 + 4 个 WS Operation（或按需精简）
- [ ] `SwaggerAuth` 死代码已删
- [ ] `swaggertype` tag 残留已清
- [ ] `api-openapi.json` 路径数 ≥ 130（v1 为 117，新增约 15+ 接口）
- [ ] `open2api-openapi.json` 不变（14 paths）
- [ ] `go build ./...` 通过
- [ ] `go vet ./...` 通过
- [ ] `CHANGELOG.md` 已更新

---

## 5. 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| SSE 迁移后事件格式变化 | 前端 EventSource 解析失败 | 严格保持 `data:` 后 JSON 与原 `{code,data,msg}` 一致；迁移后对比抓包 |
| 文件流 `BodyWriter` 在 humagin 适配器下是否支持 `http.Flusher` | 大文件下载卡死 | humagin 适配器 `BodyWriter` 返回 `gin.ResponseWriter`，实现 `http.Flusher`，已验证 |
| `prefixAdapter` selector 误判 | 鉴权缺失或过度 | selector 以路径前缀匹配，新增接口必须显式归类；增加单元测试覆盖路径→中间件映射 |
| `interconnect/report` 无鉴权但内部校验 Token | 不在 v2 范围，保留 Gin | 无需处理 |
| `/api/agent/*` 独立鉴权域 | 不在 v2 范围，保留 Gin | 无需处理 |
| 代理接口 ANY 方法 | OpenAPI 不支持 ANY | 注册 GET/POST/PUT/DELETE/PATCH 5 个 Operation |

---

## 6. 工作量估算

| 阶段 | 预估工时 | 依赖 |
|------|----------|------|
| 阶段 5：基础设施改造 | 0.5 天 | 无 |
| 阶段 6：Auth 普通用户接口 | 0.5 天 | 阶段 5 |
| 阶段 7：SSE | 1 天 | 阶段 5 |
| 阶段 8：文件流与代理 | 1.5 天 | 阶段 5 |
| 阶段 9：NotifyToken 接口 | 0.5 天 | 阶段 5 |
| 阶段 10：清理与文档收尾 | 0.5 天 | 阶段 6-9 |
| **合计** | **4.5 天** | |

> 阶段 6-9 可部分并行（不同 controller 互不依赖），但阶段 5 必须先完成。

---

## 7. 不在 v2 范围内的接口（保留 Gin）

以下接口**本次不迁移**，保留 Gin 原生处理：

| 接口 | 原因 |
|------|------|
| `/api/v1/ping` | 公开接口，无鉴权 |
| `/api/v1/auth/login`, `/auth/login/otp`, `/logout` | 公开接口，无鉴权 |
| `/api/v1/settings/public` | 公开接口，无鉴权 |
| `/api/v1/interconnect/tunnel` | WebSocket |
| `/api/v1/interconnect/report` | 内部 Token，handler 内校验 |
| `/api/v1/internal/*` | LocalhostOnly，独立鉴权 |
| `/api/agent/*` | 独立鉴权域（Agent Token） |
| `/api/v1/terminal/ws` | WebSocket |
| `/api/v1/ws/events` | WebSocket |
| `/api/v1/agent/ws` | WebSocket |

> 这些接口的 OpenAPI 文档覆盖问题，由 `huma_special_ops.go` 手动维护（可选）。
