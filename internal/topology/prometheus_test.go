package topology

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const topologyEdgesResponse = `{
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [
      {
        "metric": {"name": "svc-go", "namespace": "default", "dependency": "postgres", "type": "postgres", "host": "pg-primary", "port": "5432", "critical": "yes"},
        "value": [1700000000, "1"]
      },
      {
        "metric": {"name": "svc-go", "namespace": "default", "dependency": "redis", "type": "redis", "host": "redis", "port": "6379", "critical": "no"},
        "value": [1700000000, "1"]
      },
      {
        "metric": {"name": "svc-python", "namespace": "default", "dependency": "postgres", "type": "postgres", "host": "pg-primary", "port": "5432", "critical": "yes"},
        "value": [1700000000, "1"]
      }
    ]
  }
}`

const healthStateResponse = `{
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [
      {
        "metric": {"name": "svc-go", "dependency": "postgres", "host": "pg-primary", "port": "5432"},
        "value": [1700000000, "1"]
      },
      {
        "metric": {"name": "svc-go", "dependency": "redis", "host": "redis", "port": "6379"},
        "value": [1700000000, "0"]
      },
      {
        "metric": {"name": "svc-python", "dependency": "postgres", "host": "pg-primary", "port": "5432"},
        "value": [1700000000, "1"]
      }
    ]
  }
}`

const latencyResponse = `{
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [
      {
        "metric": {"name": "svc-go", "dependency": "postgres", "host": "pg-primary", "port": "5432"},
        "value": [1700000000, "0.0052"]
      },
      {
        "metric": {"name": "svc-go", "dependency": "redis", "host": "redis", "port": "6379"},
        "value": [1700000000, "0.001"]
      }
    ]
  }
}`

func newTestPromServer(response string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	}))
}

func TestQueryTopologyEdges(t *testing.T) {
	srv := newTestPromServer(topologyEdgesResponse)
	defer srv.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: srv.URL})
	edges, err := client.QueryTopologyEdges(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("QueryTopologyEdges() error: %v", err)
	}

	if len(edges) != 3 {
		t.Fatalf("got %d edges, want 3", len(edges))
	}

	e := edges[0]
	if e.Name != "svc-go" || e.Dependency != "postgres" || e.Type != "postgres" {
		t.Errorf("edge[0] = %+v, unexpected", e)
	}
	if e.Namespace != "default" {
		t.Errorf("edge[0].Namespace = %q, want %q", e.Namespace, "default")
	}
	if e.Host != "pg-primary" || e.Port != "5432" {
		t.Errorf("edge[0].Host=%q Port=%q, want pg-primary:5432", e.Host, e.Port)
	}
	if !e.Critical {
		t.Errorf("edge[0].Critical = false, want true")
	}

	// Check non-critical edge.
	e1 := edges[1]
	if e1.Critical {
		t.Errorf("edge[1].Critical = true, want false (redis is non-critical)")
	}
}

func TestQueryHealthState(t *testing.T) {
	srv := newTestPromServer(healthStateResponse)
	defer srv.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: srv.URL})
	health, err := client.QueryHealthState(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("QueryHealthState() error: %v", err)
	}

	if len(health) != 3 {
		t.Fatalf("got %d entries, want 3", len(health))
	}

	key := EdgeKey{Name: "svc-go", Host: "redis", Port: "6379"}
	if v, ok := health[key]; !ok || v != 0 {
		t.Errorf("health[svc-go/redis:6379] = %v, want 0", v)
	}

	key = EdgeKey{Name: "svc-go", Host: "pg-primary", Port: "5432"}
	if v, ok := health[key]; !ok || v != 1 {
		t.Errorf("health[svc-go/pg-primary:5432] = %v, want 1", v)
	}
}

func TestQueryLatency(t *testing.T) {
	srv := newTestPromServer(latencyResponse)
	defer srv.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: srv.URL})
	latency, err := client.QueryAvgLatency(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("QueryAvgLatency() error: %v", err)
	}

	if len(latency) != 2 {
		t.Fatalf("got %d entries, want 2", len(latency))
	}

	key := EdgeKey{Name: "svc-go", Host: "pg-primary", Port: "5432"}
	if v := latency[key]; v != 0.0052 {
		t.Errorf("latency[svc-go/pg-primary:5432] = %v, want 0.0052", v)
	}
}

func TestQueryBasicAuth(t *testing.T) {
	var gotUser, gotPass string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, _ = r.BasicAuth()
		w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer srv.Close()

	client := NewPrometheusClient(PrometheusConfig{
		URL:      srv.URL,
		Username: "testuser",
		Password: "testpass",
	})
	_, _ = client.QueryTopologyEdges(context.Background(), QueryOptions{})

	if gotUser != "testuser" || gotPass != "testpass" {
		t.Errorf("Basic auth = %q:%q, want testuser:testpass", gotUser, gotPass)
	}
}

func TestQueryWithNamespaceFilter(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer srv.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: srv.URL})

	// Without namespace — no filter injected.
	_, _ = client.QueryTopologyEdges(context.Background(), QueryOptions{})
	if capturedQuery != `group by (name, namespace, dependency, type, host, port, critical) (app_dependency_health)` {
		t.Errorf("unfiltered query = %q", capturedQuery)
	}

	// With namespace — filter injected.
	_, _ = client.QueryTopologyEdges(context.Background(), QueryOptions{Namespace: "prod"})
	want := `group by (name, namespace, dependency, type, host, port, critical) (app_dependency_health{namespace="prod"})`
	if capturedQuery != want {
		t.Errorf("filtered query = %q, want %q", capturedQuery, want)
	}
}

func TestNsFilter(t *testing.T) {
	if got := nsFilter(""); got != "" {
		t.Errorf("nsFilter(\"\") = %q, want \"\"", got)
	}
	if got := nsFilter("prod"); got != `{namespace="prod"}` {
		t.Errorf("nsFilter(\"prod\") = %q, want {namespace=\"prod\"}", got)
	}
}

func TestQueryTopologyEdgesLookback(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(topologyEdgesResponse))
	}))
	defer srv.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: srv.URL})

	// Without namespace.
	edges, err := client.QueryTopologyEdgesLookback(context.Background(), QueryOptions{}, time.Hour)
	if err != nil {
		t.Fatalf("QueryTopologyEdgesLookback() error: %v", err)
	}
	if len(edges) != 3 {
		t.Fatalf("got %d edges, want 3", len(edges))
	}
	want := `group by (name, namespace, dependency, type, host, port, critical) (last_over_time(app_dependency_health[1h]))`
	if capturedQuery != want {
		t.Errorf("query = %q, want %q", capturedQuery, want)
	}

	// With namespace.
	_, _ = client.QueryTopologyEdgesLookback(context.Background(), QueryOptions{Namespace: "prod"}, 30*time.Minute)
	want = `group by (name, namespace, dependency, type, host, port, critical) (last_over_time(app_dependency_health{namespace="prod"}[30m]))`
	if capturedQuery != want {
		t.Errorf("filtered query = %q, want %q", capturedQuery, want)
	}
}

func TestFormatPromDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{time.Hour, "1h"},
		{2 * time.Hour, "2h"},
		{30 * time.Minute, "30m"},
		{5 * time.Minute, "5m"},
		{90 * time.Second, "90s"},
		{time.Hour + 30*time.Minute, "90m"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatPromDuration(tt.d)
			if got != tt.want {
				t.Errorf("formatPromDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestQueryErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("service unavailable"))
	}))
	defer srv.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: srv.URL})
	_, err := client.QueryTopologyEdges(context.Background(), QueryOptions{})
	if err == nil {
		t.Fatal("expected error for 503 response")
	}
}
