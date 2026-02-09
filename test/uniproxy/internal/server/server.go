// Package server provides the HTTP server for uniproxy.
package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/BigKAA/topologymetrics/sdk-go/dephealth"
)

// HealthProvider exposes dependency health status.
type HealthProvider interface {
	Health() map[string]bool
}

// StatusResponse is the JSON payload for GET /.
type StatusResponse struct {
	Name      string          `json:"name"`
	PodName   string          `json:"podName"`
	Namespace string          `json:"namespace"`
	Health    map[string]bool `json:"health"`
}

// Server is the uniproxy HTTP server.
type Server struct {
	router chi.Router
	dh     *dephealth.DepHealth
	name   string
}

// New creates a new Server wired to the given dephealth instance.
func New(dh *dephealth.DepHealth, name string) *Server {
	s := &Server{
		dh:   dh,
		name: name,
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	r.Get("/", s.handleRoot)
	r.Get("/healthz", s.handleHealthz)
	r.Get("/readyz", s.handleReadyz)
	r.Handle("/metrics", promhttp.Handler())

	s.router = r
}

// Handler returns the http.Handler.
func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	resp := StatusResponse{
		Name:      s.name,
		PodName:   os.Getenv("POD_NAME"),
		Namespace: os.Getenv("NAMESPACE"),
		Health:    s.dh.Health(),
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("encode response", "error", err)
	}
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *Server) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
