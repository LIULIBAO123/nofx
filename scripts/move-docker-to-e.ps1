# 将 Docker Desktop (WSL2) 数据从 C 盘迁移到 E 盘
# 会导出 docker-desktop-data 到 E 盘并重新导入，原 C 盘 WSL 数据将被注销（请先确保无重要未推送/未备份数据）
# 用法：请先关闭 Docker Desktop，然后以管理员身份运行此脚本

$ExportTar = "E:\DevData\Docker\wsl\docker-desktop-data.tar"
$ImportPath = "E:\DevData\Docker\wsl"

# 1. 检查 Docker 与 WSL
Write-Host "请确保已关闭 Docker Desktop 后再继续。"
$cont = Read-Host "已关闭 Docker Desktop？(y/n)"
if ($cont -ne "y" -and $cont -ne "Y") {
    Write-Host "请先关闭 Docker Desktop，然后重新运行本脚本。"
    exit 1
}

# 2. 创建 E 盘目录
if (-not (Test-Path "E:\DevData\Docker\wsl")) {
    New-Item -ItemType Directory -Path "E:\DevData\Docker\wsl" -Force
}

# 3. 关闭 WSL（所有 WSL 发行版会停止）
Write-Host "正在关闭 WSL..."
wsl --shutdown
Start-Sleep -Seconds 3

# 4. 导出 docker-desktop-data
Write-Host "正在导出 docker-desktop-data 到 E 盘（可能较慢）..."
wsl --export docker-desktop-data $ExportTar
if ($LASTEXITCODE -ne 0) {
    Write-Error "导出失败。请确认 Docker Desktop 已关闭且 WSL 中存在 docker-desktop-data。"
    exit 1
}
Write-Host "导出完成: $ExportTar"

# 5. 注销原有 docker-desktop-data（C 盘上的 vhdx 将失效）
Write-Host "正在注销原 docker-desktop-data 发行版..."
wsl --unregister docker-desktop-data
if ($LASTEXITCODE -ne 0) {
    Write-Warning "注销可能失败，请检查是否已导出成功。"
}

# 6. 在 E 盘重新导入
Write-Host "正在将 docker-desktop-data 导入到 E 盘..."
wsl --import docker-desktop-data $ImportPath $ExportTar --version 2
if ($LASTEXITCODE -ne 0) {
    Write-Error "导入失败。"
    exit 1
}
Write-Host "导入完成。Docker 数据现已位于: $ImportPath"

# 7. 可选：删除导出用的 tar 以节省空间（导入后已不需要）
$del = Read-Host "是否删除临时导出文件以节省空间？(y/n)"
if ($del -eq "y" -or $del -eq "Y") {
    Remove-Item $ExportTar -Force -ErrorAction SilentlyContinue
    Write-Host "已删除: $ExportTar"
}

Write-Host "请重新启动 Docker Desktop。镜像与容器应仍在，数据已使用 E 盘。"
