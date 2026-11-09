package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/xtulnx/webhook/internal/network"
)

func TestHTTPRouteGroups(t *testing.T) {
	restore := setupAdminTestState(t)
	defer restore()
	savedHooks, savedStatus, savedAdmin := *hooksURLPrefix, *statusURLPrefix, *adminURLPrefix
	savedWhitelist, savedBlacklist := parsedAccessWhitelist, parsedAccessBlacklist
	defer func() {
		*hooksURLPrefix, *statusURLPrefix, *adminURLPrefix = savedHooks, savedStatus, savedAdmin
		parsedAccessWhitelist, parsedAccessBlacklist = savedWhitelist, savedBlacklist
	}()
	*hooksURLPrefix, *statusURLPrefix, *adminURLPrefix = "hooks", "status", "wh1/admin"
	parsedAccessWhitelist, parsedAccessBlacklist = nil, nil
	*adminEnabled = true
	currentAdminAuth.basePath = "/wh1/admin"
	hub := network.NewHub(&network.HubConfig{
		Nodes: []network.NodePolicy{{ID: "node", Services: []string{"webhook"}}},
		Routes: []network.HookRoute{
			{Path: "/ingress/deploy", Hook: "deploy", Agents: []string{"node"}},
			{Path: "/hooks/remote", Hook: "deploy", Agents: []string{"node"}},
		},
	})
	defer hub.Close()
	handler := newHTTPHandler(hub)
	for _, test := range []struct {
		path   string
		status int
		body   string
	}{
		{"/", 200, "OK"},
		{"/status", 200, `"status"`},
		{network.ControlPath, 401, "unauthorized"},
		{network.DataPath, 401, "unauthorized"},
		{"/ingress/deploy", 503, ""},
		{"/hooks/remote", 503, ""}, // Explicit Hub route wins over local hook matching.
		{"/hooks/missing", 404, "Hook not found."},
		{"/wh1/admin/", 200, `/wh1/admin/assets/app.js`},
		{"/wh1/admin/assets/app.js", 200, ""},
		{"/wh1/admin/api/network/nodes", 401, ""},
		{"/admin/", 404, ""},
		{"/missing", 404, ""},
	} {
		t.Run(test.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, test.path, nil))
			if w.Code != test.status || !strings.Contains(w.Body.String(), test.body) {
				t.Fatalf("status %d, body %q; want status %d containing %q", w.Code, w.Body.String(), test.status, test.body)
			}
		})
	}

	code := hotpCode(currentAdminAuth.totpSecret, uint64(time.Now().Unix()/30), 6)
	req := httptest.NewRequest(http.MethodPost, "/wh1/admin/api/auth/login", strings.NewReader(`{"code":"`+code+`"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK || len(w.Result().Cookies()) == 0 {
		t.Fatalf("prefixed login failed: %d %s", w.Code, w.Body.String())
	}
	cookie := w.Result().Cookies()[0]
	if cookie.Path != "/wh1/admin" {
		t.Fatalf("session cookie path = %q", cookie.Path)
	}
	req = httptest.NewRequest(http.MethodGet, "/wh1/admin/api/network/nodes", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("prefixed management API: %d %s", w.Code, w.Body.String())
	}

	// HTTP IP restrictions continue to apply to all HTTP route groups, while
	// transport endpoints keep their independent node authentication.
	var err error
	parsedAccessBlacklist, err = parseIPNetworks("192.0.2.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", "/status", "/ingress/deploy", "/hooks/missing", "/wh1/admin/", network.ControlPath, network.DataPath} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "192.0.2.1:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		want := http.StatusForbidden
		if hub.Handles(path) {
			want = http.StatusUnauthorized
		}
		if w.Code != want {
			t.Errorf("restricted %s: status %d, want %d", path, w.Code, want)
		}
	}
}
