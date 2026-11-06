package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/adnanh/webhook/internal/network"
)

func TestWebhookAutoServiceUsesInProcessWithTLSOrSocket(t *testing.T) {
	savedSecure, savedSocket := *secure, socket
	defer func() { *secure = savedSecure; socket = savedSocket }()
	for _, test := range []struct {
		tls    bool
		socket string
	}{{false, ""}, {true, ""}, {false, "/unused.sock"}} {
		*secure = test.tls
		socket = test.socket
		cfg := &network.AgentConfig{WithWebhook: true}
		if err := prepareWebhookAgent(cfg); err != nil {
			t.Fatal(err)
		}
		if len(cfg.Services) != 1 || !cfg.Services[0].InProcess || cfg.Services[0].Target != "" {
			t.Fatalf("implicit TCP target remains: %+v", cfg.Services)
		}
	}
	cfg := &network.AgentConfig{WithWebhook: true, Services: []network.Service{{Name: "webhook", Target: "127.0.0.1:9000"}}}
	if err := prepareWebhookAgent(cfg); err != nil || cfg.Services[0].InProcess {
		t.Fatal("explicit service target changed")
	}
}

func TestNetworkClientIPContextOverridesProxyHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/hooks/deploy", nil)
	req.Header.Set("X-Real-IP", "203.0.113.66")
	req = req.WithContext(context.WithValue(req.Context(), networkClientIPKey{}, "198.51.100.24"))
	if got := resolveRealIP(req); got != "198.51.100.24" {
		t.Fatalf("network origin lost: %s", got)
	}
}
