package export

import (
	"fmt"
	"strings"
)

// DOTOptions configures DOT graph rendering.
type DOTOptions struct {
	RankDir string // "TB" (default), "LR", "BT", "RL"
}

// State colors matching frontend STATE_COLORS (node fill).
var stateColors = map[string]string{
	"ok":       "#d4edda",
	"up":       "#d4edda",
	"degraded": "#fff3cd",
	"down":     "#f8d7da",
	"unknown":  "#e2e3e5",
	"stale":    "#e2e3e5",
}

// Status colors matching frontend STATUS_COLORS (edge).
var statusColors = map[string]string{
	"ok":               "#28a745",
	"timeout":          "#fd7e14",
	"connection_error": "#dc3545",
	"dns_error":        "#6f42c1",
	"auth_error":       "#e83e8c",
	"tls_error":        "#20c997",
	"unhealthy":        "#ffc107",
	"error":            "#dc3545",
}

// Cluster background color.
const clusterFillColor = "#dae8fc"

// ExportDOT produces a Graphviz DOT format representation of the export data.
func ExportDOT(data *ExportData, opts DOTOptions) ([]byte, error) {
	rankDir := opts.RankDir
	if rankDir == "" {
		rankDir = "TB"
	}

	var b strings.Builder

	b.WriteString("digraph dephealth {\n")
	fmt.Fprintf(&b, "  rankdir=%s;\n", rankDir)
	b.WriteString("  node [shape=box, style=\"rounded,filled\"];\n\n")

	// Group nodes by namespace (or group if available).
	clusters := groupNodes(data.Nodes)

	for clusterKey, nodes := range clusters {
		if clusterKey != "" {
			dotID := sanitizeDotID(clusterKey)
			fmt.Fprintf(&b, "  subgraph cluster_%s {\n", dotID)
			fmt.Fprintf(&b, "    label=%s;\n", quoteDot(clusterKey))
			fmt.Fprintf(&b, "    style=filled; fillcolor=%q;\n", clusterFillColor)
			for _, n := range nodes {
				writeNode(&b, n, "    ")
			}
			b.WriteString("  }\n\n")
		} else {
			for _, n := range nodes {
				writeNode(&b, n, "  ")
			}
			b.WriteString("\n")
		}
	}

	// Edges.
	for _, e := range data.Edges {
		writeEdge(&b, e)
	}

	b.WriteString("}\n")
	return []byte(b.String()), nil
}

// groupNodes returns nodes grouped by their cluster key (group or namespace).
// Preserves insertion order using a slice of keys.
func groupNodes(nodes []ExportNode) map[string][]ExportNode {
	m := make(map[string][]ExportNode)
	for _, n := range nodes {
		key := n.Group
		if key == "" {
			key = n.Namespace
		}
		m[key] = append(m[key], n)
	}
	return m
}

func writeNode(b *strings.Builder, n ExportNode, indent string) {
	color := stateColors[n.State]
	if color == "" {
		color = stateColors["unknown"]
	}
	fmt.Fprintf(b, "%s%s [fillcolor=%q];\n", indent, quoteDot(n.ID), color)
}

func writeEdge(b *strings.Builder, e ExportEdge) {
	color := statusColors[e.Status]
	if color == "" {
		color = statusColors["ok"]
	}

	attrs := []string{
		fmt.Sprintf("color=%q", color),
	}
	if e.Type != "" {
		attrs = append(attrs, fmt.Sprintf("label=%s", quoteDot(e.Type)))
	}
	if e.Critical {
		attrs = append(attrs, "style=bold")
	}

	fmt.Fprintf(b, "  %s -> %s [%s];\n",
		quoteDot(e.Source), quoteDot(e.Target), strings.Join(attrs, ", "))
}

// quoteDot wraps a string in double quotes, escaping internal quotes.
func quoteDot(s string) string {
	escaped := strings.ReplaceAll(s, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

// sanitizeDotID replaces non-alphanumeric characters for use as a DOT subgraph ID suffix.
func sanitizeDotID(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}
