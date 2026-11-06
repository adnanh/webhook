# 本机演示手册

所有命令从**仓库根目录**执行，每次只启动一个独立服务演示（组网除外）。需要 Go、curl；签名和 Admin 密钥生成还用 Python 3 / OpenSSL。以下为 POSIX shell 命令，Windows 原生对应方法见[快速开始](../docs/Quickstart.md)。

## 一键验证全部演示

```sh
make smoke
# 无 Make 的环境：python3 scripts/smoke-examples.py
```

脚本仅使用 Python 标准库，自动构建、选择空闲回环端口、生成临时密钥和配置，并实际验证八类场景（包括 Admin 登录/新增和 Hub/Agent 转发）。临时文件限制在仓库 `build/` 内，结束后停止进程并清理，不修改原始演示配置。以下各节适合逐步手动学习。

## 准备与约定

```sh
make demo
# 等价：
# go build -o build/webhook ./cmd/webhook
# go build -o build/demo-hook ./examples/demo-hook
```

`demo-hook` 为跨平台 Go 程序，只打印输入、读取指定环境变量中的演示文件路径、等待或主动返回失败。Hook 中的 `./build/demo-hook` 相对于启动工作目录；Windows 改成 `./build/demo-hook.exe`。每节启动命令占用一个终端，curl 在另一个终端运行；演示结束回到服务终端按 Ctrl+C。

以下除 HMAC 和基本鉴权示例外，多数 Hook 没有认证，只能按示例在本机回环监听。用于真实环境前增加自己的签名规则。

## 参数、环境与组合规则

```sh
./build/webhook -c examples/basic/webhook.yaml
```

```sh
curl -i 'http://127.0.0.1:9000/hooks/hello?env=dev' \
  -H 'Content-Type: application/json' \
  -H 'X-Demo-Token: local-demo-only' -H 'X-Trace: trace-001' \
  --data '{"user":{"name":"小明"}}'
# 200；{"arguments":["小明","dev","POST"],"trace":"trace-001"}
curl -i -X POST 'http://127.0.0.1:9000/hooks/hello?env=dev'
# 403：缺少 token
curl -i 'http://127.0.0.1:9000/hooks/hello?env=dev'
# 405：不允许 GET
curl http://127.0.0.1:9000/status
# status=ok、hooks.count=1
```

对应 [hooks.yaml](basic/hooks.yaml) 同时展示了 payload 点路径、url query、request method、header → 环境变量、固定参数、AND/value/regex 和自定义响应头。把 `env` 改为 `prod` 即使 token 正确也返回 403。

## HMAC 签名与模板

先在启动服务的终端设置一个随机十六进制密钥：

```sh
export DEMO_HMAC_SECRET="$(openssl rand -hex 32)"
./build/webhook -hooks examples/signature/hooks.yaml.tmpl -template -ip 127.0.0.1 -verbose
```

请求终端也设置**同一个**密钥（可通过安全的本地方式复制），然后对实际发送的文件字节计算签名：

```sh
# export DEMO_HMAC_SECRET=启动终端中的同一个值
SIGNATURE="$(python3 - <<'PY'
import hashlib,hmac,os,pathlib
body=pathlib.Path('examples/signature/payload.json').read_bytes()
print(hmac.new(os.environ['DEMO_HMAC_SECRET'].encode(),body,hashlib.sha256).hexdigest())
PY
)"
curl -i http://127.0.0.1:9000/hooks/signed \
  -H 'Content-Type: application/json' \
  -H "X-Hub-Signature-256: sha256=$SIGNATURE" \
  --data-binary @examples/signature/payload.json
# 200，输出包含 refs/heads/main
```

修改 body 而不重算签名会失败（默认签名错误为 500）；签名正确但 ref 不符合会返回 403。若修改 ref 后测试分支规则，必须重新计算修改后的 body 签名。密钥不是普通 shell 全局变量，两个终端都需导出。

## 文件上传与参数文件

```sh
./build/webhook -hooks examples/files/hooks.yaml -ip 127.0.0.1 -verbose
```

```sh
curl -i -F 'pkg=@examples/files/hello.txt;type=text/plain' http://127.0.0.1:9000/hooks/upload
# 200，内容为 你好，webhook！
curl -i -H 'Content-Type: application/json' --data '{"content":"aGVsbG8K"}' \
  http://127.0.0.1:9000/hooks/base64
# 200，内容为 hello
curl -i --data-urlencode 'metadata={"version":"1.2.3"}' \
  http://127.0.0.1:9000/hooks/nested-json
# 200，输出 arguments=["1.2.3"]
```

三者分别演示上传文件环境变量、Base64 临时文件、表单内 JSON 字符串再解析。文件在命令完成后清理。[详细映射](../docs/Referencing-Request-Values.md)

## 超时、并发、异步和失败

```sh
./build/webhook -hooks examples/execution/hooks.yaml -ip 127.0.0.1 -verbose
```

```sh
curl -i -X POST http://127.0.0.1:9000/hooks/timeout
# 约 200ms 后返回 500：命令超时
curl -sS -X POST http://127.0.0.1:9000/hooks/limited > build/first.out &
FIRST_PID=$!
sleep 0.3
curl -i -X POST http://127.0.0.1:9000/hooks/limited
# 第二次为 503；第一次约 2s 后完成
wait "$FIRST_PID"
cat build/first.out
curl -i -X POST http://127.0.0.1:9000/hooks/async
# 立即返回 202 accepted，日志约 2s 后记录命令结束
curl -i -X POST http://127.0.0.1:9000/hooks/failure
# 500，响应包含 demo failure
```

全局 `-command-timeout 30s -max-concurrency 2` 是默认值，单 Hook 覆盖；单 Hook 的 `"0"` / `0` 显式禁用对应限制。并发为每 Hook 上限，满时不排队。

## 目录热加载和状态

```sh
./build/webhook -c examples/hotreload/webhook.yaml
```

```sh
curl -X POST http://127.0.0.1:9000/hooks/ping
curl http://127.0.0.1:9000/status
# 新增一个不同 ID 的文件，避免修改原始示例：
sed 's/id: ping/id: pong/' examples/hotreload/hooks/ping.yaml > examples/hotreload/hooks/pong.yaml
curl -X POST http://127.0.0.1:9000/hooks/pong
# 等待文件监听应用后，200，参数包含 pong
rm examples/hotreload/hooks/pong.yaml
# 删除后请求 pong 返回 404
```

文件事件异步处理，写入后短暂等待或重试请求。只有直接子文件会加载。可临时新增无效 JSON 文件观察 `/status` 的 `loadErrors` 与 `degraded`，之后删除该测试文件；无效候选不会覆盖已有有效配置。显式文件自动监听需 `-hotreload`，目录监听无需设置它。完整语义见[配置文档](../docs/Configuration.md)。

## Admin 在线编辑

按 [Admin 文档](../docs/Admin.md) 生成两个随机密钥，复制 `admin/webhook.yaml.example` 到 `build/admin.yaml`，复制 basic hooks 到 `build/admin-hooks.yaml`，填好密钥后：

```sh
./build/webhook -c build/admin.yaml
```

访问 `http://127.0.0.1:9000/admin/`，使用认证器当前六位码登录。编辑 `hello` 的参数后触发请求，确认新值生效。测试只修改 `build/` 下的复制文件。开启 template 时页面仍可查看但不能保存；API 用法见 Admin 文档。

## Hub 转发到无本地端口的 Agent

演示需要两个服务终端以及一个请求终端。两个服务终端分别导出**同一个**随机节点令牌（至少 32 字符）：

```sh
export DEMO_NODE_TOKEN="$(openssl rand -hex 32)"
./build/webhook -c examples/network/hub.yaml
# Hub 监听 127.0.0.1:19000
```

Agent 终端复制同一个令牌：

```sh
# export DEMO_NODE_TOKEN=Hub终端中的同一个值
./build/webhook -c examples/network/agent.yaml
# with_webhook + webhook_only_via_hub：Agent 不监听本地 9000
```

请求终端：

```sh
curl -i 'http://127.0.0.1:19000/demo/hello?env=dev' \
  -H 'Content-Type: application/json' \
  -H 'X-Demo-Token: local-demo-only' -H 'X-Trace: via-hub' \
  --data '{"user":{"name":"内网节点"}}'
# Agent 上报清单后返回 200；trace=via-hub
```

`/demo/hello` 由 Hub 转成 Agent 的 `/hooks/hello`；Agent 仍会完整校验 Hook 规则。未上线路由可能先返回 503，稍候再试；Hub 不会自动重试已发到 Agent 的请求。`ws://` 只允许本机回环演示，生产使用 `wss://`。完整 TCP、级联、临时文件共享和节点状态见[组网指南](../docs/Network.md)。结束后分别停止两个进程并 `unset DEMO_NODE_TOKEN`。

## 清理

停止演示进程，删除自己创建的 `examples/hotreload/hooks/pong.yaml`，执行 `unset DEMO_HMAC_SECRET DEMO_NODE_TOKEN`。构建、演示 cookie 和复制后的 Admin 配置在忽略的 `build/` 中，确认不再需要后可用 `make clean` 清理。不要删除自己的业务配置。
