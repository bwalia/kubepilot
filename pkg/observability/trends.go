package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/ai"
)

// TrendAnalyzer detects degradation patterns in Prometheus metrics
// before they become incidents.
type TrendAnalyzer struct {
	prom *PrometheusClient
	log  *zap.Logger
}

// TrendAlert represents a detected metric trend that may indicate an upcoming issue.
type TrendAlert struct {
	ID          string      `json:"id"`
	DetectedAt  time.Time   `json:"detected_at"`
	Metric      string      `json:"metric"`
	Resource    string      `json:"resource"`
	Namespace   string      `json:"namespace"`
	Severity    ai.Severity `json:"severity"`
	Description string      `json:"description"`
	CurrentVal  float64     `json:"current_value"`
	Threshold   float64     `json:"threshold"`
}

// NewTrendAnalyzer creates a trend analyzer backed by Prometheus.
func NewTrendAnalyzer(prom *PrometheusClient, log *zap.Logger) *TrendAnalyzer {
	return &TrendAnalyzer{
		prom: prom,
		log:  log,
	}
}

// AnalyzeCPUTrends checks for pods approaching their CPU limits.
func (t *TrendAnalyzer) AnalyzeCPUTrends(ctx context.Context) ([]TrendAlert, error) {
	if !t.prom.IsConfigured() {
		return nil, nil
	}

	result, err := t.prom.Query(ctx, PodCPUUsageQuery)
	if err != nil {
		return nil, fmt.Errorf("querying CPU trends: %w", err)
	}

	return t.parseAlerts(result, "cpu_usage", 0.8, ai.SeverityMedium)
}

// AnalyzeMemoryTrends checks for pods approaching their memory limits.
func (t *TrendAnalyzer) AnalyzeMemoryTrends(ctx context.Context) ([]TrendAlert, error) {
	if !t.prom.IsConfigured() {
		return nil, nil
	}

	result, err := t.prom.Query(ctx, PodMemoryUsageQuery)
	if err != nil {
		return nil, fmt.Errorf("querying memory trends: %w", err)
	}

	return t.parseAlerts(result, "memory_usage", 0.9, ai.SeverityHigh)
}

// AnalyzeRestartTrends checks for pods with increasing restart rates.
func (t *TrendAnalyzer) AnalyzeRestartTrends(ctx context.Context) ([]TrendAlert, error) {
	if !t.prom.IsConfigured() {
		return nil, nil
	}

	result, err := t.prom.Query(ctx, PodRestartRateQuery)
	if err != nil {
		return nil, fmt.Errorf("querying restart trends: %w", err)
	}

	var alerts []TrendAlert
	for _, raw := range result.Data.Result {
		var sample VectorSample
		if err := json.Unmarshal(raw, &sample); err != nil {
			continue
		}

		valStr, ok := sample.Value[1].(string)
		if !ok {
			continue
		}
		var val float64
		fmt.Sscanf(valStr, "%f", &val)

		// Alert if restart rate > 3 per hour.
		if val > 3 {
			alerts = append(alerts, TrendAlert{
				ID:          fmt.Sprintf("trend-restarts-%s-%d", sample.Metric["pod"], time.Now().Unix()),
				DetectedAt:  time.Now().UTC(),
				Metric:      "restart_rate",
				Resource:    sample.Metric["pod"],
				Namespace:   sample.Metric["namespace"],
				Severity:    ai.SeverityHigh,
				Description: fmt.Sprintf("Pod %s has %0.f restarts in the last hour", sample.Metric["pod"], val),
				CurrentVal:  val,
				Threshold:   3,
			})
		}
	}

	return alerts, nil
}

func (t *TrendAnalyzer) parseAlerts(result *QueryResult, metric string, threshold float64, severity ai.Severity) ([]TrendAlert, error) {
	var alerts []TrendAlert

	for _, raw := range result.Data.Result {
		var sample VectorSample
		if err := json.Unmarshal(raw, &sample); err != nil {
			continue
		}

		valStr, ok := sample.Value[1].(string)
		if !ok {
			continue
		}
		var val float64
		fmt.Sscanf(valStr, "%f", &val)

		if val > threshold {
			alerts = append(alerts, TrendAlert{
				ID:          fmt.Sprintf("trend-%s-%s-%d", metric, sample.Metric["pod"], time.Now().Unix()),
				DetectedAt:  time.Now().UTC(),
				Metric:      metric,
				Resource:    sample.Metric["pod"],
				Namespace:   sample.Metric["namespace"],
				Severity:    severity,
				Description: fmt.Sprintf("Resource %s at %.1f%% (threshold: %.0f%%)", metric, val*100, threshold*100),
				CurrentVal:  val,
				Threshold:   threshold,
			})
		}
	}

	return alerts, nil
}
