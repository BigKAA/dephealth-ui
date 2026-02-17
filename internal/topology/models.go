package topology

import "time"

// Node represents a service node in the topology graph.
type Node struct {
	ID              string `json:"id"`
	Label           string `json:"label"`
	State           string `json:"state"`           // "ok", "degraded", "down", "unknown"
	Type            string `json:"type"`            // "service" or dependency type
	Namespace       string `json:"namespace"`
	Host            string `json:"host,omitempty"`
	Port            string `json:"port,omitempty"`
	DependencyCount int    `json:"dependencyCount"`
	Stale           bool   `json:"stale,omitempty"`
	GrafanaURL      string `json:"grafanaUrl,omitempty"`
	AlertCount      int    `json:"alertCount,omitempty"`
	AlertSeverity   string `json:"alertSeverity,omitempty"`
}

// Edge represents a directed dependency edge between two nodes.
type Edge struct {
	Source        string  `json:"source"`
	Target        string  `json:"target"`
	Type          string  `json:"type,omitempty"`  // grpc, http, postgres, redis, etc.
	Latency       string  `json:"latency"`         // human-readable "5.2ms"
	LatencyRaw    float64 `json:"latencyRaw"`
	Health        float64 `json:"health"`          // 0 or 1; -1 for stale
	State         string  `json:"state"`           // "ok", "degraded", "down", "unknown"
	Critical      bool    `json:"critical"`
	Status        string  `json:"status,omitempty"`  // SDK v0.4.1: ok, timeout, connection_error, dns_error, auth_error, tls_error, unhealthy, error
	Detail        string  `json:"detail,omitempty"`  // SDK v0.4.1: e.g. http_503, grpc_not_serving, connection_refused
	Stale         bool    `json:"stale,omitempty"`
	GrafanaURL    string  `json:"grafanaUrl,omitempty"`
	AlertCount    int     `json:"alertCount,omitempty"`
	AlertSeverity string  `json:"alertSeverity,omitempty"`
}

// AlertInfo represents an active alert associated with the topology.
type AlertInfo struct {
	AlertName  string `json:"alertname"`
	Service    string `json:"service"`
	Dependency string `json:"dependency"`
	Severity   string `json:"severity"`
	State      string `json:"state"`
	Since      string `json:"since"`
	Summary    string `json:"summary,omitempty"`
}

// TopologyMeta holds metadata about the topology response.
type TopologyMeta struct {
	CachedAt  time.Time  `json:"cachedAt"`
	TTL       int        `json:"ttl"`
	NodeCount int        `json:"nodeCount"`
	EdgeCount int        `json:"edgeCount"`
	Partial   bool       `json:"partial"`
	Errors    []string   `json:"errors,omitempty"`
	Time      *time.Time `json:"time,omitempty"`      // Historical timestamp when set.
	IsHistory bool       `json:"isHistory,omitempty"` // True when viewing historical data.
}

// HistoricalAlert represents an alert reconstructed from the ALERTS metric at a historical timestamp.
type HistoricalAlert struct {
	AlertName string
	Namespace string
	Service   string // "name" or "service" label
	Severity  string
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
	Name string
	Host string
	Port string
}

// TopologyEdge represents a raw edge discovered from Prometheus metrics.
type TopologyEdge struct {
	Name       string
	Namespace  string
	Dependency string
	Type       string
	Host       string
	Port       string
	Critical   bool
}

// QueryOptions holds optional parameters for topology queries.
type QueryOptions struct {
	Namespace string
	Time      *time.Time // Historical timestamp; nil means "now" (live mode).
}

// Instance represents a single instance (pod or container) of a service.
type Instance struct {
	Instance string `json:"instance"`           // Required: host:port or instance identifier
	Pod      string `json:"pod,omitempty"`      // Optional: Kubernetes pod name
	Job      string `json:"job,omitempty"`      // Optional: Prometheus job label
	Service  string `json:"service,omitempty"`  // Service name this instance belongs to
}
