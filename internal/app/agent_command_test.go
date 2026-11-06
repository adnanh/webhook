package app

import (
	"bytes"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/adnanh/webhook/internal/hook"
	"github.com/adnanh/webhook/internal/network"
)

func TestTemporaryShares(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "app.war")
	if err := os.WriteFile(file, []byte("abcdef"), 0600); err != nil {
		t.Fatal(err)
	}
	handler, err := newShareHandler([]string{"pkg=" + file, "dist=" + dir})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		method, path string
		status       int
	}{{"GET", "/pkg", 200}, {"HEAD", "/pkg", 200}, {"GET", "/dist/app.war", 200}, {"GET", "/dist", 404}, {"GET", "/dist/../app.war", 404}, {"GET", "/dist/%2e%2e/app.war", 404}, {"POST", "/pkg", 405}} {
		r := httptest.NewRequest(test.method, test.path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != test.status {
			t.Fatalf("%s %s: %d", test.method, test.path, w.Code)
		}
	}
	if err := os.Symlink(file, filepath.Join(dir, "link")); err == nil {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/dist/link", nil))
		if w.Code != 404 {
			t.Fatal("symlink exposed")
		}
	}
	r := httptest.NewRequest("GET", "/pkg", nil)
	r.Header.Set("Range", "bytes=1-3")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 206 || w.Body.String() != "bcd" {
		t.Fatal("range failed")
	}
}

func TestTemporaryAgentCommand(t *testing.T) {
	binary, cleanup := buildWebhook(t)
	t.Cleanup(cleanup)
	dir := t.TempDir()
	token := strings.Repeat("s", 64)
	cfg := `mode: hub
hub:
  nodes:
    - id: temp
      token_env: NETWORK_CLI_TEST_TOKEN
      services: [files]
    - id: ops
      token_env: NETWORK_CLI_TEST_TOKEN
      allow: [temp/files]
`
	path := filepath.Join(dir, "hub.yaml")
	os.WriteFile(path, []byte(cfg), 0600)
	file := filepath.Join(dir, "file.txt")
	os.WriteFile(file, []byte("shared"), 0600)
	host, port := serverAddress(t)
	origin := net.JoinHostPort(host, port)
	env := append(webhookEnv(), "NETWORK_CLI_TEST_TOKEN="+token)
	start := func(args ...string) *exec.Cmd {
		cmd := exec.Command(binary, args...)
		cmd.Env = env
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { killAndWait(cmd) })
		return cmd
	}
	start("-network-config", path, "-ip", host, "-port", port)
	waitForServerReady(t, origin, http.DefaultClient)
	start("agent", "--server", "ws://"+origin, "--id", "temp", "--token-env", "NETWORK_CLI_TEST_TOKEN", "--share", "pkg="+file, "--duration", "30s")
	localHost, localPort := serverAddress(t)
	entry := net.JoinHostPort(localHost, localPort)
	start("agent", "--server", "ws://"+origin, "--id", "ops", "--token-env", "NETWORK_CLI_TEST_TOKEN", "--forward", entry+"=temp/files", "--duration", "30s")
	deadline := time.Now().Add(5 * time.Second)
	for {
		response, err := (&http.Client{Timeout: time.Second}).Get("http://" + entry + "/pkg")
		if err == nil {
			body, _ := io.ReadAll(response.Body)
			response.Body.Close()
			if response.StatusCode == 200 && string(body) == "shared" {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("temporary share unavailable: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestAsyncMultipartSurvivesResponseCleanup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell command")
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, _ := form.CreateFormFile("pkg", "app.war")
	io.WriteString(part, "package")
	form.Close()
	raw := httptest.NewRequest("POST", "/hooks/deploy", &body)
	raw.Header.Set("Content-Type", form.FormDataContentType())
	if err := raw.ParseMultipartForm(1); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "received")
	h := &hook.Hook{ID: "async-file-test", ExecuteCommand: "/bin/sh", KeepFileEnvironment: true, PassArgumentsToCommand: []hook.Argument{{Source: "string", Name: "-c"}, {Source: "string", Name: `sleep 0.1; cp "$HOOK_FILE_PKG" "$1"`}, {Source: "string", Name: "sh"}, {Source: "string", Name: target}}}
	if err := handleHookAsync(h, &hook.Request{ID: "test", RawRequest: raw}); err != nil {
		t.Fatal(err)
	}
	// net/http performs this cleanup after the HTTP handler returns.
	if raw.MultipartForm != nil {
		raw.MultipartForm.RemoveAll()
		t.Fatal("multipart ownership was not detached")
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		data, err := os.ReadFile(target)
		if err == nil && string(data) == "package" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("async command lost its upload")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestTemporaryIdleAgentCLI(t *testing.T) {
	binary, cleanup := buildWebhook(t)
	t.Cleanup(cleanup)
	token := strings.Repeat("t", 64)
	cfg, err := network.ParseConfig([]byte("mode: hub\nhub:\n  nodes:\n    - id: idle\n      token: "+token+"\n"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	hub := network.NewHub(cfg.Hub)
	defer hub.Close()
	server := httptest.NewServer(hub)
	defer server.Close()
	cmd := exec.Command(binary, "agent", "--server", "ws"+strings.TrimPrefix(server.URL, "http"), "--id", "idle", "--token", token, "--duration", "2s")
	cmd.Env = webhookEnv()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer killAndWait(cmd)
	deadline := time.Now().Add(time.Second)
	for {
		node := hub.Status().Nodes[0]
		if node.Status == "online" {
			if node.Version != version || !node.Temporary || node.ExpiresAt == nil || len(node.Services) != 0 {
				t.Fatalf("invalid CLI metadata: %+v", node)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("idle CLI did not join hub")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("temporary CLI did not exit cleanly: %v", err)
	}
	deadline = time.Now().Add(time.Second)
	for hub.Status().Nodes[0].Status != "offline" {
		if time.Now().After(deadline) {
			t.Fatal("expired CLI still online")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
