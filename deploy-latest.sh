#!/bin/bash

# ============================================================================
# NOFX 最新版一键部署脚本
# 包含 RSA 密钥自动生成和 AI 交易分析功能
# ============================================================================

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  $1${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
}

# 检查是否为 root 用户
check_root() {
    if [ "$EUID" -ne 0 ]; then 
        print_error "请使用 root 用户运行此脚本"
        print_info "使用命令: sudo bash $0"
        exit 1
    fi
}

# 检查系统要求
check_requirements() {
    print_header "检查系统要求"
    
    # 检查操作系统
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        print_info "操作系统: $NAME $VERSION"
    else
        print_error "无法识别操作系统"
        exit 1
    fi
    
    # 检查必要命令
    local required_commands=("curl" "git" "openssl")
    for cmd in "${required_commands[@]}"; do
        if ! command -v $cmd &> /dev/null; then
            print_error "缺少必要命令: $cmd"
            print_info "正在安装..."
            apt-get update -qq
            apt-get install -y $cmd
        else
            print_success "✓ $cmd 已安装"
        fi
    done
}

# 安装 Docker
install_docker() {
    print_header "安装 Docker"
    
    if command -v docker &> /dev/null; then
        print_success "Docker 已安装: $(docker --version)"
        return
    fi
    
    print_info "正在安装 Docker..."
    
    # 卸载旧版本
    apt-get remove -y docker docker-engine docker.io containerd runc 2>/dev/null || true
    
    # 安装依赖
    apt-get update
    apt-get install -y \
        ca-certificates \
        curl \
        gnupg \
        lsb-release
    
    # 添加 Docker 官方 GPG key
    mkdir -p /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    
    # 设置仓库
    echo \
      "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
      $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
    
    # 安装 Docker Engine
    apt-get update
    apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
    
    # 启动 Docker
    systemctl start docker
    systemctl enable docker
    
    print_success "Docker 安装完成: $(docker --version)"
}

# 克隆或更新代码
setup_code() {
    print_header "设置代码仓库"
    
    local install_dir="/opt/nofx"
    
    if [ -d "$install_dir" ]; then
        print_warning "检测到已存在的安装目录: $install_dir"
        read -p "是否删除并重新安装? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            print_info "停止现有服务..."
            cd "$install_dir"
            docker compose -f docker-compose.prod.yml down 2>/dev/null || true
            
            print_info "删除旧目录..."
            cd /opt
            rm -rf "$install_dir"
        else
            print_info "更新现有代码..."
            cd "$install_dir"
            git fetch origin
            git reset --hard origin/dev
            git pull origin dev
            print_success "代码更新完成"
            return
        fi
    fi
    
    print_info "克隆代码仓库..."
    cd /opt
    git clone -b dev https://github.com/LIULIBAO123/nofx.git
    cd nofx
    
    print_success "代码设置完成"
}

# 生成 RSA 密钥
generate_rsa_keys() {
    print_header "生成 RSA 密钥"
    
    local keys_dir="/opt/nofx/keys"
    mkdir -p "$keys_dir"
    
    if [ -f "$keys_dir/private.pem" ]; then
        print_warning "RSA 密钥已存在"
        
        # 验证现有密钥
        if openssl rsa -in "$keys_dir/private.pem" -check -noout 2>/dev/null; then
            print_success "现有密钥验证通过"
            return
        else
            print_warning "现有密钥无效，重新生成..."
            rm -f "$keys_dir/private.pem" "$keys_dir/public.pem"
        fi
    fi
    
    print_info "生成新的 RSA 密钥对..."
    openssl genrsa -out "$keys_dir/private.pem" 2048 2>/dev/null
    openssl rsa -in "$keys_dir/private.pem" -pubout -out "$keys_dir/public.pem" 2>/dev/null
    
    # 设置权限
    chmod 600 "$keys_dir/private.pem"
    chmod 644 "$keys_dir/public.pem"
    
    # 验证密钥
    if openssl rsa -in "$keys_dir/private.pem" -check -noout 2>/dev/null; then
        print_success "RSA 密钥生成并验证成功"
        print_info "私钥: $keys_dir/private.pem"
        print_info "公钥: $keys_dir/public.pem"
    else
        print_error "RSA 密钥验证失败"
        exit 1
    fi
}

# 配置环境变量
configure_env() {
    print_header "配置环境变量"
    
    cd /opt/nofx
    
    if [ -f ".env" ]; then
        print_warning ".env 文件已存在"
        read -p "是否重新配置? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "保留现有配置"
            return
        fi
    fi
    
    print_info "创建 .env 配置文件..."
    
    # 生成随机密钥
    local jwt_secret=$(openssl rand -base64 32)
    local data_key=$(openssl rand -base64 32)
    
    cat > .env << EOF
# ============================================================================
# NOFX 环境配置
# ============================================================================

# 应用配置
APP_ENV=production
APP_PORT=8080
APP_HOST=0.0.0.0

# JWT 配置
JWT_SECRET=$jwt_secret
JWT_EXPIRY=24h

# 数据库配置
DB_TYPE=sqlite
DB_PATH=./data/nofx.db

# 加密配置
DATA_ENCRYPTION_KEY=$data_key
RSA_PRIVATE_KEY_PATH=/app/keys/private.pem
RSA_PUBLIC_KEY_PATH=/app/keys/public.pem

# 日志配置
LOG_LEVEL=info
LOG_FILE=./logs/nofx.log

# CORS 配置
CORS_ALLOWED_ORIGINS=*

# AI 配置（可选，用于交易分析功能）
# MCP_SERVER_URL=http://your-mcp-server:port
# MCP_API_KEY=your-api-key

# 其他配置
TZ=Asia/Shanghai
EOF
    
    chmod 600 .env
    print_success ".env 配置完成"
}

# 创建必要目录
create_directories() {
    print_header "创建必要目录"
    
    cd /opt/nofx
    
    local dirs=("data" "logs" "keys")
    for dir in "${dirs[@]}"; do
        mkdir -p "$dir"
        print_success "✓ 创建目录: $dir"
    done
}

# 启动服务
start_services() {
    print_header "启动服务"
    
    cd /opt/nofx
    
    print_info "拉取 Docker 镜像..."
    docker compose -f docker-compose.prod.yml pull
    
    print_info "启动容器..."
    docker compose -f docker-compose.prod.yml up -d
    
    print_success "服务启动完成"
}

# 等待服务就绪
wait_for_services() {
    print_header "等待服务就绪"
    
    local max_attempts=30
    local attempt=0
    
    while [ $attempt -lt $max_attempts ]; do
        if curl -s http://localhost:8080/api/health > /dev/null 2>&1; then
            print_success "服务已就绪！"
            return
        fi
        
        attempt=$((attempt + 1))
        echo -n "."
        sleep 2
    done
    
    echo ""
    print_warning "服务启动超时，请检查日志"
}

# 显示服务状态
show_status() {
    print_header "服务状态"
    
    cd /opt/nofx
    docker compose -f docker-compose.prod.yml ps
    
    echo ""
    print_info "查看日志: docker compose -f docker-compose.prod.yml logs -f"
    print_info "停止服务: docker compose -f docker-compose.prod.yml down"
    print_info "重启服务: docker compose -f docker-compose.prod.yml restart"
}

# 显示访问信息
show_access_info() {
    print_header "访问信息"
    
    local server_ip=$(curl -s ifconfig.me 2>/dev/null || echo "YOUR_SERVER_IP")
    
    echo ""
    print_success "🎉 NOFX 部署完成！"
    echo ""
    print_info "访问地址:"
    echo "  - 本地: http://localhost:8080"
    echo "  - 外网: http://$server_ip:8080"
    echo ""
    print_info "健康检查:"
    echo "  curl http://localhost:8080/api/health"
    echo ""
    print_info "注册账号:"
    echo "  访问 http://$server_ip:8080 并点击注册"
    echo ""
    print_warning "重要提示:"
    echo "  1. 请确保防火墙开放 8080 端口"
    echo "  2. 首次使用需要注册账号"
    echo "  3. 注册后会生成 OTP 二维码，请使用 Google Authenticator 扫描"
    echo "  4. RSA 密钥位于 /opt/nofx/keys/ 目录，请妥善保管"
    echo ""
    print_info "功能特性:"
    echo "  ✓ 用户注册/登录（OTP 双因素认证）"
    echo "  ✓ 交易所 API 管理（加密存储）"
    echo "  ✓ 回测实验室"
    echo "  ✓ AI 交易分析（需配置 MCP）"
    echo "  ✓ 策略管理"
    echo ""
}

# 主函数
main() {
    clear
    
    cat << "EOF"
╔════════════════════════════════════════════════════════════╗
║                                                            ║
║           🚀 NOFX - AI-Powered Trading System              ║
║                                                            ║
║                    一键部署脚本 v2.0                        ║
║                                                            ║
╚════════════════════════════════════════════════════════════╝
EOF
    
    echo ""
    print_info "开始部署 NOFX 交易系统..."
    echo ""
    
    # 执行部署步骤
    check_root
    check_requirements
    install_docker
    setup_code
    generate_rsa_keys
    configure_env
    create_directories
    start_services
    wait_for_services
    show_status
    show_access_info
    
    print_success "部署完成！"
}

# 运行主函数
main

