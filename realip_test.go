package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInitAccessControl(t *testing.T) {
	savedWhitelist, savedBlacklist := *accessWhitelist, *accessBlacklist
	savedWL, savedBL := parsedAccessWhitelist, parsedAccessBlacklist
	defer func() {
		*accessWhitelist, *accessBlacklist = savedWhitelist, savedBlacklist
		parsedAccessWhitelist, parsedAccessBlacklist = savedWL, savedBL
	}()

	*accessWhitelist, *accessBlacklist = "10.0.0.0/8", ""
	if err := initAccessControl(); err != nil {
		t.Fatal(err)
	}
	if len(parsedAccessWhitelist) != 1 {
		t.Fatalf("whitelist entries = %d, want 1", len(parsedAccessWhitelist))
	}

	*accessWhitelist, *accessBlacklist = "10.0.0.1", "192.0.2.1"
	if err := initAccessControl(); err == nil {
		t.Fatal("expected whitelist/blacklist conflict")
	}
}

func TestAccessControlMiddleware(t *testing.T) {
	savedWhitelist, savedBlacklist := *accessWhitelist, *accessBlacklist
	savedWL, savedBL := parsedAccessWhitelist, parsedAccessBlacklist
	defer func() {
		*accessWhitelist, *accessBlacklist = savedWhitelist, savedBlacklist
		parsedAccessWhitelist, parsedAccessBlacklist = savedWL, savedBL
	}()

	*accessWhitelist, *accessBlacklist = "192.0.2.0/24", ""
	if err := initAccessControl(); err != nil {
		t.Fatal(err)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := accessControlMiddleware(next)

	allowed := httptest.NewRequest(http.MethodGet, "/", nil)
	allowed.RemoteAddr = "192.0.2.10:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, allowed)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("allowed status = %d", rec.Code)
	}

	denied := httptest.NewRequest(http.MethodGet, "/", nil)
	denied.RemoteAddr = "198.51.100.10:1234"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, denied)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("denied status = %d, want 403", rec.Code)
	}
}

func TestResolveRealIPOnlyTrustedProxy(t *testing.T) {
	savedHeader, savedTrusted := *realIPHeader, parsedTrustedProxies
	defer func() { *realIPHeader, parsedTrustedProxies = savedHeader, savedTrusted }()
	*realIPHeader = "X-Real-IP"
	parsedTrustedProxies, _ = parseIPNetworks("127.0.0.1", "trusted-proxies")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:80"
	req.Header.Set("X-Real-IP", "203.0.113.7")
	if got := resolveRealIP(req); got != "203.0.113.7" {
		t.Fatalf("trusted real IP = %q", got)
	}

	req.RemoteAddr = "198.51.100.1:80"
	if got := resolveRealIP(req); got != req.RemoteAddr {
		t.Fatalf("untrusted real IP = %q, want %q", got, req.RemoteAddr)
	}
}
