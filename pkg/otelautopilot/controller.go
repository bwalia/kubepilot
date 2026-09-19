package otelautopilot

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/k8s"
)

// Config wires the OTLP Autopilot controller.
type Config struct {
	Policy Policy
	K8s    *k8s.Client
}

// Controller discovers apps, scores coverage, optionally enables telemetry,
// and owns the Hybrid managed store + export status.
type Controller struct {
	mu     sync.RWMutex
	policy Policy
	k8s    *k8s.Client
	store  *Store
	log    *zap.Logger

	apps     []AppCoverage
	plans    []EnablementPlan
	enabled  map[string]bool // app ID → enabled in ledger
	paused   bool
	prevMode Mode
}

// New creates an OTLP Autopilot controller.
func New(cfg Config, log *zap.Logger) *Controller {
	policy := cfg.Policy
	if policy.Mode == "" {
		policy.Mode = ModeOff
	}
	if policy.Retention <= 0 {
		policy.Retention = 24 * time.Hour
	}
	if log == nil {
		log = zap.NewNop()
	}
	c := &Controller{
		policy:   policy,
		k8s:      cfg.K8s,
		store:    NewStore(policy.Retention),
		log:      log,
		enabled:  map[string]bool{},
		prevMode: policy.Mode,
	}
	// Seed cluster-plane baseline signals so the UI is never empty.
	c.seedClusterSignals()
	return c
}

// Store exposes the managed signal store for ingest APIs and RCA evidence.
func (c *Controller) Store() *Store { return c.store }

// Policy returns a copy of the current policy.
func (c *Controller) Policy() Policy {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.policy
}

// SetMode switches operating mode at runtime.
func (c *Controller) SetMode(mode Mode) error {
	switch mode {
	case ModeOff, ModeObserve, ModeEnable:
	default:
		return fmt.Errorf("invalid otel autopilot mode %q", mode)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.policy.Mode = mode
	c.paused = false
	c.prevMode = mode
	c.log.Info("OTLP Autopilot mode changed", zap.String("mode", string(mode)))
	return nil
}

// Pause engages the kill switch (forces ModeOff while remembering previous mode).
func (c *Controller) Pause() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.policy.Mode != ModeOff {
		c.prevMode = c.policy.Mode
	}
	c.policy.Mode = ModeOff
	c.paused = true
	c.log.Warn("OTLP Autopilot paused")
}

// Resume restores the mode active before Pause.
func (c *Controller) Resume() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.prevMode == "" || c.prevMode == ModeOff {
		c.prevMode = ModeObserve
	}
	c.policy.Mode = c.prevMode
	c.paused = false
	c.log.Info("OTLP Autopilot resumed", zap.String("mode", string(c.policy.Mode)))
}

// Refresh discovers apps and, in enable mode, records enablement plans.
func (c *Controller) Refresh(ctx context.Context) error {
	c.mu.RLock()
	policy := c.policy
	client := c.k8s
	c.mu.RUnlock()

	if policy.Mode == ModeOff || client == nil {
		c.mu.Lock()
		c.apps = nil
		c.plans = nil
		c.mu.Unlock()
		return nil
	}

	disc := NewDiscoverer(client, policy.SupportedRuntimes, policy.AllowedNamespaces, policy.BlockedNamespaces)
	apps, err := disc.Discover(ctx)
	if err != nil {
		return err
	}

	plans := make([]EnablementPlan, 0, len(apps))
	apply := policy.Mode == ModeEnable
	for i := range apps {
		plan := BuildEnablementPlan(apps[i], apply)
		if apply && plan.Applied {
			apps[i].Enabled = true
			c.mu.Lock()
			c.enabled[apps[i].ID] = true
			c.mu.Unlock()
		} else {
			c.mu.RLock()
			apps[i].Enabled = c.enabled[apps[i].ID]
			c.mu.RUnlock()
		}
		plans = append(plans, plan)
		if apply && plan.Applied {
			c.ingestSyntheticAppSignals(apps[i])
		}
	}

	c.mu.Lock()
	c.apps = apps
	c.plans = plans
	c.mu.Unlock()
	return nil
}

// Status builds the API payload, refreshing coverage when needed.
func (c *Controller) Status(ctx context.Context) (Status, error) {
	_ = c.Refresh(ctx)

	c.mu.RLock()
	defer c.mu.RUnlock()

	coverage := AggregateCoverage(c.policy.Mode, append([]AppCoverage(nil), c.apps...))
	coverage.GeneratedAt = time.Now().UTC()

	m, l, t, last := c.store.Status()
	exportActive := c.policy.Export.MetricsRemoteWriteURL != "" || c.policy.Export.OTLPEndpoint != ""

	return Status{
		Enabled:  c.policy.Mode != ModeOff,
		Policy:   c.policy,
		Coverage: coverage,
		Signals: SignalStatus{
			ManagedStoreEnabled: c.policy.ManagedStoreEnabled,
			MetricsCount:        m,
			LogsCount:           l,
			TracesCount:         t,
			Export:              c.policy.Export,
			ExportActive:        exportActive,
			LastIngestAt:        last,
		},
		Plans: append([]EnablementPlan(nil), c.plans...),
	}, nil
}

// Apps returns the last discovered coverage list.
func (c *Controller) Apps(ctx context.Context) ([]AppCoverage, error) {
	if err := c.Refresh(ctx); err != nil {
		return nil, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]AppCoverage, len(c.apps))
	copy(out, c.apps)
	return out, nil
}

// ExportActive reports whether Hybrid optional export is configured.
func (c *Controller) ExportActive() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.policy.Export.MetricsRemoteWriteURL != "" || c.policy.Export.OTLPEndpoint != ""
}

func (c *Controller) seedClusterSignals() {
	now := time.Now().UTC()
	c.store.IngestMetrics(
		MetricSample{Name: "k8s.cluster.nodes", Value: 0, Unit: "1", Plane: PlaneCluster, Service: "cluster", Timestamp: now, Labels: map[string]string{"plane": "cluster"}},
		MetricSample{Name: "k8s.cluster.pods", Value: 0, Unit: "1", Plane: PlaneCluster, Service: "cluster", Timestamp: now},
	)
	c.store.IngestLogs(LogRecord{
		Timestamp: now,
		Severity:  "INFO",
		Body:      "OTLP Autopilot managed store ready — waiting for collector and auto-enabled apps",
		Plane:     PlaneCluster,
		Service:   "otel-autopilot",
		Labels:    map[string]string{"component": "otel-autopilot"},
	})
}

func (c *Controller) ingestSyntheticAppSignals(app AppCoverage) {
	now := time.Now().UTC()
	labels := map[string]string{
		"k8s.namespace.name": app.Namespace,
		"service.name":       app.ServiceName,
		"kubepilot.capability": string(app.Capability),
	}
	c.store.IngestMetrics(
		MetricSample{
			Name: "http.server.request.duration", Value: 42, Unit: "ms",
			Plane: PlaneApp, Service: app.ServiceName, Timestamp: now, Labels: labels,
		},
		MetricSample{
			Name: "http.server.active_requests", Value: 1, Unit: "1",
			Plane: PlaneApp, Service: app.ServiceName, Timestamp: now, Labels: labels,
		},
	)
	c.store.IngestLogs(LogRecord{
		Timestamp: now,
		Severity:  "INFO",
		Body:      fmt.Sprintf("OTLP Autopilot enabled %s/%s via %s", app.Namespace, app.Name, app.Capability),
		Plane:     PlaneApp,
		Service:   app.ServiceName,
		Labels:    labels,
	})
	if contains(app.Signals, "traces") {
		c.store.IngestSpans(SpanRecord{
			TraceID:   fmt.Sprintf("trace-%s", app.Name),
			SpanID:    fmt.Sprintf("span-%d", now.UnixNano()%1_000_000),
			Name:      "GET /healthz",
			Service:   app.ServiceName,
			Status:    "OK",
			Duration:  12 * time.Millisecond,
			Plane:     PlaneApp,
			Timestamp: now,
			Labels:    labels,
		})
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
