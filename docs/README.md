# 中文文档导航

本指南面向当前仓库的实现，包含上游 webhook 能力和本分支的 Admin、执行限制、状态接口及内网组网扩展。第一次使用按“快速开始 → 配置 → Hook → 演示”阅读；部署时再看运维指南。

| 文档 | 内容 |
| --- | --- |
| [快速开始](Quickstart.md) | 构建、第一次请求、预期输出、Windows 用法 |
| [总配置与加载](Configuration.md) | CLI/YAML 优先级、路径、文件/目录、热更新 |
| [完整参数表](Webhook-Parameters.md) | 所有服务参数、默认值、子命令 |
| [Hook 字段参考](Hook-Definition.md) | 每个字段、默认行为、同步/异步、执行限制 |
| [请求取值](Referencing-Request-Values.md) | Header、Query、JSON/XML、表单、文件、环境变量 |
| [触发规则](Hook-Rules.md) | AND/OR/NOT、值、正则、HMAC、IP、Scalr |
| [配置模板](Templates.md) | 环境变量、文件密钥、systemd credentials |
| [演示与场景](Hook-Examples.md) | 可运行演示及现有部署场景索引 |
| [Admin 管理](Admin.md) | TOTP、会话、UI、配置修改与完整 API |
| [内网组网](Network.md) | Hub/Agent、HTTP 路由、TCP、级联、临时文件共享 |
| [部署与运维](Operations.md) | 安装、TLS、Nginx、systemd、更新、排错 |
| [systemd socket 激活](Systemd-Activation.md) | 由 systemd 持有监听 socket |
| [开发与项目结构](Development.md) | Go Modules、vendor 的取舍、迁移、构建测试及发布 |

## 功能覆盖

- 请求支持 JSON 对象/数组、XML、URL 编码表单、multipart 和原始请求体。
- 请求字段可映射到命令参数、环境变量、临时文件；支持 Base64 解码和嵌套 JSON。
- 可组合值、正则、IP、HMAC SHA-1/256/512、Scalr 签名规则。
- 可控制 HTTP 方法、响应头、状态码、输出回传、命令超时和每个 hook 的并发数。
- 支持多文件、目录自动发现、自动或信号重载、YAML 总配置及 Go 模板。
- 提供可信代理真实 IP、服务级 IP 黑白名单、请求体大小限制、TLS、Unix socket / Windows named pipe、PID、请求日志与状态信息。
- 内嵌 Admin 通过 TOTP 登录及 JWT 会话管理已有 hook 文件；认证 API 可查询更新和组网节点。
- Hub 接收公网 HTTP 回调，Agent 主动连出并执行内网 hook；还支持授权 TCP 转发、级联、限时只读共享。
- 安装脚本和多平台 Release、更新清单签名验证、二进制替换及回滚。

配置中引用的命令以服务用户身份执行。演示只打印数据、读取演示文件或等待，不执行真实发布。
