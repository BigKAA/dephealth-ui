package grafana

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Config holds Grafana API connection settings.
type Config struct {
	BaseURL  string
	Token    string // API key or service account token (priority over basic auth)
	Username string // Basic auth username
	Password string // Basic auth password
	Timeout  time.Duration
}

// Checker validates Grafana availability and dashboard existence.
type Checker struct {
	cfg  Config
	http *http.Client
}

// NewChecker creates a new Grafana availability checker.
func NewChecker(cfg Config) *Checker {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &Checker{
		cfg:  cfg,
		http: &http.Client{Timeout: timeout},
	}
}

// applyAuth adds authentication headers to the request.
// Priority: token (Bearer) > basic auth > none.
func (c *Checker) applyAuth(req *http.Request) {
	if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
		return
	}
	if c.cfg.Username != "" {
		req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	}
}

// Available checks if Grafana is reachable via GET /api/health.
func (c *Checker) Available(ctx context.Context) bool {
	url := c.cfg.BaseURL + "/api/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	c.applyAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// CheckDashboard checks if a dashboard with the given UID exists.
func (c *Checker) CheckDashboard(ctx context.Context, uid string) bool {
	url := fmt.Sprintf("%s/api/dashboards/uid/%s", c.cfg.BaseURL, uid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	c.applyAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
