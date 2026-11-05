package network

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNodeStatusLifecycle(t *testing.T) {
	token := strings.Repeat("t", 64)
	hub := NewHub(&HubConfig{MaxSessions: 4, Nodes: []NodePolicy{{ID: "temp", tokenHash: sha256.Sum256([]byte(token)), Services: []string{"files"}}, {ID: "unused"}}})
	defer hub.Close()
	server := httptest.NewServer(hub)
	defer server.Close()
	cfg := &AgentConfig{Server: "ws" + strings.TrimPrefix(server.URL, "http"), ID: "temp", token: token, MaxConnections: 4, Services: []Service{{Name: "files", InProcess: true}}}
	expiry := time.Now().Add(time.Hour).UTC()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- RunAgent(ctx, cfg, AgentOptions{Version: "test-1.2.3", Temporary: true, ExpiresAt: &expiry, Handlers: map[string]http.Handler{"files": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})}})
	}()
	wait := func(status string) NodeStatus {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			snapshot := hub.Status()
			node := snapshot.Nodes[0]
			if node.Status == status {
				return node
			}
			if time.Now().After(deadline) {
				t.Fatalf("expected %s: %+v", status, node)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	online := wait("online")
	if online.Version != "test-1.2.3" || online.OS == "" || online.Arch == "" || !online.Temporary || online.ExpiresAt == nil || !online.ExpiresAt.Equal(expiry) || online.ConnectedAt == nil || online.LastSeenAt == nil || len(online.Services) != 1 {
		t.Fatalf("missing metadata: %+v", online)
	}
	*online.ExpiresAt = time.Time{}
	*online.ConnectedAt = time.Time{}
	if hub.Status().Nodes[0].ConnectedAt.IsZero() || hub.Status().Nodes[0].ExpiresAt.IsZero() {
		t.Fatal("snapshot mutates hub state")
	}
	conn, err := hub.DialService(ctx, "temp", "files")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot := hub.Status(); snapshot.ActiveSessions != 1 || snapshot.Nodes[0].ActiveSessions != 1 {
		t.Fatalf("session not counted: %+v", snapshot)
	}
	conn.Close()
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	offline := wait("offline")
	if offline.DisconnectedAt == nil || offline.Version != "test-1.2.3" || len(offline.Services) != 0 {
		t.Fatalf("invalid offline metadata: %+v", offline)
	}
	if hub.Status().Nodes[1].Status != "never_connected" {
		t.Fatal("unused node status lost")
	}
}

func TestIdleAgentAndMetadataValidation(t *testing.T) {
	cfg := &AgentConfig{Server: "ws://127.0.0.1:9000", ID: "temp", Token: strings.Repeat("t", 64)}
	if err := ValidateAgent(cfg); err != nil {
		t.Fatalf("idle CLI join rejected: %v", err)
	}
	if validAgentInfo(AgentInfo{Version: strings.Repeat("v", 129)}) || validAgentInfo(AgentInfo{OS: "bad\nvalue"}) {
		t.Fatal("invalid metadata accepted")
	}
}
