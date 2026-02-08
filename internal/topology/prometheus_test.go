package topology

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const topologyEdgesResponse = `{
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [
      {
        "metric": {"job": "svc-go", "dependency": "postgres", "type": "postgres", "host": "pg-primary", "port": "5432"},
        "value": [1700000000, "1"]
      },
      {
        "metric": {"job": "svc-go", "dependency": "redis", "type": "redis", "host": "redis", "port": "6379"},
        "value": [1700000000, "1"]
      },
      {
        "metric": {"job": "svc-python", "dependency": "postgres", "type": "postgres", "host": "pg-primary", "port": "5432"},
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
        "metric": {"job": "svc-go", "dependency": "postgres"},
        "value": [1700000000, "1"]
      },
      {
        "metric": {"job": "svc-go", "dependency": "redis"},
        "value": [1700000000, "0"]
      },
      {
        "metric": {"job": "svc-python", "dependency": "postgres"},
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
        "metric": {"job": "svc-go", "dependency": "postgres"},
        "value": [1700000000, "0.0052"]
      },
      {
        "metric": {"job": "svc-go", "dependency": "redis"},
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
	edges, err := client.QueryTopologyEdges(context.Background())
	if err != nil {
		t.Fatalf("QueryTopologyEdges() error: %v", err)
	}

	if len(edges) != 3 {
		t.Fatalf("got %d edges, want 3", len(edges))
	}

	e := edges[0]
	if e.Job != "svc-go" || e.Dependency != "postgres" || e.Type != "postgres" {
		t.Errorf("edge[0] = %+v, unexpected", e)
	}
}

func TestQueryHealthState(t *testing.T) {
	srv := newTestPromServer(healthStateResponse)
	defer srv.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: srv.URL})
	health, err := client.QueryHealthState(context.Background())
	if err != nil {
		t.Fatalf("QueryHealthState() error: %v", err)
	}

	if len(health) != 3 {
		t.Fatalf("got %d entries, want 3", len(health))
	}

	key := EdgeKey{Job: "svc-go", Dependency: "redis"}
	if v, ok := health[key]; !ok || v != 0 {
		t.Errorf("health[svc-go/redis] = %v, want 0", v)
	}

	key = EdgeKey{Job: "svc-go", Dependency: "postgres"}
	if v, ok := health[key]; !ok || v != 1 {
		t.Errorf("health[svc-go/postgres] = %v, want 1", v)
	}
}

func TestQueryLatency(t *testing.T) {
	srv := newTestPromServer(latencyResponse)
	defer srv.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: srv.URL})
	latency, err := client.QueryAvgLatency(context.Background())
	if err != nil {
		t.Fatalf("QueryAvgLatency() error: %v", err)
	}

	if len(latency) != 2 {
		t.Fatalf("got %d entries, want 2", len(latency))
	}

	key := EdgeKey{Job: "svc-go", Dependency: "postgres"}
	if v := latency[key]; v != 0.0052 {
		t.Errorf("latency[svc-go/postgres] = %v, want 0.0052", v)
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
	_, _ = client.QueryTopologyEdges(context.Background())

	if gotUser != "testuser" || gotPass != "testpass" {
		t.Errorf("Basic auth = %q:%q, want testuser:testpass", gotUser, gotPass)
	}
}

func TestQueryErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("service unavailable"))
	}))
	defer srv.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: srv.URL})
	_, err := client.QueryTopologyEdges(context.Background())
	if err == nil {
		t.Fatal("expected error for 503 response")
	}
}
