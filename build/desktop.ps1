# TokenBridge 桌面版打包脚本 (Wails)
# 支持 Windows 本地打包，输出到 build/bin/

$ErrorActionPreference = "Stop"
$projectRoot = Split-Path -Parent $PSScriptRoot
Set-Location $projectRoot

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "TokenBridge Desktop Build (Wails)" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

# 1. 确保 Wails CLI 可用
Write-Host "`n[1/4] 检查 Wails CLI..." -ForegroundColor Yellow
$wails = Get-Command wails -ErrorAction SilentlyContinue
if (-not $wails) {
    Write-Host "Wails CLI 未找到，正在安装..." -ForegroundColor Yellow
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
}

# 2. 准备前端构建产物
Write-Host "`n[2/4] 准备前端构建产物..." -ForegroundColor Yellow
if (Test-Path "frontend") {
    Remove-Item -Recurse -Force "frontend"
}
robocopy "web\admin" "frontend" /E /COPY:DAT | Out-Null

Set-Location "frontend"
npm install
npm run build:wails
Set-Location $projectRoot

# 3. 构建 Wails 桌面应用并生成 bindings
Write-Host "`n[3/4] 构建 Wails 桌面应用..." -ForegroundColor Yellow
wails build -s -platform windows/amd64 -o TokenBridge.exe

# 4. 完成
Write-Host "`n[4/4] 构建完成！" -ForegroundColor Green
Write-Host "输出文件: $projectRoot\build\bin\TokenBridge.exe" -ForegroundColor Green
Write-Host "`n提示: 双击即可运行，无需浏览器" -ForegroundColor Gray

