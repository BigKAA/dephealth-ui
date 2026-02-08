package alerts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Alert represents a parsed alert from AlertManager mapped to topology entities.
type Alert struct {
	AlertName  string `json:"alertname"`
	Service    string `json:"service"`    // job label (source service)
	Dependency string `json:"dependency"` // dependency label (target)
	Severity   string `json:"severity"`   // "critical", "warning", "info"
	State      string `json:"state"`      // "firing"
	Since      string `json:"since"`      // RFC3339 timestamp
	Summary    string `json:"summary,omitempty"`
}

// AlertManagerClient fetches active alerts from AlertManager.
type AlertManagerClient interface {
	FetchAlerts(ctx context.Context) ([]Alert, error)
}

// Config holds AlertManager connection settings.
type Config struct {
	URL      string
	Username string
	Password string
	Timeout  time.Duration
}

type client struct {
	cfg    Config
	http   *http.Client
}

// NewClient creates a new AlertManager client.
func NewClient(cfg Config) AlertManagerClient {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &client{
		cfg:  cfg,
		http: &http.Client{Timeout: timeout},
	}
}

// amAlert represents AlertManager API v2 alert format.
type amAlert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    string            `json:"startsAt"`
	Status      amStatus          `json:"status"`
}

type amStatus struct {
	State string `json:"state"` // "active", "suppressed", "unprocessed"
}

func (c *client) FetchAlerts(ctx context.Context) ([]Alert, error) {
	if c.cfg.URL == "" {
		return nil, nil
	}

	url := c.cfg.URL + "/api/v2/alerts?active=true&silenced=false&inhibited=false"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if c.cfg.Username != "" {
		req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching alerts: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alertmanager returned %d: %s", resp.StatusCode, string(body))
	}

	var amAlerts []amAlert
	if err := json.Unmarshal(body, &amAlerts); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return mapAlerts(amAlerts), nil
}

// mapAlerts converts AlertManager alerts to topology-mapped alerts.
// Only alerts with "job" and "dependency" labels are relevant.
func mapAlerts(amAlerts []amAlert) []Alert {
	var result []Alert
	for _, a := range amAlerts {
		job := a.Labels["job"]
		dep := a.Labels["dependency"]
		if job == "" || dep == "" {
			continue
		}

		result = append(result, Alert{
			AlertName:  a.Labels["alertname"],
			Service:    job,
			Dependency: dep,
			Severity:   a.Labels["severity"],
			State:      "firing",
			Since:      a.StartsAt,
			Summary:    a.Annotations["summary"],
		})
	}
	return result
}
