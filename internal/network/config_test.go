package network

import (
	"strings"
	"testing"
)

func TestAgentServerBaseURL(t *testing.T) {
	for _, test := range []struct {
		server, want string
	}{
		{"wss://hub.example.com", "wss://hub.example.com"},
		{"wss://hub.example.com/", "wss://hub.example.com"},
		{"wss://hub.example.com/wh1", "wss://hub.example.com/wh1"},
		{"wss://hub.example.com/wh1/", "wss://hub.example.com/wh1"},
		{"wss://hub.example.com/team/wh1/", "wss://hub.example.com/team/wh1"},
		{"wss://hub.example.com/custom-hub/", "wss://hub.example.com/custom-hub"},
		{"wss://hub.example.com/teams/project-a/hub/", "wss://hub.example.com/teams/project-a/hub"},
		{"ws://127.0.0.1:19001/wh1/", "ws://127.0.0.1:19001/wh1"},
		{"ws://hub.example.com/wh1/", ""},
		{"https://hub.example.com/wh1/", ""},
		{"/wh1/", ""},
		{"wss://user:pass@hub.example.com/wh1/", ""},
		{"wss://hub.example.com/wh1/?key=value", ""},
		{"wss://hub.example.com/wh1/?", ""},
		{"wss://hub.example.com/wh1/#fragment", ""},
		{"wss://hub.example.com/wh1/#", ""},
		{"wss://hub.example.com/%zz", ""},
	} {
		t.Run(test.server, func(t *testing.T) {
			cfg := &AgentConfig{Server: test.server, ID: "node", Token: strings.Repeat("x", 64)}
			err := ValidateAgent(cfg)
			if test.want == "" {
				if err == nil {
					t.Fatal("invalid server accepted")
				}
				return
			}
			if err != nil || cfg.Server != test.want {
				t.Fatalf("server = %q, error = %v; want %q", cfg.Server, err, test.want)
			}
		})
	}
}
