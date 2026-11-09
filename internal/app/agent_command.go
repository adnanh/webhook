package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/xtulnx/webhook/internal/network"
)

type agentValues []string

func (v *agentValues) String() string     { return strings.Join(*v, ",") }
func (v *agentValues) Set(s string) error { *v = append(*v, s); return nil }

func runAgentCommand(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("webhook agent", flag.ContinueOnError)
	flags.SetOutput(stderr)
	setCommandHelp(flags, "使用 Hub 预先授权的节点标识和令牌临时加入组网，可选发布服务或建立本机转发。")
	cfg := &network.AgentConfig{}
	flags.StringVar(&cfg.Server, "server", "", "Hub 基础地址，需包含自定义反向代理路径前缀（如有）")
	flags.StringVar(&cfg.ID, "id", "", "Hub 预先授权的节点标识")
	flags.StringVar(&cfg.Token, "token", "", "直接指定节点令牌（会出现在进程参数中，建议使用 token-env 或 token-file）")
	flags.StringVar(&cfg.TokenEnv, "token-env", "", "保存节点令牌的环境变量名称")
	flags.StringVar(&cfg.TokenFile, "token-file", "", "保存节点令牌的文件路径")
	flags.StringVar(&cfg.CAFile, "ca-file", "", "私有 CA 证书路径（可选）")
	flags.IntVar(&cfg.MaxConnections, "max-connections", 8, "最大并发隧道数")
	lifetime := flags.Duration("duration", time.Hour, "到达指定时长后退出，必须大于 0（例如 30m、1h）")
	var shares, publishes, forwards agentValues
	flags.Var(&shares, "share", "共享只读文件或目录，格式为 URL名称=本地路径，可重复指定；服务名为 files")
	flags.Var(&publishes, "publish", "发布 TCP 服务，格式为 服务名=主机:端口，可重复指定")
	flags.Var(&forwards, "forward", "本机回环转发入口，格式为 主机:端口=节点/服务，可重复指定")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	fail := func(err error) int { fmt.Fprintln(stderr, "error:", err); return 2 }
	if flags.NArg() != 0 || *lifetime <= 0 {
		return fail(errors.New("unexpected arguments or invalid duration"))
	}
	handlers := map[string]http.Handler{}
	if len(shares) > 0 {
		handler, err := newShareHandler(shares)
		if err != nil {
			return fail(err)
		}
		handlers["files"] = handler
		cfg.Services = append(cfg.Services, network.Service{Name: "files", InProcess: true})
	}
	for _, value := range publishes {
		name, target, ok := strings.Cut(value, "=")
		if !ok || name == "webhook" {
			return fail(errors.New("publish requires name=host:port; webhook is reserved"))
		}
		cfg.Services = append(cfg.Services, network.Service{Name: name, Target: target})
	}
	for _, value := range forwards {
		listen, to, ok := strings.Cut(value, "=")
		if !ok {
			return fail(errors.New("forward requires host:port=node/service"))
		}
		cfg.Forwards = append(cfg.Forwards, network.Forward{Listen: listen, To: to})
	}
	if err := network.ValidateAgent(cfg); err != nil {
		return fail(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *lifetime)
	defer cancel()
	fmt.Fprintf(stdout, "temporary agent %s; lifetime %s; hub authorization required\n", cfg.ID, *lifetime)
	if err := network.RunAgent(ctx, cfg, network.AgentOptions{Handlers: handlers, Version: version, Temporary: true, ExpiresAt: agentExpiry(ctx)}); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
}

// Only explicitly selected roots are served. No writes, listings, or symlinks.
func newShareHandler(values []string) (http.Handler, error) {
	type share struct {
		path string
		dir  bool
	}
	roots := map[string]share{}
	for _, value := range values {
		name, path, ok := strings.Cut(value, "=")
		if !ok || !network.ValidHookID(name) {
			return nil, errors.New("share requires a simple URL-name=path")
		}
		if _, exists := roots[name]; exists {
			return nil, errors.New("duplicate share name")
		}
		path, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		path, err = filepath.EvalSymlinks(path)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return nil, errors.New("share must be a regular file or directory")
		}
		roots[name] = share{path, info.IsDir()}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", 405)
			return
		}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
		root, ok := roots[parts[0]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		path := root.path
		if !root.dir && len(parts) != 1 {
			http.NotFound(w, r)
			return
		}
		for _, part := range parts[1:] {
			if part == "" || part == "." || part == ".." || strings.ContainsAny(part, "\\\x00:") {
				http.NotFound(w, r)
				return
			}
			path = filepath.Join(path, part)
			info, err := os.Lstat(path)
			if err != nil || info.Mode()&os.ModeSymlink != 0 {
				http.NotFound(w, r)
				return
			}
		}
		file, err := os.Open(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, info.Name(), info.ModTime(), file)
	}), nil
}

func agentExpiry(ctx context.Context) *time.Time {
	deadline, ok := ctx.Deadline()
	if !ok {
		return nil
	}
	deadline = deadline.UTC()
	return &deadline
}
