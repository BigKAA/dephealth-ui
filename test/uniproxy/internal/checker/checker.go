// Package checker provides health check implementations for various dependency types.
package checker

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/BigKAA/uniproxy/internal/config"
	"github.com/BigKAA/uniproxy/internal/metrics"
)

// Result holds the outcome of a single health check.
type Result struct {
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Host      string    `json:"host"`
	Port      string    `json:"port"`
	Healthy   bool      `json:"healthy"`
	LastCheck time.Time `json:"lastCheck"`
	LatencyMs float64   `json:"latencyMs"`
}

// Checker performs a health check for a single connection.
type Checker interface {
	Check(ctx context.Context) Result
}

// Manager runs periodic health checks for all configured connections.
type Manager struct {
	checkers []Checker
	interval time.Duration
	hostname string

	mu      sync.RWMutex
	results []Result
	ready   bool
}

// NewManager creates a Manager from config connections.
func NewManager(connections []config.Connection, interval time.Duration) *Manager {
	hostname, _ := os.Hostname()

	checkers := make([]Checker, 0, len(connections))
	for _, conn := range connections {
		switch conn.Type {
		case "http":
			checkers = append(checkers, NewHTTPChecker(conn))
		case "redis":
			checkers = append(checkers, NewRedisChecker(conn))
		case "postgres":
			checkers = append(checkers, NewPostgresChecker(conn))
		case "grpc":
			checkers = append(checkers, NewGRPCChecker(conn))
		}
	}

	return &Manager{
		checkers: checkers,
		interval: interval,
		hostname: hostname,
	}
}

// Run starts the periodic health check loop. Blocks until ctx is cancelled.
func (m *Manager) Run(ctx context.Context) {
	// Run first check immediately.
	m.checkAll(ctx)
	m.mu.Lock()
	m.ready = true
	m.mu.Unlock()

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkAll(ctx)
		}
	}
}

func (m *Manager) checkAll(ctx context.Context) {
	results := make([]Result, len(m.checkers))
	for i, c := range m.checkers {
		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		r := c.Check(checkCtx)
		cancel()
		results[i] = r

		// Update Prometheus metrics.
		labels := []string{r.Name, r.Type, r.Host, r.Port, m.hostname}

		healthy := 0.0
		if r.Healthy {
			healthy = 1.0
		}
		metrics.DependencyHealth.WithLabelValues(labels...).Set(healthy)
		metrics.DependencyLatency.WithLabelValues(labels...).Observe(r.LatencyMs / 1000.0)

		slog.Debug("health check",
			"dependency", r.Name,
			"type", r.Type,
			"healthy", r.Healthy,
			"latency_ms", r.LatencyMs,
		)
	}

	m.mu.Lock()
	m.results = results
	m.mu.Unlock()
}

// Results returns the latest check results.
func (m *Manager) Results() []Result {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Result, len(m.results))
	copy(out, m.results)
	return out
}

// Ready returns true after the first check cycle completes.
func (m *Manager) Ready() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ready
}
