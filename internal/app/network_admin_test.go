package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adnanh/webhook/internal/network"
	"github.com/gorilla/mux"
)

func TestNetworkAdminAPIAuthentication(t *testing.T) {
	restore := setupAdminTestState(t)
	defer restore()
	hub := network.NewHub(&network.HubConfig{MaxSessions: 4, Nodes: []network.NodePolicy{{ID: "node", Token: "secret-that-must-not-appear", TokenFile: "private-token-path"}}})
	defer hub.Close()
	*adminEnabled = true
	router := mux.NewRouter()
	registerNetworkAdminRoutes(router, hub)
	cookie := loginCookieForTest(t)
	for _, path := range []string{"/admin/api/network/nodes", "/admin/api/network/nodes/node", "/admin/api/network/nodes/missing"} {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated %s: %d", path, w.Code)
		}
		req = httptest.NewRequest("GET", path, nil)
		req.AddCookie(cookie)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		expected := 200
		if strings.HasSuffix(path, "missing") {
			expected = 404
		}
		if w.Code != expected {
			t.Fatalf("authenticated %s: %d %s", path, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "secret-that-must-not-appear") || strings.Contains(w.Body.String(), "private-token-path") {
			t.Fatal("credential leaked")
		}
		if expected == 200 && w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("management response is cacheable")
		}
	}
	*adminEnabled = false
	router = mux.NewRouter()
	registerNetworkAdminRoutes(router, hub)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/admin/api/network/nodes", nil))
	if w.Code != 404 {
		t.Fatal("disabled admin API registered")
	}
}
