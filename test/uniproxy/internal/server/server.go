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

	"github.com/BigKAA/uniproxy/internal/checker"
)

// StatusResponse is the JSON payload for GET /.
type StatusResponse struct {
	PodName     string           `json:"podName"`
	Namespace   string           `json:"namespace"`
	Connections []checker.Result `json:"connections"`
}

// Server is the uniproxy HTTP server.
type Server struct {
	router      chi.Router
	manager     *checker.Manager
	metricsPath string
}

// New creates a new Server wired to the given checker manager.
func New(manager *checker.Manager, metricsPath string) *Server {
	s := &Server{
		manager:     manager,
		metricsPath: metricsPath,
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
	r.Handle(s.metricsPath, promhttp.Handler())

	s.router = r
}

// Handler returns the http.Handler.
func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	resp := StatusResponse{
		PodName:     os.Getenv("POD_NAME"),
		Namespace:   os.Getenv("NAMESPACE"),
		Connections: s.manager.Results(),
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
	if s.manager.Ready() {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	w.Write([]byte("not ready"))
}
