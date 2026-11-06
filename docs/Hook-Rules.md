# 触发规则

`trigger-rule` 在执行命令前求值；没有规则时无需任何认证。规则不能替代脚本自己的参数校验和业务幂等设计。

## 逻辑组合

每个规则节点选择一个 `and`、`or`、`not` 或 `match`，可任意嵌套：

```yaml
trigger-rule:
  and:
    - match:
        type: value
        value: refs/heads/main
        parameter: {source: payload, name: ref}
    - or:
        - match:
            type: value
            value: push
            parameter: {source: header, name: X-GitHub-Event}
        - match:
            type: value
            value: workflow_dispatch
            parameter: {source: header, name: X-GitHub-Event}
    - not:
        match:
          type: value
          value: 'true'
          parameter: {source: payload, name: deleted}
```

`and` 要求全部通过；`or` 任一通过；`not` 对一个子规则取反。值、数字和布尔比较统一按字符串配置，因此 `value: 'true'`、`value: '42'` 应加引号。

## 所有 match 类型

| `type` | 其他字段 | 用途 |
| --- | --- | --- |
| `value` | `value`, `parameter` | 精确字符串比较 |
| `regex` | `regex`, `parameter` | Go 正则表达式；需要完整匹配时加 `^...$` |
| `payload-hmac-sha1` | `secret`, `parameter` | 校验原始 body 的 HMAC-SHA1 |
| `payload-hmac-sha256` | `secret`, `parameter` | 校验原始 body 的 HMAC-SHA256，推荐新接入使用 |
| `payload-hmac-sha512` | `secret`, `parameter` | 校验原始 body 的 HMAC-SHA512 |
| `ip-whitelist` | `ip-range` | 依据真实来源地址检查 IPv4/IPv6 IP 或 CIDR，无需 parameter |
| `scalr-signature` | `secret` | 检查 Scalr 签名和请求时间，允许距当前时间五分钟内的请求 |

兼容旧配置的 `payload-hash-sha1`、`payload-hash-sha256`、`payload-hash-sha512` 别名仍可用，但会记录弃用提示，建议改为对应 `payload-hmac-*`。

正则使用 [Go regexp 语法](https://pkg.go.dev/regexp/syntax)，不支持所有 PCRE 特性。

## HMAC 示例

```yaml
trigger-rule:
  match:
    type: payload-hmac-sha256
    secret: replace-with-your-secret
    parameter:
      source: header
      name: X-Hub-Signature-256
```

签名是对**发送的原始 body 字节**计算的摘要，不能先解析再重新序列化。SHA-256 可传 `sha256=<hex>`，SHA-1/512 对应 `sha1=`、`sha512=`；也接受无前缀摘要。同一 header 支持逗号分隔多个带算法前缀的签名，任一匹配即可。

完整签名和分支过滤演示见 [examples/signature](../examples/README.md#hmac-签名与模板)。生产密钥可通过 [模板](Templates.md) 从环境或文件加载。HMAC 本身不提供时间窗或重放保护；需要时在业务中校验事件 ID、时间戳和幂等键。

默认签名不匹配属于求值错误，可能直接返回 500，即使设置了普通规则不匹配状态码。需要同时兼容多种签名时，可在 Hook 上设置 `trigger-signature-soft-failures: true`，把签名失败变为 false，让 OR 继续尝试其他规则。该选项不表示签名可省略，应确保所有可通过分支都具有足够认证。

## IP 规则和可信代理

```yaml
trigger-rule:
  match:
    type: ip-whitelist
    ip-range: 192.0.2.0/24
```

单地址可用 `192.0.2.8` 或 `/32`；IPv6 单地址使用 `/128`。`ip-range` 支持以逗号或空白分隔多个 IP/CIDR，也可用 OR 组合多个 match。

本分支支持 `real-ip-header` 与 `trusted-proxies`：只有实际连接对端受信任时才采用代理头；`X-Forwarded-For` 从右往左跳过受信任代理。未配置代理信任时，检查的是连接对端地址。规则级 IP 检查只影响当前 Hook，服务级 `access-whitelist`/`access-blacklist` 影响普通 HTTP 入口。见[部署指南](Operations.md#反向代理与访问控制)。

## 失败行为

普通不匹配默认返回 HTTP 200 和 `Hook rules were not satisfied.`，建议设置 `trigger-rule-mismatch-http-response-code: 403`。缺失字段、无效签名等求值错误不一定等同于普通 false；排错时检查服务日志，不能只看 HTTP 状态。Scalr 时间检查依赖系统时钟，部署时保持时钟同步。
