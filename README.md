# 目录同步程序

一个用 Go 编写的高性能目录同步工具，可以实时监控本地目录变化并同步到远程 VPS。

## 功能特性

- **实时监控**：使用 fsnotify 库监控文件系统变化
- **后台运行**：编译成 Windows 无窗口后台程序
- **自动重连**：网络断开时自动重连 SSH/SFTP
- **智能同步**：防抖机制避免频繁同步
- **灵活配置**：支持忽略文件模式、同步间隔等配置
- **日志轮转**：自动管理日志文件大小

## 快速开始

### 1. 编译程序

```bash
# Windows
build.bat

# Linux/macOS
go build -o directory-sync
```

### 2. 配置文件

修改 `config.yaml`：

```yaml
local_directory: "C:\\path\\to\\local\\directory"
remote_directory: "/home/user/remote/directory"

ssh:
  host: "your-vps-ip"
  port: 22
  username: "your-username"
  password: "your-password"

sync:
  ignore_patterns:
    - ".git"
    - "*.tmp"
    - "*.log"
  delete_remote: false
  sync_interval: 2

log:
  level: "info"
  file: "sync.log"
  max_size_mb: 10
```

### 3. 运行程序

```bash
# 后台运行（无窗口）
directory-sync.exe

# 调试模式（显示窗口）
directory-sync-debug.exe
```

## 编译选项

- **directory-sync.exe**: 后台运行，无控制台窗口
- **directory-sync-debug.exe**: 显示控制台窗口，便于调试

## 配置说明

| 配置项 | 说明 | 必填 |
|--------|------|------|
| `local_directory` | 本地监控目录路径 | 是 |
| `remote_directory` | 远程目标目录路径 | 是 |
| `ssh.host` | VPS IP 地址 | 是 |
| `ssh.username` | SSH 用户名 | 是 |
| `ssh.password` | SSH 密码 | 是* |
| `ssh.private_key_path` | SSH 私钥路径 | 是* |
| `sync.ignore_patterns` | 忽略文件模式列表 | 否 |
| `sync.delete_remote` | 是否删除远程不存在文件 | 否 |
| `sync.sync_interval` | 同步防抖间隔（秒） | 否 |

*注：密码和私钥至少提供一个

## 停止程序

- Windows: Ctrl+C 或结束进程
- Linux/macOS: Ctrl+C 或 `kill -TERM <pid>`

## 日志文件

程序运行时会生成 `sync.log` 文件，记录同步状态和错误信息。

## 注意事项

1. 确保 VPS 上的目标目录存在且有写权限
2. 首次运行会进行完整同步，耗时取决于文件数量
3. 大文件同步时请保持网络稳定
4. 建议在测试环境先验证配置正确性