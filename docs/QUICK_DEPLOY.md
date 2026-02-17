# NOFX 交易系统 - 云服务器部署

## 🚀 快速开始

### 方法一：一键部署（最简单）

在您的云服务器上执行以下命令：

```bash
curl -fsSL https://raw.githubusercontent.com/LIULIBAO123/nofx/dev/quick-deploy.sh | sudo bash
```

或者：

```bash
wget -qO- https://raw.githubusercontent.com/LIULIBAO123/nofx/dev/quick-deploy.sh | sudo bash
```

### 方法二：完整部署（推荐）

如果您需要更多自定义选项：

```bash
# 下载部署脚本
wget https://raw.githubusercontent.com/LIULIBAO123/nofx/dev/deploy.sh

# 赋予执行权限
chmod +x deploy.sh

# 运行部署脚本
sudo ./deploy.sh
```

部署脚本会引导您完成以下配置：
- ✅ 自动安装 Docker 和 Git
- ✅ 选择仓库和分支
- ✅ 配置安装目录
- ✅ 配置防火墙规则
- ✅ 配置开机自启
- ✅ 创建管理脚本

## 📋 系统要求

- **操作系统**: Ubuntu 20.04+, Debian 10+, CentOS 7+
- **CPU**: 2 核或更多
- **内存**: 4GB RAM 或更多
- **硬盘**: 20GB 可用空间
- **网络**: 开放端口 8080 (API) 和 3000 (前端)

## 🔧 手动部署

如果您想手动控制每一步：

### 1. 安装 Docker

```bash
curl -fsSL https://get.docker.com | sudo sh
sudo systemctl start docker
sudo systemctl enable docker
```

### 2. 克隆项目

```bash
cd /opt
sudo git clone https://github.com/LIULIBAO123/nofx.git
cd nofx
sudo git checkout dev
```

### 3. 启动服务

```bash
sudo docker compose up -d
```

### 4. 查看状态

```bash
sudo docker compose ps
sudo docker compose logs -f
```

## 🌐 访问系统

部署完成后，通过以下地址访问：

- **前端**: `http://your-server-ip:3000`
- **API**: `http://your-server-ip:8080`

## 📝 管理命令

使用完整部署脚本后，可以使用以下快捷命令：

```bash
nofx-start      # 启动服务
nofx-stop       # 停止服务
nofx-restart    # 重启服务
nofx-logs       # 查看日志
nofx-update     # 更新到最新版本
```

或使用 Docker Compose 命令：

```bash
cd /opt/nofx

# 查看状态
sudo docker compose ps

# 查看日志
sudo docker compose logs -f

# 停止服务
sudo docker compose down

# 启动服务
sudo docker compose up -d

# 重启服务
sudo docker compose restart
```

## 🔒 安全配置

### 配置防火墙

**Ubuntu/Debian (UFW):**
```bash
sudo ufw allow 8080/tcp
sudo ufw allow 3000/tcp
sudo ufw allow 22/tcp
sudo ufw enable
```

**CentOS/RHEL (Firewalld):**
```bash
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --permanent --add-port=3000/tcp
sudo firewall-cmd --reload
```

### 云服务器安全组

确保在云服务商控制台（阿里云、腾讯云、AWS 等）的安全组中开放以下端口：
- 8080 (API)
- 3000 (前端)
- 22 (SSH)

## 🔄 更新系统

```bash
cd /opt/nofx
sudo git pull
sudo docker compose build
sudo docker compose up -d
```

或使用快捷命令：
```bash
nofx-update
```

## 💾 备份数据

```bash
# 备份数据库
sudo cp /opt/nofx/data/nofx.db /backup/nofx.db.$(date +%Y%m%d)

# 备份整个数据目录
sudo tar -czf /backup/nofx-data-$(date +%Y%m%d).tar.gz /opt/nofx/data
```

## 🐛 故障排查

### 查看日志
```bash
cd /opt/nofx
sudo docker compose logs -f
```

### 重启服务
```bash
cd /opt/nofx
sudo docker compose restart
```

### 完全重建
```bash
cd /opt/nofx
sudo docker compose down
sudo docker compose build --no-cache
sudo docker compose up -d
```

### 检查端口占用
```bash
sudo lsof -i :8080
sudo lsof -i :3000
```

## 📚 详细文档

查看完整部署文档：[DEPLOYMENT.md](./DEPLOYMENT.md)

## 🆘 获取帮助

- GitHub Issues: https://github.com/LIULIBAO123/nofx/issues
- 文档目录: https://github.com/LIULIBAO123/nofx/tree/dev/docs

## 📄 许可证

本项目遵循原项目的许可证。

