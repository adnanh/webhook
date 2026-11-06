package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	webhookupdate "github.com/adnanh/webhook/internal/update"
)

var (
	updateEnabled    = flag.Bool("update-enabled", true, "启用 Admin API 的更新检查功能")
	updateRepository = flag.String("update-repository", webhookupdate.DefaultRepository, "获取更新的 GitHub 仓库（所有者/仓库名）")
	updateStateDir   = flag.String("update-state-dir", "", "预留的更新状态目录（当前 Admin API 不使用）")

	updateManifestPublicKey string
)

func isUpdateCommand(args []string) bool {
	return len(args) > 1 && args[1] == "update"
}

func newUpdateClient(repository string) *webhookupdate.Client {
	return &webhookupdate.Client{
		Repository: repository,
		Version:    version,
		PublicKey:  updateManifestPublicKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func runUpdateCommand(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		writeUpdateUsage(stderr)
		return 2
	}
	if args[0] == "-h" || args[0] == "--help" {
		writeUpdateUsage(stdout)
		return 0
	}

	switch args[0] {
	case "check":
		return runUpdateCheck(args[1:], stdout, stderr)
	case "apply":
		return runUpdateApply(args[1:], stdin, stdout, stderr)
	case "rollback":
		return runUpdateRollback(args[1:], stdout, stderr)
	case "help":
		writeUpdateUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "未知的更新子命令 %q\n", args[0])
		writeUpdateUsage(stderr)
		return 2
	}
}

func runUpdateCheck(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("webhook update check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	setCommandHelp(flags, "检查指定 GitHub 仓库的发布版本，可选择以 JSON 格式输出结果。")
	repository := flags.String("repository", webhookupdate.DefaultRepository, "GitHub 仓库（所有者/仓库名）")
	requestedVersion := flags.String("version", "", "发布版本号，默认使用最新版本")
	jsonOutput := flags.Bool("json", false, "以 JSON 格式输出结果")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "update check does not accept positional arguments")
		return 2
	}

	result, err := newUpdateClient(*repository).Check(context.Background(), *requestedVersion)
	if err != nil {
		fmt.Fprintln(stderr, "update check failed:", err)
		return 1
	}
	if *jsonOutput {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintln(stderr, "encode update result:", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(stdout, "Current version: %s\n", result.CurrentVersion)
	fmt.Fprintf(stdout, "Latest version:  %s\n", result.LatestVersion)
	if result.Available {
		fmt.Fprintln(stdout, "Update available: yes")
	} else {
		fmt.Fprintln(stdout, "Update available: no")
	}
	if result.Verified {
		fmt.Fprintln(stdout, "Manifest signature: verified")
	} else {
		fmt.Fprintln(stdout, "Manifest signature: unavailable in this build")
	}
	return 0
}

func runUpdateApply(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("webhook update apply", flag.ContinueOnError)
	flags.SetOutput(stderr)
	setCommandHelp(flags, "下载并验证更新，替换指定的可执行文件；更新后需重启服务。")
	repository := flags.String("repository", webhookupdate.DefaultRepository, "GitHub 仓库（所有者/仓库名）")
	requestedVersion := flags.String("version", "", "发布版本号，默认使用最新版本")
	target := flags.String("target", "", "要替换的可执行文件路径，默认为当前程序")
	stateDir := flags.String("state-dir", "", "更新状态目录，默认为当前工作目录")
	yes := flags.Bool("yes", false, "直接应用更新，跳过交互确认")
	allowUnsigned := flags.Bool("allow-unsigned", false, "允许在未配置签名公钥时应用未经验证的更新清单")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "update apply does not accept positional arguments")
		return 2
	}

	client := newUpdateClient(*repository)
	result, err := client.Check(context.Background(), *requestedVersion)
	if err != nil {
		fmt.Fprintln(stderr, "update check failed:", err)
		return 1
	}
	if !result.Verified && !*allowUnsigned {
		fmt.Fprintln(stderr, "update cannot be applied because this build has no trusted manifest public key")
		return 1
	}
	if *requestedVersion == "" && !result.Available {
		fmt.Fprintf(stdout, "webhook %s is already up to date.\n", version)
		return 0
	}
	if !*yes {
		fmt.Fprintf(stdout, "Replace webhook %s with %s? [y/N] ", version, result.LatestVersion)
		answer, _ := bufio.NewReader(stdin).ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(stdout, "Update cancelled.")
			return 0
		}
	}

	state, err := client.Apply(context.Background(), webhookupdate.ApplyOptions{
		Version:       result.LatestVersion,
		Target:        *target,
		StateDir:      *stateDir,
		AllowUnsigned: *allowUnsigned,
	})
	if err != nil {
		fmt.Fprintln(stderr, "update failed:", err)
		return 1
	}
	fmt.Fprintf(stdout, "Updated webhook from %s to %s.\n", state.CurrentVersion, state.InstalledVersion)
	fmt.Fprintln(stdout, "Restart the running webhook service to activate the new version.")
	return 0
}

func runUpdateRollback(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("webhook update rollback", flag.ContinueOnError)
	flags.SetOutput(stderr)
	setCommandHelp(flags, "根据已有更新状态恢复上一版本的可执行文件；回滚后需重启服务。")
	target := flags.String("target", "", "要恢复的可执行文件路径，默认为当前程序")
	stateDir := flags.String("state-dir", "", "更新状态目录，默认为当前工作目录")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "update rollback does not accept positional arguments")
		return 2
	}

	state, err := webhookupdate.Rollback(*target, *stateDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(stderr, "rollback is unavailable: no previous update state was found")
		} else {
			fmt.Fprintln(stderr, "rollback failed:", err)
		}
		return 1
	}
	fmt.Fprintf(stdout, "Restored webhook %s over %s.\n", state.CurrentVersion, state.InstalledVersion)
	fmt.Fprintln(stdout, "Restart the running webhook service to activate the restored version.")
	return 0
}

func writeUpdateUsage(output io.Writer) {
	fmt.Fprintln(output, "用法：")
	fmt.Fprintln(output, "  webhook update check [--repository 所有者/仓库名] [--version 版本号] [--json]")
	fmt.Fprintln(output, "    检查可用更新，默认检查最新版本。")
	fmt.Fprintln(output, "  webhook update apply [--repository 所有者/仓库名] [--version 版本号] [--target 程序路径] [--state-dir 目录] [--yes] [--allow-unsigned]")
	fmt.Fprintln(output, "    下载并应用更新，保存回滚状态；默认需要交互确认。")
	fmt.Fprintln(output, "  webhook update rollback [--target 程序路径] [--state-dir 目录]")
	fmt.Fprintln(output, "    使用已有更新状态恢复上一版本。")
	fmt.Fprintln(output, "\n在具体子命令后添加 --help 查看参数说明，例如 webhook update apply --help。")
}
