<div align="center">

# TokenBridge

**本地 AI 网关 · 多提供商调度中枢 · 可观测的模型访问控制平面**

[![Go](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![React](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=111)](https://react.dev/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.6-3178C6?logo=typescript&logoColor=white)](https://www.typescriptlang.org/)
[![Vite](https://img.shields.io/badge/Vite-5.4-646CFF?logo=vite&logoColor=white)](https://vitejs.dev/)
[![Wails](https://img.shields.io/badge/Wails-2.12-DF0000)](https://wails.io/)
[![SQLite](https://img.shields.io/badge/SQLite-embedded-003B57?logo=sqlite&logoColor=white)](https://sqlite.org/)
[![Build](https://img.shields.io/badge/CI-GitHub%20Actions-2088FF?logo=githubactions&logoColor=white)](.github/workflows/build-desktop.yml)

**一个本地入口，统一接入多个 AI Provider；一套控制台，管理路由、密钥、监控、日志与故障转移。**

[核心特性](#核心特性) · [系统架构](#系统架构) · [快速开始](#快速开始) · [配置说明](#配置说明) · [API-参考](#api-参考) · [贡献规范](#贡献规范)

</div>

---

## 项目简介

TokenBridge 是一个面向本地部署、团队内网和私有化场景的 **AI 网关管理平台**。它将多 Provider 接入、本地密钥分发、模型路由、请求转发、故障转移、调用日志和用量分析收束到统一控制面，让应用只需要访问一个本地兼容接口，即可获得可治理、可观测、可回退的 AI 访问能力。

它不是一个简单的 HTTP 代理，而是一个具备生产化边界的本地 AI 控制平面：

- 将 OpenAI-compatible、Anthropic-compatible 和自定义 Provider 统一接入本地网关；
- 通过本地访问密钥隔离不同工具、项目或团队成员的调用权限；
- 按模型、通配规则、Provider 优先级、Provider Chain 和 Fallback Chain 做请求调度；
- 在上游限流、超时、网络错误或 5xx 异常时自动尝试备用 Provider；
- 实时记录 Trace、延迟、状态码、失败原因、备用切换链路和用量数据；
- 提供 React + TypeScript 监控后台，面向日常运维、排障和容量分析。

> **TokenBridge = Local AI Gateway + Routing Control Plane + Observability Console + Desktop-ready Distribution.**

## 核心特性

### 网关能力

- **OpenAI 兼容接口**：提供 `/v1/chat/completions`，支持非流式请求与最小 SSE 流式透传。
- **Claude 兼容接口**：提供 `/v1/messages`，复用本地鉴权、路由决策与请求日志链路。
- **多 Provider 管理**：维护 Provider 名称、类型、Base URL、Organization、模型列表、RPM/TPM、优先级和启用状态。
- **本地密钥鉴权**：客户端只持有 Local Key，避免上游 Provider Key 在多个应用中扩散。
- **模型别名映射**：将业务侧模型名映射到真实上游模型名，降低调用方改造成本。
- **规则化路由**：支持模型通配匹配、Provider Chain、Fallback Chain 和路由模拟。
- **自动故障转移**：主 Provider 遇到 `429`、`5xx` 或网络类可重试错误时，按备用链路尝试切换。
- **请求 Trace**：成功请求返回 `X-Request-Trace-Id`，日志中保留请求模型、实际模型、Provider 与 Fallback 信息。

### 管理与可观测性

- **Dashboard 总览**：总请求、成功率、7 日费用趋势、失败请求、备用切换次数、平均延迟、Provider 数量。
- **Provider 健康状态**：汇总 Provider 状态、基础健康信息和异常提示。
- **请求延迟监控**：记录请求级 `latency_ms`，并在 Dashboard / Logs 中展示。
- **失败告警**：对近期失败请求、非 2xx 状态码和异常 Provider 生成告警卡片。
- **备用切换分析**：统计 Fallback 发生次数、趋势和每条请求的备用链路。
- **日志检索**：按 Provider、API Format、状态、Fallback、时间范围、Trace ID 和文本搜索筛选。
- **CSV 导出**：按当前筛选条件导出请求日志，包含 Trace、Provider、状态、耗时、Fallback、模型与错误信息。
- **用量分析**：按 Provider、模型和 Local Key 查看请求量、Token 与费用统计。

### 桌面与交付

- **浏览器 / 托盘模式**：Windows 正式 exe 隐藏控制台，常驻系统托盘，启动后自动打开管理后台。
- **单实例运行**：通过 Windows Mutex 避免重复启动；二次启动会唤起已有实例的管理后台。
- **Wails 桌面版**：内嵌 WebView、无边框窗口、Mica / 半透明效果、托盘菜单、窗口状态恢复、桌面自检。
- **前端资源嵌入**：Vite 构建产物同步到 Go embed 资源，最终二进制可独立分发。
- **CI 双端构建**：GitHub Actions 支持 Windows 与 macOS 桌面产物构建。

## 技术栈

| 层级 | 技术选型 |
| --- | --- |
| 后端服务 | Go 1.22, `chi`, `zerolog`, `viper` |
| 数据存储 | SQLite, GORM, `github.com/glebarez/sqlite` |
| 网关协议 | OpenAI Chat Completions, Anthropic Claude Messages, SSE pass-through |
| 管理后台 | React 18, TypeScript 5.6, Vite 5, React Router |
| 可视化与交互 | Recharts, Lucide React, Framer Motion, Zustand |
| 桌面运行时 | Wails v2, WebView2, System Tray |
| 打包分发 | PowerShell, Go embed, Windows resource metadata |
| CI/CD | GitHub Actions |

## 系统架构

```text
┌──────────────────────────────────────────────────────────────────────┐
│                            Client Layer                              │
│       OpenAI SDK / Claude SDK / curl / internal tools / agents        │
└───────────────────────────────┬──────────────────────────────────────┘
                                │
                                ▼
┌──────────────────────────────────────────────────────────────────────┐
│                         TokenBridge API                             │
│  /v1/chat/completions      /v1/messages        /admin/api/*           │
│  Local Key Auth            Request Trace       Embedded Admin UI      │
└───────────────┬───────────────────────┬──────────────────────────────┘
                │                       │
                ▼                       ▼
┌──────────────────────────┐  ┌────────────────────────────────────────┐
│ Routing Control Plane    │  │ Admin / Observability Plane             │
│ - Model alias            │  │ - Dashboard aggregation                 │
│ - Wildcard rules         │  │ - Provider health summary               │
│ - Provider priority      │  │ - Request logs / CSV export             │
│ - Fallback chain         │  │ - Usage analytics / alerts              │
└───────────────┬──────────┘  └──────────────────┬─────────────────────┘
                │                                │
                ▼                                ▼
┌──────────────────────────┐  ┌────────────────────────────────────────┐
│ Provider Adapter Layer   │  │ SQLite Persistence                      │
│ - OpenAI-compatible      │  │ providers / local_keys / routing_rules  │
│ - Anthropic-compatible   │  │ aliases / usage_records / request_logs  │
│ - Custom base URL        │  │ settings                                │
└───────────────┬──────────┘  └────────────────────────────────────────┘
                │
                ▼
┌──────────────────────────────────────────────────────────────────────┐
│                         Upstream Providers                           │
│      OpenAI / Anthropic / DeepSeek / custom OpenAI-compatible APIs    │
└──────────────────────────────────────────────────────────────────────┘
```

### 核心模块

| 模块 | 路径 | 职责 |
| --- | --- | --- |
| 应用装配 | `internal/app/app.go` | 解析运行路径、加载配置、打开 SQLite、初始化服务并挂载 Router。 |
| HTTP 路由 | `internal/server/router.go` | 注册网关接口、Admin API、Health Check 和内嵌管理后台。 |
| OpenAI 转发 | `internal/server/chat.go` | 处理 `/v1/chat/completions`，完成鉴权、路由、Fallback、错误映射和日志记录。 |
| Claude 转发 | `internal/server/claude.go` | 处理 `/v1/messages`，复用 Local Key 与 Routing 决策链路。 |
| 流式转发 | `internal/server/chat_stream.go`, `internal/server/claude_stream.go` | 对上游 SSE 响应做最小透传。 |
| Provider 服务 | `internal/provider/service.go` | Provider CRUD、优先级排序、连接测试、模型发现和按模型解析 Provider。 |
| 路由引擎 | `internal/routing/service.go` | 模型别名、通配规则匹配、Provider Chain、Fallback Chain 和路由模拟。 |
| Admin 聚合 | `internal/admin/service.go` | 汇总 Overview、Dashboard、Analytics、Provider Health、Alerts、热点模型和 Provider 排行。 |
| 请求日志 | `internal/requestlog/service.go` | 查询、筛选、统计、Fallback 识别、失败趋势和 CSV 导出。 |
| 用量统计 | `internal/usage/service.go` | 记录请求用量，生成 Provider / Model / Key 维度分析。 |
| 浏览器版入口 | `cmd/tokenbridge/main.go` | HTTP Server、Windows 托盘、单实例检查、自动打开管理后台。 |
| 桌面版入口 | `main.go`, `internal/desktop/` | `main.go` 保留 Wails bootstrap；桌面窗口、SPA Proxy、桌面 Bindings、托盘菜单、自检与状态恢复放在 `internal/desktop/`。 |

## Dashboard 监控设计

管理后台位于 `web/admin`，采用 React + TypeScript 构建。`DashboardPage.tsx` 通过 `/admin/api/dashboard` 获取真实运行数据，核心关注四类生产指标：

| 指标域 | 数据来源 | 展示目的 |
| --- | --- | --- |
| Provider 健康状态 | `provider_health` | 识别异常、禁用或不可用 Provider。 |
| 请求延迟 | `recent_logs.latency_ms`, `log_stats.avg_latency_ms` | 观察平均耗时与最近请求链路质量。 |
| 失败告警 | `log_stats.failures`, `recent_logs.status_code`, `provider_health.status` | 对失败请求、非 2xx 响应和异常 Provider 生成告警。 |
| 备用切换 | `log_stats.fallbacks`, `failure_trend.fallbacks`, request metadata | 追踪主链路失效后的备用 Provider 尝试情况。 |

Dashboard 目前包含：

- KPI Cards：总请求、累计费用、失败请求、平均延迟；
- 费用趋势图：最近 7 天成本走势；
- 最近 Alert：失败请求、Provider 异常、非 2xx 请求；
- 实时链路条：最近请求路径、Provider 和延迟点位；
- 失败与备用切换数据：由后端 `requestlog.Service` 聚合后暴露。

## 管理后台页面

| 页面 | 路由 | 能力 |
| --- | --- | --- |
| Dashboard | `/dashboard` | KPI、趋势图、异常告警、实时链路、Provider 健康状态。 |
| Providers | `/providers` | Provider 配置、启停、优先级、模型列表、连接测试、模型发现。 |
| Keys | `/keys` | Local Key 创建、权限、预算、过期、吊销、轮换和延期。 |
| Routing | `/routing` | 路由规则、模型别名、Provider Chain、Fallback Chain 和路由模拟。 |
| Analytics | `/analytics` | 按 Provider、Model、Local Key 分析用量与请求表现。 |
| Logs | `/logs` | Trace 检索、失败分析、Fallback 筛选、高级过滤和 CSV 导出。 |
| Settings | `/settings` | 运行配置、备份、分发和系统设置。 |

## 快速开始

### 环境要求

| 依赖 | 版本 | 用途 |
| --- | --- | --- |
| Go | `1.22+` | 后端编译 |
| Node.js | `20+` | 前端构建 |
| npm | `10+` | 前端包管理 |
| Wails CLI | `v2.12` | 桌面版构建（仅桌面版需要） |

### 1. 本地开发 — 克隆与安装

```bash
# 克隆仓库
git clone <your-repo-url> tokenbridge
cd tokenbridge

# 安装 Go 依赖
go mod download

# 安装前端依赖
cd web/admin
npm install
cd ../..

# 安装 Wails CLI（仅桌面版开发需要）
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
```

### 2. 浏览器版启动（页面端）

浏览器版以 HTTP Server 方式运行，通过浏览器访问管理后台。

```bash
# 构建前端资源并嵌入 Go
cd web/admin
npm run build
cd ../..

# 启动服务
go run ./cmd/tokenbridge
```

启动后访问：

```text
http://127.0.0.1:18743/admin
```

开发模式下可分别启动前后端：

```bash
# 终端 1：启动后端
go run ./cmd/tokenbridge

# 终端 2：启动前端开发服务器（热更新）
cd web/admin
npm run dev
```

前端开发服务器默认运行在 `http://127.0.0.1:5174`，API 请求会代理到后端。

### 3. 桌面版启动（Wails）

桌面版以内嵌 WebView 窗口运行，无需浏览器。

```bash
# 开发模式（热更新）
wails dev

# 或手动构建前端后运行
cd web/admin
npm run build:wails
cd ../..
wails dev
```

`wails dev` 会自动构建前端、生成 Go bindings 并启动桌面窗口。

### 4. Windows 打包

```powershell
# 浏览器 / 托盘版便携包
powershell -File build/package.ps1

# Wails 桌面版
powershell -File build/desktop.ps1
```

便携包输出到 `build/portable/TokenBridge/`，包含：
- `tokenbridge.exe`（Windows GUI 模式，常驻托盘）
- `config.yaml`（配置模板）

桌面版输出到 `build/bin/TokenBridge.exe`（Wails 原生窗口）。

### 5. macOS 打包

```bash
# 构建前端
cd web/admin
npm run build:wails
cd ../..

# Wails 桌面版
wails build -platform darwin/universal -o TokenBridge.app
```

输出到 `build/bin/TokenBridge.app`。

如需生成 DMG：

```bash
# 使用 create-dmg（需先 brew install create-dmg）
create-dmg build/bin/TokenBridge.app build/bin/
```

## 构建与分发

### GitHub Actions

`.github/workflows/build-desktop.yml` 支持：

- Windows：输出 `TokenBridge.exe`
- macOS：输出 `TokenBridge.app`

触发方式：

- 推送 `v*` tag；
- 手动 `workflow_dispatch`。

## 配置说明

配置模板位于 `configs/config.example.yaml`。

```yaml
server:
  host: "127.0.0.1"
  port: 18743
  admin_path: "/admin"
  auto_open_admin: true
  prefer_browser: "chrome"
  read_timeout: 15
  write_timeout: 120
  idle_timeout: 120

proxy:
  request_timeout: 120
  stream_timeout: 300
  max_retries: 2
  retry_delay_ms: 500

security:
  api_key_encryption: "aes-256-gcm"
  encryption_key_file: ".secret"
  cors_enabled: false
  allowed_origins: []

logging:
  level: "standard"
  retention_days: 30
  max_log_size_mb: 500
  log_prompts: false

database:
  path: "./data/tokenbridge.db"
  wal_mode: true
  auto_vacuum: true
  backup_interval: "24h"

routing:
  default_strategy: priority
  fallback:
    enabled: true
    max_retries: 2
    cooldown_seconds: 60
    retry_on:
      - rate_limit
      - server_error
      - timeout

providers: []
local_keys: []
```

### 关键配置项

| 配置 | 说明 |
| --- | --- |
| `server.host` / `server.port` | 本地监听地址与端口。 |
| `server.admin_path` | 管理后台路径，默认 `/admin`。 |
| `server.auto_open_admin` | 服务启动后是否自动打开管理后台。 |
| `server.prefer_browser` | Windows 下优先尝试 Chrome，失败后回退默认浏览器。 |
| `proxy.request_timeout` | 非流式请求上游超时时间。 |
| `proxy.stream_timeout` | 流式请求上游超时时间。 |
| `security.allowed_origins` | CORS 允许来源列表。 |
| `database.path` | SQLite 数据库路径，运行时会解析到用户数据目录。 |
| `routing.default_strategy` | 默认路由策略，目前以 Provider 优先级和规则链路为核心。 |
| `routing.fallback.*` | 备用切换策略元信息，用于描述可重试错误、冷却时间和重试上限。 |

### 环境变量

| 变量 | 说明 |
| --- | --- |
| `TB_CONFIG` | 指定配置文件路径。 |
| `TB_CONSOLE_MODE=1` | 强制 Windows 正式 exe 使用控制台模式运行，便于排障。 |

## API 参考

### Health Check

```http
GET /health
```

返回服务状态与当前时间。

### OpenAI-compatible Chat Completions

```http
POST /v1/chat/completions
Authorization: Bearer <local-key>
Content-Type: application/json
```

```json
{
  "model": "gpt-4o-mini",
  "stream": false,
  "messages": [
    { "role": "user", "content": "Hello from TokenBridge" }
  ]
}
```

请求链路：

1. 校验 Local Key；
2. 校验模型和消息体；
3. 解析模型别名与路由规则；
4. 选择主 Provider；
5. 转发到上游 `/v1/chat/completions`；
6. 遇到可重试错误时尝试 Fallback Chain；
7. 写入 usage 与 request log；
8. 返回 `X-Request-Trace-Id`。

### Claude-compatible Messages

```http
POST /v1/messages
Authorization: Bearer <local-key>
Content-Type: application/json
```

```json
{
  "model": "claude-sonnet-4",
  "max_tokens": 1024,
  "messages": [
    { "role": "user", "content": "Hello from TokenBridge" }
  ]
}
```

### Admin APIs

| Endpoint | 说明 |
| --- | --- |
| `GET /admin/api/dashboard` | Dashboard KPI、趋势、Provider Health、最近日志和告警。 |
| `GET /admin/api/analytics?days=7` | 用量统计与维度拆分。 |
| `GET /admin/api/logs` | 请求日志列表与筛选。 |
| `GET /admin/api/logs/export` | 导出 CSV。 |
| `GET /admin/api/providers` | Provider 列表。 |
| `POST /admin/api/providers/{id}/test` | Provider 连接测试。 |
| `POST /admin/api/providers/{id}/discover-models` | 模型发现。 |
| `GET /admin/api/keys` | Local Key 列表。 |
| `GET /admin/api/routing` | 路由规则列表。 |
| `POST /admin/api/routing/test` | 路由模拟。 |
| `GET /admin/api/settings` | 读取系统设置。 |

## 路由与故障转移机制

```text
requested model
      │
      ▼
model alias resolution
      │
      ▼
wildcard routing rule match ──► provider_chain / fallback_chain
      │
      ▼
provider selection by priority and supported models
      │
      ▼
upstream request
      │
      ├── success ──► response + usage record + request log
      │
      ├── 4xx     ──► mapped gateway error + request log
      │
      └── 429 / 5xx / network error
              │
              ▼
        fallback provider chain
```

Fallback 结果会写入请求日志 metadata，并在以下位置可见：

- Dashboard 备用切换 KPI；
- 失败 / 备用切换趋势；
- Logs 页面 Fallback 筛选；
- CSV 导出中的 Fallback Chain；
- Trace metadata 中的 `fallbackTried`。

## 项目结构

```text
.
├── cmd/tokenbridge/          # 浏览器 / 托盘版入口
├── internal/                  # Go 后端服务与 HTTP handlers
│   ├── admin/                 # Dashboard / Analytics 聚合
│   ├── app/                   # 应用启动与依赖装配
│   ├── auth/                  # Local Key 服务
│   ├── desktop/               # Wails 桌面壳、桌面 bindings、托盘、AI 统计浮窗
│   ├── provider/              # Provider 注册、状态与模型能力
│   ├── requestlog/            # Trace、日志查询、统计与导出
│   ├── routing/               # 模型别名、路由规则、Fallback Chain
│   ├── server/                # HTTP Router、网关接口、Admin API
│   ├── storage/               # SQLite / GORM 初始化
│   └── usage/                 # 用量统计与分析
├── web/admin/                 # React + TypeScript 管理后台
│   ├── src/pages/             # Dashboard / Providers / Keys / Routing / Analytics / Logs / Settings
│   └── scripts/sync-embed.mjs # 同步前端构建产物到 Go embed 目录
├── build/                     # 图标、资源、打包脚本和嵌入资源目标
├── configs/                   # YAML 配置模板
├── migrations/                # 数据库迁移
├── packaging/                 # 分发说明
├── .github/workflows/         # CI 构建流程
├── main.go                    # Wails 桌面版薄入口，不承载业务实现
├── AGENTS.md                  # Agent 代码生成与目录规范
└── wails.json                 # Wails 配置
```

## 开发命令速查

```bash
# — 后端 —
go run ./cmd/tokenbridge              # 启动浏览器版
go build -o tokenbridge ./cmd/tokenbridge  # 编译二进制

# — 前端 —
cd web/admin
npm install                            # 安装依赖
npm run dev                            # 开发服务器（热更新）
npm run build                          # 生产构建 + 同步 embed
npm run build:wails                    # Wails 模式构建

# — 桌面版 —
wails dev                              # 开发模式（热更新）
wails build                            # 生产构建

# — 打包 —
powershell -File build/package.ps1     # Windows 便携包
powershell -File build/desktop.ps1     # Windows 桌面版
wails build -platform darwin/universal  # macOS 桌面版
```

## 生产运行说明

- Windows 正式浏览器版默认隐藏控制台并常驻托盘。
- `TB_CONSOLE_MODE=1` 可让正式 exe 以前台控制台运行，便于排查日志。
- 运行时应用日志会写入用户配置目录下的 `TokenBridge/logs/tokenbridge.log`，`error` 及以上级别同时写入 `TokenBridge/logs/tokenbridge.err.log`。
- 单实例 Mutex 会阻止重复启动；已有实例运行时，新进程会打开管理后台后退出。
- 管理后台资源会嵌入 Go 二进制，分发时不需要额外 Web Server。
- 请求日志以 Trace ID、Provider、状态码、耗时、Fallback、请求模型和实际模型为核心字段，继续保存在 SQLite 中供控制台检索和导出。

## 安全建议

- 不要把真实 Provider API Key 暴露给业务应用，业务侧只使用 Local Key。
- 不要提交 `configs/config.yaml`、`data/`、`.secret`、日志和真实密钥。
- 非必要不要将服务绑定到公网地址；如需外部访问，请放在可信反向代理或内网策略之后。
- 开启 CORS 时显式配置 `allowed_origins`，避免宽泛开放。

## Roadmap

- 更完整的主动 Provider 健康探测与定时巡检。
- 更严格的 `routing.fallback.retry_on` 策略执行。
- Local Key 预算硬限制与额度告警。
- Provider 成本表与实时费用估算。
- 更多协议适配器与 Provider 模板。
- 更完善的安装包、升级通道和备份恢复体验。

## 贡献规范

欢迎提交 Issue 和 Pull Request。建议保持改动边界清晰、可验证、可回滚。

1. Fork 仓库。
2. 创建功能分支：

   ```bash
   git checkout -b feat/your-feature
   ```

3. 完成代码和文档修改。
4. 执行必要检查：

   ```bash
   go build ./...
   cd web/admin && npm run build
   ```

5. 使用清晰的提交信息：

   ```bash
   git commit -m "feat: add provider health probe"
   ```

6. 提交 Pull Request，并说明：
   - 改动内容；
   - 改动原因；
   - 测试方式；
   - UI 改动截图或录屏。

### Git hygiene

建议纳入版本控制：

- `cmd/`, `internal/`, `configs/`, `migrations/`, `web/admin/src/`, `web/admin/public/`
- `go.mod`, `go.sum`, 前端 package 文件、构建脚本、文档

不建议纳入版本控制：

- `data/`, `logs/`, `web/admin/dist/`, `build/embed/admin/`, `build/portable/`
- `*.exe`, `*.log`, 本地运行产物、真实密钥

## License

当前仓库尚未包含 License 文件。正式开源前建议补充 `LICENSE`，明确授权范围。

---

<div align="center">

**TokenBridge** makes local AI access observable, governable, and failover-ready.

</div>
