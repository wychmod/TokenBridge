# 桌面端 / 网页端统一前端合并复核

## 完成内容

- 已核实当前真实运行链路：浏览器端与 Wails 桌面端都以 `web/admin` 为唯一交付前端，`wails.json`、Go embed、自检与 README 均指向 `web/admin`。
- 已确认仓库内 `frontend/` 是分叉的参考/旧设计目录，不参与主线构建；差异主要集中在初始化、安全、发布、版本、构建检查、接入助手等页面。
- 已将 `frontend/` 中有价值的页面能力迁移到 `web/admin/src`：
  - `BootstrapPage.tsx`
  - `BootstrapSuccessPage.tsx`
  - `SecurityPage.tsx`
  - `ReleaseStatusPage.tsx`
  - `VersionInfoPage.tsx`
  - `BuildChecksPage.tsx`
  - `QuickSetupPage.tsx`
  - `InitializationGuard.tsx`
- 已更新 `web/admin/src/App.tsx`，让新增页面进入同一套路由体系，兼容现有 `/admin` basename 与桌面根路径。
- 已更新 `web/admin/src/layouts/AppShell.tsx`，把导航拆为“运营 / 交付 / 系统”分组，避免新增页面让侧栏失控。
- 已更新 `web/admin/src/store/ui-store.ts`，保留初始化状态能力，但默认 `initialized=true`，避免首次引导错误阻塞现有生产页面。
- 已在 `web/admin/src/styles/global.css` 中补齐统一交付页样式、导航分组、响应式布局、焦点态和触控尺寸，不整体覆盖原有 Quiet Console 工业风设计系统。

## 关键决策

- 统一基线选择 `web/admin`，不切换 Wails 到 `frontend/`，因为 `web/admin` 已接真实后端 API、构建同步和 Go embed 链路。
- 迁移页面均改为读取 `web/admin` 真实 store/API 状态，不保留 mock-only 的演示行为。
- 初始化流程作为“可执行引导页”保留，但不强制锁定生产页面，避免破坏已有用户工作流。
- 工业级设计方向维持沉稳、低噪音、控制台式产品质感，避免紫蓝玻璃拟态整站覆盖。

## 验证结果

- `web/admin` 生产构建：通过，并同步资源到 `build/embed/admin`。
- `go test ./...`：通过。
- `wails build`：通过，输出 `build/bin/Lingshu.exe`。

## 注意事项

- 直接运行 `npm run build` 和 `wails build` 时，当前环境的 `NODE_OPTIONS=--use-system-ca` 会导致 Node 报错；已通过清除该环境变量验证成功。
- 前端构建仍存在 Vite chunk size warning（约 696KB JS），不阻塞本次同源合并；后续建议按路由做 code splitting。
- 工作区存在本次任务前已有的后端/Provider 相关变更，本次只围绕 `web/admin` 统一前端增量修改。
