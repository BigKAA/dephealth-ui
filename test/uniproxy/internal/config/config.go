// Package config provides environment variable configuration parsing for uniproxy.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the top-level application configuration.
type Config struct {
	Name          string
	ListenAddr    string
	LogLevel      string
	CheckInterval time.Duration
	Dependencies  []Dependency
}

// Dependency describes a single dependency to health-check.
type Dependency struct {
	Name       string
	Type       string // http, redis, postgres, grpc
	URL        string
	Host       string
	Port       string
	Critical   bool
	HealthPath string // HTTP-specific
}

// Load parses configuration from environment variables.
//
// Required:
//   - DEPHEALTH_NAME — application name
//   - DEPHEALTH_DEPS — comma-separated "name:type" pairs
//
// Optional:
//   - LISTEN_ADDR (default ":8080")
//   - LOG_LEVEL (default "info")
//   - DEPHEALTH_CHECK_INTERVAL — seconds (default "10")
//
// Per-dependency (NAME is uppercase with hyphens replaced by underscores):
//   - DEPHEALTH_<NAME>_URL or DEPHEALTH_<NAME>_HOST + DEPHEALTH_<NAME>_PORT
//   - DEPHEALTH_<NAME>_CRITICAL — "yes" or "no"
//   - DEPHEALTH_<NAME>_HEALTH_PATH — HTTP health check path
func Load() (*Config, error) {
	cfg := &Config{
		ListenAddr: getEnv("LISTEN_ADDR", ":8080"),
		LogLevel:   getEnv("LOG_LEVEL", "info"),
	}

	// Application name.
	cfg.Name = os.Getenv("DEPHEALTH_NAME")
	if cfg.Name == "" {
		return nil, fmt.Errorf("DEPHEALTH_NAME is required")
	}

	// Check interval.
	intervalStr := getEnv("DEPHEALTH_CHECK_INTERVAL", "10")
	sec, err := strconv.ParseFloat(intervalStr, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid DEPHEALTH_CHECK_INTERVAL %q: %w", intervalStr, err)
	}
	cfg.CheckInterval = time.Duration(sec * float64(time.Second))

	// Dependencies.
	depsStr := os.Getenv("DEPHEALTH_DEPS")
	if depsStr == "" {
		return nil, fmt.Errorf("DEPHEALTH_DEPS is required")
	}
	deps, err := parseDeps(depsStr)
	if err != nil {
		return nil, err
	}
	cfg.Dependencies = deps

	return cfg, nil
}

// parseDeps parses "name1:type1,name2:type2,..." into a slice of Dependency.
func parseDeps(s string) ([]Dependency, error) {
	pairs := strings.Split(s, ",")
	deps := make([]Dependency, 0, len(pairs))

	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid dependency format %q, expected name:type", pair)
		}
		name := parts[0]
		depType := parts[1]

		switch depType {
		case "http", "redis", "postgres", "grpc":
		default:
			return nil, fmt.Errorf("dependency %q: unsupported type %q", name, depType)
		}

		dep, err := parseSingleDep(name, depType)
		if err != nil {
			return nil, err
		}
		deps = append(deps, dep)
	}

	if len(deps) == 0 {
		return nil, fmt.Errorf("DEPHEALTH_DEPS: no dependencies defined")
	}
	return deps, nil
}

// parseSingleDep reads per-dependency env vars for a given dependency.
func parseSingleDep(name, depType string) (Dependency, error) {
	prefix := "DEPHEALTH_" + EnvName(name) + "_"

	dep := Dependency{
		Name: name,
		Type: depType,
	}

	// URL or Host+Port.
	dep.URL = os.Getenv(prefix + "URL")
	if dep.URL == "" {
		dep.Host = os.Getenv(prefix + "HOST")
		dep.Port = os.Getenv(prefix + "PORT")
		if dep.Host == "" || dep.Port == "" {
			return dep, fmt.Errorf("dependency %q: either %sURL or %sHOST + %sPORT is required",
				name, prefix, prefix, prefix)
		}
	}

	// Critical flag.
	critStr := os.Getenv(prefix + "CRITICAL")
	switch strings.ToLower(critStr) {
	case "yes", "true", "1":
		dep.Critical = true
	case "no", "false", "0":
		dep.Critical = false
	case "":
		return dep, fmt.Errorf("dependency %q: %sCRITICAL is required (yes/no)", name, prefix)
	default:
		return dep, fmt.Errorf("dependency %q: invalid %sCRITICAL value %q (expected yes/no)",
			name, prefix, critStr)
	}

	// HTTP-specific options.
	if depType == "http" {
		dep.HealthPath = os.Getenv(prefix + "HEALTH_PATH")
	}

	return dep, nil
}

// EnvName converts a dependency name to environment variable format:
// "uniproxy-02" → "UNIPROXY_02".
func EnvName(name string) string {
	return strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
