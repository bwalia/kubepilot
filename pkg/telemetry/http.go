package telemetry

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// routeCtxKey addresses the per-request slot the mux middleware writes the
// matched route template into.
type routeCtxKey struct{}

// routeBox carries the matched route template back from inside the router to
// the outer middleware that owns the span and the metrics.
//
// It exists because the two facts are learned at different depths of the stack:
// the outer middleware sees every request (including the ones auth rejects
// before routing), but only the router knows that /api/v1/clusters/pods/foo/bar
// is really /clusters/pods/{namespace}/{pod}. Passing a pointer through the
// request context is what lets one span carry both. Each request gets its own
// box, written then read on a single goroutine, so no locking is needed.
type routeBox struct {
	template string
}

// HTTPMiddleware instruments every inbound request: one span, plus request
// duration, size, and in-flight counts.
//
// Wrap the outermost handler with it — outside auth and CORS — so rejected and
// preflight requests are recorded too. Those are exactly the ones worth seeing
// when a dashboard suddenly stops loading.
func (p *Provider) HTTPMiddleware(next http.Handler) http.Handler {
	if p == nil {
		return next
	}

	propagator := otel.GetTextMapPropagator()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// /metrics and /healthz are scraped and probed constantly. Tracing them
		// would bury real traffic in noise and, for /metrics, let the telemetry
		// pipeline observe itself.
		if isNoiseEndpoint(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()

		// Continue an upstream trace when the caller sent one.
		ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		box := &routeBox{}
		ctx = context.WithValue(ctx, routeCtxKey{}, box)

		// The span opens with the method alone; the route template is not known
		// until the router matches, and is filled in below.
		ctx, span := p.Tracer.Start(ctx, r.Method,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPRequestMethodKey.String(r.Method),
				semconv.URLPath(r.URL.Path),
				semconv.URLScheme(schemeOf(r)),
				semconv.ServerAddress(r.Host),
				semconv.UserAgentOriginal(r.UserAgent()),
				semconv.ClientAddress(clientAddr(r)),
			),
		)
		defer span.End()

		inflightAttrs := metric.WithAttributes(semconv.HTTPRequestMethodKey.String(r.Method))
		p.Metrics.HTTPActiveRequests.Add(ctx, 1, inflightAttrs)
		defer p.Metrics.HTTPActiveRequests.Add(ctx, -1, inflightAttrs)

		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r.WithContext(ctx))

		route := box.template
		if route == "" {
			// No route matched, or auth rejected the request before routing.
			// Reporting the raw path here would create unbounded metric
			// cardinality, so the series is deliberately left unlabelled.
			route = "unmatched"
		} else {
			span.SetName(r.Method + " " + route)
			span.SetAttributes(semconv.HTTPRoute(route))
		}

		span.SetAttributes(semconv.HTTPResponseStatusCode(rec.status))
		// Server errors mark the span as failed; 4xx is a client mistake and
		// leaving those unmarked keeps error dashboards about real faults.
		if rec.status >= 500 {
			span.SetStatus(codes.Error, http.StatusText(rec.status))
			span.SetAttributes(semconv.ErrorTypeKey.String(strconv.Itoa(rec.status)))
		}

		attrs := metric.WithAttributes(
			semconv.HTTPRequestMethodKey.String(r.Method),
			semconv.HTTPRoute(route),
			semconv.HTTPResponseStatusCode(rec.status),
		)
		p.Metrics.HTTPDuration.Record(ctx, time.Since(start).Seconds(), attrs)
		p.Metrics.HTTPResponseSize.Record(ctx, rec.written, attrs)
		if r.ContentLength > 0 {
			p.Metrics.HTTPRequestSize.Record(ctx, r.ContentLength, attrs)
		}
	})
}

// MuxRouteNamer reports the matched route template back to HTTPMiddleware.
//
// Register it on the router with router.Use(...). Without it every request is
// still traced, just labelled "unmatched" rather than by its route template.
func (p *Provider) MuxRouteNamer() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if box, ok := r.Context().Value(routeCtxKey{}).(*routeBox); ok {
				if route := mux.CurrentRoute(r); route != nil {
					if tmpl, err := route.GetPathTemplate(); err == nil {
						box.template = tmpl
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MetricsHandler serves the Prometheus exposition endpoint, or 404s when
// Prometheus scraping is disabled — so the route can be registered
// unconditionally and still behave honestly when it is switched off.
func (p *Provider) MetricsHandler() http.Handler {
	if p != nil && p.PrometheusHandler != nil {
		return p.PrometheusHandler
	}
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "metrics endpoint is disabled", http.StatusNotFound)
	})
}

// WrapTransport instruments an outbound HTTP transport so every call KubePilot
// makes — to the Kubernetes API server, to the AI backend — becomes a client
// span carrying the current trace context.
//
// A nil base means http.DefaultTransport, matching net/http's own convention.
func (p *Provider) WrapTransport(base http.RoundTripper, spanNameFor func(*http.Request) string) http.RoundTripper {
	if p == nil {
		return base
	}
	if base == nil {
		base = http.DefaultTransport
	}
	opts := []otelhttp.Option{}
	if spanNameFor != nil {
		opts = append(opts, otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return spanNameFor(r)
		}))
	}
	return otelhttp.NewTransport(base, opts...)
}

// WrapKubernetesTransport instruments the transport used for API-server calls.
//
// It layers two things over the base transport: otelhttp, which produces the
// client spans that make an API call visible inside a trace, and a recorder
// that reports latency by the resource being acted on.
//
// The second layer exists because otelhttp's own client metrics are labelled by
// server address and port. That answers "is the API server slow", but not
// "which of our calls is slow" — and with one API server behind every request,
// only the latter is actionable.
func (p *Provider) WrapKubernetesTransport(base http.RoundTripper) http.RoundTripper {
	if p == nil {
		return base
	}
	if base == nil {
		base = http.DefaultTransport
	}
	return &k8sTransport{
		base: p.WrapTransport(base, KubernetesSpanName),
		p:    p,
	}
}

// k8sTransport records each Kubernetes API call against the resource-aware
// instruments. It sits outside the otelhttp layer so the measured duration
// covers the whole call.
type k8sTransport struct {
	base http.RoundTripper
	p    *Provider
}

func (t *k8sTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := t.base.RoundTrip(r)

	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	t.p.RecordK8sCall(r.Context(), r.Method, KubernetesResource(r.URL.Path), status, time.Since(start), err)
	return resp, err
}

// RecordK8sCall records the latency and outcome of one Kubernetes API call.
//
// The status is reduced to its class (2xx, 4xx, 5xx) rather than the exact
// code, keeping the label set small while still separating "worked" from
// "rejected" from "server broke".
func (p *Provider) RecordK8sCall(ctx context.Context, method, resource string, status int, d time.Duration, err error) {
	if p == nil {
		return
	}
	if resource == "" {
		resource = "unknown"
	}
	attrs := []attribute.KeyValue{
		attribute.String("http.request.method", method),
		attribute.String("k8s.resource", resource),
		attribute.String("http.response.status_class", statusClass(status)),
	}
	p.Metrics.K8sDuration.Record(ctx, d.Seconds(), metric.WithAttributes(attrs...))

	// A transport error and an HTTP error are both failures worth counting, but
	// they fail differently: one never reached the API server, the other was
	// refused by it.
	switch {
	case err != nil:
		p.Metrics.K8sErrors.Add(ctx, 1, metric.WithAttributes(
			append(attrs, semconv.ErrorTypeKey.String(errorType(err)))...))
	case status >= 400:
		p.Metrics.K8sErrors.Add(ctx, 1, metric.WithAttributes(
			append(attrs, semconv.ErrorTypeKey.String(strconv.Itoa(status)))...))
	}
}

// statusClass buckets an HTTP status into its class. A zero status means the
// request never produced a response at all.
func statusClass(status int) string {
	switch {
	case status == 0:
		return "none"
	case status < 200:
		return "1xx"
	case status < 300:
		return "2xx"
	case status < 400:
		return "3xx"
	case status < 500:
		return "4xx"
	default:
		return "5xx"
	}
}

// responseRecorder captures the status code and body size the handler produced.
//
// It forwards Flush and Hijack to the underlying writer. That is not optional
// here: the dashboard streams pod logs and proxies port-forwarded connections,
// both of which break outright behind a wrapper that swallows those interfaces.
type responseRecorder struct {
	http.ResponseWriter
	status      int
	written     int64
	wroteHeader bool
}

func (r *responseRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.status = status
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		// net/http implies 200 on a bare Write; record the same.
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.written += int64(n)
	return n, err
}

func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("telemetry: underlying ResponseWriter does not support hijacking")
	}
	return h.Hijack()
}

// Unwrap lets net/http helpers such as http.ResponseController reach the
// original writer for interfaces this recorder does not implement.
func (r *responseRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func isNoiseEndpoint(path string) bool {
	return path == "/metrics" || path == "/healthz"
}

func schemeOf(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	// Honour the proxy's view when KubePilot sits behind an ingress.
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	return "http"
}

func clientAddr(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		// The left-most entry is the original client.
		if i := strings.IndexByte(fwd, ','); i >= 0 {
			return strings.TrimSpace(fwd[:i])
		}
		return strings.TrimSpace(fwd)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// errorType reduces an error to a bounded label. Using err.Error() would put
// pod names and namespaces into metric labels and blow up cardinality.
func errorType(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	default:
		return "error"
	}
}
