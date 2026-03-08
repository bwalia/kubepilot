package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// PrometheusClient queries a Prometheus/Thanos endpoint for metrics data.
type PrometheusClient struct {
	baseURL    string
	httpClient *http.Client
}

// PrometheusConfig holds Prometheus client configuration.
type PrometheusConfig struct {
	URL     string
	Timeout time.Duration
}

// NewPrometheusClient creates a Prometheus client for querying metrics.
func NewPrometheusClient(cfg PrometheusConfig) *PrometheusClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &PrometheusClient{
		baseURL: cfg.URL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// QueryResult holds the result of a Prometheus instant query.
type QueryResult struct {
	Status string          `json:"status"`
	Data   QueryResultData `json:"data"`
}

// QueryResultData holds the data from a Prometheus query response.
type QueryResultData struct {
	ResultType string            `json:"resultType"`
	Result     []json.RawMessage `json:"result"`
}

// VectorSample is a single sample from a Prometheus vector result.
type VectorSample struct {
	Metric map[string]string `json:"metric"`
	Value  [2]interface{}    `json:"value"` // [timestamp, value]
}

// RangeResult is a single time series from a Prometheus range query.
type RangeResult struct {
	Metric map[string]string `json:"metric"`
	Values [][2]interface{}  `json:"values"` // [[timestamp, value], ...]
}

// Query executes an instant PromQL query.
func (p *PrometheusClient) Query(ctx context.Context, query string) (*QueryResult, error) {
	if p.baseURL == "" {
		return nil, fmt.Errorf("prometheus URL not configured")
	}

	u := fmt.Sprintf("%s/api/v1/query?query=%s", p.baseURL, url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("building prometheus request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querying prometheus: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading prometheus response: %w", err)
	}

	var result QueryResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing prometheus response: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("prometheus query failed: status=%s", result.Status)
	}

	return &result, nil
}

// QueryRange executes a range PromQL query over a time window.
func (p *PrometheusClient) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*QueryResult, error) {
	if p.baseURL == "" {
		return nil, fmt.Errorf("prometheus URL not configured")
	}

	u := fmt.Sprintf("%s/api/v1/query_range?query=%s&start=%s&end=%s&step=%s",
		p.baseURL,
		url.QueryEscape(query),
		url.QueryEscape(start.Format(time.RFC3339)),
		url.QueryEscape(end.Format(time.RFC3339)),
		url.QueryEscape(step.String()),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("building prometheus range request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querying prometheus range: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading prometheus range response: %w", err)
	}

	var result QueryResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing prometheus range response: %w", err)
	}

	return &result, nil
}

// IsConfigured returns true if a Prometheus URL has been set.
func (p *PrometheusClient) IsConfigured() bool {
	return p.baseURL != ""
}

// Common PromQL queries for Kubernetes monitoring.
var (
	// PodCPUUsage queries the 5-minute rate of CPU usage by pod.
	PodCPUUsageQuery = `sum(rate(container_cpu_usage_seconds_total{pod!=""}[5m])) by (pod, namespace)`

	// PodMemoryUsage queries current memory usage by pod.
	PodMemoryUsageQuery = `sum(container_memory_working_set_bytes{pod!=""}) by (pod, namespace)`

	// PodRestartRate queries the restart rate over the last hour.
	PodRestartRateQuery = `sum(increase(kube_pod_container_status_restarts_total[1h])) by (pod, namespace)`

	// NodeCPUUsage queries CPU utilization percent by node.
	NodeCPUUsageQuery = `100 - (avg by(instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)`

	// NodeMemoryUsage queries memory utilization percent by node.
	NodeMemoryUsageQuery = `(1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100`

	// HTTPErrorRate queries the rate of 5xx errors.
	HTTPErrorRateQuery = `sum(rate(http_requests_total{code=~"5.."}[5m])) by (service)`
)
