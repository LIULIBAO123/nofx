#!/bin/bash

# ============================================================================
# NOFX 服务器更新脚本 - 修复 RSA 密钥问题
# ============================================================================

set -e

echo "=========================================="
echo "  NOFX 服务器更新 - 修复 RSA 密钥问题"
echo "=========================================="
echo ""

# 进入项目目录
cd /opt/nofx

echo "1. 停止服务..."
docker compose -f docker-compose.prod.yml down

echo ""
echo "2. 拉取最新代码..."
git fetch origin
git pull origin dev

echo ""
echo "3. 检查当前版本..."
git log --oneline -1

echo ""
echo "4. 创建 keys 目录..."
mkdir -p keys

echo ""
echo "5. 生成 RSA 密钥..."
if [ ! -f "keys/private.pem" ]; then
    openssl genrsa -out keys/private.pem 2048 2>/dev/null
    openssl rsa -in keys/private.pem -pubout -out keys/public.pem 2>/dev/null
    chmod 600 keys/private.pem
    chmod 644 keys/public.pem
    echo "✓ RSA 密钥生成成功"
else
    echo "✓ RSA 密钥已存在"
    # 验证密钥
    if openssl rsa -in keys/private.pem -check -noout 2>/dev/null; then
        echo "✓ RSA 密钥验证成功"
    else
        echo "✗ RSA 密钥无效，重新生成..."
        rm -f keys/private.pem keys/public.pem
        openssl genrsa -out keys/private.pem 2048 2>/dev/null
        openssl rsa -in keys/private.pem -pubout -out keys/public.pem 2>/dev/null
        chmod 600 keys/private.pem
        chmod 644 keys/public.pem
        echo "✓ RSA 密钥重新生成成功"
    fi
fi

echo ""
echo "6. 检查 docker-compose.prod.yml 配置..."
if grep -q "./keys:/app/keys" docker-compose.prod.yml; then
    echo "✓ docker-compose.prod.yml 配置正确"
else
    echo "✗ docker-compose.prod.yml 缺少 keys 目录挂载"
    echo "请手动编辑 docker-compose.prod.yml"
    exit 1
fi

echo ""
echo "7. 启动服务..."
docker compose -f docker-compose.prod.yml up -d

echo ""
echo "=========================================="
echo "  更新完成！"
echo "=========================================="
echo ""
echo "等待服务启动（30秒）..."
sleep 30

echo ""
echo "检查服务状态..."
docker compose -f docker-compose.prod.yml ps

echo ""
echo "查看最新日志..."
docker compose -f docker-compose.prod.yml logs --tail=30 nofx

echo ""
echo "=========================================="
echo "  验证步骤"
echo "=========================================="
echo ""
echo "1. 检查日志中是否有 '✅ Encryption service initialized successfully'"
echo "2. 测试健康检查："
echo "   curl http://localhost:8080/api/health"
echo ""
echo "3. 如果仍有错误，查看完整日志："
echo "   docker compose -f docker-compose.prod.yml logs -f nofx"
echo ""

