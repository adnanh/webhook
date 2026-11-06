# OSS Web 发布场景

这是已有部署脚本的使用说明，区别于 [无副作用的本机演示](../../../examples/README.md)。脚本会写入本地缓存并同步 OSS 资源，先在专用测试 bucket 与独立 APP 中验证。

## 文件

- `hooks.yaml`：上传入口，映射 `app` / `code` 字段到 APP / CODE，并启用上传文件环境变量。
- `.env.update-web.example`：OSS URI、公开访问地址、ossutil 路径、APP 清单和各 APP 校验码。
- `update.web-dev.sh`：读取上传包、校验身份和内容、更新缓存与 OSS。

## 配置与调用

1. 复制 `.env.update-web.example` 为 `.env.update-web`，填写真实 bucket 和访问地址，配置独立 APP 校验码；不要提交实际配置。
2. 修改 `hooks.yaml` 的命令绝对路径、工作目录、query token 和允许来源 IP；准备可用的 ossutil 凭据及所需权限。
3. 用 `webhook -hooks /absolute/path/hooks.yaml -verbose` 启动。默认异步响应只表示请求被接受，需检查日志和 OSS 结果。
4. 准备 tar.gz 资源包，上传使用 multipart 字段 `pkg`，同时提交 `app`、`code`。上传 URL 还需带上配置的 query token。

服务端默认读取工作目录的 `.env.update-web`，可用 `UPDATE_WEB_ENV_FILE` 指定其他文件。URL 中的 token 不应写入共享日志或提交记录。

脚本的部分业务失败通过文本消息报告且退出码为 0，不可只根据 HTTP 状态或进程退出码判定发布成功，应核对日志和实际资源。

上传上限需同时调整 webhook `max-body-size` 与前置代理限制。同步可能删除 OSS 目标中不在资源包里的文件，请使用独立目标目录；生产发布还应准备备份、回滚和并发互斥。
