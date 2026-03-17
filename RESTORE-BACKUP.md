# 从旧电脑备份恢复 nofx Docker 部署

你的备份分布在两个位置，需要按下面步骤合并并恢复。

## 备份内容说明

| 位置 | 内容 | 用途 |
|------|------|------|
| `E:\Downloads\Backup\docker` | `data/`、`keys/`、`.env` | 运行时数据与配置 |
| `E:\Downloads\Backup\docker-images` | `nofx-nofx.tar`、`nofx-nofx-frontend.tar` | Docker 镜像（docker save 导出） |

当前**尚未完成**的恢复步骤：未把备份的数据/配置复制到项目目录，也未把 `.tar` 镜像加载进本机 Docker。

---

## 一、把数据与配置恢复到项目目录

在**项目根目录**使用备份里的 `data`、`keys`、`.env`（不要用空目录或其它环境的配置）：

```powershell
# 进入 nofx 项目目录
cd E:\Downloads\Backup\nofx

# 若当前 data/keys 不是从旧电脑来的，用备份覆盖（请先备份当前若有重要内容）
Copy-Item -Path "E:\Downloads\Backup\docker\data\*" -Destination ".\data\" -Recurse -Force
Copy-Item -Path "E:\Downloads\Backup\docker\keys\*" -Destination ".\keys\" -Recurse -Force
Copy-Item -Path "E:\Downloads\Backup\docker\.env" -Destination ".\.env" -Force

# 若没有 logs 目录可创建（compose 里可能挂载了 ./logs）
New-Item -ItemType Directory -Force -Path ".\logs"
```

这样项目目录下的 `data`、`keys`、`.env` 就与旧电脑一致，用于后续启动容器。

---

## 二、把备份的镜像加载进本机 Docker

`.tar` 是 `docker save` 导出的镜像，需要在本机用 `docker load` 导入：

```powershell
docker load -i "E:\Downloads\Backup\docker-images\nofx-nofx.tar"
docker load -i "E:\Downloads\Backup\docker-images\nofx-nofx-frontend.tar"
```

执行后终端会输出类似：`Loaded image: nofx-nofx:latest`。请记下实际输出的镜像名（含 tag）。

若输出的名字**不是** `nofx-nofx:latest` 和 `nofx-nofx-frontend:latest`，需要改 `docker-compose.restore.yml` 里的 `image` 为实际名称，或用 `docker tag` 打成这两个名字再用 compose 启动。

---

## 三、用恢复专用 compose 启动

项目里已添加 `docker-compose.restore.yml`，使用本机刚加载的镜像（不拉取 ghcr.io）：

```powershell
cd E:\Downloads\Backup\nofx
docker compose -f docker-compose.restore.yml up -d
```

检查容器与健康状态：

```powershell
docker compose -f docker-compose.restore.yml ps
docker compose -f docker-compose.restore.yml logs -f
```

---

## 四、若 load 后的镜像名与 compose 不一致

先查看本机镜像名：

```powershell
docker images | findstr nofx
```

若例如是 `nofx_nofx` 或带其它 tag，可以打 tag 成 restore 用的名字：

```powershell
docker tag <实际镜像名> nofx-nofx:latest
docker tag <实际前端镜像名> nofx-nofx-frontend:latest
```

再执行第三步的 `docker compose -f docker-compose.restore.yml up -d`。

---

## 五、简要自检

- 后端健康：浏览器或 `curl http://localhost:8080/api/health`
- 前端：浏览器打开 `http://localhost:3000`（或你在 `.env` 里设的 `NOFX_FRONTEND_PORT`）

按上述顺序做完「复制 data/keys/.env → docker load 两个 .tar → compose restore up」，即算从备份正确恢复 nofx 的 Docker 部署。
