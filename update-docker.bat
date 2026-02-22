@echo off
REM NOFX Docker 快速更新脚本 (Windows)
REM 用于更新 Meridian 设计系统后的前端部署

setlocal enabledelayedexpansion

echo ========================================
echo    NOFX Docker 更新脚本 (Windows)
echo ========================================
echo.

REM 检查 Docker 是否运行
docker info >nul 2>&1
if errorlevel 1 (
    echo [错误] Docker 未运行，请先启动 Docker Desktop
    pause
    exit /b 1
)

echo [信息] 当前服务状态:
docker-compose ps
echo.

echo 请选择更新类型:
echo 1) 完整更新（前端 + 后端）
echo 2) 仅更新前端（推荐用于 Meridian 设计系统更新）
echo 3) 仅更新后端
echo 4) 快速重启
echo.
set /p choice="请输入选项 (1-4): "

if "%choice%"=="1" goto full_update
if "%choice%"=="2" goto frontend_update
if "%choice%"=="3" goto backend_update
if "%choice%"=="4" goto quick_restart
echo [错误] 无效选项
pause
exit /b 1

:full_update
echo.
echo [执行] 完整更新...
echo.

echo [1/5] 停止服务...
docker-compose down

echo [2/5] 拉取最新代码...
git pull

echo [3/5] 重新构建镜像（不使用缓存）...
docker-compose build --no-cache

echo [4/5] 启动服务...
docker-compose up -d

echo [5/5] 等待服务就绪...
timeout /t 10 /nobreak >nul

goto check_health

:frontend_update
echo.
echo [执行] 前端更新（Meridian 设计系统）...
echo.

echo [1/5] 停止前端服务...
docker-compose stop nofx-frontend

echo [2/5] 删除旧镜像...
docker rmi nofx-nofx-frontend 2>nul

echo [3/5] 重新构建前端（不使用缓存）...
docker-compose build --no-cache nofx-frontend

echo [4/5] 启动前端服务...
docker-compose up -d nofx-frontend

echo [5/5] 等待服务就绪...
timeout /t 5 /nobreak >nul

goto check_health

:backend_update
echo.
echo [执行] 后端更新...
echo.

echo [1/5] 停止后端服务...
docker-compose stop nofx

echo [2/5] 删除旧镜像...
docker rmi nofx-nofx 2>nul

echo [3/5] 重新构建后端（不使用缓存）...
docker-compose build --no-cache nofx

echo [4/5] 启动后端服务...
docker-compose up -d nofx

echo [5/5] 等待服务就绪...
timeout /t 10 /nobreak >nul

goto check_health

:quick_restart
echo.
echo [执行] 快速重启...
echo.
docker-compose restart
timeout /t 5 /nobreak >nul
goto check_health

:check_health
echo.
echo ========================================
echo [完成] 更新完成！
echo ========================================
echo.

echo [信息] 服务状态:
docker-compose ps
echo.

echo [信息] 健康检查:

REM 检查后端
curl -s http://localhost:8080/api/health >nul 2>&1
if errorlevel 1 (
    echo [警告] 后端不健康
) else (
    echo [成功] 后端健康
)

REM 检查前端
curl -s http://localhost:3000/health >nul 2>&1
if errorlevel 1 (
    echo [警告] 前端不健康
) else (
    echo [成功] 前端健康
)

echo.
echo ========================================
echo [提示] 查看日志:
echo   docker-compose logs -f
echo.
echo [提示] 访问地址:
echo   前端: http://localhost:3000
echo   后端: http://localhost:8080
echo ========================================
echo.

pause



