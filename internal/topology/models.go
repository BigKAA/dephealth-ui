package topology

import "time"

// Node represents a service node in the topology graph.
type Node struct {
	ID              string `json:"id"`
	Label           string `json:"label"`
	State           string `json:"state"`           // "ok", "degraded", "down", "unknown"
	Type            string `json:"type"`            // "service" or dependency type
	DependencyCount int    `json:"dependencyCount"`
	GrafanaURL      string `json:"grafanaUrl,omitempty"`
}

// Edge represents a directed dependency edge between two nodes.
type Edge struct {
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	Latency    string  `json:"latency"`       // human-readable "5.2ms"
	LatencyRaw float64 `json:"latencyRaw"`
	Health     float64 `json:"health"`        // 0 or 1
	State      string  `json:"state"`         // "ok", "degraded", "down"
	GrafanaURL string  `json:"grafanaUrl,omitempty"`
}

// AlertInfo represents an active alert associated with the topology.
type AlertInfo struct {
	Service   string `json:"service"`
	AlertName string `json:"alertname"`
	Severity  string `json:"severity"`
	Since     string `json:"since"`
}

// TopologyMeta holds metadata about the topology response.
type TopologyMeta struct {
	CachedAt  time.Time `json:"cachedAt"`
	TTL       int       `json:"ttl"`
	NodeCount int       `json:"nodeCount"`
	EdgeCount int       `json:"edgeCount"`
}

// TopologyResponse is the complete topology API response.
type TopologyResponse struct {
	Nodes  []Node       `json:"nodes"`
	Edges  []Edge       `json:"edges"`
	Alerts []AlertInfo  `json:"alerts"`
	Meta   TopologyMeta `json:"meta"`
}

// EdgeKey uniquely identifies an edge in the topology.
type EdgeKey struct {
	Job        string
	Dependency string
}

// TopologyEdge represents a raw edge discovered from Prometheus metrics.
type TopologyEdge struct {
	Job        string
	Dependency string
	Type       string
	Host       string
	Port       string
}
