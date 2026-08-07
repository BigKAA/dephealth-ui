package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BigKAA/dephealth-ui/internal/export"
)

func TestExportJSON(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/v1/export/json", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, ".json") {
		t.Errorf("Content-Disposition = %q, want filename with .json", cd)
	}

	var data export.ExportData
	if err := json.NewDecoder(w.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if data.Version != "1.0" {
		t.Errorf("Version = %q, want %q", data.Version, "1.0")
	}
	if data.Scope != "full" {
		t.Errorf("Scope = %q, want %q", data.Scope, "full")
	}
	if len(data.Nodes) == 0 {
		t.Error("expected at least one node")
	}
}

func TestExportCSV(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/v1/export/csv", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/zip")
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, ".zip") {
		t.Errorf("Content-Disposition = %q, want filename with .zip", cd)
	}

	// Verify ZIP structure.
	zr, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatalf("failed to open ZIP: %v", err)
	}
	names := make(map[string]bool)
	for _, f := range zr.File {
		names[f.Name] = true
	}
	if !names["nodes.csv"] {
		t.Error("ZIP missing nodes.csv")
	}
	if !names["edges.csv"] {
		t.Error("ZIP missing edges.csv")
	}
}

func TestExportDOT(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/v1/export/dot", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/vnd.graphviz" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/vnd.graphviz")
	}

	body := w.Body.String()
	if !strings.Contains(body, "digraph") {
		t.Error("DOT output should contain 'digraph'")
	}
}

func TestExportPNGRequiresGraphviz(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/v1/export/png", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if export.GraphvizAvailable() {
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
		}
		if ct := w.Header().Get("Content-Type"); ct != "image/png" {
			t.Errorf("Content-Type = %q, want %q", ct, "image/png")
		}
	} else {
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want %d (Graphviz not installed)", w.Code, http.StatusServiceUnavailable)
		}
	}
}

func TestExportSVGRequiresGraphviz(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/v1/export/svg", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if export.GraphvizAvailable() {
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
		}
		if ct := w.Header().Get("Content-Type"); ct != "image/svg+xml" {
			t.Errorf("Content-Type = %q, want %q", ct, "image/svg+xml")
		}
	} else {
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want %d (Graphviz not installed)", w.Code, http.StatusServiceUnavailable)
		}
	}
}

func TestExportInvalidFormat(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/v1/export/pdf", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestExportInvalidScope(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/v1/export/json?scope=invalid", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestExportInvalidScale(t *testing.T) {
	srv := newTestServer()

	tests := []struct {
		name  string
		query string
	}{
		{"not a number", "/api/v1/export/json?scale=abc"},
		{"too low", "/api/v1/export/json?scale=0"},
		{"too high", "/api/v1/export/json?scale=5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.query, nil)
			w := httptest.NewRecorder()
			srv.router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestExportInvalidTime(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/v1/export/json?time=bad", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestExportScopeCurrent(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/v1/export/json?scope=current&namespace=default", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var data export.ExportData
	if err := json.NewDecoder(w.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if data.Scope != "current" {
		t.Errorf("Scope = %q, want %q", data.Scope, "current")
	}
	if data.Filters["namespace"] != "default" {
		t.Errorf("Filters[namespace] = %q, want %q", data.Filters["namespace"], "default")
	}
}

func TestExportWithHistory(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/v1/export/json?time=2026-01-15T12:00:00Z", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var data export.ExportData
	if err := json.NewDecoder(w.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if len(data.Nodes) == 0 {
		t.Error("expected at least one node in historical export")
	}
}

func TestExportRouteRegistered(t *testing.T) {
	srv := newTestServer()

	formats := []string{"json", "csv", "dot", "png", "svg"}
	for _, f := range formats {
		t.Run(f, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/export/"+f, nil)
			w := httptest.NewRecorder()
			srv.router.ServeHTTP(w, req)

			// Should not be 404 or 405.
			if w.Code == http.StatusNotFound || w.Code == http.StatusMethodNotAllowed {
				t.Errorf("route /api/v1/export/%s returned %d", f, w.Code)
			}
		})
	}
}
