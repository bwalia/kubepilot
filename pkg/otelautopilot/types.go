// Package otelautopilot implements KubePilot's automatic observability engine.
//
// OTLP Autopilot is not an OpenTelemetry configuration UI. It discovers
// applications on K3s clusters and enables Logs, Metrics, and Traces with
// zero or very-low code changes, using the fallback ladder:
//
//	eBPF → auto-instrumentation → collector-based telemetry → minimal config → manual SDK
//
// Hybrid storage: KubePilot-managed short-retention store powers the built-in
// UI by default; optional export dual-writes to Prometheus/Thanos/Cortex and
// external OTLP sinks.
package otelautopilot

import "time"

// Mode controls how aggressively Autopilot enables telemetry.
type Mode string

const (
	// ModeOff disables discovery and enablement.
	ModeOff Mode = "off"
	// ModeObserve discovers apps and scores coverage without mutating workloads.
	ModeObserve Mode = "observe"
	// ModeEnable discovers and applies zero/low-code enablement for the supported subset.
	ModeEnable Mode = "enable"
)

// Capability is where an app sits on the automatic-instrumentation ladder.
type Capability string

const (
	CapabilityEBPF         Capability = "ebpf"
	CapabilityAutoInstr    Capability = "auto-instr"
	CapabilityCollector    Capability = "collector-only"
	CapabilityNeedsConfig  Capability = "needs-config"
	CapabilityUnsupported  Capability = "unsupported"
)

// Plane separates cluster-level OTEL from application OTEL.
type Plane string

const (
	PlaneCluster Plane = "cluster"
	PlaneApp     Plane = "application"
)

// Policy is the set of guardrails for OTLP Autopilot.
type Policy struct {
	Mode Mode `json:"mode"`

	// ManagedStoreEnabled keeps the KubePilot short-retention store (default true).
	ManagedStoreEnabled bool `json:"managed_store_enabled"`

	// AllowedNamespaces, when non-empty, restricts discovery/enablement.
	AllowedNamespaces []string `json:"allowed_namespaces,omitempty"`
	// BlockedNamespaces are never touched.
	BlockedNamespaces []string `json:"blocked_namespaces"`

	// SupportedRuntimes lists runtimes eligible for auto-instrumentation.
	SupportedRuntimes []string `json:"supported_runtimes"`

	// Export holds optional Hybrid dual-write sinks (off by default).
	Export ExportConfig `json:"export"`

	// Retention is how long the managed store keeps signals.
	Retention time.Duration `json:"retention"`
}

// ExportConfig is the optional Hybrid export surface.
type ExportConfig struct {
	// MetricsRemoteWriteURL is a Prometheus/Thanos/Cortex remote_write endpoint.
	MetricsRemoteWriteURL string `json:"metrics_remote_write_url,omitempty"`
	// OTLPEndpoint is an optional external OTLP sink for logs/traces (and metrics).
	OTLPEndpoint string `json:"otlp_endpoint,omitempty"`
	// OTLPInsecure disables TLS for the optional OTLP sink (lab use).
	OTLPInsecure bool `json:"otlp_insecure,omitempty"`
}

// DefaultPolicy returns a conservative policy: observe-capable defaults, off mode.
func DefaultPolicy() Policy {
	return Policy{
		Mode:                ModeOff,
		ManagedStoreEnabled: true,
		BlockedNamespaces: []string{
			"kube-system", "kube-public", "kube-node-lease", "kubepilot-system",
		},
		SupportedRuntimes: []string{"java", "nodejs", "python", "dotnet", "go"},
		Retention:         24 * time.Hour,
	}
}

// AppCoverage describes one discovered workload and how Autopilot would cover it.
type AppCoverage struct {
	ID             string     `json:"id"`
	Kind           string     `json:"kind"`
	Name           string     `json:"name"`
	Namespace      string     `json:"namespace"`
	Runtime        string     `json:"runtime,omitempty"`
	Capability     Capability `json:"capability"`
	Reason         string     `json:"reason"`
	NextStep       string     `json:"next_step,omitempty"`
	Enabled        bool       `json:"enabled"`
	Signals        []string   `json:"signals"` // logs, metrics, traces
	Plane          Plane      `json:"plane"`
	LastSeen       time.Time  `json:"last_seen"`
	ServiceName    string     `json:"service_name,omitempty"`
	Image          string     `json:"image,omitempty"`
}

// CoverageReport is the cluster-wide enablement picture.
type CoverageReport struct {
	GeneratedAt time.Time     `json:"generated_at"`
	Mode        Mode          `json:"mode"`
	Totals      CoverageStats `json:"totals"`
	Apps        []AppCoverage `json:"apps"`
}

// CoverageStats aggregates capability counts.
type CoverageStats struct {
	Total        int `json:"total"`
	EBPF         int `json:"ebpf"`
	AutoInstr    int `json:"auto_instr"`
	Collector    int `json:"collector_only"`
	NeedsConfig  int `json:"needs_config"`
	Unsupported  int `json:"unsupported"`
	Enabled      int `json:"enabled"`
}

// EnablementPlan is what Autopilot would (or did) apply to a workload.
type EnablementPlan struct {
	AppID      string     `json:"app_id"`
	Capability Capability `json:"capability"`
	Actions    []string   `json:"actions"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Applied    bool       `json:"applied"`
	Message    string     `json:"message,omitempty"`
}

// SignalStatus summarises managed-store + export health.
type SignalStatus struct {
	ManagedStoreEnabled bool         `json:"managed_store_enabled"`
	MetricsCount        int          `json:"metrics_count"`
	LogsCount           int          `json:"logs_count"`
	TracesCount         int          `json:"traces_count"`
	Export              ExportConfig `json:"export"`
	ExportActive        bool         `json:"export_active"`
	LastIngestAt        *time.Time   `json:"last_ingest_at,omitempty"`
}

// Status is the dashboard/API payload for OTLP Autopilot.
type Status struct {
	Enabled  bool           `json:"enabled"`
	Policy   Policy         `json:"policy"`
	Coverage CoverageReport `json:"coverage"`
	Signals  SignalStatus   `json:"signals"`
	Plans    []EnablementPlan `json:"plans,omitempty"`
}
