# 将 Fork 仓库转换为独立仓库指南

将 Fork 仓库转换为独立仓库后，您就可以将 GHCR 镜像设置为公开，无需使用 PAT 认证。

## 转换步骤

### 方法 1：通过 GitHub Web 界面（推荐）

1. **访问仓库设置**
   - 打开您的仓库：https://github.com/LIULIBAO123/nofx
   - 点击 **Settings**（设置）

2. **进入危险区域**
   - 滚动到页面底部
   - 找到 **Danger Zone**（危险区域）部分

3. **删除 Fork 关系**
   - 点击 **Delete this repository**（删除此仓库）下方的链接
   - 或者直接访问：`https://github.com/LIULIBAO123/nofx/settings#danger-zone`
   - 找到 **"Detach fork"** 或 **"Unlink from upstream"** 选项
   - 点击并确认操作

   **注意**：如果看不到 "Detach fork" 选项，请使用下面的方法 2。

### 方法 2：通过 GitHub API（如果方法 1 不可用）

如果 Web 界面没有 "Detach fork" 选项，可以使用 GitHub API：

```bash
# 使用您的 GitHub Personal Access Token
# 需要权限：repo, delete_repo

curl -X DELETE \
  -H "Authorization: token YOUR_PAT" \
  -H "Accept: application/vnd.github.v3+json" \
  https://api.github.com/repos/LIULIBAO123/nofx
```

**⚠️ 警告**：这会删除整个仓库！请先备份代码。

### 方法 3：创建新仓库并推送代码（最安全）

这是最安全的方法，不会影响现有仓库：

1. **在 GitHub 创建新仓库**
   - 访问：https://github.com/new
   - 仓库名：`nofx`（或您喜欢的名称）
   - 设置为 **Public** 或 **Private**（根据您的需求）
   - **不要**初始化 README、.gitignore 或 license

2. **更新本地仓库的远程地址**
   ```bash
   # 查看当前远程地址
   git remote -v
   
   # 删除旧的远程地址
   git remote remove origin
   
   # 添加新的远程地址（替换为您的用户名）
   git remote add origin https://github.com/LIULIBAO123/nofx.git
   
   # 或者使用 SSH
   git remote add origin git@github.com:LIULIBAO123/nofx.git
   ```

3. **推送所有分支和标签**
   ```bash
   # 推送所有分支
   git push -u origin --all
   
   # 推送所有标签
   git push -u origin --tags
   ```

4. **删除旧的 Fork 仓库**（可选）
   - 在 GitHub 上删除旧的 Fork 仓库
   - Settings → Danger Zone → Delete this repository

## 转换后的操作

### 1. 更新 GitHub Actions 工作流

转换后，GitHub Actions 会自动使用新的仓库信息构建镜像。镜像路径会变为：
- `ghcr.io/liulibao123/nofx-backend:dev`
- `ghcr.io/liulibao123/nofx-frontend:dev`

### 2. 设置镜像为公开

1. **访问 GitHub Packages**
   - 访问：https://github.com/users/LIULIBAO123/packages
   - 或直接访问：`https://github.com/LIULIBAO123?tab=packages`

2. **找到您的包**
   - 找到 `nofx-backend` 和 `nofx-frontend` 包

3. **更改可见性**
   - 点击包名称进入详情页
   - 点击 **Package settings**（包设置）
   - 在 **Danger Zone** 部分
   - 点击 **Change visibility**（更改可见性）
   - 选择 **Public**（公开）
   - 确认操作

### 3. 更新部署配置

镜像公开后，可以更新 `docker-compose.prod.yml`，移除认证要求：

```yaml
services:
  nofx:
    image: ghcr.io/liulibao123/nofx-backend:dev
    # 不再需要认证
    ...
```

### 4. 更新部署脚本

可以简化部署脚本，移除 GHCR 登录步骤（因为镜像已公开）。

## 验证转换

转换完成后，验证以下内容：

1. **仓库信息**
   ```bash
   git remote -v
   # 应该显示新的仓库地址，而不是 fork 关系
   ```

2. **镜像可见性**
   ```bash
   # 无需登录即可拉取（如果已设置为公开）
   docker pull ghcr.io/liulibao123/nofx-backend:dev
   docker pull ghcr.io/liulibao123/nofx-frontend:dev
   ```

3. **GitHub Actions**
   - 检查 Actions 是否正常运行
   - 确认镜像成功构建并推送

## 注意事项

1. **备份代码**：转换前确保代码已备份
2. **更新文档**：更新所有引用旧仓库地址的文档
3. **通知协作者**：如果有协作者，通知他们更新远程地址
4. **Webhooks 和集成**：检查并更新所有相关的 Webhooks 和 CI/CD 集成

## 常见问题

### Q: 转换后还能同步上游仓库的更新吗？

A: 不能。转换为独立仓库后，与上游仓库的关联会被切断。如果需要同步更新，需要手动操作：
```bash
# 添加上游仓库作为远程
git remote add upstream https://github.com/NoFxAiOS/nofx.git

# 获取上游更新
git fetch upstream

# 合并到您的分支
git merge upstream/dev
```

### Q: 转换会影响现有的 Issues 和 Pull Requests 吗？

A: 不会。Issues 和 Pull Requests 会保留在您的仓库中。

### Q: 转换后 GitHub Actions 还能正常工作吗？

A: 可以。GitHub Actions 会继续工作，但需要确保 Secrets 和权限设置正确。

