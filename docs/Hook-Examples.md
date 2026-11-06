# 演示与接入场景

可以直接运行的本机演示统一放在 [examples/README.md](../examples/README.md)，每个步骤包含构建、启动、请求、预期输出和停止方法。

| 场景 | 示例 | 学习目标 |
| --- | --- | --- |
| 最小请求与规则 | [basic](../examples/basic/hooks.yaml) | JSON 嵌套字段、Query、Header → 参数/环境，AND/regex |
| 签名验证 | [signature](../examples/signature/hooks.yaml.tmpl) | 模板密钥、HMAC-SHA256、分支匹配 |
| 文件和表单 | [files](../examples/files/hooks.yaml) | multipart、Base64 临时文件、表单 JSON |
| 命令执行控制 | [execution](../examples/execution/hooks.yaml) | 超时、每 Hook 并发、异步 202、失败输出 |
| 目录热加载 | [hotreload](../examples/hotreload/webhook.yaml) | 新增/删除/修改文件与状态接口 |
| Admin | [admin](../examples/admin/webhook.yaml.example) | 独立随机密钥、TOTP、在线编辑、API |
| 同机 Hub/Agent | [network](../examples/network/hub.yaml) | 不监听本地 webhook 端口的内网执行 |
| 完整参考配置 | [config](../examples/config/) | 总配置、JSON/YAML Hook、模板 |

## 接入 GitHub / GitLab 等平台

先选平台实际支持的认证方式。GitHub 常用 `X-Hub-Signature-256` 与 HMAC-SHA256，可在签名规则外通过 AND 组合 `X-GitHub-Event` 和 `payload.ref`；GitLab 的 token 模式可用 `type: value` 比较 `X-Gitlab-Token`，并通过可信 HTTPS 传输。平台字段约定应以其当前官方文档和真实测试请求为准。

部署命令使用固定脚本路径，仓库名/分支/提交 ID 通过独立 argv 传入，并在脚本里验证允许的值。部署需另行实现锁、回滚与幂等；本服务的异步执行不会持久化任务或自动重试。

## 已有场景

- [组网完整指南](Network.md)：原有示例在 [docs/examples/network](examples/network/)，包含 TCP、公开 Hook 路由、级联、临时节点和大文件上传。
- [OSS Web 发布](examples/oss-release/README.md)：保留已有上传 Hook、更新脚本和配置示例。这些脚本会操作实际部署目录，阅读说明并修改环境后再运行。

演示命令 `demo-hook` 只提供 echo、file、sleep、fail，不会访问仓库外的业务数据或执行部署。
