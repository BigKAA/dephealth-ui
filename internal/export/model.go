package export

import (
	"fmt"
	"time"

	"github.com/BigKAA/dephealth-ui/internal/topology"
)

// ExportData is the top-level export structure containing graph nodes, edges, and metadata.
type ExportData struct {
	Version   string            `json:"version"`
	Timestamp string            `json:"timestamp"`
	Scope     string            `json:"scope"`
	Filters   map[string]string `json:"filters"`
	Nodes     []ExportNode      `json:"nodes"`
	Edges     []ExportEdge      `json:"edges"`
}

// ExportNode is a simplified node representation for export.
type ExportNode struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Group     string `json:"group,omitempty"`
	Type      string `json:"type"`
	State     string `json:"state"`
	Alerts    int    `json:"alerts"`
}

// ExportEdge is a simplified edge representation for export.
type ExportEdge struct {
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	Dependency string  `json:"dependency"`
	Type       string  `json:"type"`
	Host       string  `json:"host"`
	Port       string  `json:"port"`
	Critical   bool    `json:"critical"`
	Health     float64 `json:"health"`
	Status     string  `json:"status"`
	Detail     string  `json:"detail"`
	LatencyMs  float64 `json:"latency_ms"`
}

// ConvertTopology converts a TopologyResponse into an ExportData structure.
func ConvertTopology(resp *topology.TopologyResponse, scope string, filters map[string]string) *ExportData {
	if filters == nil {
		filters = map[string]string{}
	}

	// Build node lookup for host/port enrichment.
	nodeMap := make(map[string]topology.Node, len(resp.Nodes))
	for _, n := range resp.Nodes {
		nodeMap[n.ID] = n
	}

	nodes := make([]ExportNode, 0, len(resp.Nodes))
	for _, n := range resp.Nodes {
		nodes = append(nodes, ExportNode{
			ID:        n.ID,
			Name:      n.Label,
			Namespace: n.Namespace,
			Group:     n.Group,
			Type:      n.Type,
			State:     nodeState(n),
			Alerts:    n.AlertCount,
		})
	}

	edges := make([]ExportEdge, 0, len(resp.Edges))
	for _, e := range resp.Edges {
		host, port := targetHostPort(e, nodeMap)
		edges = append(edges, ExportEdge{
			Source:     e.Source,
			Target:     e.Target,
			Dependency: e.Target,
			Type:       e.Type,
			Host:       host,
			Port:       port,
			Critical:   e.Critical,
			Health:     e.Health,
			Status:     e.Status,
			Detail:     e.Detail,
			LatencyMs:  e.LatencyRaw,
		})
	}

	return &ExportData{
		Version:   "1.0",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Scope:     scope,
		Filters:   filters,
		Nodes:     nodes,
		Edges:     edges,
	}
}

// nodeState returns a unified state string for a node.
func nodeState(n topology.Node) string {
	if n.Stale {
		return "stale"
	}
	if n.State != "" {
		return n.State
	}
	return "unknown"
}

// targetHostPort resolves host and port from the target node.
func targetHostPort(e topology.Edge, nodeMap map[string]topology.Node) (string, string) {
	if target, ok := nodeMap[e.Target]; ok {
		return target.Host, target.Port
	}
	return "", ""
}

// ExportFilename generates a filename for an exported file.
func ExportFilename(format string) string {
	ts := time.Now().UTC().Format("20060102-150405")
	return fmt.Sprintf("dephealth-topology-%s.%s", ts, format)
}
