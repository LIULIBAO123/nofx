# NOFX 交易系统 - 云服务器部署指南

## 📋 目录

- [系统要求](#系统要求)
- [快速部署](#快速部署)
- [手动部署](#手动部署)
- [配置说明](#配置说明)
- [常见问题](#常见问题)
- [维护管理](#维护管理)

---

## 系统要求

### 最低配置
- **CPU**: 2 核
- **内存**: 4GB RAM
- **硬盘**: 20GB 可用空间
- **操作系统**: Ubuntu 20.04+, Debian 10+, CentOS 7+

### 推荐配置
- **CPU**: 4 核或更多
- **内存**: 8GB RAM 或更多
- **硬盘**: 50GB SSD
- **操作系统**: Ubuntu 22.04 LTS

### 网络要求
- 开放端口: 8080 (API), 3000 (前端)
- 稳定的互联网连接
- 能够访问 GitHub 和 Docker Hub

---

## 快速部署

### 方法一：使用一键部署脚本（推荐）

1. **登录云服务器**
   ```bash
   ssh root@your-server-ip
   ```

2. **下载部署脚本**
   ```bash
   wget https://raw.githubusercontent.com/LIULIBAO123/nofx/dev/deploy.sh
   # 或使用 curl
   curl -O https://raw.githubusercontent.com/LIULIBAO123/nofx/dev/deploy.sh
   ```

3. **赋予执行权限**
   ```bash
   chmod +x deploy.sh
   ```

4. **运行部署脚本**
   ```bash
   ./deploy.sh
   ```

5. **按照提示完成配置**
   - 选择仓库地址
   - 设置安装目录
   - 选择分支
   - 配置防火墙
   - 配置开机自启

6. **访问系统**
   - 前端: `http://your-server-ip:3000`
   - API: `http://your-server-ip:8080`

---

## 手动部署

如果您想更精细地控制部署过程，可以按照以下步骤手动部署：

### 1. 安装 Docker

**Ubuntu/Debian:**
```bash
# 更新包索引
sudo apt-get update

# 安装依赖
sudo apt-get install -y ca-certificates curl gnupg lsb-release

# 添加 Docker GPG 密钥
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg

# 设置 Docker 仓库
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# 安装 Docker
sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# 启动 Docker
sudo systemctl start docker
sudo systemctl enable docker
```

**CentOS/RHEL:**
```bash
# 安装依赖
sudo yum install -y yum-utils

# 添加 Docker 仓库
sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo

# 安装 Docker
sudo yum install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# 启动 Docker
sudo systemctl start docker
sudo systemctl enable docker
```

### 2. 克隆项目

```bash
# 安装 Git
sudo apt-get install -y git  # Ubuntu/Debian
# 或
sudo yum install -y git       # CentOS/RHEL

# 克隆项目
cd /opt
sudo git clone https://github.com/LIULIBAO123/nofx.git
cd nofx

# 切换到 dev 分支
sudo git checkout dev
```

### 3. 配置环境变量

```bash
# 创建 .env 文件
sudo nano .env
```

添加以下内容：
```env
# 数据库配置
DB_PATH=./data/nofx.db

# 服务端口
API_PORT=8080
FRONTEND_PORT=3000

# 日志级别
LOG_LEVEL=info

# JWT 密钥（请修改为随机字符串）
JWT_SECRET=your-secret-key-change-this

# 时区
TZ=Asia/Shanghai
```

### 4. 配置防火墙

**Ubuntu (UFW):**
```bash
sudo ufw allow 8080/tcp
sudo ufw allow 3000/tcp
sudo ufw allow 22/tcp
sudo ufw enable
```

**CentOS (Firewalld):**
```bash
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --permanent --add-port=3000/tcp
sudo firewall-cmd --reload
```

### 5. 构建并启动服务

```bash
cd /opt/nofx

# 构建镜像
sudo docker compose build

# 启动服务
sudo docker compose up -d

# 查看服务状态
sudo docker compose ps

# 查看日志
sudo docker compose logs -f
```

---

## 配置说明

### 环境变量配置

编辑 `.env` 文件来配置系统参数：

```env
# 数据库配置
DB_PATH=./data/nofx.db              # 数据库文件路径

# 服务端口
API_PORT=8080                        # API 服务端口
FRONTEND_PORT=3000                   # 前端服务端口

# 日志级别
LOG_LEVEL=info                       # debug, info, warn, error

# JWT 密钥
JWT_SECRET=your-secret-key           # 请修改为随机字符串

# HTTPS 配置（可选）
ENABLE_HTTPS=false                   # 是否启用 HTTPS
SSL_CERT_PATH=/path/to/cert.pem     # SSL 证书路径
SSL_KEY_PATH=/path/to/key.pem       # SSL 密钥路径

# 时区
TZ=Asia/Shanghai                     # 时区设置
```

### Docker Compose 配置

如需自定义 Docker 配置，编辑 `docker-compose.yml`：

```yaml
version: '3.8'

services:
  nofx:
    build:
      context: .
      dockerfile: Dockerfile.backend
    ports:
      - "${API_PORT:-8080}:8080"
    volumes:
      - ./data:/app/data
    environment:
      - TZ=${TZ:-Asia/Shanghai}
    restart: unless-stopped

  nofx-frontend:
    build:
      context: .
      dockerfile: Dockerfile.frontend
    ports:
      - "${FRONTEND_PORT:-3000}:80"
    depends_on:
      - nofx
    restart: unless-stopped
```

---

## 常见问题

### 1. 端口被占用

**问题**: 启动时提示端口 8080 或 3000 已被占用

**解决方案**:
```bash
# 查看占用端口的进程
sudo lsof -i :8080
sudo lsof -i :3000

# 停止占用端口的进程
sudo kill -9 <PID>

# 或修改 .env 文件使用其他端口
API_PORT=8081
FRONTEND_PORT=3001
```

### 2. Docker 构建失败

**问题**: 构建镜像时出现错误

**解决方案**:
```bash
# 清理 Docker 缓存
sudo docker system prune -a

# 重新构建
cd /opt/nofx
sudo docker compose build --no-cache
```

### 3. 无法访问服务

**问题**: 浏览器无法访问前端或 API

**解决方案**:
```bash
# 检查服务状态
sudo docker compose ps

# 检查防火墙
sudo ufw status                    # Ubuntu
sudo firewall-cmd --list-all       # CentOS

# 检查云服务器安全组
# 确保在云服务商控制台开放了 8080 和 3000 端口

# 查看日志
sudo docker compose logs -f
```

### 4. 内存不足

**问题**: 服务运行缓慢或崩溃

**解决方案**:
```bash
# 查看内存使用
free -h

# 添加 swap 空间
sudo fallocate -l 4G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile

# 永久启用 swap
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
```

### 5. 数据库锁定

**问题**: 提示数据库被锁定

**解决方案**:
```bash
# 停止所有服务
sudo docker compose down

# 删除锁文件
sudo rm -f /opt/nofx/data/*.db-shm
sudo rm -f /opt/nofx/data/*.db-wal

# 重启服务
sudo docker compose up -d
```

---

## 维护管理

### 常用命令

使用一键部署脚本后，可以使用以下快捷命令：

```bash
# 启动服务
nofx-start

# 停止服务
nofx-stop

# 重启服务
nofx-restart

# 查看日志
nofx-logs

# 更新到最新版本
nofx-update
```

### 手动管理命令

```bash
# 进入项目目录
cd /opt/nofx

# 查看服务状态
sudo docker compose ps

# 查看日志
sudo docker compose logs -f

# 查看特定服务日志
sudo docker compose logs -f nofx
sudo docker compose logs -f nofx-frontend

# 停止服务
sudo docker compose down

# 启动服务
sudo docker compose up -d

# 重启服务
sudo docker compose restart

# 重启特定服务
sudo docker compose restart nofx
sudo docker compose restart nofx-frontend

# 查看资源使用
sudo docker stats
```

### 更新系统

```bash
cd /opt/nofx

# 拉取最新代码
sudo git pull

# 重新构建镜像
sudo docker compose build

# 重启服务
sudo docker compose up -d
```

### 备份数据

```bash
# 备份数据库
sudo cp /opt/nofx/data/nofx.db /backup/nofx.db.$(date +%Y%m%d)

# 备份整个数据目录
sudo tar -czf /backup/nofx-data-$(date +%Y%m%d).tar.gz /opt/nofx/data

# 定期备份（添加到 crontab）
sudo crontab -e
# 添加以下行（每天凌晨 2 点备份）
0 2 * * * tar -czf /backup/nofx-data-$(date +\%Y\%m\%d).tar.gz /opt/nofx/data
```

### 恢复数据

```bash
# 停止服务
sudo docker compose down

# 恢复数据库
sudo cp /backup/nofx.db.20260217 /opt/nofx/data/nofx.db

# 或恢复整个数据目录
sudo tar -xzf /backup/nofx-data-20260217.tar.gz -C /

# 启动服务
sudo docker compose up -d
```

### 监控服务

```bash
# 查看 CPU 和内存使用
sudo docker stats

# 查看磁盘使用
df -h

# 查看 Docker 磁盘使用
sudo docker system df

# 清理未使用的 Docker 资源
sudo docker system prune -a
```

### 配置 HTTPS（可选）

使用 Let's Encrypt 免费 SSL 证书：

```bash
# 安装 Certbot
sudo apt-get install -y certbot  # Ubuntu/Debian
# 或
sudo yum install -y certbot      # CentOS/RHEL

# 获取证书
sudo certbot certonly --standalone -d your-domain.com

# 证书路径
# /etc/letsencrypt/live/your-domain.com/fullchain.pem
# /etc/letsencrypt/live/your-domain.com/privkey.pem

# 配置 Nginx 反向代理（推荐）
sudo apt-get install -y nginx

# 创建 Nginx 配置
sudo nano /etc/nginx/sites-available/nofx

# 添加以下内容
server {
    listen 80;
    server_name your-domain.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name your-domain.com;

    ssl_certificate /etc/letsencrypt/live/your-domain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/your-domain.com/privkey.pem;

    # 前端
    location / {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # API
    location /api {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}

# 启用配置
sudo ln -s /etc/nginx/sites-available/nofx /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl restart nginx

# 自动续期证书
sudo certbot renew --dry-run
```

---

## 性能优化

### 1. 调整 Docker 资源限制

编辑 `docker-compose.yml`：

```yaml
services:
  nofx:
    # ... 其他配置 ...
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 1G
```

### 2. 启用日志轮转

```bash
# 编辑 Docker daemon 配置
sudo nano /etc/docker/daemon.json

# 添加以下内容
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  }
}

# 重启 Docker
sudo systemctl restart docker
```

### 3. 优化数据库

```bash
# 定期优化数据库（添加到 crontab）
0 3 * * 0 docker exec nofx-nofx-1 sqlite3 /app/data/nofx.db "VACUUM;"
```

---

## 安全建议

1. **修改默认端口**: 在 `.env` 文件中使用非标准端口
2. **配置防火墙**: 只开放必要的端口
3. **使用 HTTPS**: 配置 SSL 证书加密通信
4. **定期更新**: 保持系统和 Docker 镜像最新
5. **备份数据**: 定期备份数据库和配置文件
6. **限制访问**: 使用 IP 白名单或 VPN
7. **监控日志**: 定期检查系统日志

---

## 技术支持

如有问题，请：
1. 查看日志: `sudo docker compose logs -f`
2. 检查 GitHub Issues: https://github.com/LIULIBAO123/nofx/issues
3. 查看文档: https://github.com/LIULIBAO123/nofx/tree/dev/docs

---

## 许可证

本项目遵循原项目的许可证。

