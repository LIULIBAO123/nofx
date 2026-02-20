# GitHub Container Registry (GHCR) 认证指南

由于您的仓库是 Fork 仓库，GitHub 不允许直接更改 GHCR 包的可见性。因此，镜像默认是私有的，需要使用 Personal Access Token (PAT) 来拉取。

## 创建 Personal Access Token

1. 访问 GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)
   - 链接：https://github.com/settings/tokens

2. 点击 "Generate new token (classic)"

3. 设置 Token 权限：
   - ✅ `read:packages` - 读取包
   - ✅ `write:packages` - 写入包（如果需要推送）

4. 生成并复制 Token（**只显示一次，请妥善保存**）

## 使用 PAT 登录 GHCR

### 方法 1：交互式登录

```bash
docker login ghcr.io -u YOUR_GITHUB_USERNAME
# 当提示输入密码时，输入您的 PAT
```

### 方法 2：使用命令行（推荐）

```bash
echo YOUR_PAT | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin
```

### 方法 3：使用环境变量

```bash
export GITHUB_TOKEN=YOUR_PAT
echo $GITHUB_TOKEN | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin
```

## 在部署脚本中使用

部署脚本 (`deploy.sh` 和 `quick-deploy.sh`) 已经支持自动登录 GHCR。运行脚本时，如果检测到使用 GHCR 镜像，会提示您输入 PAT。

## 验证登录

登录成功后，可以拉取镜像：

```bash
docker pull ghcr.io/liulibao123/nofx-backend:dev
docker pull ghcr.io/liulibao123/nofx-frontend:dev
```

## 安全建议

1. **不要将 PAT 提交到代码仓库**
2. **使用最小权限原则**：只授予必要的权限
3. **定期轮换 Token**：建议每 90 天更换一次
4. **使用环境变量或密钥管理工具**存储 PAT

## 替代方案

如果您希望使用公开镜像，可以考虑：

1. **将 Fork 转换为独立仓库**（推荐）
   - 📖 详细步骤请参考：[FORK_TO_INDEPENDENT.md](./FORK_TO_INDEPENDENT.md)
   - 转换后可以将 GHCR 镜像设置为公开，无需 PAT 认证
2. **推送到 Docker Hub**（公开注册表）
3. **使用其他公开容器注册表**

## 故障排除

### 问题：`unauthorized: authentication required`

**解决方案**：
- 确认 PAT 已正确设置 `read:packages` 权限
- 确认用户名和 Token 正确
- 重新登录：`docker logout ghcr.io` 然后重新登录

### 问题：`denied: permission denied`

**解决方案**：
- 确认 PAT 有访问该仓库的权限
- 如果是组织仓库，确认您有访问权限

