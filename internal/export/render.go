package export

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

const renderTimeout = 10 * time.Second

// RenderDOT takes DOT source and renders it to the specified format (png or svg)
// by invoking the Graphviz dot CLI. The scale parameter sets DPI for PNG output
// (scale * 72 DPI; default scale=2 → 144 DPI). For SVG, scale is ignored.
func RenderDOT(dot []byte, format string, scale int) ([]byte, error) {
	if format != "png" && format != "svg" {
		return nil, fmt.Errorf("unsupported render format: %s", format)
	}

	if scale < 1 {
		scale = 2
	}
	if scale > 4 {
		scale = 4
	}

	args := []string{fmt.Sprintf("-T%s", format)}
	if format == "png" {
		dpi := scale * 72
		args = append(args, fmt.Sprintf("-Gdpi=%d", dpi))
	}

	ctx, cancel := context.WithTimeout(context.Background(), renderTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "dot", args...)
	cmd.Stdin = bytes.NewReader(dot)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("graphviz rendering timed out after %s", renderTimeout)
		}
		return nil, fmt.Errorf("graphviz rendering failed: %w: %s", err, stderr.String())
	}

	return stdout.Bytes(), nil
}

// GraphvizAvailable checks if the dot binary is available in PATH.
func GraphvizAvailable() bool {
	_, err := exec.LookPath("dot")
	return err == nil
}
