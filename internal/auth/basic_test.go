package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/BigKAA/dephealth-ui/internal/config"
)

func hashPassword(t *testing.T, password string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt hash: %v", err)
	}
	return string(h)
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func TestBasicValidCredentials(t *testing.T) {
	hash := hashPassword(t, "secret")
	auth := NewBasic([]User{{Username: "admin", PasswordHash: hash}})

	handler := auth.Middleware()(okHandler())
	req := httptest.NewRequest("GET", "/", nil)
	req.SetBasicAuth("admin", "secret")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestBasicWrongPassword(t *testing.T) {
	hash := hashPassword(t, "secret")
	auth := NewBasic([]User{{Username: "admin", PasswordHash: hash}})

	handler := auth.Middleware()(okHandler())
	req := httptest.NewRequest("GET", "/", nil)
	req.SetBasicAuth("admin", "wrong")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestBasicUnknownUser(t *testing.T) {
	hash := hashPassword(t, "secret")
	auth := NewBasic([]User{{Username: "admin", PasswordHash: hash}})

	handler := auth.Middleware()(okHandler())
	req := httptest.NewRequest("GET", "/", nil)
	req.SetBasicAuth("unknown", "secret")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestBasicNoHeader(t *testing.T) {
	hash := hashPassword(t, "secret")
	auth := NewBasic([]User{{Username: "admin", PasswordHash: hash}})

	handler := auth.Middleware()(okHandler())
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	if w.Header().Get("WWW-Authenticate") == "" {
		t.Error("expected WWW-Authenticate header")
	}
}

func TestBasicMalformedHeader(t *testing.T) {
	hash := hashPassword(t, "secret")
	auth := NewBasic([]User{{Username: "admin", PasswordHash: hash}})

	handler := auth.Middleware()(okHandler())
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestNoneAuthPassesThrough(t *testing.T) {
	auth := &noneAuth{}
	handler := auth.Middleware()(okHandler())
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestFactoryNone(t *testing.T) {
	auth, err := NewFromConfig(config.AuthConfig{Type: "none"})
	if err != nil {
		t.Fatalf("NewFromConfig error: %v", err)
	}
	if auth == nil {
		t.Fatal("expected non-nil authenticator")
	}
}

func TestFactoryEmpty(t *testing.T) {
	auth, err := NewFromConfig(config.AuthConfig{Type: ""})
	if err != nil {
		t.Fatalf("NewFromConfig error: %v", err)
	}
	if auth == nil {
		t.Fatal("expected non-nil authenticator")
	}
}

func TestFactoryBasic(t *testing.T) {
	hash := hashPassword(t, "pass")
	auth, err := NewFromConfig(config.AuthConfig{
		Type: "basic",
		Basic: config.BasicConfig{
			Users: []config.BasicUser{
				{Username: "admin", PasswordHash: hash},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewFromConfig error: %v", err)
	}
	if auth == nil {
		t.Fatal("expected non-nil authenticator")
	}
}

func TestFactoryBasicNoUsers(t *testing.T) {
	_, err := NewFromConfig(config.AuthConfig{
		Type: "basic",
	})
	if err == nil {
		t.Fatal("expected error for basic auth with no users")
	}
}

func TestFactoryBasicEmptyUsername(t *testing.T) {
	_, err := NewFromConfig(config.AuthConfig{
		Type: "basic",
		Basic: config.BasicConfig{
			Users: []config.BasicUser{
				{Username: "", PasswordHash: "$2a$10$abc"},
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for empty username")
	}
}

func TestFactoryBasicEmptyHash(t *testing.T) {
	_, err := NewFromConfig(config.AuthConfig{
		Type: "basic",
		Basic: config.BasicConfig{
			Users: []config.BasicUser{
				{Username: "admin", PasswordHash: ""},
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for empty passwordHash")
	}
}

func TestFactoryUnknownType(t *testing.T) {
	_, err := NewFromConfig(config.AuthConfig{Type: "ldap"})
	if err == nil {
		t.Fatal("expected error for unknown auth type")
	}
}
