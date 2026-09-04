package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adnanh/webhook/internal/hook"
	"github.com/gorilla/mux"
)

const testAdminBase32Secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

func setupAdminTestState(t *testing.T) func() {
	t.Helper()

	savedSecure := *secure
	savedAsTemplate := *asTemplate
	savedAdminEnabled := *adminEnabled
	savedAdminAuth := currentAdminAuth

	loadedHooksMu.RLock()
	savedHooksFiles := append(hook.HooksFiles(nil), hooksFiles...)
	savedLoaded := cloneLoadedHooksMapLocked()
	loadedHooksMu.RUnlock()

	decodedSecret, err := decodeTOTPSecret(testAdminBase32Secret)
	if err != nil {
		t.Fatalf("decode TOTP secret: %v", err)
	}

	currentAdminAuth = &adminAuthConfig{
		basePath:   "/admin",
		totpSecret: decodedSecret,
		jwtSecret:  []byte("test-jwt-secret"),
		sessionTTL: time.Hour,
	}

	*secure = false
	*asTemplate = false
	*adminEnabled = false

	loadedHooksMu.Lock()
	loadedHooksFromFiles = make(map[string]hook.Hooks)
	hooksFiles = nil
	loadedHooksMu.Unlock()

	return func() {
		currentAdminAuth = savedAdminAuth
		*secure = savedSecure
		*asTemplate = savedAsTemplate
		*adminEnabled = savedAdminEnabled

		loadedHooksMu.Lock()
		loadedHooksFromFiles = savedLoaded
		hooksFiles = savedHooksFiles
		loadedHooksMu.Unlock()
	}
}

func setLoadedHooksForTest(path string, hooksInFile hook.Hooks) {
	loadedHooksMu.Lock()
	defer loadedHooksMu.Unlock()

	loadedHooksFromFiles = map[string]hook.Hooks{
		path: cloneHooks(hooksInFile),
	}
	hooksFiles = hook.HooksFiles{path}
}

func readHooksFileForTest(t *testing.T, path string) hook.Hooks {
	t.Helper()

	var hooksInFile hook.Hooks
	if err := hooksInFile.LoadFromFile(path, false); err != nil {
		t.Fatalf("load hooks file %s: %v", path, err)
	}

	return hooksInFile
}

func loginCookieForTest(t *testing.T) *http.Cookie {
	t.Helper()

	code := hotpCode(currentAdminAuth.totpSecret, uint64(time.Now().Unix()/30), 6)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/auth/login", strings.NewReader(`{"code":"`+code+`"}`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	adminLoginHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == adminTokenCookieName {
			return cookie
		}
	}

	t.Fatal("admin login did not issue session cookie")
	return nil
}

func TestVerifyTOTP(t *testing.T) {
	secret, err := decodeTOTPSecret(testAdminBase32Secret)
	if err != nil {
		t.Fatalf("decode TOTP secret: %v", err)
	}

	if got := hotpCode(secret, 1, 6); got != "287082" {
		t.Fatalf("hotpCode(...) = %q, want %q", got, "287082")
	}

	if !verifyTOTP(secret, "287082", time.Unix(59, 0)) {
		t.Fatal("verifyTOTP should accept the RFC test vector")
	}

	if verifyTOTP(secret, "287083", time.Unix(59, 0)) {
		t.Fatal("verifyTOTP should reject an invalid code")
	}
}

func TestAdminJWTRoundTrip(t *testing.T) {
	restore := setupAdminTestState(t)
	defer restore()

	now := time.Unix(1700000000, 0)
	token, err := signAdminJWT(now)
	if err != nil {
		t.Fatalf("signAdminJWT: %v", err)
	}

	claims, err := parseAdminJWT(token, now.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("parseAdminJWT: %v", err)
	}

	if claims.Subject != "webhook-admin" {
		t.Fatalf("claims.Subject = %q, want %q", claims.Subject, "webhook-admin")
	}

	if _, err := parseAdminJWT(token, now.Add(2*time.Hour)); err == nil {
		t.Fatal("parseAdminJWT should reject expired tokens")
	}
}

func TestAdminConfigRequiresAuth(t *testing.T) {
	restore := setupAdminTestState(t)
	defer restore()

	handler := adminRequireAuth(adminConfigHandler)
	req := httptest.NewRequest(http.MethodGet, "/admin/api/config", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAdminCRUDHandlers(t *testing.T) {
	restore := setupAdminTestState(t)
	defer restore()

	tmpDir := t.TempDir()
	hooksPath := filepath.Join(tmpDir, "hooks.json")

	initialHooks := hook.Hooks{
		{
			ID:              "deploy",
			ExecuteCommand:  "/bin/echo",
			ResponseMessage: "ok",
			HTTPMethods:     []string{"POST"},
		},
	}

	if err := writeHooksFile(hooksPath, initialHooks); err != nil {
		t.Fatalf("writeHooksFile: %v", err)
	}
	setLoadedHooksForTest(hooksPath, initialHooks)

	cookie := loginCookieForTest(t)

	configReq := httptest.NewRequest(http.MethodGet, "/admin/api/config", nil)
	configReq.AddCookie(cookie)
	configRec := httptest.NewRecorder()
	adminRequireAuth(adminConfigHandler)(configRec, configReq)

	if configRec.Code != http.StatusOK {
		t.Fatalf("config status = %d, want %d", configRec.Code, http.StatusOK)
	}

	var config adminHooksState
	if err := json.Unmarshal(configRec.Body.Bytes(), &config); err != nil {
		t.Fatalf("decode config response: %v", err)
	}
	if len(config.Files) != 1 || len(config.Files[0].Hooks) != 1 {
		t.Fatalf("unexpected config payload: %+v", config)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/admin/api/hooks", strings.NewReader(`{
		"file": "`+hooksPath+`",
		"hook": {
			"id": "build",
			"execute-command": "/usr/bin/env",
			"response-message": "created",
			"http-methods": [" post ", "get"]
		}
	}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createRec := httptest.NewRecorder()
	adminRequireAuth(adminCreateHookHandler)(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d, body=%s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}

	hooksAfterCreate := readHooksFileForTest(t, hooksPath)
	if len(hooksAfterCreate) != 2 {
		t.Fatalf("hook count after create = %d, want %d", len(hooksAfterCreate), 2)
	}
	if hooksAfterCreate.Match("build") == nil {
		t.Fatal("expected created hook to be written to file")
	}
	if got := hooksAfterCreate.Match("build").HTTPMethods; len(got) != 2 || got[0] != "POST" || got[1] != "GET" {
		t.Fatalf("created hook methods = %#v, want %#v", got, []string{"POST", "GET"})
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/admin/api/hooks", strings.NewReader(`{
		"file": "`+hooksPath+`",
		"currentId": "build",
		"hook": {
			"id": "build-renamed",
			"execute-command": "/bin/echo",
			"response-message": "updated",
			"http-methods": ["PATCH"]
		}
	}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.AddCookie(cookie)
	updateRec := httptest.NewRecorder()
	adminRequireAuth(adminUpdateHookHandler)(updateRec, updateReq)

	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d, body=%s", updateRec.Code, http.StatusOK, updateRec.Body.String())
	}

	hooksAfterUpdate := readHooksFileForTest(t, hooksPath)
	if hooksAfterUpdate.Match("build") != nil {
		t.Fatal("expected old hook ID to be removed after rename")
	}

	renamed := hooksAfterUpdate.Match("build-renamed")
	if renamed == nil {
		t.Fatal("expected renamed hook to be persisted")
	}
	if renamed.ResponseMessage != "updated" {
		t.Fatalf("updated response message = %q, want %q", renamed.ResponseMessage, "updated")
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/admin/api/hooks", strings.NewReader(`{
		"file": "`+hooksPath+`",
		"currentId": "build-renamed"
	}`))
	deleteReq.Header.Set("Content-Type", "application/json")
	deleteReq.AddCookie(cookie)
	deleteRec := httptest.NewRecorder()
	adminRequireAuth(adminDeleteHookHandler)(deleteRec, deleteReq)

	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want %d, body=%s", deleteRec.Code, http.StatusOK, deleteRec.Body.String())
	}

	hooksAfterDelete := readHooksFileForTest(t, hooksPath)
	if len(hooksAfterDelete) != 1 {
		t.Fatalf("hook count after delete = %d, want %d", len(hooksAfterDelete), 1)
	}
	if hooksAfterDelete.Match("build-renamed") != nil {
		t.Fatal("expected deleted hook to be removed from file")
	}
}

func TestAdminConfigEmptyHooksUsesArray(t *testing.T) {
	restore := setupAdminTestState(t)
	defer restore()

	hooksPath := filepath.Join(t.TempDir(), "hooks.json")
	setLoadedHooksForTest(hooksPath, nil)

	cookie := loginCookieForTest(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/api/config", nil)
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	adminRequireAuth(adminConfigHandler)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("config status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if strings.Contains(body, `"hooks":null`) {
		t.Fatalf("config response should not contain null hooks: %s", body)
	}
	if !strings.Contains(body, `"hooks":[]`) {
		t.Fatalf("config response should contain empty hooks array: %s", body)
	}
}

func TestAdminUpdateHookWithSlashIDViaRouter(t *testing.T) {
	restore := setupAdminTestState(t)
	defer restore()

	*adminEnabled = true
	defer func() {
		*adminEnabled = false
	}()

	tmpDir := t.TempDir()
	hooksPath := filepath.Join(tmpDir, "hooks.json")

	initialHooks := hook.Hooks{
		{
			ID:              "iot/thjk/web",
			ExecuteCommand:  "/bin/echo",
			ResponseMessage: "ok",
			HTTPMethods:     []string{"POST"},
		},
	}

	if err := writeHooksFile(hooksPath, initialHooks); err != nil {
		t.Fatalf("writeHooksFile: %v", err)
	}
	setLoadedHooksForTest(hooksPath, initialHooks)

	router := mux.NewRouter()
	registerAdminRoutes(router)

	cookie := loginCookieForTest(t)
	req := httptest.NewRequest(http.MethodPut, "/admin/api/hooks", strings.NewReader(`{
		"file": "`+hooksPath+`",
		"currentId": "iot/thjk/web",
		"hook": {
			"id": "iot/thjk/web",
			"execute-command": "/usr/bin/env",
			"response-message": "updated slash id"
		}
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("slash id update status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	updatedHooks := readHooksFileForTest(t, hooksPath)
	updated := updatedHooks.Match("iot/thjk/web")
	if updated == nil {
		t.Fatal("expected slash-id hook to still exist after update")
	}
	if updated.ExecuteCommand != "/usr/bin/env" {
		t.Fatalf("updated execute-command = %q, want %q", updated.ExecuteCommand, "/usr/bin/env")
	}
}

func TestAdminWritesBlockedInTemplateMode(t *testing.T) {
	restore := setupAdminTestState(t)
	defer restore()

	tmpDir := t.TempDir()
	hooksPath := filepath.Join(tmpDir, "hooks.json")
	setLoadedHooksForTest(hooksPath, hook.Hooks{})

	*asTemplate = true

	err := upsertHookInFile(hooksPath, "", hook.Hook{ID: "blocked"})
	if err == nil || !strings.Contains(err.Error(), errAdminReadOnly.Error()) {
		t.Fatalf("upsertHookInFile error = %v, want %v", err, errAdminReadOnly)
	}
}
