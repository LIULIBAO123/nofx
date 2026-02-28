# NOFX 本地 Docker 一键部署 (Windows PowerShell)
# 用法: .\scripts\deploy-local.ps1

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..

Write-Host "=== NOFX 本地 Docker 部署 ===" -ForegroundColor Cyan

# 1. .env
if (-not (Test-Path .env)) {
    Write-Host "[1/4] 创建 .env（从 .env.example 复制）" -ForegroundColor Yellow
    Copy-Item .env.example .env
    Write-Host "      请编辑 .env 设置 JWT_SECRET、DATA_ENCRYPTION_KEY 后重新运行。" -ForegroundColor Gray
} else {
    Write-Host "[1/4] .env 已存在，跳过" -ForegroundColor Green
}

# 2. data 目录
New-Item -ItemType Directory -Force -Path data | Out-Null
Write-Host "[2/4] 数据目录 data/ 已就绪" -ForegroundColor Green

# 3. RSA 密钥
if (-not (Test-Path keys\private.pem)) {
    Write-Host "[3/4] 生成 RSA 密钥到 keys/" -ForegroundColor Yellow
    New-Item -ItemType Directory -Force -Path keys | Out-Null
    if (Get-Command openssl -ErrorAction SilentlyContinue) {
        openssl genrsa -out keys/private.pem 2048
        openssl rsa -in keys/private.pem -pubout -out keys/public.pem
        Write-Host "      keys/private.pem 与 keys/public.pem 已生成" -ForegroundColor Green
    } else {
        Write-Host "      未找到 openssl。请安装 OpenSSL 或 Git for Windows 后重试，或手动在 keys/ 下生成 private.pem、public.pem" -ForegroundColor Red
        exit 1
    }
} else {
    Write-Host "[3/4] keys/private.pem 已存在，跳过" -ForegroundColor Green
}

# 4. 构建并启动
Write-Host "[4/4] 构建并启动 Docker 服务..." -ForegroundColor Yellow
$portFrontend = if ($env:NOFX_FRONTEND_PORT) { $env:NOFX_FRONTEND_PORT } else { "3000" }
$portBackend  = if ($env:NOFX_BACKEND_PORT)  { $env:NOFX_BACKEND_PORT } else { "8080" }

try {
    docker compose up -d --build
} catch {
    try {
        docker-compose up -d --build
    } catch {
        Write-Host "未找到 docker compose / docker-compose，请先安装 Docker Desktop。" -ForegroundColor Red
        exit 1
    }
}

Write-Host ""
Write-Host "=== 部署完成 ===" -ForegroundColor Green
Write-Host "  前端: http://localhost:$portFrontend"
Write-Host "  后端: http://localhost:$portBackend/api/health"
Write-Host "  查看日志: docker compose logs -f"
Write-Host "  停止: docker compose down"
