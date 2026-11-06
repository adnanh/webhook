# 开发、目录与依赖管理

## 当前结构

```text
cmd/webhook/main.go         标准命令入口，只传递构建版本信息
webhook.go                 兼容入口：保留 go build / go run .
internal/
  app/                     CLI、服务启动、总配置、Admin、执行控制、平台适配
    adminui/               embed 资源，随二进制分发，无运行时前端依赖
    testdata/              集成测试命令与 JSON/YAML 模板，Go 不当作独立包扫描
  hook/                    Hook 定义、规则、请求解析与参数提取
  middleware/              请求 ID、日志与调试
  network/                 Hub/Agent、认证、HTTP/TCP 隧道、节点状态
  pidfile/                 平台相关 PID 文件管理
  update/                  清单、签名、下载校验、替换和回滚
examples/
  config/                  完整配置和历史模板样例
  demo-hook/               跨平台、无发布副作用的演示命令
  basic/ signature/ files/ execution/ hotreload/ admin/ network/
docs/                      中文文档入口 README.md
  examples/                既有组网/OSS 发布场景，保留原路径
scripts/                   shell / PowerShell 安装脚本
.github/workflows/         多系统 CI 与 Release
```

按照 [Go 官方模块组织建议](https://go.dev/doc/modules/layout)，命令集中在 `cmd`，不可供其他模块导入的实现放在 `internal`，无需为了目录数量拆成多个 Go module。此仓库保持一个 go.mod，没有引入没有实际需求的 `pkg/` 或 go.work。应用层仍是进程级 CLI，使用全局 flag，不承诺作为可反复启动的库使用。

模块路径继续保留 `github.com/adnanh/webhook`，避免修改现有 import 身份。本次没有顺带升级依赖或提升最低语言版本：`go 1.21` 是兼容下限，`toolchain go1.22.0` 是工具链建议，不等于生产环境应长期使用旧工具链；推荐使用[仍受支持的稳定 Go](https://go.dev/doc/devel/release#policy)，CI 同时覆盖最低语言版本和 stable。

## vendor 是不是过时了？

**vendor 没有被 Go 废弃，也不是错误用法。** Go Modules 同时支持模块缓存和 vendoring；Go 官方仍提供 `go mod vendor` 与 `-mod=vendor`。对于需要离线交付、把依赖源码一并审计或不能访问模块代理的环境，vendor 仍有价值。[官方模块参考](https://go.dev/ref/mod#vendoring)

此仓库的默认流程改为维护 `go.mod` 和 `go.sum`，移除已跟踪的 vendor 源码，避免把业务改动淹没在依赖副本里。`go.sum` 负责内容校验，`go.mod` 描述模块版本与依赖图；移除 vendor 不等于取消版本固定。构建自动下载缺失依赖，`go mod download` 可提前预热缓存。

```sh
go mod download
go mod verify
go build ./cmd/webhook
go test ./...
# 调整依赖或 import 后检查生成差异：
go mod tidy
git diff -- go.mod go.sum
```

`go get module@version` 只用于有意调整依赖版本，不再作为每次 build/test 的前置“安装”动作。普通构建测试不会主动 tidy 或升级依赖。

确需离线分发时，在联网、可信的准备环境生成完整源码包，再交付离线机器：

```sh
go mod vendor
# vendor/ 默认被 .gitignore 忽略，可纳入单独离线源码归档。
# 离线机器同样需要兼容 Go 工具链：
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go build -mod=vendor ./cmd/webhook
```

存在 vendor 时 Go 可能默认使用 vendor，因此本地临时生成后若要回到模块模式，删除自己生成的目录或显式 `-mod=mod`。离线需要提前准备 Go 工具链，只有 vendor 并不能满足自动下载工具链的需求。

## 迁移兼容性

- `go build`、`go run .` 保留兼容；新增规范入口 `./cmd/webhook`。推荐输出到 `build/`。
- 旧的根目录 `hooks*.example` 和 `webhook.yaml.example` 移到 `examples/config/`；引用这些文件的自定义脚本需要更新路径。默认实际配置名 `hooks.json` 行为不变。
- 根目录 `adminui/` 随应用移到 `internal/app/adminui/`，embed 路径仍相对同一包，不影响最终二进制。
- `test/` 移到 `internal/app/testdata/`，集成测试仍编译辅助命令，并同时验证 JSON/YAML。
- 旧的 `go test -run ... .` 应改成 `go test -run ... ./internal/app`；`go test ./...` 保持不变。
- 保留 `make install` 的 INSTALL_DIR / BINARY_NAME 自定义以及 release-linux/release-linux-x86/release-windows。
- 发布仍可使用 `-X main.version=...`、`-X main.updateManifestPublicKey=...`；两个入口使用同一套应用逻辑。
- 不再使用 Go 1.14 的遗留 Travis 配置，统一 GitHub Actions。

## 常用命令

```sh
make build             # build/webhook，Windows 为 build/webhook.exe
make demo              # 额外编译 build/demo-hook
make smoke             # Python 标准库驱动的完整演示验证
make test              # go test ./...
make vet               # go vet ./...
go test -race ./...    # 支持 race 的平台可用
make install INSTALL_DIR="$HOME/.local/bin"
make release-linux-x86
make release OS='darwin linux' ARCHS='amd64 arm64'
make release-windows
```

Release 目标按 `go tool dist list` 过滤不支持的平台组合；构建失败立即停止，多个 release 目标不会互相清空前一次归档。`make clean` 显式清理 build、dist 及根目录二进制。Make 打包命令需要 POSIX shell、tar、grep；Windows 原生优先使用 Go 命令或 GitHub Actions。

代码执行 gofmt，测试跟随所属包，平台代码保持 `_windows.go`、`_unix.go` 等后缀和 build tags。测试专用 socket 辅助函数已放入 `_test.go`，不进入正式程序。

## 发布和签名

`.github/workflows/build.yml` 在分支推送和 PR 时运行，也作为 Release 的可复用检查流程。Ubuntu 覆盖 Go 1.21 兼容下限，Linux / macOS / Windows 覆盖 stable；使用 go.sum 缓存依赖及构建结果，同一分支新任务取消过期 CI。检查包含 build、test、vet、离线安装器测试，以及 Linux 上的演示 smoke。安装器测试也可本地执行：

```sh
python3 scripts/test-installers.py
```

`.github/workflows/release.yml` 在推送 `v*` tag 或手动指定**已存在的 tag** 时发布。手动输入 `2.8.4` 会归一化为 `v2.8.4`，所有检查、构建和清单都使用该 tag 对应的同一提交，避免把运行工作流的分支误发为指定版本。文档中的 `vX.Y.Z` 均应替换为实际新版本。

发布构建按 5 个系统分组复用工具链，生成 15 个平台的 tar.gz；Windows 另外生成 386 / amd64 / arm64 ZIP，Linux ARM 使用 GOARM=6（ARMv6 / ARMv7）。所有二进制禁用 CGO、使用 trimpath 并注入版本。Release 包含 checksums.txt、update-manifest.json、签名文件及三个安装入口 `install.sh` / `install.ps1` / `install.cmd`；内置更新清单继续引用 tar.gz，保持更新器兼容。

签名使用 GitHub secret `UPDATE_SIGNING_PRIVATE_KEY`（PEM Ed25519 私钥），构建使用 repository variable `UPDATE_MANIFEST_PUBLIC_KEY`（Base64 编码的原始 32 字节公钥；格式以 `internal/update` 实现为准）。私钥留在 CI secret 或独立签名系统，不能提交仓库。公私钥均未设置时发布空签名文件，并输出警告；内置 apply 默认仍需受信签名，显式 `--allow-unsigned` 才可降级安装。只配置一项或公私钥不匹配会使发布失败。

发布先创建草稿，全部资产上传后才公开，避免安装脚本看到不完整的 latest；带 `-rc.1` 等后缀的版本自动标为 prerelease，不参与默认 latest 安装。失败的草稿可重新运行补齐，已公开的版本拒绝覆盖，应使用新 tag 修复。工作流默认仅有 contents:read，只有 publish 任务有 contents:write。
```sh
# 示例构建元数据注入；不要把私钥传给 -ldflags。
go build -trimpath -ldflags='-s -w -X main.version=2.8.4' -o build/webhook ./cmd/webhook
./build/webhook -version
```

发布新 tag 前跑测试、静态检查和演示，检查归档包含正确平台二进制，且已配置的签名公钥能验证发布清单。安装/更新回滚流程见[运维](Operations.md)。
