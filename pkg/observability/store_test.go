package observability

import (
	"testing"
	"time"

	"github.com/kubepilot/kubepilot/pkg/ai"
)

func TestRCAStore_AddAndGetReport(t *testing.T) {
	store := NewRCAStore(100)

	report := &ai.RCAReport{
		ID:        "rca-test-001",
		Timestamp: time.Now(),
		Severity:  ai.SeverityHigh,
		TargetResource: ai.ResourceRef{
			Kind:      "Pod",
			Name:      "nginx",
			Namespace: "default",
		},
	}

	store.AddReport(report)

	got := store.GetReport("rca-test-001")
	if got == nil {
		t.Fatal("expected to find report 'rca-test-001'")
	}
	if got.Severity != ai.SeverityHigh {
		t.Errorf("expected severity 'high', got %q", got.Severity)
	}
}

func TestRCAStore_GetReportNotFound(t *testing.T) {
	store := NewRCAStore(100)
	got := store.GetReport("nonexistent")
	if got != nil {
		t.Error("expected nil for nonexistent report")
	}
}

func TestRCAStore_ListReports_NoFilter(t *testing.T) {
	store := NewRCAStore(100)
	store.AddReport(&ai.RCAReport{ID: "r1", Severity: ai.SeverityHigh, Timestamp: time.Now()})
	store.AddReport(&ai.RCAReport{ID: "r2", Severity: ai.SeverityLow, Timestamp: time.Now()})

	results := store.ListReports(ReportFilter{})
	if len(results) != 2 {
		t.Errorf("expected 2 reports, got %d", len(results))
	}
}

func TestRCAStore_ListReports_SeverityFilter(t *testing.T) {
	store := NewRCAStore(100)
	store.AddReport(&ai.RCAReport{ID: "r1", Severity: ai.SeverityHigh, Timestamp: time.Now()})
	store.AddReport(&ai.RCAReport{ID: "r2", Severity: ai.SeverityCritical, Timestamp: time.Now()})
	store.AddReport(&ai.RCAReport{ID: "r3", Severity: ai.SeverityHigh, Timestamp: time.Now()})

	results := store.ListReports(ReportFilter{Severity: "high"})
	if len(results) != 2 {
		t.Errorf("expected 2 high-severity reports, got %d", len(results))
	}
}

func TestRCAStore_ListReports_NamespaceFilter(t *testing.T) {
	store := NewRCAStore(100)
	store.AddReport(&ai.RCAReport{
		ID: "r1", TargetResource: ai.ResourceRef{Namespace: "prod"},
	})
	store.AddReport(&ai.RCAReport{
		ID: "r2", TargetResource: ai.ResourceRef{Namespace: "dev"},
	})

	results := store.ListReports(ReportFilter{Namespace: "prod"})
	if len(results) != 1 {
		t.Errorf("expected 1 prod report, got %d", len(results))
	}
}

func TestRCAStore_ListReports_SinceFilter(t *testing.T) {
	store := NewRCAStore(100)
	store.AddReport(&ai.RCAReport{
		ID: "old", Timestamp: time.Now().Add(-2 * time.Hour),
	})
	store.AddReport(&ai.RCAReport{
		ID: "new", Timestamp: time.Now(),
	})

	results := store.ListReports(ReportFilter{
		Since: time.Now().Add(-1 * time.Hour),
	})
	if len(results) != 1 || results[0].ID != "new" {
		t.Errorf("expected 1 recent report, got %d", len(results))
	}
}

func TestRCAStore_MaxItems(t *testing.T) {
	store := NewRCAStore(3)
	store.AddReport(&ai.RCAReport{ID: "r1"})
	store.AddReport(&ai.RCAReport{ID: "r2"})
	store.AddReport(&ai.RCAReport{ID: "r3"})
	store.AddReport(&ai.RCAReport{ID: "r4"})

	// r1 should be evicted.
	if store.GetReport("r1") != nil {
		t.Error("expected r1 to be evicted")
	}
	if store.GetReport("r4") == nil {
		t.Error("expected r4 to exist")
	}

	all := store.ListReports(ReportFilter{})
	if len(all) != 3 {
		t.Errorf("expected 3 reports after eviction, got %d", len(all))
	}
}

func TestRCAStore_AddAndListAnomalies(t *testing.T) {
	store := NewRCAStore(100)

	anomaly := &Anomaly{
		ID:         "a-001",
		DetectedAt: time.Now(),
		Rule:       "CrashLoopDetector",
		Resource:   ai.ResourceRef{Kind: "Pod", Name: "app", Namespace: "default"},
		Severity:   ai.SeverityHigh,
	}

	store.AddAnomaly(anomaly)

	results := store.ListAnomalies(AnomalyFilter{})
	if len(results) != 1 {
		t.Errorf("expected 1 anomaly, got %d", len(results))
	}
	if results[0].ID != "a-001" {
		t.Errorf("expected anomaly ID 'a-001', got %q", results[0].ID)
	}
}

func TestRCAStore_ListAnomalies_Filters(t *testing.T) {
	store := NewRCAStore(100)
	store.AddAnomaly(&Anomaly{
		ID: "a1", DetectedAt: time.Now(), Severity: ai.SeverityHigh,
		Resource: ai.ResourceRef{Namespace: "prod"},
	})
	store.AddAnomaly(&Anomaly{
		ID: "a2", DetectedAt: time.Now(), Severity: ai.SeverityMedium,
		Resource: ai.ResourceRef{Namespace: "dev"},
	})

	results := store.ListAnomalies(AnomalyFilter{Severity: "high"})
	if len(results) != 1 {
		t.Errorf("expected 1 high anomaly, got %d", len(results))
	}

	results = store.ListAnomalies(AnomalyFilter{Namespace: "dev"})
	if len(results) != 1 {
		t.Errorf("expected 1 dev anomaly, got %d", len(results))
	}
}

func TestRCAStore_RecentAnomalyCount(t *testing.T) {
	store := NewRCAStore(100)
	store.AddAnomaly(&Anomaly{ID: "recent", DetectedAt: time.Now()})
	store.AddAnomaly(&Anomaly{ID: "old", DetectedAt: time.Now().Add(-2 * time.Hour)})

	count := store.RecentAnomalyCount(1 * time.Hour)
	if count != 1 {
		t.Errorf("expected 1 recent anomaly, got %d", count)
	}
}

func TestNewRCAStore_DefaultMaxItems(t *testing.T) {
	store := NewRCAStore(0)
	// Should default to 1000, not panic.
	if store.maxItems != 1000 {
		t.Errorf("expected default maxItems=1000, got %d", store.maxItems)
	}
}
