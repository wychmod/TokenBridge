# AGENTS.md

## Project Structure Rules

根目录只保留项目级配置、文档、构建入口和 Wails 必需的薄入口文件。不要在根目录新增业务 Go 文件。

这次根目录曾经出现大量 Go 文件，原因是 Wails 模板默认把桌面 `package main` 放在项目根部。当前项目已经采用 `cmd/` + `internal/` 的 Go 项目结构，因此桌面实现已移动到 `internal/desktop/`，根目录 `main.go` 只负责启动 Wails 并绑定桌面应用。

### Go 代码放置规范

- `main.go`: 仅保留 Wails bootstrap、`wails.Run` 配置和对 `internal/desktop` 的调用。不要在这里新增业务逻辑。
- `cmd/tokenbridge/`: 浏览器版 / 托盘版命令入口，包括 HTTP Server 启动、Windows 单实例、浏览器打开逻辑。
- `internal/app/`: 应用装配、配置加载、数据库初始化、服务初始化和 Router 挂载。
- `internal/server/`: HTTP Router、OpenAI/Claude 兼容网关、Admin API handler。
- `internal/desktop/`: Wails 桌面窗口、桌面 bindings、SPA proxy、托盘菜单、桌面自检、AI 统计浮窗和桌面平台相关实现。
- `internal/<domain>/`: 领域服务，例如 `provider`、`routing`、`auth`、`pricing`、`usage`、`requestlog`。
- `build/embed/`: Go embed 入口和前端构建产物目标目录。不要手写业务逻辑。
- `third_party/`: vendored 或 forked 第三方代码。只在明确升级或修补第三方依赖时修改。

### 新文件创建规则

- 新增 Go 业务代码必须先判断职责归属，放入对应 `internal/<domain>/` 包。
- 新增可执行入口才允许放入 `cmd/<name>/`。
- 新增桌面能力放入 `internal/desktop/`，不要再拆到根目录。
- 新增前端代码放入 `web/admin/src/` 对应的 `pages`、`components`、`stores`、`utils` 等目录。
- 新增构建脚本放入 `build/` 或 `scripts/`，分发说明放入 `packaging/` 或 `release/`。
- 运行日志、临时输出、测试产物和本地二进制不要放入根目录；需要保留时放入 `logs/`、`build/` 或明确的临时目录，并确认 `.gitignore` 覆盖。

### 修改前检查

在生成或移动代码前，先执行：

```powershell
rg --files -g "*.go"
```

确认现有包结构后再创建文件。除非正在修改 `main.go` 的 Wails 启动配置，否则不要在根目录创建 `.go` 文件。

### 验证要求

涉及 Go 代码移动或包名调整后，至少运行：

```powershell
go test ./...
wails build -dryrun -s -skipbindings -platform windows/amd64 -o TokenBridge.exe
```

涉及前端代码后，进入 `web/admin` 运行对应的 lint/build/test 命令。
