GO ?= go
OS ?= darwin freebsd linux openbsd
ARCHS ?= 386 arm amd64 arm64
INSTALL_DIR ?= /usr/local/bin
BINARY_NAME ?= webhook
COMMAND := ./cmd/webhook
LDFLAGS ?=

.DEFAULT_GOAL := help
.PHONY: help all build install test vet deps tidy clean release release-linux release-linux-x86 release-windows demo smoke

help: ## 显示可用命令
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {printf "%-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

all: build test ## 构建并测试

build: ## 构建到 build/webhook（Windows 为 webhook.exe）
	@mkdir -p build
	$(GO) build -trimpath -ldflags="$(LDFLAGS)" -o build/$(BINARY_NAME)$(if $(filter windows,$(shell $(GO) env GOOS)),.exe,) $(COMMAND)

install: build ## 安装到 INSTALL_DIR（保留 BINARY_NAME 自定义）
	install -d "$(INSTALL_DIR)"
	install -m 0755 "build/$(BINARY_NAME)$(if $(filter windows,$(shell $(GO) env GOOS)),.exe,)" "$(INSTALL_DIR)/$(BINARY_NAME)$(if $(filter windows,$(shell $(GO) env GOOS)),.exe,)"

test: ## 执行完整测试
	$(GO) test ./...

vet: ## 执行 Go 静态检查
	$(GO) vet ./...

deps: ## 下载 go.mod 锁定的依赖（不升级版本）
	$(GO) mod download

tidy: ## 整理模块依赖（修改 go.mod/go.sum 后需检查差异）
	$(GO) mod tidy

demo: build ## 构建无副作用的演示命令，然后按 examples/README.md 启动
	$(GO) build -o build/demo-hook$(if $(filter windows,$(shell $(GO) env GOOS)),.exe,) ./examples/demo-hook

smoke: ## 自动运行全部本机演示（需要 Python 3）
	python3 scripts/smoke-examples.py

# Query Go's supported target list instead of trying invalid OS/architecture pairs.
# Do not clean here: separate release targets must not erase each other's archives.
release: ## 为 OS 和 ARCHS 中支持的 Unix 组合打包到 build/
	@set -eu; targets="$$($(GO) tool dist list)"; \
	for os in $(OS); do \
	  for arch in $(ARCHS); do \
	    if ! echo "$$targets" | grep -qx "$$os/$$arch"; then continue; fi; \
	    target="webhook-$$os-$$arch"; binary=webhook; \
	    if [ "$$os" = windows ]; then binary=webhook.exe; fi; \
	    echo "Building $$target"; mkdir -p "build/$$target"; \
	    CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o "build/$$target/$$binary" $(COMMAND); \
	    cp LICENSE README.md README_CN.md "build/$$target/"; \
	    tar czf "build/$$target.tar.gz" -C build "$$target"; \
	  done; \
	done

release-linux: ## 为 Linux 打包
	$(MAKE) release OS=linux

release-linux-x86: ## 为 Linux amd64 打包（兼容已有命令）
	$(MAKE) release OS=linux ARCHS=amd64

release-windows: ## 为 Windows 打包
	$(MAKE) release OS=windows

clean: ## 清理构建产物
	rm -rf build dist
	rm -f webhook webhook.exe
