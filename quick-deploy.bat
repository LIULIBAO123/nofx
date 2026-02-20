@echo off
chcp 65001 >nul
echo.
echo ==========================================
echo   nofx AI 交易系统 - 一键部署
echo ==========================================
echo.
echo 📦 正在添加文件...
git add .
echo.
echo 💾 正在提交...
git commit -m "feat: 全面优化 nofx AI 交易系统 - 新增仓位管理、回撤控制、智能前端组件"
echo.
echo 🚀 正在推送到 GitHub...
git push
echo.
echo ==========================================
echo ✅ 部署完成！
echo ==========================================
echo.
echo 📋 本次更新:
echo   - 仓位管理器 (金字塔加仓、分批止盈)
echo   - 回撤控制器 (分级响应、恢复机制)
echo   - 4个智能前端组件
echo   - 完整文档和分析报告
echo.
pause

