package export

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
)

// utf8BOM is prepended to each CSV file for Excel auto-detection.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// ExportCSV produces a ZIP archive containing nodes.csv and edges.csv.
func ExportCSV(data *ExportData) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	if err := writeNodesCSV(zw, data.Nodes); err != nil {
		return nil, fmt.Errorf("writing nodes.csv: %w", err)
	}
	if err := writeEdgesCSV(zw, data.Edges); err != nil {
		return nil, fmt.Errorf("writing edges.csv: %w", err)
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("closing zip: %w", err)
	}
	return buf.Bytes(), nil
}

func writeNodesCSV(zw *zip.Writer, nodes []ExportNode) error {
	w, err := zw.Create("nodes.csv")
	if err != nil {
		return err
	}
	if _, err := w.Write(utf8BOM); err != nil {
		return err
	}

	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"id", "name", "namespace", "group", "type", "state", "alerts"}); err != nil {
		return err
	}
	for _, n := range nodes {
		if err := cw.Write([]string{
			n.ID,
			n.Name,
			n.Namespace,
			n.Group,
			n.Type,
			n.State,
			fmt.Sprintf("%d", n.Alerts),
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func writeEdgesCSV(zw *zip.Writer, edges []ExportEdge) error {
	w, err := zw.Create("edges.csv")
	if err != nil {
		return err
	}
	if _, err := w.Write(utf8BOM); err != nil {
		return err
	}

	cw := csv.NewWriter(w)
	if err := cw.Write([]string{
		"source", "target", "dependency", "type", "host", "port",
		"critical", "health", "status", "detail", "latency_ms",
	}); err != nil {
		return err
	}
	for _, e := range edges {
		if err := cw.Write([]string{
			e.Source,
			e.Target,
			e.Dependency,
			e.Type,
			e.Host,
			e.Port,
			fmt.Sprintf("%t", e.Critical),
			fmt.Sprintf("%g", e.Health),
			e.Status,
			e.Detail,
			fmt.Sprintf("%g", e.LatencyMs),
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
