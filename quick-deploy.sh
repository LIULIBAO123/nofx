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

# 登录 GHCR（如果需要）
login_ghcr() {
    if [ ! -f "$INSTALL_DIR/docker-compose.prod.yml" ]; then
        return 0
    fi
    
    if ! grep -q "ghcr.io" "$INSTALL_DIR/docker-compose.prod.yml"; then
        return 0
    fi
    
    echo "检测到使用 GHCR 私有镜像，需要登录认证"
    
    # 方法1: 使用环境变量 GITHUB_PAT 和 GITHUB_USERNAME
    if [ -n "$GITHUB_PAT" ] && [ -n "$GITHUB_USERNAME" ]; then
        echo "使用环境变量中的 PAT 登录 GHCR..."
        echo "$GITHUB_PAT" | docker login ghcr.io -u "$GITHUB_USERNAME" --password-stdin
        if [ $? -eq 0 ]; then
            echo "✓ GHCR 登录成功"
            return 0
        else
            echo "✗ GHCR 登录失败，请检查 PAT 和用户名"
        fi
    fi
    
    # 方法2: 使用命令行参数
    if [ -n "$1" ] && [ -n "$2" ]; then
        echo "使用命令行参数中的 PAT 登录 GHCR..."
        echo "$1" | docker login ghcr.io -u "$2" --password-stdin
        if [ $? -eq 0 ]; then
            echo "✓ GHCR 登录成功"
            return 0
        else
            echo "✗ GHCR 登录失败，请检查 PAT 和用户名"
        fi
    fi
    
    # 方法3: 交互式输入
    echo ""
    echo "请选择登录方式："
    echo "1) 使用 Personal Access Token (PAT)"
    echo "2) 跳过（如果已登录）"
    read -p "请选择 [1-2]: " choice
    
    case $choice in
        1)
            read -p "请输入 GitHub 用户名: " github_username
            read -sp "请输入 Personal Access Token: " github_pat
            echo ""
            if [ -n "$github_username" ] && [ -n "$github_pat" ]; then
                echo "$github_pat" | docker login ghcr.io -u "$github_username" --password-stdin
                if [ $? -eq 0 ]; then
                    echo "✓ GHCR 登录成功"
                else
                    echo "✗ GHCR 登录失败"
                    exit 1
                fi
            else
                echo "✗ 用户名或 PAT 为空"
                exit 1
            fi
            ;;
        2)
            echo "跳过登录，假设已登录 GHCR"
            ;;
        *)
            echo "无效选择，退出"
            exit 1
            ;;
    esac
}

# 调用登录函数（支持命令行参数: PAT 用户名）
login_ghcr "$1" "$2"

# 启动服务
echo "启动服务..."
if [ -f "$INSTALL_DIR/docker-compose.prod.yml" ]; then
    docker compose -f "$INSTALL_DIR/docker-compose.prod.yml" up -d
else
    docker compose up -d
fi

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
if [ -f "$INSTALL_DIR/docker-compose.prod.yml" ]; then
    echo "  查看日志: cd $INSTALL_DIR && docker compose -f docker-compose.prod.yml logs -f"
    echo "  停止服务: cd $INSTALL_DIR && docker compose -f docker-compose.prod.yml down"
    echo "  重启服务: cd $INSTALL_DIR && docker compose -f docker-compose.prod.yml restart"
else
    echo "  查看日志: cd $INSTALL_DIR && docker compose logs -f"
    echo "  停止服务: cd $INSTALL_DIR && docker compose down"
    echo "  重启服务: cd $INSTALL_DIR && docker compose restart"
fi
echo ""

