#!/bin/bash

# ============================================================================
# NOFX RSA 密钥修复脚本
# ============================================================================

set -e

echo "=========================================="
echo "  修复 RSA 密钥问题"
echo "=========================================="
echo ""

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 创建 keys 目录
echo "1. 创建 keys 目录..."
mkdir -p keys

# 检查是否已存在有效的密钥
if [ -f "keys/private.pem" ]; then
    echo "检测到现有私钥，验证中..."
    if openssl rsa -in keys/private.pem -check -noout 2>/dev/null; then
        echo "✓ 现有 RSA 私钥有效"
        echo ""
        read -p "是否重新生成密钥？这将使现有加密数据无法解密 [y/N]: " confirm
        if [[ ! $confirm == [yY] ]]; then
            echo "保留现有密钥"
            exit 0
        fi
    else
        echo "✗ 现有私钥无效，将重新生成"
    fi
fi

# 生成 RSA 密钥对
echo "2. 生成 RSA 密钥对（2048位）..."
openssl genrsa -out keys/private.pem 2048 2>/dev/null

if [ $? -ne 0 ]; then
    echo "✗ 生成私钥失败"
    exit 1
fi

echo "3. 生成公钥..."
openssl rsa -in keys/private.pem -pubout -out keys/public.pem 2>/dev/null

if [ $? -ne 0 ]; then
    echo "✗ 生成公钥失败"
    exit 1
fi

# 设置权限
echo "4. 设置文件权限..."
chmod 600 keys/private.pem
chmod 644 keys/public.pem

# 验证密钥
echo "5. 验证密钥..."
if openssl rsa -in keys/private.pem -check -noout 2>/dev/null; then
    echo "✓ RSA 私钥验证成功"
else
    echo "✗ RSA 私钥验证失败"
    exit 1
fi

if openssl rsa -pubin -in keys/public.pem -text -noout 2>/dev/null; then
    echo "✓ RSA 公钥验证成功"
else
    echo "✗ RSA 公钥验证失败"
    exit 1
fi

echo ""
echo "=========================================="
echo "  密钥生成完成！"
echo "=========================================="
echo ""
echo "密钥文件位置："
echo "  私钥: $(pwd)/keys/private.pem"
echo "  公钥: $(pwd)/keys/public.pem"
echo ""
echo "⚠️  重要提示："
echo "  1. 请妥善保管私钥文件"
echo "  2. 不要将私钥提交到代码仓库"
echo "  3. 定期备份密钥文件"
echo ""

