package main

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

	"github.com/adnanh/webhook/internal/network"
)

type agentValues []string

func (v *agentValues) String() string     { return strings.Join(*v, ",") }
func (v *agentValues) Set(s string) error { *v = append(*v, s); return nil }

func runAgentCommand(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("webhook agent", flag.ContinueOnError)
	flags.SetOutput(stderr)
	cfg := &network.AgentConfig{}
	flags.StringVar(&cfg.Server, "server", "", "hub origin, normally wss://hub.example.com")
	flags.StringVar(&cfg.ID, "id", "", "node identity pre-authorized by the hub")
	flags.StringVar(&cfg.Token, "token", "", "literal node token (visible in process arguments; prefer token-env or token-file)")
	flags.StringVar(&cfg.TokenEnv, "token-env", "", "environment variable containing the node token")
	flags.StringVar(&cfg.TokenFile, "token-file", "", "file containing the node token")
	flags.StringVar(&cfg.CAFile, "ca-file", "", "optional private CA certificate")
	flags.IntVar(&cfg.MaxConnections, "max-connections", 8, "maximum concurrent tunnels")
	lifetime := flags.Duration("duration", time.Hour, "stop after this duration; must be positive")
	var shares, publishes, forwards agentValues
	flags.Var(&shares, "share", "read-only file or directory: URL-name=path; repeatable; service name is files")
	flags.Var(&publishes, "publish", "publish a TCP service: name=host:port; repeatable")
	flags.Var(&forwards, "forward", "local loopback entry: host:port=node/service; repeatable")
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
