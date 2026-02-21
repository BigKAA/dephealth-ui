package export

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"io"
	"strings"
	"testing"
)

func TestExportCSV_ZipStructure(t *testing.T) {
	resp := sampleTopologyResponse()
	data := ConvertTopology(resp, "full", nil)
	b, err := ExportCSV(data)
	if err != nil {
		t.Fatalf("ExportCSV error: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatalf("zip.NewReader error: %v", err)
	}

	fileNames := make(map[string]bool)
	for _, f := range zr.File {
		fileNames[f.Name] = true
	}

	if !fileNames["nodes.csv"] {
		t.Error("ZIP missing nodes.csv")
	}
	if !fileNames["edges.csv"] {
		t.Error("ZIP missing edges.csv")
	}
	if len(zr.File) != 2 {
		t.Errorf("ZIP contains %d files, want 2", len(zr.File))
	}
}

func TestExportCSV_NodesBOM(t *testing.T) {
	resp := sampleTopologyResponse()
	data := ConvertTopology(resp, "full", nil)
	b, err := ExportCSV(data)
	if err != nil {
		t.Fatalf("ExportCSV error: %v", err)
	}

	content := readZipFile(t, b, "nodes.csv")
	if !bytes.HasPrefix(content, utf8BOM) {
		t.Error("nodes.csv missing UTF-8 BOM")
	}
}

func TestExportCSV_EdgesBOM(t *testing.T) {
	resp := sampleTopologyResponse()
	data := ConvertTopology(resp, "full", nil)
	b, err := ExportCSV(data)
	if err != nil {
		t.Fatalf("ExportCSV error: %v", err)
	}

	content := readZipFile(t, b, "edges.csv")
	if !bytes.HasPrefix(content, utf8BOM) {
		t.Error("edges.csv missing UTF-8 BOM")
	}
}

func TestExportCSV_NodesContent(t *testing.T) {
	resp := sampleTopologyResponse()
	data := ConvertTopology(resp, "full", nil)
	b, err := ExportCSV(data)
	if err != nil {
		t.Fatalf("ExportCSV error: %v", err)
	}

	content := readZipFile(t, b, "nodes.csv")
	// Strip BOM before parsing.
	content = bytes.TrimPrefix(content, utf8BOM)
	r := csv.NewReader(bytes.NewReader(content))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("CSV parse error: %v", err)
	}

	// Header + 3 data rows.
	if len(records) != 4 {
		t.Fatalf("nodes.csv rows = %d, want 4 (header + 3)", len(records))
	}

	header := records[0]
	expectedHeader := []string{"id", "name", "namespace", "group", "type", "state", "alerts"}
	for i, h := range expectedHeader {
		if header[i] != h {
			t.Errorf("header[%d] = %q, want %q", i, header[i], h)
		}
	}

	// First data row: order-api.
	row := records[1]
	if row[0] != "order-api" {
		t.Errorf("nodes row[0].id = %q, want %q", row[0], "order-api")
	}
}

func TestExportCSV_EdgesContent(t *testing.T) {
	resp := sampleTopologyResponse()
	data := ConvertTopology(resp, "full", nil)
	b, err := ExportCSV(data)
	if err != nil {
		t.Fatalf("ExportCSV error: %v", err)
	}

	content := readZipFile(t, b, "edges.csv")
	content = bytes.TrimPrefix(content, utf8BOM)
	r := csv.NewReader(bytes.NewReader(content))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("CSV parse error: %v", err)
	}

	// Header + 2 data rows.
	if len(records) != 3 {
		t.Fatalf("edges.csv rows = %d, want 3 (header + 2)", len(records))
	}

	header := records[0]
	expectedHeader := []string{"source", "target", "dependency", "type", "host", "port",
		"critical", "health", "status", "detail", "latency_ms"}
	for i, h := range expectedHeader {
		if header[i] != h {
			t.Errorf("header[%d] = %q, want %q", i, header[i], h)
		}
	}

	// First edge: order-api -> postgres-main.
	row := records[1]
	if row[0] != "order-api" {
		t.Errorf("edge row[0].source = %q, want %q", row[0], "order-api")
	}
	if row[6] != "true" {
		t.Errorf("edge row[0].critical = %q, want %q", row[6], "true")
	}
	if !strings.Contains(row[10], "3.2") {
		t.Errorf("edge row[0].latency_ms = %q, want to contain %q", row[10], "3.2")
	}
}

// readZipFile extracts a file from a ZIP archive.
func readZipFile(t *testing.T, zipData []byte, name string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("zip.NewReader error: %v", err)
	}
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open %s: %v", name, err)
			}
			defer func() { _ = rc.Close() }()
			data, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			return data
		}
	}
	t.Fatalf("file %s not found in ZIP", name)
	return nil
}
