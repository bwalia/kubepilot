// Package telemetry wires OpenTelemetry into KubePilot.
//
// It owns the process-wide trace and metric providers, the OTLP exporters that
// ship spans and metrics to a collector, and the Prometheus handler the
// dashboard serves on /metrics.
//
// Everything here is opt-in. With no OTLP endpoint configured the tracer is a
// no-op and metrics stay local to /metrics, so an unconfigured KubePilot opens
// no outbound telemetry connections and behaves exactly as it did before this
// package existed. That matters for the home-lab installs, where the binary
// often runs with no collector anywhere on the network.
package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/internal/version"
)

const (
	// ScopeName identifies KubePilot's own instrumentation on every span and
	// metric it emits, distinguishing them from library-generated telemetry.
	ScopeName = "github.com/kubepilot/kubepilot"

	// DefaultServiceName is the service.name reported when none is configured.
	DefaultServiceName = "kubepilot"

	// ProtocolGRPC and ProtocolHTTP are the supported OTLP wire protocols.
	// They mirror the values OTEL_EXPORTER_OTLP_PROTOCOL accepts.
	ProtocolGRPC = "grpc"
	ProtocolHTTP = "http/protobuf"

	defaultSampleRatio    = 1.0
	defaultMetricInterval = 60 * time.Second
	exportTimeout         = 15 * time.Second
	shutdownTimeout       = 5 * time.Second
)

// Config describes how this process exports telemetry.
//
// The zero value is valid and yields a fully disabled pipeline apart from the
// local Prometheus registry, which callers enable explicitly.
type Config struct {
	// ServiceName is reported as service.name. Defaults to "kubepilot".
	ServiceName string
	// ServiceNamespace groups several KubePilot deployments under one name,
	// e.g. "home-lab". Optional.
	ServiceNamespace string
	// Environment is reported as deployment.environment, e.g. "int" or "test".
	// Optional, but it is what makes the int/test/mac instances tellable apart
	// in a shared backend.
	Environment string

	// Endpoint is the OTLP collector endpoint. Empty disables OTLP export
	// entirely: traces become no-ops and metrics stay local to /metrics.
	Endpoint string
	// Protocol selects the OTLP wire protocol: "grpc" or "http/protobuf".
	Protocol string
	// Insecure sends OTLP over plaintext instead of TLS. Typical for a
	// collector reached over a trusted LAN or a cluster-local service.
	Insecure bool
	// Headers carries extra OTLP headers, such as the auth token a hosted
	// backend requires.
	Headers map[string]string

	// SampleRatio is the head-sampling probability applied to root spans,
	// from 0.0 (drop all) to 1.0 (keep all). Spans that arrive with a sampled
	// parent are always kept, so a distributed trace is never truncated.
	SampleRatio float64
	// MetricInterval is how often metrics are pushed over OTLP. It does not
	// affect Prometheus scraping, which is pull-based.
	MetricInterval time.Duration
	// PrometheusEnabled serves the collected metrics on the dashboard's
	// /metrics route in Prometheus exposition format.
	PrometheusEnabled bool
}

// Provider is an initialised OpenTelemetry pipeline.
type Provider struct {
	// Tracer and Meter are KubePilot's own instrumentation handles. They are
	// always non-nil — when telemetry is off they are backed by no-op SDKs, so
	// callers never need a nil check.
	Tracer trace.Tracer
	Meter  metric.Meter

	// Metrics holds the pre-declared KubePilot instruments.
	Metrics *Metrics

	// PrometheusHandler serves the Prometheus exposition format, or nil when
	// Prometheus scraping is disabled.
	PrometheusHandler http.Handler

	cfg            Config
	log            *zap.Logger
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
}

// TracingEnabled reports whether spans are actually exported. Callers can use
// it to skip expensive attribute construction that a no-op tracer would drop.
func (p *Provider) TracingEnabled() bool {
	return p != nil && p.tracerProvider != nil
}

// withDefaults fills in the settings the operator left unset and normalises the
// ones that only accept a fixed set of values.
func (c Config) withDefaults() Config {
	if strings.TrimSpace(c.ServiceName) == "" {
		c.ServiceName = DefaultServiceName
	}
	c.Endpoint = strings.TrimSpace(c.Endpoint)

	switch strings.ToLower(strings.TrimSpace(c.Protocol)) {
	case "", ProtocolGRPC:
		c.Protocol = ProtocolGRPC
	case "http", "http/protobuf", "httpprotobuf":
		c.Protocol = ProtocolHTTP
	default:
		// Unknown protocol: fall back to gRPC rather than refusing to start.
		// A typo in a config file should not take the server down.
		c.Protocol = ProtocolGRPC
	}

	// Clamp rather than reject — an out-of-range ratio is a config slip, and
	// silently exporting nothing is worse than exporting everything.
	if c.SampleRatio <= 0 || c.SampleRatio > 1 {
		c.SampleRatio = defaultSampleRatio
	}
	if c.MetricInterval <= 0 {
		c.MetricInterval = defaultMetricInterval
	}
	return c
}

// Init builds the pipeline described by cfg and installs it as the process-wide
// default. On a nil error the returned Provider is always usable; call Shutdown
// to flush buffered telemetry before the process exits.
func Init(ctx context.Context, cfg Config, log *zap.Logger) (*Provider, error) {
	cfg = cfg.withDefaults()

	// Route the SDK's own internal errors (export failures, dropped batches)
	// through our logger rather than its default stderr writer, so a collector
	// outage shows up alongside everything else the server reports.
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		log.Warn("OpenTelemetry export error", zap.Error(err))
	}))

	// Always install the W3C propagators. They cost nothing while tracing is
	// off, and they let KubePilot join a trace an upstream caller started.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	res := buildResource(ctx, cfg, log)

	p := &Provider{cfg: cfg, log: log}

	if err := p.initTracing(ctx, res); err != nil {
		return nil, err
	}
	if err := p.initMetrics(ctx, res); err != nil {
		// Tracing may already be up; tear it down so a partial failure never
		// leaves half a pipeline installed.
		p.shutdown(ctx)
		return nil, err
	}

	p.Tracer = otel.Tracer(ScopeName, trace.WithInstrumentationVersion(version.Version))
	p.Meter = otel.Meter(ScopeName, metric.WithInstrumentationVersion(version.Version))

	m, err := newMetrics(p.Meter)
	if err != nil {
		p.shutdown(ctx)
		return nil, fmt.Errorf("creating telemetry instruments: %w", err)
	}
	p.Metrics = m
	setDefault(p)

	log.Info("Telemetry initialised",
		zap.String("service", cfg.ServiceName),
		zap.String("environment", cfg.Environment),
		zap.Bool("traces", p.tracerProvider != nil),
		zap.Bool("otlp_metrics", cfg.Endpoint != ""),
		zap.Bool("prometheus", p.PrometheusHandler != nil),
		zap.String("endpoint", cfg.Endpoint),
		zap.String("protocol", cfg.Protocol),
		zap.Float64("sample_ratio", cfg.SampleRatio),
	)
	return p, nil
}

// buildResource describes this process to the backend: which service it is,
// which build, which host, and which environment.
//
// Resource detection is best-effort by design. A missing hostname or an
// unparseable OTEL_RESOURCE_ATTRIBUTES degrades the labels on the data, which
// is never a good enough reason to stop the server from starting.
func buildResource(ctx context.Context, cfg Config, log *zap.Logger) *resource.Resource {
	attrs := []attribute.KeyValue{
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(version.Version),
		semconv.ServiceInstanceID(instanceID()),
	}
	if ns := strings.TrimSpace(cfg.ServiceNamespace); ns != "" {
		attrs = append(attrs, semconv.ServiceNamespace(ns))
	}
	if env := strings.TrimSpace(cfg.Environment); env != "" {
		attrs = append(attrs, semconv.DeploymentEnvironment(env))
	}
	// Build provenance, so a span can be tied back to an exact binary.
	attrs = append(attrs,
		attribute.String("kubepilot.build_time", version.BuildTime),
		attribute.String("kubepilot.commit", version.Commit),
	)

	res, err := resource.New(ctx,
		// WithFromEnv honours OTEL_RESOURCE_ATTRIBUTES / OTEL_SERVICE_NAME, so
		// a Kubernetes deployment can inject pod and node identity via the
		// Downward API without KubePilot knowing anything about it.
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithProcessPID(),
		resource.WithProcessRuntimeVersion(),
		resource.WithAttributes(attrs...),
	)
	if err != nil {
		// resource.New still returns a usable resource alongside partial
		// errors; only a schema conflict yields nil.
		log.Warn("Partial OpenTelemetry resource detection", zap.Error(err))
	}
	if res == nil {
		res = resource.NewWithAttributes(semconv.SchemaURL, attrs...)
	}
	return res
}

// instanceID gives each running process a stable, human-recognisable identity,
// which is what separates the three KubePilot instances in a shared backend.
func instanceID() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown-host"
	}
	return fmt.Sprintf("%s-%d", host, os.Getpid())
}

// initTracing installs the span pipeline. With no endpoint configured it leaves
// the global no-op tracer in place, which is the normal state for a home-lab
// install with no collector on the network.
func (p *Provider) initTracing(ctx context.Context, res *resource.Resource) error {
	if p.cfg.Endpoint == "" {
		return nil
	}

	exporter, err := p.newTraceExporter(ctx)
	if err != nil {
		return fmt.Errorf("creating OTLP trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithExportTimeout(exportTimeout),
		),
		// ParentBased keeps a distributed trace intact: once an upstream caller
		// has sampled a trace, every KubePilot span in it is kept regardless of
		// the local ratio.
		sdktrace.WithSampler(sdktrace.ParentBased(
			sdktrace.TraceIDRatioBased(p.cfg.SampleRatio),
		)),
	)

	p.tracerProvider = tp
	otel.SetTracerProvider(tp)
	return nil
}

func (p *Provider) newTraceExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
	if p.cfg.Protocol == ProtocolHTTP {
		opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(p.cfg.Endpoint)}
		if p.cfg.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		if len(p.cfg.Headers) > 0 {
			opts = append(opts, otlptracehttp.WithHeaders(p.cfg.Headers))
		}
		return otlptracehttp.New(ctx, opts...)
	}

	opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(p.cfg.Endpoint)}
	if p.cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	if len(p.cfg.Headers) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(p.cfg.Headers))
	}
	return otlptracegrpc.New(ctx, opts...)
}

// initMetrics installs the metric pipeline, which may have up to two readers:
// a Prometheus reader for local scraping and a periodic OTLP reader for push.
// They are independent — a deployment can use either, both, or neither.
func (p *Provider) initMetrics(ctx context.Context, res *resource.Resource) error {
	var readers []sdkmetric.Reader

	if p.cfg.PrometheusEnabled {
		// A dedicated registry rather than the global default: KubePilot owns
		// what /metrics exposes, and nothing can register into it by accident
		// from an unrelated init() somewhere in the dependency tree.
		registry := prometheus.NewRegistry()
		registry.MustRegister(
			collectors.NewGoCollector(),
			collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		)

		reader, err := promexporter.New(promexporter.WithRegisterer(registry))
		if err != nil {
			return fmt.Errorf("creating Prometheus exporter: %w", err)
		}
		readers = append(readers, reader)
		p.PrometheusHandler = promhttp.HandlerFor(registry, promhttp.HandlerOpts{
			ErrorHandling: promhttp.ContinueOnError,
		})
	}

	if p.cfg.Endpoint != "" {
		exporter, err := p.newMetricExporter(ctx)
		if err != nil {
			return fmt.Errorf("creating OTLP metric exporter: %w", err)
		}
		readers = append(readers, sdkmetric.NewPeriodicReader(exporter,
			sdkmetric.WithInterval(p.cfg.MetricInterval),
			sdkmetric.WithTimeout(exportTimeout),
		))
	}

	if len(readers) == 0 {
		// Nothing to collect into. Leave the global no-op meter provider in
		// place; the instruments built from it are inert but safe to call.
		return nil
	}

	opts := []sdkmetric.Option{sdkmetric.WithResource(res)}
	for _, r := range readers {
		opts = append(opts, sdkmetric.WithReader(r))
	}
	mp := sdkmetric.NewMeterProvider(opts...)

	p.meterProvider = mp
	otel.SetMeterProvider(mp)
	return nil
}

func (p *Provider) newMetricExporter(ctx context.Context) (sdkmetric.Exporter, error) {
	if p.cfg.Protocol == ProtocolHTTP {
		opts := []otlpmetrichttp.Option{otlpmetrichttp.WithEndpoint(p.cfg.Endpoint)}
		if p.cfg.Insecure {
			opts = append(opts, otlpmetrichttp.WithInsecure())
		}
		if len(p.cfg.Headers) > 0 {
			opts = append(opts, otlpmetrichttp.WithHeaders(p.cfg.Headers))
		}
		return otlpmetrichttp.New(ctx, opts...)
	}

	opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(p.cfg.Endpoint)}
	if p.cfg.Insecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}
	if len(p.cfg.Headers) > 0 {
		opts = append(opts, otlpmetricgrpc.WithHeaders(p.cfg.Headers))
	}
	return otlpmetricgrpc.New(ctx, opts...)
}

// Shutdown flushes and stops the pipeline. It is safe to call on a nil Provider
// and safe to call more than once, so deferred cleanup never needs a guard.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil {
		return nil
	}
	return p.shutdown(ctx)
}

func (p *Provider) shutdown(ctx context.Context) error {
	// Shutdown runs on the way out, often from a context that is already
	// cancelled by the signal handler. Use a fresh bounded context so buffered
	// spans still get a chance to flush.
	flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()

	var firstErr error
	if p.tracerProvider != nil {
		if err := p.tracerProvider.Shutdown(flushCtx); err != nil {
			firstErr = fmt.Errorf("shutting down tracer provider: %w", err)
		}
		p.tracerProvider = nil
	}
	if p.meterProvider != nil {
		if err := p.meterProvider.Shutdown(flushCtx); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("shutting down meter provider: %w", err)
		}
		p.meterProvider = nil
	}
	return firstErr
}

// ─────────────────────────────────────────────────────────────────────────────
// Process-wide default
// ─────────────────────────────────────────────────────────────────────────────

var (
	defaultMu       sync.RWMutex
	defaultProvider *Provider
)

// Default returns the process-wide Provider that Init installed.
//
// Before Init runs — in the CLI subcommands that never set telemetry up, and in
// tests — it returns a Provider backed by OpenTelemetry's global no-op tracer
// and meter. Recording against it is safe and nearly free, which is what lets
// instrumented code skip a nil check at every call site.
//
// Init must therefore run before any instrumented code path, which `serve` does
// as its first step. Code that records before Init simply loses those data
// points; nothing fails.
func Default() *Provider {
	defaultMu.RLock()
	p := defaultProvider
	defaultMu.RUnlock()
	if p != nil {
		return p
	}

	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultProvider == nil {
		defaultProvider = newNoopProvider()
	}
	return defaultProvider
}

// setDefault installs p as the process-wide Provider.
func setDefault(p *Provider) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultProvider = p
}

// newNoopProvider builds a Provider over the global OpenTelemetry no-ops.
//
// otel.Tracer and otel.Meter return no-op implementations until a real provider
// is registered, so the instruments below are valid and inert.
func newNoopProvider() *Provider {
	p := &Provider{
		Tracer: otel.Tracer(ScopeName),
		Meter:  otel.Meter(ScopeName),
		log:    zap.NewNop(),
	}
	// newMetrics cannot fail against a no-op meter, but if it somehow did we
	// would rather run uninstrumented than panic during startup.
	if m, err := newMetrics(p.Meter); err == nil {
		p.Metrics = m
	} else {
		m, _ := newMetrics(otel.Meter(ScopeName))
		p.Metrics = m
	}
	return p
}
