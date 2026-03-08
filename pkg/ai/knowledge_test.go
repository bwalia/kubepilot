package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempKBPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "kb.json")
}

func TestKnowledgeBase_StoreAndGet(t *testing.T) {
	kb := NewKnowledgeBase(tempKBPath(t), 100)

	report := &RCAReport{
		ID:        "rca-001",
		Timestamp: time.Now(),
		TargetResource: ResourceRef{
			Kind:      "Pod",
			Name:      "nginx-abc",
			Namespace: "default",
		},
		Severity: SeverityHigh,
		RootCause: RootCause{
			Category: "CrashLoop",
			Summary:  "App crashes on startup",
		},
		Confidence: 0.9,
		Status:     RCAStatusComplete,
	}

	if err := kb.Store(report); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	got := kb.Get("rca-001")
	if got == nil {
		t.Fatal("Get returned nil for existing report")
	}
	if got.ID != "rca-001" {
		t.Errorf("expected ID 'rca-001', got %q", got.ID)
	}
	if got.RootCause.Category != "CrashLoop" {
		t.Errorf("expected category 'CrashLoop', got %q", got.RootCause.Category)
	}
}

func TestKnowledgeBase_GetNotFound(t *testing.T) {
	kb := NewKnowledgeBase(tempKBPath(t), 100)
	got := kb.Get("nonexistent")
	if got != nil {
		t.Error("expected nil for nonexistent report")
	}
}

func TestKnowledgeBase_Search(t *testing.T) {
	kb := NewKnowledgeBase(tempKBPath(t), 100)

	reports := []*RCAReport{
		{
			ID: "rca-001", Timestamp: time.Now().Add(-2 * time.Hour),
			TargetResource: ResourceRef{Namespace: "default"},
			Severity: SeverityHigh, RootCause: RootCause{Category: "OOM", Summary: "out of memory"},
		},
		{
			ID: "rca-002", Timestamp: time.Now().Add(-1 * time.Hour),
			TargetResource: ResourceRef{Namespace: "production"},
			Severity: SeverityCritical, RootCause: RootCause{Category: "CrashLoop", Summary: "crash loop detected"},
		},
		{
			ID: "rca-003", Timestamp: time.Now(),
			TargetResource: ResourceRef{Namespace: "default"},
			Severity: SeverityMedium, RootCause: RootCause{Category: "OOM", Summary: "memory limit"},
		},
	}

	for _, r := range reports {
		if err := kb.Store(r); err != nil {
			t.Fatalf("Store failed: %v", err)
		}
	}

	// Search by severity.
	results := kb.Search(KBQuery{Severity: "critical"})
	if len(results) != 1 || results[0].ID != "rca-002" {
		t.Errorf("severity filter: expected 1 critical result, got %d", len(results))
	}

	// Search by category.
	results = kb.Search(KBQuery{Category: "OOM"})
	if len(results) != 2 {
		t.Errorf("category filter: expected 2 OOM results, got %d", len(results))
	}

	// Search by namespace.
	results = kb.Search(KBQuery{Namespace: "production"})
	if len(results) != 1 {
		t.Errorf("namespace filter: expected 1 production result, got %d", len(results))
	}

	// Search by keyword.
	results = kb.Search(KBQuery{Keyword: "memory"})
	if len(results) != 2 {
		t.Errorf("keyword filter: expected 2 memory results, got %d", len(results))
	}

	// Search with limit.
	results = kb.Search(KBQuery{Limit: 1})
	if len(results) != 1 {
		t.Errorf("limit filter: expected 1 result, got %d", len(results))
	}
}

func TestKnowledgeBase_MaxItems(t *testing.T) {
	kb := NewKnowledgeBase(tempKBPath(t), 3)

	for i := 0; i < 5; i++ {
		_ = kb.Store(&RCAReport{
			ID:        fmt.Sprintf("rca-%03d", i),
			Timestamp: time.Now(),
		})
	}

	stats := kb.Stats()
	if stats.TotalReports != 3 {
		t.Errorf("expected maxItems=3 to cap at 3 reports, got %d", stats.TotalReports)
	}

	// The oldest reports (0, 1) should have been evicted.
	if kb.Get("rca-000") != nil {
		t.Error("expected rca-000 to be evicted")
	}
	if kb.Get("rca-001") != nil {
		t.Error("expected rca-001 to be evicted")
	}
	if kb.Get("rca-004") == nil {
		t.Error("expected rca-004 to exist")
	}
}

func TestKnowledgeBase_Persistence(t *testing.T) {
	path := tempKBPath(t)

	// Store a report.
	kb1 := NewKnowledgeBase(path, 100)
	_ = kb1.Store(&RCAReport{
		ID:        "persist-001",
		Timestamp: time.Now(),
		Severity:  SeverityLow,
		RootCause: RootCause{Category: "Config"},
	})

	// Create a new KB from the same file — should load the report.
	kb2 := NewKnowledgeBase(path, 100)
	got := kb2.Get("persist-001")
	if got == nil {
		t.Fatal("persisted report not found after reload")
	}
	if got.Severity != SeverityLow {
		t.Errorf("expected severity 'low', got %q", got.Severity)
	}
}

func TestKnowledgeBase_Stats(t *testing.T) {
	kb := NewKnowledgeBase(tempKBPath(t), 100)
	_ = kb.Store(&RCAReport{
		ID: "s1", Timestamp: time.Now(), Severity: SeverityHigh,
		RootCause: RootCause{Category: "OOM"},
	})
	_ = kb.Store(&RCAReport{
		ID: "s2", Timestamp: time.Now(), Severity: SeverityHigh,
		RootCause: RootCause{Category: "CrashLoop"},
	})
	_ = kb.Store(&RCAReport{
		ID: "s3", Timestamp: time.Now(), Severity: SeverityCritical,
		RootCause: RootCause{Category: "OOM"},
	})

	stats := kb.Stats()
	if stats.TotalReports != 3 {
		t.Errorf("expected 3 total reports, got %d", stats.TotalReports)
	}
	if stats.BySeverity["high"] != 2 {
		t.Errorf("expected 2 high severity, got %d", stats.BySeverity["high"])
	}
	if stats.BySeverity["critical"] != 1 {
		t.Errorf("expected 1 critical severity, got %d", stats.BySeverity["critical"])
	}
	if stats.ByCategory["OOM"] != 2 {
		t.Errorf("expected 2 OOM category, got %d", stats.ByCategory["OOM"])
	}
}

func TestKnowledgeBase_FindSimilar(t *testing.T) {
	kb := NewKnowledgeBase(tempKBPath(t), 100)
	_ = kb.Store(&RCAReport{
		ID: "f1", Timestamp: time.Now(), RootCause: RootCause{Category: "OOM"},
	})
	_ = kb.Store(&RCAReport{
		ID: "f2", Timestamp: time.Now(), RootCause: RootCause{Category: "CrashLoop"},
	})
	_ = kb.Store(&RCAReport{
		ID: "f3", Timestamp: time.Now(), RootCause: RootCause{Category: "OOM"},
	})

	target := &RCAReport{RootCause: RootCause{Category: "OOM"}}
	similar := kb.FindSimilar(target, 10)
	if len(similar) != 2 {
		t.Errorf("expected 2 similar OOM reports, got %d", len(similar))
	}
}

func TestKnowledgeBase_NonexistentFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subdir", "deep", "kb.json")
	kb := NewKnowledgeBase(path, 100)

	// Should not panic — just starts empty.
	if kb.Get("anything") != nil {
		t.Error("expected nil from empty KB")
	}

	// Store should create the directory.
	err := kb.Store(&RCAReport{ID: "test"})
	if err != nil {
		t.Fatalf("Store to new directory failed: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected KB file to be created")
	}
}

