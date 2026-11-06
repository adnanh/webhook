# Hook 配置模板

启动参数 `-template` 或总配置 `template: true` 先把 **Hook 文件**作为 Go `text/template` 渲染，再解析 JSON/YAML。总配置本身不执行模板；目录自动发现只识别 `.json`、`.yaml`、`.yml`，以 `.tmpl` 结尾的模板用 `-hooks` 显式加载。

```sh
# 演示密钥只含十六进制字符，避免 YAML 引号转义问题。
export DEMO_HMAC_SECRET="$(openssl rand -hex 32)"
./build/webhook -hooks examples/signature/hooks.yaml.tmpl -template -ip 127.0.0.1 -verbose
```

```yaml
trigger-rule:
  match:
    type: payload-hmac-sha256
    secret: '{{ getenv "DEMO_HMAC_SECRET" }}'
    parameter: {source: header, name: X-Hub-Signature-256}
```

模板使用标准 Go 语法（条件、循环、变量及内置函数）并额外提供：

| 函数 | 示例 | 说明 |
| --- | --- | --- |
| `getenv` | `{{ getenv "WEBHOOK_SECRET" }}` | 读取环境变量；缺失时空串 |
| `cat` | `{{ cat "/run/secrets/webhook" }}` | 读取文件原始内容；不存在/读取失败时空串 |
| `credential` | `{{ credential "webhook-secret" }}` | 从 `$CREDENTIALS_DIRECTORY/webhook-secret` 读取；未设目录时读同名环境变量 |

`cat`、`credential` 可能读取到文件末尾换行；密钥文件应按预期写入原始值。空 secret 会使 HMAC 校验失败，应在部署启动前确认环境和文件已注入。

## 转义和密钥

JSON 模板中可用 `js` 进行 JSON 字符串转义：

```json
"secret": "{{ getenv "WEBHOOK_SECRET" | js }}"
```

YAML 与 JSON 的转义方式不同，不能把 JSON 的 `js` 规则直接当作任意 YAML 字符串转义方案。随附 YAML 示例约束密钥为十六进制字符串。复杂任意文本可选择 JSON 模板并验证解析结果。

开启模板时 Admin 可以查看配置，但所有 Hook 写入禁用，避免把渲染后的内容覆盖模板源文件。模板能读取进程环境和本地文件，只加载自己信任的模板。密钥轮换后需触发重载；环境变量若在外部变更，则需重启进程才能获得新环境。

完整示例：[YAML 模板](../examples/config/hooks.yaml.tmpl.example)、[JSON 模板](../examples/config/hooks.json.tmpl.example)。
