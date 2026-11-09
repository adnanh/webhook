# 部署与运维

## 安装和构建

源码构建：[快速开始](Quickstart.md)。规范命令为 `go build -o build/webhook ./cmd/webhook`，根目录 `go build` 继续可用。以下脚本自动识别系统和架构，安装或更新到 GitHub Releases 的最新正式版本；重复执行同一命令即可更新，不需要 Go 环境。

Linux、macOS、FreeBSD、OpenBSD：

```sh
curl -fsSL https://raw.githubusercontent.com/xtulnx/webhook/master/scripts/install.sh -o install.sh
sh install.sh
webhook -version
```

默认安装到 `/usr/local/bin`，无写权限时使用 sudo。安装到用户目录、不需要 sudo：

```sh
WEBHOOK_INSTALL_DIR="$HOME/.local/bin" sh install.sh
# 若尚未配置 PATH：
export PATH="$HOME/.local/bin:$PATH"
```

Windows 使用 PowerShell 5.1 或更新版本，默认安装到 `%LOCALAPPDATA%\Programs\webhook`，并添加到用户 PATH，无需管理员权限：

```powershell
Invoke-WebRequest -UseBasicParsing https://raw.githubusercontent.com/xtulnx/webhook/master/scripts/install.ps1 -OutFile install.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\install.ps1
# 新开终端后可直接运行：
webhook -version
```

SHA256 校验和 ZIP 解压直接使用 .NET，不依赖 `Get-FileHash` / `Expand-Archive` 的模块自动发现，无需另外安装 tar；旧 Release 若只有 tar.gz，在系统已有 `tar.exe` 时自动回退安装。下载后可以先审阅脚本；执行策略仅对本次 PowerShell 进程生效。仓库中还提供 CMD/双击入口，需将 `install.cmd` 和 `install.ps1` 放在同一目录：

```bat
scripts\install.cmd
scripts\install.cmd -Version v1.0.0 -InstallDir "D:\Tools\webhook"
```

以上版本号仅作格式示例，需替换为真实发布版本。PowerShell 可直接使用 `-Version`、`-InstallDir`、`-Repo`、`-BinaryName` 参数；两个平台均支持以下环境变量：

| 环境变量 | 默认值 / 作用 |
| --- | --- |
| `WEBHOOK_REPO` | `xtulnx/webhook`，格式为 owner/repo |
| `WEBHOOK_VERSION` | `latest`，也可指定 `vX.Y.Z` 或 `X.Y.Z`；预发布版本需显式指定 |
| `WEBHOOK_INSTALL_DIR` | Unix `/usr/local/bin`；Windows `%LOCALAPPDATA%\Programs\webhook` |
| `WEBHOOK_BINARY_NAME` | `webhook` / `webhook.exe`，仅文件名，Windows 自动补 `.exe` |

```sh
WEBHOOK_VERSION=v1.0.0 WEBHOOK_INSTALL_DIR="$HOME/.local/bin" sh install.sh
```

脚本先将 latest 解析为固定 tag，再下载对应归档及 checksums.txt，强制核验 SHA256，运行新程序确认版本，最后替换目标。下载、校验、解压或版本检查失败时保留原文件。Unix 使用同目录临时文件和重命名，可替换正在运行的二进制；Windows 更新前需停止占用 webhook.exe 的服务或进程。

安装脚本只替换指定目录下的程序，不修改服务配置，也不自动重启服务。更新已有安装时，`WEBHOOK_INSTALL_DIR` / `-InstallDir` 应指向原安装目录；安装后手动重启服务。需要更新状态和回滚记录时，使用下文内置 `webhook update` 命令。

运行环境需要能访问 GitHub API、GitHub Releases 及其下载域名。Unix 需要 curl 或 wget、tar，以及 sha256sum / shasum / sha256 中的一种；缺少校验工具时会报错。Windows ZIP 和安装脚本也随新 Release 发布。安装脚本的 SHA256 校验与下述更新清单签名验证是两个不同机制。

## 历史版本迁移

当前版本线从 `v1.0.0` 开始；源码构建默认显示 `1.0.0-dev`，正式发布以 tag 为准。历史安装若显示 `2.8.3` 等版本，内置更新器不会自动将较小的 `1.0.0` 判断为更新。首次使用安装脚本指定已发布的 1.x 版本完成迁移，并指向原安装目录；命令名、配置和服务路径可继续沿用。操作步骤见[迁移说明](Project-Origin.md#从历史-2x-构建迁移)。

## systemd 常驻

先准备服务用户、可读配置、可执行脚本和可写工作目录。示例路径按自己的机器调整：

```ini
[Unit]
Description=Webhook service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=webhook
Group=webhook
WorkingDirectory=/var/lib/webhook
ExecStart=/usr/local/bin/webhook -c /etc/webhook/webhook.yaml
Restart=on-failure
RestartSec=5
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
```

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now webhook
sudo systemctl status webhook
journalctl -u webhook -f
```

上面单元通过 journald 收集标准错误；需要运行日志时在总配置设置 `verbose: true`。Hook 命令继承 `User` 和 `Group`；不要为了监听低端口让部署脚本长期以 root 执行，可通过反向代理或 [socket 激活](Systemd-Activation.md) 监听。

PID 文件用于外部进程管理，systemd 通常无需它。Unix SIGTERM/Ctrl+C 会移除 PID 文件及普通 Unix socket；当前退出流程不提供“等待全部异步命令完成”的保证。重启前评估任务状态。

## 原生 HTTPS 与 socket

```sh
webhook -hooks /etc/webhook/hooks.yaml -secure \
  -cert /etc/webhook/fullchain.pem -key /etc/webhook/privkey.pem \
  -tls-min-version 1.2 -verbose
webhook -list-cipher-suites -tls-min-version 1.2
# Unix socket（父目录需存在且可写）
webhook -hooks /etc/webhook/hooks.yaml -socket /run/webhook/webhook.sock -verbose
```

Windows 可使用 `-socket '\\.\pipe\webhook'`。socket 优先于 IP/port；Unix `setuid` 和 `setgid` 必须配对，且不能与 socket 一起使用。证书续期后需重启进程，Hook 热重载不会重载证书。TLS 1.3 cipher 不由 `cipher-suites` 控制。

## 反向代理与访问控制

总配置示例：

```yaml
ip: 127.0.0.1
port: 9000
real-ip-header: X-Real-IP
trusted-proxies: 127.0.0.1,::1
# 白名单和黑名单只配置一种；按实际访问者网段调整。
access-whitelist: 192.0.2.0/24,127.0.0.1,::1
max-body-size: 10485760
command-timeout: 30s
max-concurrency: 2
```

Nginx 的 HTTPS server 内：

```nginx
location / {
    proxy_pass http://127.0.0.1:9000;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $remote_addr;
    proxy_set_header X-Forwarded-Proto $scheme;
    client_max_body_size 10m;
    proxy_read_timeout 65s;
}
```

只信任自己控制的代理地址，不把所有私网网段一概作为可信代理。`X-Forwarded-For` 模式从右向左找第一个不可信地址；请求体经过 Hub 转发时由 Hub 设置受保护的来源信息。完整 WebSocket/Nginx 配置见 [组网指南](Network.md)。Nginx 与应用上限应相互配合；请求体限制针对 Hook 解析，Admin 登录等入口应另在代理限制。

Admin 默认为关闭，启用时建议单独限制来源和登录频率；反向代理终止 TLS 时注意会话 cookie 的 Secure 属性，见 [Admin](Admin.md)。日志可能记录参数、环境、输出，debug 还可能暴露请求体；密钥和个人信息应在外围日志系统做好访问控制和保留期设置。

## 检查、更新、应用与回滚

```sh
webhook update check
webhook update check --json
webhook update check --version vX.Y.Z
webhook update apply --state-dir /var/lib/webhook/update --yes
webhook update rollback --state-dir /var/lib/webhook/update
```

`vX.Y.Z` 代表实际存在的发布 tag。CLI 默认更新来源 `xtulnx/webhook`，`--repository owner/repo` 可指定其他兼容仓库；它不读取服务总配置中的 `update-repository`。`--target` 可指定替换目标，默认当前可执行文件。状态目录默认**当前工作目录**，不是可执行文件目录；apply 和 rollback 应指定同一个目录。Admin 只显示进程内最近检查结果，当前不会读取此磁盘状态；服务参数 `update-state-dir` 尚未接入实际逻辑。

更新机制下载对应 GOOS/GOARCH 的归档，校验 SHA256、检查更新清单 Ed25519 签名（有内置公钥时），执行新二进制确认版本，再替换目标，并保存 `<目标>.previous` 与状态文件。目标目录和状态目录都需相应读写权限。未内置受信公钥时 check 可查询，但 apply 默认拒绝；`--allow-unsigned` 是显式跳过清单签名信任的降级选项，不建议日常启用。有公钥而签名无效时应查清原因。

应用/回滚不会自动重启已运行服务，之后执行对应服务管理命令，例如 `sudo systemctl restart webhook`。Windows 文件替换采用平台专用流程，仍需确认服务重启成功。不要同时执行多个更新/回滚操作。

Admin 只提供认证后的状态和检查，不提供安装接口。`update-enabled: false` 禁用 Admin 检查，并不禁用独立 CLI 子命令。

## 排错表

| 表现 | 检查方式 |
| --- | --- |
| 启动找不到 hooks | 分清总配置对象与 Hook 数组；检查工作目录和绝对路径；临时启用 verbose |
| `/status` 是 degraded | 检查 hooks.count 和 loadErrors；加载失败会保留旧配置；纯 Hub 无 hook 也可能如此 |
| HTTP 200 但没执行 | 规则不匹配默认 200，显式改成 403；查看规则日志 |
| HTTP 500 | 同步命令退出、超时、签名/规则错误；用本机受控请求复现，谨慎启用输出回传 |
| HTTP 503 | 当前 Hook 并发已满，或 Hub 没有可用路由节点；查看对应层日志 |
| HTTP 413 | 应用 max-body-size 或前置代理 body 限制；max-multipart-mem 不是上传上限 |
| 命令找不到/权限拒绝 | execute-command 只填路径；检查可执行位、shebang、PATH、工作目录、服务用户 |
| 上传文件消失 | 文件只在命令生命周期存在；避免交给脱离父命令的后台进程 |
| Admin 无法登录 | Base32 密钥、六位码、两端时间、JWT secret、路径和代理 cookie 设置 |
| Admin 保存失败 | 模板只读、文件未加载、权限不足、重复 Hook ID、schema 错误 |
| IP 限制错误 | trusted-proxies 只列真实代理；检查反代覆盖头与 XFF 链 |
| 修改配置没生效 | 仅 Hook 定义支持重载；总配置、网络路由、证书、密钥需重启 |
| 组网路由失败 | 按 TLS → 节点令牌 → 服务/hook 清单 → Hub 路由 → 最终规则顺序排查 |
| 更新被拒绝 | 确认版本归档存在、目标/状态目录权限、签名公钥与清单匹配 |
