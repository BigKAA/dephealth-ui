package checker

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/BigKAA/uniproxy/internal/config"
)

// HTTPChecker performs HTTP GET health checks.
type HTTPChecker struct {
	conn   config.Connection
	client *http.Client
}

// NewHTTPChecker creates an HTTP checker for the given connection.
func NewHTTPChecker(conn config.Connection) *HTTPChecker {
	return &HTTPChecker{
		conn: conn,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Check performs an HTTP GET and returns the result.
func (c *HTTPChecker) Check(ctx context.Context) Result {
	path := c.conn.Path
	if path == "" {
		path = "/"
	}
	url := fmt.Sprintf("http://%s:%s%s", c.conn.Host, c.conn.Port, path)

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return c.result(false, time.Since(start))
	}

	resp, err := c.client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		return c.result(false, elapsed)
	}
	resp.Body.Close()

	healthy := resp.StatusCode >= 200 && resp.StatusCode < 300
	return c.result(healthy, elapsed)
}

func (c *HTTPChecker) result(healthy bool, elapsed time.Duration) Result {
	return Result{
		Name:      c.conn.Name,
		Type:      c.conn.Type,
		Host:      c.conn.Host,
		Port:      c.conn.Port,
		Healthy:   healthy,
		LastCheck: time.Now(),
		LatencyMs: float64(elapsed.Microseconds()) / 1000.0,
	}
}
