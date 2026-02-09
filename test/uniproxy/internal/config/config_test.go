package config

import (
	"os"
	"testing"
	"time"
)

// setEnvs sets env vars and returns a cleanup function.
func setEnvs(t *testing.T, envs map[string]string) {
	t.Helper()
	for k, v := range envs {
		t.Setenv(k, v)
	}
}

func TestLoad_MinimalValid(t *testing.T) {
	setEnvs(t, map[string]string{
		"DEPHEALTH_NAME":              "test-app",
		"DEPHEALTH_DEPS":              "svc:http",
		"DEPHEALTH_SVC_URL":           "http://svc.default.svc:8080",
		"DEPHEALTH_SVC_CRITICAL":      "yes",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "test-app" {
		t.Errorf("Name = %q, want %q", cfg.Name, "test-app")
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":8080")
	}
	if cfg.CheckInterval != 10*time.Second {
		t.Errorf("CheckInterval = %v, want %v", cfg.CheckInterval, 10*time.Second)
	}
	if len(cfg.Dependencies) != 1 {
		t.Fatalf("len(Dependencies) = %d, want 1", len(cfg.Dependencies))
	}
	dep := cfg.Dependencies[0]
	if dep.Name != "svc" || dep.Type != "http" || dep.URL != "http://svc.default.svc:8080" || !dep.Critical {
		t.Errorf("dep = %+v", dep)
	}
}

func TestLoad_MultipleDeps(t *testing.T) {
	setEnvs(t, map[string]string{
		"DEPHEALTH_NAME":                    "uniproxy-01",
		"DEPHEALTH_DEPS":                    "uniproxy-02:http,redis:redis,postgresql:postgres,grpc-stub:grpc",
		"DEPHEALTH_UNIPROXY_02_URL":         "http://uniproxy-02.svc:8080",
		"DEPHEALTH_UNIPROXY_02_CRITICAL":    "yes",
		"DEPHEALTH_UNIPROXY_02_HEALTH_PATH": "/readyz",
		"DEPHEALTH_REDIS_URL":               "redis://redis.svc:6379",
		"DEPHEALTH_REDIS_CRITICAL":          "no",
		"DEPHEALTH_POSTGRESQL_URL":          "postgres://user:pass@pg.svc:5432/db",
		"DEPHEALTH_POSTGRESQL_CRITICAL":     "yes",
		"DEPHEALTH_GRPC_STUB_HOST":          "grpc.svc",
		"DEPHEALTH_GRPC_STUB_PORT":          "9090",
		"DEPHEALTH_GRPC_STUB_CRITICAL":      "no",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Dependencies) != 4 {
		t.Fatalf("len(Dependencies) = %d, want 4", len(cfg.Dependencies))
	}

	// HTTP dep with health path.
	http := cfg.Dependencies[0]
	if http.HealthPath != "/readyz" {
		t.Errorf("http dep HealthPath = %q, want %q", http.HealthPath, "/readyz")
	}
	if !http.Critical {
		t.Error("http dep should be critical")
	}

	// Redis dep.
	redis := cfg.Dependencies[1]
	if redis.Critical {
		t.Error("redis dep should not be critical")
	}

	// gRPC dep via host+port.
	grpc := cfg.Dependencies[3]
	if grpc.Host != "grpc.svc" || grpc.Port != "9090" {
		t.Errorf("grpc dep Host:Port = %s:%s, want grpc.svc:9090", grpc.Host, grpc.Port)
	}
	if grpc.URL != "" {
		t.Errorf("grpc dep URL should be empty, got %q", grpc.URL)
	}
}

func TestLoad_CustomInterval(t *testing.T) {
	setEnvs(t, map[string]string{
		"DEPHEALTH_NAME":           "app",
		"DEPHEALTH_DEPS":           "svc:http",
		"DEPHEALTH_SVC_URL":        "http://svc:80",
		"DEPHEALTH_SVC_CRITICAL":   "yes",
		"DEPHEALTH_CHECK_INTERVAL": "30",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CheckInterval != 30*time.Second {
		t.Errorf("CheckInterval = %v, want %v", cfg.CheckInterval, 30*time.Second)
	}
}

func TestLoad_MissingName(t *testing.T) {
	// Ensure DEPHEALTH_NAME is not set.
	os.Unsetenv("DEPHEALTH_NAME")
	setEnvs(t, map[string]string{
		"DEPHEALTH_DEPS":         "svc:http",
		"DEPHEALTH_SVC_URL":      "http://svc:80",
		"DEPHEALTH_SVC_CRITICAL": "yes",
	})

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing DEPHEALTH_NAME")
	}
}

func TestLoad_MissingDeps(t *testing.T) {
	os.Unsetenv("DEPHEALTH_DEPS")
	setEnvs(t, map[string]string{
		"DEPHEALTH_NAME": "app",
	})

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing DEPHEALTH_DEPS")
	}
}

func TestLoad_InvalidDepType(t *testing.T) {
	setEnvs(t, map[string]string{
		"DEPHEALTH_NAME": "app",
		"DEPHEALTH_DEPS": "svc:mysql",
	})

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestLoad_MissingCritical(t *testing.T) {
	os.Unsetenv("DEPHEALTH_SVC_CRITICAL")
	setEnvs(t, map[string]string{
		"DEPHEALTH_NAME":    "app",
		"DEPHEALTH_DEPS":    "svc:http",
		"DEPHEALTH_SVC_URL": "http://svc:80",
	})

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing CRITICAL")
	}
}

func TestLoad_MissingURL(t *testing.T) {
	os.Unsetenv("DEPHEALTH_SVC_URL")
	os.Unsetenv("DEPHEALTH_SVC_HOST")
	os.Unsetenv("DEPHEALTH_SVC_PORT")
	setEnvs(t, map[string]string{
		"DEPHEALTH_NAME":         "app",
		"DEPHEALTH_DEPS":         "svc:http",
		"DEPHEALTH_SVC_CRITICAL": "yes",
	})

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing URL/HOST+PORT")
	}
}

func TestEnvName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"uniproxy-02", "UNIPROXY_02"},
		{"redis", "REDIS"},
		{"grpc-stub", "GRPC_STUB"},
		{"postgresql", "POSTGRESQL"},
		{"my-long-service-name", "MY_LONG_SERVICE_NAME"},
	}
	for _, tt := range tests {
		got := EnvName(tt.input)
		if got != tt.want {
			t.Errorf("EnvName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
