package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/adnanh/webhook/internal/hook"
	"github.com/gorilla/mux"
)

var (
	adminEnabled    = flag.Bool("admin", false, "enable the admin UI and API for managing hooks")
	adminURLPrefix  = flag.String("admin-path", "admin", "url prefix to use for the admin UI and API")
	adminTOTPSecret = flag.String("admin-totp-secret", "", "base32 encoded TOTP secret for admin login")
	adminJWTSecret  = flag.String("admin-jwt-secret", "", "secret used to sign admin JWT sessions")
	adminSessionTTL = flag.String("admin-session-ttl", "12h", "lifetime of an admin JWT session")
)

const adminTokenCookieName = "webhook_admin_token"

type adminAuthConfig struct {
	basePath   string
	totpSecret []byte
	jwtSecret  []byte
	sessionTTL time.Duration
}

type adminJWTHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

type adminJWTClaims struct {
	Subject   string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	NotBefore int64  `json:"nbf"`
	ExpiresAt int64  `json:"exp"`
}

type adminLoginRequest struct {
	Code string `json:"code"`
}

type adminHookMutationRequest struct {
	File      string    `json:"file"`
	CurrentID string    `json:"currentId,omitempty"`
	Hook      hook.Hook `json:"hook"`
}

var currentAdminAuth *adminAuthConfig

func initAdmin() error {
	if !*adminEnabled {
		return nil
	}

	trimmedAdminPath := strings.Trim(strings.TrimSpace(*adminURLPrefix), "/")
	if trimmedAdminPath == "" {
		return errors.New("admin-path must not be empty")
	}

	trimmedHooksPath := strings.Trim(strings.TrimSpace(*hooksURLPrefix), "/")
	if trimmedHooksPath == trimmedAdminPath {
		return errors.New("admin-path must not match urlprefix")
	}

	ttl, err := time.ParseDuration(strings.TrimSpace(*adminSessionTTL))
	if err != nil {
		return fmt.Errorf("invalid admin-session-ttl: %w", err)
	}
	if ttl <= 0 {
		return errors.New("admin-session-ttl must be greater than zero")
	}

	totpSecret, err := decodeTOTPSecret(*adminTOTPSecret)
	if err != nil {
		return fmt.Errorf("invalid admin-totp-secret: %w", err)
	}

	jwtSecret := strings.TrimSpace(*adminJWTSecret)
	if jwtSecret == "" {
		return errors.New("admin-jwt-secret must not be empty")
	}

	*adminURLPrefix = trimmedAdminPath
	currentAdminAuth = &adminAuthConfig{
		basePath:   makeBaseURL(adminURLPrefix),
		totpSecret: totpSecret,
		jwtSecret:  []byte(jwtSecret),
		sessionTTL: ttl,
	}

	return nil
}

func registerAdminRoutes(r *mux.Router) {
	if !*adminEnabled {
		return
	}

	basePath := currentAdminAuth.basePath
	staticHandler := http.StripPrefix(basePath+"/", adminStaticHandler())

	r.HandleFunc(basePath, adminUIHandler).Methods(http.MethodGet)
	r.HandleFunc(basePath+"/", adminUIHandler).Methods(http.MethodGet)
	r.HandleFunc(basePath+"/api/auth/login", adminLoginHandler).Methods(http.MethodPost)
	r.HandleFunc(basePath+"/api/auth/logout", adminLogoutHandler).Methods(http.MethodPost)
	r.HandleFunc(basePath+"/api/config", adminRequireAuth(adminConfigHandler)).Methods(http.MethodGet)
	r.HandleFunc(basePath+"/api/update/status", adminRequireAuth(adminUpdateStatusHandler)).Methods(http.MethodGet)
	r.HandleFunc(basePath+"/api/update/check", adminRequireAuth(adminUpdateCheckHandler)).Methods(http.MethodPost)
	r.HandleFunc(basePath+"/api/hooks", adminRequireAuth(adminCreateHookHandler)).Methods(http.MethodPost)
	r.HandleFunc(basePath+"/api/hooks", adminRequireAuth(adminUpdateHookHandler)).Methods(http.MethodPut)
	r.HandleFunc(basePath+"/api/hooks", adminRequireAuth(adminDeleteHookHandler)).Methods(http.MethodDelete)
	r.PathPrefix(basePath + "/assets/").Handler(staticHandler).Methods(http.MethodGet)
}

func decodeTOTPSecret(secret string) ([]byte, error) {
	normalized := strings.ToUpper(strings.TrimSpace(secret))
	replacer := strings.NewReplacer(" ", "", "-", "", "\n", "", "\r", "", "\t", "", "=", "")
	normalized = replacer.Replace(normalized)

	if normalized == "" {
		return nil, errors.New("secret is empty")
	}

	decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(normalized)
	if err == nil {
		return decoded, nil
	}

	if rem := len(normalized) % 8; rem != 0 {
		normalized += strings.Repeat("=", 8-rem)
	}

	return base32.StdEncoding.DecodeString(normalized)
}

func hotpCode(secret []byte, counter uint64, digits int) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)

	mac := hmac.New(sha1.New, secret)
	_, _ = mac.Write(buf[:])
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	value := (int(sum[offset])&0x7f)<<24 |
		int(sum[offset+1])<<16 |
		int(sum[offset+2])<<8 |
		int(sum[offset+3])

	mod := 1
	for i := 0; i < digits; i++ {
		mod *= 10
	}

	code := value % mod
	format := "%0" + strconv.Itoa(digits) + "d"
	return fmt.Sprintf(format, code)
}

func verifyTOTP(secret []byte, code string, now time.Time) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}
	for _, ch := range code {
		if ch < '0' || ch > '9' {
			return false
		}
	}

	counter := now.Unix() / 30
	for offset := int64(-1); offset <= 1; offset++ {
		current := counter + offset
		if current < 0 {
			continue
		}

		expected := hotpCode(secret, uint64(current), 6)
		if hmac.Equal([]byte(expected), []byte(code)) {
			return true
		}
	}

	return false
}

func signAdminJWT(now time.Time) (string, error) {
	if currentAdminAuth == nil {
		return "", errors.New("admin auth is not initialized")
	}

	header := adminJWTHeader{
		Algorithm: "HS256",
		Type:      "JWT",
	}
	claims := adminJWTClaims{
		Subject:   "webhook-admin",
		IssuedAt:  now.Unix(),
		NotBefore: now.Add(-30 * time.Second).Unix(),
		ExpiresAt: now.Add(currentAdminAuth.sessionTTL).Unix(),
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	payload := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)

	mac := hmac.New(sha256.New, currentAdminAuth.jwtSecret)
	_, _ = mac.Write([]byte(payload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return payload + "." + signature, nil
}

func parseAdminJWT(token string, now time.Time) (*adminJWTClaims, error) {
	if currentAdminAuth == nil {
		return nil, errors.New("admin auth is not initialized")
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	signed := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("invalid token signature encoding")
	}

	mac := hmac.New(sha256.New, currentAdminAuth.jwtSecret)
	_, _ = mac.Write([]byte(signed))
	expected := mac.Sum(nil)
	if !hmac.Equal(signature, expected) {
		return nil, errors.New("invalid token signature")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("invalid token header encoding")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid token payload encoding")
	}

	var header adminJWTHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, errors.New("invalid token header")
	}
	if header.Algorithm != "HS256" || header.Type != "JWT" {
		return nil, errors.New("unsupported token header")
	}

	var claims adminJWTClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, errors.New("invalid token payload")
	}
	if claims.Subject != "webhook-admin" {
		return nil, errors.New("invalid token subject")
	}

	nowUnix := now.Unix()
	if claims.NotBefore != 0 && nowUnix < claims.NotBefore {
		return nil, errors.New("token is not active yet")
	}
	if claims.ExpiresAt == 0 || nowUnix >= claims.ExpiresAt {
		return nil, errors.New("token has expired")
	}

	return &claims, nil
}

func setAdminTokenCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     adminTokenCookieName,
		Value:    token,
		Path:     currentAdminAuth.basePath,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   *secure,
		MaxAge:   int(currentAdminAuth.sessionTTL.Seconds()),
	})
}

func clearAdminTokenCookie(w http.ResponseWriter) {
	path := "/"
	if currentAdminAuth != nil {
		path = currentAdminAuth.basePath
	}

	http.SetCookie(w, &http.Cookie{
		Name:     adminTokenCookieName,
		Value:    "",
		Path:     path,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   *secure,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func adminTokenFromRequest(r *http.Request) string {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[7:])
	}

	cookie, err := r.Cookie(adminTokenCookieName)
	if err != nil {
		return ""
	}

	return cookie.Value
}

func authenticateAdminRequest(r *http.Request) error {
	token := adminTokenFromRequest(r)
	if token == "" {
		return errors.New("missing admin token")
	}

	_, err := parseAdminJWT(token, time.Now())
	return err
}

func adminRequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := authenticateAdminRequest(r); err != nil {
			writeAdminError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		next(w, r)
	}
}

func writeAdminJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeAdminError(w http.ResponseWriter, status int, message string) {
	writeAdminJSON(w, status, map[string]string{"error": message})
}

func decodeAdminJSON(r *http.Request, payload interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(payload)
}

func adminLoginHandler(w http.ResponseWriter, r *http.Request) {
	var loginReq adminLoginRequest
	if err := decodeAdminJSON(r, &loginReq); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid login payload")
		return
	}

	if !verifyTOTP(currentAdminAuth.totpSecret, loginReq.Code, time.Now()) {
		log.Printf("admin login failed from %s", r.RemoteAddr)
		writeAdminError(w, http.StatusUnauthorized, "invalid TOTP code")
		return
	}

	token, err := signAdminJWT(time.Now())
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "could not create session token")
		return
	}

	setAdminTokenCookie(w, token)
	log.Printf("admin login succeeded from %s", r.RemoteAddr)
	writeAdminJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func adminLogoutHandler(w http.ResponseWriter, r *http.Request) {
	clearAdminTokenCookie(w)
	writeAdminJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func adminConfigHandler(w http.ResponseWriter, r *http.Request) {
	writeAdminJSON(w, http.StatusOK, hooksStateSnapshot())
}

func adminMutationStatus(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, errAdminReadOnly):
		return http.StatusConflict
	case errors.Is(err, errUnknownHookFile):
		return http.StatusNotFound
	case strings.Contains(err.Error(), "was not found"):
		return http.StatusNotFound
	case strings.Contains(err.Error(), "already defined"):
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}

func adminCreateHookHandler(w http.ResponseWriter, r *http.Request) {
	var req adminHookMutationRequest
	if err := decodeAdminJSON(r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid hook payload")
		return
	}

	if err := upsertHookInFile(req.File, "", req.Hook); err != nil {
		writeAdminError(w, adminMutationStatus(err), err.Error())
		return
	}

	writeAdminJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func adminUpdateHookHandler(w http.ResponseWriter, r *http.Request) {
	var req adminHookMutationRequest
	if err := decodeAdminJSON(r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid hook payload")
		return
	}
	if strings.TrimSpace(req.CurrentID) == "" {
		writeAdminError(w, http.StatusBadRequest, "currentId is required")
		return
	}

	if err := upsertHookInFile(req.File, req.CurrentID, req.Hook); err != nil {
		writeAdminError(w, adminMutationStatus(err), err.Error())
		return
	}

	writeAdminJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func adminDeleteHookHandler(w http.ResponseWriter, r *http.Request) {
	var req adminHookMutationRequest
	if err := decodeAdminJSON(r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid delete payload")
		return
	}
	if strings.TrimSpace(req.File) == "" {
		writeAdminError(w, http.StatusBadRequest, "file is required")
		return
	}
	if strings.TrimSpace(req.CurrentID) == "" {
		writeAdminError(w, http.StatusBadRequest, "currentId is required")
		return
	}

	if err := deleteHookFromFile(req.File, req.CurrentID); err != nil {
		writeAdminError(w, adminMutationStatus(err), err.Error())
		return
	}

	writeAdminJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
