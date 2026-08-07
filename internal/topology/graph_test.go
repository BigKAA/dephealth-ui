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
	edges            []TopologyEdge
	lookbackEdges    []TopologyEdge // edges returned by lookback query (nil = same as edges)
	health           map[EdgeKey]float64
	avg              map[EdgeKey]float64
	p99              map[EdgeKey]float64
	depStatus        map[EdgeKey]string // SDK v0.4.1 dependency status
	depDetail        map[EdgeKey]string // SDK v0.4.1 dependency status detail
	historicalAlerts []HistoricalAlert  // historical alerts for history mode
	err              error              // default error for all methods
	edgesErr         error              // override for QueryTopologyEdges
	lookbackErr      error              // override for QueryTopologyEdgesLookback
	healthErr        error              // override for QueryHealthState
	avgErr           error              // override for QueryAvgLatency
	depStatusErr     error              // override for QueryDependencyStatus
	depDetailErr     error              // override for QueryDependencyStatusDetail
	histAlertsErr    error              // override for QueryHistoricalAlerts
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

func (m *mockPrometheusClient) QueryDependencyStatus(_ context.Context, _ QueryOptions) (map[EdgeKey]string, error) {
	if m.depStatusErr != nil {
		return nil, m.depStatusErr
	}
	return m.depStatus, m.err
}

func (m *mockPrometheusClient) QueryDependencyStatusDetail(_ context.Context, _ QueryOptions) (map[EdgeKey]string, error) {
	if m.depDetailErr != nil {
		return nil, m.depDetailErr
	}
	return m.depDetail, m.err
}

func (m *mockPrometheusClient) QueryHistoricalAlerts(_ context.Context, _ time.Time) ([]HistoricalAlert, error) {
	if m.histAlertsErr != nil {
		return nil, m.histAlertsErr
	}
	return m.historicalAlerts, m.err
}

func (m *mockPrometheusClient) QueryStatusRange(_ context.Context, _, _ time.Time, _ time.Duration, _ string) ([]RangeResult, error) {
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
			{Name: "svc-go", Dependency: "postgres"}:     1,
			{Name: "svc-go", Dependency: "redis"}:        0,
			{Name: "svc-python", Dependency: "postgres"}: 1,
		},
		avg: map[EdgeKey]float64{
			{Name: "svc-go", Dependency: "postgres"}:     0.0052,
			{Name: "svc-go", Dependency: "redis"}:        0.001,
			{Name: "svc-python", Dependency: "postgres"}: 0.003,
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

	// Nodes: svc-go, svc-python, svc-go/postgres, svc-go/redis, svc-python/postgres = 5
	// (composite "source/dependency" ids: distinct per service→dependency pair).
	if len(resp.Nodes) != 5 {
		t.Errorf("got %d nodes, want 5", len(resp.Nodes))
	}

	// Edges: svc-go→svc-go/postgres, svc-go→svc-go/redis, svc-python→svc-python/postgres = 3.
	if len(resp.Edges) != 3 {
		t.Errorf("got %d edges, want 3", len(resp.Edges))
	}

	// Check meta.
	if resp.Meta.TTL != 15 {
		t.Errorf("Meta.TTL = %d, want 15", resp.Meta.TTL)
	}
	if resp.Meta.NodeCount != 5 {
		t.Errorf("Meta.NodeCount = %d, want 5", resp.Meta.NodeCount)
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

	// svc-go/postgres — dependency node with 1 incoming healthy edge → ok.
	if n, ok := nodeByID["svc-go/postgres"]; !ok {
		t.Error("missing svc-go/postgres node")
	} else {
		if n.State != "ok" {
			t.Errorf("svc-go/postgres.State = %q, want %q", n.State, "ok")
		}
		if n.Type != "postgres" {
			t.Errorf("svc-go/postgres.Type = %q, want %q", n.Type, "postgres")
		}
		if n.Label != "postgres" {
			t.Errorf("svc-go/postgres.Label = %q, want %q", n.Label, "postgres")
		}
		if n.Host != "pg-primary" {
			t.Errorf("svc-go/postgres.Host = %q, want %q", n.Host, "pg-primary")
		}
		if n.Port != "5432" {
			t.Errorf("svc-go/postgres.Port = %q, want %q", n.Port, "5432")
		}
	}

	// svc-go/redis — dependency node with 1 incoming down edge → down.
	if n, ok := nodeByID["svc-go/redis"]; !ok {
		t.Error("missing svc-go/redis node")
	} else {
		if n.State != "down" {
			t.Errorf("svc-go/redis.State = %q, want %q", n.State, "down")
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
	if e, ok := edgeByTarget["svc-go→svc-go/postgres"]; ok {
		if !e.Critical {
			t.Errorf("edge svc-go→svc-go/postgres Critical = false, want true")
		}
	}
	if e, ok := edgeByTarget["svc-go→svc-go/redis"]; ok {
		if e.Critical {
			t.Errorf("edge svc-go→svc-go/redis Critical = true, want false")
		}
	}
}

func TestDepNode_SameEndpointDifferentDependency_NotMerged(t *testing.T) {
	// Two services connect to the same host:port but reference distinct
	// dependency names. A real connection is identified by the dependency
	// name, not the shared transport endpoint, so each pair must produce its
	// own node — they must NOT be merged.
	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Name: "svc-go", Dependency: "my-redis", Type: "redis", Host: "redis-host", Port: "6379"},
			{Name: "svc-python", Dependency: "redis-cache", Type: "redis", Host: "redis-host", Port: "6379"},
		},
		health: map[EdgeKey]float64{
			{Name: "svc-go", Dependency: "my-redis"}:     1,
			{Name: "svc-python", Dependency: "redis-cache"}: 0,
		},
		avg: map[EdgeKey]float64{},
	}

	builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, 0, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Nodes: svc-go, svc-python, svc-go/my-redis, svc-python/redis-cache = 4 (NOT merged).
	if len(resp.Nodes) != 4 {
		t.Errorf("got %d nodes, want 4 (same host:port, different dependency → not merged)", len(resp.Nodes))
	}

	// Edges: 2 (one per service→dependency pair).
	if len(resp.Edges) != 2 {
		t.Errorf("got %d edges, want 2", len(resp.Edges))
	}

	nodeByID := make(map[string]Node)
	for _, n := range resp.Nodes {
		nodeByID[n.ID] = n
	}

	// Each dependency node has a single incoming edge, so it reflects its own health.
	if dep, ok := nodeByID["svc-go/my-redis"]; !ok {
		t.Fatal("missing svc-go/my-redis node")
	} else if dep.State != "ok" {
		t.Errorf("svc-go/my-redis.State = %q, want ok", dep.State)
	}
	if dep, ok := nodeByID["svc-python/redis-cache"]; !ok {
		t.Fatal("missing svc-python/redis-cache node")
	} else if dep.State != "down" {
		t.Errorf("svc-python/redis-cache.State = %q, want down", dep.State)
	}
}

func TestDepNode_SameDependencyDifferentService_NotMerged(t *testing.T) {
	// Two services depend on the same dependency name ("postgres") reached via
	// the same host:port from different groups. Because the dependency node id
	// is composite ("source/dependency"), each service gets its own dep node
	// with independent health.
	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Name: "svc-a", Group: "cluster-1", Dependency: "postgres", Type: "postgres", Host: "postgresql.db.svc", Port: "5432"},
			{Name: "svc-b", Group: "cluster-2", Dependency: "postgres", Type: "postgres", Host: "postgresql.db.svc", Port: "5432"},
		},
		health: map[EdgeKey]float64{
			{Name: "svc-a", Dependency: "postgres"}: 1,
			{Name: "svc-b", Dependency: "postgres"}: 0,
		},
		avg: map[EdgeKey]float64{},
	}

	builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, 0, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Nodes: svc-a, svc-b, svc-a/postgres, svc-b/postgres = 4 (NOT merged).
	if len(resp.Nodes) != 4 {
		t.Errorf("got %d nodes, want 4 (composite dep node id, not merged)", len(resp.Nodes))
	}

	nodeByID := make(map[string]Node)
	for _, n := range resp.Nodes {
		nodeByID[n.ID] = n
	}

	// Each dep node reflects its single incoming edge's health.
	if dep, ok := nodeByID["svc-a/postgres"]; !ok {
		t.Fatal("missing svc-a/postgres node")
	} else if dep.State != "ok" {
		t.Errorf("svc-a/postgres.State = %q, want ok", dep.State)
	}
	if dep, ok := nodeByID["svc-b/postgres"]; !ok {
		t.Fatal("missing svc-b/postgres node")
	} else if dep.State != "down" {
		t.Errorf("svc-b/postgres.State = %q, want down", dep.State)
	}
}

func TestDepNode_SameHost_DifferentDependencies_NotCollapsed(t *testing.T) {
	// Regression: a single service (nginx-back-app1) reaches four DISTINCT
	// dependencies behind one shared ingress endpoint (same host:port, e.g. from
	// a reverse proxy). Edges are keyed by (service, dependency), so each
	// dependency must produce its own node and edge — they must NOT collapse.
	const (
		src  = "nginx-back-app1"
		host = "smart.prod-dc-cloud2.passport.local"
		port = "80"
	)
	deps := []string{"app-mkaddh-bpm", "app-mkaddh-order", "app-oasi-agr", "app-oasi-pkr"}

	edges := make([]TopologyEdge, 0, len(deps))
	health := make(map[EdgeKey]float64, len(deps))
	avg := make(map[EdgeKey]float64, len(deps))
	for _, d := range deps {
		edges = append(edges, TopologyEdge{
			Name: src, Group: "oasi", Dependency: d, Type: "http", Host: host, Port: port, Critical: true,
		})
		health[EdgeKey{Name: src, Dependency: d}] = 1
		avg[EdgeKey{Name: src, Dependency: d}] = 0.01
	}

	mock := &mockPrometheusClient{edges: edges, health: health, avg: avg}
	builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, 0, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// 1 service node + 4 dependency nodes.
	if len(resp.Nodes) != 5 {
		t.Errorf("got %d nodes, want 5 (1 service + 4 distinct deps)", len(resp.Nodes))
	}

	// Exactly 4 edges from the service, one per dependency.
	var out []Edge
	for _, e := range resp.Edges {
		if e.Source == src {
			out = append(out, e)
		}
	}
	if len(out) != 4 {
		t.Fatalf("got %d edges from %s, want 4", len(out), src)
	}

	// Each edge targets a distinct composite "source/dependency" id, and each
	// shares the same host:port — proving the shared endpoint was not used as the id.
	seen := make(map[string]bool, len(out))
	for _, e := range out {
		if seen[e.Target] {
			t.Errorf("duplicate edge target %q (deps must not collapse)", e.Target)
		}
		seen[e.Target] = true
	}
	for _, d := range deps {
		id := src + "/" + d
		if !seen[id] {
			t.Errorf("missing expected composite target %q", id)
		}
	}
}

func TestDependencyNodeColoring(t *testing.T) {
	// With composite "source/dependency" node ids, each dep node state depends
	// on incoming edge health.
	tests := []struct {
		name      string
		edges     []TopologyEdge
		health    map[EdgeKey]float64
		depNodeID string
		wantState string
	}{
		{
			name: "healthy edge → ok",
			edges: []TopologyEdge{
				{Name: "svc-a", Dependency: "pg", Type: "postgres", Host: "pg", Port: "5432"},
			},
			health: map[EdgeKey]float64{
				{Name: "svc-a", Dependency: "pg"}: 1,
			},
			depNodeID: "svc-a/pg",
			wantState: "ok",
		},
		{
			name: "down edge → down",
			edges: []TopologyEdge{
				{Name: "svc-a", Dependency: "pg", Type: "postgres", Host: "pg", Port: "5432"},
			},
			health: map[EdgeKey]float64{
				{Name: "svc-a", Dependency: "pg"}: 0,
			},
			depNodeID: "svc-a/pg",
			wantState: "down",
		},
		{
			name: "single service → state from that edge",
			edges: []TopologyEdge{
				{Name: "svc-a", Dependency: "redis", Type: "redis", Host: "redis", Port: "6379"},
			},
			health: map[EdgeKey]float64{
				{Name: "svc-a", Dependency: "redis"}: 0,
			},
			depNodeID: "svc-a/redis",
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
			{Name: "svc-go", Dependency: "postgres"}: 1,
			{Name: "svc-go", Dependency: "redis"}:    1,
		},
		avg: map[EdgeKey]float64{
			{Name: "svc-go", Dependency: "postgres"}: 0.005,
			{Name: "svc-go", Dependency: "redis"}:    0.001,
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

	// Edge svc-go→svc-go/postgres should be overridden to "down".
	edgeByTarget := make(map[string]Edge)
	for _, e := range resp.Edges {
		edgeByTarget[e.Target] = e
	}

	pgEdge := edgeByTarget["svc-go/postgres"]
	if pgEdge.State != "down" {
		t.Errorf("svc-go/postgres edge State = %q, want %q", pgEdge.State, "down")
	}
	if pgEdge.Health != 0 {
		t.Errorf("svc-go/postgres edge Health = %v, want 0", pgEdge.Health)
	}
	if pgEdge.AlertCount != 1 {
		t.Errorf("svc-go/postgres edge AlertCount = %d, want 1", pgEdge.AlertCount)
	}
	if pgEdge.AlertSeverity != "critical" {
		t.Errorf("svc-go/postgres edge AlertSeverity = %q, want %q", pgEdge.AlertSeverity, "critical")
	}

	// redis edge should have no alerts.
	redisEdge := edgeByTarget["svc-go/redis"]
	if redisEdge.AlertCount != 0 {
		t.Errorf("svc-go/redis edge AlertCount = %d, want 0", redisEdge.AlertCount)
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
			{Name: "svc-go", Dependency: "postgres"}: 1,
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
			{Name: "uniproxy-01", Dependency: "uniproxy-02"}: 1,
			{Name: "uniproxy-02", Dependency: "redis"}:       1,
		},
		avg: map[EdgeKey]float64{},
	}

	builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, 0, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Nodes: uniproxy-01, uniproxy-02, uniproxy-02/redis = 3 (NOT 4!)
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

	// No separate composite node for uniproxy-02 (it is a known service, so it
	// connects into the service node rather than spawning uniproxy-01/uniproxy-02).
	if _, ok := nodeByID["uniproxy-01/uniproxy-02"]; ok {
		t.Error("unexpected separate dependency node uniproxy-01/uniproxy-02; should be merged into service node")
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

	// Edge from uniproxy-02 to redis targets the composite dep node "uniproxy-02/redis".
	if _, ok := edgeByKey["uniproxy-02→uniproxy-02/redis"]; !ok {
		t.Error("missing edge uniproxy-02→uniproxy-02/redis")
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
			{Name: "svc-a", Dependency: "svc-b"}:     1,
			{Name: "svc-b", Dependency: "postgres"}: 1,
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
					{Name: "svc-go", Dependency: "postgres"}: 1,
					{Name: "svc-go", Dependency: "redis"}:    1,
				},
				avg: map[EdgeKey]float64{
					{Name: "svc-go", Dependency: "postgres"}: 0.005,
					{Name: "svc-go", Dependency: "redis"}:    0.001,
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
			{Name: "svc-go", Dependency: "postgres"}: 1,
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
			{Name: "svc-go", Dependency: "postgres"}: 1,
			{Name: "svc-go", Dependency: "redis"}:    1,
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

	pgEdge := edgeByKey["svc-go→svc-go/postgres"]
	if pgEdge.AlertSeverity != "critical" {
		t.Errorf("pg edge AlertSeverity = %q, want %q", pgEdge.AlertSeverity, "critical")
	}
	if pgEdge.AlertCount != 1 {
		t.Errorf("pg edge AlertCount = %d, want 1", pgEdge.AlertCount)
	}

	redisEdge := edgeByKey["svc-go→svc-go/redis"]
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
			{Name: "svc-go", Dependency: "postgres"}: 1,
			{Name: "svc-go", Dependency: "redis"}:    1,
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
			{Name: "svc-python", Dependency: "postgres"}: 1,
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

	// Under composite dep node ids, svc-go and svc-python each get their own
	// postgres dependency node (postgres is not a known service).
	// svc-go/postgres: only stale incoming (svc-go) → stale.
	goPg := nodeByID["svc-go/postgres"]
	if !goPg.Stale {
		t.Errorf("svc-go/postgres.Stale = false, want true (only stale incoming edge from svc-go)")
	}
	// svc-python/postgres: only current incoming (svc-python) → not stale.
	pyPg := nodeByID["svc-python/postgres"]
	if pyPg.Stale {
		t.Errorf("svc-python/postgres.Stale = true, want false (current incoming edge from svc-python)")
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
			{Name: "svc-go", Dependency: "postgres"}: 1,
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

	// svc-go/redis — only stale incoming → fully stale.
	redis := nodeByID["svc-go/redis"]
	if !redis.Stale {
		t.Errorf("svc-go/redis.Stale = false, want true")
	}
	if redis.State != "down" {
		t.Errorf("svc-go/redis.State = %q, want %q", redis.State, "down")
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
			{Name: "svc-a", Dependency: "svc-b"}: 1,
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

	// svc-b/postgres — only stale incoming → stale.
	if !nodeByID["svc-b/postgres"].Stale {
		t.Errorf("svc-b/postgres.Stale = false, want true")
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
	if e, ok := edgeByKey["svc-b→svc-b/postgres"]; !ok {
		t.Error("missing edge svc-b→svc-b/postgres")
	} else if !e.Stale {
		t.Error("edge svc-b→svc-b/postgres should be stale")
	}
}

func TestStaleDetection_LookbackDisabled(t *testing.T) {
	// lookback=0 → current behavior, no stale detection.
	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Name: "svc-go", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432"},
		},
		health: map[EdgeKey]float64{
			{Name: "svc-go", Dependency: "postgres"}: 1,
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

func TestBuildWithDependencyStatus(t *testing.T) {
	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Name: "svc-go", Namespace: "default", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432", Critical: true},
			{Name: "svc-go", Namespace: "default", Dependency: "redis", Type: "redis", Host: "redis", Port: "6379", Critical: false},
			{Name: "svc-old", Namespace: "default", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432", Critical: true},
		},
		health: map[EdgeKey]float64{
			{Name: "svc-go", Dependency: "postgres"}: 1,
			{Name: "svc-go", Dependency: "redis"}:    0,
			{Name: "svc-old", Dependency: "postgres"}: 1,
		},
		avg: map[EdgeKey]float64{},
		depStatus: map[EdgeKey]string{
			{Name: "svc-go", Dependency: "postgres"}: "ok",
			{Name: "svc-go", Dependency: "redis"}:    "timeout",
		},
		depDetail: map[EdgeKey]string{
			{Name: "svc-go", Dependency: "redis"}: "connection_refused",
		},
	}

	builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, 0, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	edgeByKey := make(map[string]Edge)
	for _, e := range resp.Edges {
		edgeByKey[e.Source+"→"+e.Target] = e
	}

	// svc-go → svc-go/postgres: has status=ok, no detail.
	pgEdge := edgeByKey["svc-go→svc-go/postgres"]
	if pgEdge.Status != "ok" {
		t.Errorf("pg edge Status = %q, want ok", pgEdge.Status)
	}
	if pgEdge.Detail != "" {
		t.Errorf("pg edge Detail = %q, want empty", pgEdge.Detail)
	}

	// svc-go → svc-go/redis: has status=timeout, detail=connection_refused.
	redisEdge := edgeByKey["svc-go→svc-go/redis"]
	if redisEdge.Status != "timeout" {
		t.Errorf("redis edge Status = %q, want timeout", redisEdge.Status)
	}
	if redisEdge.Detail != "connection_refused" {
		t.Errorf("redis edge Detail = %q, want connection_refused", redisEdge.Detail)
	}

	// svc-old → svc-old/postgres: old SDK, no status/detail (backward compat).
	oldPgEdge := edgeByKey["svc-old→svc-old/postgres"]
	if oldPgEdge.Status != "" {
		t.Errorf("old pg edge Status = %q, want empty", oldPgEdge.Status)
	}
	if oldPgEdge.Detail != "" {
		t.Errorf("old pg edge Detail = %q, want empty", oldPgEdge.Detail)
	}
}

func TestBuildWithDependencyStatusQueryFailure(t *testing.T) {
	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Name: "svc-go", Namespace: "default", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432"},
		},
		health: map[EdgeKey]float64{
			{Name: "svc-go", Dependency: "postgres"}: 1,
		},
		avg:          map[EdgeKey]float64{},
		depStatusErr: errors.New("metric not found"),
		depDetailErr: errors.New("metric not found"),
	}

	builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, 0, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("Build() should not fail on status query error: %v", err)
	}

	// Partial data: response should still have edges.
	if len(resp.Edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(resp.Edges))
	}
	// Status/detail should be empty (graceful degradation).
	if resp.Edges[0].Status != "" {
		t.Errorf("edge Status = %q, want empty on query failure", resp.Edges[0].Status)
	}
	// Meta should report partial.
	if !resp.Meta.Partial {
		t.Error("expected partial=true when status queries fail")
	}
}

func TestStaleDetection_AllStale(t *testing.T) {
	// All edges in lookback but none in current health → everything unknown.
	mock := &mockPrometheusClient{
		lookbackEdges: []TopologyEdge{
			{Name: "svc-go", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432"},
			{Name: "svc-go", Dependency: "redis", Type: "redis", Host: "redis", Port: "6379"},
		},
		health: map[EdgeKey]float64{}, // empty — nothing is current
		avg:    map[EdgeKey]float64{},
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

func TestGraphBuilder_Build_HistoryMode(t *testing.T) {
	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Name: "svc-go", Namespace: "default", Dependency: "postgres", Type: "postgres", Host: "pg-primary", Port: "5432", Critical: true},
		},
		health: map[EdgeKey]float64{
			{Name: "svc-go", Dependency: "postgres"}: 1,
		},
		avg: map[EdgeKey]float64{
			{Name: "svc-go", Dependency: "postgres"}: 0.005,
		},
		historicalAlerts: []HistoricalAlert{
			{AlertName: "DependencyDown", Service: "svc-go", Namespace: "default", Severity: "critical"},
		},
	}

	builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, 0, nil, testSeverityLevels())

	ts := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	resp, err := builder.Build(context.Background(), QueryOptions{Time: &ts})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Meta should indicate history mode.
	if !resp.Meta.IsHistory {
		t.Error("Meta.IsHistory = false, want true")
	}
	if resp.Meta.Time == nil {
		t.Fatal("Meta.Time = nil, want non-nil")
	}
	if !resp.Meta.Time.Equal(ts) {
		t.Errorf("Meta.Time = %v, want %v", resp.Meta.Time, ts)
	}

	// Should have alerts from historical data.
	if len(resp.Alerts) != 1 {
		t.Fatalf("got %d alerts, want 1", len(resp.Alerts))
	}
	if resp.Alerts[0].AlertName != "DependencyDown" {
		t.Errorf("alert[0].AlertName = %q, want DependencyDown", resp.Alerts[0].AlertName)
	}
}

func TestGraphBuilder_Build_HistoryMode_UsesHistoricalAlerts(t *testing.T) {
	// Verify that in history mode, historical alerts from Prometheus are used
	// instead of AlertManager alerts.
	amMock := &mockAlertManagerClient{
		alerts: []alerts.Alert{
			{AlertName: "LiveAlert", Service: "svc-go", Dependency: "postgres", Severity: "critical", State: "firing"},
		},
	}

	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Name: "svc-go", Namespace: "default", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432", Critical: true},
		},
		health: map[EdgeKey]float64{
			{Name: "svc-go", Dependency: "postgres"}: 1,
		},
		avg: map[EdgeKey]float64{},
		historicalAlerts: []HistoricalAlert{
			{AlertName: "HistoricalAlert", Service: "svc-go", Namespace: "default", Severity: "warning"},
		},
	}

	builder := NewGraphBuilder(mock, amMock, GrafanaConfig{}, 15*time.Second, 0, nil, testSeverityLevels())

	ts := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	resp, err := builder.Build(context.Background(), QueryOptions{Time: &ts})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Should have historical alert, not AM alert.
	if len(resp.Alerts) != 1 {
		t.Fatalf("got %d alerts, want 1", len(resp.Alerts))
	}
	if resp.Alerts[0].AlertName != "HistoricalAlert" {
		t.Errorf("alert = %q, want HistoricalAlert (not LiveAlert from AM)", resp.Alerts[0].AlertName)
	}
}

func TestGraphBuilder_Build_LiveMode_Meta(t *testing.T) {
	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Name: "svc-go", Namespace: "default", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432", Critical: true},
		},
		health: map[EdgeKey]float64{
			{Name: "svc-go", Dependency: "postgres"}: 1,
		},
		avg: map[EdgeKey]float64{},
	}

	builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, 0, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Live mode: meta should NOT indicate history.
	if resp.Meta.IsHistory {
		t.Error("Meta.IsHistory = true, want false in live mode")
	}
	if resp.Meta.Time != nil {
		t.Errorf("Meta.Time = %v, want nil in live mode", resp.Meta.Time)
	}
}

// --- Group label tests ---

func TestGroupPropagation(t *testing.T) {
	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Name: "svc-go", Namespace: "ns1", Group: "cluster-1", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432"},
			{Name: "svc-python", Namespace: "ns1", Group: "cluster-2", Dependency: "redis", Type: "redis", Host: "redis", Port: "6379"},
		},
		health: map[EdgeKey]float64{
			{Name: "svc-go", Dependency: "postgres"}:     1,
			{Name: "svc-python", Dependency: "redis"}:    1,
		},
		avg: map[EdgeKey]float64{},
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

	// Service nodes should have group.
	if nodeByID["svc-go"].Group != "cluster-1" {
		t.Errorf("svc-go.Group = %q, want cluster-1", nodeByID["svc-go"].Group)
	}
	if nodeByID["svc-python"].Group != "cluster-2" {
		t.Errorf("svc-python.Group = %q, want cluster-2", nodeByID["svc-python"].Group)
	}

	// Dependency nodes should have group from the edge.
	if nodeByID["svc-go/postgres"].Group != "cluster-1" {
		t.Errorf("svc-go/postgres.Group = %q, want cluster-1", nodeByID["svc-go/postgres"].Group)
	}
	if nodeByID["svc-python/redis"].Group != "cluster-2" {
		t.Errorf("svc-python/redis.Group = %q, want cluster-2", nodeByID["svc-python/redis"].Group)
	}
}

func TestGroupBackwardCompat(t *testing.T) {
	// Old SDK metrics without group label should still work.
	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Name: "svc-old", Namespace: "default", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432"},
		},
		health: map[EdgeKey]float64{
			{Name: "svc-old", Dependency: "postgres"}: 1,
		},
		avg: map[EdgeKey]float64{},
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

	// Group should be empty (backward compat).
	if nodeByID["svc-old"].Group != "" {
		t.Errorf("svc-old.Group = %q, want empty (old SDK)", nodeByID["svc-old"].Group)
	}
	// Namespace should still work.
	if nodeByID["svc-old"].Namespace != "default" {
		t.Errorf("svc-old.Namespace = %q, want default", nodeByID["svc-old"].Namespace)
	}
	// Dependency node should use composite source/dependency ID (no group).
	if _, ok := nodeByID["svc-old/postgres"]; !ok {
		t.Error("missing svc-old/postgres node")
	}
}

func TestResolveDepNamespace(t *testing.T) {
	tests := []struct {
		host string
		want string
	}{
		// FQDN patterns.
		{"redis.dephealth-redis.svc.cluster.local", "dephealth-redis"},
		{"pg-primary.dephealth-postgresql.svc", "dephealth-postgresql"},
		{"svc-b.ns1.svc.cluster.local", "ns1"},
		// Short names or bare metal hosts — no namespace.
		{"redis", ""},
		{"pg-primary", ""},
		{"192.168.1.100", ""},
		{"", ""},
		// Edge case: just "svc" in the name but not the right position.
		{"svc.example.com", ""},
		// Two-part name with svc at index 1 (need index >= 2).
		{"a.svc", ""},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			got := resolveDepNamespace(tt.host)
			if got != tt.want {
				t.Errorf("resolveDepNamespace(%q) = %q, want %q", tt.host, got, tt.want)
			}
		})
	}
}

func TestDepNamespaceResolution_FQDN(t *testing.T) {
	// Dependency node with FQDN host should get namespace resolved.
	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Name: "svc-go", Namespace: "ns1", Group: "cluster-1", Dependency: "redis", Type: "redis", Host: "redis.dephealth-redis.svc.cluster.local", Port: "6379"},
		},
		health: map[EdgeKey]float64{
			{Name: "svc-go", Dependency: "redis"}: 1,
		},
		avg: map[EdgeKey]float64{},
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

	depNode := nodeByID["svc-go/redis"]
	if depNode.Namespace != "dephealth-redis" {
		t.Errorf("dep node Namespace = %q, want dephealth-redis", depNode.Namespace)
	}
}

func TestDepNamespaceResolution_Inheritance(t *testing.T) {
	// Dependency with bare-metal host (no FQDN) connected by a single service
	// should inherit that service's namespace.
	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Name: "svc-go", Namespace: "ns1", Group: "cluster-1", Dependency: "redis", Type: "redis", Host: "redis-host", Port: "6379"},
		},
		health: map[EdgeKey]float64{
			{Name: "svc-go", Dependency: "redis"}: 1,
		},
		avg: map[EdgeKey]float64{},
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

	depNode := nodeByID["svc-go/redis"]
	if depNode.Namespace != "ns1" {
		t.Errorf("dep node Namespace = %q, want ns1 (inherited)", depNode.Namespace)
	}
}

func TestDepNamespaceResolution_MultiSource(t *testing.T) {
	// Two services from different namespaces connect to the same host:port (no group).
	// Under composite "source/dependency" node ids, each service gets its own dep
	// node (svc-a/redis and svc-b/redis) — they are NOT merged. Each dep node has a
	// single source, so it inherits that source's namespace unambiguously.
	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Name: "svc-a", Namespace: "ns1", Dependency: "redis", Type: "redis", Host: "redis-host", Port: "6379"},
			{Name: "svc-b", Namespace: "ns2", Dependency: "redis", Type: "redis", Host: "redis-host", Port: "6379"},
		},
		health: map[EdgeKey]float64{
			{Name: "svc-a", Dependency: "redis"}: 1,
			{Name: "svc-b", Dependency: "redis"}: 1,
		},
		avg: map[EdgeKey]float64{},
	}

	builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, 0, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Nodes: svc-a, svc-b, svc-a/redis, svc-b/redis = 4 (NOT merged under composite ids).
	if len(resp.Nodes) != 4 {
		t.Errorf("got %d nodes, want 4", len(resp.Nodes))
	}

	nodeByID := make(map[string]Node)
	for _, n := range resp.Nodes {
		nodeByID[n.ID] = n
	}

	// Each dep node inherits the namespace of its single source service.
	depA, ok := nodeByID["svc-a/redis"]
	if !ok {
		t.Fatal("missing svc-a/redis node")
	}
	if depA.Namespace != "ns1" {
		t.Errorf("svc-a/redis.Namespace = %q, want ns1 (inherited from svc-a)", depA.Namespace)
	}
	depB, ok := nodeByID["svc-b/redis"]
	if !ok {
		t.Fatal("missing svc-b/redis node")
	}
	if depB.Namespace != "ns2" {
		t.Errorf("svc-b/redis.Namespace = %q, want ns2 (inherited from svc-b)", depB.Namespace)
	}
	// Each node exists and is healthy.
	if depA.State != "ok" {
		t.Errorf("svc-a/redis.State = %q, want ok", depA.State)
	}
	if depB.State != "ok" {
		t.Errorf("svc-b/redis.State = %q, want ok", depB.State)
	}
}

func TestIsEntryFromLabel(t *testing.T) {
	// Topology: A→B→redis, D→redis
	// Only svc-a has isentry=yes label on its edges.
	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Name: "svc-a", Namespace: "ns1", Dependency: "svc-b", Type: "http", Host: "svc-b.ns1.svc", Port: "8080", IsEntry: true},
			{Name: "svc-b", Namespace: "ns1", Dependency: "redis", Type: "redis", Host: "redis-host", Port: "6379"},
			{Name: "svc-d", Namespace: "ns1", Dependency: "redis", Type: "redis", Host: "redis-host", Port: "6379"},
		},
		health: map[EdgeKey]float64{
			{Name: "svc-a", Dependency: "svc-b"}:    1,
			{Name: "svc-b", Dependency: "redis"}:    1,
			{Name: "svc-d", Dependency: "redis"}:    1,
		},
		avg: map[EdgeKey]float64{},
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

	// svc-a: entry point (has isentry label)
	if n, ok := nodeByID["svc-a"]; !ok {
		t.Error("missing svc-a node")
	} else if !n.IsEntry {
		t.Error("svc-a.IsEntry = false, want true")
	}

	// svc-b: not entry (no isentry label)
	if n, ok := nodeByID["svc-b"]; !ok {
		t.Error("missing svc-b node")
	} else if n.IsEntry {
		t.Error("svc-b.IsEntry = true, want false")
	}

	// svc-d: not entry (no isentry label, even though no incoming edges)
	if n, ok := nodeByID["svc-d"]; !ok {
		t.Error("missing svc-d node")
	} else if n.IsEntry {
		t.Error("svc-d.IsEntry = true, want false")
	}

	// Dependency nodes are never entry points. Under composite ids svc-b/redis and
	// svc-d/redis are distinct nodes (svc-b and svc-d each depend on redis).
	if n, ok := nodeByID["svc-b/redis"]; !ok {
		t.Error("missing svc-b/redis node")
	} else if n.IsEntry {
		t.Error("svc-b/redis.IsEntry = true, want false")
	}
	if n, ok := nodeByID["svc-d/redis"]; !ok {
		t.Error("missing svc-d/redis node")
	} else if n.IsEntry {
		t.Error("svc-d/redis.IsEntry = true, want false")
	}
}

func TestIsEntry_NoLabels(t *testing.T) {
	// When no edges have IsEntry=true, no nodes should be entry points.
	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Name: "gateway", Namespace: "ns1", Dependency: "postgres", Type: "postgres", Host: "pg", Port: "5432"},
		},
		health: map[EdgeKey]float64{
			{Name: "gateway", Dependency: "postgres"}: 1,
		},
		avg: map[EdgeKey]float64{},
	}

	builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, 0, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	for _, n := range resp.Nodes {
		if n.IsEntry {
			t.Errorf("node %q.IsEntry = true, want false (no isentry labels in topology)", n.ID)
		}
	}
}

func TestEdgeDedup_DuplicateHostProducesOneEdge(t *testing.T) {
	// Edges are keyed by the logical (service, dependency) pair. When lookback
	// returns two series for the same dependency reached via different hosts
	// (e.g. a host label change), they collapse to a single EdgeKey, so only one
	// edge must be produced. Since the dependency is reported as live by the
	// health query, the surviving edge is not stale.
	currentKey := EdgeKey{Name: "svc-a", Dependency: "svc-b"}

	mock := &mockPrometheusClient{
		lookbackEdges: []TopologyEdge{
			// svc-a → svc-b via current host.
			{Name: "svc-a", Namespace: "ns", Dependency: "svc-b", Type: "http", Host: "svc-b.ns.svc", Port: "8080", Critical: true},
			// svc-a → svc-b via old host (same dependency name, different host).
			{Name: "svc-a", Namespace: "ns", Dependency: "svc-b", Type: "http", Host: "svc-b-old.ns.svc", Port: "8080", Critical: true},
			// svc-b is also a source so it's a known service name.
			{Name: "svc-b", Namespace: "ns", Dependency: "pg", Type: "postgres", Host: "pg", Port: "5432", Critical: true},
		},
		health: map[EdgeKey]float64{
			currentKey:                            1, // the dependency is reported as live
			{Name: "svc-b", Dependency: "pg"}: 1,
		},
		avg: map[EdgeKey]float64{
			currentKey: 0.003,
		},
	}

	builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, time.Hour, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Count edges from svc-a to svc-b: should be exactly 1 (collapsed to one EdgeKey).
	var svcAToB []Edge
	for _, e := range resp.Edges {
		if e.Source == "svc-a" && e.Target == "svc-b" {
			svcAToB = append(svcAToB, e)
		}
	}
	if len(svcAToB) != 1 {
		t.Fatalf("got %d edges svc-a→svc-b, want 1", len(svcAToB))
	}

	edge := svcAToB[0]
	// The dependency is live, so the edge must not be stale.
	if edge.Stale {
		t.Error("edge.Stale = true, want false (dependency is live)")
	}
	if edge.State != "ok" {
		t.Errorf("edge.State = %q, want %q", edge.State, "ok")
	}

	// Verify svc-b service node is not marked stale.
	nodeByID := make(map[string]Node)
	for _, n := range resp.Nodes {
		nodeByID[n.ID] = n
	}
	svcB := nodeByID["svc-b"]
	if svcB.Stale {
		t.Error("svc-b node Stale = true, want false")
	}
}

func TestEdgeDedup_BothCurrentKeepsWorstHealth(t *testing.T) {
	// Two non-stale series for the same (service, dependency) reached via
	// different host:port collapse into a single EdgeKey. The health map keeps
	// the minimum (worst) value across instances, so the surviving edge carries
	// the worst health.
	key1 := EdgeKey{Name: "svc-a", Dependency: "svc-b"}

	mock := &mockPrometheusClient{
		edges: []TopologyEdge{
			{Name: "svc-a", Namespace: "ns", Dependency: "svc-b", Type: "http", Host: "svc-b.ns.svc", Port: "8080"},
			{Name: "svc-a", Namespace: "ns", Dependency: "svc-b", Type: "http", Host: "svc-b.ns.svc", Port: "9090"},
			// svc-b must be a known service (source in some edge).
			{Name: "svc-b", Namespace: "ns", Dependency: "pg", Type: "postgres", Host: "pg", Port: "5432"},
		},
		health: map[EdgeKey]float64{
			key1:                               0, // worst health wins (min across instances)
			{Name: "svc-b", Dependency: "pg"}: 1,
		},
		avg: map[EdgeKey]float64{},
	}

	builder := NewGraphBuilder(mock, nil, GrafanaConfig{}, 15*time.Second, 0, nil, testSeverityLevels())
	resp, err := builder.Build(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Count edges from svc-a to svc-b (should dedup to 1).
	var svcAToB []Edge
	for _, e := range resp.Edges {
		if e.Source == "svc-a" && e.Target == "svc-b" {
			svcAToB = append(svcAToB, e)
		}
	}
	if len(svcAToB) != 1 {
		t.Fatalf("got %d edges svc-a→svc-b, want 1", len(svcAToB))
	}

	edge := svcAToB[0]
	if edge.Health != 0 {
		t.Errorf("edge.Health = %v, want 0 (worst health should win)", edge.Health)
	}
	if edge.State != "down" {
		t.Errorf("edge.State = %q, want %q", edge.State, "down")
	}
}

func TestEdgeBetter(t *testing.T) {
	tests := []struct {
		name      string
		candidate Edge
		existing  Edge
		want      bool
	}{
		{
			name:      "non-stale beats stale",
			candidate: Edge{Stale: false, Health: 1},
			existing:  Edge{Stale: true, Health: -1},
			want:      true,
		},
		{
			name:      "stale does not beat non-stale",
			candidate: Edge{Stale: true, Health: -1},
			existing:  Edge{Stale: false, Health: 1},
			want:      false,
		},
		{
			name:      "worse health beats better among non-stale",
			candidate: Edge{Stale: false, Health: 0},
			existing:  Edge{Stale: false, Health: 1},
			want:      true,
		},
		{
			name:      "better health does not beat worse among non-stale",
			candidate: Edge{Stale: false, Health: 1},
			existing:  Edge{Stale: false, Health: 0},
			want:      false,
		},
		{
			name:      "equal edges — no replacement",
			candidate: Edge{Stale: false, Health: 1},
			existing:  Edge{Stale: false, Health: 1},
			want:      false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := edgeBetter(tc.candidate, tc.existing)
			if got != tc.want {
				t.Errorf("edgeBetter() = %v, want %v", got, tc.want)
			}
		})
	}
}
