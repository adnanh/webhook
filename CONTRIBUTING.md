# 参与贡献

欢迎为 webhook 提交问题、文档和代码改进。项目提供 Webhook 自动化、Admin 管理及 Hub/Agent 内网执行，来源与许可见[项目说明](docs/Project-Origin.md)。

## 提交问题

请在 [项目 Issues](https://github.com/xtulnx/webhook/issues) 搜索已有讨论，再说明：

- `webhook -version` 输出、操作系统和 CPU 架构。
- 预期行为、实际行为和最小复现步骤。
- 已去除真实密钥、令牌、业务数据的配置与相关日志。

请区分当前 1.x 版本、历史构建以及上游 adnanh/webhook 的版本，方便定位代码来源。

## 开发与验证

规范入口为 `./cmd/webhook`，Go module 为 `github.com/xtulnx/webhook`。使用 go.mod / go.sum 中锁定的依赖，代码提交前执行 gofmt。

```sh
go build ./cmd/webhook
go test ./...
go vet ./...
```

修改安装或发布逻辑时运行对应的离线测试：

```sh
python3 scripts/test-installers.py
python3 scripts/test-release.py
```

涉及演示、参数或配置行为时，更新相关文档和样例，并运行 `python3 scripts/smoke-examples.py`。更多命令和发布签名说明见[开发指南](docs/Development.md)。

## Pull Request

PR 应说明解决的问题、最终行为和验证结果。行为变化应包含有意义的测试；影响 CLI、配置或安装更新时同步更新中文文档。保持改动集中，避免顺带升级依赖或混入其他功能。

保留已有 `webhook` 命令、配置与协议的兼容性；涉及不兼容行为时，请明确描述影响和迁移方法。版本号由本项目的发布 tag 决定，不沿用上游版本数字来表示功能进度。

## 致谢与许可

感谢 [adnanh/webhook](https://github.com/adnanh/webhook) 及其贡献者提供基础实现，也感谢本仓库的贡献者。贡献内容应与仓库的 [MIT License](LICENSE) 兼容；保留原作者的版权及许可信息。
