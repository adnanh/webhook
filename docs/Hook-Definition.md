# Hook 字段参考

Hook 配置的顶层必须是数组；JSON 和 YAML 描述相同结构。通常每项至少配置 `id` 和 `execute-command`，ID 应在全部已加载文件中唯一。基础示例见 [examples/basic/hooks.yaml](../examples/basic/hooks.yaml)。

## 标识和执行

| 字段 | 类型 / 默认 | 说明 |
| --- | --- | --- |
| `id` | string | URL 中的 Hook ID，如 `/hooks/deploy` |
| `execute-command` | string | 可执行文件路径或 PATH 中的命令名，不包含参数 |
| `command-working-directory` | string / 当前进程目录 | 子进程工作目录，也影响相对命令路径和参数临时文件位置 |
| `command-timeout` | string / 继承全局 | Go duration，如 `30s`、`2m`；`"0"` 禁用继承的超时；不能为负 |
| `max-concurrency` | integer / 继承全局 | **每个 Hook ID** 的同时执行上限；`0` 禁用；不能为负 |
| `pass-arguments-to-command` | array | 按数组顺序传入命令参数，每项见[取值参考](Referencing-Request-Values.md) |
| `pass-environment-to-command` | array | 传入环境变量，可通过 `envname` 指定名字 |
| `pass-file-to-command` | array | 把取出的值写入临时文件，路径经环境变量传入；支持 `base64decode` |
| `parse-parameters-as-json` | array | 把 JSON 字符串参数解码后再次用于规则或取值；通常是 payload/header/url 中的字段 |
| `keep-file-environment` | bool / false | 把 multipart 上传暴露为 `HOOK_FILE_<大写字段名>` 与 `HOOK_FILENAME_<大写字段名>` |

服务通过 `exec.Command` 执行命令，不自动启动 shell，不解释管道、重定向或变量替换。使用脚本时需要正确 shebang 和执行权限，或者显式指定解释器及脚本参数。缺失参数的提取错误可能只记录日志并继续执行；脚本仍须自行检查必需参数。

## HTTP 输入、规则与响应

| 字段 | 类型 / 默认 | 说明 |
| --- | --- | --- |
| `http-methods` | string array / 继承全局 | 如 `[POST]`；Hook 非空设置优先于全局；双方均未设时允许所有方法 |
| `incoming-payload-content-type` | string / 请求 Content-Type | 强制按该类型解析；适用于上游 Content-Type 错误时 |
| `trigger-rule` | object / 无 | [组合触发规则](Hook-Rules.md)；缺省即无规则限制 |
| `trigger-signature-soft-failures` | bool / false | 将签名错误作为普通不匹配，允许 OR 中继续尝试其他签名规则 |
| `trigger-rule-mismatch-http-response-code` | integer / 200 | 规则不匹配的状态码；建议外部接口显式设为 403 |
| `response-message` | string / 空 | 异步执行被接受后立即返回的文本 |
| `response-headers` | array | `[{name: Content-Type, value: application/json}]`；覆盖同名全局响应头 |
| `success-http-response-code` | integer / 200 | 接受异步任务或同步执行成功时返回；常用异步 202；不认识的状态码回退 200 |
| `include-command-output-in-response` | bool / false | 同步等待命令，响应包含 stdout 与 stderr 的合并结果 |
| `include-command-output-in-response-on-error` | bool / false | 同步执行出错时也返回已捕获输出；需要同时开启上一项 |

## 同步、异步和错误

默认**异步**：先获取并发名额，启动后台任务，立即响应 `response-message`。HTTP 成功只代表已接受任务；后续命令失败只能从日志判断。它不是持久化任务队列，进程退出后不保证继续执行，也不自动重试。

同步模式等待命令结束，成功默认 200，失败或超时通常 500。同步请求取消时会取消其命令上下文；异步命令使用独立上下文，可通过 `command-timeout` 限时。超时通过 Go 的 `exec.CommandContext` 终止直接子进程，并不保证终止脚本创建的整棵进程树。

并发限制在同步/异步两种模式都生效，达到上限立即返回 **503**，不排队。限制按 Hook ID 计算，不是整个服务统一总量。运行中改变上限会创建新的限制器，不能视为跨配置变化的严格全局调度。

| 情况 | 通常状态 / 行为 |
| --- | --- |
| Hook 不存在 | 404 |
| HTTP 方法不允许 | 405 |
| 服务级 IP 拒绝 | 403 |
| 请求体超过上限 | 413 |
| 普通规则不匹配 | 默认 200，可配置 403；响应 `Hook rules were not satisfied.` |
| 签名校验或规则求值发生错误 | 默认 500；软失败模式可转为普通不匹配 |
| 同步命令失败/超时 | 500 |
| 并发已满 | 503 |
| 异步接受成功 | 默认 200，可配置 202 |

执行限制演示见 [examples/execution/hooks.yaml](../examples/execution/hooks.yaml)。
