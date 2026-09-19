package otelautopilot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kubepilot/kubepilot/pkg/ai"
)

// ContributeEvidence implements ai.EvidenceContributor so RCA can attach
// available OTEL metrics/logs/traces without requiring an external backend.
func (c *Controller) ContributeEvidence(_ context.Context, namespace, name string) []ai.Evidence {
	if c == nil || c.store == nil {
		return nil
	}
	c.mu.RLock()
	managed := c.policy.ManagedStoreEnabled
	c.mu.RUnlock()
	if !managed {
		return nil
	}

	metrics, logs, traces := c.store.EvidenceForResource(namespace, name, 3)
	out := make([]ai.Evidence, 0, 3)
	now := time.Now().UTC()

	if len(metrics) > 0 {
		parts := make([]string, 0, len(metrics))
		for _, m := range metrics {
			parts = append(parts, fmt.Sprintf("%s=%.3f%s", m.Name, m.Value, m.Unit))
		}
		out = append(out, ai.Evidence{
			Source:    "otel-metrics",
			Data:      strings.Join(parts, "; "),
			Relevance: "OTLP Autopilot managed-store metrics near this workload",
			Timestamp: now,
		})
	}
	if len(logs) > 0 {
		parts := make([]string, 0, len(logs))
		for _, l := range logs {
			parts = append(parts, truncate(l.Body, 160))
		}
		out = append(out, ai.Evidence{
			Source:    "otel-logs",
			Data:      strings.Join(parts, " | "),
			Relevance: "OTLP Autopilot managed-store logs near this workload",
			Timestamp: now,
		})
	}
	if len(traces) > 0 {
		parts := make([]string, 0, len(traces))
		for _, sp := range traces {
			parts = append(parts, fmt.Sprintf("%s trace=%s status=%s dur=%s", sp.Name, sp.TraceID, sp.Status, sp.Duration))
		}
		out = append(out, ai.Evidence{
			Source:    "otel-traces",
			Data:      strings.Join(parts, "; "),
			Relevance: "OTLP Autopilot managed-store traces near this workload",
			Timestamp: now,
		})
	}
	return out
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
