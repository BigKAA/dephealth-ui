package topology

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// PrometheusClient queries Prometheus/VictoriaMetrics for topology data.
type PrometheusClient interface {
	// QueryTopologyEdges returns all unique topology edges.
	QueryTopologyEdges(ctx context.Context, opts QueryOptions) ([]TopologyEdge, error)

	// QueryHealthState returns the current health value per edge.
	QueryHealthState(ctx context.Context, opts QueryOptions) (map[EdgeKey]float64, error)

	// QueryAvgLatency returns the average latency per edge.
	QueryAvgLatency(ctx context.Context, opts QueryOptions) (map[EdgeKey]float64, error)

	// QueryP99Latency returns the P99 latency per edge.
	QueryP99Latency(ctx context.Context, opts QueryOptions) (map[EdgeKey]float64, error)

	// QueryInstances returns all instances (pods/containers) for a given service.
	QueryInstances(ctx context.Context, serviceName string) ([]Instance, error)
}

// PrometheusConfig holds Prometheus connection settings.
type PrometheusConfig struct {
	URL      string
	Username string
	Password string
	Timeout  time.Duration
}

type prometheusClient struct {
	cfg    PrometheusConfig
	client *http.Client
}

// NewPrometheusClient creates a new Prometheus client.
func NewPrometheusClient(cfg PrometheusConfig) PrometheusClient {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &prometheusClient{
		cfg:    cfg,
		client: &http.Client{Timeout: timeout},
	}
}

// PromQL query templates for topology construction.
// When namespace is provided, a label filter is injected.
const (
	queryTopologyEdges = `group by (name, namespace, dependency, type, host, port, critical) (app_dependency_health%s)`
	queryHealthState   = `app_dependency_health%s`
	queryAvgLatency    = `rate(app_dependency_latency_seconds_sum%s[5m]) / rate(app_dependency_latency_seconds_count%s[5m])`
	queryP99Latency    = `histogram_quantile(0.99, rate(app_dependency_latency_seconds_bucket%s[5m]))`
	queryInstances     = `group by (instance, pod, job) (app_dependency_health{name="%s"})`
)

// nsFilter returns a PromQL label filter for the given namespace.
// Returns empty string if namespace is empty.
func nsFilter(ns string) string {
	if ns == "" {
		return ""
	}
	return fmt.Sprintf(`{namespace="%s"}`, ns)
}

// promResponse represents Prometheus API v1 instant query response.
type promResponse struct {
	Status string   `json:"status"`
	Data   promData `json:"data"`
}

type promData struct {
	ResultType string       `json:"resultType"`
	Result     []promResult `json:"result"`
}

type promResult struct {
	Metric map[string]string  `json:"metric"`
	Value  [2]json.RawMessage `json:"value"`
}

func (c *prometheusClient) query(ctx context.Context, promql string) ([]promResult, error) {
	u, err := url.Parse(c.cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid prometheus URL: %w", err)
	}
	u.Path = "/api/v1/query"
	u.RawQuery = url.Values{"query": {promql}}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if c.cfg.Username != "" {
		req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querying prometheus: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus returned %d: %s", resp.StatusCode, string(body))
	}

	var pr promResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if pr.Status != "success" {
		return nil, fmt.Errorf("prometheus query failed: status=%s", pr.Status)
	}

	return pr.Data.Result, nil
}

func (c *prometheusClient) QueryTopologyEdges(ctx context.Context, opts QueryOptions) ([]TopologyEdge, error) {
	f := nsFilter(opts.Namespace)
	results, err := c.query(ctx, fmt.Sprintf(queryTopologyEdges, f))
	if err != nil {
		return nil, err
	}

	edges := make([]TopologyEdge, 0, len(results))
	for _, r := range results {
		edges = append(edges, TopologyEdge{
			Name:       r.Metric["name"],
			Namespace:  r.Metric["namespace"],
			Dependency: r.Metric["dependency"],
			Type:       r.Metric["type"],
			Host:       r.Metric["host"],
			Port:       r.Metric["port"],
			Critical:   r.Metric["critical"] == "yes",
		})
	}
	return edges, nil
}

func (c *prometheusClient) QueryHealthState(ctx context.Context, opts QueryOptions) (map[EdgeKey]float64, error) {
	f := nsFilter(opts.Namespace)
	results, err := c.query(ctx, fmt.Sprintf(queryHealthState, f))
	if err != nil {
		return nil, err
	}
	return parseEdgeValues(results)
}

func (c *prometheusClient) QueryAvgLatency(ctx context.Context, opts QueryOptions) (map[EdgeKey]float64, error) {
	f := nsFilter(opts.Namespace)
	results, err := c.query(ctx, fmt.Sprintf(queryAvgLatency, f, f))
	if err != nil {
		return nil, err
	}
	return parseEdgeValues(results)
}

func (c *prometheusClient) QueryP99Latency(ctx context.Context, opts QueryOptions) (map[EdgeKey]float64, error) {
	f := nsFilter(opts.Namespace)
	results, err := c.query(ctx, fmt.Sprintf(queryP99Latency, f))
	if err != nil {
		return nil, err
	}
	return parseEdgeValues(results)
}

// QueryInstances returns all instances (pods/containers) for a given service.
func (c *prometheusClient) QueryInstances(ctx context.Context, serviceName string) ([]Instance, error) {
	results, err := c.query(ctx, fmt.Sprintf(queryInstances, serviceName))
	if err != nil {
		return nil, err
	}

	instances := make([]Instance, 0, len(results))
	for _, r := range results {
		inst := Instance{
			Instance: r.Metric["instance"],
			Pod:      r.Metric["pod"],
			Job:      r.Metric["job"],
			Service:  serviceName,
		}
		// Skip if instance is empty (shouldn't happen, but defensive)
		if inst.Instance == "" {
			continue
		}
		instances = append(instances, inst)
	}

	return instances, nil
}

func parseEdgeValues(results []promResult) (map[EdgeKey]float64, error) {
	m := make(map[EdgeKey]float64, len(results))
	for _, r := range results {
		key := EdgeKey{
			Name: r.Metric["name"],
			Host: r.Metric["host"],
			Port: r.Metric["port"],
		}

		var valStr string
		if err := json.Unmarshal(r.Value[1], &valStr); err != nil {
			continue
		}
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			continue
		}
		// For health: take minimum (worst) across instances.
		// For latency: take maximum across instances.
		if existing, ok := m[key]; ok {
			if val < existing {
				m[key] = val
			}
		} else {
			m[key] = val
		}
	}
	return m, nil
}
