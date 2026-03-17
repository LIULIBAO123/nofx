# nofx 整体情况检查报告

检查时间：2026-03-09

---

## 一、Docker 部署状态

| 项目 | 状态 |
|------|------|
| **后端 nofx-trading** | 运行中，健康检查通过 (healthy) |
| **前端 nofx-frontend** | 运行中，健康检查显示 unhealthy（见下方说明） |
| **后端端口** | 8080 |
| **前端端口** | 3000 |
| **使用配置** | `docker-compose.restore.yml`（本地备份镜像） |

### 镜像

| 镜像名 | 标签 | 大小 |
|--------|------|------|
| nofx-nofx | latest | 445 MB |
| nofx-nofx-frontend | latest | 106 MB |

### 前端显示 unhealthy 说明

- 前端容器实际在正常提供页面（访问 http://localhost:3000 返回 200）。
- 健康检查命令为 `wget --spider http://localhost:80`，在部分环境下可能判定失败。
- 若需改为“通过”，可将 `docker-compose.restore.yml` 中前端 healthcheck 改为：  
  `http://127.0.0.1/health`（与 nginx 中 `/health` 一致）。

---

## 二、服务可用性

| 检查项 | 结果 |
|--------|------|
| 后端健康接口 `GET /api/health` | 200 OK |
| 前端首页 http://localhost:3000 | 可访问，静态资源与页面正常 |
| 前端请求后端 `GET /api/config` | 200 OK |
| 前端请求 `GET /api/klines?symbol=...` | **500**（见下方说明） |

### /api/klines 返回 500

- 日志显示多个交易对（如 BTCUSDT、ETHUSDT）的 klines 请求返回 500。
- 可能原因：交易所接口未配置、网络限制、或数据源/API Key 问题。
- 不影响登录与基础功能，仅影响行情/图表数据。可在后端日志中搜索 `klines` 或 `500` 进一步排查。

---

## 三、项目结构与配置

### 目录与关键文件

- **代码与配置**：`api/`、`auth/`、`cmd/`、`config/`、`docker/`、`web/`、`go.mod`、`main.go` 等齐全。
- **运行所需**：  
  - `.env` 存在（约 2.6 KB）  
  - `data/` 存在（约 19 个文件/子目录）  
  - `keys/` 存在（2 个文件，应为 RSA 密钥相关）  
  - `logs/` 已创建  
- **Docker**：  
  - `docker-compose.yml`（本地构建）  
  - `docker-compose.prod.yml`（拉取 ghcr.io 镜像）  
  - `docker-compose.restore.yml`（使用本地备份镜像，当前在用）  
  - `docker-compose.stable.yml`  

### 文档与脚本

- **恢复与迁移**：`RESTORE-BACKUP.md`、`docs/Cursor-Docker-存储位置与迁移.md`
- **脚本**：`scripts/move-cursor-to-e.ps1`、`scripts/move-docker-to-e.ps1` 等

---

## 四、总结与建议

| 项目 | 结论 |
|------|------|
| 部署方式 | 已用备份镜像 + 恢复用 compose 正确跑起前后端 |
| 后端 | 正常，健康检查通过 |
| 前端 | 实际可用，仅健康检查显示异常，可改 healthcheck 或忽略 |
| 行情接口 | klines 当前 500，需根据环境检查交易所/API 配置与网络 |
| 数据与密钥 | .env、data、keys 齐全，备份恢复有效 |

**建议**  
1. 若只做本地使用，当前状态可正常使用，无需强制改前端 healthcheck。  
2. 需要图表/行情时：在后端日志中查 klines/交易所相关错误，并核对 `.env` 中交易所 API 配置与网络。  
3. 日常可用的自检命令：  
   - `docker compose -f docker-compose.restore.yml ps`  
   - `curl http://localhost:8080/api/health`  
   - 浏览器打开 http://localhost:3000  
