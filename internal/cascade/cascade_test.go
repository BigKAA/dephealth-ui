package cascade

import (
	"fmt"
	"testing"

	"github.com/BigKAA/dephealth-ui/internal/topology"
)

// helper to create a node.
func node(id, state, typ, ns string) topology.Node {
	return topology.Node{
		ID:        id,
		Label:     id,
		State:     state,
		Type:      typ,
		Namespace: ns,
	}
}

// helper to create a critical edge.
func critEdge(source, target, edgeType string) topology.Edge {
	return topology.Edge{
		Source:   source,
		Target:   target,
		Type:     edgeType,
		Critical: true,
	}
}

// helper to create a non-critical edge.
func nonCritEdge(source, target, edgeType string) topology.Edge {
	return topology.Edge{
		Source:   source,
		Target:   target,
		Type:     edgeType,
		Critical: false,
	}
}

func TestAnalyze_LinearChain(t *testing.T) {
	// A(ok) → B(down/service) → C(down/dependency)
	// Root cause: C. Affected: A (depends on B which depends on C).
	nodes := []topology.Node{
		node("A", "ok", "service", "ns1"),
		node("B", "down", "service", "ns1"),
		node("C", "down", "dependency", "ns1"),
	}
	edges := []topology.Edge{
		critEdge("A", "B", "http"),
		critEdge("B", "C", "postgres"),
	}

	result := Analyze(nodes, edges, Options{})

	if result.Summary.RootCauseCount != 1 {
		t.Errorf("expected 1 root cause, got %d", result.Summary.RootCauseCount)
	}
	if len(result.RootCauses) != 1 || result.RootCauses[0].ID != "C" {
		t.Errorf("expected root cause C, got %v", result.RootCauses)
	}
	if result.Summary.AffectedServiceCount != 1 {
		t.Errorf("expected 1 affected service, got %d", result.Summary.AffectedServiceCount)
	}
	if len(result.AffectedServices) != 1 || result.AffectedServices[0].Service != "A" {
		t.Errorf("expected affected service A, got %v", result.AffectedServices)
	}
}

func TestAnalyze_DiamondDependency(t *testing.T) {
	// A(ok) → B(down/service) → D(down/dep)
	// A(ok) → C(down/service) → D(down/dep)
	// Root cause: D. Affected: A.
	nodes := []topology.Node{
		node("A", "ok", "service", "ns1"),
		node("B", "down", "service", "ns1"),
		node("C", "down", "service", "ns1"),
		node("D", "down", "dependency", "ns1"),
	}
	edges := []topology.Edge{
		critEdge("A", "B", "http"),
		critEdge("A", "C", "http"),
		critEdge("B", "D", "postgres"),
		critEdge("C", "D", "postgres"),
	}

	result := Analyze(nodes, edges, Options{})

	if result.Summary.RootCauseCount != 1 {
		t.Errorf("expected 1 root cause, got %d", result.Summary.RootCauseCount)
	}
	if len(result.RootCauses) != 1 || result.RootCauses[0].ID != "D" {
		t.Errorf("expected root cause D, got %v", result.RootCauses)
	}
	// A should be affected via both paths.
	if result.Summary.AffectedServiceCount != 1 {
		t.Errorf("expected 1 affected service, got %d", result.Summary.AffectedServiceCount)
	}
}

func TestAnalyze_NoFailures(t *testing.T) {
	nodes := []topology.Node{
		node("A", "ok", "service", "ns1"),
		node("B", "ok", "service", "ns1"),
	}
	edges := []topology.Edge{
		critEdge("A", "B", "http"),
	}

	result := Analyze(nodes, edges, Options{})

	if result.Summary.RootCauseCount != 0 {
		t.Errorf("expected 0 root causes, got %d", result.Summary.RootCauseCount)
	}
	if result.Summary.AffectedServiceCount != 0 {
		t.Errorf("expected 0 affected services, got %d", result.Summary.AffectedServiceCount)
	}
	if result.Summary.TotalFailureCount != 0 {
		t.Errorf("expected 0 failures, got %d", result.Summary.TotalFailureCount)
	}
	if len(result.RootCauses) != 0 {
		t.Errorf("expected empty root causes slice, got %v", result.RootCauses)
	}
}

func TestAnalyze_SelfContainedDownNode(t *testing.T) {
	// A(down) with no downstream critical edges — itself is root cause.
	nodes := []topology.Node{
		node("A", "down", "service", "ns1"),
	}

	result := Analyze(nodes, nil, Options{})

	if result.Summary.RootCauseCount != 1 {
		t.Errorf("expected 1 root cause, got %d", result.Summary.RootCauseCount)
	}
	if result.RootCauses[0].ID != "A" {
		t.Errorf("expected root cause A, got %v", result.RootCauses)
	}
}

func TestAnalyze_NonCriticalEdgesIgnored(t *testing.T) {
	// A(ok) → B(down) via non-critical edge.
	// No cascade should propagate.
	nodes := []topology.Node{
		node("A", "ok", "service", "ns1"),
		node("B", "down", "dependency", "ns1"),
	}
	edges := []topology.Edge{
		nonCritEdge("A", "B", "redis"),
	}

	result := Analyze(nodes, edges, Options{})

	// B is down (root cause of itself) but A is NOT affected (non-critical edge).
	if result.Summary.RootCauseCount != 1 {
		t.Errorf("expected 1 root cause, got %d", result.Summary.RootCauseCount)
	}
	if result.Summary.AffectedServiceCount != 0 {
		t.Errorf("expected 0 affected services, got %d", result.Summary.AffectedServiceCount)
	}
}

func TestAnalyze_CycleProtection(t *testing.T) {
	// A(down) → B(down) → A(down) — cycle.
	nodes := []topology.Node{
		node("A", "down", "service", "ns1"),
		node("B", "down", "service", "ns1"),
	}
	edges := []topology.Edge{
		critEdge("A", "B", "http"),
		critEdge("B", "A", "http"),
	}

	// Should not hang.
	result := Analyze(nodes, edges, Options{})

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// Both are down in a cycle; algorithm should still return results.
	if result.Summary.RootCauseCount == 0 {
		t.Error("expected at least 1 root cause in cycle scenario")
	}
}

func TestAnalyze_CrossServiceCascade(t *testing.T) {
	// svc-A(ok) → svc-B(down/service) → db(down/dependency)
	// svc-B depends on db, svc-A depends on svc-B.
	// Connected-graph scenario: service-to-service dependency.
	nodes := []topology.Node{
		node("svc-A", "ok", "service", "ns1"),
		node("svc-B", "down", "service", "ns2"),
		node("db", "down", "dependency", "ns2"),
	}
	edges := []topology.Edge{
		critEdge("svc-A", "svc-B", "http"),
		critEdge("svc-B", "db", "postgres"),
	}

	result := Analyze(nodes, edges, Options{})

	if result.Summary.RootCauseCount != 1 {
		t.Errorf("expected 1 root cause, got %d", result.Summary.RootCauseCount)
	}
	if result.RootCauses[0].ID != "db" {
		t.Errorf("expected root cause db, got %s", result.RootCauses[0].ID)
	}
	if result.Summary.AffectedServiceCount != 1 {
		t.Errorf("expected 1 affected service, got %d", result.Summary.AffectedServiceCount)
	}
	if result.AffectedServices[0].Service != "svc-A" {
		t.Errorf("expected affected service svc-A, got %s", result.AffectedServices[0].Service)
	}
}

func TestAnalyze_MixedStates(t *testing.T) {
	// A(ok) → B(unknown) — B is root cause.
	nodes := []topology.Node{
		node("A", "ok", "service", "ns1"),
		node("B", "unknown", "dependency", "ns1"),
	}
	edges := []topology.Edge{
		critEdge("A", "B", "http"),
	}

	result := Analyze(nodes, edges, Options{})

	// B is unknown (not down), so it's a failure but not a cascade trigger.
	// The algorithm only starts from down nodes, so no cascade here.
	if result.Summary.RootCauseCount != 0 {
		t.Errorf("expected 0 root causes (unknown only, no down), got %d", result.Summary.RootCauseCount)
	}
}

func TestAnalyze_DownToUnknown(t *testing.T) {
	// A(ok) → B(down/service) → C(unknown/dep)
	// B is down, traces to C(unknown) — C is root cause.
	nodes := []topology.Node{
		node("A", "ok", "service", "ns1"),
		node("B", "down", "service", "ns1"),
		node("C", "unknown", "dependency", "ns1"),
	}
	edges := []topology.Edge{
		critEdge("A", "B", "http"),
		critEdge("B", "C", "postgres"),
	}

	result := Analyze(nodes, edges, Options{})

	if result.Summary.RootCauseCount != 1 {
		t.Errorf("expected 1 root cause, got %d", result.Summary.RootCauseCount)
	}
	if result.RootCauses[0].ID != "C" {
		t.Errorf("expected root cause C, got %s", result.RootCauses[0].ID)
	}
}

func TestAnalyze_MaxDepth(t *testing.T) {
	// Chain: A(ok) → B(down) → C(down) → D(down/dep)
	// With MaxDepth=1, tracing from B should stop at C (not reach D).
	nodes := []topology.Node{
		node("A", "ok", "service", "ns1"),
		node("B", "down", "service", "ns1"),
		node("C", "down", "service", "ns1"),
		node("D", "down", "dependency", "ns1"),
	}
	edges := []topology.Edge{
		critEdge("A", "B", "http"),
		critEdge("B", "C", "http"),
		critEdge("C", "D", "postgres"),
	}

	result := Analyze(nodes, edges, Options{MaxDepth: 1})

	// With depth=1, B→C is found, but C→D is not traversed.
	// C is treated as root cause (depth limit hit), D also found from C's own tracing.
	if result.Summary.RootCauseCount == 0 {
		t.Error("expected at least 1 root cause with max depth")
	}
}

func TestAnalyze_CascadeChains(t *testing.T) {
	// A(ok) → B(down/service) → C(down/dep)
	// Chain: A → B → C
	nodes := []topology.Node{
		node("A", "ok", "service", "ns1"),
		node("B", "down", "service", "ns1"),
		node("C", "down", "dependency", "ns1"),
	}
	edges := []topology.Edge{
		critEdge("A", "B", "http"),
		critEdge("B", "C", "postgres"),
	}

	result := Analyze(nodes, edges, Options{})

	if len(result.CascadeChains) == 0 {
		t.Fatal("expected at least 1 cascade chain")
	}

	chain := result.CascadeChains[0]
	if chain.AffectedService != "A" {
		t.Errorf("expected chain from A, got %s", chain.AffectedService)
	}
	if chain.Depth < 1 {
		t.Errorf("expected depth >= 1, got %d", chain.Depth)
	}
}

func TestAnalyze_AllFailures(t *testing.T) {
	// A → B(down), A → C(ok)
	nodes := []topology.Node{
		node("A", "ok", "service", "ns1"),
		node("B", "down", "dependency", "ns1"),
		node("C", "ok", "dependency", "ns1"),
	}
	edges := []topology.Edge{
		critEdge("A", "B", "postgres"),
		critEdge("A", "C", "redis"),
	}

	result := Analyze(nodes, edges, Options{})

	if result.Summary.TotalFailureCount != 1 {
		t.Errorf("expected 1 failure (only A→B), got %d", result.Summary.TotalFailureCount)
	}
	if result.AllFailures[0].Dependency != "B" {
		t.Errorf("expected failed dependency B, got %s", result.AllFailures[0].Dependency)
	}
}

func TestAnalyzeForService(t *testing.T) {
	// A(ok) → B(down) → D(down/dep)
	// C(ok) → D(down/dep)
	// AnalyzeForService("A") should only return A's cascades.
	nodes := []topology.Node{
		node("A", "ok", "service", "ns1"),
		node("B", "down", "service", "ns1"),
		node("C", "ok", "service", "ns2"),
		node("D", "down", "dependency", "ns1"),
	}
	edges := []topology.Edge{
		critEdge("A", "B", "http"),
		critEdge("B", "D", "postgres"),
		critEdge("C", "D", "postgres"),
	}

	result := AnalyzeForService(nodes, edges, "A", Options{})

	if result.Summary.AffectedServiceCount != 1 {
		t.Errorf("expected 1 affected service, got %d", result.Summary.AffectedServiceCount)
	}
	for _, as := range result.AffectedServices {
		if as.Service != "A" {
			t.Errorf("unexpected affected service %s", as.Service)
		}
	}
}

func TestAnalyze_NamespaceFilter(t *testing.T) {
	nodes := []topology.Node{
		node("A", "ok", "service", "ns1"),
		node("B", "down", "dependency", "ns1"),
		node("C", "ok", "service", "ns2"),
		node("D", "down", "dependency", "ns2"),
	}
	edges := []topology.Edge{
		critEdge("A", "B", "http"),
		critEdge("C", "D", "http"),
	}

	result := AnalyzeForService(nodes, edges, "A", Options{Namespace: "ns1"})

	// Should only include ns1 results.
	for _, f := range result.AllFailures {
		if f.Namespace != "ns1" {
			t.Errorf("expected namespace ns1, got %s", f.Namespace)
		}
	}
}

func TestAnalyze_EmptyGraph(t *testing.T) {
	result := Analyze(nil, nil, Options{})

	if result.Summary.RootCauseCount != 0 {
		t.Errorf("expected 0 root causes, got %d", result.Summary.RootCauseCount)
	}
	// Ensure non-nil slices.
	if result.RootCauses == nil {
		t.Error("expected non-nil RootCauses slice")
	}
	if result.AllFailures == nil {
		t.Error("expected non-nil AllFailures slice")
	}
}

func TestAnalyze_LargeGraph(t *testing.T) {
	// Build a chain of 300 nodes: svc-0(ok) → svc-1(ok) → ... → svc-299(down)
	const n = 300
	nodes := make([]topology.Node, n)
	edges := make([]topology.Edge, n-1)

	for i := range n {
		state := "ok"
		if i == n-1 {
			state = "down"
		}
		nodes[i] = node(fmt.Sprintf("svc-%d", i), state, "service", "ns1")
	}
	// Make last node a dependency type to be a terminal root cause.
	nodes[n-1].Type = "dependency"

	for i := range n - 1 {
		edges[i] = critEdge(fmt.Sprintf("svc-%d", i), fmt.Sprintf("svc-%d", i+1), "http")
	}

	result := Analyze(nodes, edges, Options{})

	if result.Summary.RootCauseCount != 1 {
		t.Errorf("expected 1 root cause, got %d", result.Summary.RootCauseCount)
	}
	if result.RootCauses[0].ID != fmt.Sprintf("svc-%d", n-1) {
		t.Errorf("expected root cause svc-%d, got %s", n-1, result.RootCauses[0].ID)
	}
}

func TestAnalyze_MultipleRootCauses(t *testing.T) {
	// A(ok) → B(down/service) → C(down/dep)
	// A(ok) → D(down/dep)
	// Root causes: C and D.
	nodes := []topology.Node{
		node("A", "ok", "service", "ns1"),
		node("B", "down", "service", "ns1"),
		node("C", "down", "dependency", "ns1"),
		node("D", "down", "dependency", "ns1"),
	}
	edges := []topology.Edge{
		critEdge("A", "B", "http"),
		critEdge("B", "C", "postgres"),
		critEdge("A", "D", "redis"),
	}

	result := Analyze(nodes, edges, Options{})

	if result.Summary.RootCauseCount != 2 {
		t.Errorf("expected 2 root causes, got %d", result.Summary.RootCauseCount)
	}

	rcIDs := map[string]bool{}
	for _, rc := range result.RootCauses {
		rcIDs[rc.ID] = true
	}
	if !rcIDs["C"] || !rcIDs["D"] {
		t.Errorf("expected root causes C and D, got %v", result.RootCauses)
	}
}

func TestAnalyze_DependencyNodeDown(t *testing.T) {
	// When a service loses metrics it becomes a dependency-type node.
	// Cascade should still trigger from it.
	// A(ok) → B(down/dependency)
	nodes := []topology.Node{
		node("A", "ok", "service", "ns1"),
		node("B", "down", "dependency", "ns1"),
	}
	edges := []topology.Edge{
		critEdge("A", "B", "http"),
	}

	result := Analyze(nodes, edges, Options{})

	if result.Summary.RootCauseCount != 1 {
		t.Errorf("expected 1 root cause, got %d", result.Summary.RootCauseCount)
	}
	if result.RootCauses[0].ID != "B" {
		t.Errorf("expected root cause B, got %s", result.RootCauses[0].ID)
	}
	if result.Summary.AffectedServiceCount != 1 {
		t.Errorf("expected 1 affected service, got %d", result.Summary.AffectedServiceCount)
	}
}

func TestAnalyze_TotalServicesCount(t *testing.T) {
	nodes := []topology.Node{
		node("A", "ok", "service", "ns1"),
		node("B", "ok", "service", "ns1"),
		node("C", "ok", "dependency", "ns1"),
	}

	result := Analyze(nodes, nil, Options{})

	if result.Summary.TotalServices != 2 {
		t.Errorf("expected 2 total services, got %d", result.Summary.TotalServices)
	}
}

func TestAnalyze_CascadeThroughDegradedNodes(t *testing.T) {
	// Real-world scenario: cascade through intermediate ok/degraded services.
	// svc-01(ok) → svc-02(ok) → svc-04(ok) → svc-06(degraded) → svc-07(down)
	// svc-07 is the root cause. All upstream services are affected.
	// Cascade chains and allFailures must trace through the full path.
	nodes := []topology.Node{
		node("svc-01", "ok", "service", "ns1"),
		node("svc-02", "ok", "service", "ns1"),
		node("svc-04", "ok", "service", "ns2"),
		node("svc-06", "degraded", "service", "ns2"),
		node("svc-07", "down", "service", "ns2"),
	}
	edges := []topology.Edge{
		critEdge("svc-01", "svc-02", "http"),
		critEdge("svc-02", "svc-04", "http"),
		critEdge("svc-04", "svc-06", "http"),
		critEdge("svc-06", "svc-07", "http"),
	}

	result := Analyze(nodes, edges, Options{})

	// Root cause: svc-07 (down, no further downstream).
	if result.Summary.RootCauseCount != 1 {
		t.Errorf("expected 1 root cause, got %d", result.Summary.RootCauseCount)
	}
	if result.RootCauses[0].ID != "svc-07" {
		t.Errorf("expected root cause svc-07, got %s", result.RootCauses[0].ID)
	}

	// Affected services: svc-01, svc-02, svc-04, svc-06 (all upstream of svc-07).
	if result.Summary.AffectedServiceCount != 4 {
		t.Errorf("expected 4 affected services, got %d", result.Summary.AffectedServiceCount)
	}

	// Cascade chains should include chains for all 4 affected services.
	if len(result.CascadeChains) != 4 {
		t.Errorf("expected 4 cascade chains, got %d", len(result.CascadeChains))
	}

	// Check that svc-02's chain goes through the full path.
	for _, c := range result.CascadeChains {
		if c.AffectedService == "svc-02" {
			if c.Depth != 3 {
				t.Errorf("expected svc-02 chain depth 3, got %d", c.Depth)
			}
			expectedPath := []string{"svc-02", "svc-04", "svc-06", "svc-07"}
			if len(c.Path) != len(expectedPath) {
				t.Errorf("expected path %v, got %v", expectedPath, c.Path)
			}
			for i, p := range expectedPath {
				if i < len(c.Path) && c.Path[i] != p {
					t.Errorf("path[%d] = %s, want %s", i, c.Path[i], p)
				}
			}
		}
	}

	// DependsOn should be set for all affected services.
	for _, as := range result.AffectedServices {
		if as.DependsOn == "" {
			t.Errorf("affected service %s has empty dependsOn", as.Service)
		}
	}
}

func TestAnalyzeForService_CascadeThroughDegradedNodes(t *testing.T) {
	// Same scenario, but filtered for svc-02.
	nodes := []topology.Node{
		node("svc-01", "ok", "service", "ns1"),
		node("svc-02", "ok", "service", "ns1"),
		node("svc-04", "ok", "service", "ns2"),
		node("svc-06", "degraded", "service", "ns2"),
		node("svc-07", "down", "service", "ns2"),
	}
	edges := []topology.Edge{
		critEdge("svc-01", "svc-02", "http"),
		critEdge("svc-02", "svc-04", "http"),
		critEdge("svc-04", "svc-06", "http"),
		critEdge("svc-06", "svc-07", "http"),
	}

	result := AnalyzeForService(nodes, edges, "svc-02", Options{})

	// Root cause: svc-07.
	if result.Summary.RootCauseCount != 1 {
		t.Errorf("expected 1 root cause, got %d", result.Summary.RootCauseCount)
	}

	// Affected: only svc-02.
	if result.Summary.AffectedServiceCount != 1 {
		t.Errorf("expected 1 affected service, got %d", result.Summary.AffectedServiceCount)
	}

	// Cascade chains: svc-02 → svc-04 → svc-06 → svc-07.
	if len(result.CascadeChains) != 1 {
		t.Fatalf("expected 1 cascade chain, got %d", len(result.CascadeChains))
	}
	if result.CascadeChains[0].Depth != 3 {
		t.Errorf("expected chain depth 3, got %d", result.CascadeChains[0].Depth)
	}

	// AllFailures should include the actual failure edge (svc-06 → svc-07).
	if result.Summary.TotalFailureCount != 1 {
		t.Errorf("expected 1 failure, got %d", result.Summary.TotalFailureCount)
	}
	if len(result.AllFailures) > 0 && result.AllFailures[0].Dependency != "svc-07" {
		t.Errorf("expected failure dependency svc-07, got %s", result.AllFailures[0].Dependency)
	}
}

func TestAnalyze_NonNilSlices(t *testing.T) {
	result := Analyze(nil, nil, Options{})

	if result.RootCauses == nil {
		t.Error("RootCauses should not be nil")
	}
	if result.AffectedServices == nil {
		t.Error("AffectedServices should not be nil")
	}
	if result.AllFailures == nil {
		t.Error("AllFailures should not be nil")
	}
	if result.CascadeChains == nil {
		t.Error("CascadeChains should not be nil")
	}
}
