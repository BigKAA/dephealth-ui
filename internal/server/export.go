package server

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/BigKAA/dephealth-ui/internal/export"
	"github.com/BigKAA/dephealth-ui/internal/topology"
)

// handleExport handles GET /api/v1/export/{format}.
// Supported formats: json, csv, dot, png, svg.
// Query parameters:
//   - scope: "full" (default) or "current"
//   - namespace: filter by namespace (used when scope=current)
//   - group: filter by group (used when scope=current)
//   - time: RFC3339 timestamp for historical export
//   - scale: PNG scale factor 1-4 (default 2)
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	format := chi.URLParam(r, "format")

	switch format {
	case "json", "csv", "dot", "png", "svg":
		// valid
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"error":"unsupported export format: %s"}`, format)
		return
	}

	// Parse query parameters.
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = "full"
	}
	if scope != "full" && scope != "current" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"scope must be 'full' or 'current'"}`)
		return
	}

	namespace := r.URL.Query().Get("namespace")
	group := r.URL.Query().Get("group")

	// Parse optional time parameter.
	opts := topology.QueryOptions{}
	if scope == "current" {
		opts.Namespace = namespace
		opts.Group = group
	}
	if timeStr := r.URL.Query().Get("time"); timeStr != "" {
		t, err := time.Parse(time.RFC3339, timeStr)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":"invalid time parameter: must be RFC3339 format"}`)
			return
		}
		opts.Time = &t
	}

	// Parse scale for PNG.
	scale := 2
	if scaleStr := r.URL.Query().Get("scale"); scaleStr != "" {
		v, err := strconv.Atoi(scaleStr)
		if err != nil || v < 1 || v > 4 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":"scale must be an integer between 1 and 4"}`)
			return
		}
		scale = v
	}

	// Get topology data.
	var resp *topology.TopologyResponse
	if opts.Time == nil && opts.Namespace == "" && opts.Group == "" {
		// Try cache for unfiltered, non-historical requests.
		if cached, ok := s.cache.Get(); ok {
			resp = cached
		}
	}
	if resp == nil {
		built, err := s.builder.Build(r.Context(), opts)
		if err != nil {
			s.logger.Error("failed to build topology for export", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, `{"error":"failed to fetch topology data: %s"}`, err.Error())
			return
		}
		resp = built
		// Cache unfiltered live requests.
		if opts.Time == nil && opts.Namespace == "" && opts.Group == "" {
			s.cache.Set(resp)
		}
	}

	// Build filters map for export metadata.
	filters := map[string]string{}
	if namespace != "" {
		filters["namespace"] = namespace
	}
	if group != "" {
		filters["group"] = group
	}

	data := export.ConvertTopology(resp, scope, filters)

	// Generate export output.
	var output []byte
	var contentType string
	var fileExt string
	var err error

	switch format {
	case "json":
		output, err = export.ExportJSON(data)
		contentType = "application/json"
		fileExt = "json"
	case "csv":
		output, err = export.ExportCSV(data)
		contentType = "application/zip"
		fileExt = "zip"
	case "dot":
		output, err = export.ExportDOT(data, export.DOTOptions{RankDir: "TB"})
		contentType = "text/vnd.graphviz"
		fileExt = "dot"
	case "png":
		if !export.GraphvizAvailable() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `{"error":"Graphviz is not installed on the server"}`)
			return
		}
		dot, dotErr := export.ExportDOT(data, export.DOTOptions{RankDir: "TB"})
		if dotErr != nil {
			err = dotErr
			break
		}
		output, err = export.RenderDOT(dot, "png", scale)
		contentType = "image/png"
		fileExt = "png"
	case "svg":
		if !export.GraphvizAvailable() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `{"error":"Graphviz is not installed on the server"}`)
			return
		}
		dot, dotErr := export.ExportDOT(data, export.DOTOptions{RankDir: "TB"})
		if dotErr != nil {
			err = dotErr
			break
		}
		output, err = export.RenderDOT(dot, "svg", scale)
		contentType = "image/svg+xml"
		fileExt = "svg"
	}

	if err != nil {
		s.logger.Error("export failed", "format", format, "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error":"export failed: %s"}`, err.Error())
		return
	}

	filename := export.ExportFilename(fileExt)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(output)))
	w.Write(output)
}
