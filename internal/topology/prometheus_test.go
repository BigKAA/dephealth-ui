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
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
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
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
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
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer srv.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: srv.URL})

	// Without namespace — no filter injected.
	_, _ = client.QueryTopologyEdges(context.Background(), QueryOptions{})
	if capturedQuery != `group by (name, namespace, group, dependency, type, host, port, critical) (app_dependency_health)` {
		t.Errorf("unfiltered query = %q", capturedQuery)
	}

	// With namespace — filter injected.
	_, _ = client.QueryTopologyEdges(context.Background(), QueryOptions{Namespace: "prod"})
	want := `group by (name, namespace, group, dependency, type, host, port, critical) (app_dependency_health{namespace="prod"})`
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
		_, _ = w.Write([]byte(topologyEdgesResponse))
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
	want := `group by (name, namespace, group, dependency, type, host, port, critical) (last_over_time(app_dependency_health[1h]))`
	if capturedQuery != want {
		t.Errorf("query = %q, want %q", capturedQuery, want)
	}

	// With namespace.
	_, _ = client.QueryTopologyEdgesLookback(context.Background(), QueryOptions{Namespace: "prod"}, 30*time.Minute)
	want = `group by (name, namespace, group, dependency, type, host, port, critical) (last_over_time(app_dependency_health{namespace="prod"}[30m]))`
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

const dependencyStatusResponse = `{
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [
      {
        "metric": {"name": "svc-go", "host": "pg-primary", "port": "5432", "status": "ok"},
        "value": [1700000000, "1"]
      },
      {
        "metric": {"name": "svc-go", "host": "redis", "port": "6379", "status": "timeout"},
        "value": [1700000000, "1"]
      }
    ]
  }
}`

const dependencyStatusDetailResponse = `{
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [
      {
        "metric": {"name": "svc-go", "host": "redis", "port": "6379", "detail": "connection_refused"},
        "value": [1700000000, "1"]
      }
    ]
  }
}`

func TestQueryDependencyStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(dependencyStatusResponse))
	}))
	defer srv.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: srv.URL})
	result, err := client.QueryDependencyStatus(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("QueryDependencyStatus() error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("got %d results, want 2", len(result))
	}

	pgKey := EdgeKey{Name: "svc-go", Host: "pg-primary", Port: "5432"}
	if result[pgKey] != "ok" {
		t.Errorf("pg status = %q, want ok", result[pgKey])
	}

	redisKey := EdgeKey{Name: "svc-go", Host: "redis", Port: "6379"}
	if result[redisKey] != "timeout" {
		t.Errorf("redis status = %q, want timeout", result[redisKey])
	}
}

func TestQueryDependencyStatusDetail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(dependencyStatusDetailResponse))
	}))
	defer srv.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: srv.URL})
	result, err := client.QueryDependencyStatusDetail(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("QueryDependencyStatusDetail() error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("got %d results, want 1", len(result))
	}

	redisKey := EdgeKey{Name: "svc-go", Host: "redis", Port: "6379"}
	if result[redisKey] != "connection_refused" {
		t.Errorf("redis detail = %q, want connection_refused", result[redisKey])
	}
}

func TestParseEdgeStringValues(t *testing.T) {
	results := []promResult{
		{Metric: map[string]string{"name": "svc-a", "host": "h1", "port": "80", "status": "ok"}},
		{Metric: map[string]string{"name": "svc-b", "host": "h2", "port": "443", "status": "timeout"}},
		{Metric: map[string]string{"name": "svc-c", "host": "h3", "port": "8080"}}, // no status label
	}

	m := parseEdgeStringValues(results, "status")
	if len(m) != 2 {
		t.Fatalf("got %d entries, want 2", len(m))
	}
	if m[EdgeKey{Name: "svc-a", Host: "h1", Port: "80"}] != "ok" {
		t.Error("expected svc-a status=ok")
	}
	if m[EdgeKey{Name: "svc-b", Host: "h2", Port: "443"}] != "timeout" {
		t.Error("expected svc-b status=timeout")
	}
	if _, ok := m[EdgeKey{Name: "svc-c", Host: "h3", Port: "8080"}]; ok {
		t.Error("svc-c should not be in map (no status label)")
	}
}

func TestQueryWithTimeParameter(t *testing.T) {
	var capturedTime string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTime = r.URL.Query().Get("time")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer srv.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: srv.URL})

	// Without time — no time parameter.
	_, _ = client.QueryTopologyEdges(context.Background(), QueryOptions{})
	if capturedTime != "" {
		t.Errorf("expected no time param, got %q", capturedTime)
	}

	// With time — time parameter should be Unix timestamp.
	ts := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	_, _ = client.QueryTopologyEdges(context.Background(), QueryOptions{Time: &ts})
	if capturedTime == "" {
		t.Fatal("expected time param, got empty")
	}
	if capturedTime != "1768478400" {
		t.Errorf("time param = %q, want 1768478400", capturedTime)
	}
}

const historicalAlertsResponse = `{
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [
      {
        "metric": {"__name__": "ALERTS", "alertname": "DependencyDown", "alertstate": "firing", "name": "svc-go", "namespace": "default", "severity": "critical"},
        "value": [1700000000, "1"]
      },
      {
        "metric": {"__name__": "ALERTS", "alertname": "DependencyDegraded", "alertstate": "firing", "service": "svc-python", "namespace": "default", "severity": "warning"},
        "value": [1700000000, "1"]
      },
      {
        "metric": {"__name__": "ALERTS", "alertname": "SomeOtherAlert", "alertstate": "firing"},
        "value": [1700000000, "1"]
      }
    ]
  }
}`

func TestQueryHistoricalAlerts(t *testing.T) {
	srv := newTestPromServer(historicalAlertsResponse)
	defer srv.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: srv.URL})
	at := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	alerts, err := client.QueryHistoricalAlerts(context.Background(), at)
	if err != nil {
		t.Fatalf("QueryHistoricalAlerts() error: %v", err)
	}

	// Third entry has no name/service label, should be skipped.
	if len(alerts) != 2 {
		t.Fatalf("got %d alerts, want 2", len(alerts))
	}

	a0 := alerts[0]
	if a0.AlertName != "DependencyDown" {
		t.Errorf("alert[0].AlertName = %q, want DependencyDown", a0.AlertName)
	}
	if a0.Service != "svc-go" {
		t.Errorf("alert[0].Service = %q, want svc-go", a0.Service)
	}
	if a0.Severity != "critical" {
		t.Errorf("alert[0].Severity = %q, want critical", a0.Severity)
	}
	if a0.Namespace != "default" {
		t.Errorf("alert[0].Namespace = %q, want default", a0.Namespace)
	}

	// Second alert uses "service" label instead of "name".
	a1 := alerts[1]
	if a1.Service != "svc-python" {
		t.Errorf("alert[1].Service = %q, want svc-python (from service label)", a1.Service)
	}
}

func TestQueryHistoricalAlertsEmpty(t *testing.T) {
	srv := newTestPromServer(`{"status":"success","data":{"resultType":"vector","result":[]}}`)
	defer srv.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: srv.URL})
	at := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	alerts, err := client.QueryHistoricalAlerts(context.Background(), at)
	if err != nil {
		t.Fatalf("QueryHistoricalAlerts() error: %v", err)
	}
	if len(alerts) != 0 {
		t.Errorf("got %d alerts, want 0", len(alerts))
	}
}

const queryRangeResponse = `{
  "status": "success",
  "data": {
    "resultType": "matrix",
    "result": [
      {
        "metric": {"name": "svc-go", "host": "pg", "port": "5432", "status": "ok"},
        "values": [[1700000000, "1"], [1700000015, "1"], [1700000030, "0"]]
      },
      {
        "metric": {"name": "svc-go", "host": "pg", "port": "5432", "status": "timeout"},
        "values": [[1700000000, "0"], [1700000015, "0"], [1700000030, "1"]]
      }
    ]
  }
}`

func TestQueryStatusRange(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(queryRangeResponse))
	}))
	defer srv.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: srv.URL})
	start := time.Date(2023, 11, 15, 0, 0, 0, 0, time.UTC)
	end := time.Date(2023, 11, 15, 1, 0, 0, 0, time.UTC)

	results, err := client.QueryStatusRange(context.Background(), start, end, 15*time.Second, "")
	if err != nil {
		t.Fatalf("QueryStatusRange() error: %v", err)
	}

	if capturedPath != "/api/v1/query_range" {
		t.Errorf("path = %q, want /api/v1/query_range", capturedPath)
	}

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	// Check first result (ok status).
	r0 := results[0]
	if r0.Key.Name != "svc-go" || r0.Key.Host != "pg" || r0.Key.Port != "5432" {
		t.Errorf("result[0].Key = %+v, unexpected", r0.Key)
	}
	if r0.Status != "ok" {
		t.Errorf("result[0].Status = %q, want ok", r0.Status)
	}
	if len(r0.Values) != 3 {
		t.Fatalf("result[0] has %d values, want 3", len(r0.Values))
	}
	if r0.Values[0].Value != 1 {
		t.Errorf("result[0].Values[0].Value = %v, want 1", r0.Values[0].Value)
	}
	if r0.Values[2].Value != 0 {
		t.Errorf("result[0].Values[2].Value = %v, want 0", r0.Values[2].Value)
	}
}

func TestQueryStatusRangeWithNamespace(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
	}))
	defer srv.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: srv.URL})
	start := time.Date(2023, 11, 15, 0, 0, 0, 0, time.UTC)
	end := time.Date(2023, 11, 15, 1, 0, 0, 0, time.UTC)

	_, err := client.QueryStatusRange(context.Background(), start, end, time.Minute, "prod")
	if err != nil {
		t.Fatalf("QueryStatusRange() error: %v", err)
	}

	want := `app_dependency_status{namespace="prod"} == 1`
	if capturedQuery != want {
		t.Errorf("query = %q, want %q", capturedQuery, want)
	}
}

func TestQueryErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("service unavailable"))
	}))
	defer srv.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: srv.URL})
	_, err := client.QueryTopologyEdges(context.Background(), QueryOptions{})
	if err == nil {
		t.Fatal("expected error for 503 response")
	}
}

// --- Group label tests ---

const topologyEdgesWithGroupResponse = `{
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [
      {
        "metric": {"name": "svc-go", "namespace": "ns1", "group": "cluster-1", "dependency": "postgres", "type": "postgres", "host": "pg-primary", "port": "5432", "critical": "yes"},
        "value": [1700000000, "1"]
      },
      {
        "metric": {"name": "svc-python", "namespace": "ns1", "group": "cluster-2", "dependency": "redis", "type": "redis", "host": "redis", "port": "6379", "critical": "no"},
        "value": [1700000000, "1"]
      }
    ]
  }
}`

func TestQueryTopologyEdges_GroupLabel(t *testing.T) {
	srv := newTestPromServer(topologyEdgesWithGroupResponse)
	defer srv.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: srv.URL})
	edges, err := client.QueryTopologyEdges(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("QueryTopologyEdges() error: %v", err)
	}

	if len(edges) != 2 {
		t.Fatalf("got %d edges, want 2", len(edges))
	}

	if edges[0].Group != "cluster-1" {
		t.Errorf("edge[0].Group = %q, want cluster-1", edges[0].Group)
	}
	if edges[1].Group != "cluster-2" {
		t.Errorf("edge[1].Group = %q, want cluster-2", edges[1].Group)
	}
}

func TestQueryTopologyEdges_NoGroupLabel(t *testing.T) {
	// Old SDK: no group label → Group should be empty.
	srv := newTestPromServer(topologyEdgesResponse)
	defer srv.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: srv.URL})
	edges, err := client.QueryTopologyEdges(context.Background(), QueryOptions{})
	if err != nil {
		t.Fatalf("QueryTopologyEdges() error: %v", err)
	}

	for i, e := range edges {
		if e.Group != "" {
			t.Errorf("edge[%d].Group = %q, want empty (no group label)", i, e.Group)
		}
	}
}

func TestOptFilter(t *testing.T) {
	tests := []struct {
		name string
		opts QueryOptions
		want string
	}{
		{"empty", QueryOptions{}, ""},
		{"namespace only", QueryOptions{Namespace: "prod"}, `{namespace="prod"}`},
		{"group only", QueryOptions{Group: "cluster-1"}, `{group="cluster-1"}`},
		{"both", QueryOptions{Namespace: "prod", Group: "cluster-1"}, `{namespace="prod",group="cluster-1"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := optFilter(tt.opts)
			if got != tt.want {
				t.Errorf("optFilter(%+v) = %q, want %q", tt.opts, got, tt.want)
			}
		})
	}
}

func TestQueryWithGroupFilter(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer srv.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: srv.URL})

	// Group filter only.
	_, _ = client.QueryTopologyEdges(context.Background(), QueryOptions{Group: "cluster-1"})
	want := `group by (name, namespace, group, dependency, type, host, port, critical) (app_dependency_health{group="cluster-1"})`
	if capturedQuery != want {
		t.Errorf("group filter query = %q, want %q", capturedQuery, want)
	}

	// Combined namespace + group.
	_, _ = client.QueryTopologyEdges(context.Background(), QueryOptions{Namespace: "prod", Group: "cluster-1"})
	want = `group by (name, namespace, group, dependency, type, host, port, critical) (app_dependency_health{namespace="prod",group="cluster-1"})`
	if capturedQuery != want {
		t.Errorf("combined filter query = %q, want %q", capturedQuery, want)
	}
}
