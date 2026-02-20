# 服务器更新指南 - 修复 RSA 密钥问题

## 问题原因

服务器上运行的代码是旧版本，不支持从文件路径加载 RSA 密钥。

## 解决方案

在服务器上执行以下命令：

### 方法 1：使用自动更新脚本（推荐）

```bash
# SSH 登录服务器后执行

cd /opt/nofx

# 下载并运行更新脚本
curl -fsSL https://raw.githubusercontent.com/LIULIBAO123/nofx/dev/update-server.sh -o update-server.sh
chmod +x update-server.sh
./update-server.sh
```

### 方法 2：手动更新

```bash
# SSH 登录服务器后执行

cd /opt/nofx

# 1. 停止服务
docker compose -f docker-compose.prod.yml down

# 2. 拉取最新代码
git pull origin dev

# 3. 检查版本（应该看到 RSA 相关的 commit）
git log --oneline -5

# 4. 生成 RSA 密钥
mkdir -p keys
openssl genrsa -out keys/private.pem 2048
openssl rsa -in keys/private.pem -pubout -out keys/public.pem
chmod 600 keys/private.pem
chmod 644 keys/public.pem

# 5. 验证密钥
openssl rsa -in keys/private.pem -check -noout

# 6. 启动服务
docker compose -f docker-compose.prod.yml up -d

# 7. 查看日志
docker compose -f docker-compose.prod.yml logs -f nofx
```

## 验证更新

### 1. 检查代码版本

```bash
cd /opt/nofx
git log --oneline -1
```

应该看到类似：
```
6bd1e030 docs: add final RSA fix guide
```

或更新的 commit。

### 2. 检查密钥文件

```bash
ls -la /opt/nofx/keys/
```

应该看到：
```
-rw------- 1 root root 1675 Feb 20 14:10 private.pem
-rw-r--r-- 1 root root  451 Feb 20 14:10 public.pem
```

### 3. 检查服务日志

```bash
docker compose -f docker-compose.prod.yml logs nofx | grep -i encryption
```

应该看到：
```
🔐 Initializing encryption service...
✅ Encryption service initialized successfully
```

而不是：
```
❌ Failed to initialize encryption service...
```

### 4. 测试健康检查

```bash
curl http://localhost:8080/api/health
```

应该返回：
```json
{"status":"ok","time":null}
```

### 5. 测试注册功能

```bash
curl -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "Test123!@#"
  }'
```

应该返回 OTP 设置信息，而不是错误。

## 预期结果

更新后，日志应该显示：

```
02-20 14:15:25 [INFO] nofx/main.go:30 ╔════════════════════════════════════════════════════════════╗
02-20 14:15:25 [INFO] nofx/main.go:31 ║           🚀 NOFX - AI-Powered Trading System              ║
02-20 14:15:25 [INFO] nofx/main.go:32 ╚════════════════════════════════════════════════════════════╝
02-20 14:15:25 [INFO] nofx/main.go:37 ✅ Configuration loaded
02-20 14:15:25 [INFO] nofx/main.go:40 🔐 Initializing encryption service...
02-20 14:15:25 [INFO] nofx/main.go:46 ✅ Encryption service initialized successfully
02-20 14:15:25 [INFO] nofx/main.go:65 📋 Initializing database (sqlite)...
...
02-20 14:15:26 [INFO] nofx/main.go:XXX ✅ System started successfully
```

## 故障排除

### 问题 1：git pull 失败

```bash
# 如果有本地修改冲突
cd /opt/nofx
git stash
git pull origin dev
git stash pop
```

### 问题 2：密钥文件权限错误

```bash
cd /opt/nofx
chmod 600 keys/private.pem
chmod 644 keys/public.pem
docker compose -f docker-compose.prod.yml restart
```

### 问题 3：容器无法访问密钥文件

```bash
# 检查 docker-compose.prod.yml 是否包含 keys 挂载
grep -A 5 "volumes:" /opt/nofx/docker-compose.prod.yml

# 应该看到：
#   volumes:
#     - ./data:/app/data
#     - ./logs:/app/logs
#     - ./keys:/app/keys

# 如果没有，需要更新 docker-compose.prod.yml
```

### 问题 4：仍然报错

```bash
# 查看完整日志
docker compose -f docker-compose.prod.yml logs --tail=100 nofx

# 检查容器内的文件
docker compose -f docker-compose.prod.yml exec nofx ls -la /app/keys/

# 检查环境变量
docker compose -f docker-compose.prod.yml exec nofx env | grep RSA
```

## 更新内容

此次更新包含：

1. ✅ 支持从文件路径加载 RSA 密钥
2. ✅ 支持从环境变量加载 RSA 密钥
3. ✅ 自动尝试默认路径（./keys/private.pem, /app/keys/private.pem）
4. ✅ 更友好的错误提示
5. ✅ docker-compose.prod.yml 挂载 keys 目录
6. ✅ 部署脚本自动生成 RSA 密钥

## 相关 Commits

- `6bd1e030` - docs: add final RSA fix guide
- `60f03d05` - fix: support loading RSA keys from file path with fallback to default locations
- `8f0fa5b9` - fix: add RSA key generation to deployment scripts and fix registration issue

## 需要帮助？

如果更新后仍有问题，请提供：

1. `git log --oneline -1` 的输出
2. `ls -la /opt/nofx/keys/` 的输出
3. `docker compose -f docker-compose.prod.yml logs --tail=50 nofx` 的输出

