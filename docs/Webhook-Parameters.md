# 完整命令参数

以 `webhook -h` 查看当前平台实际支持项。下面的服务参数也可写成 YAML 总配置同名键；`-c` 是预处理的配置入口，不出现在标准 flag 帮助中。

## 配置、监听与运行

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `-c` / `-config` / `--config` | 空 | YAML 总配置路径 |
| `-hooks` | 无输入时尝试 hooks.json | Hook 文件，可重复；JSON/YAML |
| `-hooks-dir` | 空 | Hook 目录，可重复；只扫描直接子文件并始终监听 |
| `-hotreload` | false | 监听显式 hooks 文件变化 |
| `-template` | false | 按 Go 模板渲染 Hook 文件 |
| `-ip` | 0.0.0.0 | 监听 IP |
| `-port` | 9000 | 监听端口 |
| `-socket` | 空 | Unix socket 或 Windows named pipe；优先于 IP/port |
| `-urlprefix` | hooks | Hook URL 前缀 |
| `-status-path` | status | 状态接口路径，仅 GET/HEAD |
| `-nopanic` | false | 无法加载 hooks 时允许非 verbose 服务继续运行 |
| `-pidfile` | 空 | 写 PID 文件 |
| `-setuid` / `-setgid` | 0 | Unix 打开监听端口后降权，需同时设置，不可配 socket；Windows 不支持 |
| `-version` | false | 输出版本后退出 |

## HTTP、安全与执行

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `-http-methods` | 空（全部） | 逗号分隔默认方法，如 POST,PUT；Hook 非空配置可覆盖 |
| `-header` | 空 | 响应头，格式 `Name=Value`，可重复 |
| `-command-timeout` | 空（不限） | 默认命令超时，Go duration；0 禁用 |
| `-max-concurrency` | 0 | 每个 Hook 默认并发上限，0 不限 |
| `-max-body-size` | 10485760 | Hook 请求体上限字节（10 MiB）；0 不限；不能负数 |
| `-max-multipart-mem` | 1048576 | multipart 内存阈值（1 MiB），超过可落盘 |
| `-real-ip-header` | 空 | 真实 IP 头，例如 X-Real-IP / X-Forwarded-For |
| `-trusted-proxies` | 空 | 信任的代理 IP/CIDR，逗号分隔；只信任这些对端发来的真实 IP 头 |
| `-access-whitelist` | 空 | 普通 HTTP 接口来源白名单，IP/CIDR 逗号分隔 |
| `-access-blacklist` | 空 | 普通 HTTP 接口来源黑名单，与白名单互斥 |
| `-secure` | false | 开启原生 HTTPS |
| `-cert` | cert.pem | HTTPS PEM 证书路径 |
| `-key` | key.pem | HTTPS PEM 私钥路径 |
| `-tls-min-version` | 1.2 | 最低 TLS，允许 1.0/1.1/1.2/1.3；生产建议至少 1.2 |
| `-cipher-suites` | 空（Go 默认） | 逗号分隔 TLS cipher 名称；不用于配置 TLS 1.3 套件 |
| `-list-cipher-suites` | false | 列出支持的 TLS 套件后退出，可配 tls-min-version |

白名单以外的来源会被拒绝；黑白名单保护普通 hooks、公开路由、状态和 Admin。组网控制/数据端点使用独立 Agent 认证，详见[组网指南](Network.md)。`max-body-size` 限制 Hook body，不是整个 HTTP 服务器每个 API 的统一 body 限额。

## 日志与追踪

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `-verbose` | false | 打印运行日志 |
| `-debug` | false | 请求调试输出，同时启用 verbose；可能包含敏感数据 |
| `-logfile` | 空 | 追加日志到文件，同时启用 verbose |
| `-x-request-id` | false | 采用传入 X-Request-Id 作为请求 ID |
| `-x-request-id-limit` | 0（不限） | 截断传入请求 ID 的长度 |

## Admin、更新与组网

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `-admin` | false | 开启管理界面/API |
| `-admin-path` | admin | 管理路径前缀 |
| `-admin-totp-secret` | 空 | Base32 TOTP 密钥，启用 Admin 时必填 |
| `-admin-jwt-secret` | 空 | 会话 HMAC 密钥，启用 Admin 时必填 |
| `-admin-session-ttl` | 12h | 会话有效期，须大于 0 |
| `-update-enabled` | true | 允许 Admin 更新检查 |
| `-update-repository` | xtulnx/webhook | Admin 更新来源仓库 |
| `-update-state-dir` | 空 | 预留配置项，当前 Admin 不读取该目录，也不影响 CLI 子命令 |
| `-network-config` | 空 | 独立组网 YAML 路径，覆盖总配置的 network 节 |

`network:` 是总配置中的专用嵌套对象，不是可以直接放进 CLI 的字符串参数。字段详见 [Network.md](Network.md)。

## 子命令

子命令使用自己的参数，不继承服务总配置或服务级 `-update-*`。`webhook agent --help`、`webhook update --help` 提供对应帮助。

| 子命令 | 参数 |
| --- | --- |
| `update check` | `--repository`、`--version`、`--json` |
| `update apply` | `--repository`、`--version`、`--target`、`--state-dir`、`--yes`、`--allow-unsigned` |
| `update rollback` | `--target`、`--state-dir` |
| `agent` | `--server`、`--id`、`--token`/`--token-env`/`--token-file`、`--ca-file`、`--duration`、`--max-connections`、可重复的 `--share`/`--publish`/`--forward` |

临时 Agent 默认 duration=1h、max-connections=8，令牌三选一；正式 YAML Agent 默认 max_connections=64。更新子命令默认仓库为 xtulnx/webhook，apply/rollback 状态目录默认当前工作目录，target 默认当前可执行文件。使用说明和权限约束分别见[运维](Operations.md)、[组网](Network.md)。
