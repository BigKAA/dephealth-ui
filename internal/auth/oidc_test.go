package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/BigKAA/dephealth-ui/internal/config"
)

// mockOIDCProvider creates a test HTTP server that mimics an OIDC provider
// with discovery, JWKS, and token endpoints.
type mockOIDCProvider struct {
	server     *httptest.Server
	privateKey *rsa.PrivateKey
	keyID      string
}

func newMockOIDCProvider(t *testing.T) *mockOIDCProvider {
	t.Helper()

	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	m := &mockOIDCProvider{
		privateKey: privKey,
		keyID:      "test-key-1",
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", m.handleDiscovery)
	mux.HandleFunc("/jwks", m.handleJWKS)
	mux.HandleFunc("/token", m.handleToken)
	mux.HandleFunc("/authorize", m.handleAuthorize)

	m.server = httptest.NewServer(mux)
	return m
}

func (m *mockOIDCProvider) handleDiscovery(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"issuer":                 m.server.URL,
		"authorization_endpoint": m.server.URL + "/authorize",
		"token_endpoint":         m.server.URL + "/token",
		"jwks_uri":               m.server.URL + "/jwks",
		"response_types_supported": []string{"code"},
		"subject_types_supported":  []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func (m *mockOIDCProvider) handleJWKS(w http.ResponseWriter, _ *http.Request) {
	jwk := jose.JSONWebKey{
		Key:       &m.privateKey.PublicKey,
		KeyID:     m.keyID,
		Algorithm: string(jose.RS256),
		Use:       "sig",
	}
	jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(jwks)
}

func (m *mockOIDCProvider) handleToken(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()

	idToken, err := m.issueIDToken("test-user", "Test User", "test@example.com")
	if err != nil {
		http.Error(w, "Failed to issue token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": "mock-access-token",
		"token_type":   "Bearer",
		"id_token":     idToken,
		"expires_in":   3600,
	})
}

func (m *mockOIDCProvider) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	// Redirect back with a code
	redirectURI := r.URL.Query().Get("redirect_uri")
	state := r.URL.Query().Get("state")
	u, _ := url.Parse(redirectURI)
	q := u.Query()
	q.Set("code", "mock-auth-code")
	q.Set("state", state)
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (m *mockOIDCProvider) issueIDToken(sub, name, email string) (string, error) {
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: m.privateKey},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", m.keyID),
	)
	if err != nil {
		return "", err
	}

	now := time.Now()
	claims := map[string]any{
		"iss":   m.server.URL,
		"sub":   sub,
		"aud":   "dephealth-ui",
		"exp":   now.Add(1 * time.Hour).Unix(),
		"iat":   now.Unix(),
		"name":  name,
		"email": email,
	}

	token, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		return "", err
	}
	return token, nil
}

func (m *mockOIDCProvider) close() {
	m.server.Close()
}

func setupTestOIDC(t *testing.T) (*oidcAuth, *mockOIDCProvider) {
	t.Helper()

	mock := newMockOIDCProvider(t)
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	cfg := config.OIDCConfig{
		Issuer:      mock.server.URL,
		ClientID:    "dephealth-ui",
		RedirectURL: "http://localhost:8080/auth/callback",
	}

	auth, err := NewOIDC(context.Background(), cfg, logger)
	if err != nil {
		mock.close()
		t.Fatalf("NewOIDC() error: %v", err)
	}

	return auth, mock
}

func TestNewOIDC_ProviderDiscovery(t *testing.T) {
	auth, mock := setupTestOIDC(t)
	defer mock.close()
	defer auth.Stop()

	if auth.verifier == nil {
		t.Error("verifier should not be nil after successful discovery")
	}
	if auth.secureCookie {
		t.Error("secureCookie should be false for http:// redirect URL")
	}
}

func TestNewOIDC_SecureCookie(t *testing.T) {
	mock := newMockOIDCProvider(t)
	defer mock.close()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	cfg := config.OIDCConfig{
		Issuer:      mock.server.URL,
		ClientID:    "dephealth-ui",
		RedirectURL: "https://dephealth.example.com/auth/callback",
	}

	auth, err := NewOIDC(context.Background(), cfg, logger)
	if err != nil {
		t.Fatalf("NewOIDC() error: %v", err)
	}
	defer auth.Stop()

	if !auth.secureCookie {
		t.Error("secureCookie should be true for https:// redirect URL")
	}
}

func TestNewOIDC_FailFast(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	cfg := config.OIDCConfig{
		Issuer:      "http://127.0.0.1:1", // unreachable
		ClientID:    "dephealth-ui",
		RedirectURL: "http://localhost:8080/auth/callback",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := NewOIDC(ctx, cfg, logger)
	if err == nil {
		t.Error("NewOIDC() should fail when provider is unreachable")
	}
}

func TestOIDC_LoginRedirect(t *testing.T) {
	auth, mock := setupTestOIDC(t)
	defer mock.close()
	defer auth.Stop()

	routes := auth.Routes()
	req := httptest.NewRequest("GET", "/login", nil)
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("login status = %d, want %d", w.Code, http.StatusFound)
	}

	location := w.Header().Get("Location")
	if !strings.Contains(location, "/authorize") {
		t.Errorf("redirect Location should contain /authorize, got %q", location)
	}
	if !strings.Contains(location, "code_challenge=") {
		t.Error("redirect should contain code_challenge parameter")
	}
	if !strings.Contains(location, "code_challenge_method=S256") {
		t.Error("redirect should contain code_challenge_method=S256")
	}
}

func TestOIDC_CallbackFlow(t *testing.T) {
	auth, mock := setupTestOIDC(t)
	defer mock.close()
	defer auth.Stop()

	routes := auth.Routes()

	// Step 1: Call /login to populate state store
	loginReq := httptest.NewRequest("GET", "/login", nil)
	loginW := httptest.NewRecorder()
	routes.ServeHTTP(loginW, loginReq)

	location := loginW.Header().Get("Location")
	u, _ := url.Parse(location)
	state := u.Query().Get("state")
	if state == "" {
		t.Fatal("login redirect missing state parameter")
	}

	// Step 2: Call /callback with code and state
	callbackURL := fmt.Sprintf("/callback?code=mock-auth-code&state=%s", state)
	callbackReq := httptest.NewRequest("GET", callbackURL, nil)
	callbackW := httptest.NewRecorder()
	routes.ServeHTTP(callbackW, callbackReq)

	if callbackW.Code != http.StatusFound {
		t.Fatalf("callback status = %d, want %d; body: %s", callbackW.Code, http.StatusFound, callbackW.Body.String())
	}

	cookies := callbackW.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == sessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("callback should set session cookie")
	}
	if sessionCookie.Value == "" {
		t.Error("session cookie should have a non-empty value")
	}
}

func TestOIDC_CallbackMissingParams(t *testing.T) {
	auth, mock := setupTestOIDC(t)
	defer mock.close()
	defer auth.Stop()

	routes := auth.Routes()

	req := httptest.NewRequest("GET", "/callback", nil)
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("callback with no params status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestOIDC_CallbackInvalidState(t *testing.T) {
	auth, mock := setupTestOIDC(t)
	defer mock.close()
	defer auth.Stop()

	routes := auth.Routes()

	req := httptest.NewRequest("GET", "/callback?code=x&state=invalid", nil)
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("callback with invalid state status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestOIDC_Logout(t *testing.T) {
	auth, mock := setupTestOIDC(t)
	defer mock.close()
	defer auth.Stop()

	// Create a session
	sessionID, _ := auth.sessions.Create(UserInfo{Subject: "user-1"})

	routes := auth.Routes()
	req := httptest.NewRequest("GET", "/logout", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("logout status = %d, want %d", w.Code, http.StatusFound)
	}

	// Verify session is deleted
	if auth.sessions.Get(sessionID) != nil {
		t.Error("session should be deleted after logout")
	}

	// Verify cookie is cleared
	cookies := w.Result().Cookies()
	for _, c := range cookies {
		if c.Name == sessionCookieName && c.MaxAge != -1 {
			t.Error("session cookie should be cleared (MaxAge=-1)")
		}
	}
}

func TestOIDC_UserInfo(t *testing.T) {
	auth, mock := setupTestOIDC(t)
	defer mock.close()
	defer auth.Stop()

	user := UserInfo{Subject: "user-1", Name: "Alice", Email: "alice@example.com"}
	sessionID, _ := auth.sessions.Create(user)

	routes := auth.Routes()
	req := httptest.NewRequest("GET", "/userinfo", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("userinfo status = %d, want %d", w.Code, http.StatusOK)
	}

	var got UserInfo
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode userinfo: %v", err)
	}
	if got.Subject != "user-1" {
		t.Errorf("Subject = %q, want %q", got.Subject, "user-1")
	}
	if got.Name != "Alice" {
		t.Errorf("Name = %q, want %q", got.Name, "Alice")
	}
}

func TestOIDC_UserInfoUnauthorized(t *testing.T) {
	auth, mock := setupTestOIDC(t)
	defer mock.close()
	defer auth.Stop()

	routes := auth.Routes()
	req := httptest.NewRequest("GET", "/userinfo", nil)
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("userinfo without session status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestOIDC_MiddlewareValidSession(t *testing.T) {
	auth, mock := setupTestOIDC(t)
	defer mock.close()
	defer auth.Stop()

	sessionID, _ := auth.sessions.Create(UserInfo{Subject: "user-1"})

	handler := auth.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "ok")
	}))

	req := httptest.NewRequest("GET", "/api/v1/topology", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("middleware with valid session status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestOIDC_MiddlewareNoSession(t *testing.T) {
	auth, mock := setupTestOIDC(t)
	defer mock.close()
	defer auth.Stop()

	handler := auth.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/topology", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("middleware without session status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestOIDC_MiddlewareExpiredSession(t *testing.T) {
	auth, mock := setupTestOIDC(t)
	defer mock.close()
	defer auth.Stop()

	// Manually create an expired session
	auth.sessions.mu.Lock()
	auth.sessions.sessions["expired-session"] = &Session{
		ID:        "expired-session",
		User:      UserInfo{Subject: "user-1"},
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	auth.sessions.mu.Unlock()

	handler := auth.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/topology", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "expired-session"})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("middleware with expired session status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestGenerateCodeChallenge(t *testing.T) {
	verifier := "test-verifier-12345"
	challenge := generateCodeChallenge(verifier)
	if challenge == "" {
		t.Error("generateCodeChallenge returned empty string")
	}
	// S256 challenge should be base64url-encoded (no +, /, or =)
	if strings.ContainsAny(challenge, "+/=") {
		t.Errorf("code challenge contains invalid chars for base64url: %q", challenge)
	}
}
