package topology

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"time"

	"github.com/BigKAA/dephealth-ui/internal/alerts"
	"github.com/BigKAA/dephealth-ui/internal/config"
)

// GrafanaConfig holds Grafana URL generation settings.
type GrafanaConfig struct {
	BaseURL               string
	ServiceStatusDashUID  string
	LinkStatusDashUID     string
	ServiceListDashUID    string
	ServicesStatusDashUID string
	LinksStatusDashUID    string
}

// depAlertKey maps alert labels (name + dependency name) to find the corresponding edge.
type depAlertKey struct {
	Name       string
	Dependency string
}

// GraphBuilder constructs a TopologyResponse from Prometheus and AlertManager data.
type GraphBuilder struct {
	prom           PrometheusClient
	am             alerts.AlertManagerClient
	grafana        GrafanaConfig
	ttl            time.Duration
	lookback       time.Duration
	logger         *slog.Logger
	severityLevels []config.SeverityLevel
}

// NewGraphBuilder creates a new GraphBuilder.
func NewGraphBuilder(prom PrometheusClient, am alerts.AlertManagerClient, grafana GrafanaConfig, ttl time.Duration, lookback time.Duration, logger *slog.Logger, severityLevels []config.SeverityLevel) *GraphBuilder {
	if logger == nil {
		logger = slog.Default()
	}
	return &GraphBuilder{
		prom:           prom,
		am:             am,
		grafana:        grafana,
		ttl:            ttl,
		lookback:       lookback,
		logger:         logger,
		severityLevels: severityLevels,
	}
}

// Build queries Prometheus and AlertManager, then constructs the full topology response.
// Only QueryTopologyEdges is fatal. Health, latency, and alert failures result in partial data.
func (b *GraphBuilder) Build(ctx context.Context, opts QueryOptions) (*TopologyResponse, error) {
	var rawEdges []TopologyEdge
	var err error

	if b.lookback > 0 {
		rawEdges, err = b.prom.QueryTopologyEdgesLookback(ctx, opts, b.lookback)
	} else {
		rawEdges, err = b.prom.QueryTopologyEdges(ctx, opts)
	}
	if err != nil {
		return nil, fmt.Errorf("querying topology edges: %w", err)
	}

	var queryErrors []string

	health, err := b.prom.QueryHealthState(ctx, opts)
	if err != nil {
		b.logger.Warn("failed to query health state, using defaults", "error", err)
		health = make(map[EdgeKey]float64)
		queryErrors = append(queryErrors, fmt.Sprintf("health state: %v", err))
	}

	avgLatency, err := b.prom.QueryAvgLatency(ctx, opts)
	if err != nil {
		b.logger.Warn("failed to query avg latency, using defaults", "error", err)
		avgLatency = make(map[EdgeKey]float64)
		queryErrors = append(queryErrors, fmt.Sprintf("avg latency: %v", err))
	}

	depStatus, err := b.prom.QueryDependencyStatus(ctx, opts)
	if err != nil {
		b.logger.Warn("failed to query dependency status, using defaults", "error", err)
		depStatus = make(map[EdgeKey]string)
		queryErrors = append(queryErrors, fmt.Sprintf("dependency status: %v", err))
	}

	depStatusDetail, err := b.prom.QueryDependencyStatusDetail(ctx, opts)
	if err != nil {
		b.logger.Warn("failed to query dependency status detail, using defaults", "error", err)
		depStatusDetail = make(map[EdgeKey]string)
		queryErrors = append(queryErrors, fmt.Sprintf("dependency status detail: %v", err))
	}

	// Fetch alerts: use historical ALERTS metric when in history mode,
	// otherwise fetch from AlertManager (live mode).
	var fetchedAlerts []alerts.Alert
	if opts.Time != nil {
		histAlerts, hErr := b.prom.QueryHistoricalAlerts(ctx, *opts.Time)
		if hErr != nil {
			b.logger.Warn("failed to query historical alerts", "error", hErr)
			queryErrors = append(queryErrors, fmt.Sprintf("historical alerts: %v", hErr))
		} else {
			fetchedAlerts = historicalToAlerts(histAlerts)
		}
	} else if b.am != nil {
		fetchedAlerts, err = b.am.FetchAlerts(ctx)
		if err != nil {
			b.logger.Warn("failed to fetch alerts from AlertManager", "error", err)
			queryErrors = append(queryErrors, fmt.Sprintf("alerts: %v", err))
		}
	}

	// When lookback is enabled, derive the set of currently live edge keys
	// from the health query results. Edges present in lookback topology but
	// absent from this set are stale (metrics disappeared).
	var currentEdgeKeys map[EdgeKey]bool
	if b.lookback > 0 {
		currentEdgeKeys = make(map[EdgeKey]bool, len(health))
		for k := range health {
			currentEdgeKeys[k] = true
		}
	}

	nodes, edges, depLookup := b.buildGraph(rawEdges, health, avgLatency, currentEdgeKeys, depStatus, depStatusDetail)

	alertInfos := b.enrichWithAlerts(nodes, edges, fetchedAlerts, depLookup)

	meta := TopologyMeta{
		CachedAt:  time.Now().UTC(),
		TTL:       int(b.ttl.Seconds()),
		NodeCount: len(nodes),
		EdgeCount: len(edges),
		Partial:   len(queryErrors) > 0,
		Errors:    queryErrors,
	}
	if opts.Time != nil {
		meta.Time = opts.Time
		meta.IsHistory = true
	}

	return &TopologyResponse{
		Nodes:  nodes,
		Edges:  edges,
		Alerts: alertInfos,
		Meta:   meta,
	}, nil
}

// buildGraph constructs nodes and edges from raw topology data.
// When currentEdgeKeys is non-nil (lookback mode), edges whose key is absent
// from the set are marked as stale with state "unknown".
func (b *GraphBuilder) buildGraph(
	rawEdges []TopologyEdge,
	health map[EdgeKey]float64,
	avgLatency map[EdgeKey]float64,
	currentEdgeKeys map[EdgeKey]bool,
	depStatus map[EdgeKey]string,
	depStatusDetail map[EdgeKey]string,
) ([]Node, []Edge, map[depAlertKey]EdgeKey) {
	// First pass: collect all known service names (sources that report metrics).
	serviceNames := make(map[string]bool)
	for _, e := range rawEdges {
		serviceNames[e.Name] = true
	}

	// Collect unique nodes (services = names, dependencies = host:port).
	type nodeInfo struct {
		typ       string
		namespace string
		host      string
		port      string
		deps      map[string]bool // for services: set of dependency endpoint IDs
	}
	nodeMap := make(map[string]*nodeInfo)

	// Track edge health per source node (outgoing, with critical flag) and per target node (incoming).
	nodeOutgoingHealth := make(map[string][]edgeHealthInfo)
	nodeIncomingHealth := make(map[string][]float64)

	// Build unique edges keyed by {Name, Host, Port}.
	edgeMap := make(map[EdgeKey]TopologyEdge)

	// Reverse lookup: (name, dependency_name) → EdgeKey for alert matching.
	depLookup := make(map[depAlertKey]EdgeKey)

	// resolveTarget returns the target node ID for a dependency edge.
	// If the dependency name matches a known service, link to that service node
	// to build a connected (through) graph. Otherwise, use host:port.
	resolveTarget := func(e TopologyEdge) string {
		if serviceNames[e.Dependency] {
			return e.Dependency
		}
		return e.Host + ":" + e.Port
	}

	for _, e := range rawEdges {
		key := EdgeKey{Name: e.Name, Host: e.Host, Port: e.Port}
		edgeMap[key] = e

		// Build reverse lookup for alerts.
		depLookup[depAlertKey{Name: e.Name, Dependency: e.Dependency}] = key

		depNodeID := resolveTarget(e)

		// Register source node (service).
		if _, ok := nodeMap[e.Name]; !ok {
			nodeMap[e.Name] = &nodeInfo{
				typ:       "service",
				namespace: e.Namespace,
				deps:      make(map[string]bool),
			}
		}
		nodeMap[e.Name].deps[depNodeID] = true

		// Register target node (dependency) — only if not a known service.
		if !serviceNames[e.Dependency] {
			if _, ok := nodeMap[depNodeID]; !ok {
				nodeMap[depNodeID] = &nodeInfo{
					typ:  e.Type,
					host: e.Host,
					port: e.Port,
					deps: make(map[string]bool),
				}
			}
		}
	}

	// Track stale edge counts per node for node-level stale determination.
	nodeStaleOutgoing := make(map[string]int)
	nodeTotalOutgoing := make(map[string]int)
	nodeStaleIncoming := make(map[string]int)
	nodeTotalIncoming := make(map[string]int)

	// Build edges.
	edges := make([]Edge, 0, len(edgeMap))
	for key, raw := range edgeMap {
		depNodeID := resolveTarget(raw)

		// Check if this edge is stale (lookback mode only).
		stale := currentEdgeKeys != nil && !currentEdgeKeys[key]

		if stale {
			edge := Edge{
				Source:     raw.Name,
				Target:     depNodeID,
				Type:       raw.Type,
				Latency:    "",
				LatencyRaw: 0,
				Health:     -1,
				State:      "unknown",
				Critical:   raw.Critical,
				Stale:      true,
				GrafanaURL: b.linkGrafanaURL(raw.Dependency, raw.Host, raw.Port),
			}
			edges = append(edges, edge)

			nodeStaleOutgoing[raw.Name]++
			nodeTotalOutgoing[raw.Name]++
			nodeStaleIncoming[depNodeID]++
			nodeTotalIncoming[depNodeID]++
			continue
		}

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
			Source:     raw.Name,
			Target:     depNodeID,
			Type:       raw.Type,
			Latency:    formatLatency(lat),
			LatencyRaw: lat,
			Health:     h,
			State:      state,
			Critical:   raw.Critical,
			GrafanaURL: b.linkGrafanaURL(raw.Dependency, raw.Host, raw.Port),
		}
		if s, ok := depStatus[key]; ok {
			edge.Status = s
		}
		if d, ok := depStatusDetail[key]; ok {
			edge.Detail = d
		}
		edges = append(edges, edge)

		nodeOutgoingHealth[raw.Name] = append(nodeOutgoingHealth[raw.Name], edgeHealthInfo{Health: h, Critical: raw.Critical})
		nodeIncomingHealth[depNodeID] = append(nodeIncomingHealth[depNodeID], h)
		nodeTotalOutgoing[raw.Name]++
		nodeTotalIncoming[depNodeID]++
	}

	// Build nodes.
	nodes := make([]Node, 0, len(nodeMap))
	for id, info := range nodeMap {
		var state string
		var stale bool

		if info.typ == "service" {
			// Service nodes: state from outgoing non-stale edges.
			allStale := nodeTotalOutgoing[id] > 0 && nodeStaleOutgoing[id] == nodeTotalOutgoing[id]
			if allStale {
				state = "down"
				stale = true
			} else {
				state = calcServiceNodeState(nodeOutgoingHealth[id])
			}
		} else {
			// Dependency nodes: state from incoming non-stale edges.
			allStale := nodeTotalIncoming[id] > 0 && nodeStaleIncoming[id] == nodeTotalIncoming[id]
			if allStale {
				state = "down"
				stale = true
			} else {
				state = calcNodeState(nodeIncomingHealth[id])
			}
		}

		node := Node{
			ID:              id,
			Label:           id,
			State:           state,
			Type:            info.typ,
			Namespace:       info.namespace,
			Host:            info.host,
			Port:            info.port,
			DependencyCount: len(info.deps),
			Stale:           stale,
		}

		if info.typ == "service" {
			node.GrafanaURL = b.serviceGrafanaURL(id)
		}

		// For dependency nodes, use host as label (cleaner than host:port).
		if info.typ != "service" && info.host != "" {
			node.Label = info.host
		}

		nodes = append(nodes, node)
	}

	return nodes, edges, depLookup
}

// edgeHealthInfo holds health and criticality for a single edge.
type edgeHealthInfo struct {
	Health   float64
	Critical bool
}

// calcNodeState determines a dependency node's state from its incoming edge health values.
// Used for non-service nodes (postgres, redis, etc.) where critical-aware logic does not apply.
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

// calcServiceNodeState determines a service node's state using critical-aware logic.
//
// Rules:
//   - No critical edges at all → always "ok" (non-critical failures don't affect state)
//   - Any critical edge down (health=0) → "down"
//   - All critical edges healthy, some non-critical down → "degraded"
//   - All edges healthy → "ok"
func calcServiceNodeState(edges []edgeHealthInfo) string {
	if len(edges) == 0 {
		return "unknown"
	}

	for _, e := range edges {
		if e.Health == 0 {
			return "degraded"
		}
	}

	return "ok"
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

func (b *GraphBuilder) serviceGrafanaURL(name string) string {
	if b.grafana.BaseURL == "" || b.grafana.ServiceStatusDashUID == "" {
		return ""
	}
	return fmt.Sprintf("%s/d/%s?var-service=%s",
		b.grafana.BaseURL, b.grafana.ServiceStatusDashUID, url.QueryEscape(name))
}

func (b *GraphBuilder) linkGrafanaURL(dependency, host, port string) string {
	if b.grafana.BaseURL == "" || b.grafana.LinkStatusDashUID == "" {
		return ""
	}
	return fmt.Sprintf("%s/d/%s?var-dependency=%s&var-host=%s&var-port=%s",
		b.grafana.BaseURL, b.grafana.LinkStatusDashUID,
		url.QueryEscape(dependency), url.QueryEscape(host), url.QueryEscape(port))
}

// enrichWithAlerts applies alert-based state overrides to edges and nodes,
// computes alertCount and alertSeverity for nodes and edges,
// and returns the list of topology-mapped AlertInfo entries.
func (b *GraphBuilder) enrichWithAlerts(nodes []Node, edges []Edge, fetched []alerts.Alert, depLookup map[depAlertKey]EdgeKey) []AlertInfo {
	if len(fetched) == 0 {
		return []AlertInfo{}
	}

	// Build severity priority index: value → priority (lower = more critical).
	severityPriority := make(map[string]int, len(b.severityLevels))
	for i, level := range b.severityLevels {
		severityPriority[level.Value] = i
	}

	// Build edge index: (source, target) → index in edges slice.
	type edgeRef struct {
		source, target string
	}
	edgeIdx := make(map[edgeRef]int, len(edges))
	for i, e := range edges {
		edgeIdx[edgeRef{e.Source, e.Target}] = i
	}

	// Track alert-based health overrides per source node.
	nodeAlertHealth := make(map[string][]float64)
	// Track alert counts and worst severity per node and edge.
	nodeAlertCounts := make(map[string]int)
	nodeWorstSeverity := make(map[string]int)  // node ID → best (lowest) severity priority
	edgeWorstSeverity := make(map[int]int)      // edge index → best (lowest) severity priority

	// Initialize worst severity to a value beyond all levels.
	const maxPriority = 999

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

		// Count alert per service node.
		nodeAlertCounts[a.Service]++

		// Track worst severity for the service node.
		if pri, ok := severityPriority[a.Severity]; ok {
			if cur, exists := nodeWorstSeverity[a.Service]; !exists || pri < cur {
				nodeWorstSeverity[a.Service] = pri
			}
		}

		// Translate alert labels (name, dependency_name) to edge via reverse lookup.
		alertKey := depAlertKey{Name: a.Service, Dependency: a.Dependency}
		ek, ok := depLookup[alertKey]
		if !ok {
			continue
		}

		// Try host:port target first, then dependency name (service-to-service edges).
		ref := edgeRef{source: a.Service, target: ek.Host + ":" + ek.Port}
		idx, ok := edgeIdx[ref]
		if !ok {
			ref = edgeRef{source: a.Service, target: a.Dependency}
			idx, ok = edgeIdx[ref]
			if !ok {
				continue
			}
		}

		// Track edge alert count and worst severity.
		edges[idx].AlertCount++
		if pri, ok := severityPriority[a.Severity]; ok {
			if cur, exists := edgeWorstSeverity[idx]; !exists || pri < cur {
				edgeWorstSeverity[idx] = pri
			}
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

	// Apply worst severity to edges.
	for idx, pri := range edgeWorstSeverity {
		if pri < len(b.severityLevels) {
			edges[idx].AlertSeverity = b.severityLevels[pri].Value
		}
	}

	// Build node index for applying alert counts and severity.
	nodeIdx := make(map[string]int, len(nodes))
	for i, n := range nodes {
		nodeIdx[n.ID] = i
	}

	// Apply alert counts and worst severity to nodes.
	for nodeID, count := range nodeAlertCounts {
		if idx, ok := nodeIdx[nodeID]; ok {
			nodes[idx].AlertCount = count
		}
	}
	for nodeID, pri := range nodeWorstSeverity {
		if idx, ok := nodeIdx[nodeID]; ok {
			if pri < len(b.severityLevels) {
				nodes[idx].AlertSeverity = b.severityLevels[pri].Value
			}
		}
	}

	// Recalculate node states for nodes affected by alerts.
	// Alert-affected nodes are always service nodes (alerts target services),
	// so we use critical-aware calcServiceNodeState.
	if len(nodeAlertHealth) > 0 {
		// Collect edge health+critical per source node (with alert overrides applied).
		nodeEdgeHealth := make(map[string][]edgeHealthInfo)
		for _, e := range edges {
			nodeEdgeHealth[e.Source] = append(nodeEdgeHealth[e.Source], edgeHealthInfo{Health: e.Health, Critical: e.Critical})
		}

		for nodeID := range nodeAlertHealth {
			if idx, ok := nodeIdx[nodeID]; ok {
				nodes[idx].State = calcServiceNodeState(nodeEdgeHealth[nodeID])
			}
		}
	}

	return alertInfos
}

// historicalToAlerts converts HistoricalAlert slice to alerts.Alert slice
// for compatibility with enrichWithAlerts.
func historicalToAlerts(hist []HistoricalAlert) []alerts.Alert {
	result := make([]alerts.Alert, 0, len(hist))
	for _, h := range hist {
		result = append(result, alerts.Alert{
			AlertName: h.AlertName,
			Service:   h.Service,
			Severity:  h.Severity,
			State:     "firing",
		})
	}
	return result
}

// QueryInstances returns all instances (pods/containers) for a given service.
func (b *GraphBuilder) QueryInstances(ctx context.Context, serviceName string) ([]Instance, error) {
	return b.prom.QueryInstances(ctx, serviceName)
}
