# 🐳 Docker 部署快速指南

## 📦 Meridian 设计系统更新后的部署

由于前端已更新到 Meridian 设计系统，需要重新构建前端 Docker 镜像。

## 🚀 快速更新（推荐）

### Windows 用户

双击运行 `update-docker.bat`，然后选择选项 2（仅更新前端）。

或在命令行中：
```cmd
update-docker.bat
```

### Linux/Mac 用户

```bash
chmod +x update-docker.sh
./update-docker.sh
```

然后选择选项 2（仅更新前端）。

## 📝 手动更新步骤

如果脚本无法运行，可以手动执行以下命令：

### 1. 停止前端服务
```bash
docker-compose stop nofx-frontend
```

### 2. 删除旧镜像（可选）
```bash
docker rmi nofx-nofx-frontend
```

### 3. 重新构建前端
```bash
docker-compose build --no-cache nofx-frontend
```

### 4. 启动前端
```bash
docker-compose up -d nofx-frontend
```

### 5. 查看日志
```bash
docker-compose logs -f nofx-frontend
```

### 6. 验证部署
```bash
# 检查健康状态
curl http://localhost:3000/health

# 访问前端
# 浏览器打开: http://localhost:3000
```

## 🔍 验证 Meridian 设计系统

部署完成后，访问 http://localhost:3000，你应该看到：

- ✅ 青色主题（而不是金色）
- ✅ 毛玻璃效果卡片
- ✅ 纹理叠加层
- ✅ 青色发光阴影
- ✅ 现代化的按钮样式

## 📊 更新选项说明

### 选项 1: 完整更新
- 更新前端和后端
- 适用于：大版本更新、功能更新
- 时间：约 5-10 分钟

### 选项 2: 仅更新前端（推荐）
- 仅更新前端界面
- 适用于：Meridian 设计系统更新、UI 修改
- 时间：约 2-3 分钟

### 选项 3: 仅更新后端
- 仅更新后端服务
- 适用于：API 更新、业务逻辑修改
- 时间：约 3-5 分钟

### 选项 4: 快速重启
- 重启所有服务
- 适用于：配置更改、临时问题
- 时间：约 30 秒

## 🛠️ 常见问题

### Q: 更新后前端无法访问？
```bash
# 检查容器状态
docker-compose ps

# 查看日志
docker-compose logs nofx-frontend

# 重启前端
docker-compose restart nofx-frontend
```

### Q: 看不到新的设计？
```bash
# 清除浏览器缓存
# Chrome: Ctrl+Shift+Delete
# Firefox: Ctrl+Shift+Delete

# 或使用无痕模式访问
```

### Q: 构建失败？
```bash
# 清理 Docker 缓存
docker builder prune

# 清理未使用的镜像
docker image prune -a

# 重新构建
docker-compose build --no-cache nofx-frontend
```

### Q: 端口被占用？
```bash
# Windows: 查看端口占用
netstat -ano | findstr :3000

# Linux/Mac: 查看端口占用
lsof -i :3000

# 修改端口（编辑 .env 文件）
NOFX_FRONTEND_PORT=3001
```

## 📚 详细文档

更多详细信息请查看：
- `DOCKER_DEPLOYMENT.md` - 完整部署文档
- `docker-compose.yml` - Docker 配置
- `nginx/nginx.conf` - Nginx 配置

## 🆘 获取帮助

如果遇到问题：

1. 查看日志：`docker-compose logs -f`
2. 检查状态：`docker-compose ps`
3. 重启服务：`docker-compose restart`
4. 查看文档：`DOCKER_DEPLOYMENT.md`

## ✅ 验收清单

部署完成后，确认以下项目：

- [ ] 前端可以访问 (http://localhost:3000)
- [ ] 后端健康检查通过 (http://localhost:8080/api/health)
- [ ] 前端健康检查通过 (http://localhost:3000/health)
- [ ] 界面显示青色主题（Meridian 设计系统）
- [ ] 毛玻璃效果正常显示
- [ ] 所有功能正常工作

---

**更新日期**: 2026-02-21  
**设计系统**: Meridian v1.0  
**状态**: ✅ 就绪



