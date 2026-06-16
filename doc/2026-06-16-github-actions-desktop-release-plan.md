# GitHub Actions 桌面端自动打包发布实施计划

> **给执行代理的要求：** 如果后续由 agent 执行本计划，必须按任务逐项执行并更新复选框状态。推荐使用 `superpowers:subagent-driven-development`，也可以使用 `superpowers:executing-plans`。

**目标：** 给 TokenBridge 增加 GitHub Actions 自动打包发布能力：Windows 自动生成 `.exe`，macOS 自动生成 `.dmg`，并在推送 `v*` 标签时自动上传到 GitHub Release。

**架构：** 继续以现有 Wails 桌面构建为核心，不改变 Go 项目结构。Windows 使用 `windows-latest` runner 构建 `.exe`，macOS 使用 `macos-latest` runner 构建 `.app` 并转换为 `.dmg`，最后由独立 release job 下载两个平台的产物并发布到 GitHub Release。签名和 macOS 公证作为第二阶段处理，先保证未签名自动打包链路稳定。

**技术栈：** GitHub Actions、Wails v2.12、Go 1.22、Node.js 20、npm、Vite、PowerShell、bash、macOS `hdiutil`、GitHub CLI `gh`。

---

## 当前仓库情况

当前相关文件：

- `.github/workflows/build-desktop.yml`：已经有基础桌面端构建流程，触发条件是 `v*` 标签和手动触发。
- `wails.json`：Wails 配置直接指向 `web/admin`，前端构建命令是 `npm run build:wails`，输出名是 `TokenBridge`。
- `web/admin/package.json`：`build:wails` 会执行 `vite build --base ./ && node scripts/sync-embed.mjs`。
- `main.go`：根目录 Wails 薄入口，只负责启动 Wails 和绑定桌面应用。
- `internal/desktop/`：Wails 桌面能力实现目录。

必须遵守的项目约束：

- 不要在根目录新增 Go 业务文件。
- CI、发布、打包脚本应该放在 `.github/workflows/`、`scripts/`、`build/`、`packaging/`、`release/` 或 `doc/`。
- 不要提交生成的 `.exe`、`.dmg`、`.zip`、`.app`、日志、临时文件和本地构建产物。

## 发布策略

### 第一阶段：未签名自动发布

这是推荐优先实现的版本。

生成产物：

- `TokenBridge-windows-amd64.exe`
- `TokenBridge-macos-universal.dmg`

触发行为：

- `workflow_dispatch`：手动触发，只构建并保存 Actions artifact，不发布 Release。
- 推送 `v*` 标签：构建两个平台产物，创建或更新 GitHub Release，并上传 `.exe`、`.dmg` 和校验文件。

优点：

- 实现快。
- 不需要额外证书和 GitHub Secrets。
- 可以先验证整个自动打包链路。

限制：

- Windows 可能出现 SmartScreen 提示。
- macOS 可能出现 Gatekeeper 拦截或“无法验证开发者”的提示。

### 第二阶段：签名和公证

第一阶段稳定后再做。

增加内容：

- Windows Authenticode 代码签名。
- macOS Developer ID 签名。
- macOS Apple notarization 公证。
- 使用 `stapler` 把公证票据装订到 app 或 DMG。

优点：

- 更适合公开分发。
- 用户打开时系统安全提示更少。

代价：

- 需要 Apple Developer 账号。
- 需要管理证书、密钥和 GitHub Secrets。
- CI 流程复杂度明显增加。

## 文件规划

计划修改或新增这些文件：

- 修改：`.github/workflows/build-desktop.yml`
  - 替换当前只上传 artifact 的流程。
  - 保留 Windows 和 macOS 独立 job。
  - 增加 tag 发布时运行的 release job。

- 新增：`scripts/create-dmg.sh`
  - 只在 macOS runner 上使用。
  - 把 `build/bin/TokenBridge.app` 转换为 `dist/TokenBridge-macos-universal.dmg`。
  - 避免把较长的 DMG 打包命令直接塞进 workflow。

- 新增：`packaging/macos/README.md`
  - 说明未签名 DMG 的行为。
  - 记录后续签名、公证需要的 secrets 和流程。

- 新增：`packaging/windows/README.md`
  - 说明未签名 `.exe` 的行为。
  - 记录后续 Windows 签名方案。

- 可选修改：`.gitignore`
  - 如果后续本地会生成新的发布目录，再补充 `/dist/`、`/release/*.dmg`、`/release/*.sha256`。
  - 如果实现只在 CI 中生成 `dist/`，本地不保留产物，可以暂时不改。

## GitHub Secrets 规划

第一阶段不需要额外 GitHub Secrets。

第一阶段只使用 GitHub 内置的 `GITHUB_TOKEN`，但 workflow 顶部必须声明：

```yaml
permissions:
  contents: write
```

第二阶段 macOS 签名和公证需要这些 GitHub Secrets：

```text
APPLE_CERTIFICATE_P12
APPLE_CERTIFICATE_PASSWORD
APPLE_TEAM_ID
APPLE_API_KEY_ID
APPLE_API_ISSUER_ID
APPLE_API_KEY_P8
```

第二阶段 Windows 签名需要这些 GitHub Secrets：

```text
WINDOWS_CERTIFICATE_PFX
WINDOWS_CERTIFICATE_PASSWORD
```

## 任务 1：替换桌面端构建 workflow

**文件：**

- 修改：`.github/workflows/build-desktop.yml`

- [ ] **步骤 1：查看当前 workflow**

运行：

```powershell
Get-Content -Raw .github\workflows\build-desktop.yml
```

预期结果：

- workflow 名称是 `Build Desktop Apps`。
- 存在 `build-windows` 和 `build-macos` 两个 job。
- 当前只上传 artifact，不创建 GitHub Release。
- macOS 当前上传的是 `build/bin/TokenBridge.app`，不是 `.dmg`。

- [ ] **步骤 2：替换 workflow 内容**

把 `.github/workflows/build-desktop.yml` 替换为以下内容：

```yaml
name: Build Desktop Apps

on:
  push:
    tags:
      - 'v*'
  workflow_dispatch:

permissions:
  contents: write

jobs:
  build-windows:
    name: Build Windows
    runs-on: windows-latest

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache: true

      - name: Setup Node
        uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: npm
          cache-dependency-path: web/admin/package-lock.json

      - name: Install Wails
        shell: pwsh
        run: go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0

      - name: Install frontend dependencies
        working-directory: web/admin
        shell: pwsh
        run: npm ci

      - name: Build frontend
        working-directory: web/admin
        shell: pwsh
        run: npm run build:wails

      - name: Test Go packages
        shell: pwsh
        run: go test ./...

      - name: Build Windows app
        shell: pwsh
        run: wails build -s -skipbindings -platform windows/amd64 -o TokenBridge.exe

      - name: Prepare Windows artifact
        shell: pwsh
        run: |
          New-Item -ItemType Directory -Force dist | Out-Null
          Copy-Item build\bin\TokenBridge.exe dist\TokenBridge-windows-amd64.exe -Force
          Get-FileHash dist\TokenBridge-windows-amd64.exe -Algorithm SHA256 |
            ForEach-Object { "$($_.Hash.ToLower())  TokenBridge-windows-amd64.exe" } |
            Set-Content dist\TokenBridge-windows-amd64.exe.sha256

      - name: Upload Windows artifact
        uses: actions/upload-artifact@v4
        with:
          name: tokenbridge-windows-amd64
          path: |
            dist/TokenBridge-windows-amd64.exe
            dist/TokenBridge-windows-amd64.exe.sha256
          if-no-files-found: error

  build-macos:
    name: Build macOS
    runs-on: macos-latest

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache: true

      - name: Setup Node
        uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: npm
          cache-dependency-path: web/admin/package-lock.json

      - name: Install Wails
        run: go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0

      - name: Install frontend dependencies
        working-directory: web/admin
        run: npm ci

      - name: Build frontend
        working-directory: web/admin
        run: npm run build:wails

      - name: Test Go packages
        run: go test ./...

      - name: Build macOS app
        run: wails build -s -skipbindings -platform darwin/universal -o TokenBridge

      - name: Create DMG
        run: bash scripts/create-dmg.sh

      - name: Upload macOS artifact
        uses: actions/upload-artifact@v4
        with:
          name: tokenbridge-macos-universal
          path: |
            dist/TokenBridge-macos-universal.dmg
            dist/TokenBridge-macos-universal.dmg.sha256
          if-no-files-found: error

  release:
    name: Publish GitHub Release
    runs-on: ubuntu-latest
    needs:
      - build-windows
      - build-macos
    if: startsWith(github.ref, 'refs/tags/v')

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Download artifacts
        uses: actions/download-artifact@v4
        with:
          path: dist
          merge-multiple: true

      - name: List release files
        run: ls -la dist

      - name: Create or update release
        env:
          GH_TOKEN: ${{ github.token }}
          TAG_NAME: ${{ github.ref_name }}
        run: |
          if gh release view "$TAG_NAME" >/dev/null 2>&1; then
            gh release upload "$TAG_NAME" dist/* --clobber
          else
            gh release create "$TAG_NAME" dist/* \
              --title "TokenBridge $TAG_NAME" \
              --notes "Automated desktop release for TokenBridge $TAG_NAME."
          fi
```

- [ ] **步骤 3：本地检查 workflow 内容**

运行：

```powershell
Get-Content -Raw .github\workflows\build-desktop.yml
```

预期结果：

- 文件包含 `permissions: contents: write`。
- 文件包含 `TokenBridge-windows-amd64.exe`。
- 文件包含 `TokenBridge-macos-universal.dmg`。
- 文件包含 `release` job。
- `release` job 内包含 `gh release create` 和 `gh release upload`。

- [ ] **步骤 4：提交 workflow 修改**

运行：

```powershell
git add .github/workflows/build-desktop.yml
git commit -m "ci: publish desktop release artifacts"
```

预期结果：

- 提交成功。

## 任务 2：新增 macOS DMG 打包脚本

**文件：**

- 新增：`scripts/create-dmg.sh`

- [ ] **步骤 1：创建脚本**

创建 `scripts/create-dmg.sh`，内容如下：

```bash
#!/usr/bin/env bash
set -euo pipefail

APP_PATH="build/bin/TokenBridge.app"
DIST_DIR="dist"
DMG_PATH="${DIST_DIR}/TokenBridge-macos-universal.dmg"
VOLUME_NAME="TokenBridge"

if [[ ! -d "${APP_PATH}" ]]; then
  echo "Missing app bundle: ${APP_PATH}" >&2
  exit 1
fi

rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

hdiutil create \
  -volname "${VOLUME_NAME}" \
  -srcfolder "${APP_PATH}" \
  -ov \
  -format UDZO \
  "${DMG_PATH}"

shasum -a 256 "${DMG_PATH}" | awk '{ print $1 "  TokenBridge-macos-universal.dmg" }' > "${DMG_PATH}.sha256"
```

- [ ] **步骤 2：设置脚本可执行权限**

运行：

```powershell
git update-index --chmod=+x scripts/create-dmg.sh
```

预期结果：

- Git 记录该脚本的可执行权限，macOS/Linux runner 可以直接执行。

- [ ] **步骤 3：提交脚本**

运行：

```powershell
git add scripts/create-dmg.sh
git commit -m "build: add macos dmg packaging script"
```

预期结果：

- 提交成功。

## 任务 3：新增打包说明文档

**文件：**

- 新增：`packaging/macos/README.md`
- 新增：`packaging/windows/README.md`

- [ ] **步骤 1：新增 macOS 打包说明**

创建 `packaging/macos/README.md`，内容如下：

````markdown
# macOS 打包说明

GitHub Actions 桌面端 workflow 会构建 Wails universal macOS app，并打包为：

```text
TokenBridge-macos-universal.dmg
```

第一版发布流程不做签名。未签名构建适合内部测试，但 macOS Gatekeeper 可能会在用户打开应用时提示无法验证开发者。

## 后续签名与公证

公开分发 macOS 应用时，建议增加 Developer ID 签名和 Apple notarization 公证。

需要配置的 GitHub Secrets：

```text
APPLE_CERTIFICATE_P12
APPLE_CERTIFICATE_PASSWORD
APPLE_TEAM_ID
APPLE_API_KEY_ID
APPLE_API_ISSUER_ID
APPLE_API_KEY_P8
```

预期签名和公证流程：

1. 把 Developer ID Application 证书导入临时 keychain。
2. 使用 Wails 构建 `TokenBridge.app`。
3. 使用 `codesign --deep --force --options runtime --timestamp` 签名 `.app`。
4. 创建 DMG。
5. 使用 `xcrun notarytool submit --wait` 提交公证。
6. 使用 `xcrun stapler staple` 装订公证票据。
7. 把已公证 DMG 上传到 GitHub Release。
````

- [ ] **步骤 2：新增 Windows 打包说明**

创建 `packaging/windows/README.md`，内容如下：

````markdown
# Windows 打包说明

GitHub Actions 桌面端 workflow 会构建 Wails Windows 应用，并发布：

```text
TokenBridge-windows-amd64.exe
```

第一版发布流程不做签名。未签名构建适合内部测试，但 Windows SmartScreen 可能会在用户打开可执行文件时出现安全提示。

## 后续代码签名

公开分发 Windows 应用时，建议增加 Authenticode 签名。

需要配置的 GitHub Secrets：

```text
WINDOWS_CERTIFICATE_PFX
WINDOWS_CERTIFICATE_PASSWORD
```

预期签名流程：

1. 使用 Wails 构建 `TokenBridge.exe`。
2. 导入或引用签名证书。
3. 使用 `signtool` 签名可执行文件。
4. 验证签名。
5. 把已签名 EXE 上传到 GitHub Release。
````

- [ ] **步骤 3：提交打包文档**

运行：

```powershell
git add packaging/macos/README.md packaging/windows/README.md
git commit -m "docs: document desktop packaging strategy"
```

预期结果：

- 提交成功。

## 任务 4：推送前本地验证

**文件：**

- 验证：`.github/workflows/build-desktop.yml`
- 验证：`scripts/create-dmg.sh`
- 验证：`packaging/macos/README.md`
- 验证：`packaging/windows/README.md`

- [ ] **步骤 1：运行 Go 测试**

运行：

```powershell
go test ./...
```

预期结果：

- 所有 Go 包测试通过。

- [ ] **步骤 2：运行 Wails dry run**

运行：

```powershell
wails build -dryrun -s -skipbindings -platform windows/amd64 -o TokenBridge.exe
```

预期结果：

- Wails 输出 Windows 构建命令。
- 不执行完整构建。
- 不出现 Wails 配置错误。

- [ ] **步骤 3：构建前端**

运行：

```powershell
Push-Location web\admin
npm ci
npm run build:wails
Pop-Location
```

预期结果：

- `npm ci` 成功。
- `npm run build:wails` 成功。
- 本地生成 `web/admin/dist`。
- `web/admin/dist` 仍然被 git ignore，不应被提交。

- [ ] **步骤 4：检查 git 状态**

运行：

```powershell
git status --short
```

预期结果：

- 只出现本次有意修改的 workflow、脚本和文档。
- 不应暂存或提交 `.exe`、`.dmg`、`.app`、`dist`、`build/bin`、`web/admin/dist`、`node_modules` 等生成物。

## 任务 5：在 GitHub Actions 中验证

**文件：**

- 远程验证：`.github/workflows/build-desktop.yml`

- [ ] **步骤 1：推送分支**

运行：

```powershell
git push
```

预期结果：

- 分支推送到 GitHub。

- [ ] **步骤 2：手动触发 workflow**

在 GitHub 仓库 Actions 页面执行：

```text
Build Desktop Apps -> Run workflow
```

预期结果：

- `build-windows` 成功。
- `build-macos` 成功。
- workflow run 中能看到两个 artifacts：
  - `tokenbridge-windows-amd64`
  - `tokenbridge-macos-universal`
- `release` job 被跳过，因为这不是 tag 构建。

- [ ] **步骤 3：下载并检查 artifacts**

预期文件：

```text
TokenBridge-windows-amd64.exe
TokenBridge-windows-amd64.exe.sha256
TokenBridge-macos-universal.dmg
TokenBridge-macos-universal.dmg.sha256
```

预期结果：

- 文件名符合规划。
- 校验文件存在。
- Windows 产物是 `.exe`。
- macOS 产物是 `.dmg`，不是 `.app` 文件夹。

## 任务 6：验证 tag 自动发布

**文件：**

- 远程验证：GitHub Release 页面

- [ ] **步骤 1：创建测试 tag**

运行：

```powershell
git tag v0.1.0-test
git push origin v0.1.0-test
```

预期结果：

- GitHub Actions 自动启动。

- [ ] **步骤 2：确认 release job**

预期结果：

- `build-windows` 成功。
- `build-macos` 成功。
- `release` 成功。
- GitHub Releases 中出现 `TokenBridge v0.1.0-test`。

- [ ] **步骤 3：确认 Release assets**

预期 Release assets：

```text
TokenBridge-windows-amd64.exe
TokenBridge-windows-amd64.exe.sha256
TokenBridge-macos-universal.dmg
TokenBridge-macos-universal.dmg.sha256
```

- [ ] **步骤 4：验证后删除测试 Release 和 tag**

运行：

```powershell
gh release delete v0.1.0-test --yes
git push origin --delete v0.1.0-test
git tag -d v0.1.0-test
```

预期结果：

- 测试 Release 被删除。
- 远程测试 tag 被删除。
- 本地测试 tag 被删除。

## 任务 7：第二阶段签名计划

**文件：**

- 后续修改：`.github/workflows/build-desktop.yml`
- 后续修改：`packaging/macos/README.md`
- 后续修改：`packaging/windows/README.md`

- [ ] **步骤 1：在未签名 DMG 流程稳定后增加 macOS 签名**

核心命令示例：

```bash
security create-keychain -p "$KEYCHAIN_PASSWORD" build.keychain
security import certificate.p12 -k build.keychain -P "$APPLE_CERTIFICATE_PASSWORD" -T /usr/bin/codesign
security list-keychains -d user -s build.keychain
security unlock-keychain -p "$KEYCHAIN_PASSWORD" build.keychain
codesign --deep --force --options runtime --timestamp --sign "Developer ID Application" build/bin/TokenBridge.app
```

预期结果：

```bash
codesign --verify --deep --strict build/bin/TokenBridge.app
```

能够成功通过。

- [ ] **步骤 2：增加 macOS 公证**

核心命令示例：

```bash
xcrun notarytool submit dist/TokenBridge-macos-universal.dmg \
  --key AuthKey.p8 \
  --key-id "$APPLE_API_KEY_ID" \
  --issuer "$APPLE_API_ISSUER_ID" \
  --wait

xcrun stapler staple dist/TokenBridge-macos-universal.dmg
```

预期结果：

- Apple 公证成功。
- `xcrun stapler validate dist/TokenBridge-macos-universal.dmg` 成功。

- [ ] **步骤 3：增加 Windows 签名**

核心命令示例：

```powershell
signtool sign /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 /f certificate.pfx /p "$env:WINDOWS_CERTIFICATE_PASSWORD" dist\TokenBridge-windows-amd64.exe
signtool verify /pa /v dist\TokenBridge-windows-amd64.exe
```

预期结果：

- 签名验证成功。

## 风险清单

- macOS 最终构建必须在 macOS runner 上完成，不要尝试在 Windows runner 上生成可分发的 macOS app。
- 未签名 macOS DMG 被 Gatekeeper 提示是预期现象。
- 未签名 Windows EXE 被 SmartScreen 提示是预期现象。
- `actions/upload-artifact@v4` 不适合同一个 job 多次上传同名 artifact，保持 artifact 名称唯一。
- `npm ci` 依赖 `web/admin/package-lock.json`，该文件必须保留在仓库中。
- 本地生成物不要提交。
- 当前工作区已有其他未提交修改，执行本计划时不要回滚或顺手提交无关文件。

## 完成标准

满足以下条件才算完成：

- 手动触发 GitHub Actions 能生成 Windows 和 macOS artifacts。
- 推送 `v*` tag 能自动创建 GitHub Release。
- Release 中包含 `.exe`、`.dmg` 和对应 `.sha256` 文件。
- 推送前本地验证命令通过。
- 没有把生成的二进制或构建产物提交进仓库。

## 参考资料

- Wails CLI 本地帮助：`wails build -help`
- GitHub CLI release upload：<https://cli.github.com/manual/gh_release_upload>
- GitHub Actions token permissions：<https://docs.github.com/en/actions/using-jobs/assigning-permissions-to-jobs>
- Apple notarization overview：<https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution>
