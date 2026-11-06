# 部署与运维

## 安装和构建

源码构建：[快速开始](Quickstart.md)。规范命令为 `go build -o build/webhook ./cmd/webhook`，根目录 `go build` 继续可用。Release 安装脚本支持 Linux、macOS、FreeBSD、OpenBSD，以及 Windows PowerShell，按平台下载归档并核验 SHA256：

```sh
curl -fsSL https://raw.githubusercontent.com/xtulnx/webhook/master/scripts/install.sh | sh
```

```powershell
irm https://raw.githubusercontent.com/xtulnx/webhook/master/scripts/install.ps1 | iex
```

运行下载脚本前可先下载审阅。脚本需要 GitHub Releases 中存在对应资产；不假定某个尚未发布的版本可下载。

| 环境变量 | 默认值 / 作用 |
| --- | --- |
| `WEBHOOK_REPO` | xtulnx/webhook |
| `WEBHOOK_VERSION` | latest，也可指定已发布 tag |
| `WEBHOOK_INSTALL_DIR` | Unix `/usr/local/bin`；Windows `%LOCALAPPDATA%\Programs\webhook` |
| `WEBHOOK_BINARY_NAME` | webhook / webhook.exe |

Unix 可用 `WEBHOOK_INSTALL_DIR="$HOME/.local/bin" sh scripts/install.sh` 安装到用户目录；重复运行安装脚本可覆盖旧版。安装脚本的 SHA256 校验与下述更新清单签名验证是两个不同机制。

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
