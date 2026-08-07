package export

import (
	"encoding/json"
	"testing"

	"github.com/BigKAA/dephealth-ui/internal/topology"
)

func sampleTopologyResponse() *topology.TopologyResponse {
	return &topology.TopologyResponse{
		Nodes: []topology.Node{
			{
				ID:        "order-api",
				Label:     "order-api",
				Namespace: "payments",
				Group:     "payments",
				Type:      "service",
				State:     "ok",
			},
			{
				ID:        "postgres-main",
				Label:     "postgres-main",
				Namespace: "infrastructure",
				Type:      "postgres",
				State:     "ok",
				Host:      "pg-master.db.svc",
				Port:      "5432",
			},
			{
				ID:        "redis-cache",
				Label:     "redis-cache",
				Namespace: "infrastructure",
				Type:      "redis",
				State:     "ok",
				Host:      "redis.cache.svc",
				Port:      "6379",
				Stale:     true,
			},
		},
		Edges: []topology.Edge{
			{
				Source:     "order-api",
				Target:     "postgres-main",
				Type:       "postgres",
				Health:     1,
				LatencyRaw: 3.2,
				Critical:   true,
				Status:     "ok",
			},
			{
				Source:     "order-api",
				Target:     "redis-cache",
				Type:       "redis",
				Health:     -1,
				LatencyRaw: 0,
				Critical:   false,
				Status:     "timeout",
				Detail:     "connection_refused",
				Stale:      true,
			},
		},
	}
}

func TestExportJSON_ValidSchema(t *testing.T) {
	resp := sampleTopologyResponse()
	data := ConvertTopology(resp, "full", nil)
	b, err := ExportJSON(data)
	if err != nil {
		t.Fatalf("ExportJSON error: %v", err)
	}

	// Verify it's valid JSON by round-tripping.
	var parsed ExportData
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	if parsed.Version != "1.0" {
		t.Errorf("version = %q, want %q", parsed.Version, "1.0")
	}
	if parsed.Scope != "full" {
		t.Errorf("scope = %q, want %q", parsed.Scope, "full")
	}
	if len(parsed.Nodes) != 3 {
		t.Errorf("nodes count = %d, want 3", len(parsed.Nodes))
	}
	if len(parsed.Edges) != 2 {
		t.Errorf("edges count = %d, want 2", len(parsed.Edges))
	}
}

func TestExportJSON_NodeFields(t *testing.T) {
	resp := sampleTopologyResponse()
	data := ConvertTopology(resp, "full", nil)
	b, err := ExportJSON(data)
	if err != nil {
		t.Fatalf("ExportJSON error: %v", err)
	}

	var parsed ExportData
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	node := parsed.Nodes[0]
	if node.ID != "order-api" {
		t.Errorf("node.id = %q, want %q", node.ID, "order-api")
	}
	if node.Namespace != "payments" {
		t.Errorf("node.namespace = %q, want %q", node.Namespace, "payments")
	}
	if node.Group != "payments" {
		t.Errorf("node.group = %q, want %q", node.Group, "payments")
	}
	if node.Type != "service" {
		t.Errorf("node.type = %q, want %q", node.Type, "service")
	}
	if node.State != "ok" {
		t.Errorf("node.state = %q, want %q", node.State, "ok")
	}
}

func TestExportJSON_EdgeFields(t *testing.T) {
	resp := sampleTopologyResponse()
	data := ConvertTopology(resp, "full", nil)
	b, err := ExportJSON(data)
	if err != nil {
		t.Fatalf("ExportJSON error: %v", err)
	}

	var parsed ExportData
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	edge := parsed.Edges[0]
	if edge.Source != "order-api" {
		t.Errorf("edge.source = %q, want %q", edge.Source, "order-api")
	}
	if edge.Target != "postgres-main" {
		t.Errorf("edge.target = %q, want %q", edge.Target, "postgres-main")
	}
	if edge.Host != "pg-master.db.svc" {
		t.Errorf("edge.host = %q, want %q", edge.Host, "pg-master.db.svc")
	}
	if edge.Port != "5432" {
		t.Errorf("edge.port = %q, want %q", edge.Port, "5432")
	}
	if !edge.Critical {
		t.Error("edge.critical = false, want true")
	}
	if edge.Health != 1 {
		t.Errorf("edge.health = %f, want 1", edge.Health)
	}
	if edge.Status != "ok" {
		t.Errorf("edge.status = %q, want %q", edge.Status, "ok")
	}
	if edge.LatencyMs != 3.2 {
		t.Errorf("edge.latency_ms = %f, want 3.2", edge.LatencyMs)
	}
}

func TestExportJSON_StaleNodeState(t *testing.T) {
	resp := sampleTopologyResponse()
	data := ConvertTopology(resp, "full", nil)

	// redis-cache is stale.
	redisNode := data.Nodes[2]
	if redisNode.State != "stale" {
		t.Errorf("stale node state = %q, want %q", redisNode.State, "stale")
	}
}

func TestExportJSON_Filters(t *testing.T) {
	resp := sampleTopologyResponse()
	filters := map[string]string{"namespace": "payments"}
	data := ConvertTopology(resp, "current", filters)
	b, err := ExportJSON(data)
	if err != nil {
		t.Fatalf("ExportJSON error: %v", err)
	}

	var parsed ExportData
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	if parsed.Scope != "current" {
		t.Errorf("scope = %q, want %q", parsed.Scope, "current")
	}
	if parsed.Filters["namespace"] != "payments" {
		t.Errorf("filters.namespace = %q, want %q", parsed.Filters["namespace"], "payments")
	}
}
