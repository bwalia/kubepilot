package observability

import (
	"sync"
	"testing"

	"go.uber.org/zap"
)

func TestAutoRCADefaultsOnWhenUnset(t *testing.T) {
	w := NewClusterWatcher(nil, nil, nil, WatcherConfig{}, zap.NewNop())
	if !w.AutoRCAEnabled() {
		t.Fatal("auto-RCA should default to enabled for callers that predate EnableAutoRCA")
	}
}

func TestAutoRCAHonoursConfig(t *testing.T) {
	off := false
	w := NewClusterWatcher(nil, nil, nil, WatcherConfig{EnableAutoRCA: &off}, zap.NewNop())
	if w.AutoRCAEnabled() {
		t.Fatal("auto-RCA should be disabled when EnableAutoRCA=false")
	}
}

func TestSetAutoRCAPausesAndResumes(t *testing.T) {
	w := NewClusterWatcher(nil, nil, nil, WatcherConfig{}, zap.NewNop())
	w.SetAutoRCA(false)
	if w.AutoRCAEnabled() {
		t.Fatal("expected paused after SetAutoRCA(false)")
	}
	w.SetAutoRCA(false) // idempotent
	w.SetAutoRCA(true)
	if !w.AutoRCAEnabled() {
		t.Fatal("expected resumed after SetAutoRCA(true)")
	}
}

func TestSetAutoRCAIsRaceFree(t *testing.T) {
	w := NewClusterWatcher(nil, nil, nil, WatcherConfig{}, zap.NewNop())
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(on bool) { defer wg.Done(); w.SetAutoRCA(on) }(i%2 == 0)
		go func() { defer wg.Done(); _ = w.AutoRCAEnabled() }()
	}
	wg.Wait()
}
