package observability

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/ai"
	"github.com/kubepilot/kubepilot/pkg/k8s"
)

// ClusterWatcher continuously monitors the cluster, detects anomalies,
// and triggers RCA analysis for detected issues.
type ClusterWatcher struct {
	k8s      *k8s.Client
	rca      *ai.RCAEngine
	store    *RCAStore
	interval time.Duration
	rules    []AnomalyRule
	log      *zap.Logger

	// seenAnomalies tracks recently detected anomalies to avoid duplicate RCA triggers.
	// Key is "resource_kind/namespace/name/rule".
	mu             sync.Mutex
	seenAnomalies  map[string]time.Time
	dedupeWindow   time.Duration
}

// WatcherConfig holds configuration for the ClusterWatcher.
type WatcherConfig struct {
	Interval      time.Duration
	Rules         []AnomalyRule
	DedupeWindow  time.Duration
}

// NewClusterWatcher creates a watcher that polls the cluster at the given interval.
func NewClusterWatcher(k8sClient *k8s.Client, rcaEngine *ai.RCAEngine, store *RCAStore, cfg WatcherConfig, log *zap.Logger) *ClusterWatcher {
	interval := cfg.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}

	rules := cfg.Rules
	if len(rules) == 0 {
		rules = DefaultRules()
	}

	dedupeWindow := cfg.DedupeWindow
	if dedupeWindow <= 0 {
		dedupeWindow = 5 * time.Minute
	}

	return &ClusterWatcher{
		k8s:           k8sClient,
		rca:           rcaEngine,
		store:         store,
		interval:      interval,
		rules:         rules,
		log:           log,
		seenAnomalies: make(map[string]time.Time),
		dedupeWindow:  dedupeWindow,
	}
}

// Start begins the continuous watch loop. It blocks until ctx is cancelled.
func (w *ClusterWatcher) Start(ctx context.Context) {
	w.log.Info("Cluster watcher started",
		zap.Duration("interval", w.interval),
		zap.Int("rules", len(w.rules)),
	)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Run once immediately on startup.
	w.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			w.log.Info("Cluster watcher stopped")
			return
		case <-ticker.C:
			w.poll(ctx)
		}
	}
}

// poll takes a cluster snapshot, evaluates all rules, and triggers RCA for new anomalies.
func (w *ClusterWatcher) poll(ctx context.Context) {
	snapshot, err := w.k8s.TakeClusterSnapshot(ctx)
	if err != nil {
		w.log.Error("Failed to take cluster snapshot", zap.Error(err))
		return
	}

	var allAnomalies []Anomaly
	for _, rule := range w.rules {
		detected, err := rule.Evaluate(ctx, snapshot)
		if err != nil {
			w.log.Warn("Anomaly rule evaluation failed",
				zap.String("rule", rule.Name()),
				zap.Error(err),
			)
			continue
		}
		allAnomalies = append(allAnomalies, detected...)
	}

	if len(allAnomalies) > 0 {
		w.log.Info("Anomalies detected",
			zap.Int("count", len(allAnomalies)),
		)
	}

	// Process each anomaly: store it and trigger RCA if it's new.
	for i := range allAnomalies {
		anomaly := &allAnomalies[i]
		w.store.AddAnomaly(anomaly)

		if w.isNew(anomaly) {
			w.markSeen(anomaly)

			// Only trigger RCA for Pod-level anomalies.
			if anomaly.Resource.Kind == "Pod" && anomaly.Resource.Namespace != "" {
				go w.triggerRCA(ctx, anomaly)
			}
		}
	}

	// Clean up expired dedupe entries.
	w.cleanupSeen()
}

// triggerRCA runs an RCA analysis for the anomaly's target resource.
func (w *ClusterWatcher) triggerRCA(ctx context.Context, anomaly *Anomaly) {
	w.log.Info("Triggering RCA for anomaly",
		zap.String("anomaly_id", anomaly.ID),
		zap.String("resource", anomaly.Resource.Name),
		zap.String("namespace", anomaly.Resource.Namespace),
	)

	report, err := w.rca.AnalyzePod(ctx, anomaly.Resource.Namespace, anomaly.Resource.Name)
	if err != nil {
		w.log.Warn("RCA analysis failed for anomaly",
			zap.String("anomaly_id", anomaly.ID),
			zap.Error(err),
		)
		return
	}

	w.store.AddReport(report)
	anomaly.RCAReportID = report.ID

	w.log.Info("RCA report stored",
		zap.String("report_id", report.ID),
		zap.String("severity", string(report.Severity)),
		zap.String("root_cause", report.RootCause.Category),
		zap.Float64("confidence", report.Confidence),
	)
}

// dedupeKey generates a unique key for an anomaly to prevent duplicate RCA triggers.
func dedupeKey(a *Anomaly) string {
	return a.Resource.Kind + "/" + a.Resource.Namespace + "/" + a.Resource.Name + "/" + a.Rule
}

// isNew returns true if this anomaly hasn't been seen within the dedupe window.
func (w *ClusterWatcher) isNew(a *Anomaly) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	key := dedupeKey(a)
	if lastSeen, ok := w.seenAnomalies[key]; ok {
		return time.Since(lastSeen) > w.dedupeWindow
	}
	return true
}

// markSeen records that this anomaly was detected now.
func (w *ClusterWatcher) markSeen(a *Anomaly) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.seenAnomalies[dedupeKey(a)] = time.Now()
}

// cleanupSeen removes expired entries from the dedupe map.
func (w *ClusterWatcher) cleanupSeen() {
	w.mu.Lock()
	defer w.mu.Unlock()
	cutoff := time.Now().Add(-w.dedupeWindow * 2)
	for key, seen := range w.seenAnomalies {
		if seen.Before(cutoff) {
			delete(w.seenAnomalies, key)
		}
	}
}

// Store returns the watcher's RCA store for external access (dashboard API, etc.).
func (w *ClusterWatcher) Store() *RCAStore {
	return w.store
}
