#!/usr/bin/env bash
# NOFX 本地 Docker 一键部署
# 用法: bash scripts/deploy-local.sh  或  ./scripts/deploy-local.sh

set -e
cd "$(dirname "$0")/.."

echo "=== NOFX 本地 Docker 部署 ==="

# 1. .env
if [ ! -f .env ]; then
  echo "[1/4] 创建 .env（从 .env.example 复制）"
  cp .env.example .env
  echo "      请编辑 .env 设置 JWT_SECRET、DATA_ENCRYPTION_KEY 等后重新运行此脚本。"
  echo "      若仅本地试用，可直接继续：docker 会使用当前 .env。"
else
  echo "[1/4] .env 已存在，跳过"
fi

# 2. data 目录
mkdir -p data
echo "[2/4] 数据目录 data/ 已就绪"

# 3. RSA 密钥（用于 API 传输加密）
if [ ! -f keys/private.pem ]; then
  echo "[3/4] 生成 RSA 密钥到 keys/"
  mkdir -p keys
  if command -v openssl &>/dev/null; then
    openssl genrsa -out keys/private.pem 2048
    openssl rsa -in keys/private.pem -pubout -out keys/public.pem
    chmod 600 keys/private.pem 2>/dev/null || true
    chmod 644 keys/public.pem 2>/dev/null || true
    echo "      keys/private.pem 与 keys/public.pem 已生成"
  else
    echo "      未找到 openssl。请手动执行："
    echo "        mkdir -p keys"
    echo "        openssl genrsa -out keys/private.pem 2048"
    echo "        openssl rsa -in keys/private.pem -pubout -out keys/public.pem"
    exit 1
  fi
else
  echo "[3/4] keys/private.pem 已存在，跳过"
fi

# 4. 构建并启动
echo "[4/4] 构建并启动 Docker 服务..."
if docker compose version &>/dev/null; then
  docker compose up -d --build
elif command -v docker-compose &>/dev/null; then
  docker-compose up -d --build
else
  echo "未找到 docker compose 或 docker-compose，请先安装 Docker。"
  exit 1
fi

echo ""
echo "=== 部署完成 ==="
echo "  前端: http://localhost:${NOFX_FRONTEND_PORT:-3000}"
echo "  后端: http://localhost:${NOFX_BACKEND_PORT:-8080}/api/health"
echo "  查看日志: docker compose logs -f"
echo "  停止: docker compose down"
