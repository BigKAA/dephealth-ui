package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/BigKAA/dephealth-ui/internal/auth"
	"github.com/BigKAA/dephealth-ui/internal/cache"
	"github.com/BigKAA/dephealth-ui/internal/config"
	"github.com/BigKAA/dephealth-ui/internal/topology"
)

func newTestPromServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{"job":"svc-go","dependency":"postgres","type":"postgres","host":"pg","port":"5432"},"value":[1700000000,"1"]}
		]}}`))
	}))
}

func newTestServer() *Server {
	promSrv := newTestPromServer()
	cfg := &config.Config{
		Server: config.ServerConfig{Listen: ":8080"},
		Datasources: config.DatasourcesConfig{
			Prometheus: config.PrometheusConfig{URL: promSrv.URL},
		},
		Cache: config.CacheConfig{TTL: 15 * time.Second},
		Auth:  config.AuthConfig{Type: "none"},
		Grafana: config.GrafanaConfig{
			BaseURL: "https://grafana.example.com",
			Dashboards: config.DashboardsConfig{
				ServiceStatus: "svc-dash",
				LinkStatus:    "link-dash",
			},
		},
		Alerts: config.AlertsConfig{
			SeverityLabel: "severity",
			SeverityLevels: []config.SeverityLevel{
				{Value: "critical", Color: "#f44336"},
				{Value: "warning", Color: "#ff9800"},
				{Value: "info", Color: "#2196f3"},
			},
		},
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	promClient := topology.NewPrometheusClient(topology.PrometheusConfig{URL: promSrv.URL})
	grafanaCfg := topology.GrafanaConfig{
		BaseURL:              cfg.Grafana.BaseURL,
		ServiceStatusDashUID: cfg.Grafana.Dashboards.ServiceStatus,
		LinkStatusDashUID:    cfg.Grafana.Dashboards.LinkStatus,
	}
	builder := topology.NewGraphBuilder(promClient, nil, grafanaCfg, cfg.Cache.TTL, 0, logger, cfg.Alerts.SeverityLevels)
	topologyCache := cache.New(cfg.Cache.TTL)
	authenticator, _ := auth.NewFromConfig(config.AuthConfig{Type: "none"})
	return New(cfg, logger, builder, promClient, nil, topologyCache, authenticator)
}

func TestRoutes(t *testing.T) {
	srv := newTestServer()

	tests := []struct {
		method string
		path   string
		status int
	}{
		{"GET", "/healthz", http.StatusOK},
		{"GET", "/readyz", http.StatusOK},
		{"GET", "/api/v1/topology", http.StatusOK},
		{"GET", "/api/v1/alerts", http.StatusOK},
		{"GET", "/api/v1/config", http.StatusOK},
		{"GET", "/api/v1/cascade-analysis", http.StatusOK},
		{"GET", "/api/v1/cascade-graph", http.StatusOK},
		{"GET", "/", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			srv.router.ServeHTTP(w, req)

			if w.Code != tt.status {
				t.Errorf("status = %d, want %d", w.Code, tt.status)
			}
		})
	}
}

func TestTopologyReturnsJSON(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/v1/topology", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var resp topology.TopologyResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Nodes) == 0 {
		t.Error("expected at least one node")
	}
	if len(resp.Edges) == 0 {
		t.Error("expected at least one edge")
	}
}

func TestConfigReturnsGrafana(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/v1/config", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	var resp configResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Grafana.BaseURL != "https://grafana.example.com" {
		t.Errorf("Grafana.BaseURL = %q, want %q", resp.Grafana.BaseURL, "https://grafana.example.com")
	}
	if resp.Cache.TTL != 15 {
		t.Errorf("Cache.TTL = %d, want 15", resp.Cache.TTL)
	}
	if resp.Auth.Type != "none" {
		t.Errorf("Auth.Type = %q, want %q", resp.Auth.Type, "none")
	}
	if resp.Alerts.Enabled {
		t.Error("Alerts.Enabled = true, want false (no AlertManager URL configured)")
	}
	if len(resp.Alerts.SeverityLevels) != 3 {
		t.Fatalf("Alerts.SeverityLevels = %d, want 3", len(resp.Alerts.SeverityLevels))
	}
	if resp.Alerts.SeverityLevels[0].Value != "critical" {
		t.Errorf("SeverityLevels[0].Value = %q, want %q", resp.Alerts.SeverityLevels[0].Value, "critical")
	}
	if resp.Alerts.SeverityLevels[0].Color != "#f44336" {
		t.Errorf("SeverityLevels[0].Color = %q, want %q", resp.Alerts.SeverityLevels[0].Color, "#f44336")
	}
}

func TestConfigAlertsEnabled(t *testing.T) {
	promSrv := newTestPromServer()
	cfg := &config.Config{
		Server: config.ServerConfig{Listen: ":8080"},
		Datasources: config.DatasourcesConfig{
			Prometheus:   config.PrometheusConfig{URL: promSrv.URL},
			Alertmanager: config.AlertmanagerConfig{URL: "http://am:9093"},
		},
		Cache: config.CacheConfig{TTL: 15 * time.Second},
		Auth:  config.AuthConfig{Type: "none"},
		Alerts: config.AlertsConfig{
			SeverityLabel: "severity",
			SeverityLevels: []config.SeverityLevel{
				{Value: "critical", Color: "#f44336"},
			},
		},
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	promClient := topology.NewPrometheusClient(topology.PrometheusConfig{URL: promSrv.URL})
	grafanaCfg := topology.GrafanaConfig{}
	builder := topology.NewGraphBuilder(promClient, nil, grafanaCfg, cfg.Cache.TTL, 0, logger, cfg.Alerts.SeverityLevels)
	topologyCache := cache.New(cfg.Cache.TTL)
	authenticator, _ := auth.NewFromConfig(config.AuthConfig{Type: "none"})
	srv := New(cfg, logger, builder, promClient, nil, topologyCache, authenticator)

	req := httptest.NewRequest("GET", "/api/v1/config", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	var resp configResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Alerts.Enabled {
		t.Error("Alerts.Enabled = false, want true (AlertManager URL configured)")
	}
}

func TestHealthzJSON(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

func TestTopologyETag(t *testing.T) {
	srv := newTestServer()

	// First request — should return 200 with ETag
	req1 := httptest.NewRequest("GET", "/api/v1/topology", nil)
	w1 := httptest.NewRecorder()
	srv.router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", w1.Code)
	}

	etag := w1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header on first request")
	}

	// Second request with matching If-None-Match — should return 304
	req2 := httptest.NewRequest("GET", "/api/v1/topology", nil)
	req2.Header.Set("If-None-Match", etag)
	w2 := httptest.NewRecorder()
	srv.router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNotModified {
		t.Errorf("second request status = %d, want 304", w2.Code)
	}
	if w2.Body.Len() != 0 {
		t.Errorf("304 response should have empty body, got %d bytes", w2.Body.Len())
	}

	// Third request with wrong ETag — should return 200
	req3 := httptest.NewRequest("GET", "/api/v1/topology", nil)
	req3.Header.Set("If-None-Match", `"wrong-etag"`)
	w3 := httptest.NewRecorder()
	srv.router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Errorf("third request status = %d, want 200", w3.Code)
	}
	if w3.Header().Get("ETag") == "" {
		t.Error("expected ETag header on third request")
	}
}

func TestTopologyNamespaceBypassesCache(t *testing.T) {
	srv := newTestServer()

	// First request without namespace — should return 200 with ETag.
	req1 := httptest.NewRequest("GET", "/api/v1/topology", nil)
	w1 := httptest.NewRecorder()
	srv.router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("unfiltered request status = %d, want 200", w1.Code)
	}
	if w1.Header().Get("ETag") == "" {
		t.Fatal("expected ETag header on unfiltered request")
	}

	// Namespace-filtered request — should return 200 without ETag.
	req2 := httptest.NewRequest("GET", "/api/v1/topology?namespace=default", nil)
	w2 := httptest.NewRecorder()
	srv.router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("namespace-filtered request status = %d, want 200", w2.Code)
	}
	if w2.Header().Get("ETag") != "" {
		t.Error("namespace-filtered request should not have ETag header")
	}

	var resp topology.TopologyResponse
	if err := json.NewDecoder(w2.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode namespace-filtered response: %v", err)
	}
	if len(resp.Nodes) == 0 {
		t.Error("expected nodes in namespace-filtered response")
	}
}

func TestCascadeAnalysisReturnsJSON(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/v1/cascade-analysis", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var result map[string]any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	// Verify all expected top-level fields are present.
	for _, field := range []string{"rootCauses", "affectedServices", "allFailures", "cascadeChains", "summary"} {
		if _, ok := result[field]; !ok {
			t.Errorf("missing field %q in response", field)
		}
	}
}

func TestCascadeAnalysisInvalidDepth(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/v1/cascade-analysis?depth=abc", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCascadeGraphReturnsJSON(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/v1/cascade-graph", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var result map[string]any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	for _, field := range []string{"nodes", "edges"} {
		v, ok := result[field]
		if !ok {
			t.Errorf("missing field %q in response", field)
			continue
		}
		if _, isArr := v.([]any); !isArr {
			t.Errorf("field %q should be an array", field)
		}
	}
}

func TestCascadeGraphWithServiceFilter(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/v1/cascade-graph?service=svc-go", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var result map[string]any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if _, ok := result["nodes"]; !ok {
		t.Error("missing nodes field")
	}
	if _, ok := result["edges"]; !ok {
		t.Error("missing edges field")
	}
}

func TestCascadeGraphInvalidDepth(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/v1/cascade-graph?depth=xyz", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestTopologyHistoryMode(t *testing.T) {
	srv := newTestServer()

	// Valid RFC3339 time parameter.
	req := httptest.NewRequest("GET", "/api/v1/topology?time=2026-01-15T12:00:00Z", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp topology.TopologyResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Meta.IsHistory {
		t.Error("Meta.IsHistory = false, want true")
	}
	if resp.Meta.Time == nil {
		t.Fatal("Meta.Time = nil, want non-nil")
	}

	// Historical requests should not have ETag.
	if w.Header().Get("ETag") != "" {
		t.Error("historical request should not have ETag header")
	}
}

func TestTopologyHistoryModeBypassesCache(t *testing.T) {
	srv := newTestServer()

	// First request — populate cache.
	req1 := httptest.NewRequest("GET", "/api/v1/topology", nil)
	w1 := httptest.NewRecorder()
	srv.router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", w1.Code)
	}
	etag := w1.Header().Get("ETag")

	// Historical request with matching ETag — should return 200, not 304.
	req2 := httptest.NewRequest("GET", "/api/v1/topology?time=2026-01-15T12:00:00Z", nil)
	req2.Header.Set("If-None-Match", etag)
	w2 := httptest.NewRecorder()
	srv.router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("historical request status = %d, want 200 (bypass cache)", w2.Code)
	}
}

func TestTopologyInvalidTimeParam(t *testing.T) {
	srv := newTestServer()

	req := httptest.NewRequest("GET", "/api/v1/topology?time=not-a-date", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCascadeAnalysisHistoryMode(t *testing.T) {
	srv := newTestServer()

	req := httptest.NewRequest("GET", "/api/v1/cascade-analysis?time=2026-01-15T12:00:00Z", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

func TestCascadeAnalysisInvalidTimeParam(t *testing.T) {
	srv := newTestServer()

	req := httptest.NewRequest("GET", "/api/v1/cascade-analysis?time=bad", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestTimelineEventsMissingParams(t *testing.T) {
	srv := newTestServer()

	tests := []struct {
		name string
		url  string
	}{
		{"no params", "/api/v1/timeline/events"},
		{"only start", "/api/v1/timeline/events?start=2026-01-15T12:00:00Z"},
		{"only end", "/api/v1/timeline/events?end=2026-01-15T13:00:00Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			w := httptest.NewRecorder()
			srv.router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestTimelineEventsInvalidFormat(t *testing.T) {
	srv := newTestServer()

	tests := []struct {
		name string
		url  string
	}{
		{"bad start", "/api/v1/timeline/events?start=bad&end=2026-01-15T13:00:00Z"},
		{"bad end", "/api/v1/timeline/events?start=2026-01-15T12:00:00Z&end=bad"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			w := httptest.NewRecorder()
			srv.router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestTimelineEventsStartAfterEnd(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/v1/timeline/events?start=2026-01-15T14:00:00Z&end=2026-01-15T12:00:00Z", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestTimelineEventsReturnsJSON(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/v1/timeline/events?start=2026-01-15T12:00:00Z&end=2026-01-15T13:00:00Z", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	// Response should be a JSON array.
	var events []any
	if err := json.NewDecoder(w.Body).Decode(&events); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	// With the mock server returning simple data, we expect an empty array.
	if events == nil {
		t.Error("expected non-nil array (even if empty)")
	}
}

func TestCORSHeaders(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("OPTIONS", "/api/v1/topology", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("expected Access-Control-Allow-Origin header")
	}
}
