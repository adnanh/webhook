package app

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/adnanh/webhook/internal/hook"
	"github.com/adnanh/webhook/internal/network"
	"gopkg.in/yaml.v2"
)

// Exercise the actual documented configs and demo executable through a server,
// so examples cannot silently drift from the implementation.
func TestDocumentedExamples(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	demo := filepath.Join(dir, "demo-hook")
	if runtime.GOOS == "windows" {
		demo += ".exe"
	}
	cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-o", demo, "./examples/demo-hook")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build demo: %v\n%s", err, output)
	}
	t.Setenv("DEMO_HMAC_SECRET", "documented-example-test-secret")
	var hooks hook.Hooks
	for _, name := range []string{
		"basic/hooks.yaml", "files/hooks.yaml", "execution/hooks.yaml",
		"signature/hooks.yaml.tmpl", "hotreload/hooks/ping.yaml",
	} {
		var current hook.Hooks
		if err := current.LoadFromFile(filepath.Join(root, "examples", name), true); err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		for i := range current {
			// Only replace the executable path for portable temporary builds.
			current[i].ExecuteCommand = demo
		}
		hooks = append(hooks, current...)
	}
	data, err := json.Marshal(hooks)
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "hooks.json")
	if err := os.WriteFile(config, data, 0600); err != nil {
		t.Fatal(err)
	}
	binary, cleanup := buildWebhook(t)
	defer cleanup()
	host, port := serverAddress(t)
	cmd = exec.Command(binary, "-hooks", config, "-ip", host, "-port", port)
	cmd.Env = webhookEnv()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer killAndWait(cmd)
	authority := net.JoinHostPort(host, port)
	waitForServerReady(t, authority, http.DefaultClient)
	client := &http.Client{Timeout: 10 * time.Second}
	base := "http://" + authority
	request := func(t *testing.T, method, path, contentType string, body io.Reader, headers map[string]string, status int) string {
		t.Helper()
		req, err := http.NewRequest(method, base+path, body)
		if err != nil {
			t.Fatal(err)
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != status {
			t.Fatalf("%s: status %d, want %d: %s", path, res.StatusCode, status, data)
		}
		return string(data)
	}
	t.Run("arguments and rules", func(t *testing.T) {
		body := request(t, "POST", "/hooks/hello?env=dev", "application/json", strings.NewReader(`{"user":{"name":"小明"}}`), map[string]string{
			"X-Demo-Token": "local-demo-only", "X-Trace": "trace-001",
		}, 200)
		if strings.TrimSpace(body) != `{"arguments":["小明","dev","POST"],"trace":"trace-001"}` {
			t.Fatalf("unexpected mapping: %s", body)
		}
		request(t, "POST", "/hooks/hello?env=dev", "", nil, nil, 403)
		request(t, "GET", "/hooks/hello?env=dev", "", nil, nil, 405)
	})
	t.Run("multipart file", func(t *testing.T) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("pkg", "hello.txt")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(part, "你好，webhook！\n")
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		got := request(t, "POST", "/hooks/upload", writer.FormDataContentType(), &body, nil, 200)
		if got != "你好，webhook！\n" {
			t.Fatalf("unexpected upload: %q", got)
		}
	})
	t.Run("base64 file", func(t *testing.T) {
		got := request(t, "POST", "/hooks/base64", "application/json", strings.NewReader(`{"content":"aGVsbG8K"}`), nil, 200)
		if got != "hello\n" {
			t.Fatalf("unexpected decoded file: %q", got)
		}
	})
	t.Run("form JSON", func(t *testing.T) {
		form := url.Values{"metadata": {`{"version":"1.2.3"}`}}
		got := request(t, "POST", "/hooks/nested-json", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()), nil, 200)
		if !strings.Contains(got, `"arguments":["1.2.3"]`) {
			t.Fatalf("unexpected nested JSON: %s", got)
		}
	})
	t.Run("HMAC", func(t *testing.T) {
		body, err := os.ReadFile(filepath.Join(root, "examples/signature/payload.json"))
		if err != nil {
			t.Fatal(err)
		}
		mac := hmac.New(sha256.New, []byte(os.Getenv("DEMO_HMAC_SECRET")))
		_, _ = mac.Write(body)
		header := map[string]string{"X-Hub-Signature-256": "sha256=" + hex.EncodeToString(mac.Sum(nil))}
		got := request(t, "POST", "/hooks/signed", "application/json", bytes.NewReader(body), header, 200)
		if !strings.Contains(got, "refs/heads/main") {
			t.Fatalf("unexpected signed response: %s", got)
		}
		request(t, "POST", "/hooks/signed", "application/json", strings.NewReader(`{"ref":"tampered"}`), header, 500)
	})
	t.Run("timeout", func(t *testing.T) {
		request(t, "POST", "/hooks/timeout", "", nil, nil, 500)
	})
	t.Run("async", func(t *testing.T) {
		if got := request(t, "POST", "/hooks/async", "", nil, nil, 202); got != "accepted" {
			t.Fatalf("unexpected async response: %q", got)
		}
	})
	t.Run("failure output", func(t *testing.T) {
		if got := request(t, "POST", "/hooks/failure", "", nil, nil, 500); !strings.Contains(got, "demo failure") {
			t.Fatalf("missing error output: %q", got)
		}
	})
	t.Run("concurrency", func(t *testing.T) {
		type result struct {
			status int
			err    error
		}
		results := make(chan result, 2)
		for i := 0; i < 2; i++ {
			go func() {
				res, err := client.Post(base+"/hooks/limited", "text/plain", nil)
				if err != nil {
					results <- result{err: err}
					return
				}
				_, err = io.Copy(io.Discard, res.Body)
				_ = res.Body.Close()
				results <- result{res.StatusCode, err}
			}()
		}
		statuses := map[int]int{}
		for i := 0; i < 2; i++ {
			r := <-results
			if r.err != nil {
				t.Fatal(r.err)
			}
			statuses[r.status]++
		}
		if statuses[200] != 1 || statuses[503] != 1 {
			t.Fatalf("expected one success and one rejection, got %v", statuses)
		}
	})
	t.Run("status and ping", func(t *testing.T) {
		request(t, "POST", "/hooks/ping", "", nil, nil, 200)
		got := request(t, "GET", "/status", "", nil, nil, 200)
		var status serviceStatusResponse
		if err := json.Unmarshal([]byte(got), &status); err != nil {
			t.Fatal(err)
		}
		if status.Status != "ok" || status.Hooks.Count != len(hooks) {
			t.Fatalf("unexpected status: %s", got)
		}
	})
}

func TestDocumentedNetworkConfigs(t *testing.T) {
	t.Setenv("DEMO_NODE_TOKEN", strings.Repeat("a", 64))
	for _, name := range []string{"hub", "agent"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("../../examples/network", name+".yaml")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var config struct {
				Network interface{} `yaml:"network"`
			}
			if err := yaml.Unmarshal(data, &config); err != nil {
				t.Fatal(err)
			}
			data, err = yaml.Marshal(config.Network)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := network.ParseConfig(data, filepath.Dir(path)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestEntrypointBuildVersion(t *testing.T) {
	for _, target := range []string{"../..", "../../cmd/webhook"} {
		t.Run(target, func(t *testing.T) {
			binary := filepath.Join(t.TempDir(), "webhook")
			if runtime.GOOS == "windows" {
				binary += ".exe"
			}
			cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-ldflags=-X main.version=9.8.7", "-o", binary, target)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("build: %v\n%s", err, output)
			}
			output, err := exec.Command(binary, "-version").CombinedOutput()
			if err != nil || strings.TrimSpace(string(output)) != "webhook version 9.8.7" {
				t.Fatalf("version metadata not preserved: %v, %s", err, output)
			}
		})
	}
}
