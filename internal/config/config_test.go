package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadFromFile(t *testing.T) {
	content := `
server:
  listen: ":9090"
datasources:
  prometheus:
    url: "http://vm:8428"
    username: "user"
    password: "pass"
  alertmanager:
    url: "http://am:9093"
cache:
  ttl: 30s
auth:
  type: "basic"
grafana:
  baseUrl: "https://grafana.example.com"
  dashboards:
    serviceStatus: "svc-dash"
    linkStatus: "link-dash"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Server.Listen != ":9090" {
		t.Errorf("Server.Listen = %q, want %q", cfg.Server.Listen, ":9090")
	}
	if cfg.Datasources.Prometheus.URL != "http://vm:8428" {
		t.Errorf("Prometheus.URL = %q, want %q", cfg.Datasources.Prometheus.URL, "http://vm:8428")
	}
	if cfg.Datasources.Prometheus.Username != "user" {
		t.Errorf("Prometheus.Username = %q, want %q", cfg.Datasources.Prometheus.Username, "user")
	}
	if cfg.Datasources.Alertmanager.URL != "http://am:9093" {
		t.Errorf("Alertmanager.URL = %q, want %q", cfg.Datasources.Alertmanager.URL, "http://am:9093")
	}
	if cfg.Cache.TTL != 30*time.Second {
		t.Errorf("Cache.TTL = %v, want %v", cfg.Cache.TTL, 30*time.Second)
	}
	if cfg.Auth.Type != "basic" {
		t.Errorf("Auth.Type = %q, want %q", cfg.Auth.Type, "basic")
	}
	if cfg.Grafana.BaseURL != "https://grafana.example.com" {
		t.Errorf("Grafana.BaseURL = %q, want %q", cfg.Grafana.BaseURL, "https://grafana.example.com")
	}
	if cfg.Grafana.Dashboards.ServiceStatus != "svc-dash" {
		t.Errorf("Dashboards.ServiceStatus = %q, want %q", cfg.Grafana.Dashboards.ServiceStatus, "svc-dash")
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Server.Listen != ":8080" {
		t.Errorf("default Server.Listen = %q, want %q", cfg.Server.Listen, ":8080")
	}
	if cfg.Cache.TTL != 15*time.Second {
		t.Errorf("default Cache.TTL = %v, want %v", cfg.Cache.TTL, 15*time.Second)
	}
	if cfg.Auth.Type != "none" {
		t.Errorf("default Auth.Type = %q, want %q", cfg.Auth.Type, "none")
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("DEPHEALTH_SERVER_LISTEN", ":3000")
	t.Setenv("DEPHEALTH_DATASOURCES_PROMETHEUS_URL", "http://env-vm:8428")
	t.Setenv("DEPHEALTH_DATASOURCES_ALERTMANAGER_URL", "http://env-am:9093")
	t.Setenv("DEPHEALTH_CACHE_TTL", "45s")
	t.Setenv("DEPHEALTH_AUTH_TYPE", "oidc")
	t.Setenv("DEPHEALTH_GRAFANA_BASEURL", "https://env-grafana.example.com")

	cfg, err := Load("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Server.Listen != ":3000" {
		t.Errorf("Server.Listen = %q, want %q", cfg.Server.Listen, ":3000")
	}
	if cfg.Datasources.Prometheus.URL != "http://env-vm:8428" {
		t.Errorf("Prometheus.URL = %q, want %q", cfg.Datasources.Prometheus.URL, "http://env-vm:8428")
	}
	if cfg.Datasources.Alertmanager.URL != "http://env-am:9093" {
		t.Errorf("Alertmanager.URL = %q, want %q", cfg.Datasources.Alertmanager.URL, "http://env-am:9093")
	}
	if cfg.Cache.TTL != 45*time.Second {
		t.Errorf("Cache.TTL = %v, want %v", cfg.Cache.TTL, 45*time.Second)
	}
	if cfg.Auth.Type != "oidc" {
		t.Errorf("Auth.Type = %q, want %q", cfg.Auth.Type, "oidc")
	}
	if cfg.Grafana.BaseURL != "https://env-grafana.example.com" {
		t.Errorf("Grafana.BaseURL = %q, want %q", cfg.Grafana.BaseURL, "https://env-grafana.example.com")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				Server:      ServerConfig{Listen: ":8080"},
				Datasources: DatasourcesConfig{Prometheus: PrometheusConfig{URL: "http://vm:8428"}},
			},
			wantErr: false,
		},
		{
			name: "missing prometheus url",
			cfg: Config{
				Server: ServerConfig{Listen: ":8080"},
			},
			wantErr: true,
		},
		{
			name: "missing listen address",
			cfg: Config{
				Datasources: DatasourcesConfig{Prometheus: PrometheusConfig{URL: "http://vm:8428"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
