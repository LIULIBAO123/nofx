# 如何获取 GitHub Personal Access Token (PAT)

## 快速步骤（3 分钟）

### 步骤 1：访问 GitHub Token 设置页面

**方法 1：直接链接**
- 访问：https://github.com/settings/tokens

**方法 2：通过 GitHub 界面**
1. 登录 GitHub
2. 点击右上角头像
3. 点击 **Settings**（设置）
4. 左侧菜单找到 **Developer settings**（开发者设置）
5. 点击 **Personal access tokens**（个人访问令牌）
6. 点击 **Tokens (classic)**（经典令牌）

### 步骤 2：生成新 Token

1. 点击 **"Generate new token"**（生成新令牌）
2. 选择 **"Generate new token (classic)"**（生成经典令牌）

### 步骤 3：配置 Token

1. **Note（备注）**：输入一个描述性名称
   - 例如：`NOFX Docker Deploy` 或 `GHCR Access`

2. **Expiration（过期时间）**：选择过期时间
   - 建议：`90 days`（90天）或 `No expiration`（永不过期）
   - ⚠️ 注意：如果选择永不过期，请妥善保管 Token

3. **Select scopes（选择权限）**：勾选以下权限
   - ✅ **`read:packages`** - 读取包（必需，用于拉取 GHCR 镜像）
   - ✅ **`write:packages`** - 写入包（可选，如果需要推送镜像）

### 步骤 4：生成并复制 Token

1. 滚动到页面底部
2. 点击 **"Generate token"**（生成令牌）
3. **重要**：Token 只显示一次，请立即复制保存！
   - Token 格式类似：`ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`
   - 如果关闭页面，将无法再次查看，需要重新生成

4. **保存 Token**：
   - 复制到安全的地方（密码管理器、文本文件等）
   - ⚠️ 不要将 Token 提交到代码仓库！

## 使用 Token

### 在部署脚本中使用

```bash
# 设置环境变量
export GITHUB_USERNAME="LIULIBAO123"
export GITHUB_PAT="ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

# 运行部署脚本
curl -fsSL https://raw.githubusercontent.com/LIULIBAO123/nofx/dev/quick-deploy.sh | bash
```

### 手动登录 GHCR

```bash
# 使用 Token 登录
echo "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" | docker login ghcr.io -u LIULIBAO123 --password-stdin
```

## 完整操作截图说明

### 1. 访问设置页面

```
GitHub 首页
  ↓
点击右上角头像
  ↓
Settings（设置）
  ↓
左侧菜单：Developer settings（开发者设置）
  ↓
Personal access tokens（个人访问令牌）
  ↓
Tokens (classic)（经典令牌）
```

### 2. 生成 Token 页面

在生成页面，您会看到：

```
Note: [输入描述，如：NOFX Deploy]
      ↓
Expiration: [选择过期时间]
      ↓
Select scopes:
  ☐ repo
  ☐ workflow
  ☑ read:packages  ← 勾选这个
  ☐ write:packages ← 可选
      ↓
[Generate token] 按钮
```

### 3. 复制 Token

生成后，页面会显示：

```
Your new personal access token
ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

⚠️ Make sure to copy your personal access token now. 
   You won't be able to see it again!
```

**立即复制这个 Token！**

## 常见问题

### Q: Token 在哪里查看？

**A**: Token 生成后只显示一次。如果丢失，需要：
1. 删除旧 Token（在 Token 列表页面）
2. 重新生成新 Token

### Q: 如何查看已生成的 Token？

**A**: GitHub 不保存 Token 明文，无法再次查看。只能看到：
- Token 名称（Note）
- 创建时间
- 过期时间
- 权限范围

### Q: Token 过期了怎么办？

**A**: 
1. 生成新 Token
2. 更新环境变量或部署脚本中的 Token
3. 重新登录 GHCR

### Q: 如何撤销 Token？

**A**: 
1. 访问：https://github.com/settings/tokens
2. 找到对应的 Token
3. 点击右侧的 **Revoke**（撤销）按钮

### Q: Token 权限不够怎么办？

**A**: 
- 如果无法拉取镜像，检查是否勾选了 `read:packages`
- 如果无法推送镜像，检查是否勾选了 `write:packages`
- 重新生成 Token 并勾选正确权限

### Q: 可以给 Token 设置更细粒度的权限吗？

**A**: 
- **Classic Token**：使用 scopes（权限范围）
- **Fine-grained Token**：更细粒度的权限控制（GitHub 新功能）
  - 可以限制到特定仓库
  - 可以设置更精确的权限

## 安全建议

1. ✅ **不要将 Token 提交到代码仓库**
2. ✅ **使用环境变量存储 Token**
3. ✅ **定期轮换 Token**（建议 90 天）
4. ✅ **使用最小权限原则**（只授予必要权限）
5. ✅ **使用密码管理器保存 Token**
6. ✅ **不要在公共场合分享 Token**

## 验证 Token 是否有效

```bash
# 测试登录
echo "YOUR_TOKEN" | docker login ghcr.io -u YOUR_USERNAME --password-stdin

# 如果成功，会显示：
# Login Succeeded

# 测试拉取镜像
docker pull ghcr.io/liulibao123/nofx-backend:dev
```

## 相关链接

- 📖 [GitHub 官方文档：创建个人访问令牌](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token)
- 📖 [一键部署指南](./ONE_CLICK_DEPLOY_PAT.md)
- 📖 [GHCR 认证指南](./GHCR_AUTH.md)

