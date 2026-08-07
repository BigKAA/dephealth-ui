package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	handler := RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/topology", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON log: %v, raw: %s", err, buf.String())
	}
	if m["method"] != "GET" {
		t.Errorf("expected method=GET, got %v", m["method"])
	}
	if m["path"] != "/api/v1/topology" {
		t.Errorf("expected path=/api/v1/topology, got %v", m["path"])
	}
	if m["status"] != float64(200) {
		t.Errorf("expected status=200, got %v", m["status"])
	}
	if _, ok := m["duration_ms"]; !ok {
		t.Error("expected duration_ms field")
	}
}

func TestRequestLogger_Status500(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	handler := RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON log: %v", err)
	}
	if m["status"] != float64(500) {
		t.Errorf("expected status=500, got %v", m["status"])
	}
	if m["method"] != "POST" {
		t.Errorf("expected method=POST, got %v", m["method"])
	}
}
