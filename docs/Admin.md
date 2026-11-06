# Admin 管理界面与 API

Admin 默认关闭，启用后在线管理已加载的 Hook 文件。它具有修改执行命令的能力，应只交给可信管理员。

## 启用与登录

```yaml
admin: true
admin-path: admin
admin-totp-secret: 替换为独立的随机Base32密钥
admin-jwt-secret: 替换为另一个长随机字符串
admin-session-ttl: 1h
```

生成 TOTP 和 JWT 密钥（各生成一次并安全保存）：

```sh
python3 -c 'import base64,secrets; print(base64.b32encode(secrets.token_bytes(20)).decode().rstrip("="))'
openssl rand -hex 32
```

可复制 [示例总配置](../examples/admin/webhook.yaml.example) 到 `build/admin.yaml`，填好密钥，将一个 Hook 文件复制到 `build/admin-hooks.yaml` 后，从仓库根目录运行：

```sh
cp examples/admin/webhook.yaml.example build/admin.yaml
cp examples/basic/hooks.yaml build/admin-hooks.yaml
# 编辑 build/admin.yaml，填写随机密钥
./build/webhook -c build/admin.yaml
```

访问 `http://127.0.0.1:9000/admin/`，把 Base32 密钥录入兼容 TOTP 的认证器，使用当前六位动态码登录。实现使用 30 秒步长并接受前后一个时间窗口；服务器和认证器需保持时钟同步。默认会话有效期 12h，上面示例缩短为 1h。`admin-path` 不能与 hook 前缀相同，状态路径也不能与 Admin 路径重叠。

## 编辑行为

页面可查看当前加载的文件和 hooks，新增、修改、删除文件内的 Hook，提供结构化字段和 JSON 预览。写入只限已加载文件，不提供任意路径写入或创建任意配置文件的接口。保存后更新磁盘文件及运行时状态，无需另行开启 hotreload。

启用 `template: true` 时整体只读；文件权限不足也无法保存。保存 JSON/YAML 时会重新序列化，原注释和排版可能不保留，修改前应备份配置。完整配置与命令权限需要由运维管理，Admin 不负责隔离脚本权限。

## API

下表使用默认 `/admin`，修改前缀后相应替换。除登录、登出和静态页面外均要求有效会话。

| 方法 | 路径 | 功能 |
| --- | --- | --- |
| POST | `/admin/api/auth/login` | body 为 `{"code":"123456"}`；成功设置会话 cookie，返回 `{"ok":true}` |
| POST | `/admin/api/auth/logout` | 清除 cookie |
| GET | `/admin/api/config` | 文件路径、格式、可写标记、hooks、全局只读原因 |
| POST | `/admin/api/hooks` | 新增；`{"file":"已加载的绝对路径","hook":{...}}` |
| PUT | `/admin/api/hooks` | 修改；另传 `currentId` 表示原 Hook ID |
| DELETE | `/admin/api/hooks` | 删除；`{"file":"已加载的绝对路径","currentId":"hello"}` |
| GET | `/admin/api/update/status` | 更新设置和本进程最近一次检查结果 |
| POST | `/admin/api/update/check` | 检查配置仓库的最新 Release；当前不读取请求 body |
| GET | `/admin/api/network/nodes` | 仅 Hub 启用时提供，返回 Hub 版本及全部已配置节点状态 |
| GET | `/admin/api/network/nodes/{id}` | 指定节点状态，未知节点 404 |

命令行登录与查询示例，`CODE` 由当前认证器提供：

```sh
curl -c build/admin.cookies -H 'Content-Type: application/json' \
  --data "{\"code\":\"$CODE\"}" http://127.0.0.1:9000/admin/api/auth/login
curl -b build/admin.cookies http://127.0.0.1:9000/admin/api/config
```

成功登录只返回 cookie，不在 JSON 中直接返回 token。接口也接受 `Authorization: Bearer <JWT>`，如调用方已有有效会话 token。Cookie 为 HttpOnly、SameSite=Strict，作用路径是 Admin 前缀；`Secure` 标记跟随服务的 `-secure` 参数，不会根据反向代理头自动切换，反代终止 TLS 时可由代理额外设置安全 cookie 属性。退出只清理当前 cookie，不代表服务端维护了 JWT 撤销列表；轮换 JWT secret 并重启可使旧会话失效。

认证失败返回 401；非法请求常为 400；未知文件或 ID 为 404；只读模板/冲突为 409；新增成功为 201。配置 API 会暴露命令、路径和 Hook 配置中的 secret，客户端同样应妥善处理数据。

## 更新与节点信息的边界

Admin 只提供更新查询，不在线替换或重启程序。最近检查结果保存在进程内存，重启后清空，不读取 CLI 更新的磁盘状态。`update-state-dir` 当前仅注册为预留配置项，尚未接入 Admin 实现。真正安装和回滚使用 [CLI](Operations.md#检查更新应用与回滚)。节点 API 可查看在线状态、版本、连接/容量及临时节点到期信息，当前没有在线下发组网配置或强制跨版本更新能力。

当前没有独立登录速率限制或持久化 TOTP 防重放存储。对外部署用 HTTPS，并通过反向代理或网络访问控制限制 Admin 来源、增加登录限流。不要把演示用固定密钥用于真实服务。
