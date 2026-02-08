package topology

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"time"

	"github.com/BigKAA/dephealth-ui/internal/alerts"
)

// GrafanaConfig holds Grafana URL generation settings.
type GrafanaConfig struct {
	BaseURL              string
	ServiceStatusDashUID string
	LinkStatusDashUID    string
}

// GraphBuilder constructs a TopologyResponse from Prometheus and AlertManager data.
type GraphBuilder struct {
	prom    PrometheusClient
	am      alerts.AlertManagerClient
	grafana GrafanaConfig
	ttl     time.Duration
	logger  *slog.Logger
}

// NewGraphBuilder creates a new GraphBuilder.
func NewGraphBuilder(prom PrometheusClient, am alerts.AlertManagerClient, grafana GrafanaConfig, ttl time.Duration, logger *slog.Logger) *GraphBuilder {
	if logger == nil {
		logger = slog.Default()
	}
	return &GraphBuilder{
		prom:    prom,
		am:      am,
		grafana: grafana,
		ttl:     ttl,
		logger:  logger,
	}
}

// Build queries Prometheus and AlertManager, then constructs the full topology response.
func (b *GraphBuilder) Build(ctx context.Context) (*TopologyResponse, error) {
	rawEdges, err := b.prom.QueryTopologyEdges(ctx)
	if err != nil {
		return nil, fmt.Errorf("querying topology edges: %w", err)
	}

	health, err := b.prom.QueryHealthState(ctx)
	if err != nil {
		return nil, fmt.Errorf("querying health state: %w", err)
	}

	avgLatency, err := b.prom.QueryAvgLatency(ctx)
	if err != nil {
		return nil, fmt.Errorf("querying avg latency: %w", err)
	}

	// Fetch alerts (non-fatal: log and continue with empty alerts).
	var fetchedAlerts []alerts.Alert
	if b.am != nil {
		fetchedAlerts, err = b.am.FetchAlerts(ctx)
		if err != nil {
			b.logger.Warn("failed to fetch alerts from AlertManager", "error", err)
		}
	}

	nodes, edges := b.buildGraph(rawEdges, health, avgLatency)

	alertInfos := b.enrichWithAlerts(nodes, edges, fetchedAlerts)

	return &TopologyResponse{
		Nodes:  nodes,
		Edges:  edges,
		Alerts: alertInfos,
		Meta: TopologyMeta{
			CachedAt:  time.Now().UTC(),
			TTL:       int(b.ttl.Seconds()),
			NodeCount: len(nodes),
			EdgeCount: len(edges),
		},
	}, nil
}

func (b *GraphBuilder) buildGraph(
	rawEdges []TopologyEdge,
	health map[EdgeKey]float64,
	avgLatency map[EdgeKey]float64,
) ([]Node, []Edge) {
	// Collect unique nodes (services = jobs, dependencies = targets).
	type nodeInfo struct {
		typ  string
		deps map[string]bool // for services: set of dependency names
	}
	nodeMap := make(map[string]*nodeInfo)

	// Track edge health per source node for state calculation.
	nodeEdgeHealth := make(map[string][]float64)

	// Build unique edges keyed by {job, dependency}.
	edgeMap := make(map[EdgeKey]TopologyEdge)
	for _, e := range rawEdges {
		key := EdgeKey{Job: e.Job, Dependency: e.Dependency}
		edgeMap[key] = e

		// Register source node (service).
		if _, ok := nodeMap[e.Job]; !ok {
			nodeMap[e.Job] = &nodeInfo{typ: "service", deps: make(map[string]bool)}
		}
		nodeMap[e.Job].deps[e.Dependency] = true

		// Register target node (dependency).
		if _, ok := nodeMap[e.Dependency]; !ok {
			nodeMap[e.Dependency] = &nodeInfo{typ: e.Type, deps: make(map[string]bool)}
		}
	}

	// Build edges.
	edges := make([]Edge, 0, len(edgeMap))
	for key, raw := range edgeMap {
		h := float64(1)
		if v, ok := health[key]; ok {
			h = v
		}

		lat := float64(0)
		if v, ok := avgLatency[key]; ok && !math.IsNaN(v) && !math.IsInf(v, 0) {
			lat = v
		}

		state := "ok"
		if h == 0 {
			state = "down"
		}

		edge := Edge{
			Source:     raw.Job,
			Target:     raw.Dependency,
			Latency:    formatLatency(lat),
			LatencyRaw: lat,
			Health:     h,
			State:      state,
			GrafanaURL: b.linkGrafanaURL(raw.Job, raw.Dependency),
		}
		edges = append(edges, edge)

		nodeEdgeHealth[raw.Job] = append(nodeEdgeHealth[raw.Job], h)
	}

	// Build nodes.
	nodes := make([]Node, 0, len(nodeMap))
	for id, info := range nodeMap {
		state := calcNodeState(nodeEdgeHealth[id])
		nodes = append(nodes, Node{
			ID:              id,
			Label:           id,
			State:           state,
			Type:            info.typ,
			DependencyCount: len(info.deps),
			GrafanaURL:      b.serviceGrafanaURL(id),
		})
	}

	return nodes, edges
}

// calcNodeState determines a node's state from its outgoing edge health values.
func calcNodeState(healthValues []float64) string {
	if len(healthValues) == 0 {
		return "unknown"
	}

	allHealthy := true
	allDown := true
	for _, h := range healthValues {
		if h == 0 {
			allHealthy = false
		} else {
			allDown = false
		}
	}

	switch {
	case allHealthy:
		return "ok"
	case allDown:
		return "down"
	default:
		return "degraded"
	}
}

// formatLatency converts seconds to human-readable format.
func formatLatency(seconds float64) string {
	if seconds == 0 {
		return "0ms"
	}
	if seconds < 0.001 {
		return fmt.Sprintf("%.0fµs", seconds*1_000_000)
	}
	if seconds < 1 {
		return fmt.Sprintf("%.1fms", seconds*1000)
	}
	return fmt.Sprintf("%.2fs", seconds)
}

func (b *GraphBuilder) serviceGrafanaURL(job string) string {
	if b.grafana.BaseURL == "" || b.grafana.ServiceStatusDashUID == "" {
		return ""
	}
	return fmt.Sprintf("%s/d/%s?var-job=%s",
		b.grafana.BaseURL, b.grafana.ServiceStatusDashUID, url.QueryEscape(job))
}

func (b *GraphBuilder) linkGrafanaURL(job, dep string) string {
	if b.grafana.BaseURL == "" || b.grafana.LinkStatusDashUID == "" {
		return ""
	}
	return fmt.Sprintf("%s/d/%s?var-job=%s&var-dep=%s",
		b.grafana.BaseURL, b.grafana.LinkStatusDashUID,
		url.QueryEscape(job), url.QueryEscape(dep))
}

// enrichWithAlerts applies alert-based state overrides to edges and nodes,
// and returns the list of topology-mapped AlertInfo entries.
func (b *GraphBuilder) enrichWithAlerts(nodes []Node, edges []Edge, fetched []alerts.Alert) []AlertInfo {
	if len(fetched) == 0 {
		return []AlertInfo{}
	}

	// Build edge index for state overrides.
	edgeIdx := make(map[EdgeKey]int, len(edges))
	for i, e := range edges {
		edgeIdx[EdgeKey{Job: e.Source, Dependency: e.Target}] = i
	}

	// Track alert-based health overrides per source node.
	nodeAlertHealth := make(map[string][]float64)

	var alertInfos []AlertInfo
	for _, a := range fetched {
		alertInfos = append(alertInfos, AlertInfo{
			AlertName:  a.AlertName,
			Service:    a.Service,
			Dependency: a.Dependency,
			Severity:   a.Severity,
			State:      a.State,
			Since:      a.Since,
			Summary:    a.Summary,
		})

		key := EdgeKey{Job: a.Service, Dependency: a.Dependency}
		idx, ok := edgeIdx[key]
		if !ok {
			continue
		}

		// Alert-based state override (alerts are more authoritative).
		switch a.AlertName {
		case "DependencyDown":
			edges[idx].State = "down"
			edges[idx].Health = 0
			nodeAlertHealth[a.Service] = append(nodeAlertHealth[a.Service], 0)
		case "DependencyDegraded":
			if edges[idx].State != "down" {
				edges[idx].State = "degraded"
			}
			nodeAlertHealth[a.Service] = append(nodeAlertHealth[a.Service], 0.5)
		}
	}

	// Recalculate node states for nodes affected by alerts.
	if len(nodeAlertHealth) > 0 {
		nodeIdx := make(map[string]int, len(nodes))
		for i, n := range nodes {
			nodeIdx[n.ID] = i
		}

		// Collect all edge health values per node (with alert overrides applied).
		nodeHealth := make(map[string][]float64)
		for _, e := range edges {
			nodeHealth[e.Source] = append(nodeHealth[e.Source], e.Health)
		}

		for nodeID := range nodeAlertHealth {
			if idx, ok := nodeIdx[nodeID]; ok {
				nodes[idx].State = calcNodeState(nodeHealth[nodeID])
			}
		}
	}

	return alertInfos
}
