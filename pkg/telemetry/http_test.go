package telemetry

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// newTestProvider returns a Provider with local metrics enabled and no
// exporter, so tests can read the recorded series straight off /metrics.
func newTestProvider(t *testing.T) *Provider {
	t.Helper()
	p, err := Init(context.Background(), Config{PrometheusEnabled: true}, zap.NewNop())
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })
	return p
}

func scrape(t *testing.T, p *Provider) string {
	t.Helper()
	rec := httptest.NewRecorder()
	p.MetricsHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	return rec.Body.String()
}

// TestMiddlewareLabelsByRouteTemplate is the cardinality guard for the HTTP
// surface: the recorded series must name the route pattern, never the concrete
// namespace and pod that were requested.
func TestMiddlewareLabelsByRouteTemplate(t *testing.T) {
	p := newTestProvider(t)

	router := mux.NewRouter()
	router.Use(p.MuxRouteNamer())
	router.HandleFunc("/api/v1/pods/{namespace}/{pod}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := p.HTTPMiddleware(router)
	for _, path := range []string{"/api/v1/pods/prod/checkout-abc123", "/api/v1/pods/staging/web-xyz789"} {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status for %s = %d, want 200", path, rec.Code)
		}
	}

	body := scrape(t, p)
	if !strings.Contains(body, `/api/v1/pods/{namespace}/{pod}`) {
		t.Errorf("metrics missing the route template.\n%s", body)
	}
	for _, leaked := range []string{"checkout-abc123", "web-xyz789", "staging"} {
		if strings.Contains(body, leaked) {
			t.Errorf("metrics leaked the identifier %q into a label, which would blow up cardinality", leaked)
		}
	}
}

func TestMiddlewareRecordsStatusCode(t *testing.T) {
	p := newTestProvider(t)

	router := mux.NewRouter()
	router.Use(p.MuxRouteNamer())
	router.HandleFunc("/boom", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "kaboom", http.StatusInternalServerError)
	})

	rec := httptest.NewRecorder()
	p.HTTPMiddleware(router).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if body := scrape(t, p); !strings.Contains(body, `500`) {
		t.Errorf("metrics missing the 500 status label.\n%s", body)
	}
}

// TestMiddlewareSkipsNoiseEndpoints guards against the telemetry pipeline
// observing its own scrapes and the liveness probe.
func TestMiddlewareSkipsNoiseEndpoints(t *testing.T) {
	p := newTestProvider(t)

	var served []string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served = append(served, r.URL.Path)
		w.WriteHeader(http.StatusOK)
	})
	srv := p.HTTPMiddleware(next)

	for _, path := range []string{"/healthz", "/metrics"} {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s status = %d, want 200 — the request must still be served", path, rec.Code)
		}
	}
	if len(served) != 2 {
		t.Fatalf("handler saw %d requests, want 2 — skipping must not drop the request", len(served))
	}
	if body := scrape(t, p); strings.Contains(body, `http_route="/healthz"`) {
		t.Error("/healthz was recorded despite being on the skip list")
	}
}

// TestMiddlewareLabelsUnmatchedRoutes covers requests rejected before routing,
// such as the ones auth turns away. They must be counted, but under a fixed
// label rather than their raw path.
func TestMiddlewareLabelsUnmatchedRoutes(t *testing.T) {
	p := newTestProvider(t)

	// No router at all — nothing ever sets a route template.
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})

	rec := httptest.NewRecorder()
	p.HTTPMiddleware(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/secret/path/abc123", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	body := scrape(t, p)
	if !strings.Contains(body, "unmatched") {
		t.Errorf("expected the unmatched route label.\n%s", body)
	}
	if strings.Contains(body, "abc123") {
		t.Error("raw request path leaked into a metric label")
	}
}

// TestResponseRecorderPreservesFlush matters because the dashboard streams pod
// logs; a wrapper that swallows http.Flusher would buffer them indefinitely.
func TestResponseRecorderPreservesFlush(t *testing.T) {
	p := newTestProvider(t)

	flushed := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		f, ok := w.(http.Flusher)
		if !ok {
			t.Error("http.Flusher not available to the handler")
			return
		}
		_, _ = w.Write([]byte("chunk"))
		f.Flush()
		flushed = true
	})

	rec := httptest.NewRecorder()
	p.HTTPMiddleware(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/logs", nil))

	if !flushed {
		t.Error("handler could not flush through the telemetry wrapper")
	}
	if got := rec.Body.String(); got != "chunk" {
		t.Errorf("body = %q, want %q", got, "chunk")
	}
}

// hijackableRecorder is an httptest.ResponseRecorder that also supports
// hijacking, so the passthrough can be asserted without a real socket.
type hijackableRecorder struct {
	*httptest.ResponseRecorder
	hijacked bool
}

func (h *hijackableRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.hijacked = true
	client, server := net.Pipe()
	_ = server.Close()
	return client, bufio.NewReadWriter(bufio.NewReader(client), bufio.NewWriter(client)), nil
}

// TestResponseRecorderPreservesHijack matters because the dashboard proxies
// port-forwarded connections, which require taking over the socket.
func TestResponseRecorderPreservesHijack(t *testing.T) {
	p := newTestProvider(t)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		h, ok := w.(http.Hijacker)
		if !ok {
			t.Error("http.Hijacker not available to the handler")
			return
		}
		conn, _, err := h.Hijack()
		if err != nil {
			t.Errorf("Hijack() error = %v", err)
			return
		}
		_ = conn.Close()
	})

	rec := &hijackableRecorder{ResponseRecorder: httptest.NewRecorder()}
	p.HTTPMiddleware(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/forward/1/", nil))

	if !rec.hijacked {
		t.Error("Hijack did not reach the underlying ResponseWriter")
	}
}

// TestNilProviderMiddlewareIsPassthrough keeps the wiring safe for callers that
// never initialised telemetry.
func TestNilProviderMiddlewareIsPassthrough(t *testing.T) {
	var p *Provider
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	})

	rec := httptest.NewRecorder()
	p.HTTPMiddleware(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !called || rec.Code != http.StatusTeapot {
		t.Errorf("nil Provider did not pass the request through (called=%v, status=%d)", called, rec.Code)
	}
}
