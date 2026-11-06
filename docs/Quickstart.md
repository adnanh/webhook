# 快速开始

以下命令均从仓库根目录运行。需要 Go 和 curl；源码最低语言版本为 Go 1.21，`go.mod` 建议工具链为 1.22.0，实际使用建议选择仍受 Go 官方支持的稳定版本。

## 构建与启动

```sh
go mod download
go build -o build/webhook ./cmd/webhook
go build -o build/demo-hook ./examples/demo-hook
# 也可以直接 make demo
./build/webhook -c examples/basic/webhook.yaml
```

服务监听 `127.0.0.1:9000`。在另一个位于仓库根目录的终端发请求：

```sh
curl -i 'http://127.0.0.1:9000/hooks/hello?env=dev' \
  -H 'Content-Type: application/json' \
  -H 'X-Demo-Token: local-demo-only' \
  -H 'X-Trace: trace-001' \
  --data '{"user":{"name":"小明"}}'
```

应返回 HTTP 200，响应体为：

```json
{"arguments":["小明","dev","POST"],"trace":"trace-001"}
```

去掉 `X-Demo-Token` 或把 `env` 改成 `prod` 应返回 403；用 GET 调用同一个地址应返回 405。示例密钥只供本机学习。

查看状态：

```sh
curl http://127.0.0.1:9000/status
```

应看到 `status: "ok"` 和 `hooks.count: 1`。按 Ctrl+C 停止服务。

## Windows PowerShell

```powershell
go build -o build/webhook.exe ./cmd/webhook
go build -o build/demo-hook.exe ./examples/demo-hook
# 所有演示 hooks 中的 execute-command 需改成 ./build/demo-hook.exe。
# 可以复制文件再改，避免覆盖原始示例：
New-Item -ItemType Directory -Force build | Out-Null
(Get-Content examples/basic/hooks.yaml -Raw).Replace('./build/demo-hook', './build/demo-hook.exe') | Set-Content -Encoding utf8 build/hooks.windows.yaml
./build/webhook.exe -hooks build/hooks.windows.yaml -ip 127.0.0.1 -port 9000 -verbose
```

另一个 PowerShell 终端：

```powershell
$headers = @{ 'X-Demo-Token' = 'local-demo-only'; 'X-Trace' = 'trace-001' }
$body = '{"user":{"name":"demo"}}'
Invoke-RestMethod 'http://127.0.0.1:9000/hooks/hello?env=dev' -Method Post -Headers $headers -ContentType 'application/json' -Body $body
```

后续演示的 POSIX shell 命令可在 Linux、macOS 或 WSL 中执行；Windows 原生使用 `.exe` 和等效 PowerShell 请求。

## 接入自己的命令

Hook 文件是数组，最小配置如下：

```yaml
- id: notify
  execute-command: /usr/local/bin/notify.sh
  http-methods: [POST]
  response-message: accepted
```

通过 `./build/webhook -hooks /absolute/path/hooks.yaml -verbose` 启动。默认 Hook URL 是 `/hooks/notify`，默认执行方式为异步。没有 `trigger-rule` 时，任何能访问该地址的人都能触发命令；对外部署先配置 [签名规则](Hook-Rules.md)。命令参数通过 `pass-arguments-to-command` 传入，不把完整命令行写入 `execute-command`。

继续阅读 [总配置与加载](Configuration.md) 和 [可运行演示](../examples/README.md)。
