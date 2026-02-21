package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/BigKAA/dephealth-ui/internal/logging"
	"gopkg.in/yaml.v3"
)

var hexColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Config holds the complete application configuration.
type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Datasources DatasourcesConfig `yaml:"datasources"`
	Cache       CacheConfig       `yaml:"cache"`
	Topology    TopologyConfig    `yaml:"topology"`
	Auth        AuthConfig        `yaml:"auth"`
	Grafana     GrafanaConfig     `yaml:"grafana"`
	Alerts      AlertsConfig      `yaml:"alerts"`
	Log         logging.LogConfig `yaml:"log"`
}

// AlertsConfig holds alert severity display settings.
type AlertsConfig struct {
	SeverityLabel  string          `yaml:"severityLabel"`
	SeverityLevels []SeverityLevel `yaml:"severityLevels"`
}

// SeverityLevel defines a single alert severity level with its display color.
type SeverityLevel struct {
	Value string `yaml:"value" json:"value"`
	Color string `yaml:"color" json:"color"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Listen string `yaml:"listen"`
}

// DatasourcesConfig holds external datasource connection settings.
type DatasourcesConfig struct {
	Prometheus  PrometheusConfig  `yaml:"prometheus"`
	Alertmanager AlertmanagerConfig `yaml:"alertmanager"`
}

// PrometheusConfig holds Prometheus/VictoriaMetrics connection settings.
type PrometheusConfig struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// AlertmanagerConfig holds AlertManager connection settings.
type AlertmanagerConfig struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// CacheConfig holds cache settings.
type CacheConfig struct {
	TTL time.Duration `yaml:"ttl"`
}

// TopologyConfig holds topology graph settings.
type TopologyConfig struct {
	// Lookback window for retaining stale nodes.
	// Uses last_over_time() to keep nodes visible after metrics disappear.
	// Set to 0 to disable (default: show only current metrics).
	Lookback time.Duration `yaml:"lookback"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	Type  string      `yaml:"type"`
	Basic BasicConfig `yaml:"basic"`
	OIDC  OIDCConfig  `yaml:"oidc"`
}

// OIDCConfig holds OpenID Connect authentication settings.
type OIDCConfig struct {
	Issuer       string `yaml:"issuer"`
	ClientID     string `yaml:"clientId"`
	ClientSecret string `yaml:"clientSecret"`
	RedirectURL  string `yaml:"redirectUrl"`
}

// BasicConfig holds HTTP Basic authentication settings.
type BasicConfig struct {
	Users []BasicUser `yaml:"users"`
}

// BasicUser represents a single Basic auth user.
type BasicUser struct {
	Username     string `yaml:"username"`
	PasswordHash string `yaml:"passwordHash"`
}

// GrafanaConfig holds Grafana integration settings.
type GrafanaConfig struct {
	BaseURL    string           `yaml:"baseUrl"`
	Token      string           `yaml:"token"`    // API key or service account token
	Username   string           `yaml:"username"` // Basic auth
	Password   string           `yaml:"password"` // Basic auth
	Dashboards DashboardsConfig `yaml:"dashboards"`
}

// DashboardsConfig holds Grafana dashboard UIDs.
type DashboardsConfig struct {
	ServiceStatus   string `yaml:"serviceStatus"`
	LinkStatus      string `yaml:"linkStatus"`
	ServiceList     string `yaml:"serviceList"`
	ServicesStatus  string `yaml:"servicesStatus"`
	LinksStatus     string `yaml:"linksStatus"`
	CascadeOverview        string `yaml:"cascadeOverview"`
	RootCause              string `yaml:"rootCause"`
	ConnectionDiagnostics  string `yaml:"connectionDiagnostics"`
}

// Load reads a YAML config file and applies environment variable overrides.
func Load(path string) (*Config, error) {
	cfg := defaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
		// Config file not found — use defaults + env overrides.
	} else {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config file: %w", err)
		}
	}

	applyEnvOverrides(cfg)

	return cfg, nil
}

// Validate checks that all required configuration fields are set.
func (c *Config) Validate() error {
	if c.Datasources.Prometheus.URL == "" {
		return fmt.Errorf("datasources.prometheus.url is required")
	}
	if c.Server.Listen == "" {
		return fmt.Errorf("server.listen is required")
	}
	if c.Topology.Lookback < 0 {
		return fmt.Errorf("topology.lookback must not be negative")
	}
	if c.Topology.Lookback > 0 && c.Topology.Lookback < time.Minute {
		return fmt.Errorf("topology.lookback must be at least 1m (got %s)", c.Topology.Lookback)
	}

	switch c.Auth.Type {
	case "none", "":
		// ok
	case "basic":
		if len(c.Auth.Basic.Users) == 0 {
			return fmt.Errorf("auth.basic.users must not be empty when auth.type is \"basic\"")
		}
		for i, u := range c.Auth.Basic.Users {
			if u.Username == "" {
				return fmt.Errorf("auth.basic.users[%d].username is required", i)
			}
			if u.PasswordHash == "" {
				return fmt.Errorf("auth.basic.users[%d].passwordHash is required", i)
			}
		}
	case "oidc":
		if c.Auth.OIDC.Issuer == "" {
			return fmt.Errorf("auth.oidc.issuer is required when auth.type is \"oidc\"")
		}
		if c.Auth.OIDC.ClientID == "" {
			return fmt.Errorf("auth.oidc.clientId is required when auth.type is \"oidc\"")
		}
		if c.Auth.OIDC.RedirectURL == "" {
			return fmt.Errorf("auth.oidc.redirectUrl is required when auth.type is \"oidc\"")
		}
	default:
		return fmt.Errorf("unknown auth.type: %q (supported: none, basic, oidc)", c.Auth.Type)
	}

	// Validate log config.
	switch c.Log.Format {
	case "text", "json", "":
	default:
		return fmt.Errorf("log.format %q is invalid (expected text/json)", c.Log.Format)
	}
	if c.Log.Level != "" {
		var level slog.Level
		if err := level.UnmarshalText([]byte(strings.ToUpper(c.Log.Level))); err != nil {
			return fmt.Errorf("log.level %q is invalid (expected debug/info/warn/error)", c.Log.Level)
		}
	}
	switch c.Log.TimeFormat {
	case "rfc3339", "rfc3339nano", "unix", "unixmilli", "":
	default:
		return fmt.Errorf("log.timeFormat %q is invalid (expected rfc3339/rfc3339nano/unix/unixmilli)", c.Log.TimeFormat)
	}

	// Validate alerts config.
	if len(c.Alerts.SeverityLevels) == 0 {
		return fmt.Errorf("alerts.severityLevels must not be empty")
	}
	for i, level := range c.Alerts.SeverityLevels {
		if level.Value == "" {
			return fmt.Errorf("alerts.severityLevels[%d].value is required", i)
		}
		if level.Color == "" {
			return fmt.Errorf("alerts.severityLevels[%d].color is required", i)
		}
		if !hexColorRe.MatchString(level.Color) {
			return fmt.Errorf("alerts.severityLevels[%d].color %q is not a valid hex color (#RRGGBB)", i, level.Color)
		}
	}

	return nil
}

func defaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Listen: ":8080",
		},
		Cache: CacheConfig{
			TTL: 15 * time.Second,
		},
		Auth: AuthConfig{
			Type: "none",
		},
		Alerts: AlertsConfig{
			SeverityLabel: "severity",
			SeverityLevels: []SeverityLevel{
				{Value: "critical", Color: "#f44336"},
				{Value: "warning", Color: "#ff9800"},
				{Value: "info", Color: "#2196f3"},
			},
		},
		Log: logging.LogConfig{
			Format:     "json",
			Level:      "info",
			TimeFormat: "rfc3339nano",
		},
	}
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("DEPHEALTH_SERVER_LISTEN"); v != "" {
		cfg.Server.Listen = v
	}
	if v := os.Getenv("DEPHEALTH_DATASOURCES_PROMETHEUS_URL"); v != "" {
		cfg.Datasources.Prometheus.URL = v
	}
	if v := os.Getenv("DEPHEALTH_DATASOURCES_ALERTMANAGER_URL"); v != "" {
		cfg.Datasources.Alertmanager.URL = v
	}
	if v := os.Getenv("DEPHEALTH_CACHE_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Cache.TTL = d
		}
	}
	if v := os.Getenv("DEPHEALTH_TOPOLOGY_LOOKBACK"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Topology.Lookback = d
		}
	}
	if v := os.Getenv("DEPHEALTH_AUTH_TYPE"); v != "" {
		cfg.Auth.Type = v
	}
	if v := os.Getenv("DEPHEALTH_AUTH_OIDC_ISSUER"); v != "" {
		cfg.Auth.OIDC.Issuer = v
	}
	if v := os.Getenv("DEPHEALTH_AUTH_OIDC_CLIENTID"); v != "" {
		cfg.Auth.OIDC.ClientID = v
	}
	if v := os.Getenv("DEPHEALTH_AUTH_OIDC_CLIENTSECRET"); v != "" {
		cfg.Auth.OIDC.ClientSecret = v
	}
	if v := os.Getenv("DEPHEALTH_AUTH_OIDC_REDIRECTURL"); v != "" {
		cfg.Auth.OIDC.RedirectURL = v
	}
	if v := os.Getenv("DEPHEALTH_GRAFANA_BASEURL"); v != "" {
		cfg.Grafana.BaseURL = v
	}
	if v := os.Getenv("DEPHEALTH_GRAFANA_TOKEN"); v != "" {
		cfg.Grafana.Token = v
	}
	if v := os.Getenv("DEPHEALTH_GRAFANA_USERNAME"); v != "" {
		cfg.Grafana.Username = v
	}
	if v := os.Getenv("DEPHEALTH_GRAFANA_PASSWORD"); v != "" {
		cfg.Grafana.Password = v
	}
	if v := os.Getenv("DEPHEALTH_ALERTS_SEVERITYLABEL"); v != "" {
		cfg.Alerts.SeverityLabel = v
	}
	if v := os.Getenv("DEPHEALTH_ALERTS_SEVERITYLEVELS"); v != "" {
		var levels []SeverityLevel
		if err := json.Unmarshal([]byte(v), &levels); err == nil {
			cfg.Alerts.SeverityLevels = levels
		}
	}

	// Log overrides.
	if v := os.Getenv("LOG_FORMAT"); v != "" {
		cfg.Log.Format = strings.ToLower(v)
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Log.Level = strings.ToLower(v)
	}
	if v := os.Getenv("LOG_TIME_FORMAT"); v != "" {
		cfg.Log.TimeFormat = strings.ToLower(v)
	}
	if v := os.Getenv("LOG_ADD_SOURCE"); v != "" {
		cfg.Log.AddSource = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("LOG_TIME_KEY"); v != "" {
		cfg.Log.TimeKey = v
	}
	if v := os.Getenv("LOG_LEVEL_KEY"); v != "" {
		cfg.Log.LevelKey = v
	}
	if v := os.Getenv("LOG_MESSAGE_KEY"); v != "" {
		cfg.Log.MessageKey = v
	}
	if v := os.Getenv("LOG_SOURCE_KEY"); v != "" {
		cfg.Log.SourceKey = v
	}
}
