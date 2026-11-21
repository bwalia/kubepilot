package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/observability"
)

func watcherStatus(t *testing.T, rec *httptest.ResponseRecorder) bool {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		AutoRCAEnabled bool `json:"auto_rca_enabled"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body.AutoRCAEnabled
}

func TestWatcherRCAPauseAndResumeEndpoints(t *testing.T) {
	w := observability.NewClusterWatcher(nil, nil, nil, observability.WatcherConfig{}, zap.NewNop())
	s := &Server{cfg: Config{Watcher: w}}

	rec := httptest.NewRecorder()
	s.handleWatcherStatus(rec, httptest.NewRequest(http.MethodGet, "/api/v1/watcher", nil))
	if !watcherStatus(t, rec) {
		t.Fatal("expected auto-RCA enabled initially")
	}

	rec = httptest.NewRecorder()
	s.handleWatcherRCAPause(rec, httptest.NewRequest(http.MethodPost, "/api/v1/watcher/rca/pause", nil))
	if watcherStatus(t, rec) || w.AutoRCAEnabled() {
		t.Fatal("expected auto-RCA paused")
	}

	rec = httptest.NewRecorder()
	s.handleWatcherRCAResume(rec, httptest.NewRequest(http.MethodPost, "/api/v1/watcher/rca/resume", nil))
	if !watcherStatus(t, rec) || !w.AutoRCAEnabled() {
		t.Fatal("expected auto-RCA resumed")
	}
}

func TestWatcherEndpointsWithoutWatcher(t *testing.T) {
	s := &Server{cfg: Config{}}
	for name, h := range map[string]http.HandlerFunc{
		"status": s.handleWatcherStatus, "pause": s.handleWatcherRCAPause, "resume": s.handleWatcherRCAResume,
	} {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(http.MethodPost, "/", nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: status = %d, want 503", name, rec.Code)
		}
	}
}
