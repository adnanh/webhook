# 项目来源与版本迁移

## 项目概览

webhook 提供 Webhook 自动化、可视化配置管理和 Hub/Agent 内网执行能力。

| 项目项 | 当前约定 |
| --- | --- |
| 项目与命令名称 | `webhook` |
| 项目仓库 | [GitHub](https://github.com/xtulnx/webhook) |
| Go module | `github.com/xtulnx/webhook` |
| 规范构建入口 | `./cmd/webhook` |
| 安装与更新来源 | `xtulnx/webhook` 的 GitHub Releases |
| 当前版本线 | 从 `v1.0.0` 开始；预发布使用 `v1.0.0-rc.1` 等形式 |
| 未注入版本的源码构建 | `1.0.0-dev`，统一定义在 `internal/buildinfo` |

项目名、CLI、配置文件名、已有服务路径和 Hook URL 保持兼容。Go 包身份已迁移到本仓库；从源码开发时应使用新的 module/import 路径。

## 来源与致谢

本项目起源于 [adnanh/webhook](https://github.com/adnanh/webhook)，感谢 Adnan Hajdarevic 及上游贡献者建立的 HTTP Hook 执行、参数映射、触发规则等基础。

在此基础上，项目扩展了以下能力：

- YAML 总配置、多文件和目录加载、中文帮助与演示。
- Admin 可视化管理、TOTP 登录和认证 API。
- 命令超时、并发限制、状态接口、可信代理和访问控制。
- Hub/Agent 组网、内网 Hook 执行、HTTP/TCP 路由、级联及临时共享。
- 多平台发布、安装脚本、更新清单签名验证、二进制更新与回滚。

问题反馈和功能建议请提交到 [项目 Issues](https://github.com/xtulnx/webhook/issues)。

## 版本与发布约定

本项目采用 `v1.x.y` 版本线，和上游 `v2.x.y` 属于不同的发布序列，数字不能用于比较两个项目的功能新旧。历史提交及已有 tag 保留来源记录，不修改、移动或删除旧 tag。创建本项目的新版本时，应从包含 module 迁移和当前版本配置的提交打 tag。

Release 工作流从指定 tag 检出源码，将去掉 `v` 的版本注入 `main.version`。例如发布 `v1.0.0` 后，二进制输出 `webhook version 1.0.0`；普通源码构建显示 `1.0.0-dev`。两个入口 `go build` 和 `go build ./cmd/webhook` 使用相同的默认版本，并保留 `-X main.version=...` 覆盖方式。

正式 Release 在全部资产上传后显式标记为 GitHub Latest，避免继承的上游高版本号影响默认安装来源；预发布版本不会成为 Latest。手动补发旧版本时，应知晓正式发布会切换 Latest。

## 从历史 2.x 构建迁移

历史安装可能仍报告 `2.8.3` 等版本。内置更新器按语义版本比较大小，因此不会把本项目的 `1.0.0` 自动判定为比 `2.8.3` 更新。首次迁移使用安装脚本显式指定已发布的 1.x 版本，之后同一版本线内继续使用常规更新检查。

下面的 `v1.0.0` 仅在对应 Release 已实际发布后可用。迁移前保留当前二进制和配置，更新前确认正在执行的任务；安装脚本不会重启服务或生成内置更新器的回滚状态。

Linux / macOS / BSD：

```sh
curl -fsSL https://raw.githubusercontent.com/xtulnx/webhook/master/scripts/install.sh -o install.sh
# 安装目录需指向现有 webhook 所在目录；按实际路径调整。
WEBHOOK_VERSION=v1.0.0 WEBHOOK_INSTALL_DIR=/usr/local/bin sh install.sh
/usr/local/bin/webhook -version
```

Windows：先停止占用 webhook.exe 的服务或进程，再在 PowerShell 中执行：

```powershell
Invoke-WebRequest -UseBasicParsing https://raw.githubusercontent.com/xtulnx/webhook/master/scripts/install.ps1 -OutFile install.ps1
# 如使用其他安装目录，补充 -InstallDir 参数。
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\install.ps1 -Version v1.0.0
```

迁移后手动重启原服务，检查 `webhook -version`、服务状态和已有 Hook。后续更新与回滚的权限、签名和状态目录约定见[部署与运维](Operations.md)。

## 许可

项目沿用 [MIT License](../LICENSE)，保留上游版权和许可声明，并保留贡献者署名。提交历史和源码中的原作者版权信息继续保留。
