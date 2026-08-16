# 更新日志 (v1.1.26)

### 2026.08.17 - 通知内容统一与 Options SDK 支持、SSE 实时流状态帧协议、日志全屏删除修复与 Go 1.26 升级

### 2026.08.16 - Gin → Huma 迁移收尾（阶段 4：清理与优化）

**♻️ 清理与优化**
* **删除旧 Gin 响应工具 (Clean)**：移除了 `internal/utils/response.go` 中的 `Success`/`Error`/`BadRequest`/`Unauthorized`/`Forbidden`/`NotFound`/`TooManyRequests`/`ServerError` 等旧响应函数，以及 `pagination.go` 中旧的 `ParsePagination`/`PaginatedResponse`，仅保留 `Response`/`Pagination` 等被 Huma 代码复用的类型。保留的 Gin 特殊接口（Auth、WS/SSE、文件流、Agent 等）改为内联 JSON 响应。
* **错误响应统一 (Unify)**：保留的 Gin 特殊接口错误响应不再固定返回 HTTP 200，改为与 Huma 迁移接口一致的真实 HTTP 状态码（400/401/403/404/429/500）；所有 panic 由 `GinRecovery` 中间件统一捕获并返回 500。
* **删除旧 Gin controller 方法 (Clean)**：清理了 Tag、Dashboard、AppLog、Script、Env、Dependency、Mise、Log、Data 等 9 个已完全迁移 controller 的全部旧 Gin 方法，以及 Settings、Task、Interconnect、File、Terminal、Monitor、WebUI、Agent、Notification、Executor 中已迁移至 Huma 的旧方法；每个 controller 仅保留 struct 与构造函数。
* **OpenAPI 文档完善 (Docs)**：为保留 Gin 原生的特殊接口（WebSocket / SSE / 文件流 / 代理 / 隧道等 17 个）手动补充 OpenAPI Operation 描述，并添加 `servers` 配置，使 `/api/v1/openapi.json` 覆盖全部公开接口。
* **前端请求判断简化 (Simplify)**：前端请求封装移除过渡期 `json.code !== 200` 双判断，统一改为 `res.ok` 判断（保留登录页 401 不跳转的特殊处理）。

### 2026.08.10 - 企业微信应用通知、备份恢复自动调度刷新与安全漏洞修复

🎉 **新增与优化**
* **通知内容统一与 Options SDK 支持 (New)**：后端全面统一通知内容字段为 `content`（同时兼容历史旧版本 `text` 入参），并新增 `format` 格式定义（`text`/`markdown`/`html`）（#161）；Node.js (`builtin/nodejs/notify.js`) 及 Python (`builtin/python/baihu/notify.py`) 内置 SDK 均统一采用 Options 模式设计（`notify(title, content, options)`），支持多渠道及富文本渲染，且保持完全向下兼容。
* **日志 SSE Stream-as-State 协议 (New)**：重构了任务执行实时日志 SSE 通信机制，引入结构化 `finish` 帧实现任务状态与实时日志流在前端的强一致性联动同步，解决了并发场景下的执行状态延迟和刷新不及时问题。

**✨ 修复与改进**
* **历史日志全屏删除修复 (Fix)**：修复了执行历史中全屏日志查看弹窗（`LogViewer.vue`）因未向外层透传 `@delete` 事件导致删除历史日志不生效的缺陷（#157）。
* **调度器稳定性与锁优化 (Fix)**：修复了取消订阅日志流时偶发的 channel 重复关闭 panic；对 cron 触发读取 scheduler 调度器指针增加读锁保护；优化了重载配置时的锁范围以避免潜在死锁。
* **运行环境与依赖升级**：全面升级 Go 运行时至 1.26，并将两步验证 (`otp`) 升级为直接依赖。

> 💡 **提示**：出于安全及环境隔离考虑，推荐使用 Docker/Compose 部署方式。[镜像地址](https://github.com/engigu/baihu-panel/pkgs/container/baihu)

### 🐳 方式一：Docker 部署 (推荐)
[部署文档](https://github.com/engigu/baihu-panel?tab=readme-ov-file#%E5%BF%AB%E9%80%9F%E9%83%A8%E7%BD%B2)

---

### 🚀 方式二：单文件部署 (Linux / Windows)
从当前 Release 的附件中下载对应架构和平台的部署压缩包（Linux 为 `.tar.gz`，Windows 为 `.zip`）。

#### 🐧 Linux 平台

**1. 安装前置依赖 `mise`**

单文件直接运行依赖宿主机系统环境，请务必先安装 [mise](https://mise.jdx.dev/getting-started.html) 供任务调度及环境管理使用：

```bash
curl https://mise.run | sh
export PATH="~/.local/share/mise/bin:~/.local/share/mise/shims:$PATH"
```

**2. 运行面板**

```bash
tar -xzvf baihu-linux-amd64.tar.gz
chmod +x baihu-linux-amd64
./baihu-linux-amd64 server
```

#### 🪟 Windows 平台

**1. 安装前置依赖**

* **安装 `mise`**（用于统一依赖和运行时环境管理）：

  在 PowerShell 中运行以下命令使用 `winget` 安装：
  ```powershell
  winget install jdx.mise
  ```

* **安装 `pwsh`**（PowerShell 7.6+，用于执行后台任务）：

  白虎面板在 Windows 下运行任务和工具链强依赖 PowerShell 7+。请参考 [微软官方 PowerShell 安装文档](https://learn.microsoft.com/zh-cn/powershell/scripting/install/install-powershell-on-windows?view=powershell-7.6) 安装，或通过 `winget` 快捷安装：
  ```powershell
  winget install Microsoft.PowerShell
  ```

**2. 运行面板**

解压下载好的 `.zip` 压缩包，进入解压目录并打开 PowerShell，运行：

```powershell
.\baihu.exe server
```

---

**访问面板：**
* 启动后访问：`http://localhost:8052`
* **默认账号**：用户名 `admin`，密码见面板首次启动时的控制台日志。
