package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/BigKAA/dephealth-ui/internal/config"
	"github.com/BigKAA/dephealth-ui/internal/topology"
)

// Server is the main HTTP server for dephealth-ui.
type Server struct {
	cfg     *config.Config
	logger  *slog.Logger
	router  *chi.Mux
	builder *topology.GraphBuilder
}

// New creates a new Server instance with configured routes and middleware.
func New(cfg *config.Config, logger *slog.Logger, builder *topology.GraphBuilder) *Server {
	s := &Server{
		cfg:     cfg,
		logger:  logger,
		router:  chi.NewRouter(),
		builder: builder,
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
	s.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
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

	// SPA static files (embedded via embed.FS)
	s.router.Handle("/*", newStaticHandler())
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

func (s *Server) handleTopology(w http.ResponseWriter, r *http.Request) {
	resp, err := s.builder.Build(r.Context())
	if err != nil {
		s.logger.Error("failed to build topology", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, `{"error":"failed to fetch topology data: %s"}`, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Error("failed to encode topology response", "error", err)
	}
}

func (s *Server) handleAlerts(w http.ResponseWriter, _ *http.Request) {
	// TODO: implement in Phase 3
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"alerts":[],"meta":{"total":0}}`)
}

// configResponse holds frontend-relevant configuration.
type configResponse struct {
	Grafana configGrafana `json:"grafana"`
	Cache   configCache   `json:"cache"`
}

type configGrafana struct {
	BaseURL    string            `json:"baseUrl"`
	Dashboards configDashboards  `json:"dashboards"`
}

type configDashboards struct {
	ServiceStatus string `json:"serviceStatus"`
	LinkStatus    string `json:"linkStatus"`
}

type configCache struct {
	TTL int `json:"ttl"`
}

func (s *Server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	resp := configResponse{
		Grafana: configGrafana{
			BaseURL: s.cfg.Grafana.BaseURL,
			Dashboards: configDashboards{
				ServiceStatus: s.cfg.Grafana.Dashboards.ServiceStatus,
				LinkStatus:    s.cfg.Grafana.Dashboards.LinkStatus,
			},
		},
		Cache: configCache{
			TTL: int(s.cfg.Cache.TTL.Seconds()),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Error("failed to encode config response", "error", err)
	}
}

