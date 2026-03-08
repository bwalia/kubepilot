package observability

import (
	"sync"
	"time"

	"github.com/kubepilot/kubepilot/pkg/ai"
)

// RCAStore is a thread-safe in-memory store for RCA reports and anomaly history.
// It maintains a bounded buffer to prevent unbounded memory growth.
type RCAStore struct {
	mu        sync.RWMutex
	reports   []*ai.RCAReport
	anomalies []*Anomaly
	maxItems  int
}

// NewRCAStore creates a store with the given capacity for each collection.
func NewRCAStore(maxItems int) *RCAStore {
	if maxItems <= 0 {
		maxItems = 1000
	}
	return &RCAStore{
		reports:   make([]*ai.RCAReport, 0, maxItems),
		anomalies: make([]*Anomaly, 0, maxItems),
		maxItems:  maxItems,
	}
}

// AddReport stores an RCA report, evicting the oldest if capacity is exceeded.
func (s *RCAStore) AddReport(report *ai.RCAReport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.reports) >= s.maxItems {
		s.reports = s.reports[1:]
	}
	s.reports = append(s.reports, report)
}

// AddAnomaly stores an anomaly, evicting the oldest if capacity is exceeded.
func (s *RCAStore) AddAnomaly(anomaly *Anomaly) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.anomalies) >= s.maxItems {
		s.anomalies = s.anomalies[1:]
	}
	s.anomalies = append(s.anomalies, anomaly)
}

// ListReports returns all stored RCA reports, optionally filtered.
func (s *RCAStore) ListReports(filter ReportFilter) []*ai.RCAReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*ai.RCAReport
	for _, r := range s.reports {
		if filter.Severity != "" && r.Severity != ai.Severity(filter.Severity) {
			continue
		}
		if filter.Namespace != "" && r.TargetResource.Namespace != filter.Namespace {
			continue
		}
		if !filter.Since.IsZero() && r.Timestamp.Before(filter.Since) {
			continue
		}
		if filter.Status != "" && r.Status != ai.RCAStatus(filter.Status) {
			continue
		}
		results = append(results, r)
	}
	return results
}

// GetReport returns a specific RCA report by ID.
func (s *RCAStore) GetReport(id string) *ai.RCAReport {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.reports {
		if r.ID == id {
			return r
		}
	}
	return nil
}

// ListAnomalies returns all stored anomalies, optionally filtered.
func (s *RCAStore) ListAnomalies(filter AnomalyFilter) []*Anomaly {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Anomaly
	for _, a := range s.anomalies {
		if filter.Severity != "" && a.Severity != ai.Severity(filter.Severity) {
			continue
		}
		if filter.Namespace != "" && a.Resource.Namespace != filter.Namespace {
			continue
		}
		if !filter.Since.IsZero() && a.DetectedAt.Before(filter.Since) {
			continue
		}
		results = append(results, a)
	}
	return results
}

// RecentAnomalyCount returns the number of anomalies in the last duration.
func (s *RCAStore) RecentAnomalyCount(d time.Duration) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cutoff := time.Now().Add(-d)
	count := 0
	for _, a := range s.anomalies {
		if a.DetectedAt.After(cutoff) {
			count++
		}
	}
	return count
}

// ReportFilter controls which RCA reports are returned.
type ReportFilter struct {
	Severity  string
	Namespace string
	Since     time.Time
	Status    string
}

// AnomalyFilter controls which anomalies are returned.
type AnomalyFilter struct {
	Severity  string
	Namespace string
	Since     time.Time
}
