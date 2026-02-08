package topology

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BigKAA/dephealth-ui/internal/alerts"
)

// mockPrometheusClient implements PrometheusClient for testing.
type mockPrometheusClient struct {
	edges     []TopologyEdge
	health    map[EdgeKey]float64
	avg       map[EdgeKey]float64
	p99       map[EdgeKey]float64
	err       error // default error for all methods
	edgesErr  error // override for QueryTopologyEdges
	healthErr error // override for QueryHealthState
	avgErr    error // override for QueryAvgLatency
}

func (m *mockPrometheusClient) QueryTopologyEdges(_ context.Context) ([]TopologyEdge, error) {
	if m.edgesErr != nil {
		return nil, m.edgesErr
	}
	return m.edges, m.err
}

func (m *mockPrometheusClient) QueryHealthState(_ context.Context) (map[EdgeKey]float64, error) {
	if m.healthErr != nil {
		return nil, m.healthErr
	}
	return m.health, m.err
}

func (m *mockPrometheusClient) QueryAvgLatency(_ context.Context) (map[EdgeKey]float64, error) {
	if m.avgErr != nil {
		return nil, m.avgErr
	}
	return m.avg, m.err
}

func (m *mockPrometheusClient) QueryP99Latency(_ context.Context) (map[EdgeKey]float64, error) {
	return m.p99, m.err
}

func TestGraphBuilder_Build(t *testing.T) {
	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Job: "svc-go", Namespace: "default", Dependency: "postgres", Type: "postgres", Host: "pg-primary", Port: "5432"},
			{Job: "svc-go", Namespace: "default", Dependency: "redis", Type: "redis", Host: "redis", Port: "6379"},
			{Job: "svc-python", Namespace: "default", Dependency: "postgres", Type: "postgres", Host: "pg-primary", Port: "5432"},
		},
		health: map[EdgeKey]float64{
			{Job: "svc-go", Host: "pg-primary", Port: "5432"}:    1,
			{Job: "svc-go", Host: "redis", Port: "6379"}:         0,
			{Job: "svc-python", Host: "pg-primary", Port: "5432"}: 1,
		},
		avg: map[EdgeKey]float64{
			{Job: "svc-go", Host: "pg-primary", Port: "5432"}:    0.0052,
			{Job: "svc-go", Host: "redis", Port: "6379"}:         0.001,
			{Job: "svc-python", Host: "pg-primary", Port: "5432"}: 0.003,
		},
	}

	grafana := GrafanaConfig{
		BaseURL:              "https://grafana.example.com",
		ServiceStatusDashUID: "svc-dash",
		LinkStatusDashUID:    "link-dash",
	}

	builder := NewGraphBuilder(mock, nil, grafana, 15*time.Second, nil)
	resp, err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Nodes: svc-go, svc-python, pg-primary:5432, redis:6379 = 4
	// (postgres deduped into single pg-primary:5432 node)
	if len(resp.Nodes) != 4 {
		t.Errorf("got %d nodes, want 4", len(resp.Nodes))
	}

	// Edges: svc-go→pg-primary:5432, svc-go→redis:6379, svc-python→pg-primary:5432 = 3
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
		if n.Namespace != "default" {
			t.Errorf("svc-go.Namespace = %q, want %q", n.Namespace, "default")
		}
	}

	// svc-python has 1 healthy → ok.
	if n, ok := nodeByID["svc-python"]; !ok {
		t.Error("missing svc-python node")
	} else if n.State != "ok" {
		t.Errorf("svc-python.State = %q, want %q", n.State, "ok")
	}

	// pg-primary:5432 — dependency node with 2 incoming healthy edges → ok.
	if n, ok := nodeByID["pg-primary:5432"]; !ok {
		t.Error("missing pg-primary:5432 node")
	} else {
		if n.State != "ok" {
			t.Errorf("pg-primary:5432.State = %q, want %q", n.State, "ok")
		}
		if n.Type != "postgres" {
			t.Errorf("pg-primary:5432.Type = %q, want %q", n.Type, "postgres")
		}
		if n.Label != "pg-primary" {
			t.Errorf("pg-primary:5432.Label = %q, want %q", n.Label, "pg-primary")
		}
		if n.Host != "pg-primary" {
			t.Errorf("pg-primary:5432.Host = %q, want %q", n.Host, "pg-primary")
		}
		if n.Port != "5432" {
			t.Errorf("pg-primary:5432.Port = %q, want %q", n.Port, "5432")
		}
	}

	// redis:6379 — dependency node with 1 incoming down edge → down.
	if n, ok := nodeByID["redis:6379"]; !ok {
		t.Error("missing redis:6379 node")
	} else {
		if n.State != "down" {
			t.Errorf("redis:6379.State = %q, want %q", n.State, "down")
		}
	}

	// Check edge types are populated.
	for _, e := range resp.Edges {
		if e.Type == "" {
			t.Errorf("edge %s→%s has empty Type", e.Source, e.Target)
		}
	}
}

func TestGraphBuilder_Dedup(t *testing.T) {
	// Two services use different dependency names for the same host:port.
	// This should produce a single dependency node.
	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Job: "svc-go", Dependency: "my-redis", Type: "redis", Host: "redis-host", Port: "6379"},
			{Job: "svc-python", Dependency: "redis-cache", Type: "redis", Host: "redis-host", Port: "6379"},
		},
		health: map[EdgeKey]float64{
			{Job: "svc-go", Host: "redis-host", Port: "6379"}:    1,
			{Job: "svc-python", Host: "redis-host", Port: "6379"}: 0,
		},
		avg: map[EdgeKey]float64{},
	}

	builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, nil)
	resp, err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Nodes: svc-go, svc-python, redis-host:6379 = 3 (deduped!)
	if len(resp.Nodes) != 3 {
		t.Errorf("got %d nodes, want 3 (dedup should merge two dependency names to one node)", len(resp.Nodes))
	}

	// Edges: 2 (each service has its own edge to the same host:port)
	if len(resp.Edges) != 2 {
		t.Errorf("got %d edges, want 2", len(resp.Edges))
	}

	// Find the dependency node.
	nodeByID := make(map[string]Node)
	for _, n := range resp.Nodes {
		nodeByID[n.ID] = n
	}

	depNode, ok := nodeByID["redis-host:6379"]
	if !ok {
		t.Fatal("missing redis-host:6379 node")
	}

	// 1 healthy + 1 down incoming → degraded.
	if depNode.State != "degraded" {
		t.Errorf("redis-host:6379.State = %q, want %q", depNode.State, "degraded")
	}
	if depNode.Label != "redis-host" {
		t.Errorf("redis-host:6379.Label = %q, want %q", depNode.Label, "redis-host")
	}
}

func TestDependencyNodeColoring(t *testing.T) {
	tests := []struct {
		name      string
		edges     []TopologyEdge
		health    map[EdgeKey]float64
		depNodeID string
		wantState string
	}{
		{
			name: "all incoming healthy",
			edges: []TopologyEdge{
				{Job: "svc-a", Dependency: "pg", Type: "postgres", Host: "pg", Port: "5432"},
				{Job: "svc-b", Dependency: "pg", Type: "postgres", Host: "pg", Port: "5432"},
			},
			health: map[EdgeKey]float64{
				{Job: "svc-a", Host: "pg", Port: "5432"}: 1,
				{Job: "svc-b", Host: "pg", Port: "5432"}: 1,
			},
			depNodeID: "pg:5432",
			wantState: "ok",
		},
		{
			name: "all incoming down",
			edges: []TopologyEdge{
				{Job: "svc-a", Dependency: "pg", Type: "postgres", Host: "pg", Port: "5432"},
				{Job: "svc-b", Dependency: "pg", Type: "postgres", Host: "pg", Port: "5432"},
			},
			health: map[EdgeKey]float64{
				{Job: "svc-a", Host: "pg", Port: "5432"}: 0,
				{Job: "svc-b", Host: "pg", Port: "5432"}: 0,
			},
			depNodeID: "pg:5432",
			wantState: "down",
		},
		{
			name: "mixed incoming → degraded",
			edges: []TopologyEdge{
				{Job: "svc-a", Dependency: "pg", Type: "postgres", Host: "pg", Port: "5432"},
				{Job: "svc-b", Dependency: "pg", Type: "postgres", Host: "pg", Port: "5432"},
			},
			health: map[EdgeKey]float64{
				{Job: "svc-a", Host: "pg", Port: "5432"}: 1,
				{Job: "svc-b", Host: "pg", Port: "5432"}: 0,
			},
			depNodeID: "pg:5432",
			wantState: "degraded",
		},
		{
			name: "single service → state from that edge",
			edges: []TopologyEdge{
				{Job: "svc-a", Dependency: "redis", Type: "redis", Host: "redis", Port: "6379"},
			},
			health: map[EdgeKey]float64{
				{Job: "svc-a", Host: "redis", Port: "6379"}: 0,
			},
			depNodeID: "redis:6379",
			wantState: "down",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockPrometheusClient{
				edges:  tt.edges,
				health: tt.health,
				avg:    map[EdgeKey]float64{},
			}

			builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, nil)
			resp, err := builder.Build(context.Background())
			if err != nil {
				t.Fatalf("Build() error: %v", err)
			}

			nodeByID := make(map[string]Node)
			for _, n := range resp.Nodes {
				nodeByID[n.ID] = n
			}

			depNode, ok := nodeByID[tt.depNodeID]
			if !ok {
				t.Fatalf("missing node %s", tt.depNodeID)
			}
			if depNode.State != tt.wantState {
				t.Errorf("%s.State = %q, want %q", tt.depNodeID, depNode.State, tt.wantState)
			}
		})
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
	builder := NewGraphBuilder(nil, nil, GrafanaConfig{
		BaseURL:              "https://grafana.example.com",
		ServiceStatusDashUID: "svc-dash",
		LinkStatusDashUID:    "link-dash",
	}, 15*time.Second, nil)

	svcURL := builder.serviceGrafanaURL("order-service")
	if svcURL != "https://grafana.example.com/d/svc-dash?var-job=order-service" {
		t.Errorf("serviceGrafanaURL = %q", svcURL)
	}

	linkURL := builder.linkGrafanaURL("order-service", "postgres-main", "pg-host", "5432")
	want := "https://grafana.example.com/d/link-dash?var-job=order-service&var-dependency=postgres-main&var-host=pg-host&var-port=5432"
	if linkURL != want {
		t.Errorf("linkGrafanaURL = %q, want %q", linkURL, want)
	}

	// Empty base URL → empty URLs.
	emptyBuilder := NewGraphBuilder(nil, nil, GrafanaConfig{}, 15*time.Second, nil)
	if emptyBuilder.serviceGrafanaURL("svc") != "" {
		t.Error("expected empty URL when BaseURL is empty")
	}
	if emptyBuilder.linkGrafanaURL("svc", "dep", "host", "port") != "" {
		t.Error("expected empty link URL when BaseURL is empty")
	}
}

// mockAlertManagerClient implements alerts.AlertManagerClient for testing.
type mockAlertManagerClient struct {
	alerts []alerts.Alert
	err    error
}

func (m *mockAlertManagerClient) FetchAlerts(_ context.Context) ([]alerts.Alert, error) {
	return m.alerts, m.err
}

func TestBuildWithAlerts(t *testing.T) {
	promMock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Job: "svc-go", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432"},
			{Job: "svc-go", Dependency: "redis", Type: "redis", Host: "redis", Port: "6379"},
		},
		health: map[EdgeKey]float64{
			{Job: "svc-go", Host: "pg", Port: "5432"}:    1,
			{Job: "svc-go", Host: "redis", Port: "6379"}: 1,
		},
		avg: map[EdgeKey]float64{
			{Job: "svc-go", Host: "pg", Port: "5432"}:    0.005,
			{Job: "svc-go", Host: "redis", Port: "6379"}: 0.001,
		},
	}

	amMock := &mockAlertManagerClient{
		alerts: []alerts.Alert{
			{
				AlertName:  "DependencyDown",
				Service:    "svc-go",
				Dependency: "postgres",
				Severity:   "critical",
				State:      "firing",
				Since:      "2026-02-08T10:00:00Z",
			},
		},
	}

	builder := NewGraphBuilder(promMock, amMock, GrafanaConfig{}, 15*time.Second, nil)
	resp, err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Should have 1 alert in response.
	if len(resp.Alerts) != 1 {
		t.Fatalf("got %d alerts, want 1", len(resp.Alerts))
	}
	if resp.Alerts[0].AlertName != "DependencyDown" {
		t.Errorf("Alerts[0].AlertName = %q, want %q", resp.Alerts[0].AlertName, "DependencyDown")
	}

	// Edge svc-go→pg:5432 should be overridden to "down".
	edgeByTarget := make(map[string]Edge)
	for _, e := range resp.Edges {
		edgeByTarget[e.Target] = e
	}

	pgEdge := edgeByTarget["pg:5432"]
	if pgEdge.State != "down" {
		t.Errorf("pg:5432 edge State = %q, want %q", pgEdge.State, "down")
	}
	if pgEdge.Health != 0 {
		t.Errorf("pg:5432 edge Health = %v, want 0", pgEdge.Health)
	}

	// svc-go node should be degraded (1 down + 1 ok).
	nodeByID := make(map[string]Node)
	for _, n := range resp.Nodes {
		nodeByID[n.ID] = n
	}
	if nodeByID["svc-go"].State != "degraded" {
		t.Errorf("svc-go State = %q, want %q", nodeByID["svc-go"].State, "degraded")
	}
}

func TestBuildWithNilAlertManager(t *testing.T) {
	promMock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Job: "svc-go", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432"},
		},
		health: map[EdgeKey]float64{
			{Job: "svc-go", Host: "pg", Port: "5432"}: 1,
		},
		avg: map[EdgeKey]float64{},
	}

	builder := NewGraphBuilder(promMock, nil, GrafanaConfig{}, 15*time.Second, nil)
	resp, err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if len(resp.Alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(resp.Alerts))
	}
}

func TestBuildPartialData(t *testing.T) {
	baseEdges := []TopologyEdge{
		{Job: "svc-go", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432"},
		{Job: "svc-go", Dependency: "redis", Type: "redis", Host: "redis", Port: "6379"},
	}

	tests := []struct {
		name        string
		healthErr   error
		avgErr      error
		edgesErr    error
		wantErr     bool
		wantPartial bool
		wantErrors  int
	}{
		{
			name:        "health state fails",
			healthErr:   errors.New("prometheus timeout"),
			wantPartial: true,
			wantErrors:  1,
		},
		{
			name:        "avg latency fails",
			avgErr:      errors.New("prometheus timeout"),
			wantPartial: true,
			wantErrors:  1,
		},
		{
			name:        "both health and latency fail",
			healthErr:   errors.New("prometheus timeout"),
			avgErr:      errors.New("query error"),
			wantPartial: true,
			wantErrors:  2,
		},
		{
			name:     "topology edges fail is fatal",
			edgesErr: errors.New("prometheus down"),
			wantErr:  true,
		},
		{
			name:        "all queries succeed",
			wantPartial: false,
			wantErrors:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockPrometheusClient{
				edges: baseEdges,
				health: map[EdgeKey]float64{
					{Job: "svc-go", Host: "pg", Port: "5432"}:    1,
					{Job: "svc-go", Host: "redis", Port: "6379"}: 1,
				},
				avg: map[EdgeKey]float64{
					{Job: "svc-go", Host: "pg", Port: "5432"}:    0.005,
					{Job: "svc-go", Host: "redis", Port: "6379"}: 0.001,
				},
				healthErr: tt.healthErr,
				avgErr:    tt.avgErr,
				edgesErr:  tt.edgesErr,
			}

			builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, nil)
			resp, err := builder.Build(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.Meta.Partial != tt.wantPartial {
				t.Errorf("Meta.Partial = %v, want %v", resp.Meta.Partial, tt.wantPartial)
			}

			if len(resp.Meta.Errors) != tt.wantErrors {
				t.Errorf("len(Meta.Errors) = %d, want %d", len(resp.Meta.Errors), tt.wantErrors)
			}

			// Even with partial data, nodes and edges should be present.
			if len(resp.Nodes) == 0 {
				t.Error("expected nodes even with partial data")
			}
			if len(resp.Edges) == 0 {
				t.Error("expected edges even with partial data")
			}
		})
	}
}

func TestBuildPartialWithAlertFailure(t *testing.T) {
	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Job: "svc-go", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432"},
		},
		health: map[EdgeKey]float64{
			{Job: "svc-go", Host: "pg", Port: "5432"}: 1,
		},
		avg: map[EdgeKey]float64{},
	}

	amMock := &mockAlertManagerClient{
		err: errors.New("alertmanager unreachable"),
	}

	builder := NewGraphBuilder(mock, amMock, GrafanaConfig{}, 15*time.Second, nil)
	resp, err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Meta.Partial {
		t.Error("expected Meta.Partial = true when alerts fail")
	}
	if len(resp.Meta.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(resp.Meta.Errors))
	}
}
