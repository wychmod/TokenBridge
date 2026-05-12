# TokenBridge Portable Package Builder
# Usage: powershell -File build/package.ps1

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
$OutputDir   = Join-Path $ProjectRoot "build\portable\TokenBridge"
$PythonExe   = "python"
$GoVersionInfoExe = Join-Path $env:USERPROFILE "go\bin\goversioninfo.exe"

Write-Host "=== TokenBridge Portable Package Builder ===" -ForegroundColor Cyan

# Step 1: Generate app icons
Write-Host "[1/5] Generating app icons..." -ForegroundColor Yellow
Push-Location $ProjectRoot
& $PythonExe .\build\generate_icons.py
if ($LASTEXITCODE -ne 0) { Write-Host "Icon generation failed!" -ForegroundColor Red; exit 1 }
& $GoVersionInfoExe -64 -o build\resource_windows.syso build\goversioninfo.json
if ($LASTEXITCODE -ne 0) { Write-Host "Windows resource generation failed!" -ForegroundColor Red; exit 1 }
Pop-Location

# Step 2: Build frontend
Write-Host "[2/5] Building frontend..." -ForegroundColor Yellow
Push-Location (Join-Path $ProjectRoot "web\admin")
npm install --quiet
npm run build
Pop-Location

# Step 3: Copy frontend to embed dir
Write-Host "[3/5] Copying frontend assets to embed dir..." -ForegroundColor Yellow
$EmbedDir = Join-Path $ProjectRoot "build\embed\admin"
if (Test-Path $EmbedDir) { Remove-Item $EmbedDir -Recurse -Force }
New-Item -ItemType Directory -Path $EmbedDir -Force | Out-Null
Copy-Item -Path (Join-Path $ProjectRoot "web\admin\dist\*") -Destination $EmbedDir -Recurse -Force

# Step 4: Build Go binary
Write-Host "[4/5] Building Go binary..." -ForegroundColor Yellow
Push-Location $ProjectRoot
go build -ldflags="-H windowsgui" -o tokenbridge.exe ./cmd/tokenbridge
if ($LASTEXITCODE -ne 0) { Write-Host "Build failed!" -ForegroundColor Red; exit 1 }
Pop-Location

# Step 5: Assemble portable package
Write-Host "[5/5] Assembling portable package..." -ForegroundColor Yellow
if (Test-Path $OutputDir) { Remove-Item $OutputDir -Recurse -Force }
New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null

Copy-Item (Join-Path $ProjectRoot "tokenbridge.exe") $OutputDir -Force
Copy-Item (Join-Path $ProjectRoot "configs\config.example.yaml") (Join-Path $OutputDir "config.yaml") -Force

Write-Host ""
Write-Host "=== Build Complete ===" -ForegroundColor Green
Write-Host "Output: $OutputDir" -ForegroundColor White
Write-Host ""
Get-ChildItem $OutputDir -Recurse | Select-Object FullName

