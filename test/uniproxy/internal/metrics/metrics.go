// Package metrics registers Prometheus metrics for uniproxy.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// DependencyHealth is a gauge: 1=healthy, 0=unhealthy.
	DependencyHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "app_dependency_health",
			Help: "Health status of a dependency (1=healthy, 0=unhealthy)",
		},
		[]string{"dependency", "type", "host", "port", "hostname"},
	)

	// DependencyLatency is a histogram of health check latency.
	DependencyLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "app_dependency_latency_seconds",
			Help:    "Latency of dependency health check",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0},
		},
		[]string{"dependency", "type", "host", "port", "hostname"},
	)
)

// Register adds all metrics to the default Prometheus registry.
func Register() {
	prometheus.MustRegister(DependencyHealth)
	prometheus.MustRegister(DependencyLatency)
}
