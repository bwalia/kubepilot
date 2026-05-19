package observability

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/ai"

	_ "modernc.org/sqlite"
)

// Persistence wraps an in-memory RCAStore with SQLite-backed durability.
// All writes go to both the in-memory store and the database, so reads
// stay fast (RAM) but state survives restarts.
//
// On startup, persisted rows are hydrated back into the in-memory store
// (newest first, up to the store's capacity).
type Persistence struct {
	db       *sql.DB
	log      *zap.Logger
	store    *RCAStore
	retention time.Duration
}

// NewPersistence opens or creates a SQLite database at the given path,
// runs schema migrations, hydrates the in-memory store from disk, and
// returns a Persistence wrapper. Pass retention=0 to keep everything;
// otherwise rows older than retention are pruned at startup and hourly.
func NewPersistence(ctx context.Context, dbPath string, store *RCAStore, retention time.Duration, log *zap.Logger) (*Persistence, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating state dir: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("opening sqlite: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pinging sqlite: %w", err)
	}

	p := &Persistence{db: db, log: log, store: store, retention: retention}
	if err := p.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := p.hydrate(ctx); err != nil {
		log.Warn("Hydrating in-memory store from disk failed (continuing with empty cache)", zap.Error(err))
	}
	if retention > 0 {
		if err := p.prune(ctx); err != nil {
			log.Warn("Initial retention prune failed", zap.Error(err))
		}
		go p.pruneLoop(ctx)
	}
	log.Info("RCA persistence enabled", zap.String("db", dbPath), zap.Duration("retention", retention))
	return p, nil
}

// Close shuts down the underlying database connection.
func (p *Persistence) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

func (p *Persistence) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS rca_reports (
			id TEXT PRIMARY KEY,
			timestamp DATETIME NOT NULL,
			severity TEXT,
			namespace TEXT,
			status TEXT,
			payload TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS rca_reports_timestamp_idx ON rca_reports(timestamp DESC)`,
		`CREATE TABLE IF NOT EXISTS anomalies (
			id TEXT PRIMARY KEY,
			detected_at DATETIME NOT NULL,
			severity TEXT,
			namespace TEXT,
			payload TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS anomalies_detected_at_idx ON anomalies(detected_at DESC)`,
	}
	for _, s := range stmts {
		if _, err := p.db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("migration %q: %w", s, err)
		}
	}
	return nil
}

// PersistReport writes a single RCA report to the database. Failures are
// logged but never returned — the in-memory store remains source of truth.
func (p *Persistence) PersistReport(ctx context.Context, r *ai.RCAReport) {
	if p == nil || p.db == nil || r == nil {
		return
	}
	payload, err := json.Marshal(r)
	if err != nil {
		p.log.Warn("Marshal RCA report for persistence", zap.Error(err))
		return
	}
	_, err = p.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO rca_reports (id, timestamp, severity, namespace, status, payload) VALUES (?,?,?,?,?,?)`,
		r.ID, r.Timestamp, string(r.Severity), r.TargetResource.Namespace, string(r.Status), string(payload))
	if err != nil {
		p.log.Warn("Persist RCA report", zap.String("id", r.ID), zap.Error(err))
	}
}

// PersistAnomaly writes a single anomaly to the database. Failures are logged.
func (p *Persistence) PersistAnomaly(ctx context.Context, a *Anomaly) {
	if p == nil || p.db == nil || a == nil {
		return
	}
	payload, err := json.Marshal(a)
	if err != nil {
		p.log.Warn("Marshal anomaly for persistence", zap.Error(err))
		return
	}
	_, err = p.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO anomalies (id, detected_at, severity, namespace, payload) VALUES (?,?,?,?,?)`,
		a.ID, a.DetectedAt, string(a.Severity), a.Resource.Namespace, string(payload))
	if err != nil {
		p.log.Warn("Persist anomaly", zap.String("id", a.ID), zap.Error(err))
	}
}

// hydrate loads up to maxItems most-recent rows back into the in-memory store.
func (p *Persistence) hydrate(ctx context.Context) error {
	max := p.store.maxItems
	if max <= 0 {
		max = 1000
	}

	rows, err := p.db.QueryContext(ctx,
		`SELECT payload FROM rca_reports ORDER BY timestamp DESC LIMIT ?`, max)
	if err != nil {
		return fmt.Errorf("loading reports: %w", err)
	}
	defer rows.Close()
	reports := make([]*ai.RCAReport, 0, max)
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			continue
		}
		var r ai.RCAReport
		if err := json.Unmarshal([]byte(payload), &r); err == nil {
			reports = append(reports, &r)
		}
	}
	// AddReport appends in oldest-first order; we have newest-first, so reverse.
	for i := len(reports) - 1; i >= 0; i-- {
		p.store.AddReport(reports[i])
	}

	rows2, err := p.db.QueryContext(ctx,
		`SELECT payload FROM anomalies ORDER BY detected_at DESC LIMIT ?`, max)
	if err != nil {
		return fmt.Errorf("loading anomalies: %w", err)
	}
	defer rows2.Close()
	anomalies := make([]*Anomaly, 0, max)
	for rows2.Next() {
		var payload string
		if err := rows2.Scan(&payload); err != nil {
			continue
		}
		var a Anomaly
		if err := json.Unmarshal([]byte(payload), &a); err == nil {
			anomalies = append(anomalies, &a)
		}
	}
	for i := len(anomalies) - 1; i >= 0; i-- {
		p.store.AddAnomaly(anomalies[i])
	}

	p.log.Info("Hydrated RCA store from disk",
		zap.Int("reports", len(reports)),
		zap.Int("anomalies", len(anomalies)))
	return nil
}

func (p *Persistence) prune(ctx context.Context) error {
	cutoff := time.Now().Add(-p.retention)
	if _, err := p.db.ExecContext(ctx, `DELETE FROM rca_reports WHERE timestamp < ?`, cutoff); err != nil {
		return err
	}
	if _, err := p.db.ExecContext(ctx, `DELETE FROM anomalies WHERE detected_at < ?`, cutoff); err != nil {
		return err
	}
	return nil
}

func (p *Persistence) pruneLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.prune(ctx); err != nil {
				p.log.Warn("Retention prune failed", zap.Error(err))
			}
		}
	}
}
