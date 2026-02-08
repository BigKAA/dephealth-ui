package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/BigKAA/dephealth-ui/internal/config"
)

// Server is the main HTTP server for dephealth-ui.
type Server struct {
	cfg    *config.Config
	logger *slog.Logger
	router *chi.Mux
}

// New creates a new Server instance with configured routes and middleware.
func New(cfg *config.Config, logger *slog.Logger) *Server {
	s := &Server{
		cfg:    cfg,
		logger: logger,
		router: chi.NewRouter(),
	}

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

// Run starts the HTTP server and blocks until the context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.cfg.Server.Listen,
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("HTTP server listening", "addr", s.cfg.Server.Listen)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("HTTP server error: %w", err)
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		s.logger.Info("shutting down HTTP server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("HTTP server shutdown: %w", err)
		}
		return nil
	case err := <-errCh:
		return err
	}
}

func (s *Server) setupMiddleware() {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
}

func (s *Server) setupRoutes() {
	// Health probes
	s.router.Get("/healthz", s.handleHealthz)
	s.router.Get("/readyz", s.handleReadyz)

	// API v1
	s.router.Route("/api/v1", func(r chi.Router) {
		r.Get("/topology", s.handleTopology)
		r.Get("/alerts", s.handleAlerts)
		r.Get("/config", s.handleConfig)
	})

	// SPA static files (placeholder — returns 200 for now)
	s.router.Get("/*", s.handleSPA)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"status":"ok"}`)
}

func (s *Server) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	// TODO: check datasource connectivity
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"status":"ok"}`)
}

func (s *Server) handleTopology(w http.ResponseWriter, _ *http.Request) {
	// TODO: implement in Phase 1
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"nodes":[],"edges":[],"alerts":[],"meta":{}}`)
}

func (s *Server) handleAlerts(w http.ResponseWriter, _ *http.Request) {
	// TODO: implement in Phase 3
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"alerts":[],"meta":{"total":0}}`)
}

func (s *Server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	// TODO: implement in Phase 1 — return frontend-relevant config
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{}`)
}

func (s *Server) handleSPA(w http.ResponseWriter, _ *http.Request) {
	// TODO: implement in Phase 2 — serve embedded SPA files
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `<!DOCTYPE html><html><body><h1>dephealth-ui</h1><p>Frontend will be here in Phase 2.</p></body></html>`)
}
