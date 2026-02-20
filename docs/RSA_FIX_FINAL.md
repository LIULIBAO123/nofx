# RSA 密钥问题最终修复方案

## 问题已彻底解决！

已更新代码支持多种方式加载 RSA 密钥，按优先级顺序：

1. ✅ **从文件路径加载**（推荐）- 通过 `RSA_PRIVATE_KEY_PATH` 环境变量
2. ✅ **从环境变量加载** - 通过 `RSA_PRIVATE_KEY` 环境变量（PEM 内容）
3. ✅ **从默认路径加载** - 自动尝试以下路径：
   - `./keys/private.pem`
   - `/app/keys/private.pem`
   - `./private.pem`

## 立即修复服务器

### 方法 1：使用紧急修复脚本（最简单）

```bash
cd /opt/nofx

# 拉取最新代码
git pull origin dev

# 运行修复脚本
chmod +x emergency-fix-rsa.sh
./emergency-fix-rsa.sh
```

### 方法 2：手动修复

```bash
cd /opt/nofx

# 1. 停止服务
docker compose -f docker-compose.prod.yml down

# 2. 拉取最新代码
git pull origin dev

# 3. 生成密钥（如果还没有）
mkdir -p keys
openssl genrsa -out keys/private.pem 2048
openssl rsa -in keys/private.pem -pubout -out keys/public.pem
chmod 600 keys/private.pem
chmod 644 keys/public.pem

# 4. 启动服务
docker compose -f docker-compose.prod.yml up -d

# 5. 查看日志
docker compose -f docker-compose.prod.yml logs -f nofx
```

## 验证修复

```bash
# 1. 检查服务状态（应该是 Up）
docker compose -f docker-compose.prod.yml ps

# 2. 查看日志（应该看到 "Encryption service initialized successfully"）
docker compose -f docker-compose.prod.yml logs nofx | grep -i encryption

# 3. 测试健康检查
curl http://localhost:8080/api/health

# 4. 测试注册
curl -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "Test123!@#"
  }'
```

## 预期结果

修复后，日志应该显示：

```
🔐 Initializing encryption service...
✅ Encryption service initialized successfully
```

而不是：

```
❌ Failed to initialize encryption service: failed to load RSA private key...
```

## 代码更新说明

### 1. crypto/crypto.go
- ✅ 支持从文件路径加载密钥
- ✅ 支持从环境变量加载密钥
- ✅ 自动尝试默认路径
- ✅ 更友好的错误提示

### 2. docker-compose.prod.yml & docker-compose.yml
- ✅ 挂载 keys 目录到容器
- ✅ 设置环境变量 `RSA_PRIVATE_KEY_PATH`

### 3. 部署脚本
- ✅ deploy.sh - 自动生成密钥
- ✅ quick-deploy.sh - 自动生成密钥
- ✅ emergency-fix-rsa.sh - 紧急修复脚本

## 后续部署保证

所有更新已推送到 `dev` 分支，后续部署将：

1. ✅ 自动创建 keys 目录
2. ✅ 自动生成 RSA 密钥对
3. ✅ 自动挂载到容器
4. ✅ 自动配置环境变量

**不会再出现此问题！**

## 故障排除

如果仍有问题，请检查：

```bash
# 1. 确认密钥文件存在
ls -la /opt/nofx/keys/

# 2. 验证密钥格式
openssl rsa -in /opt/nofx/keys/private.pem -check -noout

# 3. 检查容器内的文件
docker compose -f docker-compose.prod.yml exec nofx ls -la /app/keys/

# 4. 查看完整日志
docker compose -f docker-compose.prod.yml logs nofx
```

如果密钥文件不存在或格式错误，重新生成：

```bash
cd /opt/nofx
rm -rf keys
mkdir -p keys
openssl genrsa -out keys/private.pem 2048
openssl rsa -in keys/private.pem -pubout -out keys/public.pem
chmod 600 keys/private.pem
chmod 644 keys/public.pem
docker compose -f docker-compose.prod.yml restart
```

