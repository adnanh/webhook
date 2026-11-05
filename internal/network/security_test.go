package network

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCredentialSources(t *testing.T) {
	secret := strings.Repeat("s", 64)
	t.Setenv("NETWORK_TEST_TOKEN", secret)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "token"), []byte(secret+"\n"), 0600)
	for _, source := range [][3]string{{secret, "", ""}, {"", "NETWORK_TEST_TOKEN", ""}, {"", "", "token"}} {
		got, err := resolveToken(dir, source[0], source[1], source[2])
		if err != nil || got != secret {
			t.Fatalf("credential resolution: %v", err)
		}
	}
	for _, source := range [][3]string{{"", "", ""}, {secret, "NETWORK_TEST_TOKEN", ""}, {"", "MISSING_NETWORK_TEST_TOKEN", ""}, {"short", "", ""}} {
		_, err := resolveToken(dir, source[0], source[1], source[2])
		if err == nil || strings.Contains(err.Error(), secret) {
			t.Fatal("invalid credentials accepted or leaked")
		}
	}
}

func TestAuthenticationReplayAndFreshness(t *testing.T) {
	token := strings.Repeat("a", 64)
	key := sha256.Sum256([]byte(token))
	hub := NewHub(&HubConfig{Nodes: []NodePolicy{{ID: "node", tokenHash: key}}})
	defer hub.Close()
	nonce, _ := newNonce()
	request := func(nonce []byte) *http.Request {
		r := httptest.NewRequest("GET", ControlPath, nil)
		r.Header.Set(nodeHeader, "node")
		r.Header.Set(nonceHeader, base64.RawURLEncoding.EncodeToString(nonce))
		r.Header.Set("Authorization", "Network-HMAC "+connectionProof(key, "webhook-network-agent-proof-v1", nonce, "node", ControlPath, "", "", ""))
		return r
	}
	if _, ok := hub.authenticate(request(nonce), nonce); !ok {
		t.Fatal("fresh proof rejected")
	}
	if _, ok := hub.authenticate(request(nonce), nonce); ok {
		t.Fatal("replayed proof accepted")
	}
	for _, offset := range []int64{-120, 120} {
		n, _ := newNonce()
		binary.BigEndian.PutUint64(n[:8], uint64(time.Now().Unix()+offset))
		if _, ok := hub.authenticate(request(n), n); ok {
			t.Fatal("expired/future proof accepted")
		}
	}
}

func TestUntrustedTLSHubReceivesNoCredentials(t *testing.T) {
	requests := make(chan bool, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests <- true }))
	defer server.Close()
	a, err := newAgent(&AgentConfig{Server: "wss" + strings.TrimPrefix(server.URL, "https"), ID: "node", token: strings.Repeat("a", 64), MaxConnections: 1})
	if err != nil {
		t.Fatal(err)
	}
	if conn, err := a.openWebSocket(context.Background(), ControlPath, "", "", ""); err == nil {
		conn.Close()
		t.Fatal("untrusted TLS accepted")
	}
	select {
	case <-requests:
		t.Fatal("credentials sent before TLS identity verification")
	default:
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func TestLargeMultipartWebhookTunnel(t *testing.T) {
	const size = 201 << 20
	token := strings.Repeat("t", 64)
	hub := NewHub(&HubConfig{MaxSessions: 4, RequestTimeout: 30 * time.Second, Nodes: []NodePolicy{{ID: "node", Services: []string{"webhook"}, HookPrefix: "/hooks", tokenHash: sha256.Sum256([]byte(token))}}, Routes: []HookRoute{{Path: "/upload", Hook: "upload", Agents: []string{"node"}}}})
	defer hub.Close()
	mux := http.NewServeMux()
	mux.HandleFunc(ControlPath, hub.ServeHTTP)
	mux.HandleFunc(DataPath, hub.ServeHTTP)
	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		hub.ServeHook(w, r, RequestOrigin{ClientIP: "127.0.0.1"}, 256<<20)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, "parse failed", 400)
			return
		}
		defer r.MultipartForm.RemoveAll()
		file, _, err := r.FormFile("pkg")
		if err != nil {
			http.Error(w, "file missing", 400)
			return
		}
		defer file.Close()
		if _, ok := file.(*os.File); !ok {
			http.Error(w, "expected disk-backed upload", 500)
			return
		}
		hash := sha256.New()
		n, err := io.Copy(hash, file)
		if err != nil || n != size {
			http.Error(w, "wrong upload length", 400)
			return
		}
		fmt.Fprintf(w, "%x", hash.Sum(nil))
	})
	cfg := &AgentConfig{Server: "ws" + strings.TrimPrefix(server.URL, "http"), ID: "node", token: token, MaxConnections: 2, WebhookOnlyViaHub: true, Services: []Service{{Name: "webhook"}}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunAgent(ctx, cfg, AgentOptions{WebhookHandler: handler, Hooks: func() []HookInfo { return []HookInfo{{ID: "upload"}} }})
	}()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		hub.mu.Lock()
		ready := hub.controls["node"] != nil
		hub.mu.Unlock()
		if ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("agent not connected")
		}
		time.Sleep(10 * time.Millisecond)
	}
	reader, writer := io.Pipe()
	form := multipart.NewWriter(writer)
	produced := make(chan error, 1)
	go func() {
		part, err := form.CreateFormFile("pkg", "app.war")
		if err == nil {
			_, err = io.CopyN(part, zeroReader{}, size)
		}
		if err == nil {
			err = form.Close()
		}
		writer.CloseWithError(err)
		produced <- err
	}()
	defer reader.Close()
	response, err := (&http.Client{Timeout: 35 * time.Second}).Post(server.URL+"/upload", form.FormDataContentType(), reader)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	expected := sha256.New()
	io.CopyN(expected, zeroReader{}, size)
	if response.StatusCode != 200 || string(body) != fmt.Sprintf("%x", expected.Sum(nil)) {
		t.Fatalf("upload failed: %d %s", response.StatusCode, body)
	}
	if err := <-produced; err != nil {
		t.Fatal(err)
	}
}
