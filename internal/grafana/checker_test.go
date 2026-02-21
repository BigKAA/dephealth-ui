package grafana

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAvailable(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{"healthy", http.StatusOK, true},
		{"server error", http.StatusInternalServerError, false},
		{"unauthorized", http.StatusUnauthorized, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/health" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			c := NewChecker(Config{BaseURL: srv.URL})
			if got := c.Available(context.Background()); got != tt.want {
				t.Errorf("Available() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAvailableUnreachable(t *testing.T) {
	c := NewChecker(Config{BaseURL: "http://127.0.0.1:1"})
	if c.Available(context.Background()) {
		t.Error("Available() = true for unreachable server, want false")
	}
}

func TestCheckDashboard(t *testing.T) {
	tests := []struct {
		name       string
		uid        string
		statusCode int
		want       bool
	}{
		{"found", "abc123", http.StatusOK, true},
		{"not found", "missing", http.StatusNotFound, false},
		{"server error", "err", http.StatusInternalServerError, false},
		{"forbidden", "noaccess", http.StatusForbidden, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := "/api/dashboards/uid/" + tt.uid
				if r.URL.Path != expectedPath {
					t.Errorf("path = %q, want %q", r.URL.Path, expectedPath)
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			c := NewChecker(Config{BaseURL: srv.URL})
			if got := c.CheckDashboard(context.Background(), tt.uid); got != tt.want {
				t.Errorf("CheckDashboard(%q) = %v, want %v", tt.uid, got, tt.want)
			}
		})
	}
}

func TestAuthToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer my-token" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer my-token")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewChecker(Config{BaseURL: srv.URL, Token: "my-token"})
	if !c.Available(context.Background()) {
		t.Error("Available() = false with valid token, want true")
	}
}

func TestAuthBasic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "secret" {
			t.Errorf("BasicAuth = (%q, %q, %v), want (admin, secret, true)", user, pass, ok)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewChecker(Config{BaseURL: srv.URL, Username: "admin", Password: "secret"})
	if !c.Available(context.Background()) {
		t.Error("Available() = false with valid basic auth, want true")
	}
}

func TestAuthTokenPriority(t *testing.T) {
	// When both token and basic auth are configured, token takes priority.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer my-token" {
			t.Errorf("Authorization = %q, want Bearer token (not basic auth)", auth)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Verify basic auth is NOT set when token is present.
		_, _, ok := r.BasicAuth()
		if ok {
			t.Error("BasicAuth should not be set when token is present")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewChecker(Config{
		BaseURL:  srv.URL,
		Token:    "my-token",
		Username: "admin",
		Password: "secret",
	})
	if !c.Available(context.Background()) {
		t.Error("Available() = false, want true")
	}
}

func TestAuthNone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Errorf("Authorization = %q, want empty (no auth)", auth)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewChecker(Config{BaseURL: srv.URL})
	if !c.Available(context.Background()) {
		t.Error("Available() = false, want true")
	}
}
