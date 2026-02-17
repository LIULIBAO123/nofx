# NOFX 交易系统 - 快速安装指南

## 🚀 一键安装（推荐）

### 使用预构建镜像（最快）

在您的服务器上运行以下命令即可完成安装：

```bash
curl -fsSL https://raw.githubusercontent.com/LIULIBAO123/nofx/dev/install.sh | bash
```

或指定安装目录：

```bash
curl -fsSL https://raw.githubusercontent.com/LIULIBAO123/nofx/dev/install.sh | bash -s -- /opt/nofx
```

**安装时间**: 约 2-5 分钟（取决于网络速度）

**优势**:
- ✅ 使用预构建的 Docker 镜像，无需本地编译
- ✅ 自动生成加密密钥
- ✅ 自动配置环境变量
- ✅ 自动创建必要目录
- ✅ 一键更新到最新版本

---

## 📋 系统要求

- **操作系统**: Linux (Ubuntu 20.04+, Debian 10+, CentOS 7+)
- **Docker**: 20.10+ 
- **Docker Compose**: 2.0+
- **CPU**: 2 核或更多
- **内存**: 4GB RAM 或更多
- **硬盘**: 20GB 可用空间

---

## 🔧 安装前准备

### 1. 安装 Docker

如果您还没有安装 Docker，可以使用官方脚本快速安装：

```bash
curl -fsSL https://get.docker.com | sudo sh
sudo systemctl start docker
sudo systemctl enable docker
```

### 2. 验证 Docker 安装

```bash
docker --version
docker compose version
```

---

## 📦 安装方式对比

| 方式 | 安装时间 | 优点 | 缺点 |
|------|----------|------|------|
| **一键安装（预构建镜像）** | 2-5 分钟 | 最快，自动化程度高 | 需要网络下载镜像 |
| **源码构建** | 10-20 分钟 | 可自定义修改 | 耗时长，需要编译 |

---

## 🌐 访问系统

安装完成后，通过以下地址访问：

- **前端界面**: `http://your-server-ip:3000`
- **API 接口**: `http://your-server-ip:8080`
- **本地访问**: `http://127.0.0.1:3000`

---

## 🔄 更新系统

### 方法一：一键更新（推荐）

```bash
curl -fsSL https://raw.githubusercontent.com/LIULIBAO123/nofx/dev/install.sh | bash
```

### 方法二：手动更新

```bash
cd ~/nofx  # 或您的安装目录
docker compose pull
docker compose up -d
```

---

## 📝 管理命令

```bash
# 进入安装目录
cd ~/nofx  # 或 /opt/nofx

# 查看服务状态
docker compose ps

# 查看日志
docker compose logs -f

# 查看特定服务日志
docker compose logs -f nofx-trading
docker compose logs -f nofx-frontend

# 重启服务
docker compose restart

# 停止服务
docker compose down

# 启动服务
docker compose up -d

# 更新到最新版本
docker compose pull && docker compose up -d
```

---

## 🔒 安全配置

### 1. 配置防火墙

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

### 2. 云服务器安全组

确保在云服务商控制台开放以下端口：
- 8080 (API)
- 3000 (前端)
- 22 (SSH)

### 3. 修改默认密钥

编辑 `.env` 文件，修改以下密钥：

```bash
cd ~/nofx
nano .env
```

建议修改：
- `JWT_SECRET`
- `DATA_ENCRYPTION_KEY`
- `RSA_PRIVATE_KEY`

---

## 💾 数据备份

### 备份数据库

```bash
# 备份数据库
cp ~/nofx/data/nofx.db ~/backup/nofx.db.$(date +%Y%m%d)

# 备份整个数据目录
tar -czf ~/backup/nofx-data-$(date +%Y%m%d).tar.gz ~/nofx/data
```

### 定时备份

添加到 crontab：

```bash
crontab -e
```

添加以下行（每天凌晨 2 点备份）：

```cron
0 2 * * * tar -czf ~/backup/nofx-data-$(date +\%Y\%m\%d).tar.gz ~/nofx/data
```

---

## 🐛 故障排查

### 问题 1: 无法访问服务

**检查服务状态**:
```bash
cd ~/nofx
docker compose ps
```

**查看日志**:
```bash
docker compose logs -f
```

**检查端口占用**:
```bash
sudo lsof -i :8080
sudo lsof -i :3000
```

### 问题 2: 镜像拉取失败

**使用国内镜像源**:

编辑 `/etc/docker/daemon.json`:

```json
{
  "registry-mirrors": [
    "https://docker.mirrors.ustc.edu.cn",
    "https://hub-mirror.c.163.com"
  ]
}
```

重启 Docker:
```bash
sudo systemctl restart docker
```

### 问题 3: 内存不足

**添加 swap 空间**:

```bash
sudo fallocate -l 4G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
```

---

## 🔗 相关链接

- **GitHub 仓库**: https://github.com/LIULIBAO123/nofx
- **完整部署文档**: [DEPLOYMENT.md](./DEPLOYMENT.md)
- **问题反馈**: https://github.com/LIULIBAO123/nofx/issues

---

## ⚠️ 风险提示

**AI 交易存在重大风险，可能导致资金损失。请务必：**

1. 仅使用您能承受损失的资金
2. 在实盘交易前充分测试
3. 设置合理的风险控制参数
4. 定期监控交易状态
5. 了解并接受所有风险

---

## 📄 许可证

本项目遵循原项目的许可证。

