// Package cascade implements BFS-based cascade failure analysis
// for the service dependency topology graph.
package cascade

import "github.com/BigKAA/dephealth-ui/internal/topology"

// Options configures cascade analysis behavior.
type Options struct {
	MaxDepth  int    // 0 = unlimited (default)
	Namespace string // filter results by namespace (optional)
}

// RootCause represents a terminal failure point in the dependency chain.
type RootCause struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Type      string `json:"type"`
	Namespace string `json:"namespace"`
	State     string `json:"state"`
}

// AffectedService represents a service impacted by a cascade failure.
type AffectedService struct {
	Service    string   `json:"service"`
	Namespace  string   `json:"namespace"`
	DependsOn  string   `json:"dependsOn"`
	RootCauses []string `json:"rootCauses"`
}

// Failure represents a single failed dependency relationship.
type Failure struct {
	Service    string `json:"service"`
	Namespace  string `json:"namespace"`
	Dependency string `json:"dependency"`
	Type       string `json:"type"`
	Host       string `json:"host"`
	Port       string `json:"port"`
}

// CascadeChain represents a path from an affected service to a root cause.
type CascadeChain struct {
	AffectedService string   `json:"affectedService"`
	Namespace       string   `json:"namespace"`
	DependsOn       string   `json:"dependsOn"`
	Path            []string `json:"path"`
	Depth           int      `json:"depth"`
}

// Summary provides aggregate counts for the analysis result.
type Summary struct {
	TotalServices        int `json:"totalServices"`
	RootCauseCount       int `json:"rootCauseCount"`
	AffectedServiceCount int `json:"affectedServiceCount"`
	TotalFailureCount    int `json:"totalFailureCount"`
	MaxDepth             int `json:"maxDepth"`
}

// AnalysisResult is the complete output of cascade analysis.
type AnalysisResult struct {
	RootCauses       []RootCause       `json:"rootCauses"`
	AffectedServices []AffectedService `json:"affectedServices"`
	AllFailures      []Failure         `json:"allFailures"`
	CascadeChains    []CascadeChain    `json:"cascadeChains"`
	Summary          Summary           `json:"summary"`
}

// adjacency holds pre-built edge lookups.
type adjacency struct {
	// outgoing[nodeID] = edges going out from nodeID (nodeID is the source)
	outgoing map[string][]topology.Edge
	// incoming[nodeID] = edges coming in to nodeID (nodeID is the target)
	incoming map[string][]topology.Edge
}

func buildAdjacency(edges []topology.Edge) adjacency {
	adj := adjacency{
		outgoing: make(map[string][]topology.Edge),
		incoming: make(map[string][]topology.Edge),
	}
	for _, e := range edges {
		adj.outgoing[e.Source] = append(adj.outgoing[e.Source], e)
		adj.incoming[e.Target] = append(adj.incoming[e.Target], e)
	}
	return adj
}

// findRootCauses traces downstream from a down node through critical edges
// to find the actual unavailable dependencies (terminal failures).
// It also collects all nodes in the failure chain for cascade filtering.
func findRootCauses(downNodeID string, nodeMap map[string]topology.Node, adj adjacency, maxDepth int) (rootCauses []string, chainNodes map[string]bool) {
	chainNodes = map[string]bool{downNodeID: true}
	visited := map[string]bool{downNodeID: true}

	type queueItem struct {
		id    string
		depth int
	}
	queue := []queueItem{{id: downNodeID, depth: 0}}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, edge := range adj.outgoing[current.id] {
			if !edge.Critical {
				continue
			}
			targetID := edge.Target
			if visited[targetID] {
				continue
			}

			target, exists := nodeMap[targetID]
			if !exists {
				continue
			}
			if target.State != "down" && target.State != "unknown" {
				continue
			}

			visited[targetID] = true
			chainNodes[targetID] = true

			nextDepth := current.depth + 1
			if maxDepth > 0 && nextDepth >= maxDepth {
				// Reached max depth — treat as terminal.
				rootCauses = append(rootCauses, targetID)
				continue
			}

			// If target is a service that's also down, recurse deeper.
			if target.Type == "service" && target.State == "down" {
				queue = append(queue, queueItem{id: targetID, depth: nextDepth})
			} else {
				// Terminal root cause: unknown/stale node or non-service dependency.
				rootCauses = append(rootCauses, targetID)
			}
		}
	}

	// Fallback: if no downstream cause found, the down node itself is the cause.
	if len(rootCauses) == 0 {
		rootCauses = []string{downNodeID}
	}
	return rootCauses, chainNodes
}

// propagateUpstream does BFS upstream from a down node through critical edges
// to find all affected (non-down) services.
func propagateUpstream(downNodeID string, nodeMap map[string]topology.Node, adj adjacency, maxDepth int) map[string][]string {
	// affected[nodeID] = list of root causes that affect it
	affected := make(map[string][]string)

	rootCauses, _ := findRootCauses(downNodeID, nodeMap, adj, maxDepth)

	visited := map[string]bool{}
	queue := []string{downNodeID}

	for len(queue) > 0 {
		currentID := queue[0]
		queue = queue[1:]

		for _, edge := range adj.incoming[currentID] {
			if !edge.Critical {
				continue
			}

			sourceID := edge.Source
			if visited[sourceID] {
				continue
			}
			visited[sourceID] = true

			source, exists := nodeMap[sourceID]
			if !exists {
				continue
			}

			// Skip nodes that are themselves down — they are their own root cause.
			if source.State == "down" {
				continue
			}

			// Merge root causes.
			existing := affected[sourceID]
			seen := make(map[string]bool)
			for _, rc := range existing {
				seen[rc] = true
			}
			for _, rc := range rootCauses {
				if !seen[rc] {
					affected[sourceID] = append(affected[sourceID], rc)
				}
			}

			queue = append(queue, sourceID)
		}
	}

	return affected
}

// buildCascadeChains builds cascade chains by finding paths from each affected
// service to its root causes via BFS through critical edges.
// cascadeSet contains all node IDs that are part of the cascade (affected + down),
// allowing traversal through intermediate non-down nodes.
func buildCascadeChains(affectedNodeID string, rootCauseSet map[string]bool, cascadeSet map[string]bool, nodeMap map[string]topology.Node, adj adjacency, maxDepth int) []CascadeChain {
	node := nodeMap[affectedNodeID]
	var chains []CascadeChain

	// BFS to find paths to root causes.
	type pathItem struct {
		id   string
		path []string
	}

	queue := []pathItem{{id: affectedNodeID, path: []string{node.Label}}}
	visited := map[string]bool{affectedNodeID: true}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, edge := range adj.outgoing[current.id] {
			if !edge.Critical {
				continue
			}
			targetID := edge.Target
			if visited[targetID] {
				continue
			}

			target, exists := nodeMap[targetID]
			if !exists {
				continue
			}
			// Follow nodes that are part of the cascade (down/unknown/affected).
			if target.State != "down" && target.State != "unknown" && !cascadeSet[targetID] {
				continue
			}

			visited[targetID] = true
			newPath := make([]string, len(current.path)+1)
			copy(newPath, current.path)
			newPath[len(current.path)] = target.Label

			if maxDepth > 0 && len(newPath)-1 >= maxDepth {
				// Reached max depth — emit chain.
				chains = append(chains, CascadeChain{
					AffectedService: node.Label,
					Namespace:       node.Namespace,
					DependsOn:       target.Label,
					Path:            newPath,
					Depth:           len(newPath) - 1,
				})
				continue
			}

			if rootCauseSet[targetID] {
				chains = append(chains, CascadeChain{
					AffectedService: node.Label,
					Namespace:       node.Namespace,
					DependsOn:       target.Label,
					Path:            newPath,
					Depth:           len(newPath) - 1,
				})
			} else {
				queue = append(queue, pathItem{id: targetID, path: newPath})
			}
		}
	}

	return chains
}

// Analyze performs cascade failure analysis on the full topology.
func Analyze(nodes []topology.Node, edges []topology.Edge, opts Options) *AnalysisResult {
	nodeMap := make(map[string]topology.Node, len(nodes))
	for _, n := range nodes {
		nodeMap[n.ID] = n
	}

	adj := buildAdjacency(edges)

	// Collect all down nodes.
	var downNodes []topology.Node
	for _, n := range nodes {
		if n.State == "down" {
			downNodes = append(downNodes, n)
		}
	}

	// Track unique root causes and affected services.
	rootCauseSet := make(map[string]bool)
	allChainNodes := make(map[string]bool)
	// affectedMap[nodeID] = deduplicated root cause IDs
	affectedMap := make(map[string]map[string]bool)

	for _, dn := range downNodes {
		rcs, chain := findRootCauses(dn.ID, nodeMap, adj, opts.MaxDepth)
		for id := range chain {
			allChainNodes[id] = true
		}
		for _, rc := range rcs {
			rootCauseSet[rc] = true
		}

		upstream := propagateUpstream(dn.ID, nodeMap, adj, opts.MaxDepth)
		for nodeID, causes := range upstream {
			if affectedMap[nodeID] == nil {
				affectedMap[nodeID] = make(map[string]bool)
			}
			for _, rc := range causes {
				affectedMap[nodeID][rc] = true
			}
		}
	}

	// Build cascade set: all nodes that are part of the cascade (affected + down).
	// Used by buildCascadeChains to traverse through intermediate non-down nodes.
	cascadeSet := make(map[string]bool)
	for nodeID := range affectedMap {
		cascadeSet[nodeID] = true
	}
	for _, dn := range downNodes {
		cascadeSet[dn.ID] = true
	}

	// Build result.
	result := &AnalysisResult{}

	// Root causes.
	for rcID := range rootCauseSet {
		n := nodeMap[rcID]
		result.RootCauses = append(result.RootCauses, RootCause{
			ID:        n.ID,
			Label:     n.Label,
			Type:      n.Type,
			Namespace: n.Namespace,
			State:     n.State,
		})
	}

	// Affected services.
	for nodeID, rcSet := range affectedMap {
		n := nodeMap[nodeID]
		var rcList []string
		for rc := range rcSet {
			rcList = append(rcList, rc)
		}

		// Find the direct dependency in the cascade chain.
		dependsOn := ""
		for _, edge := range adj.outgoing[nodeID] {
			if !edge.Critical {
				continue
			}
			target := nodeMap[edge.Target]
			if target.State == "down" || target.State == "unknown" || cascadeSet[edge.Target] {
				dependsOn = target.Label
				break
			}
		}

		result.AffectedServices = append(result.AffectedServices, AffectedService{
			Service:    n.Label,
			Namespace:  n.Namespace,
			DependsOn:  dependsOn,
			RootCauses: rcList,
		})
	}

	// All failures: edges where the dependency is down/unknown.
	for _, edge := range edges {
		target, exists := nodeMap[edge.Target]
		if !exists {
			continue
		}
		if target.State != "down" && target.State != "unknown" {
			continue
		}
		source := nodeMap[edge.Source]

		result.AllFailures = append(result.AllFailures, Failure{
			Service:    source.Label,
			Namespace:  source.Namespace,
			Dependency: target.Label,
			Type:       edge.Type,
			Host:       target.Host,
			Port:       target.Port,
		})
	}

	// Cascade chains.
	for nodeID := range affectedMap {
		chains := buildCascadeChains(nodeID, rootCauseSet, cascadeSet, nodeMap, adj, opts.MaxDepth)
		result.CascadeChains = append(result.CascadeChains, chains...)
	}

	// Count total services.
	serviceCount := 0
	for _, n := range nodes {
		if n.Type == "service" {
			serviceCount++
		}
	}

	// Summary.
	maxDepth := 0
	for _, c := range result.CascadeChains {
		if c.Depth > maxDepth {
			maxDepth = c.Depth
		}
	}

	result.Summary = Summary{
		TotalServices:        serviceCount,
		RootCauseCount:       len(result.RootCauses),
		AffectedServiceCount: len(result.AffectedServices),
		TotalFailureCount:    len(result.AllFailures),
		MaxDepth:             maxDepth,
	}

	// Ensure non-nil slices for JSON serialization.
	if result.RootCauses == nil {
		result.RootCauses = []RootCause{}
	}
	if result.AffectedServices == nil {
		result.AffectedServices = []AffectedService{}
	}
	if result.AllFailures == nil {
		result.AllFailures = []Failure{}
	}
	if result.CascadeChains == nil {
		result.CascadeChains = []CascadeChain{}
	}

	// Apply namespace filter if specified.
	if opts.Namespace != "" {
		return filterByNamespace(result, opts.Namespace)
	}

	return result
}

// AnalyzeForService performs cascade analysis filtered to a specific service.
// Returns only root causes, failures, and chains relevant to the given service.
func AnalyzeForService(nodes []topology.Node, edges []topology.Edge, serviceName string, opts Options) *AnalysisResult {
	full := Analyze(nodes, edges, opts)

	filtered := &AnalysisResult{
		RootCauses:       []RootCause{},
		AffectedServices: []AffectedService{},
		AllFailures:      []Failure{},
		CascadeChains:    []CascadeChain{},
	}

	// Find root causes relevant to this service.
	relevantRootCauses := make(map[string]bool)

	for _, as := range full.AffectedServices {
		if as.Service == serviceName {
			filtered.AffectedServices = append(filtered.AffectedServices, as)
			for _, rc := range as.RootCauses {
				relevantRootCauses[rc] = true
			}
		}
	}

	// Check if the service itself is a root cause.
	for _, rc := range full.RootCauses {
		if rc.Label == serviceName || rc.ID == serviceName {
			relevantRootCauses[rc.ID] = true
		}
	}

	// Collect relevant root causes.
	for _, rc := range full.RootCauses {
		if relevantRootCauses[rc.ID] {
			filtered.RootCauses = append(filtered.RootCauses, rc)
		}
	}

	// Filter chains.
	for _, c := range full.CascadeChains {
		if c.AffectedService == serviceName {
			filtered.CascadeChains = append(filtered.CascadeChains, c)
		}
	}

	// Collect all nodes in the service's cascade paths.
	chainNodes := make(map[string]bool)
	for _, c := range filtered.CascadeChains {
		for _, label := range c.Path {
			chainNodes[label] = true
		}
	}

	// Filter failures: include failures from this service or from nodes in its cascade chain.
	for _, f := range full.AllFailures {
		if f.Service == serviceName || chainNodes[f.Service] {
			filtered.AllFailures = append(filtered.AllFailures, f)
		}
	}

	// Recompute summary.
	maxDepth := 0
	for _, c := range filtered.CascadeChains {
		if c.Depth > maxDepth {
			maxDepth = c.Depth
		}
	}
	filtered.Summary = Summary{
		TotalServices:        full.Summary.TotalServices,
		RootCauseCount:       len(filtered.RootCauses),
		AffectedServiceCount: len(filtered.AffectedServices),
		TotalFailureCount:    len(filtered.AllFailures),
		MaxDepth:             maxDepth,
	}

	// Apply namespace filter if specified.
	if opts.Namespace != "" {
		filtered = filterByNamespace(filtered, opts.Namespace)
	}

	return filtered
}

// filterByNamespace filters analysis results to a specific namespace.
func filterByNamespace(result *AnalysisResult, namespace string) *AnalysisResult {
	filtered := &AnalysisResult{
		RootCauses:       []RootCause{},
		AffectedServices: []AffectedService{},
		AllFailures:      []Failure{},
		CascadeChains:    []CascadeChain{},
	}

	for _, rc := range result.RootCauses {
		if rc.Namespace == namespace {
			filtered.RootCauses = append(filtered.RootCauses, rc)
		}
	}
	for _, as := range result.AffectedServices {
		if as.Namespace == namespace {
			filtered.AffectedServices = append(filtered.AffectedServices, as)
		}
	}
	for _, f := range result.AllFailures {
		if f.Namespace == namespace {
			filtered.AllFailures = append(filtered.AllFailures, f)
		}
	}
	for _, c := range result.CascadeChains {
		if c.Namespace == namespace {
			filtered.CascadeChains = append(filtered.CascadeChains, c)
		}
	}

	maxDepth := 0
	for _, c := range filtered.CascadeChains {
		if c.Depth > maxDepth {
			maxDepth = c.Depth
		}
	}
	filtered.Summary = Summary{
		TotalServices:        result.Summary.TotalServices,
		RootCauseCount:       len(filtered.RootCauses),
		AffectedServiceCount: len(filtered.AffectedServices),
		TotalFailureCount:    len(filtered.AllFailures),
		MaxDepth:             maxDepth,
	}

	return filtered
}
