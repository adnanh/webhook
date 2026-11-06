# 请求取值与文件传递

参数描述结构如下，适用于参数、环境变量、临时文件、规则中的 `parameter` 等位置：

```yaml
source: payload
name: user.name
envname: APP_USER       # 仅环境变量/临时文件使用
base64decode: false    # 仅 pass-file-to-command 使用
```

## 所有来源

| `source` | `name` 示例 | 取值 |
| --- | --- | --- |
| `header` | `X-Trace` | 第一个同名 HTTP 头；名称按 MIME 规范化 |
| `url` 或 `query` | `env` | 第一个同名查询参数 |
| `payload` | `user.name` | JSON、XML 或表单解析后的字段 |
| `request` | `method` | HTTP 方法 |
| `request` | `remote-addr` | 解析后的真实来源地址；代理配置影响该值 |
| `string` | `固定字符串` | 原样字符串，无需在请求中存在 |
| `raw-request-body` | 不需要 | 原始请求体字符串 |
| `entire-payload` | 不需要 | 解析后的 payload 整体序列化为 JSON |
| `entire-query` | 不需要 | 查询参数整体 JSON |
| `entire-headers` | 不需要 | 请求头整体 JSON |

`entire-payload` 不是原始字节；签名验证应基于原始请求体。Query、表单及 Header 同名多值通常只保留第一项。

JSON 嵌套字段使用点路径，如 `commits.0.id`；顶层 JSON 数组包装在 `root` 下，通过 `root.0.id` 取值。如果对象里存在与完整点路径同名的直接键，直接键优先。对象/数组作为参数时会序列化成 JSON，数值/布尔值转换为字符串。

JSON 需要正确的 `Content-Type: application/json`；URL 编码表单为 `application/x-www-form-urlencoded`。XML 按 `application/xml` 等包含 xml 的 Content-Type 处理，例如 `<app><user id="1">小明</user></app>` 的属性为 `app.user.-id`，文本为 `app.user.#text`，重复元素通常形成数组。

## 参数和环境变量

```yaml
pass-arguments-to-command:
  - {source: string, name: echo}
  - {source: payload, name: user.name}
pass-environment-to-command:
  - {source: header, name: X-Trace, envname: DEMO_TRACE}
```

未设置 `envname` 时，环境变量默认是 `HOOK_` 加原始 `name`，不会自动替换点号或连字符；建议显式指定适合 shell 的变量名。子进程继承服务本身环境，并追加映射变量。不要把用户输入拼进 `sh -c`；使用独立 argv 和脚本内引用。

## 再解析 JSON 字符串

```yaml
parse-parameters-as-json:
  - {source: payload, name: metadata}
pass-arguments-to-command:
  - {source: payload, name: metadata.version}
```

对应表单可传 `metadata={"version":"1.2.3"}`。multipart 文件字段如果标记 `application/json`，或列入 `parse-parameters-as-json`，会作为 JSON 对象解析到 payload 的对应字段。

## 把参数写成临时文件

```yaml
pass-file-to-command:
  - source: payload
    name: content
    envname: DEMO_FILE
    base64decode: true
```

请求 `{"content":"aGVsbG8K"}` 解码后形成内容为 `hello\n` 的文件，命令通过 `DEMO_FILE` 环境变量获取路径。没有启用 Base64 时写入提取出的字符串；对象/数组会先序列化为 JSON。未指定 `envname` 时默认 `HOOK_` 加大写 `name`。

设置 `command-working-directory` 时临时文件创建在该目录，否则使用系统临时目录。文件在**命令执行结束**后删除，不能在命令退出后交给另一个后台任务继续读取。

## multipart 上传

启用 `keep-file-environment: true`，字段 `pkg` 会提供：

```text
HOOK_FILE_PKG=临时文件路径
HOOK_FILENAME_PKG=原始文件名
```

每个文件字段目前只处理第一个文件。字段名会大写后直接拼接到变量名，建议只用字母、数字、下划线。临时文件在同步/异步命令完成后清理。原始文件名是用户输入，不应用来直接拼接服务端输出路径。

`max-multipart-mem` 是内存解析阈值，超过后可落盘，**不是总上传大小上限**；总量由 `max-body-size` 控制，默认 10 MiB。可运行例子见 [files 演示](../examples/README.md#文件上传与参数文件)。
