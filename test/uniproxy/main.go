// Uniproxy is a universal test proxy that health-checks configured dependencies
// and exposes Prometheus metrics compatible with topologymetrics.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/BigKAA/uniproxy/internal/checker"
	"github.com/BigKAA/uniproxy/internal/config"
	"github.com/BigKAA/uniproxy/internal/metrics"
	"github.com/BigKAA/uniproxy/internal/server"
)

func main() {
	configPath := flag.String("config", "/etc/uniproxy/config.yaml", "path to config file")
	flag.Parse()

	// Override config path from environment.
	if env := os.Getenv("CONFIG_FILE"); env != "" {
		*configPath = env
	}

	// Set up logging.
	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	// Load configuration.
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	slog.Info("config loaded",
		"listen", cfg.Server.Listen,
		"connections", len(cfg.Connections),
		"checkInterval", cfg.CheckInterval,
	)

	// Register Prometheus metrics.
	metrics.Register()

	// Create and start the checker manager.
	mgr := checker.NewManager(cfg.Connections, cfg.CheckInterval)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go mgr.Run(ctx)

	// Create and start the HTTP server.
	srv := server.New(mgr, cfg.Server.MetricsPath)
	httpServer := &http.Server{
		Addr:    cfg.Server.Listen,
		Handler: srv.Handler(),
	}

	go func() {
		slog.Info("server starting", "addr", cfg.Server.Listen)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	httpServer.Close()
}
