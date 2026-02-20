# NOFX 注册问题排查指南

## 问题描述
注册账号时遇到错误：`[REGISTRATION_ERROR]: Server error`

## 排查步骤

### 1. 检查服务日志

```bash
# 查看最近的日志
docker compose -f docker-compose.prod.yml logs --tail=100 nofx

# 实时查看日志
docker compose -f docker-compose.prod.yml logs -f nofx

# 查找错误日志
docker compose -f docker-compose.prod.yml logs nofx | grep -i "error\|failed\|panic"
```

### 2. 测试注册 API

```bash
# 直接测试注册接口
curl -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "Test123!@#"
  }'
```

预期响应：
```json
{
  "user_id": "xxx-xxx-xxx",
  "email": "test@example.com",
  "otp_secret": "XXXXX",
  "qr_code_url": "otpauth://...",
  "message": "Please scan the QR code..."
}
```

### 3. 检查系统配置

```bash
# 检查系统配置
curl http://localhost:8080/api/system-config
```

确认以下配置：
- `registration_enabled`: 应该为 `true`
- `max_users`: 如果设置了限制，检查是否已满

### 4. 检查数据库

```bash
# 进入容器
docker compose -f docker-compose.prod.yml exec nofx sh

# 检查数据库文件
ls -la /app/data/nofx.db

# 查看表结构
sqlite3 /app/data/nofx.db ".schema users"

# 查看现有用户
sqlite3 /app/data/nofx.db "SELECT id, email, otp_verified FROM users;"

# 查看用户数量
sqlite3 /app/data/nofx.db "SELECT COUNT(*) FROM users;"
```

### 5. 常见问题及解决方案

#### 问题 1：数据库权限错误

**症状**：日志中出现 `unable to open database file` 或 `attempt to write a readonly database`

**解决方案**：
```bash
# 检查数据目录权限
ls -la data/

# 修复权限
sudo chown -R 1000:1000 data/
sudo chmod -R 755 data/
```

#### 问题 2：数据库文件损坏

**症状**：日志中出现 `database disk image is malformed`

**解决方案**：
```bash
# 备份数据库
cp data/nofx.db data/nofx.db.backup

# 尝试修复
sqlite3 data/nofx.db "PRAGMA integrity_check;"

# 如果无法修复，重建数据库
docker compose -f docker-compose.prod.yml down
rm data/nofx.db
docker compose -f docker-compose.prod.yml up -d
```

#### 问题 3：表结构缺失

**症状**：日志中出现 `no such table: users` 或 `no such column`

**解决方案**：
```bash
# 重启服务让 GORM 自动迁移
docker compose -f docker-compose.prod.yml restart nofx

# 查看启动日志确认迁移成功
docker compose -f docker-compose.prod.yml logs nofx | grep -i "migrat"
```

#### 问题 4：注册功能被禁用

**症状**：返回 `Registration is disabled`

**解决方案**：
```bash
# 检查环境变量
docker compose -f docker-compose.prod.yml exec nofx env | grep REGISTRATION

# 修改 .env 文件
echo "REGISTRATION_ENABLED=true" >> .env

# 重启服务
docker compose -f docker-compose.prod.yml restart
```

#### 问题 5：用户数量达到上限

**症状**：返回 `Not on whitelist` 或 `capacity limit`

**解决方案**：
```bash
# 检查配置
curl http://localhost:8080/api/system-config | jq .max_users

# 方案 1：增加用户限制
# 修改 .env 文件
echo "MAX_USERS=100" >> .env
docker compose -f docker-compose.prod.yml restart

# 方案 2：清理测试用户
docker compose -f docker-compose.prod.yml exec nofx sh
sqlite3 /app/data/nofx.db "DELETE FROM users WHERE email LIKE 'test%';"
```

#### 问题 6：OTP 生成失败

**症状**：日志中出现 `OTP secret generation failed`

**解决方案**：
```bash
# 检查系统熵池
cat /proc/sys/kernel/random/entropy_avail

# 如果熵池不足，安装 haveged
sudo apt-get install haveged
sudo systemctl start haveged
```

### 6. 使用诊断脚本

运行自动诊断脚本：

```bash
chmod +x diagnose-register.sh
./diagnose-register.sh
```

### 7. 手动测试完整注册流程

```bash
# 1. 注册
REGISTER_RESPONSE=$(curl -s -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "Test123!@#"
  }')

echo "注册响应:"
echo $REGISTER_RESPONSE | jq .

# 2. 提取 user_id 和 otp_secret
USER_ID=$(echo $REGISTER_RESPONSE | jq -r .user_id)
OTP_SECRET=$(echo $REGISTER_RESPONSE | jq -r .otp_secret)

echo "User ID: $USER_ID"
echo "OTP Secret: $OTP_SECRET"

# 3. 生成 OTP 代码（需要安装 oathtool）
# sudo apt-get install oathtool
OTP_CODE=$(oathtool --totp -b $OTP_SECRET)
echo "OTP Code: $OTP_CODE"

# 4. 完成注册
curl -X POST http://localhost:8080/api/complete-registration \
  -H "Content-Type: application/json" \
  -d "{
    \"user_id\": \"$USER_ID\",
    \"otp_code\": \"$OTP_CODE\"
  }"
```

### 8. 查看详细错误信息

如果前端只显示 "Server error"，需要查看后端日志获取详细错误：

```bash
# 实时监控日志
docker compose -f docker-compose.prod.yml logs -f nofx

# 然后在浏览器中尝试注册，观察日志输出
```

### 9. 检查网络连接

```bash
# 检查前端是否能访问后端
curl http://localhost:8080/api/health

# 检查防火墙
sudo ufw status

# 检查端口占用
netstat -tlnp | grep 8080
```

### 10. 重置系统（最后手段）

如果以上方法都无效，可以重置系统：

```bash
# 停止服务
docker compose -f docker-compose.prod.yml down

# 备份数据
cp -r data data.backup

# 清理数据
rm -rf data/*

# 重新启动
docker compose -f docker-compose.prod.yml up -d

# 查看启动日志
docker compose -f docker-compose.prod.yml logs -f nofx
```

## 获取帮助

如果问题仍未解决，请提供以下信息：

1. 完整的错误日志
2. 系统配置信息
3. 数据库表结构
4. 注册 API 的完整响应

```bash
# 收集诊断信息
./diagnose-register.sh > diagnosis.txt 2>&1
```

然后将 `diagnosis.txt` 文件内容提供给技术支持。

