# 本地 Docker 部署更新指南

## 问题分析

1. **回测功能失败** - 可能是旧版本代码的问题
2. **AI 分析功能未显示** - 本地容器运行的是 2 天前的镜像，不包含最新的 AI 分析功能

## 解决方案：重新构建本地镜像

### 方法 1：快速重建（推荐）

```bash
cd c:/Users/21588/nofx

# 停止现有容器
docker compose down

# 重新构建镜像（包含最新代码）
docker compose build --no-cache

# 启动服务
docker compose up -d

# 查看日志确认启动成功
docker compose logs -f
```

### 方法 2：清理后重建（彻底）

```bash
cd c:/Users/21588/nofx

# 停止并删除容器
docker compose down -v

# 删除旧镜像
docker rmi nofx-nofx nofx-nofx-frontend

# 重新构建
docker compose build --no-cache

# 启动服务
docker compose up -d
```

### 方法 3：使用 PowerShell 一键脚本

```powershell
cd c:/Users/21588/nofx

# 停止服务
docker compose down

# 重新构建（显示进度）
docker compose build --no-cache --progress=plain

# 启动服务
docker compose up -d

# 等待服务启动
Start-Sleep -Seconds 10

# 检查服务状态
docker compose ps

# 查看日志
docker compose logs --tail=50
```

## 验证更新

### 1. 检查服务状态

```bash
docker compose ps
```

应该看到两个服务都是 `Up` 状态。

### 2. 检查 API 版本

```bash
curl http://localhost:8080/api/health
```

### 3. 测试 AI 分析 API

```bash
# 测试 AI 分析接口是否存在
curl -X POST http://localhost:8080/api/backtest/analyze-trade \
  -H "Content-Type: application/json" \
  -d '{"run_id": "test", "trade_id": 1}'
```

如果返回错误但不是 404，说明接口已部署。

### 4. 测试回测功能

访问 http://localhost:3000，进入回测实验室，尝试创建新回测。

## 新功能说明

### AI 交易分析功能

更新后，回测实验室将包含以下新功能：

1. **交易列表中的 AI 评级徽章**
   - 每笔交易显示 AI 评级（优秀/良好/一般/较差）

2. **交易详情弹窗**
   - 点击交易查看详细 AI 分析
   - 盈亏原因分析
   - 改进建议
   - 风险警告

3. **分析统计面板**
   - 优秀交易数量
   - 良好交易数量
   - 需改进交易数量

### 使用 AI 分析功能

1. 创建并运行回测
2. 在交易列表中，点击任意交易
3. 查看 AI 分析结果
4. 根据建议优化策略

## 故障排除

### 问题 1：构建失败

```bash
# 清理 Docker 缓存
docker builder prune -a

# 重新构建
docker compose build --no-cache
```

### 问题 2：端口被占用

```bash
# 检查端口占用
netstat -ano | findstr :8080
netstat -ano | findstr :3000

# 停止占用端口的进程或修改 .env 文件中的端口
```

### 问题 3：数据库错误

```bash
# 备份数据
cp data/nofx.db data/nofx.db.backup

# 重启服务
docker compose restart
```

### 问题 4：前端无法连接后端

```bash
# 检查网络
docker compose exec nofx-frontend wget -qO- http://nofx:8080/api/health

# 如果失败，重建网络
docker compose down
docker network prune
docker compose up -d
```

## 预期结果

更新后，你应该能够：

1. ✅ 成功创建回测
2. ✅ 看到交易的 AI 分析评级
3. ✅ 查看详细的交易分析报告
4. ✅ 获得策略改进建议

## 构建时间

- 首次构建：约 5-10 分钟
- 后续构建：约 2-5 分钟（有缓存）

## 注意事项

1. 构建过程中会下载依赖，需要网络连接
2. 构建完成后，数据库会自动迁移添加新字段
3. 现有的回测数据不会丢失
4. AI 分析功能需要配置 AI 模型才能使用

