package network

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestNetworkForwardAndAccessControl(t *testing.T) {
	target, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	go func() {
		for {
			conn, err := target.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				body, err := io.ReadAll(conn)
				if err == nil {
					_, _ = io.WriteString(conn, "reply:"+string(body))
				}
			}()
		}
	}()

	dir := t.TempDir()
	tokenA := strings.Repeat("a", 64)
	tokenB := strings.Repeat("b", 64)
	if err := os.WriteFile(filepath.Join(dir, "a.token"), []byte(tokenA), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.token"), []byte(tokenB), 0600); err != nil {
		t.Fatal(err)
	}
	portListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	forwardAddr := portListener.Addr().String()
	_ = portListener.Close()

	hubPath := writeNetworkConfig(t, dir, "hub.yaml", `mode: hub
hub:
  max_sessions: 8
  nodes:
    - id: a
      token_file: a.token
      allow: [b/echo]
    - id: b
      token_file: b.token
      services: [echo]
`)
	hubCfg, err := LoadConfig(hubPath)
	if err != nil {
		t.Fatal(err)
	}
	hub := NewHub(hubCfg.Hub)
	server := httptest.NewServer(hub)
	defer server.Close()
	defer hub.Close()
	serverURL := "ws" + strings.TrimPrefix(server.URL, "http")
	pathB := writeNetworkConfig(t, dir, "b.yaml", fmt.Sprintf(`mode: agent
agent:
  server: %s
  id: b
  token_file: b.token
  services:
    - name: echo
      target: %s
`, serverURL, target.Addr()))
	pathA := writeNetworkConfig(t, dir, "a.yaml", fmt.Sprintf(`mode: agent
agent:
  server: %s
  id: a
  token_file: a.token
  forwards:
    - listen: %s
      to: b/echo
`, serverURL, forwardAddr))
	cfgA, err := LoadConfig(pathA)
	if err != nil {
		t.Fatal(err)
	}
	cfgB, err := LoadConfig(pathB)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	doneA := make(chan error, 1)
	doneB := make(chan error, 1)
	go func() { doneB <- RunAgent(ctx, cfgB.Agent) }()
	go func() { doneA <- RunAgent(ctx, cfgA.Agent) }()
	defer func() {
		cancel()
		select {
		case err := <-doneA:
			if err != nil {
				t.Errorf("agent A: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("agent A did not stop")
		}
		select {
		case err := <-doneB:
			if err != nil {
				t.Errorf("agent B: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("agent B did not stop")
		}
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", forwardAddr, 100*time.Millisecond)
		if err == nil {
			_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
			_, _ = io.WriteString(conn, "hello")
			_ = conn.(*net.TCPConn).CloseWrite()
			data, readErr := io.ReadAll(conn)
			_ = conn.Close()
			if readErr == nil && string(data) == "reply:hello" {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("forward did not transfer the request and half-closed response")
		}
		time.Sleep(25 * time.Millisecond)
	}
	large := strings.Repeat("payload-", 16000)
	conn, err := net.Dial("tcp", forwardAddr)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.WriteString(conn, large); err != nil {
		t.Fatal(err)
	}
	if err := conn.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	responseData, err := io.ReadAll(conn)
	_ = conn.Close()
	if err != nil || string(responseData) != "reply:"+large {
		t.Fatalf("large transfer: received %d bytes, error %v", len(responseData), err)
	}

	for _, test := range []struct {
		name, node, token, to string
		status                int
	}{
		{"bad token", "a", "wrong-token", "b/echo", http.StatusUnauthorized},
		{"not allowed", "b", tokenB, "b/echo", http.StatusForbidden},
		{"unknown service", "a", tokenA, "b/other", http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			head := http.Header{}
			head.Set(nodeHeader, test.node)
			nonce, _ := newNonce()
			head.Set(nonceHeader, base64.RawURLEncoding.EncodeToString(nonce))
			head.Set(roleHeader, "source")
			head.Set(toHeader, test.to)
			head.Set("Authorization", "Network-HMAC "+connectionProof(sha256.Sum256([]byte(test.token)), "webhook-network-agent-proof-v1", nonce, test.node, DataPath, "source", test.to, ""))
			conn, response, err := websocket.DefaultDialer.Dial(serverURL+DataPath, head)
			if conn != nil {
				_ = conn.Close()
			}
			if err == nil || response == nil || response.StatusCode != test.status {
				t.Fatalf("response = %v, err = %v; want HTTP %d", response, err, test.status)
			}
			_ = response.Body.Close()
		})
	}
}

func TestHubRejectsMissingOrMalformedNonce(t *testing.T) {
	token := strings.Repeat("s", 64)
	hub := NewHub(&HubConfig{MaxSessions: 1, Nodes: []NodePolicy{{ID: "node", tokenHash: sha256.Sum256([]byte(token))}}})
	defer hub.Close()
	server := httptest.NewServer(hub)
	defer server.Close()
	for _, nonce := range []string{"", "invalid", base64.RawURLEncoding.EncodeToString(make([]byte, 31))} {
		head := http.Header{}
		head.Set(nodeHeader, "node")
		head.Set("Authorization", "Network-HMAC "+connectionProof(sha256.Sum256([]byte(token)), "webhook-network-agent-proof-v1", make([]byte, 32), "node", ControlPath, "", "", ""))
		head.Set(nonceHeader, nonce)
		conn, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+ControlPath, head)
		if conn != nil {
			_ = conn.Close()
		}
		if err == nil || response == nil || response.StatusCode != http.StatusBadRequest {
			t.Fatalf("nonce %q: response = %v, err = %v", nonce, response, err)
		}
		_ = response.Body.Close()
	}
}

func TestHubRejectsProofForAnotherRequest(t *testing.T) {
	token := strings.Repeat("s", 64)
	hub := NewHub(&HubConfig{MaxSessions: 1, Nodes: []NodePolicy{{ID: "node", tokenHash: sha256.Sum256([]byte(token))}}})
	defer hub.Close()
	server := httptest.NewServer(hub)
	defer server.Close()
	nonce := make([]byte, 32)
	head := http.Header{}
	head.Set(nodeHeader, "node")
	head.Set(nonceHeader, base64.RawURLEncoding.EncodeToString(nonce))
	head.Set(roleHeader, "source")
	head.Set("Authorization", "Network-HMAC "+connectionProof(sha256.Sum256([]byte(token)), "webhook-network-agent-proof-v1", nonce, "node", ControlPath, "", "", ""))
	conn, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+DataPath, head)
	if conn != nil {
		_ = conn.Close()
	}
	if err == nil || response == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("response = %v, err = %v; want unauthorized", response, err)
	}
	_ = response.Body.Close()
}

func TestAgentRejectsHubWithoutTokenProof(t *testing.T) {
	for _, proof := range []string{"", "incorrect"} {
		t.Run(proof, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				head := http.Header{}
				if proof != "" {
					head.Set(proofHeader, proof)
				}
				conn, err := upgrader.Upgrade(w, r, head)
				if err == nil {
					defer conn.Close()
					_ = conn.SetReadDeadline(time.Now().Add(time.Second))
					_, _, _ = conn.ReadMessage()
				}
			}))
			defer server.Close()
			agent, err := newAgent(&AgentConfig{
				Server: "ws" + strings.TrimPrefix(server.URL, "http"),
				ID:     "node", token: strings.Repeat("s", 64), MaxConnections: 1,
			})
			if err != nil {
				t.Fatal(err)
			}
			conn, err := agent.openWebSocket(context.Background(), ControlPath, "", "", "")
			if conn != nil {
				_ = conn.Close()
			}
			if err == nil || !strings.Contains(err.Error(), "hub authentication failed") {
				t.Fatalf("connection = %v, err = %v", conn, err)
			}
		})
	}
}

func TestHubProofBindsRequest(t *testing.T) {
	key := sha256.Sum256([]byte(strings.Repeat("s", 64)))
	nonce := make([]byte, 32)
	base := connectionProof(key, "webhook-network-hub-proof-v1", nonce, "node", ControlPath, "", "", "")
	for _, changed := range []string{
		connectionProof(key, "webhook-network-hub-proof-v1", append([]byte{1}, nonce[1:]...), "node", ControlPath, "", "", ""),
		connectionProof(key, "webhook-network-hub-proof-v1", nonce, "other", ControlPath, "", "", ""),
		connectionProof(key, "webhook-network-hub-proof-v1", nonce, "node", DataPath, "", "", ""),
		connectionProof(key, "webhook-network-hub-proof-v1", nonce, "node", ControlPath, "source", "", ""),
		connectionProof(key, "webhook-network-hub-proof-v1", nonce, "node", ControlPath, "", "other/service", ""),
		connectionProof(key, "webhook-network-hub-proof-v1", nonce, "node", ControlPath, "", "", "session"),
		connectionProof(key, "webhook-network-agent-proof-v1", nonce, "node", ControlPath, "", "", ""),
	} {
		if changed == base {
			t.Fatal("proof did not bind the connection request")
		}
	}
}

func TestNetworkConfigRejectsPublicForward(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "node.token"), []byte(strings.Repeat("x", 64)), 0600); err != nil {
		t.Fatal(err)
	}
	path := writeNetworkConfig(t, dir, "agent.yaml", `mode: agent
agent:
  server: wss://hub.example.test
  id: node
  token_file: node.token
  forwards:
    - listen: 0.0.0.0:10022
      to: other/ssh
`)
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("expected a public forward listen to be rejected")
	}
}

func TestNetworkConfigWebhookOnlyViaHub(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "node.token"), []byte(strings.Repeat("x", 64)), 0600); err != nil {
		t.Fatal(err)
	}
	base := `agent:
  server: wss://hub.example.test
  id: node
  token_file: node.token
  webhook_only_via_hub: true
`
	for _, test := range []struct {
		name, prefix, extra string
		valid               bool
	}{
		{"agent only", "mode: agent\n", "  with_webhook: true\n", true},
		{"requires webhook", "mode: agent\n", "", false},
		{"cannot set local target", "mode: agent\n", "  with_webhook: true\n  services:\n    - name: webhook\n      target: 127.0.0.1:9000\n", false},
		{"cannot run hub", "mode: both\nhub:\n  nodes:\n    - id: node\n      token_file: node.token\n", "  with_webhook: true\n", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := writeNetworkConfig(t, dir, test.name+".yaml", test.prefix+base+test.extra)
			cfg, err := LoadConfig(path)
			if (err == nil) != test.valid {
				t.Fatalf("LoadConfig error = %v, valid = %v", err, test.valid)
			}
			if test.valid && (len(cfg.Agent.Services) != 1 || cfg.Agent.Services[0].Name != "webhook" || cfg.Agent.Services[0].Target != "") {
				t.Fatalf("in-process webhook service was not registered: %+v", cfg.Agent.Services)
			}
		})
	}
}

func TestHubHookRoutingFiltersAndBalances(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []string{"a", "b"} {
		if err := os.WriteFile(filepath.Join(dir, id+".token"), []byte(strings.Repeat(id, 64)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	backend := func(id string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/hooks/deploy" || r.URL.RawQuery != "key=value" ||
				r.Host != "public.example.test" || r.Header.Get("X-Test-Business") != "preserved" ||
				r.Header.Get(ClientIPHeader) != "127.0.0.1" || r.Header.Get(OriginalURIHeader) != "/public/deploy?key=value" ||
				r.Header.Get("X-Forwarded-For") != "127.0.0.1" || r.Header.Get("X-Real-Ip") != "127.0.0.1" ||
				r.Header.Get("X-Forwarded-Proto") != "http" || r.Header.Get("X-Forwarded-Host") != "public.example.test" ||
				r.Header.Get("Forwarded") != "" || r.Header.Get("X-Hop-Request") != "" || r.Header.Get("Proxy-Authorization") != "" {
				http.Error(w, "wrong forwarded request", http.StatusBadRequest)
				return
			}
			body, _ := io.ReadAll(r.Body)
			if r.Header.Get("X-Test-Slow") == "1" {
				<-r.Context().Done()
				return
			}
			w.Header().Set("Proxy-Authenticate", "remove")
			if r.Header.Get("X-Test-Trailer") == "1" {
				w.Header().Set("Trailer", "X-Result")
				_, _ = io.WriteString(w, id+":"+string(body))
				w.Header().Set("X-Result", "preserved")
				return
			}
			if r.Header.Get("X-Test-Gzip") == "1" {
				w.Header().Set("Content-Encoding", "gzip")
				compressed := gzip.NewWriter(w)
				_, _ = io.WriteString(compressed, id+":"+string(body))
				_ = compressed.Close()
				return
			}
			_, _ = io.WriteString(w, id+":"+string(body))
		}))
	}
	backendA, backendB := backend("a"), backend("b")
	defer backendA.Close()
	defer backendB.Close()
	hubPath := writeNetworkConfig(t, dir, "hub-routes.yaml", `mode: hub
hub:
  request_timeout: 1s
  nodes:
    - id: a
      token_file: a.token
      services: [webhook]
    - id: b
      token_file: b.token
      services: [webhook]
  hook_routes:
    - path: /public/deploy
      hook: deploy
      agents: [a, b]
      allow_ips: [127.0.0.0/8]
`)
	hubCfg, err := LoadConfig(hubPath)
	if err != nil {
		t.Fatal(err)
	}
	hub := NewHub(hubCfg.Hub)
	defer hub.Close()
	mux := http.NewServeMux()
	mux.HandleFunc(ControlPath, hub.ServeHTTP)
	mux.HandleFunc(DataPath, hub.ServeHTTP)
	mux.HandleFunc("/public/deploy", func(w http.ResponseWriter, r *http.Request) {
		hub.ServeHook(w, r, RequestOrigin{ClientIP: "127.0.0.1", Scheme: "http"}, 1<<20)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	serverURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider := func() []HookInfo {
		return []HookInfo{{ID: "deploy", Methods: []string{"POST"}, IPRequired: []string{"127.0.0.0/8"}}}
	}
	for _, entry := range []struct{ id, target string }{{"a", backendA.Listener.Addr().String()}, {"b", backendB.Listener.Addr().String()}} {
		path := writeNetworkConfig(t, dir, entry.id+"-routes.yaml", fmt.Sprintf(`mode: agent
agent:
  server: %s
  id: %s
  token_file: %s.token
  services:
    - name: webhook
      target: %s
`, serverURL, entry.id, entry.id, entry.target))
		cfg, err := LoadConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- RunAgent(ctx, cfg.Agent, AgentOptions{Hooks: provider}) }()
		defer func() {
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Errorf("agent stopped: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Error("agent did not stop")
			}
		}()
	}
	request := func(method string) (int, string, error) {
		req, err := http.NewRequest(method, server.URL+"/public/deploy?key=value", strings.NewReader("payload"))
		if err != nil {
			return 0, "", err
		}
		req.Host = "public.example.test"
		req.Header.Set("X-Test-Business", "preserved")
		req.Header.Set(ClientIPHeader, "203.0.113.66")
		req.Header.Set(OriginalURIHeader, "/forged")
		req.Header.Set("X-Forwarded-For", "203.0.113.66")
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("Forwarded", "for=203.0.113.66")
		req.Header.Set("Connection", "X-Hop-Request")
		req.Header.Set("X-Hop-Request", "remove")
		req.Header.Set("Proxy-Authorization", "remove")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return 0, "", err
		}
		defer res.Body.Close()
		if res.Header.Get("Proxy-Authenticate") != "" {
			t.Error("response hop header leaked")
		}
		body, err := io.ReadAll(res.Body)
		return res.StatusCode, string(body), err
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		status, _, err := request("POST")
		if err == nil && status == http.StatusOK {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("hook route unavailable: status %d, err %v", status, err)
		}
		time.Sleep(25 * time.Millisecond)
	}
	seen := make(map[string]bool)
	for i := 0; i < 60; i++ {
		status, body, err := request("POST")
		if err != nil || status != http.StatusOK {
			t.Fatalf("request %d: status %d, body %q, err %v", i, status, body, err)
		}
		seen[body] = true
	}
	if !seen["a:payload"] || !seen["b:payload"] {
		t.Fatalf("both agents were not selected: %v", seen)
	}
	compressedRequest, err := http.NewRequest(http.MethodPost, server.URL+"/public/deploy?key=value", strings.NewReader("payload"))
	if err != nil {
		t.Fatal(err)
	}
	compressedRequest.Host = "public.example.test"
	compressedRequest.Header.Set("X-Test-Business", "preserved")
	compressedRequest.Header.Set("X-Test-Gzip", "1")
	response, err := (&http.Client{Transport: &http.Transport{DisableCompression: true}}).Do(compressedRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("Content-Encoding = %q", response.Header.Get("Content-Encoding"))
	}
	decompressed, err := gzip.NewReader(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	compressedBody, err := io.ReadAll(decompressed)
	_ = decompressed.Close()
	if err != nil || !seen[string(compressedBody)] {
		t.Fatalf("compressed response = %q, err = %v", compressedBody, err)
	}
	trailerRequest, err := http.NewRequest(http.MethodPost, server.URL+"/public/deploy?key=value", strings.NewReader("payload"))
	if err != nil {
		t.Fatal(err)
	}
	trailerRequest.Host = "public.example.test"
	trailerRequest.Header.Set("X-Test-Business", "preserved")
	trailerRequest.Header.Set("X-Test-Trailer", "1")
	trailerResponse, err := http.DefaultClient.Do(trailerRequest)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.Copy(io.Discard, trailerResponse.Body)
	_ = trailerResponse.Body.Close()
	if err != nil || trailerResponse.Trailer.Get("X-Result") != "preserved" {
		t.Fatalf("response trailer = %q, err = %v", trailerResponse.Trailer.Get("X-Result"), err)
	}
	slowRequest, _ := http.NewRequest(http.MethodPost, server.URL+"/public/deploy?key=value", nil)
	slowRequest.Host = "public.example.test"
	slowRequest.Header.Set("X-Test-Business", "preserved")
	slowRequest.Header.Set("X-Test-Slow", "1")
	slowResponse, err := (&http.Client{Timeout: 3 * time.Second}).Do(slowRequest)
	if err != nil {
		t.Fatal(err)
	}
	slowResponse.Body.Close()
	if slowResponse.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("slow request: %d", slowResponse.StatusCode)
	}
	deadline = time.Now().Add(time.Second)
	for {
		hub.mu.Lock()
		remaining := len(hub.sessions)
		hub.mu.Unlock()
		if remaining == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout leaked %d sessions", remaining)
		}
		time.Sleep(10 * time.Millisecond)
	}
	status, _, err := request("GET")
	if err != nil || status != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, err = %v", status, err)
	}
	req := httptest.NewRequest(http.MethodPost, "/public/deploy", strings.NewReader("payload"))
	rec := httptest.NewRecorder()
	hub.ServeHook(rec, req, RequestOrigin{ClientIP: "203.0.113.10", Scheme: "http"}, 1<<20)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("disallowed IP status = %d", rec.Code)
	}
}

func TestCascadedAgentService(t *testing.T) {
	hookBackend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hooks/deploy" || r.URL.RawQuery != "via=parent" ||
			r.Header.Get(ClientIPHeader) != "198.51.100.24" ||
			r.Header.Get(OriginalURIHeader) != "/public/deploy?via=parent" ||
			r.Header.Get("X-Forwarded-For") != "198.51.100.24" ||
			r.Header.Get("X-Test-Business") != "cascade" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, "child hook")
	})
	target, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	go func() {
		for {
			conn, err := target.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				body, _ := io.ReadAll(conn)
				_, _ = io.WriteString(conn, "cascade:"+string(body))
			}()
		}
	}()
	dir := t.TempDir()
	for _, id := range []string{"mid", "child"} {
		if err := os.WriteFile(filepath.Join(dir, id+".token"), []byte(strings.Repeat(id, 32)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	rootPath := writeNetworkConfig(t, dir, "root.yaml", `mode: hub
hub:
  nodes:
    - id: mid
      token_file: mid.token
      services: [echo, webhook]
  hook_routes:
    - path: /public/deploy
      hook: deploy
      agents: [mid]
`)
	rootCfg, err := LoadConfig(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	rootHub := NewHub(rootCfg.Hub)
	defer rootHub.Close()
	rootServer := httptest.NewServer(rootHub)
	defer rootServer.Close()
	rootURL := "ws" + strings.TrimPrefix(rootServer.URL, "http")
	middlePath := writeNetworkConfig(t, dir, "middle.yaml", fmt.Sprintf(`mode: both
hub:
  nodes:
    - id: child
      token_file: child.token
      services: [echo, webhook]
agent:
  server: %s
  id: mid
  token_file: mid.token
  services:
    - name: echo
      relay_to: child/echo
    - name: webhook
      relay_to: child/webhook
`, rootURL))
	middleCfg, err := LoadConfig(middlePath)
	if err != nil {
		t.Fatal(err)
	}
	middleHub := NewHub(middleCfg.Hub)
	defer middleHub.Close()
	middleServer := httptest.NewServer(middleHub)
	defer middleServer.Close()
	middleURL := "ws" + strings.TrimPrefix(middleServer.URL, "http")
	childPath := writeNetworkConfig(t, dir, "child.yaml", fmt.Sprintf(`mode: agent
agent:
  server: %s
  id: child
  token_file: child.token
  with_webhook: true
  webhook_only_via_hub: true
  services:
    - name: echo
      target: %s
`, middleURL, target.Addr()))
	childCfg, err := LoadConfig(childPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	childDone := make(chan error, 1)
	middleDone := make(chan error, 1)
	go func() {
		childDone <- RunAgent(ctx, childCfg.Agent, AgentOptions{Hooks: func() []HookInfo {
			return []HookInfo{{ID: "deploy", Methods: []string{"POST"}}}
		}, WebhookHandler: hookBackend})
	}()
	go func() { middleDone <- RunAgent(ctx, middleCfg.Agent, AgentOptions{Downstream: middleHub}) }()
	defer func() {
		cancel()
		for _, done := range []chan error{middleDone, childDone} {
			select {
			case err := <-done:
				if err != nil {
					t.Errorf("agent stopped: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Error("agent did not stop")
			}
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		middleHub.mu.Lock()
		childReady := middleHub.controls["child"] != nil
		middleHub.mu.Unlock()
		rootHub.mu.Lock()
		middleReady := rootHub.controls["mid"] != nil
		rootHub.mu.Unlock()
		if childReady && middleReady {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("cascade nodes did not connect")
		}
		time.Sleep(25 * time.Millisecond)
	}
	dialCtx, stop := context.WithTimeout(ctx, 5*time.Second)
	defer stop()
	conn, err := rootHub.DialService(dialCtx, "mid", "echo")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "hello"); err != nil {
		t.Fatal(err)
	}
	if err := conn.(interface{ CloseWrite() error }).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	result, err := io.ReadAll(conn)
	if err != nil || string(result) != "cascade:hello" {
		t.Fatalf("cascade response = %q, err = %v", result, err)
	}
	hookDeadline := time.Now().Add(5 * time.Second)
	for {
		req := httptest.NewRequest(http.MethodPost, "/public/deploy?via=parent", nil)
		req.Header.Set("X-Test-Business", "cascade")
		rec := httptest.NewRecorder()
		rootHub.ServeHook(rec, req, RequestOrigin{ClientIP: "198.51.100.24", Scheme: "http"}, 1<<20)
		if rec.Code == http.StatusOK && rec.Body.String() == "child hook" {
			break
		}
		if time.Now().After(hookDeadline) {
			t.Fatalf("cascaded hook route returned %d: %s", rec.Code, rec.Body.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func writeNetworkConfig(t *testing.T, dir, name, data string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPipeHalfCloseAndDeadline(t *testing.T) {
	a, b := pipePair()
	defer a.Close()
	defer b.Close()
	go func() {
		_, _ = io.WriteString(a, "request")
		_ = a.(interface{ CloseWrite() error }).CloseWrite()
	}()
	data, err := io.ReadAll(b)
	if err != nil || string(data) != "request" {
		t.Fatalf("request = %q, err = %v", data, err)
	}
	go func() { _, _ = io.WriteString(b, "response") }()
	response := make([]byte, len("response"))
	if _, err := io.ReadFull(a, response); err != nil || string(response) != "response" {
		t.Fatalf("response = %q, err = %v", response, err)
	}
	_ = a.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
	_, err = a.Read(make([]byte, 1))
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("deadline error = %v", err)
	}
}

func TestWebhookCannotBeGrantedAsRawService(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "node.token"), []byte(strings.Repeat("x", 64)), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := ParseConfig([]byte(`mode: hub
hub:
  nodes:
    - id: node
      token_file: node.token
      services: [webhook]
      allow: [node/webhook]
`), dir)
	if err == nil || !strings.Contains(err.Error(), "webhook cannot be granted") {
		t.Fatalf("raw webhook grant accepted: %v", err)
	}
}

func TestRemoveConnectionNominatedHeaders(t *testing.T) {
	header := http.Header{}
	header.Add("Connection", "keep-alive, X-Hop")
	header.Add("Connection", "X-Other")
	header.Set("X-Hop", "remove")
	header.Set("X-Other", "remove")
	header.Set("X-Business", "keep")
	removeHopHeaders(header)
	if len(header) != 1 || header.Get("X-Business") != "keep" {
		t.Fatalf("unexpected headers: %v", header)
	}
}
