package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// validAlerts returns a valid AlertsConfig for use in test cases.
func validAlerts() AlertsConfig {
	return AlertsConfig{
		SeverityLabel: "severity",
		SeverityLevels: []SeverityLevel{
			{Value: "critical", Color: "#f44336"},
			{Value: "warning", Color: "#ff9800"},
			{Value: "info", Color: "#2196f3"},
		},
	}
}

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
	if cfg.Topology.Lookback != 0 {
		t.Errorf("default Topology.Lookback = %v, want 0", cfg.Topology.Lookback)
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("DEPHEALTH_SERVER_LISTEN", ":3000")
	t.Setenv("DEPHEALTH_DATASOURCES_PROMETHEUS_URL", "http://env-vm:8428")
	t.Setenv("DEPHEALTH_DATASOURCES_ALERTMANAGER_URL", "http://env-am:9093")
	t.Setenv("DEPHEALTH_CACHE_TTL", "45s")
	t.Setenv("DEPHEALTH_TOPOLOGY_LOOKBACK", "2h")
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
	if cfg.Topology.Lookback != 2*time.Hour {
		t.Errorf("Topology.Lookback = %v, want %v", cfg.Topology.Lookback, 2*time.Hour)
	}
	if cfg.Auth.Type != "oidc" {
		t.Errorf("Auth.Type = %q, want %q", cfg.Auth.Type, "oidc")
	}
	if cfg.Grafana.BaseURL != "https://env-grafana.example.com" {
		t.Errorf("Grafana.BaseURL = %q, want %q", cfg.Grafana.BaseURL, "https://env-grafana.example.com")
	}
}

func TestLoadTopologyFromYAML(t *testing.T) {
	content := `
server:
  listen: ":8080"
datasources:
  prometheus:
    url: "http://vm:8428"
topology:
  lookback: 1h
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

	if cfg.Topology.Lookback != time.Hour {
		t.Errorf("Topology.Lookback = %v, want %v", cfg.Topology.Lookback, time.Hour)
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
				Alerts:      validAlerts(),
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
		{
			name: "auth type basic valid",
			cfg: Config{
				Server:      ServerConfig{Listen: ":8080"},
				Datasources: DatasourcesConfig{Prometheus: PrometheusConfig{URL: "http://vm:8428"}},
				Auth: AuthConfig{
					Type: "basic",
					Basic: BasicConfig{
						Users: []BasicUser{{Username: "admin", PasswordHash: "$2a$10$hash"}},
					},
				},
				Alerts: validAlerts(),
			},
			wantErr: false,
		},
		{
			name: "auth type basic no users",
			cfg: Config{
				Server:      ServerConfig{Listen: ":8080"},
				Datasources: DatasourcesConfig{Prometheus: PrometheusConfig{URL: "http://vm:8428"}},
				Auth:        AuthConfig{Type: "basic"},
				Alerts:      validAlerts(),
			},
			wantErr: true,
		},
		{
			name: "auth type basic empty username",
			cfg: Config{
				Server:      ServerConfig{Listen: ":8080"},
				Datasources: DatasourcesConfig{Prometheus: PrometheusConfig{URL: "http://vm:8428"}},
				Auth: AuthConfig{
					Type: "basic",
					Basic: BasicConfig{
						Users: []BasicUser{{Username: "", PasswordHash: "$2a$10$hash"}},
					},
				},
				Alerts: validAlerts(),
			},
			wantErr: true,
		},
		{
			name: "auth type basic empty hash",
			cfg: Config{
				Server:      ServerConfig{Listen: ":8080"},
				Datasources: DatasourcesConfig{Prometheus: PrometheusConfig{URL: "http://vm:8428"}},
				Auth: AuthConfig{
					Type: "basic",
					Basic: BasicConfig{
						Users: []BasicUser{{Username: "admin", PasswordHash: ""}},
					},
				},
				Alerts: validAlerts(),
			},
			wantErr: true,
		},
		{
			name: "auth type oidc valid",
			cfg: Config{
				Server:      ServerConfig{Listen: ":8080"},
				Datasources: DatasourcesConfig{Prometheus: PrometheusConfig{URL: "http://vm:8428"}},
				Auth: AuthConfig{
					Type: "oidc",
					OIDC: OIDCConfig{
						Issuer:      "https://keycloak.example.com/realms/infra",
						ClientID:    "dephealth-ui",
						RedirectURL: "https://dephealth.example.com/auth/callback",
					},
				},
				Alerts: validAlerts(),
			},
			wantErr: false,
		},
		{
			name: "auth type oidc missing issuer",
			cfg: Config{
				Server:      ServerConfig{Listen: ":8080"},
				Datasources: DatasourcesConfig{Prometheus: PrometheusConfig{URL: "http://vm:8428"}},
				Auth: AuthConfig{
					Type: "oidc",
					OIDC: OIDCConfig{
						ClientID:    "dephealth-ui",
						RedirectURL: "https://dephealth.example.com/auth/callback",
					},
				},
				Alerts: validAlerts(),
			},
			wantErr: true,
		},
		{
			name: "auth type oidc missing clientId",
			cfg: Config{
				Server:      ServerConfig{Listen: ":8080"},
				Datasources: DatasourcesConfig{Prometheus: PrometheusConfig{URL: "http://vm:8428"}},
				Auth: AuthConfig{
					Type: "oidc",
					OIDC: OIDCConfig{
						Issuer:      "https://keycloak.example.com/realms/infra",
						RedirectURL: "https://dephealth.example.com/auth/callback",
					},
				},
				Alerts: validAlerts(),
			},
			wantErr: true,
		},
		{
			name: "auth type oidc missing redirectUrl",
			cfg: Config{
				Server:      ServerConfig{Listen: ":8080"},
				Datasources: DatasourcesConfig{Prometheus: PrometheusConfig{URL: "http://vm:8428"}},
				Auth: AuthConfig{
					Type: "oidc",
					OIDC: OIDCConfig{
						Issuer:   "https://keycloak.example.com/realms/infra",
						ClientID: "dephealth-ui",
					},
				},
				Alerts: validAlerts(),
			},
			wantErr: true,
		},
		{
			name: "unknown auth type",
			cfg: Config{
				Server:      ServerConfig{Listen: ":8080"},
				Datasources: DatasourcesConfig{Prometheus: PrometheusConfig{URL: "http://vm:8428"}},
				Auth:        AuthConfig{Type: "ldap"},
				Alerts:      validAlerts(),
			},
			wantErr: true,
		},
		// Topology validation test cases.
		{
			name: "topology lookback valid",
			cfg: Config{
				Server:      ServerConfig{Listen: ":8080"},
				Datasources: DatasourcesConfig{Prometheus: PrometheusConfig{URL: "http://vm:8428"}},
				Topology:    TopologyConfig{Lookback: time.Hour},
				Alerts:      validAlerts(),
			},
			wantErr: false,
		},
		{
			name: "topology lookback zero (disabled)",
			cfg: Config{
				Server:      ServerConfig{Listen: ":8080"},
				Datasources: DatasourcesConfig{Prometheus: PrometheusConfig{URL: "http://vm:8428"}},
				Topology:    TopologyConfig{Lookback: 0},
				Alerts:      validAlerts(),
			},
			wantErr: false,
		},
		{
			name: "topology lookback too small",
			cfg: Config{
				Server:      ServerConfig{Listen: ":8080"},
				Datasources: DatasourcesConfig{Prometheus: PrometheusConfig{URL: "http://vm:8428"}},
				Topology:    TopologyConfig{Lookback: 30 * time.Second},
				Alerts:      validAlerts(),
			},
			wantErr: true,
		},
		{
			name: "topology lookback negative",
			cfg: Config{
				Server:      ServerConfig{Listen: ":8080"},
				Datasources: DatasourcesConfig{Prometheus: PrometheusConfig{URL: "http://vm:8428"}},
				Topology:    TopologyConfig{Lookback: -time.Minute},
				Alerts:      validAlerts(),
			},
			wantErr: true,
		},
		// Alerts validation test cases.
		{
			name: "alerts severity levels empty",
			cfg: Config{
				Server:      ServerConfig{Listen: ":8080"},
				Datasources: DatasourcesConfig{Prometheus: PrometheusConfig{URL: "http://vm:8428"}},
				Alerts:      AlertsConfig{SeverityLabel: "severity"},
			},
			wantErr: true,
		},
		{
			name: "alerts severity level empty value",
			cfg: Config{
				Server:      ServerConfig{Listen: ":8080"},
				Datasources: DatasourcesConfig{Prometheus: PrometheusConfig{URL: "http://vm:8428"}},
				Alerts: AlertsConfig{
					SeverityLabel:  "severity",
					SeverityLevels: []SeverityLevel{{Value: "", Color: "#f44336"}},
				},
			},
			wantErr: true,
		},
		{
			name: "alerts severity level empty color",
			cfg: Config{
				Server:      ServerConfig{Listen: ":8080"},
				Datasources: DatasourcesConfig{Prometheus: PrometheusConfig{URL: "http://vm:8428"}},
				Alerts: AlertsConfig{
					SeverityLabel:  "severity",
					SeverityLevels: []SeverityLevel{{Value: "critical", Color: ""}},
				},
			},
			wantErr: true,
		},
		{
			name: "alerts severity level invalid color",
			cfg: Config{
				Server:      ServerConfig{Listen: ":8080"},
				Datasources: DatasourcesConfig{Prometheus: PrometheusConfig{URL: "http://vm:8428"}},
				Alerts: AlertsConfig{
					SeverityLabel:  "severity",
					SeverityLevels: []SeverityLevel{{Value: "critical", Color: "red"}},
				},
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

func TestLoadBasicAuthConfig(t *testing.T) {
	content := `
server:
  listen: ":8080"
datasources:
  prometheus:
    url: "http://vm:8428"
auth:
  type: "basic"
  basic:
    users:
      - username: admin
        passwordHash: "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012"
      - username: viewer
        passwordHash: "$2a$10$xyzxyzxyzxyzxyzxyzxyzxyzABCDEFGHIJKLMNOPQRSTUVWXYZ012"
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

	if cfg.Auth.Type != "basic" {
		t.Errorf("Auth.Type = %q, want %q", cfg.Auth.Type, "basic")
	}
	if len(cfg.Auth.Basic.Users) != 2 {
		t.Fatalf("got %d users, want 2", len(cfg.Auth.Basic.Users))
	}
	if cfg.Auth.Basic.Users[0].Username != "admin" {
		t.Errorf("Users[0].Username = %q, want %q", cfg.Auth.Basic.Users[0].Username, "admin")
	}
	if cfg.Auth.Basic.Users[1].Username != "viewer" {
		t.Errorf("Users[1].Username = %q, want %q", cfg.Auth.Basic.Users[1].Username, "viewer")
	}
}

func TestLoadOIDCConfig(t *testing.T) {
	content := `
server:
  listen: ":8080"
datasources:
  prometheus:
    url: "http://vm:8428"
auth:
  type: "oidc"
  oidc:
    issuer: "https://keycloak.example.com/realms/infra"
    clientId: "dephealth-ui"
    clientSecret: "my-secret"
    redirectUrl: "https://dephealth.example.com/auth/callback"
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

	if cfg.Auth.Type != "oidc" {
		t.Errorf("Auth.Type = %q, want %q", cfg.Auth.Type, "oidc")
	}
	if cfg.Auth.OIDC.Issuer != "https://keycloak.example.com/realms/infra" {
		t.Errorf("OIDC.Issuer = %q, want %q", cfg.Auth.OIDC.Issuer, "https://keycloak.example.com/realms/infra")
	}
	if cfg.Auth.OIDC.ClientID != "dephealth-ui" {
		t.Errorf("OIDC.ClientID = %q, want %q", cfg.Auth.OIDC.ClientID, "dephealth-ui")
	}
	if cfg.Auth.OIDC.ClientSecret != "my-secret" {
		t.Errorf("OIDC.ClientSecret = %q, want %q", cfg.Auth.OIDC.ClientSecret, "my-secret")
	}
	if cfg.Auth.OIDC.RedirectURL != "https://dephealth.example.com/auth/callback" {
		t.Errorf("OIDC.RedirectURL = %q, want %q", cfg.Auth.OIDC.RedirectURL, "https://dephealth.example.com/auth/callback")
	}
}

func TestOIDCEnvOverrides(t *testing.T) {
	t.Setenv("DEPHEALTH_AUTH_OIDC_ISSUER", "https://env-keycloak.example.com/realms/test")
	t.Setenv("DEPHEALTH_AUTH_OIDC_CLIENTID", "env-client")
	t.Setenv("DEPHEALTH_AUTH_OIDC_CLIENTSECRET", "env-secret")
	t.Setenv("DEPHEALTH_AUTH_OIDC_REDIRECTURL", "https://env-app.example.com/auth/callback")

	cfg, err := Load("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Auth.OIDC.Issuer != "https://env-keycloak.example.com/realms/test" {
		t.Errorf("OIDC.Issuer = %q, want env value", cfg.Auth.OIDC.Issuer)
	}
	if cfg.Auth.OIDC.ClientID != "env-client" {
		t.Errorf("OIDC.ClientID = %q, want %q", cfg.Auth.OIDC.ClientID, "env-client")
	}
	if cfg.Auth.OIDC.ClientSecret != "env-secret" {
		t.Errorf("OIDC.ClientSecret = %q, want %q", cfg.Auth.OIDC.ClientSecret, "env-secret")
	}
	if cfg.Auth.OIDC.RedirectURL != "https://env-app.example.com/auth/callback" {
		t.Errorf("OIDC.RedirectURL = %q, want env value", cfg.Auth.OIDC.RedirectURL)
	}
}

func TestDefaultAlertsConfig(t *testing.T) {
	cfg, err := Load("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Alerts.SeverityLabel != "severity" {
		t.Errorf("Alerts.SeverityLabel = %q, want %q", cfg.Alerts.SeverityLabel, "severity")
	}
	if len(cfg.Alerts.SeverityLevels) != 3 {
		t.Fatalf("got %d severity levels, want 3", len(cfg.Alerts.SeverityLevels))
	}
	if cfg.Alerts.SeverityLevels[0].Value != "critical" {
		t.Errorf("SeverityLevels[0].Value = %q, want %q", cfg.Alerts.SeverityLevels[0].Value, "critical")
	}
	if cfg.Alerts.SeverityLevels[0].Color != "#f44336" {
		t.Errorf("SeverityLevels[0].Color = %q, want %q", cfg.Alerts.SeverityLevels[0].Color, "#f44336")
	}
	if cfg.Alerts.SeverityLevels[1].Value != "warning" {
		t.Errorf("SeverityLevels[1].Value = %q, want %q", cfg.Alerts.SeverityLevels[1].Value, "warning")
	}
	if cfg.Alerts.SeverityLevels[2].Value != "info" {
		t.Errorf("SeverityLevels[2].Value = %q, want %q", cfg.Alerts.SeverityLevels[2].Value, "info")
	}
}

func TestLoadAlertsFromYAML(t *testing.T) {
	content := `
server:
  listen: ":8080"
datasources:
  prometheus:
    url: "http://vm:8428"
alerts:
  severityLabel: "priority"
  severityLevels:
    - value: "p1"
      color: "#ff0000"
    - value: "p2"
      color: "#ffaa00"
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

	if cfg.Alerts.SeverityLabel != "priority" {
		t.Errorf("Alerts.SeverityLabel = %q, want %q", cfg.Alerts.SeverityLabel, "priority")
	}
	if len(cfg.Alerts.SeverityLevels) != 2 {
		t.Fatalf("got %d severity levels, want 2", len(cfg.Alerts.SeverityLevels))
	}
	if cfg.Alerts.SeverityLevels[0].Value != "p1" {
		t.Errorf("SeverityLevels[0].Value = %q, want %q", cfg.Alerts.SeverityLevels[0].Value, "p1")
	}
	if cfg.Alerts.SeverityLevels[0].Color != "#ff0000" {
		t.Errorf("SeverityLevels[0].Color = %q, want %q", cfg.Alerts.SeverityLevels[0].Color, "#ff0000")
	}
}

func TestAlertsEnvOverrides(t *testing.T) {
	t.Setenv("DEPHEALTH_ALERTS_SEVERITYLABEL", "level")
	t.Setenv("DEPHEALTH_ALERTS_SEVERITYLEVELS", `[{"value":"high","color":"#ff0000"},{"value":"low","color":"#00ff00"}]`)

	cfg, err := Load("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Alerts.SeverityLabel != "level" {
		t.Errorf("Alerts.SeverityLabel = %q, want %q", cfg.Alerts.SeverityLabel, "level")
	}
	if len(cfg.Alerts.SeverityLevels) != 2 {
		t.Fatalf("got %d severity levels, want 2", len(cfg.Alerts.SeverityLevels))
	}
	if cfg.Alerts.SeverityLevels[0].Value != "high" {
		t.Errorf("SeverityLevels[0].Value = %q, want %q", cfg.Alerts.SeverityLevels[0].Value, "high")
	}
	if cfg.Alerts.SeverityLevels[1].Color != "#00ff00" {
		t.Errorf("SeverityLevels[1].Color = %q, want %q", cfg.Alerts.SeverityLevels[1].Color, "#00ff00")
	}
}
