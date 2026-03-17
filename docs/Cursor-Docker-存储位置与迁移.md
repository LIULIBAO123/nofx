# Cursor 与 Docker 在 Windows 上的存储位置及迁移到 E 盘

## 一、当前存储位置（均在 C 盘）

### Cursor

| 路径 | 说明 | 约大小 |
|------|------|--------|
| `C:\Users\HOYA\AppData\Roaming\Cursor` | 用户数据：设置、扩展、工作区缓存、聊天记录、Cache、Logs | **约 771 MB** |
| （程序本身可能在 `D:\cursor` 等，与数据目录无关） | 安装目录 | - |

- **User/settings.json**：编辑器设置  
- **User/workspaceStorage/**：各项目工作区数据（含聊天记录等）  
- **Cache/**：缓存，可清理  
- **logs/**：日志  

### Docker

| 路径 | 说明 | 约大小 |
|------|------|--------|
| `C:\Users\HOYA\AppData\Local\Docker\wsl` | WSL2 虚拟磁盘（镜像、容器、卷等） | **约 2.3 GB** |
| `C:\Users\HOYA\AppData\Roaming\Docker` | 桌面端设置（如 settings-store.json） | 约 0.05 MB |

- 主要占空间的是 **WSL2 的 ext4.vhdx**（在 `Local\Docker\wsl` 下），迁移 Docker 即迁移此部分到 E 盘。

---

## 二、迁移目标（E 盘新建目录）

统一在 E 盘使用一个根目录，便于管理：

| 新位置 | 用途 |
|--------|------|
| **E:\DevData\Cursor** | Cursor 用户数据（替代 Roaming\Cursor） |
| **E:\DevData\Docker\wsl** | Docker WSL2 数据（替代 Local\Docker\wsl） |

（可选）Roaming\Docker 仅设置文件，体积很小，可保留在 C 盘，或复制到 `E:\DevData\Docker\roaming` 并自行用符号链接指向新位置。

---

## 三、迁移步骤概览

1. **Cursor**：把 `Roaming\Cursor` 复制到 `E:\DevData\Cursor`，之后用“带 `--user-data-dir` 的快捷方式”启动 Cursor，使数据从 E 盘读取。  
2. **Docker**：关闭 Docker 与 WSL → 导出 `docker-desktop-data` → 注销该发行版 → 在 `E:\DevData\Docker\wsl` 重新导入 → 再启动 Docker。

具体操作见项目根目录下的脚本与说明：

- **Cursor**：在项目目录或任意位置打开 PowerShell，执行：  
  `Set-ExecutionPolicy -Scope CurrentUser -ExecutionPolicy RemoteSigned -Force`（若需放宽执行策略）  
  然后运行：`& "E:\Downloads\Backup\nofx\scripts\move-cursor-to-e.ps1"`  
  或按脚本内步骤手动执行。  
- **Docker**：**请先完全退出 Docker Desktop**，再以管理员身份运行：  
  `& "E:\Downloads\Backup\nofx\scripts\move-docker-to-e.ps1"`  
  或按脚本内步骤手动执行。

---

## 四、迁移后如何启动 Cursor（使用 E 盘数据）

- 使用脚本生成的快捷方式：**E:\DevData\Cursor\Cursor - 使用E盘数据.lnk**  
- 或命令行：  
  `"<Cursor安装路径>\Cursor.exe" --user-data-dir="E:\DevData\Cursor"`  

不要再用原来的开始菜单/桌面快捷方式（未带 `--user-data-dir`），否则会继续用 C 盘数据。

---

## 五、迁移后 Docker 行为

- 镜像、容器、卷等仍在 WSL2 的虚拟盘里，只是该虚拟盘文件从 C 盘改到 **E:\DevData\Docker\wsl**。  
- Docker Desktop 会自动使用新位置，无需改配置文件。  
- 若将来重装系统，把 `E:\DevData\Docker\wsl` 保留好，再按 Docker 官方文档重新导入 `docker-desktop-data` 即可恢复。
