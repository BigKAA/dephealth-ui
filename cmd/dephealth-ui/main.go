package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BigKAA/dephealth-ui/internal/alerts"
	"github.com/BigKAA/dephealth-ui/internal/auth"
	"github.com/BigKAA/dephealth-ui/internal/cache"
	"github.com/BigKAA/dephealth-ui/internal/config"
	"github.com/BigKAA/dephealth-ui/internal/grafana"
	"github.com/BigKAA/dephealth-ui/internal/logging"
	"github.com/BigKAA/dephealth-ui/internal/server"
	"github.com/BigKAA/dephealth-ui/internal/topology"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to configuration file")
	flag.Parse()

	// Bootstrap logger for pre-config errors (text, stderr).
	bootLogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load(*configPath)
	if err != nil {
		bootLogger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	if err := cfg.Validate(); err != nil {
		bootLogger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	// Create configured logger after successful config load.
	logger := logging.NewLogger(cfg.Log)

	logger.Info("starting dephealth-ui",
		"listen", cfg.Server.Listen,
		"prometheus", cfg.Datasources.Prometheus.URL,
		"alertmanager", cfg.Datasources.Alertmanager.URL,
	)

	// Check Grafana dashboard availability at startup.
	checkGrafanaDashboards(cfg, logger)

	promClient := topology.NewPrometheusClient(topology.PrometheusConfig{
		URL:      cfg.Datasources.Prometheus.URL,
		Username: cfg.Datasources.Prometheus.Username,
		Password: cfg.Datasources.Prometheus.Password,
	})

	amClient := alerts.NewClient(alerts.Config{
		URL:      cfg.Datasources.Alertmanager.URL,
		Username: cfg.Datasources.Alertmanager.Username,
		Password: cfg.Datasources.Alertmanager.Password,
	})

	grafanaCfg := topology.GrafanaConfig{
		BaseURL:               cfg.Grafana.BaseURL,
		ServiceStatusDashUID:  cfg.Grafana.Dashboards.ServiceStatus,
		LinkStatusDashUID:     cfg.Grafana.Dashboards.LinkStatus,
		ServiceListDashUID:    cfg.Grafana.Dashboards.ServiceList,
		ServicesStatusDashUID: cfg.Grafana.Dashboards.ServicesStatus,
		LinksStatusDashUID:    cfg.Grafana.Dashboards.LinksStatus,
	}

	builder := topology.NewGraphBuilder(promClient, amClient, grafanaCfg, cfg.Cache.TTL, cfg.Topology.Lookback, logger, cfg.Alerts.SeverityLevels)

	topologyCache := cache.New(cfg.Cache.TTL)

	authenticator, err := auth.NewFromConfigWithContext(context.Background(), cfg.Auth, logger)
	if err != nil {
		logger.Error("failed to create authenticator", "error", err)
		os.Exit(1)
	}

	srv := server.New(cfg, logger, builder, promClient, amClient, topologyCache, authenticator)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := srv.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

// checkGrafanaDashboards validates configured Grafana dashboards at startup.
// Unavailable dashboards are cleared from the config so downstream code
// (which checks for empty UID) hides the corresponding links.
func checkGrafanaDashboards(cfg *config.Config, logger *slog.Logger) {
	if cfg.Grafana.BaseURL == "" {
		return
	}

	checker := grafana.NewChecker(grafana.Config{
		BaseURL:  cfg.Grafana.BaseURL,
		Token:    cfg.Grafana.Token,
		Username: cfg.Grafana.Username,
		Password: cfg.Grafana.Password,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if !checker.Available(ctx) {
		logger.Warn("grafana is unreachable, hiding all dashboard links",
			"baseUrl", cfg.Grafana.BaseURL,
		)
		cfg.Grafana.Dashboards = config.DashboardsConfig{}
		return
	}

	logger.Info("grafana is reachable", "baseUrl", cfg.Grafana.BaseURL)

	// Check each configured dashboard UID.
	dashboards := []struct {
		name string
		uid  *string
	}{
		{"serviceStatus", &cfg.Grafana.Dashboards.ServiceStatus},
		{"linkStatus", &cfg.Grafana.Dashboards.LinkStatus},
		{"serviceList", &cfg.Grafana.Dashboards.ServiceList},
		{"servicesStatus", &cfg.Grafana.Dashboards.ServicesStatus},
		{"linksStatus", &cfg.Grafana.Dashboards.LinksStatus},
		{"cascadeOverview", &cfg.Grafana.Dashboards.CascadeOverview},
		{"rootCause", &cfg.Grafana.Dashboards.RootCause},
		{"connectionDiagnostics", &cfg.Grafana.Dashboards.ConnectionDiagnostics},
	}

	for _, d := range dashboards {
		if *d.uid == "" {
			continue
		}
		if checker.CheckDashboard(ctx, *d.uid) {
			logger.Info("grafana dashboard available", "dashboard", d.name, "uid", *d.uid)
		} else {
			logger.Warn("grafana dashboard not found, hiding link",
				"dashboard", d.name, "uid", *d.uid,
			)
			*d.uid = ""
		}
	}
}
