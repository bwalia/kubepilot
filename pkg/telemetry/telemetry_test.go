package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestConfigWithDefaults(t *testing.T) {
	tests := []struct {
		name         string
		in           Config
		wantName     string
		wantProtocol string
		wantRatio    float64
		wantInterval time.Duration
	}{
		{
			name:         "empty config gets usable defaults",
			in:           Config{},
			wantName:     DefaultServiceName,
			wantProtocol: ProtocolGRPC,
			wantRatio:    1.0,
			wantInterval: 60 * time.Second,
		},
		{
			name:         "http protocol aliases normalise",
			in:           Config{Protocol: "http"},
			wantName:     DefaultServiceName,
			wantProtocol: ProtocolHTTP,
			wantRatio:    1.0,
			wantInterval: 60 * time.Second,
		},
		{
			name:         "unknown protocol falls back to grpc rather than failing",
			in:           Config{Protocol: "carrier-pigeon"},
			wantName:     DefaultServiceName,
			wantProtocol: ProtocolGRPC,
			wantRatio:    1.0,
			wantInterval: 60 * time.Second,
		},
		{
			name:         "out-of-range sample ratio is clamped to keep-everything",
			in:           Config{SampleRatio: 7},
			wantName:     DefaultServiceName,
			wantProtocol: ProtocolGRPC,
			wantRatio:    1.0,
			wantInterval: 60 * time.Second,
		},
		{
			name:         "valid values are preserved",
			in:           Config{ServiceName: "kp", Protocol: "grpc", SampleRatio: 0.25, MetricInterval: time.Second},
			wantName:     "kp",
			wantProtocol: ProtocolGRPC,
			wantRatio:    0.25,
			wantInterval: time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in.withDefaults()
			if got.ServiceName != tt.wantName {
				t.Errorf("ServiceName = %q, want %q", got.ServiceName, tt.wantName)
			}
			if got.Protocol != tt.wantProtocol {
				t.Errorf("Protocol = %q, want %q", got.Protocol, tt.wantProtocol)
			}
			if got.SampleRatio != tt.wantRatio {
				t.Errorf("SampleRatio = %v, want %v", got.SampleRatio, tt.wantRatio)
			}
			if got.MetricInterval != tt.wantInterval {
				t.Errorf("MetricInterval = %v, want %v", got.MetricInterval, tt.wantInterval)
			}
		})
	}
}

// TestInitWithoutEndpointMakesNoExporter is the property the home-lab installs
// depend on: with no collector configured, startup must succeed and open no
// outbound connection, while local metrics still work.
func TestInitWithoutEndpointMakesNoExporter(t *testing.T) {
	p, err := Init(context.Background(), Config{PrometheusEnabled: true}, zap.NewNop())
	if err != nil {
		t.Fatalf("Init() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	if p.TracingEnabled() {
		t.Error("TracingEnabled() = true with no endpoint configured, want false")
	}
	if p.PrometheusHandler == nil {
		t.Error("PrometheusHandler is nil, want a handler when Prometheus is enabled")
	}
	if p.Metrics == nil || p.Tracer == nil || p.Meter == nil {
		t.Fatal("Init returned a Provider with nil instruments")
	}
}

func TestMetricsHandlerServesExposition(t *testing.T) {
	p, err := Init(context.Background(), Config{PrometheusEnabled: true}, zap.NewNop())
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	rec := httptest.NewRecorder()
	p.MetricsHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	// The Go collector is registered at init, so its series prove the registry
	// is wired to the handler.
	if !strings.Contains(rec.Body.String(), "go_goroutines") {
		t.Error("exposition output missing go_goroutines; the Go collector is not registered")
	}
}

func TestMetricsHandlerDisabled404s(t *testing.T) {
	p, err := Init(context.Background(), Config{PrometheusEnabled: false}, zap.NewNop())
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	rec := httptest.NewRecorder()
	p.MetricsHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d when metrics are disabled", rec.Code, http.StatusNotFound)
	}
}

// TestShutdownIsSafeToRepeat covers the deferred-cleanup path, which may run
// after an earlier failure already tore the pipeline down.
func TestShutdownIsSafeToRepeat(t *testing.T) {
	p, err := Init(context.Background(), Config{PrometheusEnabled: true}, zap.NewNop())
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if err := p.Shutdown(context.Background()); err != nil {
		t.Errorf("first Shutdown() error = %v, want nil", err)
	}
	if err := p.Shutdown(context.Background()); err != nil {
		t.Errorf("second Shutdown() error = %v, want nil", err)
	}

	var nilProvider *Provider
	if err := nilProvider.Shutdown(context.Background()); err != nil {
		t.Errorf("nil Provider Shutdown() error = %v, want nil", err)
	}
}

func TestDefaultIsUsableBeforeInit(t *testing.T) {
	// Default must hand back something safe to record against, so that code
	// paths which never call Init (the other CLI subcommands, tests) do not
	// need nil checks.
	p := Default()
	if p == nil || p.Metrics == nil || p.Tracer == nil {
		t.Fatal("Default() returned an unusable Provider")
	}
	_, span := p.Tracer.Start(context.Background(), "probe")
	span.End()
	p.Metrics.RCAReports.Add(context.Background(), 1)
}
