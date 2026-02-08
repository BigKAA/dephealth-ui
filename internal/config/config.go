package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the complete application configuration.
type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Datasources DatasourcesConfig `yaml:"datasources"`
	Cache       CacheConfig       `yaml:"cache"`
	Auth        AuthConfig        `yaml:"auth"`
	Grafana     GrafanaConfig     `yaml:"grafana"`
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

// AuthConfig holds authentication settings.
type AuthConfig struct {
	Type  string      `yaml:"type"`
	Basic BasicConfig `yaml:"basic"`
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
	Dashboards DashboardsConfig `yaml:"dashboards"`
}

// DashboardsConfig holds Grafana dashboard UIDs.
type DashboardsConfig struct {
	ServiceStatus string `yaml:"serviceStatus"`
	LinkStatus    string `yaml:"linkStatus"`
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
	default:
		return fmt.Errorf("unknown auth.type: %q (supported: none, basic)", c.Auth.Type)
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
	if v := os.Getenv("DEPHEALTH_AUTH_TYPE"); v != "" {
		cfg.Auth.Type = v
	}
	if v := os.Getenv("DEPHEALTH_GRAFANA_BASEURL"); v != "" {
		cfg.Grafana.BaseURL = v
	}
}
