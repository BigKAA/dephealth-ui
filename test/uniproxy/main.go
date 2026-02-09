// Uniproxy is a universal test proxy that health-checks configured dependencies
// using the dephealth SDK and exposes Prometheus metrics.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/BigKAA/topologymetrics/dephealth"
	// Register built-in checker factories.
	_ "github.com/BigKAA/topologymetrics/dephealth/checks"

	"github.com/BigKAA/uniproxy/internal/config"
	"github.com/BigKAA/uniproxy/internal/server"
)

func main() {
	// Load configuration from environment variables.
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Set up logging.
	level := slog.LevelInfo
	if cfg.LogLevel == "debug" {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	slog.Info("config loaded",
		"name", cfg.Name,
		"listen", cfg.ListenAddr,
		"dependencies", len(cfg.Dependencies),
		"checkInterval", cfg.CheckInterval,
	)

	// Build dephealth SDK options from config.
	opts, err := buildOptions(cfg, logger)
	if err != nil {
		slog.Error("failed to build options", "error", err)
		os.Exit(1)
	}

	// Create dephealth instance.
	dh, err := dephealth.New(cfg.Name, opts...)
	if err != nil {
		slog.Error("failed to create dephealth", "error", err)
		os.Exit(1)
	}

	// Start health checks.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := dh.Start(ctx); err != nil {
		slog.Error("failed to start dephealth", "error", err)
		os.Exit(1)
	}
	slog.Info("dephealth started", "name", cfg.Name)

	// Start HTTP server.
	srv := server.New(dh, cfg.Name)
	httpServer := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: srv.Handler(),
	}

	go func() {
		slog.Info("server starting", "addr", cfg.ListenAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	dh.Stop()
	httpServer.Close()
}

// buildOptions creates dephealth SDK options from the application config.
func buildOptions(cfg *config.Config, logger *slog.Logger) ([]dephealth.Option, error) {
	opts := []dephealth.Option{
		dephealth.WithCheckInterval(cfg.CheckInterval),
		dephealth.WithLogger(logger),
	}

	for _, dep := range cfg.Dependencies {
		opt, err := buildDependencyOption(dep)
		if err != nil {
			return nil, err
		}
		opts = append(opts, opt)
	}
	return opts, nil
}

// buildDependencyOption creates a dephealth dependency option from config.
func buildDependencyOption(dep config.Dependency) (dephealth.Option, error) {
	var depOpts []dephealth.DependencyOption

	// Connection source: URL or explicit host+port.
	if dep.URL != "" {
		depOpts = append(depOpts, dephealth.FromURL(dep.URL))
	} else {
		depOpts = append(depOpts, dephealth.FromParams(dep.Host, dep.Port))
	}

	depOpts = append(depOpts, dephealth.Critical(dep.Critical))

	if dep.HealthPath != "" {
		depOpts = append(depOpts, dephealth.WithHTTPHealthPath(dep.HealthPath))
	}

	switch dep.Type {
	case "http":
		return dephealth.HTTP(dep.Name, depOpts...), nil
	case "redis":
		return dephealth.Redis(dep.Name, depOpts...), nil
	case "postgres":
		return dephealth.Postgres(dep.Name, depOpts...), nil
	case "grpc":
		return dephealth.GRPC(dep.Name, depOpts...), nil
	default:
		return nil, fmt.Errorf("unsupported dependency type %q for %q", dep.Type, dep.Name)
	}
}
