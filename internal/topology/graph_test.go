package topology

import (
	"context"
	"testing"
	"time"
)

// mockPrometheusClient implements PrometheusClient for testing.
type mockPrometheusClient struct {
	edges   []TopologyEdge
	health  map[EdgeKey]float64
	avg     map[EdgeKey]float64
	p99     map[EdgeKey]float64
	err     error
}

func (m *mockPrometheusClient) QueryTopologyEdges(_ context.Context) ([]TopologyEdge, error) {
	return m.edges, m.err
}

func (m *mockPrometheusClient) QueryHealthState(_ context.Context) (map[EdgeKey]float64, error) {
	return m.health, m.err
}

func (m *mockPrometheusClient) QueryAvgLatency(_ context.Context) (map[EdgeKey]float64, error) {
	return m.avg, m.err
}

func (m *mockPrometheusClient) QueryP99Latency(_ context.Context) (map[EdgeKey]float64, error) {
	return m.p99, m.err
}

func TestGraphBuilder_Build(t *testing.T) {
	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Job: "svc-go", Dependency: "postgres", Type: "postgres", Host: "pg-primary", Port: "5432"},
			{Job: "svc-go", Dependency: "redis", Type: "redis", Host: "redis", Port: "6379"},
			{Job: "svc-python", Dependency: "postgres", Type: "postgres", Host: "pg-primary", Port: "5432"},
		},
		health: map[EdgeKey]float64{
			{Job: "svc-go", Dependency: "postgres"}:    1,
			{Job: "svc-go", Dependency: "redis"}:        0,
			{Job: "svc-python", Dependency: "postgres"}: 1,
		},
		avg: map[EdgeKey]float64{
			{Job: "svc-go", Dependency: "postgres"}:    0.0052,
			{Job: "svc-go", Dependency: "redis"}:        0.001,
			{Job: "svc-python", Dependency: "postgres"}: 0.003,
		},
	}

	grafana := GrafanaConfig{
		BaseURL:              "https://grafana.example.com",
		ServiceStatusDashUID: "svc-dash",
		LinkStatusDashUID:    "link-dash",
	}

	builder := NewGraphBuilder(mock, grafana, 15*time.Second)
	resp, err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Check node count: svc-go, svc-python, postgres, redis = 4
	if len(resp.Nodes) != 4 {
		t.Errorf("got %d nodes, want 4", len(resp.Nodes))
	}

	// Check edge count.
	if len(resp.Edges) != 3 {
		t.Errorf("got %d edges, want 3", len(resp.Edges))
	}

	// Check meta.
	if resp.Meta.TTL != 15 {
		t.Errorf("Meta.TTL = %d, want 15", resp.Meta.TTL)
	}
	if resp.Meta.NodeCount != 4 {
		t.Errorf("Meta.NodeCount = %d, want 4", resp.Meta.NodeCount)
	}

	// Find specific nodes by ID.
	nodeByID := make(map[string]Node)
	for _, n := range resp.Nodes {
		nodeByID[n.ID] = n
	}

	// svc-go has 1 healthy + 1 down → degraded.
	if n, ok := nodeByID["svc-go"]; !ok {
		t.Error("missing svc-go node")
	} else {
		if n.State != "degraded" {
			t.Errorf("svc-go.State = %q, want %q", n.State, "degraded")
		}
		if n.Type != "service" {
			t.Errorf("svc-go.Type = %q, want %q", n.Type, "service")
		}
		if n.DependencyCount != 2 {
			t.Errorf("svc-go.DependencyCount = %d, want 2", n.DependencyCount)
		}
		if n.GrafanaURL == "" {
			t.Error("svc-go.GrafanaURL is empty")
		}
	}

	// svc-python has 1 healthy → ok.
	if n, ok := nodeByID["svc-python"]; !ok {
		t.Error("missing svc-python node")
	} else if n.State != "ok" {
		t.Errorf("svc-python.State = %q, want %q", n.State, "ok")
	}

	// postgres is a dependency node with no outgoing edges → unknown.
	if n, ok := nodeByID["postgres"]; !ok {
		t.Error("missing postgres node")
	} else {
		if n.State != "unknown" {
			t.Errorf("postgres.State = %q, want %q", n.State, "unknown")
		}
		if n.Type != "postgres" {
			t.Errorf("postgres.Type = %q, want %q", n.Type, "postgres")
		}
	}
}

func TestCalcNodeState(t *testing.T) {
	tests := []struct {
		name   string
		health []float64
		want   string
	}{
		{"no edges", nil, "unknown"},
		{"all healthy", []float64{1, 1, 1}, "ok"},
		{"all down", []float64{0, 0}, "down"},
		{"mixed", []float64{1, 0, 1}, "degraded"},
		{"single healthy", []float64{1}, "ok"},
		{"single down", []float64{0}, "down"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcNodeState(tt.health)
			if got != tt.want {
				t.Errorf("calcNodeState(%v) = %q, want %q", tt.health, got, tt.want)
			}
		})
	}
}

func TestFormatLatency(t *testing.T) {
	tests := []struct {
		seconds float64
		want    string
	}{
		{0, "0ms"},
		{0.0001, "100µs"},
		{0.0052, "5.2ms"},
		{0.1, "100.0ms"},
		{1.5, "1.50s"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatLatency(tt.seconds)
			if got != tt.want {
				t.Errorf("formatLatency(%v) = %q, want %q", tt.seconds, got, tt.want)
			}
		})
	}
}

func TestGrafanaURLGeneration(t *testing.T) {
	builder := NewGraphBuilder(nil, GrafanaConfig{
		BaseURL:              "https://grafana.example.com",
		ServiceStatusDashUID: "svc-dash",
		LinkStatusDashUID:    "link-dash",
	}, 15*time.Second)

	svcURL := builder.serviceGrafanaURL("order-service")
	if svcURL != "https://grafana.example.com/d/svc-dash?var-job=order-service" {
		t.Errorf("serviceGrafanaURL = %q", svcURL)
	}

	linkURL := builder.linkGrafanaURL("order-service", "postgres-main")
	if linkURL != "https://grafana.example.com/d/link-dash?var-job=order-service&var-dep=postgres-main" {
		t.Errorf("linkGrafanaURL = %q", linkURL)
	}

	// Empty base URL → empty URLs.
	emptyBuilder := NewGraphBuilder(nil, GrafanaConfig{}, 15*time.Second)
	if emptyBuilder.serviceGrafanaURL("svc") != "" {
		t.Error("expected empty URL when BaseURL is empty")
	}
}
