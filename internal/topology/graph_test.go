package topology

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BigKAA/dephealth-ui/internal/alerts"
	"github.com/BigKAA/dephealth-ui/internal/config"
)

// testSeverityLevels returns default severity levels for tests.
func testSeverityLevels() []config.SeverityLevel {
	return []config.SeverityLevel{
		{Value: "critical", Color: "#f44336"},
		{Value: "warning", Color: "#ff9800"},
		{Value: "info", Color: "#2196f3"},
	}
}

// mockPrometheusClient implements PrometheusClient for testing.
type mockPrometheusClient struct {
	edges         []TopologyEdge
	lookbackEdges []TopologyEdge // edges returned by lookback query (nil = same as edges)
	health        map[EdgeKey]float64
	avg           map[EdgeKey]float64
	p99           map[EdgeKey]float64
	err           error // default error for all methods
	edgesErr      error // override for QueryTopologyEdges
	lookbackErr   error // override for QueryTopologyEdgesLookback
	healthErr     error // override for QueryHealthState
	avgErr        error // override for QueryAvgLatency
}

func (m *mockPrometheusClient) QueryTopologyEdges(_ context.Context, _ QueryOptions) ([]TopologyEdge, error) {
	if m.edgesErr != nil {
		return nil, m.edgesErr
	}
	return m.edges, m.err
}

func (m *mockPrometheusClient) QueryTopologyEdgesLookback(_ context.Context, _ QueryOptions, _ time.Duration) ([]TopologyEdge, error) {
	if m.lookbackErr != nil {
		return nil, m.lookbackErr
	}
	if m.lookbackEdges != nil {
		return m.lookbackEdges, m.err
	}
	return m.edges, m.err
}

func (m *mockPrometheusClient) QueryHealthState(_ context.Context, _ QueryOptions) (map[EdgeKey]float64, error) {
	if m.healthErr != nil {
		return nil, m.healthErr
	}
	return m.health, m.err
}

func (m *mockPrometheusClient) QueryAvgLatency(_ context.Context, _ QueryOptions) (map[EdgeKey]float64, error) {
	if m.avgErr != nil {
		return nil, m.avgErr
	}
	return m.avg, m.err
}

func (m *mockPrometheusClient) QueryP99Latency(_ context.Context, _ QueryOptions) (map[EdgeKey]float64, error) {
	return m.p99, m.err
}

func (m *mockPrometheusClient) QueryInstances(_ context.Context, _ string) ([]Instance, error) {
	return nil, m.err
}

func TestGraphBuilder_Build(t *testing.T) {
	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Name: "svc-go", Namespace: "default", Dependency: "postgres", Type: "postgres", Host: "pg-primary", Port: "5432", Critical: true},
			{Name: "svc-go", Namespace: "default", Dependency: "redis", Type: "redis", Host: "redis", Port: "6379", Critical: false},
			{Name: "svc-python", Namespace: "default", Dependency: "postgres", Type: "postgres", Host: "pg-primary", Port: "5432", Critical: true},
		},
		health: map[EdgeKey]float64{
			{Name: "svc-go", Host: "pg-primary", Port: "5432"}:    1,
			{Name: "svc-go", Host: "redis", Port: "6379"}:         0,
			{Name: "svc-python", Host: "pg-primary", Port: "5432"}: 1,
		},
		avg: map[EdgeKey]float64{
			{Name: "svc-go", Host: "pg-primary", Port: "5432"}:    0.0052,
			{Name: "svc-go", Host: "redis", Port: "6379"}:         0.001,
			{Name: "svc-python", Host: "pg-primary", Port: "5432"}: 0.003,
		},
	}

	grafana := GrafanaConfig{
		BaseURL:              "https://grafana.example.com",
		ServiceStatusDashUID: "svc-dash",
		LinkStatusDashUID:    "link-dash",
	}

	builder := NewGraphBuilder(mock, nil, grafana, 15*time.Second, 0, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
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

	// svc-go has 1 healthy critical + 1 down non-critical → degraded
	// (all critical up, some non-critical down).
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

	// Check edge types and critical flags are populated.
	edgeByTarget := make(map[string]Edge)
	for _, e := range resp.Edges {
		if e.Type == "" {
			t.Errorf("edge %s→%s has empty Type", e.Source, e.Target)
		}
		edgeByTarget[e.Source+"→"+e.Target] = e
	}

	// Check critical flag on edges.
	if e, ok := edgeByTarget["svc-go→pg-primary:5432"]; ok {
		if !e.Critical {
			t.Errorf("edge svc-go→pg-primary:5432 Critical = false, want true")
		}
	}
	if e, ok := edgeByTarget["svc-go→redis:6379"]; ok {
		if e.Critical {
			t.Errorf("edge svc-go→redis:6379 Critical = true, want false")
		}
	}
}

func TestGraphBuilder_Dedup(t *testing.T) {
	// Two services use different dependency names for the same host:port.
	// This should produce a single dependency node.
	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Name: "svc-go", Dependency: "my-redis", Type: "redis", Host: "redis-host", Port: "6379"},
			{Name: "svc-python", Dependency: "redis-cache", Type: "redis", Host: "redis-host", Port: "6379"},
		},
		health: map[EdgeKey]float64{
			{Name: "svc-go", Host: "redis-host", Port: "6379"}:    1,
			{Name: "svc-python", Host: "redis-host", Port: "6379"}: 0,
		},
		avg: map[EdgeKey]float64{},
	}

	builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, 0, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
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
				{Name: "svc-a", Dependency: "pg", Type: "postgres", Host: "pg", Port: "5432"},
				{Name: "svc-b", Dependency: "pg", Type: "postgres", Host: "pg", Port: "5432"},
			},
			health: map[EdgeKey]float64{
				{Name: "svc-a", Host: "pg", Port: "5432"}: 1,
				{Name: "svc-b", Host: "pg", Port: "5432"}: 1,
			},
			depNodeID: "pg:5432",
			wantState: "ok",
		},
		{
			name: "all incoming down",
			edges: []TopologyEdge{
				{Name: "svc-a", Dependency: "pg", Type: "postgres", Host: "pg", Port: "5432"},
				{Name: "svc-b", Dependency: "pg", Type: "postgres", Host: "pg", Port: "5432"},
			},
			health: map[EdgeKey]float64{
				{Name: "svc-a", Host: "pg", Port: "5432"}: 0,
				{Name: "svc-b", Host: "pg", Port: "5432"}: 0,
			},
			depNodeID: "pg:5432",
			wantState: "down",
		},
		{
			name: "mixed incoming → degraded",
			edges: []TopologyEdge{
				{Name: "svc-a", Dependency: "pg", Type: "postgres", Host: "pg", Port: "5432"},
				{Name: "svc-b", Dependency: "pg", Type: "postgres", Host: "pg", Port: "5432"},
			},
			health: map[EdgeKey]float64{
				{Name: "svc-a", Host: "pg", Port: "5432"}: 1,
				{Name: "svc-b", Host: "pg", Port: "5432"}: 0,
			},
			depNodeID: "pg:5432",
			wantState: "degraded",
		},
		{
			name: "single service → state from that edge",
			edges: []TopologyEdge{
				{Name: "svc-a", Dependency: "redis", Type: "redis", Host: "redis", Port: "6379"},
			},
			health: map[EdgeKey]float64{
				{Name: "svc-a", Host: "redis", Port: "6379"}: 0,
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

			builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, 0, nil, testSeverityLevels())
			resp, err := builder.Build(context.Background(), QueryOptions{})
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

func TestCalcServiceNodeState(t *testing.T) {
	// Service node state no longer returns "down" — that is reserved for stale
	// (metrics disappeared). Any dep with health=0 → "degraded".
	// Cascade warnings handle the critical-path impact separately.
	tests := []struct {
		name  string
		edges []edgeHealthInfo
		want  string
	}{
		{"no edges", nil, "unknown"},
		{"all critical healthy", []edgeHealthInfo{
			{Health: 1, Critical: true},
			{Health: 1, Critical: true},
		}, "ok"},
		{"one critical down → degraded", []edgeHealthInfo{
			{Health: 1, Critical: true},
			{Health: 0, Critical: true},
		}, "degraded"},
		{"all critical up, non-critical down → degraded", []edgeHealthInfo{
			{Health: 1, Critical: true},
			{Health: 0, Critical: false},
		}, "degraded"},
		{"no critical edges, all down → degraded", []edgeHealthInfo{
			{Health: 0, Critical: false},
			{Health: 0, Critical: false},
		}, "degraded"},
		{"no critical edges, all healthy → ok", []edgeHealthInfo{
			{Health: 1, Critical: false},
			{Health: 1, Critical: false},
		}, "ok"},
		{"no critical edges, mixed → degraded", []edgeHealthInfo{
			{Health: 1, Critical: false},
			{Health: 0, Critical: false},
		}, "degraded"},
		{"single critical down → degraded", []edgeHealthInfo{
			{Health: 0, Critical: true},
		}, "degraded"},
		{"single critical healthy → ok", []edgeHealthInfo{
			{Health: 1, Critical: true},
		}, "ok"},
		{"mixed: critical down + non-critical healthy → degraded", []edgeHealthInfo{
			{Health: 0, Critical: true},
			{Health: 1, Critical: false},
		}, "degraded"},
		{"mixed: critical down + non-critical down → degraded", []edgeHealthInfo{
			{Health: 0, Critical: true},
			{Health: 0, Critical: false},
		}, "degraded"},
		{"all healthy mixed types → ok", []edgeHealthInfo{
			{Health: 1, Critical: true},
			{Health: 1, Critical: false},
		}, "ok"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcServiceNodeState(tt.edges)
			if got != tt.want {
				t.Errorf("calcServiceNodeState() = %q, want %q", got, tt.want)
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
	}, 15*time.Second, 0, nil, testSeverityLevels())

	svcURL := builder.serviceGrafanaURL("order-service")
	if svcURL != "https://grafana.example.com/d/svc-dash?var-service=order-service" {
		t.Errorf("serviceGrafanaURL = %q", svcURL)
	}

	linkURL := builder.linkGrafanaURL("postgres-main", "pg-host", "5432")
	want := "https://grafana.example.com/d/link-dash?var-dependency=postgres-main&var-host=pg-host&var-port=5432"
	if linkURL != want {
		t.Errorf("linkGrafanaURL = %q, want %q", linkURL, want)
	}

	// Empty base URL → empty URLs.
	emptyBuilder := NewGraphBuilder(nil, nil, GrafanaConfig{}, 15*time.Second, 0, nil, testSeverityLevels())
	if emptyBuilder.serviceGrafanaURL("svc") != "" {
		t.Error("expected empty URL when BaseURL is empty")
	}
	if emptyBuilder.linkGrafanaURL("dep", "host", "port") != "" {
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
			{Name: "svc-go", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432", Critical: true},
			{Name: "svc-go", Dependency: "redis", Type: "redis", Host: "redis", Port: "6379", Critical: false},
		},
		health: map[EdgeKey]float64{
			{Name: "svc-go", Host: "pg", Port: "5432"}:    1,
			{Name: "svc-go", Host: "redis", Port: "6379"}: 1,
		},
		avg: map[EdgeKey]float64{
			{Name: "svc-go", Host: "pg", Port: "5432"}:    0.005,
			{Name: "svc-go", Host: "redis", Port: "6379"}: 0.001,
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

	builder := NewGraphBuilder(promMock, amMock, GrafanaConfig{}, 15*time.Second, 0, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
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
	if pgEdge.AlertCount != 1 {
		t.Errorf("pg:5432 edge AlertCount = %d, want 1", pgEdge.AlertCount)
	}
	if pgEdge.AlertSeverity != "critical" {
		t.Errorf("pg:5432 edge AlertSeverity = %q, want %q", pgEdge.AlertSeverity, "critical")
	}

	// redis edge should have no alerts.
	redisEdge := edgeByTarget["redis:6379"]
	if redisEdge.AlertCount != 0 {
		t.Errorf("redis:6379 edge AlertCount = %d, want 0", redisEdge.AlertCount)
	}

	// svc-go: alert forces critical pg edge down → service is "degraded"
	// (service nodes never "down" from health calc; "down" is reserved for stale).
	nodeByID := make(map[string]Node)
	for _, n := range resp.Nodes {
		nodeByID[n.ID] = n
	}
	if nodeByID["svc-go"].State != "degraded" {
		t.Errorf("svc-go State = %q, want %q", nodeByID["svc-go"].State, "degraded")
	}
	if nodeByID["svc-go"].AlertCount != 1 {
		t.Errorf("svc-go AlertCount = %d, want 1", nodeByID["svc-go"].AlertCount)
	}
	if nodeByID["svc-go"].AlertSeverity != "critical" {
		t.Errorf("svc-go AlertSeverity = %q, want %q", nodeByID["svc-go"].AlertSeverity, "critical")
	}
}

func TestBuildWithNilAlertManager(t *testing.T) {
	promMock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Name: "svc-go", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432", Critical: true},
		},
		health: map[EdgeKey]float64{
			{Name: "svc-go", Host: "pg", Port: "5432"}: 1,
		},
		avg: map[EdgeKey]float64{},
	}

	builder := NewGraphBuilder(promMock, nil, GrafanaConfig{}, 15*time.Second, 0, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if len(resp.Alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(resp.Alerts))
	}
}

func TestGraphBuilder_ConnectedGraph(t *testing.T) {
	// Service-to-service edges should produce a connected (through) graph.
	// uniproxy-01 → uniproxy-02 → redis
	// The edge from uniproxy-01 to uniproxy-02 should target the service node "uniproxy-02",
	// not a separate dependency node "uniproxy-02.ns.svc:8080".
	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Name: "uniproxy-01", Namespace: "ns1", Dependency: "uniproxy-02", Type: "http", Host: "uniproxy-02.ns1.svc", Port: "8080", Critical: true},
			{Name: "uniproxy-02", Namespace: "ns1", Dependency: "redis", Type: "redis", Host: "redis.ns2.svc", Port: "6379", Critical: false},
		},
		health: map[EdgeKey]float64{
			{Name: "uniproxy-01", Host: "uniproxy-02.ns1.svc", Port: "8080"}: 1,
			{Name: "uniproxy-02", Host: "redis.ns2.svc", Port: "6379"}:       1,
		},
		avg: map[EdgeKey]float64{},
	}

	builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, 0, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Nodes: uniproxy-01, uniproxy-02, redis.ns2.svc:6379 = 3 (NOT 4!)
	if len(resp.Nodes) != 3 {
		t.Errorf("got %d nodes, want 3 (uniproxy-02 should be a single service node)", len(resp.Nodes))
		for _, n := range resp.Nodes {
			t.Logf("  node: id=%q type=%q", n.ID, n.Type)
		}
	}

	nodeByID := make(map[string]Node)
	for _, n := range resp.Nodes {
		nodeByID[n.ID] = n
	}

	// uniproxy-02 must be a service node (not a dependency node).
	if n, ok := nodeByID["uniproxy-02"]; !ok {
		t.Error("missing uniproxy-02 service node")
	} else if n.Type != "service" {
		t.Errorf("uniproxy-02.Type = %q, want %q", n.Type, "service")
	}

	// No separate host:port node for uniproxy-02.
	if _, ok := nodeByID["uniproxy-02.ns1.svc:8080"]; ok {
		t.Error("unexpected separate dependency node uniproxy-02.ns1.svc:8080; should be merged into service node")
	}

	// Edge from uniproxy-01 should target the service node "uniproxy-02".
	edgeByKey := make(map[string]Edge)
	for _, e := range resp.Edges {
		edgeByKey[e.Source+"→"+e.Target] = e
	}

	if _, ok := edgeByKey["uniproxy-01→uniproxy-02"]; !ok {
		t.Error("missing edge uniproxy-01→uniproxy-02")
		for _, e := range resp.Edges {
			t.Logf("  edge: %s→%s", e.Source, e.Target)
		}
	}

	// Edge from uniproxy-02 to redis should use host:port (redis is not a service).
	if _, ok := edgeByKey["uniproxy-02→redis.ns2.svc:6379"]; !ok {
		t.Error("missing edge uniproxy-02→redis.ns2.svc:6379")
	}
}

func TestGraphBuilder_ConnectedGraphWithAlerts(t *testing.T) {
	// Alert on a service-to-service edge should correctly find the edge.
	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Name: "svc-a", Namespace: "ns", Dependency: "svc-b", Type: "http", Host: "svc-b.ns.svc", Port: "8080", Critical: true},
			{Name: "svc-b", Namespace: "ns", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432", Critical: true},
		},
		health: map[EdgeKey]float64{
			{Name: "svc-a", Host: "svc-b.ns.svc", Port: "8080"}: 1,
			{Name: "svc-b", Host: "pg", Port: "5432"}:           1,
		},
		avg: map[EdgeKey]float64{},
	}

	amMock := &mockAlertManagerClient{
		alerts: []alerts.Alert{
			{
				AlertName:  "DependencyDown",
				Service:    "svc-a",
				Dependency: "svc-b",
				Severity:   "critical",
				State:      "firing",
				Since:      "2026-02-09T10:00:00Z",
			},
		},
	}

	builder := NewGraphBuilder(mock, amMock, GrafanaConfig{}, 15*time.Second, 0, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Edge svc-a→svc-b should be overridden to "down" by the alert.
	edgeByKey := make(map[string]Edge)
	for _, e := range resp.Edges {
		edgeByKey[e.Source+"→"+e.Target] = e
	}

	edge, ok := edgeByKey["svc-a→svc-b"]
	if !ok {
		t.Fatal("missing edge svc-a→svc-b")
	}
	if edge.State != "down" {
		t.Errorf("svc-a→svc-b State = %q, want %q", edge.State, "down")
	}

	// svc-a: alert forced critical edge down → "degraded" (not "down"; down is stale-only).
	nodeByID := make(map[string]Node)
	for _, n := range resp.Nodes {
		nodeByID[n.ID] = n
	}
	if nodeByID["svc-a"].State != "degraded" {
		t.Errorf("svc-a State = %q, want %q", nodeByID["svc-a"].State, "degraded")
	}
}

func TestBuildPartialData(t *testing.T) {
	baseEdges := []TopologyEdge{
		{Name: "svc-go", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432", Critical: true},
		{Name: "svc-go", Dependency: "redis", Type: "redis", Host: "redis", Port: "6379", Critical: false},
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
					{Name: "svc-go", Host: "pg", Port: "5432"}:    1,
					{Name: "svc-go", Host: "redis", Port: "6379"}: 1,
				},
				avg: map[EdgeKey]float64{
					{Name: "svc-go", Host: "pg", Port: "5432"}:    0.005,
					{Name: "svc-go", Host: "redis", Port: "6379"}: 0.001,
				},
				healthErr: tt.healthErr,
				avgErr:    tt.avgErr,
				edgesErr:  tt.edgesErr,
			}

			builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, 0, nil, testSeverityLevels())
			resp, err := builder.Build(context.Background(), QueryOptions{})

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
			{Name: "svc-go", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432", Critical: true},
		},
		health: map[EdgeKey]float64{
			{Name: "svc-go", Host: "pg", Port: "5432"}: 1,
		},
		avg: map[EdgeKey]float64{},
	}

	amMock := &mockAlertManagerClient{
		err: errors.New("alertmanager unreachable"),
	}

	builder := NewGraphBuilder(mock, amMock, GrafanaConfig{}, 15*time.Second, 0, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
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

func TestAlertSeverityPriority(t *testing.T) {
	// Two alerts on the same service: warning + critical → worst should be critical.
	promMock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Name: "svc-go", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432", Critical: true},
			{Name: "svc-go", Dependency: "redis", Type: "redis", Host: "redis", Port: "6379", Critical: false},
		},
		health: map[EdgeKey]float64{
			{Name: "svc-go", Host: "pg", Port: "5432"}:    1,
			{Name: "svc-go", Host: "redis", Port: "6379"}: 1,
		},
		avg: map[EdgeKey]float64{},
	}

	amMock := &mockAlertManagerClient{
		alerts: []alerts.Alert{
			{
				AlertName:  "DependencyDegraded",
				Service:    "svc-go",
				Dependency: "redis",
				Severity:   "warning",
				State:      "firing",
				Since:      "2026-02-08T10:00:00Z",
			},
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

	builder := NewGraphBuilder(promMock, amMock, GrafanaConfig{}, 15*time.Second, 0, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	nodeByID := make(map[string]Node)
	for _, n := range resp.Nodes {
		nodeByID[n.ID] = n
	}

	// svc-go has 2 alerts, worst is critical.
	svcGo := nodeByID["svc-go"]
	if svcGo.AlertCount != 2 {
		t.Errorf("svc-go AlertCount = %d, want 2", svcGo.AlertCount)
	}
	if svcGo.AlertSeverity != "critical" {
		t.Errorf("svc-go AlertSeverity = %q, want %q", svcGo.AlertSeverity, "critical")
	}

	// Edge to pg should be critical, edge to redis should be warning.
	edgeByKey := make(map[string]Edge)
	for _, e := range resp.Edges {
		edgeByKey[e.Source+"→"+e.Target] = e
	}

	pgEdge := edgeByKey["svc-go→pg:5432"]
	if pgEdge.AlertSeverity != "critical" {
		t.Errorf("pg edge AlertSeverity = %q, want %q", pgEdge.AlertSeverity, "critical")
	}
	if pgEdge.AlertCount != 1 {
		t.Errorf("pg edge AlertCount = %d, want 1", pgEdge.AlertCount)
	}

	redisEdge := edgeByKey["svc-go→redis:6379"]
	if redisEdge.AlertSeverity != "warning" {
		t.Errorf("redis edge AlertSeverity = %q, want %q", redisEdge.AlertSeverity, "warning")
	}
	if redisEdge.AlertCount != 1 {
		t.Errorf("redis edge AlertCount = %d, want 1", redisEdge.AlertCount)
	}
}

// --- Stale detection tests (lookback mode) ---

func TestStaleDetection_AllCurrent(t *testing.T) {
	// With lookback enabled but all edges present in health → no stale nodes.
	mock := &mockPrometheusClient{
		lookbackEdges: []TopologyEdge{
			{Name: "svc-go", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432"},
			{Name: "svc-go", Dependency: "redis", Type: "redis", Host: "redis", Port: "6379"},
		},
		health: map[EdgeKey]float64{
			{Name: "svc-go", Host: "pg", Port: "5432"}:    1,
			{Name: "svc-go", Host: "redis", Port: "6379"}: 1,
		},
		avg: map[EdgeKey]float64{},
	}

	builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, time.Hour, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	for _, n := range resp.Nodes {
		if n.Stale {
			t.Errorf("node %q is stale, want not stale", n.ID)
		}
		if n.State == "unknown" {
			t.Errorf("node %q state = unknown, want ok", n.ID)
		}
	}
	for _, e := range resp.Edges {
		if e.Stale {
			t.Errorf("edge %s→%s is stale, want not stale", e.Source, e.Target)
		}
	}
}

func TestStaleDetection_ServiceDisappears(t *testing.T) {
	// svc-go is in lookback but NOT in current health → stale.
	// svc-python is current.
	mock := &mockPrometheusClient{
		lookbackEdges: []TopologyEdge{
			{Name: "svc-go", Namespace: "ns1", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432"},
			{Name: "svc-python", Namespace: "ns1", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432"},
		},
		health: map[EdgeKey]float64{
			// Only svc-python is current.
			{Name: "svc-python", Host: "pg", Port: "5432"}: 1,
		},
		avg: map[EdgeKey]float64{},
	}

	builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, time.Hour, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	nodeByID := make(map[string]Node)
	for _, n := range resp.Nodes {
		nodeByID[n.ID] = n
	}

	// svc-go: all edges stale → node is stale + down (truly unavailable).
	svcGo := nodeByID["svc-go"]
	if !svcGo.Stale {
		t.Errorf("svc-go.Stale = false, want true")
	}
	if svcGo.State != "down" {
		t.Errorf("svc-go.State = %q, want %q", svcGo.State, "down")
	}

	// svc-python: current → not stale.
	svcPy := nodeByID["svc-python"]
	if svcPy.Stale {
		t.Errorf("svc-python.Stale = true, want false")
	}
	if svcPy.State != "ok" {
		t.Errorf("svc-python.State = %q, want %q", svcPy.State, "ok")
	}

	// pg:5432 has one stale incoming + one current incoming → not fully stale.
	pgNode := nodeByID["pg:5432"]
	if pgNode.Stale {
		t.Errorf("pg:5432.Stale = true, want false (has current incoming edge)")
	}

	// Check stale edge properties.
	for _, e := range resp.Edges {
		if e.Source == "svc-go" {
			if !e.Stale {
				t.Errorf("edge svc-go→%s Stale = false, want true", e.Target)
			}
			if e.State != "unknown" {
				t.Errorf("edge svc-go→%s State = %q, want unknown", e.Target, e.State)
			}
			if e.Health != -1 {
				t.Errorf("edge svc-go→%s Health = %v, want -1", e.Target, e.Health)
			}
			if e.Latency != "" {
				t.Errorf("edge svc-go→%s Latency = %q, want empty", e.Target, e.Latency)
			}
		}
	}
}

func TestStaleDetection_PartialStale(t *testing.T) {
	// svc-go has 2 edges: postgres (current) and redis (stale).
	// Node state should be computed from non-stale edges only.
	mock := &mockPrometheusClient{
		lookbackEdges: []TopologyEdge{
			{Name: "svc-go", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432"},
			{Name: "svc-go", Dependency: "redis", Type: "redis", Host: "redis", Port: "6379"},
		},
		health: map[EdgeKey]float64{
			// Only postgres is current.
			{Name: "svc-go", Host: "pg", Port: "5432"}: 1,
		},
		avg: map[EdgeKey]float64{},
	}

	builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, time.Hour, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	nodeByID := make(map[string]Node)
	for _, n := range resp.Nodes {
		nodeByID[n.ID] = n
	}

	// svc-go has 1 current + 1 stale → NOT fully stale.
	svcGo := nodeByID["svc-go"]
	if svcGo.Stale {
		t.Errorf("svc-go.Stale = true, want false (has current edges)")
	}
	// State computed from non-stale edges only: 1 healthy → ok.
	if svcGo.State != "ok" {
		t.Errorf("svc-go.State = %q, want %q", svcGo.State, "ok")
	}

	// redis:6379 — only stale incoming → fully stale.
	redis := nodeByID["redis:6379"]
	if !redis.Stale {
		t.Errorf("redis:6379.Stale = false, want true")
	}
	if redis.State != "down" {
		t.Errorf("redis:6379.State = %q, want %q", redis.State, "down")
	}
}

func TestStaleDetection_ConnectedGraph(t *testing.T) {
	// Service-to-service edge: svc-a → svc-b → pg.
	// svc-b disappears. svc-a is still alive.
	mock := &mockPrometheusClient{
		lookbackEdges: []TopologyEdge{
			{Name: "svc-a", Namespace: "ns", Dependency: "svc-b", Type: "http", Host: "svc-b.ns.svc", Port: "8080"},
			{Name: "svc-b", Namespace: "ns", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432"},
		},
		health: map[EdgeKey]float64{
			// svc-a's edge is still live.
			{Name: "svc-a", Host: "svc-b.ns.svc", Port: "8080"}: 1,
			// svc-b's edge is gone (not in health).
		},
		avg: map[EdgeKey]float64{},
	}

	builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, time.Hour, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	nodeByID := make(map[string]Node)
	for _, n := range resp.Nodes {
		nodeByID[n.ID] = n
	}

	// svc-a: has 1 current edge → ok, not stale.
	if nodeByID["svc-a"].Stale {
		t.Errorf("svc-a.Stale = true, want false")
	}
	if nodeByID["svc-a"].State != "ok" {
		t.Errorf("svc-a.State = %q, want ok", nodeByID["svc-a"].State)
	}

	// svc-b: service node, all outgoing edges stale → down + stale.
	if !nodeByID["svc-b"].Stale {
		t.Errorf("svc-b.Stale = false, want true")
	}
	if nodeByID["svc-b"].State != "down" {
		t.Errorf("svc-b.State = %q, want down", nodeByID["svc-b"].State)
	}

	// pg:5432 — only stale incoming → stale.
	if !nodeByID["pg:5432"].Stale {
		t.Errorf("pg:5432.Stale = false, want true")
	}

	// Edge svc-a → svc-b should use connected graph (target = "svc-b", not host:port).
	edgeByKey := make(map[string]Edge)
	for _, e := range resp.Edges {
		edgeByKey[e.Source+"→"+e.Target] = e
	}
	if e, ok := edgeByKey["svc-a→svc-b"]; !ok {
		t.Error("missing edge svc-a→svc-b")
	} else if e.Stale {
		t.Error("edge svc-a→svc-b should NOT be stale (current health exists)")
	}
	if e, ok := edgeByKey["svc-b→pg:5432"]; !ok {
		t.Error("missing edge svc-b→pg:5432")
	} else if !e.Stale {
		t.Error("edge svc-b→pg:5432 should be stale")
	}
}

func TestStaleDetection_LookbackDisabled(t *testing.T) {
	// lookback=0 → current behavior, no stale detection.
	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Name: "svc-go", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432"},
		},
		health: map[EdgeKey]float64{
			{Name: "svc-go", Host: "pg", Port: "5432"}: 1,
		},
		avg: map[EdgeKey]float64{},
	}

	builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, 0, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	for _, n := range resp.Nodes {
		if n.Stale {
			t.Errorf("node %q is stale with lookback=0", n.ID)
		}
	}
	for _, e := range resp.Edges {
		if e.Stale {
			t.Errorf("edge %s→%s is stale with lookback=0", e.Source, e.Target)
		}
	}
}

func TestStaleDetection_AllStale(t *testing.T) {
	// All edges in lookback but none in current health → everything unknown.
	mock := &mockPrometheusClient{
		lookbackEdges: []TopologyEdge{
			{Name: "svc-go", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432"},
			{Name: "svc-go", Dependency: "redis", Type: "redis", Host: "redis", Port: "6379"},
		},
		health:     map[EdgeKey]float64{}, // empty — nothing is current
		avg:        map[EdgeKey]float64{},
	}

	builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, time.Hour, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// All nodes should be stale + down (truly unavailable).
	for _, n := range resp.Nodes {
		if !n.Stale {
			t.Errorf("node %q.Stale = false, want true (all stale)", n.ID)
		}
		if n.State != "down" {
			t.Errorf("node %q.State = %q, want down", n.ID, n.State)
		}
	}

	// All edges should be stale + unknown.
	for _, e := range resp.Edges {
		if !e.Stale {
			t.Errorf("edge %s→%s Stale = false, want true", e.Source, e.Target)
		}
		if e.State != "unknown" {
			t.Errorf("edge %s→%s State = %q, want unknown", e.Source, e.Target, e.State)
		}
		if e.Health != -1 {
			t.Errorf("edge %s→%s Health = %v, want -1", e.Source, e.Target, e.Health)
		}
	}
}
