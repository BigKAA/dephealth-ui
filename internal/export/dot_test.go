package export

import (
	"strings"
	"testing"
)

func TestExportDOT_BasicStructure(t *testing.T) {
	resp := sampleTopologyResponse()
	data := ConvertTopology(resp, "full", nil)
	b, err := ExportDOT(data, DOTOptions{})
	if err != nil {
		t.Fatalf("ExportDOT error: %v", err)
	}
	dot := string(b)

	if !strings.HasPrefix(dot, "digraph dephealth {") {
		t.Error("DOT output should start with 'digraph dephealth {'")
	}
	if !strings.HasSuffix(strings.TrimSpace(dot), "}") {
		t.Error("DOT output should end with '}'")
	}
}

func TestExportDOT_DefaultRankDir(t *testing.T) {
	resp := sampleTopologyResponse()
	data := ConvertTopology(resp, "full", nil)
	b, err := ExportDOT(data, DOTOptions{})
	if err != nil {
		t.Fatalf("ExportDOT error: %v", err)
	}

	if !strings.Contains(string(b), "rankdir=TB") {
		t.Error("default rankdir should be TB")
	}
}

func TestExportDOT_CustomRankDir(t *testing.T) {
	resp := sampleTopologyResponse()
	data := ConvertTopology(resp, "full", nil)
	b, err := ExportDOT(data, DOTOptions{RankDir: "LR"})
	if err != nil {
		t.Fatalf("ExportDOT error: %v", err)
	}

	if !strings.Contains(string(b), "rankdir=LR") {
		t.Error("rankdir should be LR")
	}
}

func TestExportDOT_Clusters(t *testing.T) {
	resp := sampleTopologyResponse()
	data := ConvertTopology(resp, "full", nil)
	b, err := ExportDOT(data, DOTOptions{})
	if err != nil {
		t.Fatalf("ExportDOT error: %v", err)
	}
	dot := string(b)

	// payments group should create a cluster.
	if !strings.Contains(dot, "subgraph cluster_payments") {
		t.Error("missing cluster_payments subgraph")
	}
	// infrastructure namespace should create a cluster.
	if !strings.Contains(dot, "subgraph cluster_infrastructure") {
		t.Error("missing cluster_infrastructure subgraph")
	}
}

func TestExportDOT_NodeColors(t *testing.T) {
	resp := sampleTopologyResponse()
	data := ConvertTopology(resp, "full", nil)
	b, err := ExportDOT(data, DOTOptions{})
	if err != nil {
		t.Fatalf("ExportDOT error: %v", err)
	}
	dot := string(b)

	// ok nodes should use green.
	if !strings.Contains(dot, `fillcolor="#d4edda"`) {
		t.Error("ok nodes should have green fillcolor")
	}
	// stale nodes should use gray.
	if !strings.Contains(dot, `fillcolor="#e2e3e5"`) {
		t.Error("stale nodes should have gray fillcolor")
	}
}

func TestExportDOT_EdgeColors(t *testing.T) {
	resp := sampleTopologyResponse()
	data := ConvertTopology(resp, "full", nil)
	b, err := ExportDOT(data, DOTOptions{})
	if err != nil {
		t.Fatalf("ExportDOT error: %v", err)
	}
	dot := string(b)

	// ok edge = green.
	if !strings.Contains(dot, `color="#28a745"`) {
		t.Error("ok edges should have green color")
	}
	// timeout edge = orange.
	if !strings.Contains(dot, `color="#fd7e14"`) {
		t.Error("timeout edges should have orange color")
	}
}

func TestExportDOT_CriticalEdges(t *testing.T) {
	resp := sampleTopologyResponse()
	data := ConvertTopology(resp, "full", nil)
	b, err := ExportDOT(data, DOTOptions{})
	if err != nil {
		t.Fatalf("ExportDOT error: %v", err)
	}
	dot := string(b)

	// postgres edge is critical -> style=bold.
	if !strings.Contains(dot, "style=bold") {
		t.Error("critical edges should have style=bold")
	}
}

func TestExportDOT_EdgeLabels(t *testing.T) {
	resp := sampleTopologyResponse()
	data := ConvertTopology(resp, "full", nil)
	b, err := ExportDOT(data, DOTOptions{})
	if err != nil {
		t.Fatalf("ExportDOT error: %v", err)
	}
	dot := string(b)

	if !strings.Contains(dot, `label="postgres"`) {
		t.Error("edge should have label=postgres")
	}
	if !strings.Contains(dot, `label="redis"`) {
		t.Error("edge should have label=redis")
	}
}

func TestExportDOT_SpecialCharacters(t *testing.T) {
	data := &ExportData{
		Nodes: []ExportNode{
			{ID: `node-with-"quotes"`, Namespace: "ns/special", State: "ok"},
		},
		Edges: nil,
	}
	b, err := ExportDOT(data, DOTOptions{})
	if err != nil {
		t.Fatalf("ExportDOT error: %v", err)
	}
	dot := string(b)

	// Quotes in node ID should be escaped.
	if !strings.Contains(dot, `\"quotes\"`) {
		t.Errorf("special characters not escaped properly in DOT: %s", dot)
	}
}
