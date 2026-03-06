#!/bin/bash
#
# NOFX One-Click Installation Script
# https://github.com/LIULIBAO123/nofx
#
# 首次部署（空目录）:
#   curl -fsSL https://raw.githubusercontent.com/LIULIBAO123/nofx/dev/install.sh | bash
#
# 指定目录首次部署:
#   curl -fsSL https://raw.githubusercontent.com/LIULIBAO123/nofx/dev/install.sh | bash -s -- /opt/nofx
#
# 后续更新（在已安装目录下执行）:
#   cd $HOME/nofx && curl -fsSL https://raw.githubusercontent.com/LIULIBAO123/nofx/dev/install.sh | bash -s -- $HOME/nofx
#
# 详见: docs/云服务器一键部署命令.md
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Default installation directory
INSTALL_DIR="${1:-$HOME/nofx}"

echo -e "${BLUE}"
echo "╔════════════════════════════════════════════════════════════╗"
echo "║                    NOFX AI Trading OS                      ║"
echo "║                   One-Click Installation                   ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo -e "${NC}"

# Check Docker
check_docker() {
    echo -e "${YELLOW}Checking Docker...${NC}"
    if ! command -v docker &> /dev/null; then
        echo -e "${RED}Error: Docker is not installed.${NC}"
        echo "Please install Docker first: https://docs.docker.com/get-docker/"
        exit 1
    fi

    if ! docker info &> /dev/null; then
        echo -e "${RED}Error: Docker daemon is not running.${NC}"
        echo "Please start Docker and try again."
        exit 1
    fi

    # Check Docker Compose
    if docker compose version &> /dev/null; then
        COMPOSE_CMD="docker compose"
    elif command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
    else
        echo -e "${RED}Error: Docker Compose is not available.${NC}"
        echo "Please install Docker Compose: https://docs.docker.com/compose/install/"
        exit 1
    fi

    echo -e "${GREEN}✓ Docker is ready${NC}"
}

# Create installation directory (do not create data/logs/keys here so clone can run in empty dir)
setup_directory() {
    echo -e "${YELLOW}Setting up installation directory: ${INSTALL_DIR}${NC}"
    mkdir -p "$INSTALL_DIR"
    cd "$INSTALL_DIR"
    echo -e "${GREEN}✓ Directory ready${NC}"
}

# Ensure data/keys/logs exist and generate RSA keys for backend
ensure_rsa_keys() {
    mkdir -p data logs keys
    if [ -f "keys/private.pem" ] && [ -f "keys/public.pem" ]; then
        echo -e "${GREEN}✓ RSA keys already exist in keys/${NC}"
        return
    fi
    echo -e "${YELLOW}Generating RSA keys in keys/ ...${NC}"
    if command -v openssl &> /dev/null; then
        openssl genrsa -out keys/private.pem 2048 2>/dev/null
        openssl rsa -in keys/private.pem -pubout -out keys/public.pem 2>/dev/null
        chmod 600 keys/private.pem 2>/dev/null || true
        echo -e "${GREEN}✓ RSA keys generated${NC}"
    else
        echo -e "${YELLOW}openssl not found; create keys/private.pem and keys/public.pem manually if backend requires them${NC}"
    fi
}

# Clone or update repo (build from source so one-click install gets latest preset v3.0)
clone_or_pull_repo() {
    if ! command -v git &> /dev/null; then
        echo -e "${RED}Error: git is required. Install with: apt-get install -y git (Debian/Ubuntu) or yum install -y git (CentOS)${NC}"
        exit 1
    fi
    echo -e "${YELLOW}Fetching NOFX source (dev branch)...${NC}"
    if [ -d ".git" ]; then
        git fetch origin dev
        git reset --hard origin/dev
        git checkout -B dev origin/dev 2>/dev/null || git checkout dev
        echo -e "${GREEN}✓ Repo updated (dev)${NC}"
    else
        if [ -n "$(ls -A 2>/dev/null)" ]; then
            # 已有旧安装（无 .git）：克隆到子目录并沿用现有 .env/keys/data，实现原地升级
            echo -e "${YELLOW}Existing files detected; cloning into nofx-src/ and reusing .env, keys, data...${NC}"
            git clone -b dev --depth 1 https://github.com/LIULIBAO123/nofx.git nofx-src
            [ -f .env ] && cp -a .env nofx-src/
            [ -d keys ] && cp -a keys nofx-src/ 2>/dev/null || true
            [ -d data ] && cp -a data nofx-src/ 2>/dev/null || true
            [ -d logs ] && cp -a logs nofx-src/ 2>/dev/null || true
            cd nofx-src
            INSTALL_DIR="$(pwd)"
            echo -e "${GREEN}✓ Repo cloned to nofx-src (preset v3.0)${NC}"
        else
            git clone -b dev --depth 1 https://github.com/LIULIBAO123/nofx.git .
            echo -e "${GREEN}✓ Repo cloned (dev)${NC}"
        fi
    fi
}

# Generate encryption keys and create .env file
generate_env() {
    echo -e "${YELLOW}Generating encryption keys...${NC}"

    # Skip if .env already exists
    if [ -f ".env" ]; then
        echo -e "${GREEN}✓ .env file already exists, skipping key generation${NC}"
        return
    fi

    # Generate JWT secret (32 bytes, base64)
    JWT_SECRET=$(openssl rand -base64 32 2>/dev/null || echo "nofx-jwt-secret-$(date +%s)-please-change")

    # Generate AES data encryption key (32 bytes, base64)
    DATA_ENCRYPTION_KEY=$(openssl rand -base64 32 2>/dev/null || echo "nofx-aes-key-$(date +%s)-please-change")

    # Generate RSA private key (2048 bits)
    if command -v openssl &> /dev/null; then
    RSA_PRIVATE_KEY=$(openssl genrsa 2048 2>/dev/null | tr '\n' '\\' | sed 's/\\/\\n/g' | sed 's/\\n$//')
    else
        RSA_PRIVATE_KEY="RSA-KEY-NOT-GENERATED-PLEASE-CONFIGURE-MANUALLY"
    fi

    # Create .env file
    cat > .env << EOF
# NOFX Configuration (Auto-generated)
# Generated at: $(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Server ports
NOFX_BACKEND_PORT=8080
NOFX_FRONTEND_PORT=3000

# Timezone
TZ=Asia/Shanghai

# JWT signing secret
JWT_SECRET=${JWT_SECRET}

# AES-256 data encryption key (for encrypting API keys in database)
DATA_ENCRYPTION_KEY=${DATA_ENCRYPTION_KEY}

# RSA private key (for client-server encryption)
RSA_PRIVATE_KEY=${RSA_PRIVATE_KEY}
EOF

    echo -e "${GREEN}✓ Encryption keys generated${NC}"
}

# Build images from source (ensures latest preset v3.0 / one-click strategy is used)
build_images() {
    echo -e "${YELLOW}Building Docker images from source (this may take several minutes)...${NC}"
    $COMPOSE_CMD build --no-cache
    echo -e "${GREEN}✓ Images built${NC}"
}

# Ask user if they want to clear trading data
ask_clear_trading_data() {
    local db_file="data/data.db"

    # Only ask if database file exists
    if [ ! -f "$db_file" ]; then
        CLEAR_TRADING_DATA="no"
        return 0
    fi

    echo ""
    echo -e "${YELLOW}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${YELLOW}Do you want to clear trading data? (orders, fills, positions)${NC}"
    echo -e "${BLUE}  • trader_orders    (Order records)${NC}"
    echo -e "${BLUE}  • trader_fills     (Fill/execution records)${NC}"
    echo -e "${BLUE}  • trader_positions (Position records)${NC}"
    echo -e "${YELLOW}═══════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "${BLUE}Type 'yes' to clear tables, press Enter or any other input to skip${NC}"
    echo -n "Input: "
    read -r confirm < /dev/tty

    if [ "$confirm" == "yes" ]; then
        CLEAR_TRADING_DATA="yes"
        echo -e "${YELLOW}Trading data will be cleared after services start...${NC}"
    else
        CLEAR_TRADING_DATA="no"
        echo -e "${BLUE}Skipping data clear${NC}"
    fi
    echo ""
}

# Start services
start_services() {
    echo -e "${YELLOW}Starting NOFX services...${NC}"
    $COMPOSE_CMD up -d
    echo -e "${GREEN}✓ Services started${NC}"
}

# Clear trading data
clear_trading_data() {
    if [ "$CLEAR_TRADING_DATA" != "yes" ]; then
        return 0
    fi

    local db_file="data/data.db"

    if [ ! -f "$db_file" ]; then
        echo -e "${YELLOW}Database file not found, skipping...${NC}"
        return 0
    fi

    echo -e "${YELLOW}Clearing trading data tables...${NC}"

    if command -v sqlite3 &> /dev/null; then
        sqlite3 "$db_file" 'DELETE FROM trader_fills; DELETE FROM trader_orders; DELETE FROM trader_positions;' 2>/dev/null || true
        if [ $? -eq 0 ]; then
            echo -e "${GREEN}✓ Trading data tables cleared${NC}"
        else
            echo -e "${YELLOW}Note: Some tables may not exist yet${NC}"
        fi
    else
        echo -e "${YELLOW}sqlite3 not found. To clear data manually, install sqlite3 and run:${NC}"
        echo -e "${BLUE}  sqlite3 data/data.db 'DELETE FROM trader_fills; DELETE FROM trader_orders; DELETE FROM trader_positions;'${NC}"
    fi
}

# Wait for services
wait_for_services() {
    echo -e "${YELLOW}Waiting for services to be ready...${NC}"

    local max_attempts=30
    local attempt=1

    while [ $attempt -le $max_attempts ]; do
        if curl -s http://localhost:8080/api/health > /dev/null 2>&1; then
            echo -e "${GREEN}✓ Backend is ready${NC}"
            break
        fi
        echo "  Waiting for backend... ($attempt/$max_attempts)"
        sleep 2
        ((attempt++))
    done

    if [ $attempt -gt $max_attempts ]; then
        echo -e "${YELLOW}Backend is still starting, please wait a moment...${NC}"
    fi
}

# Get server IP for display
get_server_ip() {
    # Try to get public IP first
    local public_ip=$(curl -s --max-time 3 ifconfig.me 2>/dev/null || curl -s --max-time 3 icanhazip.com 2>/dev/null || echo "")

    # If no public IP, try local IP
    if [ -z "$public_ip" ]; then
        if command -v ip &> /dev/null; then
            public_ip=$(ip route get 1 2>/dev/null | awk '{print $7}' | head -1)
        elif command -v hostname &> /dev/null; then
            public_ip=$(hostname -I 2>/dev/null | awk '{print $1}')
        fi
    fi

    echo "${public_ip:-127.0.0.1}"
}

# Print success message
print_success() {
    local SERVER_IP=$(get_server_ip)

    echo ""
    echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗"
    echo -e "║              🎉 Installation Complete! 🎉                   ║"
    echo -e "╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "  ${BLUE}Web Interface:${NC}  http://${SERVER_IP}:3000"
    echo -e "  ${BLUE}API Endpoint:${NC}   http://${SERVER_IP}:8080"
    echo -e "  ${BLUE}Install Dir:${NC}    $INSTALL_DIR"
    echo ""
    echo -e "${CYAN}╔════════════════════════════════════════════════════════════╗"
    echo -e "║  💡 Keep Updated: Re-run this script to get latest code    ║"
    echo -e "╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "  ${GREEN}cd $INSTALL_DIR && curl -fsSL https://raw.githubusercontent.com/LIULIBAO123/nofx/dev/install.sh | bash -s -- $INSTALL_DIR${NC}"
    echo ""
    echo -e "  This will git pull and rebuild from source (preset v3.0 / one-click strategy)."
    echo ""
    echo -e "${YELLOW}Quick Commands:${NC}"
    echo "  cd $INSTALL_DIR"
    echo "  $COMPOSE_CMD logs -f       # View logs"
    echo "  $COMPOSE_CMD restart       # Restart services"
    echo "  $COMPOSE_CMD down          # Stop services"
    echo "  Re-run this script from $INSTALL_DIR to update to latest"
    echo ""
    echo -e "${YELLOW}Next Steps:${NC}"
    echo "  1. Open http://${SERVER_IP}:3000 in your browser"
    echo "  2. Configure AI Models (DeepSeek, OpenAI, etc.)"
    echo "  3. Configure Exchanges (Binance, Bybit, etc.)"
    echo "  4. Create a Strategy in Strategy Studio"
    echo "  5. Create a Trader and start trading!"
    echo ""
    echo -e "${YELLOW}Note:${NC} If accessing from local machine, use http://127.0.0.1:3000"
    echo ""
    echo -e "${RED}⚠️  Risk Warning: AI trading carries significant risks.${NC}"
    echo -e "${RED}   Only use funds you can afford to lose!${NC}"
    echo ""
}

# Main
main() {
    check_docker
    setup_directory
    clone_or_pull_repo
    ensure_rsa_keys
    generate_env
    build_images
    ask_clear_trading_data
    start_services
    wait_for_services
    clear_trading_data
    print_success
}

main
