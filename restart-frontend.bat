@echo off
REM NOFX Docker 快速重启脚本 - 更新前端 (Windows)
REM 用于应用前端代码更改

echo ================================
echo 🐳 NOFX Docker 快速重启 - 更新前端
echo ================================
echo.

REM 进入项目目录
cd /d "%~dp0"

echo 📍 当前目录: %CD%
echo.

REM 1. 停止容器
echo ⏹️  停止 Docker 容器...
docker compose down
if %ERRORLEVEL% NEQ 0 (
    echo ❌ 停止容器失败
    pause
    exit /b 1
)
echo ✅ 容器已停止
echo.

REM 2. 重新构建前端镜像
echo 🔨 重新构建前端镜像...
docker compose build nofx-frontend
if %ERRORLEVEL% NEQ 0 (
    echo ❌ 构建前端镜像失败
    pause
    exit /b 1
)
echo ✅ 前端镜像构建完成
echo.

REM 3. 启动容器
echo 🚀 启动 Docker 容器...
docker compose up -d
if %ERRORLEVEL% NEQ 0 (
    echo ❌ 启动容器失败
    pause
    exit /b 1
)
echo ✅ 容器已启动
echo.

REM 4. 等待服务就绪
echo ⏳ 等待服务启动...
timeout /t 5 /nobreak >nul

REM 5. 检查容器状态
echo 📊 检查容器状态...
docker compose ps
echo.

REM 6. 显示前端日志（最后20行）
echo 📋 前端容器日志（最后20行）：
echo ==================================
docker compose logs --tail=20 nofx-frontend
echo.

REM 7. 完成提示
echo ✅ 重启完成！
echo.
echo 🌐 访问地址：
echo    前端: http://localhost:3000
echo    后端: http://localhost:8080
echo.
echo 💡 提示：
echo    1. 在浏览器中按 Ctrl+Shift+R 强制刷新
echo    2. 访问策略工作室查看动态止盈止损功能
echo    3. 查看实时日志: docker compose logs -f
echo.

pause




