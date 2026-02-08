package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGzipMiddleware_WithAcceptEncoding(t *testing.T) {
	handler := gzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"nodes":[],"edges":[]}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	resp := rec.Result()
	defer resp.Body.Close()

	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Errorf("Content-Encoding = %q, want %q", resp.Header.Get("Content-Encoding"), "gzip")
	}

	if resp.Header.Get("Vary") != "Accept-Encoding" {
		t.Errorf("Vary = %q, want %q", resp.Header.Get("Vary"), "Accept-Encoding")
	}

	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want %q", resp.Header.Get("Content-Type"), "application/json")
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gz.Close()

	body, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("failed to read gzip body: %v", err)
	}

	want := `{"nodes":[],"edges":[]}`
	if string(body) != want {
		t.Errorf("body = %q, want %q", string(body), want)
	}
}

func TestGzipMiddleware_WithoutAcceptEncoding(t *testing.T) {
	handler := gzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	resp := rec.Result()
	defer resp.Body.Close()

	if resp.Header.Get("Content-Encoding") != "" {
		t.Errorf("Content-Encoding should be empty, got %q", resp.Header.Get("Content-Encoding"))
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"ok"`) {
		t.Errorf("unexpected body: %s", body)
	}
}
