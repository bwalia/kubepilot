package runbooks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// LoadFromDir reads every *.yaml and *.yml file in dir and returns the runbooks.
// Files may contain a single runbook or a `runbooks:` list. Errors per file are
// logged but don't fail the whole load — partial sets are better than none.
func LoadFromDir(dir string, log *zap.Logger) ([]Runbook, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading runbook dir %q: %w", dir, err)
	}
	var out []Runbook
	seen := map[string]string{} // id -> source file (for dedup logging)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		rbs, err := loadFile(path)
		if err != nil {
			log.Warn("Skipping invalid runbook file", zap.String("file", path), zap.Error(err))
			continue
		}
		for _, rb := range rbs {
			if rb.ID == "" {
				log.Warn("Skipping runbook with empty ID", zap.String("file", path))
				continue
			}
			if prev, dup := seen[rb.ID]; dup {
				log.Warn("Duplicate runbook ID, later file wins",
					zap.String("id", rb.ID), zap.String("previous", prev), zap.String("current", path))
			}
			seen[rb.ID] = path
			out = append(out, rb)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// loadFile parses a single YAML file as either a Runbook or a {runbooks: []Runbook}.
func loadFile(path string) ([]Runbook, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Try wrapper form first.
	var wrapper struct {
		Runbooks []Runbook `yaml:"runbooks"`
	}
	if err := yaml.Unmarshal(data, &wrapper); err == nil && len(wrapper.Runbooks) > 0 {
		return wrapper.Runbooks, nil
	}
	// Fall back to single-runbook form.
	var single Runbook
	if err := yaml.Unmarshal(data, &single); err != nil {
		return nil, fmt.Errorf("yaml parse: %w", err)
	}
	if single.ID == "" {
		return nil, fmt.Errorf("file is neither a {runbooks: [...]} list nor a runbook with id")
	}
	return []Runbook{single}, nil
}

// WatchDir loads runbooks from dir, applies them to the engine, then watches
// the directory for changes and reloads on every write/create/remove. Runs
// until ctx is cancelled. Errors are logged, not returned — best-effort.
func WatchDir(ctx context.Context, dir string, engine *Engine, log *zap.Logger) error {
	if dir == "" {
		return nil
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// Auto-create so the operator can drop files in later without restarting.
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating runbook dir %q: %w", dir, err)
		}
		log.Info("Created runbook directory", zap.String("dir", dir))
	}

	reload := func() {
		rbs, err := LoadFromDir(dir, log)
		if err != nil {
			log.Warn("Failed to load runbooks", zap.Error(err))
			return
		}
		engine.SetUserRunbooks(rbs)
	}
	reload()

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating fsnotify watcher: %w", err)
	}
	if err := w.Add(dir); err != nil {
		_ = w.Close()
		return fmt.Errorf("watching %q: %w", dir, err)
	}

	go func() {
		defer w.Close()
		// Debounce burst events (editors save in multiple steps).
		var (
			debounce sync.Mutex
			timer    *time.Timer
		)
		schedule := func() {
			debounce.Lock()
			defer debounce.Unlock()
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(500*time.Millisecond, reload)
		}
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				name := strings.ToLower(ev.Name)
				if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
					continue
				}
				schedule()
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				log.Warn("Runbook watcher error", zap.Error(err))
			}
		}
	}()

	log.Info("Watching runbook directory for changes", zap.String("dir", dir))
	return nil
}
