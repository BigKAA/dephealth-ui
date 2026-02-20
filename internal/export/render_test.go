package export

import (
	"testing"
)

func TestRenderDOT_PNG(t *testing.T) {
	if !GraphvizAvailable() {
		t.Skip("graphviz not installed, skipping render test")
	}

	dot := []byte(`digraph G { A -> B; }`)
	b, err := RenderDOT(dot, "png", 2)
	if err != nil {
		t.Fatalf("RenderDOT png error: %v", err)
	}

	// PNG files start with the PNG magic bytes.
	if len(b) < 8 || string(b[:4]) != "\x89PNG" {
		t.Error("output is not a valid PNG file")
	}
}

func TestRenderDOT_SVG(t *testing.T) {
	if !GraphvizAvailable() {
		t.Skip("graphviz not installed, skipping render test")
	}

	dot := []byte(`digraph G { A -> B; }`)
	b, err := RenderDOT(dot, "svg", 2)
	if err != nil {
		t.Fatalf("RenderDOT svg error: %v", err)
	}

	svg := string(b)
	if len(svg) == 0 {
		t.Error("SVG output is empty")
	}
	if !contains(svg, "<svg") {
		t.Error("output does not contain <svg tag")
	}
}

func TestRenderDOT_InvalidFormat(t *testing.T) {
	dot := []byte(`digraph G { A -> B; }`)
	_, err := RenderDOT(dot, "pdf", 2)
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

func TestRenderDOT_InvalidDOT(t *testing.T) {
	if !GraphvizAvailable() {
		t.Skip("graphviz not installed, skipping render test")
	}

	dot := []byte(`this is not valid DOT`)
	_, err := RenderDOT(dot, "png", 2)
	if err == nil {
		t.Error("expected error for invalid DOT input")
	}
}

func TestRenderDOT_ScaleClamping(t *testing.T) {
	if !GraphvizAvailable() {
		t.Skip("graphviz not installed, skipping render test")
	}

	dot := []byte(`digraph G { A -> B; }`)

	// Scale 0 should be clamped to 2.
	b, err := RenderDOT(dot, "png", 0)
	if err != nil {
		t.Fatalf("RenderDOT with scale=0 error: %v", err)
	}
	if len(b) == 0 {
		t.Error("output is empty for scale=0")
	}

	// Scale 10 should be clamped to 4.
	b, err = RenderDOT(dot, "png", 10)
	if err != nil {
		t.Fatalf("RenderDOT with scale=10 error: %v", err)
	}
	if len(b) == 0 {
		t.Error("output is empty for scale=10")
	}
}

func TestRenderDOT_FullGraph(t *testing.T) {
	if !GraphvizAvailable() {
		t.Skip("graphviz not installed, skipping render test")
	}

	resp := sampleTopologyResponse()
	data := ConvertTopology(resp, "full", nil)
	dotBytes, err := ExportDOT(data, DOTOptions{})
	if err != nil {
		t.Fatalf("ExportDOT error: %v", err)
	}

	// Render DOT to SVG.
	svgBytes, err := RenderDOT(dotBytes, "svg", 2)
	if err != nil {
		t.Fatalf("RenderDOT svg error: %v", err)
	}
	if !contains(string(svgBytes), "<svg") {
		t.Error("rendered SVG does not contain <svg tag")
	}

	// Render DOT to PNG.
	pngBytes, err := RenderDOT(dotBytes, "png", 2)
	if err != nil {
		t.Fatalf("RenderDOT png error: %v", err)
	}
	if len(pngBytes) < 8 || string(pngBytes[:4]) != "\x89PNG" {
		t.Error("rendered PNG is not valid")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
