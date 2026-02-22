# 🐳 NOFX Docker 本地部署指南

## 📋 前置要求

- Docker 20.10+
- Docker Compose 2.0+
- 至少 4GB 可用内存
- 至少 10GB 可用磁盘空间

## 🚀 快速开始

### 1. 环境配置

确保 `.env` 文件已配置：

```bash
# 复制示例配置（如果还没有）
cp .env.example .env

# 编辑配置
nano .env
```

关键配置项：
```env
# 端口配置
NOFX_BACKEND_PORT=8080
NOFX_FRONTEND_PORT=3000

# 时区
TZ=Asia/Shanghai

# AI 配置
AI_MAX_TOKENS=8000

# 数据库路径
DATABASE_PATH=./data/nofx.db
```

### 2. 构建和启动

```bash
# 构建镜像
docker-compose build

# 启动服务
docker-compose up -d

# 查看日志
docker-compose logs -f
```

### 3. 验证部署

```bash
# 检查服务状态
docker-compose ps

# 检查健康状态
curl http://localhost:8080/api/health
curl http://localhost:3000/health

# 访问前端
# 浏览器打开: http://localhost:3000
```

## 🔄 更新部署

### 方法 1: 完整重建（推荐用于大更新）

```bash
# 停止服务
docker-compose down

# 拉取最新代码
git pull

# 重新构建镜像（不使用缓存）
docker-compose build --no-cache

# 启动服务
docker-compose up -d

# 查看日志
docker-compose logs -f
```

### 方法 2: 快速更新（用于小更新）

```bash
# 拉取最新代码
git pull

# 重新构建并重启
docker-compose up -d --build

# 查看日志
docker-compose logs -f nofx-frontend
```

### 方法 3: 仅更新前端

```bash
# 停止前端服务
docker-compose stop nofx-frontend

# 重新构建前端
docker-compose build nofx-frontend

# 启动前端
docker-compose up -d nofx-frontend

# 查看日志
docker-compose logs -f nofx-frontend
```

### 方法 4: 仅更新后端

```bash
# 停止后端服务
docker-compose stop nofx

# 重新构建后端
docker-compose build nofx

# 启动后端
docker-compose up -d nofx

# 查看日志
docker-compose logs -f nofx
```

## 📦 Meridian 设计系统更新

由于前端已更新到 Meridian 设计系统，需要重新构建前端镜像：

```bash
# 1. 停止前端服务
docker-compose stop nofx-frontend

# 2. 删除旧镜像（可选，释放空间）
docker rmi nofx-nofx-frontend

# 3. 重新构建前端（不使用缓存）
docker-compose build --no-cache nofx-frontend

# 4. 启动前端
docker-compose up -d nofx-frontend

# 5. 验证
curl http://localhost:3000/health
```

## 🛠️ 常用命令

### 服务管理

```bash
# 启动所有服务
docker-compose up -d

# 停止所有服务
docker-compose down

# 重启所有服务
docker-compose restart

# 重启单个服务
docker-compose restart nofx-frontend
docker-compose restart nofx

# 查看服务状态
docker-compose ps

# 查看资源使用
docker stats
```

### 日志管理

```bash
# 查看所有日志
docker-compose logs

# 实时查看日志
docker-compose logs -f

# 查看特定服务日志
docker-compose logs nofx-frontend
docker-compose logs nofx

# 查看最近 100 行日志
docker-compose logs --tail=100

# 查看带时间戳的日志
docker-compose logs -t
```

### 清理和维护

```bash
# 停止并删除容器
docker-compose down

# 停止并删除容器、网络、卷
docker-compose down -v

# 清理未使用的镜像
docker image prune -a

# 清理所有未使用的资源
docker system prune -a

# 查看磁盘使用
docker system df
```

## 🔍 故障排查

### 前端无法访问

```bash
# 检查容器状态
docker-compose ps nofx-frontend

# 查看日志
docker-compose logs nofx-frontend

# 检查端口占用
netstat -ano | findstr :3000  # Windows
lsof -i :3000                  # Linux/Mac

# 重启前端
docker-compose restart nofx-frontend
```

### 后端无法访问

```bash
# 检查容器状态
docker-compose ps nofx

# 查看日志
docker-compose logs nofx

# 检查健康状态
docker inspect nofx-trading | grep -A 10 Health

# 重启后端
docker-compose restart nofx
```

### API 请求失败

```bash
# 检查网络连接
docker network inspect nofx_nofx-network

# 测试后端连接
docker exec nofx-frontend wget -O- http://nofx:8080/api/health

# 检查 nginx 配置
docker exec nofx-frontend cat /etc/nginx/conf.d/default.conf
```

### 构建失败

```bash
# 清理构建缓存
docker builder prune

# 使用详细输出重新构建
docker-compose build --no-cache --progress=plain

# 检查磁盘空间
docker system df
```

## 📊 性能优化

### 1. 调整资源限制

编辑 `docker-compose.yml`：

```yaml
services:
  nofx:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 1G
```

### 2. 启用 Docker BuildKit

```bash
# 设置环境变量
export DOCKER_BUILDKIT=1
export COMPOSE_DOCKER_CLI_BUILD=1

# 构建
docker-compose build
```

### 3. 使用多阶段构建缓存

```bash
# 构建时保留中间层
docker-compose build --build-arg BUILDKIT_INLINE_CACHE=1
```

## 🔐 安全建议

### 1. 使用非 root 用户

在 Dockerfile 中添加：

```dockerfile
RUN addgroup -g 1001 -S nofx && \
    adduser -u 1001 -S nofx -G nofx
USER nofx
```

### 2. 限制容器权限

```yaml
services:
  nofx:
    security_opt:
      - no-new-privileges:true
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE
```

### 3. 使用 secrets 管理敏感信息

```yaml
secrets:
  db_password:
    file: ./secrets/db_password.txt

services:
  nofx:
    secrets:
      - db_password
```

## 📈 监控和日志

### 1. 集成 Prometheus

```yaml
services:
  prometheus:
    image: prom/prometheus
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
```

### 2. 集成 Grafana

```yaml
services:
  grafana:
    image: grafana/grafana
    ports:
      - "3001:3000"
    depends_on:
      - prometheus
```

### 3. 日志聚合

```yaml
services:
  nofx:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

## 🌐 生产环境部署

### 使用生产配置

```bash
# 使用生产配置文件
docker-compose -f docker-compose.prod.yml up -d

# 或使用稳定版本
docker-compose -f docker-compose.stable.yml up -d
```

### 配置 HTTPS

1. 安装 Certbot
2. 获取 SSL 证书
3. 更新 nginx 配置
4. 重启服务

```bash
# 示例 nginx HTTPS 配置
server {
    listen 443 ssl http2;
    ssl_certificate /etc/ssl/certs/cert.pem;
    ssl_certificate_key /etc/ssl/private/key.pem;
    # ... 其他配置
}
```

## 📝 备份和恢复

### 备份数据

```bash
# 备份数据库
docker exec nofx-trading tar czf - /app/data | gzip > backup-$(date +%Y%m%d).tar.gz

# 备份配置
tar czf config-backup-$(date +%Y%m%d).tar.gz .env keys/
```

### 恢复数据

```bash
# 停止服务
docker-compose down

# 恢复数据
tar xzf backup-20260221.tar.gz -C ./data/

# 启动服务
docker-compose up -d
```

## 🆘 获取帮助

- 查看日志: `docker-compose logs -f`
- 检查状态: `docker-compose ps`
- 进入容器: `docker exec -it nofx-trading sh`
- 查看文档: `README.md`

---

**更新日期**: 2026-02-21  
**版本**: Meridian v1.0  
**状态**: ✅ 生产就绪



