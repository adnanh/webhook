# 内网服务组网

[返回中文文档导航](README.md) · [从零运行本机 Hub/Agent 演示](../examples/README.md#hub-转发到无本地端口的-agent)

推荐部署是：公网 Hub 接收指定路径的 HTTP 请求，内网 Agent 主动通过 `wss://` 连接 Hub，并在进程内执行 webhook。Agent 设置 `with_webhook: true`、`webhook_only_via_hub: true`，无需监听本地 webhook 端口。Hub 只允许配置在 `hook_routes` 中的入口，并按节点令牌、hook 清单和许可规则路由。

初次部署使用 [Hub 总配置](examples/network/hub-integrated.yaml)和 [Agent 总配置](examples/network/agent-integrated.yaml)。只需要 webhook 时，无需 `forwards`、SSH 服务或级联。每个 Agent 使用独立令牌；外部调用者仍须通过 hook 自身的签名或其他触发规则认证，Agent 的连接令牌不负责认证外部调用者。Hub 是受信任的流量中转方，能读取请求内容。

通用 TCP 转发与级联作为可选功能保留，适用于已有运维场景。当前不增加 Hub TCP 端口发布、通用 HTTP 正向代理、动态端口分配或正则 rewrite，以控制维护成本。这项功能不创建虚拟网卡或虚拟 IP，也不转发 UDP。

## 统一配置

推荐把 webhook 启动参数和组网设置写在同一份 `-c` YAML 总配置中。组网字段放在 `network:` 下；Agent 还可在同一文件中配置 `hooks`、日志等参数。这样每台机器只需一份配置文件，令牌可直接填写、通过环境变量注入或独立文件读取。示例：[Hub 总配置](examples/network/hub-integrated.yaml)、[Agent 总配置](examples/network/agent-integrated.yaml)。两台机器上的 `app-a.token` 内容必须相同，且只分配给这个节点。启动命令分别为：

```sh
webhook -c hub-integrated.yaml
webhook -c agent-integrated.yaml
```

若想保持组网配置独立，仍可用 `webhook -network-config agent.yaml`，或在总配置中设置 `network-config: agent.yaml`。总配置中的相对 `network-config` 路径以该总配置所在目录为基准；命令行上的相对路径以当前工作目录为基准。`network:` 中的相对 `token_file`、`ca_file` 路径以总配置所在目录为基准；独立组网配置中的对应路径则以独立文件所在目录为基准。`hooks` 路径仍遵循原有规则，建议在服务配置中使用绝对路径。

同一份总配置不能同时设置 `network:` 和 `network-config:`。若命令行指定 `-network-config`，它会覆盖总配置里的 `network:`。Hub 有多个节点时应为每个节点配置不同的令牌，避免共享身份。每个节点只能配置 `token`、`token_env`、`token_file` 之一，至少 32 个字符且不能含空白。推荐使用 `openssl rand -hex 32` 生成高熵令牌。直接写 `token` 时整份 YAML 都属于密钥材料，不应提交到 Git。

## Hub、Agent 与 webhook 共存

`mode: hub` 在原有 webhook HTTP 服务上添加 Hub 端点，可以同时运行本机 hooks；没有配置 hooks 时也能单独运行 Hub。`mode: agent` 默认只运行 Agent。设置 `agent.with_webhook: true` 后，同一进程也会启动 webhook HTTP 服务，并自动把已加载的 hook 清单上报 Hub。`mode: both` 让一个进程同时作为下级节点的 Hub 和上级 Hub 的 Agent；也可同时设置 `with_webhook: true` 运行本机 hooks。

若只允许从 Hub 进入 hooks，设置 `with_webhook: true` 和 `webhook_only_via_hub: true`。Agent 在进程内把 HTTP 请求交给 webhook 处理，webhook 不监听本地 TCP 端口或 Unix socket；另行配置的 `forwards` 仍会按其设置监听。[Agent A](examples/network/agent-hook-a.yaml)、[Agent B](examples/network/agent-hook-b.yaml)及级联子节点都使用此模式。使用总配置时只需 `webhook -c agent-integrated.yaml`；使用独立组网配置时运行 `webhook -network-config agent-hook-a.yaml -hooks hooks.yaml`。`-ip` 和 `-port` 不用于监听。此模式仅适用于 `mode: agent`，因为 `mode: both` 还须向下级 Agent 提供 Hub 入口。

需要本机也能调用 hooks 时，可以不设置 `webhook_only_via_hub`，并用 `-ip 127.0.0.1` 将 HTTP 服务限制在回环地址。此时 Agent 默认仍通过进程内入口调用同进程 webhook，因此本地监听器可以独立使用 HTTP、HTTPS 或 Unix socket。单独运行 webhook 与 Agent 时，也可以由 Agent 的 `services[].target` 指向已有的 webhook TCP 服务。

## 可选：三节点 TCP 转发示例

示例文件位于 [docs/examples/network/](examples/network/)：

- `hub.yaml`：公网 Hub，允许 `ops` 访问 `web` 的 webhook 与 SSH 服务。
- `agent-web.yaml`：内网 `web` 节点，把已有 webhook HTTP 服务 `127.0.0.1:9000` 和可选的 OpenSSH 服务 `127.0.0.1:22` 发布给 Hub。`9000` 是 webhook 默认端口；`22` 是 SSH 的常见端口，组网本身不要求启用 SSH。
- `agent-ops.yaml`：内网 `ops` 节点开启两个仅本机可访问的入口：`127.0.0.1:19000` 转到 `web` 的 `9000`，适合运维机访问内网 webhook；`127.0.0.1:10022` 转到 `web` 的 `22`，适合从运维机 SSH 登录。入口端口可换成其他空闲端口，不需要对公网开放。

在 Hub 上为每个节点生成不同的随机令牌，分别保存到 `hub.yaml` 所在目录下的 `tokens/web.token`、`tokens/ops.token`。把对应令牌安全地复制到各 Agent 的配置目录，文件名分别为 `web.token`、`ops.token`。令牌文件应只有运行用户可读；例如：

```sh
mkdir -p tokens
umask 077
openssl rand -hex 32 > tokens/web.token
openssl rand -hex 32 > tokens/ops.token
```

令牌路径相对于各自 YAML 文件所在目录。不要把生成的令牌提交到 Git。

Hub 使用现有 HTTP 服务监听器。若由 webhook 直接终止 HTTPS，需为 Hub 的公网域名准备可信证书和私钥，再启动：

```sh
webhook -network-config hub.yaml -ip 0.0.0.0 -port 443 \
  -secure -cert fullchain.pem -key privkey.pem
```

Hub 可以只运行组网功能，不需要 hook 文件。若 Hub 已运行 webhook，可以在现有 YAML 总配置中添加内嵌 `network:`，或用 `network-config: /path/to/hub.yaml` 引用独立文件，然后正常启动。组网端点固定为 `/__webhook_network/v1/control` 和 `/__webhook_network/v1/data`。

证书可由现有 Nginx/托管服务管理，或通过 [Let's Encrypt 的 ACME 客户端](https://letsencrypt.org/getting-started/)申请。若 Hub 直接提供 HTTPS，可按系统对应的 [Certbot 官方指南](https://certbot.eff.org/instructions?ws=other)安装，并在公网域名指向 Hub、80 端口可达时运行 `sudo certbot certonly --standalone -d hub.example.com`；不能开放 80 时可选择 DNS 验证。将生成的 `fullchain.pem` 和 `privkey.pem` 路径传给 `-cert`、`-key`；确保运行用户有读取权限，并在证书更新后重启 webhook 以加载新证书。已有 Nginx 时可按 [Nginx 的 Certbot 指南](https://certbot.eff.org/instructions?ws=nginx)运行 `sudo certbot --nginx -d hub.example.com` 并验证自动续期。若由 Nginx 终止 HTTPS，证书只配置在 Nginx，Hub 进程监听 `127.0.0.1:9000` 的普通 HTTP 即可；Agent 仍使用 `wss://hub.example.com`。完整反代配置见下文。

在 `web` 节点分别运行原有 webhook 服务（监听 `127.0.0.1:9000`）及 Agent；在 `ops` 节点运行另一个 Agent：

```sh
# web 节点，以下两条命令分别在独立终端或服务中运行
webhook -hooks hooks.yaml -ip 127.0.0.1 -port 9000
webhook -network-config agent-web.yaml
# ops 节点
webhook -network-config agent-ops.yaml
```

上述 `agent-web.yaml`、`agent-ops.yaml` 的 Agent 模式只运行组网，本身不加载 hooks，也不监听 `9000`；此处 `9000` 属于另一个 webhook 进程。连接建立后，在 `ops` 节点访问：

```sh
curl http://127.0.0.1:19000/hooks/your-hook-id
ssh -p 10022 user@127.0.0.1
```

Agent 重连时会退避重试；已建立的 TCP 会话在连接中断时会断开，应用应自行重试。

这里的 `forwards` 是通用 TCP 隧道，不会解析 HTTP，也不会自动附加真实来源 IP。若 webhook 的 IP 许可规则需要外部调用者的地址，应使用下一节的 Hub `hook_routes` 作为 HTTP 入口；`ops` 这种本机 TCP 入口适用于可信运维机直接访问内网服务。

## Hub 分配公开 Hook 路径

参见 [Hub 路由示例](examples/network/hub-hooks.yaml)以及 [Agent A](examples/network/agent-hook-a.yaml)、[Agent B](examples/network/agent-hook-b.yaml)。Hub 的 `hook_routes` 明确指定公开 `path`、本地 hook 的 `hook` ID 和允许承接请求的 `agents`。例如两个 Agent 都上报 `deploy` 时，`/deploy` 的每个请求会从在线且满足预过滤条件的 Agent 中随机选一个。未列入该路由的 Agent 即使有同名 hook，也不会获得该入口。Agent 的本地 hook 路径与公开路径可以不同；默认本地前缀为 `/hooks`，需要调整时在 Hub 的 `nodes[].hook_prefix` 中设置。

公开路径本身就是一次明确的路由转换。例如 Hub 配置：

```yaml
hook_routes:
  - path: /deploy
    hook: internal-deploy
    agents: [app-a, app-b]
```

外部 `/deploy` 会转发到选中 Agent 的 `/hooks/internal-deploy`。因此目前无需额外的通配符、正则或任意 URL rewrite 配置；这类配置会增加路径歧义和绕过路由权限检查的风险。路径隐藏只减少内部 hook ID 的暴露，不能代替令牌、IP 规则或 hook 自身的鉴权。

Agent 只上报 hook ID、允许的 HTTP 方法，以及可安全提前判断的 IP 规则；不会上报命令、签名密钥或请求匹配参数。Hub 还可在每条公开路由配置 `allow_ips` 或 `deny_ips`。Agent 使用热重载更新 hooks 后，约 2 秒内更新 Hub 清单；修改 Hub 的 `hook_routes` 则需要重启 Hub。Hub 只做预过滤，最终仍由 Agent 执行完整 hook 规则。对包含 OR/NOT 的 IP 条件，Hub 不会擅自提前拒绝请求。随机路由按请求选择节点，不保证会话粘性；请求已经发往节点后，不会自动重试另一个节点，以免重复执行 hook。

Hub 负责确定真实来源 IP：直接连接时使用对端地址；处于可信 Nginx 后方时，使用配置的 `real-ip-header`，且只信任 `trusted-proxies` 中的代理地址。推荐由 Nginx **覆盖** `X-Real-IP` 为 `$remote_addr`，并在 Hub 配置 `-real-ip-header X-Real-IP -trusted-proxies 127.0.0.1`。若使用 `X-Forwarded-For`，Hub 会从右向左跳过可信代理，选择第一个不可信地址，防止客户端预置列表开头的假 IP。

Hub 会先删除请求里已有的 `X-Webhook-Network-Client-IP`，再写入自己验证过的来源 IP。级联的 `relay_to` 是经过认证的 TCP 隧道，上一层 Hub 写入的来源 IP 会原样经过中间节点，到达下级 Agent；中间节点不会用自己的 IP 覆盖它。`webhook_only_via_hub: true` 时，Agent 进程内的 webhook 只信任此内部入口；默认 `with_webhook: true` 的进程内入口也使用 Hub 传入的来源 IP，不受本地反代头配置影响；显式 TCP 目标则依赖该服务的可信代理配置。若 Agent 与 webhook 分开运行，需要为 webhook 配置 `-real-ip-header X-Webhook-Network-Client-IP -trusted-proxies 127.0.0.1,::1`，并确保该 TCP 入口只供本机 Agent 访问。hook 的 IP 规则、全局 IP 黑白名单及 `request.remote-addr` 均使用解析后的来源 IP。

转发时会保留 HTTP 方法、业务请求头、`Host`、查询参数及请求体；公开路径会转换成 Agent 的本地 hook 路径。Hub 会覆盖 `X-Forwarded-For`、`X-Real-IP`、`X-Forwarded-Proto`、`X-Forwarded-Host`，并写入 `X-Webhook-Network-Original-URI` 记录公开路径及查询参数；客户端伪造的同名信息不会透传。`Forwarded` 和部分其它可能冒充代理的头会被删除。响应状态、业务头、压缩编码、主体及 trailer 会正常转回。HTTP 请求会在每跳重新编码，因此原始 TCP 地址、TLS 连接对象、报文分块方式和原始公开路径不会自动成为下级 `RawRequest` 的值；依赖完整原始 HTTP 字节签名的业务需要自行使用专门的端到端方案。

## 级联

[上级 Hub](examples/network/cascade-parent.yaml)、[中间节点](examples/network/cascade-middle.yaml)和[下级 Agent](examples/network/cascade-child.yaml)组成完整级联示例。中间节点使用 `mode: both`：它为 `child` 提供下级 Hub，同时以 `middle` 身份连到上级 Hub。下级 Agent 必须能访问中间节点的 Hub 地址（同一局域网或可达的公网地址）。`relay_to: child/webhook` 将下级服务作为上级 Agent 的服务发布；下级 hook 清单会随连接与热重载变化转报给上级。上级 Hub 仍由自己的 `hook_routes` 分配公开路径。普通 TCP 服务也可用同样方式级联。最多转发 4 跳，以限制错误配置形成环路。每一跳都经过对应 Hub，链路上的节点权限与令牌需要分别配置；上级 Hub 看到的是中间节点身份。

## 配置与权限

保留服务名 `webhook` 只能通过 Hub 的 hook 路由或受信任的 Hub 级联进入，不能写入其他节点的 `allow`。否则其他 Agent 可通过原始 TCP 请求绕过路由规则并伪造内部来源头，启动时会拒绝此配置。可选 TCP 示例使用服务名 `http`；它不应指向信任内部来源头的 webhook 监听器，也不能通过别名重新发布受保护的 webhook。

Hub 的 `nodes[].services` 声明该节点允许发布的服务名，`nodes[].allow` 是该节点允许访问的 `节点/服务` 列表。Agent 的 `services[].target` 指向本机或可信内网中的实际 TCP 服务，`forwards[].listen` 是本机入口，`forwards[].to` 是目标。Hub 同时限制活跃会话数，默认 `max_sessions: 128`；每个 Agent 默认 `max_connections: 64`。两者均可按容量调整。

Agent 的转发监听地址默认必须是 loopback。只有明确需要让其他设备访问该转发端口时，才设置 `allow_public_listen: true` 并在 Agent 所在机器上配置防火墙。Hub 会按节点令牌和访问列表校验每条连接。令牌至少 32 个字符，更换令牌后重启对应节点与 Hub。

Hub 与 Agent 共用每个节点独立的令牌，双向验证：Agent 为每条连接生成随机 nonce，并提交绑定节点、端点、角色、目标和会话的 HMAC；Hub 验证后，在 WebSocket 握手中返回另一种 HMAC，Agent 验证成功后才上报 hook 清单或转发数据。令牌原文不会在握手中发送。连接仍必须使用 TLS 校验证书和主机名，防止中间人转发或篡改数据；应用层证明不能代替 TLS。更换令牌后重启 Hub 和对应 Agent。

Agent 的 `server` 必须是 `wss://` 地址；仅本机测试允许 `ws://127.0.0.1` 或 `ws://localhost`。使用私有 CA 时，可在 Agent 配置中设置 `ca_file`，不要关闭 TLS 证书验证。若在 Hub 前使用 Nginx 终止 TLS，先让 Hub 只监听本机 HTTP：

```sh
webhook -network-config hub.yaml -ip 127.0.0.1 -port 9000 \
  -real-ip-header X-Real-IP -trusted-proxies 127.0.0.1
```

Nginx 的 HTTPS 站点同时代理组网 WebSocket 端点和公开 hook 路由。下面的证书路径以 Certbot 管理的 `hub.example.com` 为例；`map` 放在 Nginx 的 `http` 块中，`server` 放在对应站点配置中：

```nginx
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

server {
    listen 443 ssl;
    server_name hub.example.com;
    ssl_certificate /etc/letsencrypt/live/hub.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/hub.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:9000;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $remote_addr;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_read_timeout 75s;
        proxy_send_timeout 75s;
    }
}
```

WebSocket 的 `Upgrade` 和 `Connection` 头需要显式转发，参见 [Nginx 官方说明](https://nginx.org/en/docs/http/websocket.html)。后端 `9000` 只应由本机 Nginx 访问；不要在 Nginx 之外公开该端口。

`-access-whitelist` 与 `-access-blacklist` 继续保护 webhook 路由，包括 Hub 分配的公开 Hook 路由；组网控制和数据端点仅接受通过独立令牌认证的 Agent。Hub 能看到转发的明文流量；需要端到端保密的服务应继续使用自己的 TLS 或 SSH。示例中的 SSH 端口只在 `ops` 节点的 loopback 上开放。

## 超时、容量与维护

Hub 的 `request_timeout` 默认 `60s`，覆盖请求体读取、隧道连接、等待响应和响应传输，可在 `network.hub` 中调整，例如 `request_timeout: 120s`。未收到响应头时超时返回 504；响应已经开始时会中断连接，避免将截断的数据当作完整响应。超时释放转发会话，但不保证撤销已经执行的命令；调用方重试应使用业务幂等键。Hub 不自动重试已发送的 webhook。

每个 HTTP 请求使用独立的数据通道，默认 Hub 最多 128 个会话、Agent 最多 64 个连接。流量按 32 KiB 帧转发，不缓冲完整请求或响应。控制连接长期保持，Agent 稳定连接一分钟后会重置重连退避。此实现适合低至中等频率的 webhook；每次请求仍有数据通道握手成本，尚未进行生产容量压测，不宣称能承载特定 QPS。出现实际吞吐瓶颈后再考虑连接复用。

HTTP 转发清理 `Connection` 指定的头和其他逐跳头，保留业务头、主体、压缩编码及 trailer。建议在 Nginx 设置符合业务的请求速率和请求体限制；在 Hub/Agent 配置合理的会话上限、请求体限制和命令执行超时。

修改组网配置、路由或令牌后需要重启对应进程，hook 定义可按原有热重载方式更新。节点上线/断线、目标连接失败有日志；目前没有独立的组网管理界面、配置在线下发或跨请求投递保证。排查顺序为：TLS/节点认证 → Agent 上报的服务与 hook → Hub 路由和 IP 规则 → Agent 最终触发规则。普通 `/status` 不是组网健康检查，部署检查应实际调用一个无副作用的测试 hook。

## 三种令牌来源与认证边界

Hub 的每个 `nodes[]` 以及 Agent 都支持以下任意一种配置，不能混用，没有隐含优先级：

```yaml
# token: "完整的随机令牌"
# token_env: WEBHOOK_NODE_TOKEN
# token_file: secrets/node.token
```

`token_env` 是环境变量名称，变量值在启动时读取；空值、未设置或过短均报错。令牌文件路径相对于所属配置文件。令牌轮换后需要重启两端。命令行临时 Agent 支持 `--token`、`--token-env` 或 `--token-file`（三选一）。`--token` 会出现在进程参数和可能的 shell 历史中，日常部署推荐环境变量或文件。

安全设计参考 [frp TLS 文档](https://gofrp.org/en/docs/features/common/network/network-tls/)关于验证对端身份的要求和[访问者独立密钥](https://gofrp.org/en/docs/reference/visitor/)的授权用途。本实现保留 TLS 1.2 及以上、可信 CA/主机名验证、每节点独立令牌、每条控制/数据连接的双向 HMAC 校验。没有跳过证书校验的选项。私有 CA 使用 `ca_file`；Nginx 终止 TLS 时，Nginx 及其到 Hub 的链路均属于可信边界，建议仅在同机回环地址转发。

握手 nonce 带时间戳和 192 位随机数，签名绑定节点、端点、角色、目标及会话；Hub 拒绝超出正负 60 秒的时间戳和重复 nonce。已认证 nonce 使用有界缓存，达到 65536 条且尚未过期时拒绝新握手；缓存保留至该时间戳有效期结束（最多约 121 秒）。两端应同步系统时钟，升级此认证版本需要同时更新 Hub 和 Agent。抓到握手不能直接恢复随机令牌，但弱口令仍可能被离线猜测，因此不能用人工短密码。证书验证发生在发送 HTTP 认证头之前；未经信任的假 Hub 无法获得该证明。HMAC 不能替代 TLS，也不提供对 Hub 的端到端保密。

暂不增加两个 Agent 之间的二次子密码：当前每个节点独立准入，Hub 的 `allow` 明确授权谁可访问哪个普通服务，webhook 则由 `hook_routes` 与 hook 自身认证控制。这里信任 Hub；若业务要求 Hub 也不能读写数据，应在目标服务使用 SSH/HTTPS 等端到端协议。增加一个由 Hub 校验的子密码无法满足这一要求。

## 大文件上传（超过 200 MB）

默认 `max-body-size` 仍为 10 MiB。需要大包时必须在 **Hub 与 Agent 两份主配置**中提高限制，例如允许最大 512 MiB：

```yaml
max-body-size: 536870912
max-multipart-mem: 1048576
```

Agent 示例见 [agent-upload.yaml](examples/network/agent-upload.yaml)。Hub 另设 `network.hub.request_timeout: 15m`，给上传和同步执行留出时间；该值是整个转发请求的总时限。Nginx 同时设置：

```nginx
client_max_body_size 512m;
proxy_request_buffering off;
proxy_read_timeout 900s;
proxy_send_timeout 900s;
client_body_timeout 900s;
```

文件使用 `multipart/form-data`，例如 `curl -F pkg=@app.war https://hub.example.com/deploy`。hook 配置 `keep-file-environment: true` 后，命令通过 `HOOK_FILE_PKG` 读取临时文件，通过 `HOOK_FILENAME_PKG` 读取上传文件名；命令应把需要长期保存的文件移入业务目录。不要将原始文件名直接拼接成目标路径。文件只在命令执行期间有效，异步命令也会保留到执行结束，随后清理。拒绝触发、解析失败和命令启动失败也会清理临时文件。

Hub 只按帧转发；Agent 的 multipart 文件超过内存阈值后写入临时磁盘，不必将完整 WAR 包读入内存。请给临时目录准备足够磁盘空间并限制并发上传。原始二进制 body、JSON/base64 仍使用现有内存解析方式，不适合大包；不要将 WAR 文件声明为 JSON part，也不要对大文件开启 `debug`，因为调试器会读取并记录完整报文。`max-body-size: 0` 关闭限制，通常更适合明确设置有限上限。

## 临时 Agent 子命令

临时用途无需创建 Agent YAML，默认运行一小时，可用 `--duration 30m` 调整，Ctrl+C 即退出。它不会自动获得 Hub 授权或随机公开端口：管理员须预先为临时节点配置独立身份和服务白名单，使用与正式节点不同的 ID，避免挤掉已有连接。

```sh
# 令牌由部署环境注入 WEBHOOK_TEMP_TOKEN，或改用 --token-file temp.token。
# 发布多个只读文件/目录；服务名为 files，不创建本地 HTTP 监听端口。
webhook agent --server wss://hub.example.com --id temp \
  --token-env WEBHOOK_TEMP_TOKEN --duration 30m \
  --share war=/srv/releases/app.war --share dist=/srv/releases/public

# 同时可发布 TCP 服务；不指定 --share 时也能单独使用。
webhook agent --server wss://hub.example.com --id temp \
  --token-env WEBHOOK_TEMP_TOKEN --publish ssh=127.0.0.1:22

# 另一个有权访问的节点建立本机入口。
webhook agent --server wss://hub.example.com --id ops \
  --token-env WEBHOOK_OPS_TOKEN \
  --forward 127.0.0.1:19000=temp/files
curl http://127.0.0.1:19000/war
curl http://127.0.0.1:19000/dist/another.war
```

Hub 配置示例见 [hub-temporary.yaml](examples/network/hub-temporary.yaml)。Hub 为 `temp` 配置 `services: [files, ssh]`（只保留实际需要的服务），为 `ops` 配置 `allow: [temp/files]`；需要 SSH 时再添加 `temp/ssh` 并建立对应 `--forward`。每个节点的令牌不同，只有双方各自使用的令牌须与 Hub 相应条目一致。临时命令不读取主配置，可用 `webhook agent --help` 查看参数。

文件入口仅支持 GET/HEAD、Range 下载；没有目录列表、上传或删除接口，目录下的符号链接不可访问。显式选择的根路径在启动时解析；请只共享由可信本地用户维护的目录，不能把它当作对抗恶意本地文件替换的文件系统沙箱。终端 URL 名称不能重复，目录文件按该 URL 名称挂载。文件和普通 TCP 服务只通过已有授权隧道访问，不增加公开 HTTP 路由或 Agent 之间的密码协商协议。

## 通过 CLI 临时加入 Hub

最少只需指定 Hub 地址、节点 ID 和令牌来源，无需 Agent YAML。以下命令仅建立认证和心跳连接，不发布服务、不监听本地端口，适用于检查连通性和临时在线状态：

```sh
webhook agent --server wss://hub.example.com --id temp \
  --token-env WEBHOOK_TEMP_TOKEN --duration 30m
```

如确需全部通过参数指定，可将 `--token-env WEBHOOK_TEMP_TOKEN` 换为 `--token '完整随机令牌'`。需要文件共享时追加 `--share war=/srv/releases/app.war`；需要发布端口服务时追加 `--publish ssh=127.0.0.1:22`。CLI 参数均见 `webhook agent --help`。Hub 的 `nodes` 中仍须预先配置这个 ID、匹配令牌和允许发布的服务；仅加入但不发布服务时可不配置 `services`。未知 ID 不会因为提供任意令牌而自动获准加入。

## Hub 节点管理 API

启用现有 Admin（`admin: true`，并配置 `admin-totp-secret`、`admin-jwt-secret`）后，Hub 提供以下只读接口；关闭 Admin 时不注册接口。节点令牌不能用于调用管理 API，必须先进行 Admin TOTP 登录并使用其会话 cookie 或 Bearer JWT。

| 方法与路径 | 内容 |
| --- | --- |
| `GET /admin/api/network/nodes` | Hub 版本、当前/最大会话数，以及全部配置节点 |
| `GET /admin/api/network/nodes/{id}` | 指定节点；未知 ID 返回 404 |

`/admin` 随 `admin-path` 改变。示例：

```sh
# 使用 Admin TOTP 登录并保存会话 cookie；替换当前六位验证码。
curl -c admin.cookies -H 'Content-Type: application/json' \
  -d '{"code":"123456"}' https://hub.example.com/admin/api/auth/login
curl -b admin.cookies https://hub.example.com/admin/api/network/nodes
curl -b admin.cookies https://hub.example.com/admin/api/network/nodes/temp
```

响应节点字段包含 `id`、`status`（`online`、`offline`、`never_connected`）、`version`、`os`、`arch`、`temporary`、`expires_at`、连接/断线/最近活动时间、当前发布的 `services`、`hook_ids` 和 `active_sessions`。列表另包含 `generated_at`、`hub_version`、`active_sessions`、`max_sessions`。没有发布服务或 hook 时返回空数组。离线节点保留最后上报的版本与连接时间，当前服务清单为空；状态历史仅存内存，Hub 重启后重新开始记录。

在线状态由 Hub 的控制连接判断，静默断线最多约 65 秒后被发现，`last_seen_at` 由有效控制消息或心跳 pong 更新，不代表业务 hook 健康。版本、系统和临时有效期是认证节点自报的信息，不是可信的软件证明；旧节点未上报版本时显示 `unknown`。临时期限由 CLI 自行退出执行，Hub 不把自报期限当作授权策略。`peer_address` 是 Hub 直接看到的 TCP 对端，经过 Nginx 时可能是代理地址。级联只展示直连节点，不递归展示下层拓扑。

接口不返回令牌、令牌环境变量名、令牌文件路径或本地服务目标地址。管理信息带 `Cache-Control: no-store`；已有公网 `/status` 不增加节点管理信息。
