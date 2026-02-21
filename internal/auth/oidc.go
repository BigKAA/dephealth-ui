package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-chi/chi/v5"
	"golang.org/x/oauth2"

	"github.com/BigKAA/dephealth-ui/internal/config"
)

const (
	sessionCookieName = "dephealth_session"
	sessionTTL        = 8 * time.Hour
	stateTTL          = 5 * time.Minute
)

// stateEntry holds PKCE and expiry data for an in-flight OIDC auth request.
type stateEntry struct {
	codeVerifier string
	expiresAt    time.Time
}

// oidcAuth implements OIDC Authorization Code Flow with PKCE.
type oidcAuth struct {
	oauth2Cfg  oauth2.Config
	verifier   *oidc.IDTokenVerifier
	sessions   *SessionStore
	states     map[string]stateEntry
	statesMu   sync.Mutex
	secureCookie bool
	logger     *slog.Logger
}

// NewOIDC creates a new OIDC authenticator. It performs provider discovery
// using the given context and fails fast if the provider is unreachable.
func NewOIDC(ctx context.Context, cfg config.OIDCConfig, logger *slog.Logger) (*oidcAuth, error) {
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("OIDC provider discovery failed for %q: %w", cfg.Issuer, err)
	}

	oauth2Cfg := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})
	secureCookie := strings.HasPrefix(cfg.RedirectURL, "https://")

	return &oidcAuth{
		oauth2Cfg:    oauth2Cfg,
		verifier:     verifier,
		sessions:     NewSessionStore(sessionTTL),
		states:       make(map[string]stateEntry),
		secureCookie: secureCookie,
		logger:       logger,
	}, nil
}

// Middleware returns HTTP middleware that validates the session cookie.
// Returns 401 JSON response if the session is missing or invalid.
func (a *oidcAuth) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil {
				a.unauthorizedJSON(w)
				return
			}

			sess := a.sessions.Get(cookie.Value)
			if sess == nil {
				a.unauthorizedJSON(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Routes returns a chi.Router with OIDC auth endpoints.
func (a *oidcAuth) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/login", a.handleLogin)
	r.Get("/callback", a.handleCallback)
	r.Get("/logout", a.handleLogout)
	r.Get("/userinfo", a.handleUserInfo)
	return r
}

// Stop terminates the background session cleanup goroutine.
func (a *oidcAuth) Stop() {
	a.sessions.Stop()
}

func (a *oidcAuth) handleLogin(w http.ResponseWriter, r *http.Request) {
	state, err := generateRandomString(32)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	codeVerifier, err := generateRandomString(32)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	a.statesMu.Lock()
	a.cleanupExpiredStates()
	a.states[state] = stateEntry{
		codeVerifier: codeVerifier,
		expiresAt:    time.Now().Add(stateTTL),
	}
	a.statesMu.Unlock()

	codeChallenge := generateCodeChallenge(codeVerifier)
	authURL := a.oauth2Cfg.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)

	http.Redirect(w, r, authURL, http.StatusFound)
}

func (a *oidcAuth) handleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	if state == "" || code == "" {
		http.Error(w, "Missing state or code parameter", http.StatusBadRequest)
		return
	}

	a.statesMu.Lock()
	entry, ok := a.states[state]
	if ok {
		delete(a.states, state)
	}
	a.statesMu.Unlock()

	if !ok || time.Now().After(entry.expiresAt) {
		http.Error(w, "Invalid or expired state", http.StatusBadRequest)
		return
	}

	token, err := a.oauth2Cfg.Exchange(r.Context(), code,
		oauth2.SetAuthURLParam("code_verifier", entry.codeVerifier),
	)
	if err != nil {
		a.logger.Error("OIDC token exchange failed", "error", err)
		http.Error(w, "Token exchange failed", http.StatusBadGateway)
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		a.logger.Error("OIDC response missing id_token")
		http.Error(w, "Missing id_token in response", http.StatusBadGateway)
		return
	}

	idToken, err := a.verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		a.logger.Error("OIDC ID token verification failed", "error", err)
		http.Error(w, "ID token verification failed", http.StatusBadGateway)
		return
	}

	var claims struct {
		Subject string `json:"sub"`
		Name    string `json:"name"`
		Email   string `json:"email"`
	}
	if err := idToken.Claims(&claims); err != nil {
		a.logger.Error("failed to parse ID token claims", "error", err)
		http.Error(w, "Failed to parse claims", http.StatusInternalServerError)
		return
	}

	user := UserInfo{
		Subject: claims.Subject,
		Name:    claims.Name,
		Email:   claims.Email,
	}

	sessionID, err := a.sessions.Create(user)
	if err != nil {
		a.logger.Error("failed to create session", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *oidcAuth) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		a.sessions.Delete(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *oidcAuth) handleUserInfo(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		a.unauthorizedJSON(w)
		return
	}

	sess := a.sessions.Get(cookie.Value)
	if sess == nil {
		a.unauthorizedJSON(w)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sess.User)
}

func (a *oidcAuth) unauthorizedJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = fmt.Fprint(w, `{"error":"unauthorized"}`)
}

func (a *oidcAuth) cleanupExpiredStates() {
	now := time.Now()
	for k, v := range a.states {
		if now.After(v.expiresAt) {
			delete(a.states, k)
		}
	}
}

// generateRandomString returns a hex-encoded random string of the given byte length.
func generateRandomString(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// generateCodeChallenge computes S256 PKCE code challenge from a code verifier.
func generateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
