#!/bin/bash

# ============================================================================
# NOFX 交易系统 - 快速部署脚本（简化版）
# ============================================================================

set -e

echo "=========================================="
echo "  NOFX 交易系统 - 快速部署"
echo "=========================================="
echo ""

# 检查 root 权限
if [ "$EUID" -ne 0 ]; then 
    echo "错误: 请使用 root 用户或 sudo 运行此脚本"
    exit 1
fi

# 安装 Docker
if ! command -v docker &> /dev/null; then
    echo "正在安装 Docker..."
    curl -fsSL https://get.docker.com | sh
    systemctl start docker
    systemctl enable docker
fi

# 安装 Git
if ! command -v git &> /dev/null; then
    echo "正在安装 Git..."
    apt-get update && apt-get install -y git || yum install -y git
fi

# 克隆项目
INSTALL_DIR="/opt/nofx"
if [ -d "$INSTALL_DIR" ]; then
    echo "目录已存在，更新代码..."
    cd "$INSTALL_DIR"
    git pull
else
    echo "克隆项目..."
    git clone https://github.com/LIULIBAO123/nofx.git "$INSTALL_DIR"
    cd "$INSTALL_DIR"
    git checkout dev
fi

# 创建 .env 文件
if [ ! -f "$INSTALL_DIR/.env" ]; then
    echo "创建环境变量文件..."
    if [ -f "$INSTALL_DIR/.env.example" ]; then
        cp "$INSTALL_DIR/.env.example" "$INSTALL_DIR/.env"
    else
        cat > "$INSTALL_DIR/.env" << 'EOF'
# NOFX 交易系统环境变量配置
DB_PATH=./data/nofx.db
API_PORT=8080
FRONTEND_PORT=3000
LOG_LEVEL=info
JWT_SECRET=nofx-default-secret-please-change-this-in-production
ENABLE_HTTPS=false
TZ=Asia/Shanghai
DATA_DIR=./data
LOG_DIR=./logs
EOF
    fi
    echo "环境变量文件已创建"
fi

# 创建必要的目录
mkdir -p "$INSTALL_DIR/data"
mkdir -p "$INSTALL_DIR/logs"

# 启动服务
echo "启动服务..."
docker compose up -d

# 获取服务器 IP
SERVER_IP=$(curl -s ifconfig.me 2>/dev/null || echo "YOUR_SERVER_IP")

echo ""
echo "=========================================="
echo "  部署完成！"
echo "=========================================="
echo ""
echo "访问地址："
echo "  前端: http://$SERVER_IP:3000"
echo "  API:  http://$SERVER_IP:8080"
echo ""
echo "管理命令："
echo "  查看日志: cd $INSTALL_DIR && docker compose logs -f"
echo "  停止服务: cd $INSTALL_DIR && docker compose down"
echo "  重启服务: cd $INSTALL_DIR && docker compose restart"
echo ""

