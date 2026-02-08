// Package config provides YAML configuration parsing for uniproxy.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration.
type Config struct {
	Server        ServerConfig     `yaml:"server"`
	CheckInterval time.Duration   `yaml:"checkInterval"`
	Connections   []Connection     `yaml:"connections"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Listen      string `yaml:"listen"`
	MetricsPath string `yaml:"metricsPath"`
}

// Connection describes a single dependency to health-check.
type Connection struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`     // http, redis, postgres, grpc
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Path     string `yaml:"path,omitempty"`     // HTTP-specific
	Database string `yaml:"database,omitempty"` // Postgres-specific
	Username string `yaml:"username,omitempty"` // Postgres-specific
	Password string `yaml:"password,omitempty"` // Postgres-specific
}

// Load reads and parses a YAML config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	cfg := &Config{
		Server: ServerConfig{
			Listen:      ":8080",
			MetricsPath: "/metrics",
		},
		CheckInterval: 10 * time.Second,
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	for i, conn := range c.Connections {
		if conn.Name == "" {
			return fmt.Errorf("connection[%d]: name is required", i)
		}
		switch conn.Type {
		case "http", "redis", "postgres", "grpc":
		default:
			return fmt.Errorf("connection[%d] %q: unsupported type %q", i, conn.Name, conn.Type)
		}
		if conn.Host == "" {
			return fmt.Errorf("connection[%d] %q: host is required", i, conn.Name)
		}
		if conn.Port == "" {
			return fmt.Errorf("connection[%d] %q: port is required", i, conn.Name)
		}
	}
	return nil
}
