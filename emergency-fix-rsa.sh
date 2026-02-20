#!/bin/bash

# ============================================================================
# NOFX RSA 密钥紧急修复脚本
# 用于修复服务器上的 RSA 密钥问题
# ============================================================================

set -e

echo "=========================================="
echo "  NOFX RSA 密钥紧急修复"
echo "=========================================="
echo ""

# 进入项目目录
cd /opt/nofx

# 1. 停止服务
echo "1. 停止服务..."
docker compose -f docker-compose.prod.yml down

# 2. 创建 keys 目录
echo "2. 创建 keys 目录..."
mkdir -p keys

# 3. 生成 RSA 密钥对
echo "3. 生成 RSA 密钥对..."
openssl genrsa -out keys/private.pem 2048 2>/dev/null
openssl rsa -in keys/private.pem -pubout -out keys/public.pem 2>/dev/null

# 4. 设置权限
echo "4. 设置文件权限..."
chmod 600 keys/private.pem
chmod 644 keys/public.pem

# 5. 验证密钥
echo "5. 验证密钥..."
if openssl rsa -in keys/private.pem -check -noout 2>/dev/null; then
    echo "✓ RSA 私钥验证成功"
else
    echo "✗ RSA 私钥验证失败"
    exit 1
fi

# 6. 更新 docker-compose.prod.yml（如果需要）
echo "6. 检查 docker-compose.prod.yml 配置..."
if ! grep -q "./keys:/app/keys" docker-compose.prod.yml; then
    echo "需要更新 docker-compose.prod.yml，添加 keys 目录挂载"
    echo "请手动编辑 docker-compose.prod.yml，在 volumes 部分添加："
    echo "      - ./keys:/app/keys"
fi

# 7. 启动服务
echo "7. 启动服务..."
docker compose -f docker-compose.prod.yml up -d

echo ""
echo "=========================================="
echo "  修复完成！"
echo "=========================================="
echo ""
echo "等待服务启动（30秒）..."
sleep 30

# 8. 检查服务状态
echo "检查服务状态..."
docker compose -f docker-compose.prod.yml ps

echo ""
echo "查看最新日志："
docker compose -f docker-compose.prod.yml logs --tail=20 nofx

echo ""
echo "如果仍有错误，请运行："
echo "  docker compose -f docker-compose.prod.yml logs -f nofx"

