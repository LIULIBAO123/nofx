# 一键部署指南（使用 PAT 拉取私有镜像）

使用 Personal Access Token (PAT) 拉取 GHCR 私有镜像的一键部署方案。**镜像保持私有，无需更改可见性**。

## 快速开始

### 方法 1：使用环境变量（推荐，最安全）

```bash
# 设置环境变量
export GITHUB_USERNAME="LIULIBAO123"
export GITHUB_PAT="your_personal_access_token_here"

# 运行一键部署脚本
curl -fsSL https://raw.githubusercontent.com/LIULIBAO123/nofx/dev/quick-deploy.sh | bash
```

或者使用完整部署脚本：

```bash
curl -fsSL https://raw.githubusercontent.com/LIULIBAO123/nofx/dev/deploy.sh | bash
```

### 方法 2：交互式部署

如果不想在命令行中暴露 PAT，可以运行脚本后交互式输入：

```bash
# 下载并运行脚本
curl -fsSL https://raw.githubusercontent.com/LIULIBAO123/nofx/dev/quick-deploy.sh -o quick-deploy.sh
chmod +x quick-deploy.sh
sudo ./quick-deploy.sh
```

脚本会提示您输入：
- GitHub 用户名
- Personal Access Token

### 方法 3：本地运行（已有代码）

如果您已经在服务器上克隆了代码：

```bash
cd /opt/nofx

# 使用环境变量
export GITHUB_USERNAME="LIULIBAO123"
export GITHUB_PAT="your_pat_here"
sudo ./quick-deploy.sh

# 或交互式
sudo ./quick-deploy.sh
```

## 创建 Personal Access Token

1. **访问 GitHub 设置**
   - 链接：https://github.com/settings/tokens
   - 或：GitHub → Settings → Developer settings → Personal access tokens → Tokens (classic)

2. **生成新 Token**
   - 点击 "Generate new token (classic)"
   - 输入 Token 名称（如：`nofx-deploy`）

3. **设置权限**
   - ✅ `read:packages` - 读取包（必需）
   - ✅ `write:packages` - 写入包（可选，如果需要推送）

4. **生成并复制**
   - 点击 "Generate token"
   - **重要**：Token 只显示一次，请立即复制保存

## 部署脚本说明

### quick-deploy.sh（快速部署）

**特点**：
- ✅ 自动安装 Docker 和 Git
- ✅ 自动克隆/更新代码
- ✅ 自动创建 .env 文件
- ✅ 支持 PAT 自动登录 GHCR
- ✅ 一键启动服务

**使用场景**：快速部署到新服务器

### deploy.sh（完整部署）

**特点**：
- ✅ 包含 quick-deploy.sh 的所有功能
- ✅ 交互式配置（仓库选择、分支选择）
- ✅ 防火墙配置
- ✅ 开机自启配置
- ✅ 管理脚本创建

**使用场景**：生产环境完整部署

## 完整部署示例

### 示例 1：使用环境变量一键部署

```bash
#!/bin/bash
# 一键部署脚本示例

# 设置变量
export GITHUB_USERNAME="LIULIBAO123"
export GITHUB_PAT="ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

# 运行部署
curl -fsSL https://raw.githubusercontent.com/LIULIBAO123/nofx/dev/quick-deploy.sh | bash
```

### 示例 2：保存为脚本文件

创建 `deploy-nofx.sh`：

```bash
#!/bin/bash
set -e

# 配置
GITHUB_USERNAME="LIULIBAO123"
GITHUB_PAT="your_pat_here"  # 请替换为您的 PAT

# 导出环境变量
export GITHUB_USERNAME
export GITHUB_PAT

# 下载并运行部署脚本
curl -fsSL https://raw.githubusercontent.com/LIULIBAO123/nofx/dev/quick-deploy.sh | bash
```

使用：

```bash
chmod +x deploy-nofx.sh
sudo ./deploy-nofx.sh
```

### 示例 3：使用 secrets 管理（生产环境）

```bash
#!/bin/bash
# 从密钥管理服务读取 PAT（示例）

# 从 AWS Secrets Manager 读取（示例）
GITHUB_PAT=$(aws secretsmanager get-secret-value --secret-id nofx/github-pat --query SecretString --output text)

# 从环境变量文件读取（.env.local）
source .env.local

# 导出
export GITHUB_USERNAME="LIULIBAO123"
export GITHUB_PAT

# 部署
./quick-deploy.sh
```

## 验证部署

部署完成后，验证服务：

```bash
# 检查服务状态
cd /opt/nofx
docker compose -f docker-compose.prod.yml ps

# 查看日志
docker compose -f docker-compose.prod.yml logs -f

# 测试 API
curl http://localhost:8080/api/health

# 访问前端
# 浏览器打开: http://YOUR_SERVER_IP:3000
```

## 常见问题

### Q: 镜像可见性需要更改吗？

**A: 不需要！** 使用 PAT 方案时，镜像保持私有即可。PAT 提供了访问私有镜像的权限。

### Q: PAT 会过期吗？

**A: 取决于设置**：
- Classic Token：可以设置过期时间（建议 90 天）
- Fine-grained Token：可以设置过期时间

建议定期轮换 Token。

### Q: 如何更新镜像？

```bash
cd /opt/nofx

# 重新登录（如果 Token 已更新）
export GITHUB_PAT="new_pat"
echo "$GITHUB_PAT" | docker login ghcr.io -u LIULIBAO123 --password-stdin

# 拉取最新镜像
docker compose -f docker-compose.prod.yml pull

# 重启服务
docker compose -f docker-compose.prod.yml up -d
```

### Q: 如何查看当前登录状态？

```bash
# 查看 Docker 登录信息
cat ~/.docker/config.json | grep ghcr.io

# 测试拉取镜像
docker pull ghcr.io/liulibao123/nofx-backend:dev
```

### Q: 登录失败怎么办？

1. **检查 PAT 权限**：确保有 `read:packages` 权限
2. **检查用户名**：确保用户名正确
3. **检查 Token 是否过期**：重新生成 Token
4. **手动登录测试**：
   ```bash
   docker logout ghcr.io
   echo "YOUR_PAT" | docker login ghcr.io -u YOUR_USERNAME --password-stdin
   ```

## 安全建议

1. ✅ **不要将 PAT 提交到代码仓库**
2. ✅ **使用环境变量或密钥管理工具**
3. ✅ **设置 Token 过期时间**
4. ✅ **定期轮换 Token**
5. ✅ **使用最小权限原则**（只授予 `read:packages`）

## 相关文档

- 📖 [GHCR 认证详细指南](./GHCR_AUTH.md)
- 📖 [完整部署文档](./DEPLOYMENT.md)
- 📖 [快速部署文档](./QUICK_DEPLOY.md)

