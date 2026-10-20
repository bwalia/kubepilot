package dashboard

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kubepilot/kubepilot/pkg/k8s"
)

// Pod usage is expensive to collect (a pod list, a metrics list and one
// kubelet stats call per node) and every dashboard tab polls it, so responses
// come from a stale-while-revalidate cache: anything younger than
// podUsageFresh is served as is, anything younger than podUsageMaxStale is
// served immediately while one background refresh replaces it.
const (
	podUsageFresh    = 15 * time.Second
	podUsageMaxStale = 5 * time.Minute
	podUsageTimeout  = 30 * time.Second
)

type podUsageKey struct {
	client    *k8s.Client
	namespace string
}

type podUsageEntry struct {
	report  *k8s.PodUsageReport
	fetched time.Time
	// inflight is closed when the running refresh finishes; nil when idle.
	inflight chan struct{}
	err      error
}

type podUsageCache struct {
	mu      sync.Mutex
	entries map[podUsageKey]*podUsageEntry
}

// get returns a report for (client, namespace), waiting only when nothing
// usable is cached.
func (c *podUsageCache) get(ctx context.Context, client *k8s.Client, namespace string) (*k8s.PodUsageReport, error) {
	key := podUsageKey{client, namespace}

	c.mu.Lock()
	if c.entries == nil {
		c.entries = map[podUsageKey]*podUsageEntry{}
	}
	e := c.entries[key]
	if e == nil {
		e = &podUsageEntry{}
		c.entries[key] = e
	}
	age := time.Since(e.fetched)
	if e.report != nil && age < podUsageMaxStale {
		if age >= podUsageFresh {
			c.refreshLocked(key, e)
		}
		report := e.report
		c.mu.Unlock()
		return report, nil
	}
	done := c.refreshLocked(key, e)
	c.mu.Unlock()

	select {
	case <-done:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e.report == nil {
		return nil, e.err
	}
	return e.report, nil
}

// refreshLocked starts a background refresh unless one is already running,
// and returns a channel closed when it completes. c.mu must be held.
func (c *podUsageCache) refreshLocked(key podUsageKey, e *podUsageEntry) <-chan struct{} {
	if e.inflight != nil {
		return e.inflight
	}
	done := make(chan struct{})
	e.inflight = done
	go func() {
		// Detached from any request: a browser navigating away must not
		// cancel the refresh every other viewer is waiting on.
		ctx, cancel := context.WithTimeout(context.Background(), podUsageTimeout)
		defer cancel()
		report, err := key.client.ListPodUsage(ctx, key.namespace)

		c.mu.Lock()
		if err == nil {
			e.report, e.fetched = report, time.Now()
		}
		e.err = err
		e.inflight = nil
		c.mu.Unlock()
		close(done)
	}()
	return done
}

func (s *Server) warmPodUsage() {
	if client := s.currentK8sClient(); client != nil && client.Core != nil {
		s.podUsage.mu.Lock()
		if s.podUsage.entries == nil {
			s.podUsage.entries = map[podUsageKey]*podUsageEntry{}
		}
		key := podUsageKey{client, ""}
		e := &podUsageEntry{}
		s.podUsage.entries[key] = e
		s.podUsage.refreshLocked(key, e)
		s.podUsage.mu.Unlock()
	}
}

// handlePodUsage returns live CPU, memory and storage consumption for pods in
// ?namespace= (empty or "all" for the whole cluster). With ?top=N it returns
// only the pods ranking in the top N by any of CPU, memory or storage — the
// small payload the "Top consumers" widgets need.
func (s *Server) handlePodUsage(w http.ResponseWriter, r *http.Request) {
	ns := strings.TrimSpace(r.URL.Query().Get("namespace"))
	if strings.EqualFold(ns, "all") {
		ns = ""
	}
	report, err := s.podUsage.get(r.Context(), s.k8sFor(r), ns)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	if n, _ := strconv.Atoi(r.URL.Query().Get("top")); n > 0 {
		trimmed := *report
		trimmed.Pods = topPodUsage(report.Pods, n)
		report = &trimmed
	}
	writeJSONCompressed(w, r, report)
}

// topPodUsage returns the union of the top n pods by CPU, memory and storage,
// without modifying pods.
func topPodUsage(pods []k8s.PodUsage, n int) []k8s.PodUsage {
	metrics := []func(p *k8s.PodUsage) int64{
		func(p *k8s.PodUsage) int64 { return p.CPUUsageMilli },
		func(p *k8s.PodUsage) int64 { return p.MemUsageBytes },
		func(p *k8s.PodUsage) int64 { return p.StorageUsedBytes },
	}
	idx := make([]int, len(pods))
	keep := map[int]bool{}
	for _, m := range metrics {
		for i := range idx {
			idx[i] = i
		}
		sort.SliceStable(idx, func(a, b int) bool { return m(&pods[idx[a]]) > m(&pods[idx[b]]) })
		for _, i := range idx[:min(n, len(idx))] {
			keep[i] = true
		}
	}
	out := make([]k8s.PodUsage, 0, len(keep))
	for i := range pods {
		if keep[i] {
			out = append(out, pods[i])
		}
	}
	return out
}

// writeJSONCompressed is writeJSON with gzip when the client accepts it. The
// full usage list for a large cluster is several hundred KB of JSON that
// compresses roughly tenfold.
func writeJSONCompressed(w http.ResponseWriter, r *http.Request, v any) {
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		writeJSON(w, v)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Add("Vary", "Accept-Encoding")
	gz := gzip.NewWriter(w)
	defer gz.Close()
	_ = json.NewEncoder(gz).Encode(v)
}
