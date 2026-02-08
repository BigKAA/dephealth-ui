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
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	promClient := topology.NewPrometheusClient(topology.PrometheusConfig{URL: promSrv.URL})
	grafanaCfg := topology.GrafanaConfig{
		BaseURL:              cfg.Grafana.BaseURL,
		ServiceStatusDashUID: cfg.Grafana.Dashboards.ServiceStatus,
		LinkStatusDashUID:    cfg.Grafana.Dashboards.LinkStatus,
	}
	builder := topology.NewGraphBuilder(promClient, nil, grafanaCfg, cfg.Cache.TTL, logger)
	topologyCache := cache.New(cfg.Cache.TTL)
	authenticator, _ := auth.NewFromConfig(config.AuthConfig{Type: "none"})
	return New(cfg, logger, builder, nil, topologyCache, authenticator)
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
