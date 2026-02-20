#!/bin/bash

# ============================================================================
# NOFX 交易系统 - 云服务器一键部署脚本
# ============================================================================
# 
# 使用方法：
#   1. 上传此脚本到云服务器
#   2. chmod +x deploy.sh
#   3. ./deploy.sh
#
# 支持的系统：Ubuntu 20.04+, Debian 10+, CentOS 7+
# ============================================================================

set -e  # 遇到错误立即退出

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# 检测操作系统
detect_os() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS=$ID
        VERSION=$VERSION_ID
    else
        log_error "无法检测操作系统"
        exit 1
    fi
    log_info "检测到操作系统: $OS $VERSION"
}

# 检查是否为 root 用户
check_root() {
    if [ "$EUID" -ne 0 ]; then 
        log_error "请使用 root 用户或 sudo 运行此脚本"
        exit 1
    fi
}

# 安装 Docker
install_docker() {
    log_step "检查 Docker 安装状态..."
    
    if command -v docker &> /dev/null; then
        log_info "Docker 已安装: $(docker --version)"
        return 0
    fi
    
    log_step "开始安装 Docker..."
    
    case $OS in
        ubuntu|debian)
            # 更新包索引
            apt-get update
            
            # 安装依赖
            apt-get install -y \
                ca-certificates \
                curl \
                gnupg \
                lsb-release
            
            # 添加 Docker 官方 GPG 密钥
            mkdir -p /etc/apt/keyrings
            curl -fsSL https://download.docker.com/linux/$OS/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
            
            # 设置 Docker 仓库
            echo \
              "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/$OS \
              $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
            
            # 安装 Docker Engine
            apt-get update
            apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
            ;;
            
        centos|rhel)
            # 安装依赖
            yum install -y yum-utils
            
            # 添加 Docker 仓库
            yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
            
            # 安装 Docker Engine
            yum install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
            
            # 启动 Docker
            systemctl start docker
            systemctl enable docker
            ;;
            
        *)
            log_error "不支持的操作系统: $OS"
            exit 1
            ;;
    esac
    
    log_info "Docker 安装完成: $(docker --version)"
}

# 安装 Git
install_git() {
    log_step "检查 Git 安装状态..."
    
    if command -v git &> /dev/null; then
        log_info "Git 已安装: $(git --version)"
        return 0
    fi
    
    log_step "开始安装 Git..."
    
    case $OS in
        ubuntu|debian)
            apt-get update
            apt-get install -y git
            ;;
        centos|rhel)
            yum install -y git
            ;;
    esac
    
    log_info "Git 安装完成: $(git --version)"
}

# 克隆项目
clone_project() {
    log_step "克隆 NOFX 项目..."
    
    # 询问用户选择仓库
    echo ""
    echo "请选择要克隆的仓库："
    echo "1) 您的 Fork 仓库 (https://github.com/LIULIBAO123/nofx.git)"
    echo "2) 原始仓库 (https://github.com/NoFxAiOS/nofx.git)"
    echo "3) 自定义仓库地址"
    read -p "请输入选项 [1-3]: " repo_choice
    
    case $repo_choice in
        1)
            REPO_URL="https://github.com/LIULIBAO123/nofx.git"
            ;;
        2)
            REPO_URL="https://github.com/NoFxAiOS/nofx.git"
            ;;
        3)
            read -p "请输入仓库地址: " REPO_URL
            ;;
        *)
            log_error "无效的选项"
            exit 1
            ;;
    esac
    
    # 询问安装目录
    read -p "请输入安装目录 [默认: /opt/nofx]: " INSTALL_DIR
    INSTALL_DIR=${INSTALL_DIR:-/opt/nofx}
    
    # 如果目录已存在，询问是否删除
    if [ -d "$INSTALL_DIR" ]; then
        log_warn "目录 $INSTALL_DIR 已存在"
        read -p "是否删除并重新克隆? [y/N]: " confirm
        if [[ $confirm == [yY] ]]; then
            rm -rf "$INSTALL_DIR"
        else
            log_info "使用现有目录"
            cd "$INSTALL_DIR"
            git pull
            return 0
        fi
    fi
    
    # 克隆项目
    git clone "$REPO_URL" "$INSTALL_DIR"
    cd "$INSTALL_DIR"
    
    # 询问分支
    read -p "请输入要使用的分支 [默认: dev]: " BRANCH
    BRANCH=${BRANCH:-dev}
    git checkout "$BRANCH"
    
    log_info "项目克隆完成"
}

# 配置环境变量
configure_env() {
    log_step "配置环境变量..."
    
    cd "$INSTALL_DIR"
    
    # 检查是否已有 .env 文件
    if [ -f .env ]; then
        log_warn ".env 文件已存在"
        read -p "是否重新配置? [y/N]: " confirm
        if [[ ! $confirm == [yY] ]]; then
            log_info "跳过环境变量配置"
            return 0
        fi
    fi
    
    # 如果有 .env.example，复制它
    if [ -f .env.example ]; then
        log_info "从 .env.example 创建 .env 文件..."
        cp .env.example .env
    else
        # 创建 .env 文件
        log_info "创建 .env 文件..."
        cat > .env << 'EOF'
# NOFX 交易系统环境变量配置

# 数据库配置
DB_PATH=./data/nofx.db

# 服务端口
API_PORT=8080
FRONTEND_PORT=3000

# 日志级别 (debug, info, warn, error)
LOG_LEVEL=info

# JWT 密钥 (请修改为随机字符串)
JWT_SECRET=nofx-default-secret-please-change-this-in-production

# 是否启用 HTTPS
ENABLE_HTTPS=false

# HTTPS 证书路径 (如果启用 HTTPS)
SSL_CERT_PATH=
SSL_KEY_PATH=

# 时区
TZ=Asia/Shanghai

# 数据目录
DATA_DIR=./data

# 日志目录
LOG_DIR=./logs
EOF
    fi
    
    # 创建必要的目录
    mkdir -p "$INSTALL_DIR/data"
    mkdir -p "$INSTALL_DIR/logs"
    mkdir -p "$INSTALL_DIR/keys"
    
    log_info "环境变量配置完成"
    log_warn "请编辑 $INSTALL_DIR/.env 文件，修改 JWT_SECRET 等敏感信息"
}

# 生成 RSA 密钥
generate_rsa_keys() {
    log_step "生成 RSA 密钥..."
    
    cd "$INSTALL_DIR"
    
    # 检查是否已存在有效的密钥
    if [ -f "keys/private.pem" ]; then
        log_info "检测到现有私钥，验证中..."
        if openssl rsa -in keys/private.pem -check -noout 2>/dev/null; then
            log_info "✓ 现有 RSA 私钥有效，跳过生成"
            return 0
        else
            log_warn "✗ 现有私钥无效，将重新生成"
        fi
    fi
    
    # 检查 openssl 是否安装
    if ! command -v openssl &> /dev/null; then
        log_warn "openssl 未安装，正在安装..."
        case $OS in
            ubuntu|debian)
                apt-get update && apt-get install -y openssl
                ;;
            centos|rhel)
                yum install -y openssl
                ;;
        esac
    fi
    
    # 生成 RSA 密钥对
    log_info "生成 RSA 密钥对（2048位）..."
    openssl genrsa -out keys/private.pem 2048 2>/dev/null
    
    if [ $? -ne 0 ]; then
        log_error "生成私钥失败"
        exit 1
    fi
    
    log_info "生成公钥..."
    openssl rsa -in keys/private.pem -pubout -out keys/public.pem 2>/dev/null
    
    if [ $? -ne 0 ]; then
        log_error "生成公钥失败"
        exit 1
    fi
    
    # 设置权限
    chmod 600 keys/private.pem
    chmod 644 keys/public.pem
    
    # 验证密钥
    if openssl rsa -in keys/private.pem -check -noout 2>/dev/null; then
        log_info "✓ RSA 密钥生成并验证成功"
    else
        log_error "✗ RSA 密钥验证失败"
        exit 1
    fi
    
    log_info "密钥文件位置："
    log_info "  私钥: $INSTALL_DIR/keys/private.pem"
    log_info "  公钥: $INSTALL_DIR/keys/public.pem"
}

# 配置防火墙
configure_firewall() {
    log_step "配置防火墙..."
    
    # 询问是否配置防火墙
    read -p "是否配置防火墙规则? [Y/n]: " confirm
    if [[ $confirm == [nN] ]]; then
        log_info "跳过防火墙配置"
        return 0
    fi
    
    # 检测防火墙类型
    if command -v ufw &> /dev/null; then
        # Ubuntu/Debian 使用 ufw
        log_info "检测到 UFW 防火墙"
        ufw allow 8080/tcp comment 'NOFX API'
        ufw allow 3000/tcp comment 'NOFX Frontend'
        ufw allow 22/tcp comment 'SSH'
        ufw --force enable
        log_info "UFW 防火墙规则已配置"
        
    elif command -v firewall-cmd &> /dev/null; then
        # CentOS/RHEL 使用 firewalld
        log_info "检测到 Firewalld 防火墙"
        firewall-cmd --permanent --add-port=8080/tcp
        firewall-cmd --permanent --add-port=3000/tcp
        firewall-cmd --reload
        log_info "Firewalld 防火墙规则已配置"
        
    else
        log_warn "未检测到防火墙，请手动配置以下端口："
        log_warn "  - 8080 (API 端口)"
        log_warn "  - 3000 (前端端口)"
    fi
}

# 登录 GHCR (如果需要拉取私有镜像)
login_ghcr() {
    log_step "检查是否需要登录 GHCR..."
    
    # 检查 docker-compose.prod.yml 是否使用 GHCR 镜像
    if [ ! -f "$INSTALL_DIR/docker-compose.prod.yml" ]; then
        return 0
    fi
    
    if ! grep -q "ghcr.io" "$INSTALL_DIR/docker-compose.prod.yml"; then
        return 0
    fi
    
    log_info "检测到使用 GHCR 私有镜像，需要登录认证"
    
    # 方法1: 使用环境变量 GITHUB_PAT 和 GITHUB_USERNAME
    if [ -n "$GITHUB_PAT" ] && [ -n "$GITHUB_USERNAME" ]; then
        log_info "使用环境变量中的 PAT 登录 GHCR..."
        echo "$GITHUB_PAT" | docker login ghcr.io -u "$GITHUB_USERNAME" --password-stdin
        if [ $? -eq 0 ]; then
            log_info "✓ GHCR 登录成功"
            return 0
        else
            log_error "✗ GHCR 登录失败，请检查 PAT 和用户名"
        fi
    fi
    
    # 方法2: 使用命令行参数（如果通过函数参数传入）
    if [ -n "$1" ] && [ -n "$2" ]; then
        log_info "使用命令行参数中的 PAT 登录 GHCR..."
        echo "$1" | docker login ghcr.io -u "$2" --password-stdin
        if [ $? -eq 0 ]; then
            log_info "✓ GHCR 登录成功"
            return 0
        else
            log_error "✗ GHCR 登录失败，请检查 PAT 和用户名"
        fi
    fi
    
    # 方法3: 交互式输入
    read -p "是否使用 Personal Access Token (PAT) 登录 GHCR? [Y/n]: " confirm
    if [[ ! $confirm == [nN] ]]; then
        read -p "请输入 GitHub 用户名: " github_username
        read -sp "请输入 Personal Access Token: " github_pat
        echo ""
        if [ -n "$github_username" ] && [ -n "$github_pat" ]; then
            echo "$github_pat" | docker login ghcr.io -u "$github_username" --password-stdin
            if [ $? -eq 0 ]; then
                log_info "✓ GHCR 登录成功"
            else
                log_error "✗ GHCR 登录失败"
                exit 1
            fi
        else
            log_error "✗ 用户名或 PAT 为空"
            exit 1
        fi
    else
        log_warn "跳过 GHCR 登录，假设已登录或使用本地构建"
    fi
}

# 构建并启动服务
start_services() {
    log_step "构建并启动服务..."
    
    cd "$INSTALL_DIR"
    
    # 检查是否使用预构建镜像
    if [ -f "docker-compose.prod.yml" ]; then
        log_info "检测到 docker-compose.prod.yml，使用预构建镜像"
        COMPOSE_FILE="docker-compose.prod.yml"
    else
        log_info "使用 docker-compose.yml，需要构建镜像"
        COMPOSE_FILE="docker-compose.yml"
        # 构建镜像
        log_info "开始构建 Docker 镜像（这可能需要几分钟）..."
        docker compose -f "$COMPOSE_FILE" build
    fi
    
    # 启动服务
    log_info "启动服务..."
    docker compose -f "$COMPOSE_FILE" up -d
    
    # 等待服务启动
    log_info "等待服务启动..."
    sleep 10
    
    # 检查服务状态
    if [ -f "docker-compose.prod.yml" ]; then
        docker compose -f docker-compose.prod.yml ps
    else
        docker compose ps
    fi
    
    log_info "服务启动完成"
}

# 显示访问信息
show_access_info() {
    log_step "部署完成！"
    
    # 获取服务器 IP
    SERVER_IP=$(curl -s ifconfig.me || curl -s icanhazip.com || echo "YOUR_SERVER_IP")
    
    echo ""
    echo "=========================================="
    echo "  NOFX 交易系统部署成功！"
    echo "=========================================="
    echo ""
    echo "访问地址："
    echo "  前端: http://$SERVER_IP:3000"
    echo "  API:  http://$SERVER_IP:8080"
    echo ""
    echo "常用命令："
    if [ -f "$INSTALL_DIR/docker-compose.prod.yml" ]; then
        echo "  查看日志:   cd $INSTALL_DIR && docker compose -f docker-compose.prod.yml logs -f"
        echo "  停止服务:   cd $INSTALL_DIR && docker compose -f docker-compose.prod.yml down"
        echo "  启动服务:   cd $INSTALL_DIR && docker compose -f docker-compose.prod.yml up -d"
        echo "  重启服务:   cd $INSTALL_DIR && docker compose -f docker-compose.prod.yml restart"
        echo "  查看状态:   cd $INSTALL_DIR && docker compose -f docker-compose.prod.yml ps"
    else
        echo "  查看日志:   cd $INSTALL_DIR && docker compose logs -f"
        echo "  停止服务:   cd $INSTALL_DIR && docker compose down"
        echo "  启动服务:   cd $INSTALL_DIR && docker compose up -d"
        echo "  重启服务:   cd $INSTALL_DIR && docker compose restart"
        echo "  查看状态:   cd $INSTALL_DIR && docker compose ps"
    fi
    echo ""
    echo "配置文件："
    echo "  环境变量:   $INSTALL_DIR/.env"
    echo "  数据目录:   $INSTALL_DIR/data"
    echo ""
    echo "=========================================="
    echo ""
}

# 创建管理脚本
create_management_scripts() {
    log_step "创建管理脚本..."
    
    # 创建启动脚本
    cat > /usr/local/bin/nofx-start << EOF
#!/bin/bash
cd $INSTALL_DIR
docker compose up -d
echo "NOFX 服务已启动"
EOF
    chmod +x /usr/local/bin/nofx-start
    
    # 创建停止脚本
    cat > /usr/local/bin/nofx-stop << EOF
#!/bin/bash
cd $INSTALL_DIR
docker compose down
echo "NOFX 服务已停止"
EOF
    chmod +x /usr/local/bin/nofx-stop
    
    # 创建重启脚本
    cat > /usr/local/bin/nofx-restart << EOF
#!/bin/bash
cd $INSTALL_DIR
docker compose restart
echo "NOFX 服务已重启"
EOF
    chmod +x /usr/local/bin/nofx-restart
    
    # 创建日志查看脚本
    cat > /usr/local/bin/nofx-logs << EOF
#!/bin/bash
cd $INSTALL_DIR
docker compose logs -f
EOF
    chmod +x /usr/local/bin/nofx-logs
    
    # 创建更新脚本
    cat > /usr/local/bin/nofx-update << EOF
#!/bin/bash
cd $INSTALL_DIR
echo "拉取最新代码..."
git pull
echo "重新构建镜像..."
docker compose build
echo "重启服务..."
docker compose up -d
echo "NOFX 已更新到最新版本"
EOF
    chmod +x /usr/local/bin/nofx-update
    
    log_info "管理脚本创建完成"
    log_info "可以使用以下命令管理服务："
    log_info "  nofx-start   - 启动服务"
    log_info "  nofx-stop    - 停止服务"
    log_info "  nofx-restart - 重启服务"
    log_info "  nofx-logs    - 查看日志"
    log_info "  nofx-update  - 更新到最新版本"
}

# 配置开机自启
configure_autostart() {
    log_step "配置开机自启..."
    
    read -p "是否配置开机自启? [Y/n]: " confirm
    if [[ $confirm == [nN] ]]; then
        log_info "跳过开机自启配置"
        return 0
    fi
    
    # 创建 systemd 服务
    cat > /etc/systemd/system/nofx.service << EOF
[Unit]
Description=NOFX Trading System
Requires=docker.service
After=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=$INSTALL_DIR
ExecStart=/usr/bin/docker compose up -d
ExecStop=/usr/bin/docker compose down
TimeoutStartSec=0

[Install]
WantedBy=multi-user.target
EOF
    
    # 重载 systemd
    systemctl daemon-reload
    
    # 启用服务
    systemctl enable nofx.service
    
    log_info "开机自启配置完成"
}

# 主函数
main() {
    echo ""
    echo "=========================================="
    echo "  NOFX 交易系统 - 云服务器一键部署"
    echo "=========================================="
    echo ""
    
    # 检查 root 权限
    check_root
    
    # 检测操作系统
    detect_os
    
    # 安装依赖
    install_docker
    install_git
    
    # 克隆项目
    clone_project
    
    # 配置环境
    configure_env
    
    # 生成 RSA 密钥
    generate_rsa_keys
    
    # 登录 GHCR (如果需要)
    login_ghcr
    
    # 配置防火墙
    configure_firewall
    
    # 启动服务
    start_services
    
    # 创建管理脚本
    create_management_scripts
    
    # 配置开机自启
    configure_autostart
    
    # 显示访问信息
    show_access_info
}

# 运行主函数
main

