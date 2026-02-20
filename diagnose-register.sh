#!/bin/bash

# NOFX 注册问题诊断脚本

echo "=========================================="
echo "  NOFX 注册问题诊断工具"
echo "=========================================="
echo ""

# 1. 检查服务状态
echo "1. 检查服务状态..."
docker compose -f docker-compose.prod.yml ps

echo ""
echo "2. 检查数据库连接..."
docker compose -f docker-compose.prod.yml exec nofx sh -c "ls -la /app/data/" 2>/dev/null || echo "无法访问数据目录"

echo ""
echo "3. 检查最近的错误日志..."
docker compose -f docker-compose.prod.yml logs nofx 2>&1 | grep -i "error\|failed\|panic" | tail -20

echo ""
echo "4. 测试注册 API..."
echo "发送测试注册请求..."

# 获取服务器 IP
SERVER_IP=$(curl -s ifconfig.me 2>/dev/null || echo "localhost")

# 测试注册 API
RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "Test123!@#"
  }' 2>&1)

HTTP_CODE=$(echo "$RESPONSE" | grep "HTTP_CODE:" | cut -d: -f2)
BODY=$(echo "$RESPONSE" | sed '/HTTP_CODE:/d')

echo "HTTP 状态码: $HTTP_CODE"
echo "响应内容:"
echo "$BODY" | jq . 2>/dev/null || echo "$BODY"

echo ""
echo "5. 检查系统配置..."
curl -s http://localhost:8080/api/system-config | jq . 2>/dev/null || echo "无法获取系统配置"

echo ""
echo "6. 检查数据库表..."
docker compose -f docker-compose.prod.yml exec nofx sh -c "sqlite3 /app/data/nofx.db '.tables'" 2>/dev/null || echo "无法访问数据库"

echo ""
echo "7. 检查 users 表结构..."
docker compose -f docker-compose.prod.yml exec nofx sh -c "sqlite3 /app/data/nofx.db '.schema users'" 2>/dev/null || echo "无法查询表结构"

echo ""
echo "8. 检查现有用户数量..."
docker compose -f docker-compose.prod.yml exec nofx sh -c "sqlite3 /app/data/nofx.db 'SELECT COUNT(*) FROM users;'" 2>/dev/null || echo "无法查询用户数"

echo ""
echo "=========================================="
echo "  诊断完成"
echo "=========================================="
echo ""
echo "常见问题解决方案："
echo ""
echo "1. 如果看到 'Server error' 或 500 错误："
echo "   - 检查数据库文件权限"
echo "   - 检查磁盘空间"
echo "   - 查看完整日志: docker compose -f docker-compose.prod.yml logs nofx"
echo ""
echo "2. 如果看到 'Registration is disabled'："
echo "   - 检查环境变量 REGISTRATION_ENABLED"
echo "   - 或修改 .env 文件"
echo ""
echo "3. 如果看到 'Not on whitelist'："
echo "   - 检查 MAX_USERS 配置"
echo "   - 或清理测试用户"
echo ""
echo "4. 如果看到数据库错误："
echo "   - 重启服务: docker compose -f docker-compose.prod.yml restart"
echo "   - 检查数据库文件: ls -la data/nofx.db"
echo ""

