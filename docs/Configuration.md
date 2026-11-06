# 总配置、路径与热加载

## 两类 YAML 的区别

总配置是一个对象，键对应 CLI 选项；Hook 配置是一个数组，每项描述一个命令入口。总配置通过 `hooks` 引用 Hook 文件，两者不能混成同一份顶层结构。

```yaml
# webhook.yaml：总配置
ip: 127.0.0.1
port: 9000
hooks:
  - /etc/webhook/hooks/deploy.yaml
hooks-dir:
  - /etc/webhook/hooks.d
verbose: true
command-timeout: 30s
max-concurrency: 2
header:
  - X-Service=webhook
```

```sh
webhook -c webhook.yaml
webhook -config webhook.yaml -port 9001
webhook --config=webhook.yaml -verbose=false
```

`-c`、`-config`、`--config` 支持空格或 `=` 赋值。总配置未知键会导致启动失败。它不是通用环境变量模板，`template` 只处理 Hook 文件。

## 优先级

标量的优先级为 CLI > 总配置 > 内置默认值。布尔 CLI 显式关闭时使用 `-verbose=false` 等写法。Go 标准 flag 不支持把 `-verbose false` 当作布尔赋值。

`hooks`、`hooks-dir`、`header` 是可重复参数，YAML 列表和 CLI 指定值会**追加**；CLI 不会替换整份列表。例如配置加载 A 文件、CLI 再传 B 文件，最终加载 A 和 B。跨文件 Hook ID 必须唯一。

没有指定任何 `hooks` 或 `hooks-dir` 时，普通服务尝试读取当前目录的 `hooks.json`。纯 Hub 无需 hooks；纯 Agent 不加载本地 hooks。`-nopanic` 允许普通服务在未成功加载任何 hook 时继续运行；`-verbose` 模式也允许继续启动，但应结合 `/status` 判断是否正常。

## 相对路径基准

| 字段 | 相对路径基准 |
| --- | --- |
| `hooks`、`hooks-dir`、命令工作目录、证书、日志、PID | **启动进程的工作目录**，不是总配置目录 |
| 总配置内 `network-config` | 总配置文件所在目录 |
| CLI 的 `-network-config` | 启动工作目录 |
| `network:` 的 `token_file`、`ca_file` | 总配置文件所在目录 |
| 独立组网配置的 `token_file`、`ca_file` | 独立组网配置文件所在目录 |
| `execute-command` 相对路径 | 有 `command-working-directory` 时与其拼接，否则按当前目录/PATH 查找 |

部署时推荐绝对路径，并在 systemd 指定 `WorkingDirectory`。命令只有文件名时由 PATH 查找；当前目录可执行文件应显式写 `./program`。可参考 [完整配置示例](../examples/config/webhook.yaml.example)。

## 多文件与目录

```sh
webhook -hooks team-a.yaml -hooks team-b.json -hotreload
webhook -hooks-dir /etc/webhook/hooks.d -hooks-dir /opt/app/hooks
```

- `-hooks` 接受 JSON 或 YAML；可重复传入，重复路径会归一化、去重。
- `-hooks-dir` 扫描直接子目录项中 `.json`、`.yaml`、`.yml` 文件；**不递归**，不自动加载 `.tmpl` 文件。
- 目录始终自动监听新增、修改、删除，即使没有 `-hotreload`。
- 显式 `-hooks` 文件仅在 `-hotreload` 下自动监听；目录和显式文件可以混用。
- 自动更新以整份文件/目录候选配置校验唯一 ID。解析错误或重复 ID 时保留上一份有效配置，状态中记录加载错误。初始显式文件 ID 冲突可能直接阻止启动。
- 目录内文件删除会移除对应 hooks；显式文件删除事件在开启热重载时也会移除其 hooks。恢复文件后会重新加载。
- 指向配置目录外的符号链接会被忽略。

Unix 上可手动重载：

```sh
kill -HUP <PID>
# 或 kill -USR1 <PID>
```

这里只重载 Hook 定义，不重新读取总配置、TLS 证书、Admin 密钥或组网拓扑；后几类修改后重启进程。Windows 使用文件监听或重启。

## 状态接口

`GET /status`、`HEAD /status` 默认开放，无 Admin 会话要求；可通过 `status-path: health/webhook` 修改路径。该路径不能位于 hook 前缀内或与启用的 Admin 前缀重叠。

响应包含 `version`、`startedAt`、`uptimeSeconds`、hooks 数量、文件名及 ID、最后加载时间、加载错误计数和 watcher 状态。它不返回命令或密钥。没有 hook 或有加载错误时 `status` 为 `degraded`，但 **HTTP 仍返回 200**；监控需要检查 JSON 字段。纯 Hub 没有 hook 时可能显示 degraded，这不是 Agent 在线状态检查。

服务级 IP 黑白名单也保护状态接口。状态会透露 Hook ID 与文件名，需要隐藏时在反向代理限制访问。组网节点状态使用 [Admin 组网 API](Admin.md)。
