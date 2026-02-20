# 快速开始：将 Fork 转换为独立仓库

## 为什么需要转换？

Fork 仓库的 GHCR 镜像无法设置为公开，必须使用 PAT 认证。转换为独立仓库后，可以将镜像设置为公开，部署更简单。

## 快速步骤（5 分钟）

### 步骤 1：访问仓库设置

1. 打开：https://github.com/LIULIBAO123/nofx/settings
2. 滚动到底部，找到 **Danger Zone**（危险区域）

### 步骤 2：删除 Fork 关系

1. 在 Danger Zone 中找到 **"Detach fork"** 或 **"Unlink from upstream"**
2. 点击并确认操作

**如果看不到此选项**，请使用下面的方法 3。

### 步骤 3：设置镜像为公开（转换后）

1. 访问：https://github.com/users/LIULIBAO123/packages
2. 找到 `nofx-backend` 和 `nofx-frontend` 包
3. 对每个包：
   - 点击包名称
   - 点击 **Package settings**
   - 在 **Danger Zone** 中点击 **Change visibility**
   - 选择 **Public** 并确认

### 步骤 4：验证

```bash
# 无需登录即可拉取（如果已设置为公开）
docker pull ghcr.io/liulibao123/nofx-backend:dev
docker pull ghcr.io/liulibao123/nofx-frontend:dev
```

## 如果方法 1 不可用

### 方法 2：创建新仓库（推荐，最安全）

1. **创建新仓库**
   - 访问：https://github.com/new
   - 仓库名：`nofx`
   - 不要初始化任何文件

2. **更新本地仓库**
   ```bash
   git remote set-url origin https://github.com/LIULIBAO123/nofx.git
   git push -u origin --all
   git push -u origin --tags
   ```

3. **删除旧的 Fork 仓库**（可选）

## 详细文档

需要更多信息？请查看：
- 📖 [完整转换指南](./FORK_TO_INDEPENDENT.md)
- 📖 [GHCR 认证指南](./GHCR_AUTH.md)

## 转换后的优势

✅ 镜像可以设置为公开  
✅ 无需 PAT 认证即可拉取镜像  
✅ 部署脚本更简单  
✅ 更好的用户体验  

