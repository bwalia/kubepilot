package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// KnowledgeBase stores and retrieves past RCA reports for pattern matching
// and learning. Uses a simple JSON-file-backed store.
type KnowledgeBase struct {
	mu       sync.RWMutex
	reports  []*RCAReport
	filePath string
	maxItems int
}

// NewKnowledgeBase creates a knowledge base that persists to the given file.
func NewKnowledgeBase(filePath string, maxItems int) *KnowledgeBase {
	if maxItems <= 0 {
		maxItems = 5000
	}
	kb := &KnowledgeBase{
		filePath: filePath,
		maxItems: maxItems,
	}
	_ = kb.load()
	return kb
}

// Store saves an RCA report to the knowledge base.
func (kb *KnowledgeBase) Store(report *RCAReport) error {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	if len(kb.reports) >= kb.maxItems {
		kb.reports = kb.reports[1:]
	}
	kb.reports = append(kb.reports, report)
	return kb.persist()
}

// Get retrieves a report by ID.
func (kb *KnowledgeBase) Get(id string) *RCAReport {
	kb.mu.RLock()
	defer kb.mu.RUnlock()
	for _, r := range kb.reports {
		if r.ID == id {
			return r
		}
	}
	return nil
}

// Search finds reports matching the given criteria.
func (kb *KnowledgeBase) Search(query KBQuery) []*RCAReport {
	kb.mu.RLock()
	defer kb.mu.RUnlock()

	var results []*RCAReport
	for _, r := range kb.reports {
		if query.Severity != "" && r.Severity != Severity(query.Severity) {
			continue
		}
		if query.Category != "" && !strings.EqualFold(r.RootCause.Category, query.Category) {
			continue
		}
		if query.Namespace != "" && r.TargetResource.Namespace != query.Namespace {
			continue
		}
		if !query.Since.IsZero() && r.Timestamp.Before(query.Since) {
			continue
		}
		if query.Keyword != "" {
			keyword := strings.ToLower(query.Keyword)
			found := strings.Contains(strings.ToLower(r.RootCause.Summary), keyword) ||
				strings.Contains(strings.ToLower(r.RootCause.Detail), keyword) ||
				strings.Contains(strings.ToLower(r.TargetResource.Name), keyword)
			if !found {
				continue
			}
		}
		results = append(results, r)
	}

	// Sort by timestamp descending (most recent first).
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	if query.Limit > 0 && len(results) > query.Limit {
		results = results[:query.Limit]
	}

	return results
}

// FindSimilar returns past reports with the same root cause category for the same resource kind.
func (kb *KnowledgeBase) FindSimilar(report *RCAReport, maxResults int) []*RCAReport {
	return kb.Search(KBQuery{
		Category: report.RootCause.Category,
		Limit:    maxResults,
	})
}

// Stats returns summary statistics for the knowledge base.
func (kb *KnowledgeBase) Stats() KBStats {
	kb.mu.RLock()
	defer kb.mu.RUnlock()

	stats := KBStats{
		TotalReports:  len(kb.reports),
		BySeverity:    make(map[string]int),
		ByCategory:    make(map[string]int),
	}

	for _, r := range kb.reports {
		stats.BySeverity[string(r.Severity)]++
		stats.ByCategory[r.RootCause.Category]++
	}

	if len(kb.reports) > 0 {
		stats.OldestReport = kb.reports[0].Timestamp
		stats.NewestReport = kb.reports[len(kb.reports)-1].Timestamp
	}

	return stats
}

// KBQuery defines search criteria for the knowledge base.
type KBQuery struct {
	Severity  string
	Category  string
	Namespace string
	Keyword   string
	Since     time.Time
	Limit     int
}

// KBStats provides summary statistics.
type KBStats struct {
	TotalReports int            `json:"total_reports"`
	BySeverity   map[string]int `json:"by_severity"`
	ByCategory   map[string]int `json:"by_category"`
	OldestReport time.Time      `json:"oldest_report"`
	NewestReport time.Time      `json:"newest_report"`
}

func (kb *KnowledgeBase) load() error {
	data, err := os.ReadFile(kb.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading knowledge base: %w", err)
	}
	return json.Unmarshal(data, &kb.reports)
}

func (kb *KnowledgeBase) persist() error {
	dir := filepath.Dir(kb.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating knowledge base directory: %w", err)
	}

	data, err := json.MarshalIndent(kb.reports, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling knowledge base: %w", err)
	}
	return os.WriteFile(kb.filePath, data, 0644)
}
