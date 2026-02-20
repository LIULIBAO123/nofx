# RSA 密钥问题紧急修复指南

## 问题
服务一直重启，错误：`Failed to initialize encryption service: failed to load RSA private key: invalid PEM format`

## 立即修复（在服务器上执行）

### 方法 1：使用紧急修复脚本（推荐）

```bash
# 1. 下载最新代码
cd /opt/nofx
git pull origin dev

# 2. 运行紧急修复脚本
chmod +x emergency-fix-rsa.sh
./emergency-fix-rsa.sh
```

### 方法 2：手动修复

```bash
# 1. 停止服务
cd /opt/nofx
docker compose -f docker-compose.prod.yml down

# 2. 创建 keys 目录
mkdir -p keys

# 3. 生成 RSA 密钥
openssl genrsa -out keys/private.pem 2048
openssl rsa -in keys/private.pem -pubout -out keys/public.pem

# 4. 设置权限
chmod 600 keys/private.pem
chmod 644 keys/public.pem

# 5. 验证密钥
openssl rsa -in keys/private.pem -check -noout

# 6. 更新 docker-compose.prod.yml
# 确保包含以下内容：
#   volumes:
#     - ./keys:/app/keys
#   environment:
#     - RSA_PRIVATE_KEY_PATH=/app/keys/private.pem
#     - RSA_PUBLIC_KEY_PATH=/app/keys/public.pem

# 7. 启动服务
docker compose -f docker-compose.prod.yml up -d

# 8. 查看日志确认
docker compose -f docker-compose.prod.yml logs -f nofx
```

## 验证修复

```bash
# 1. 检查服务状态（应该是 Up 而不是 Restarting）
docker compose -f docker-compose.prod.yml ps

# 2. 查看日志（不应该有 RSA 错误）
docker compose -f docker-compose.prod.yml logs --tail=50 nofx

# 3. 测试健康检查
curl http://localhost:8080/api/health

# 4. 测试注册 API
curl -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "Test123!@#"
  }'
```

## 预期结果

修复后，你应该看到：

1. **服务状态**：`Up` 而不是 `Restarting`
2. **日志**：没有 RSA 错误
3. **健康检查**：返回 200 OK
4. **注册 API**：返回 OTP 设置信息

## 如果仍有问题

### 检查密钥文件

```bash
# 检查文件是否存在
ls -la /opt/nofx/keys/

# 应该看到：
# -rw------- 1 root root 1675 Feb 20 13:40 private.pem
# -rw-r--r-- 1 root root  451 Feb 20 13:40 public.pem

# 验证私钥格式
openssl rsa -in /opt/nofx/keys/private.pem -text -noout
```

### 检查 docker-compose.prod.yml

```bash
# 确认 keys 目录已挂载
grep -A 5 "volumes:" /opt/nofx/docker-compose.prod.yml

# 应该包含：
#   - ./keys:/app/keys
```

### 检查容器内的文件

```bash
# 进入容器
docker compose -f docker-compose.prod.yml exec nofx sh

# 检查密钥文件
ls -la /app/keys/
cat /app/keys/private.pem

# 退出容器
exit
```

## 后续预防

更新后的部署脚本已自动包含 RSA 密钥生成功能：

- `deploy.sh` - 完整部署脚本（已更新）
- `quick-deploy.sh` - 快速部署脚本（已更新）
- `fix-rsa-keys.sh` - 独立修复脚本
- `emergency-fix-rsa.sh` - 紧急修复脚本

所有新部署都会自动生成 RSA 密钥，不会再出现此问题。

