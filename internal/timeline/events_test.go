package timeline

import (
	"context"
	"testing"
	"time"

	"github.com/BigKAA/dephealth-ui/internal/topology"
)

func TestAutoStep(t *testing.T) {
	tests := []struct {
		rangeDur time.Duration
		wantStep time.Duration
	}{
		{30 * time.Minute, 15 * time.Second},
		{time.Hour, 15 * time.Second},
		{2 * time.Hour, time.Minute},
		{6 * time.Hour, time.Minute},
		{12 * time.Hour, 5 * time.Minute},
		{24 * time.Hour, 5 * time.Minute},
		{3 * 24 * time.Hour, 15 * time.Minute},
		{7 * 24 * time.Hour, 15 * time.Minute},
		{14 * 24 * time.Hour, time.Hour},
		{30 * 24 * time.Hour, time.Hour},
		{60 * 24 * time.Hour, 3 * time.Hour},
		{90 * 24 * time.Hour, 3 * time.Hour},
		{180 * 24 * time.Hour, 6 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.rangeDur.String(), func(t *testing.T) {
			got := AutoStep(tt.rangeDur)
			if got != tt.wantStep {
				t.Errorf("AutoStep(%v) = %v, want %v", tt.rangeDur, got, tt.wantStep)
			}
		})
	}
}

func TestClassifyTransition(t *testing.T) {
	tests := []struct {
		from, to string
		want     string
	}{
		{"ok", "timeout", "degradation"},
		{"ok", "error", "degradation"},
		{"timeout", "ok", "recovery"},
		{"error", "ok", "recovery"},
		{"connection_error", "ok", "recovery"},
		{"timeout", "error", "degradation"},
		{"error", "timeout", "recovery"},
		{"ok", "ok", "change"}, // same severity
	}

	for _, tt := range tests {
		t.Run(tt.from+"→"+tt.to, func(t *testing.T) {
			got := classifyTransition(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("classifyTransition(%q, %q) = %q, want %q", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

// mockPromClient implements topology.PrometheusClient for timeline tests.
type mockPromClient struct {
	statusRange []topology.RangeResult
	err         error
}

func (m *mockPromClient) QueryStatusRange(_ context.Context, _, _ time.Time, _ time.Duration, _ string) ([]topology.RangeResult, error) {
	return m.statusRange, m.err
}

// Stub methods to satisfy the full interface.
func (m *mockPromClient) QueryTopologyEdges(_ context.Context, _ topology.QueryOptions) ([]topology.TopologyEdge, error) {
	return nil, nil
}
func (m *mockPromClient) QueryHealthState(_ context.Context, _ topology.QueryOptions) (map[topology.EdgeKey]float64, error) {
	return nil, nil
}
func (m *mockPromClient) QueryAvgLatency(_ context.Context, _ topology.QueryOptions) (map[topology.EdgeKey]float64, error) {
	return nil, nil
}
func (m *mockPromClient) QueryP99Latency(_ context.Context, _ topology.QueryOptions) (map[topology.EdgeKey]float64, error) {
	return nil, nil
}
func (m *mockPromClient) QueryTopologyEdgesLookback(_ context.Context, _ topology.QueryOptions, _ time.Duration) ([]topology.TopologyEdge, error) {
	return nil, nil
}
func (m *mockPromClient) QueryInstances(_ context.Context, _ string) ([]topology.Instance, error) {
	return nil, nil
}
func (m *mockPromClient) QueryDependencyStatus(_ context.Context, _ topology.QueryOptions) (map[topology.EdgeKey]string, error) {
	return nil, nil
}
func (m *mockPromClient) QueryDependencyStatusDetail(_ context.Context, _ topology.QueryOptions) (map[topology.EdgeKey]string, error) {
	return nil, nil
}
func (m *mockPromClient) QueryHistoricalAlerts(_ context.Context, _ time.Time) ([]topology.HistoricalAlert, error) {
	return nil, nil
}

func TestQueryStatusTransitions_DetectsTransitions(t *testing.T) {
	base := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	mock := &mockPromClient{
		statusRange: []topology.RangeResult{
			{
				Key:    topology.EdgeKey{Name: "svc-go", Host: "pg", Port: "5432"},
				Status: "ok",
				Values: []topology.TimeValue{
					{Timestamp: base, Value: 1},
					{Timestamp: base.Add(15 * time.Second), Value: 1},
					{Timestamp: base.Add(30 * time.Second), Value: 0}, // ok disappears
				},
			},
			{
				Key:    topology.EdgeKey{Name: "svc-go", Host: "pg", Port: "5432"},
				Status: "timeout",
				Values: []topology.TimeValue{
					{Timestamp: base, Value: 0},
					{Timestamp: base.Add(15 * time.Second), Value: 0},
					{Timestamp: base.Add(30 * time.Second), Value: 1}, // timeout appears
				},
			},
		},
	}

	req := EventsRequest{
		Start: base,
		End:   base.Add(time.Minute),
	}

	events, err := QueryStatusTransitions(context.Background(), mock, req)
	if err != nil {
		t.Fatalf("QueryStatusTransitions() error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	ev := events[0]
	if ev.Service != "svc-go" {
		t.Errorf("event.Service = %q, want svc-go", ev.Service)
	}
	if ev.FromState != "ok" {
		t.Errorf("event.FromState = %q, want ok", ev.FromState)
	}
	if ev.ToState != "timeout" {
		t.Errorf("event.ToState = %q, want timeout", ev.ToState)
	}
	if ev.Kind != "degradation" {
		t.Errorf("event.Kind = %q, want degradation", ev.Kind)
	}
}

func TestQueryStatusTransitions_NoTransitions(t *testing.T) {
	base := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	mock := &mockPromClient{
		statusRange: []topology.RangeResult{
			{
				Key:    topology.EdgeKey{Name: "svc-go", Host: "pg", Port: "5432"},
				Status: "ok",
				Values: []topology.TimeValue{
					{Timestamp: base, Value: 1},
					{Timestamp: base.Add(15 * time.Second), Value: 1},
					{Timestamp: base.Add(30 * time.Second), Value: 1},
				},
			},
		},
	}

	req := EventsRequest{
		Start: base,
		End:   base.Add(time.Minute),
	}

	events, err := QueryStatusTransitions(context.Background(), mock, req)
	if err != nil {
		t.Fatalf("QueryStatusTransitions() error: %v", err)
	}

	if len(events) != 0 {
		t.Errorf("got %d events, want 0 (no transitions)", len(events))
	}
}

func TestQueryStatusTransitions_EmptyRange(t *testing.T) {
	mock := &mockPromClient{
		statusRange: []topology.RangeResult{},
	}

	base := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	req := EventsRequest{
		Start: base,
		End:   base.Add(time.Minute),
	}

	events, err := QueryStatusTransitions(context.Background(), mock, req)
	if err != nil {
		t.Fatalf("QueryStatusTransitions() error: %v", err)
	}

	if len(events) != 0 {
		t.Errorf("got %d events, want 0", len(events))
	}
}

func TestQueryStatusTransitions_InvalidRange(t *testing.T) {
	mock := &mockPromClient{}

	base := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	req := EventsRequest{
		Start: base,
		End:   base.Add(-time.Minute), // end before start
	}

	_, err := QueryStatusTransitions(context.Background(), mock, req)
	if err == nil {
		t.Fatal("expected error for invalid range")
	}
}

func TestQueryStatusTransitions_Recovery(t *testing.T) {
	base := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	mock := &mockPromClient{
		statusRange: []topology.RangeResult{
			{
				Key:    topology.EdgeKey{Name: "svc-go", Host: "redis", Port: "6379"},
				Status: "timeout",
				Values: []topology.TimeValue{
					{Timestamp: base, Value: 1},
					{Timestamp: base.Add(30 * time.Second), Value: 0},
				},
			},
			{
				Key:    topology.EdgeKey{Name: "svc-go", Host: "redis", Port: "6379"},
				Status: "ok",
				Values: []topology.TimeValue{
					{Timestamp: base, Value: 0},
					{Timestamp: base.Add(30 * time.Second), Value: 1},
				},
			},
		},
	}

	req := EventsRequest{
		Start: base,
		End:   base.Add(time.Minute),
	}

	events, err := QueryStatusTransitions(context.Background(), mock, req)
	if err != nil {
		t.Fatalf("QueryStatusTransitions() error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	if events[0].Kind != "recovery" {
		t.Errorf("event.Kind = %q, want recovery", events[0].Kind)
	}
	if events[0].FromState != "timeout" || events[0].ToState != "ok" {
		t.Errorf("transition = %s→%s, want timeout→ok", events[0].FromState, events[0].ToState)
	}
}
