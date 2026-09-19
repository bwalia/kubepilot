package otelautopilot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kubepilot/kubepilot/pkg/k8s"
)

// Discoverer finds workloads and scores automatic-telemetry capability.
type Discoverer struct {
	k8s               *k8s.Client
	supportedRuntimes map[string]struct{}
	blocked           map[string]struct{}
	allowed           map[string]struct{}
}

// NewDiscoverer builds a discoverer from policy lists.
func NewDiscoverer(client *k8s.Client, supported, allowed, blocked []string) *Discoverer {
	d := &Discoverer{
		k8s:               client,
		supportedRuntimes: toSet(supported),
		blocked:           toSet(blocked),
		allowed:           toSet(allowed),
	}
	if len(d.supportedRuntimes) == 0 {
		d.supportedRuntimes = toSet([]string{"java", "nodejs", "python", "dotnet", "go"})
	}
	return d
}

func toSet(items []string) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.ToLower(strings.TrimSpace(item))
		if item != "" {
			out[item] = struct{}{}
		}
	}
	return out
}

func (d *Discoverer) namespaceAllowed(ns string) bool {
	if _, blocked := d.blocked[strings.ToLower(ns)]; blocked {
		return false
	}
	if len(d.allowed) == 0 {
		return true
	}
	_, ok := d.allowed[strings.ToLower(ns)]
	return ok
}

// Discover returns coverage for Deployments, StatefulSets, and DaemonSets.
func (d *Discoverer) Discover(ctx context.Context) ([]AppCoverage, error) {
	if d.k8s == nil {
		return nil, fmt.Errorf("kubernetes client is nil")
	}
	now := time.Now().UTC()
	apps := make([]AppCoverage, 0, 64)

	deps, err := d.k8s.ListDeployments(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("listing deployments: %w", err)
	}
	for _, dep := range deps {
		if !d.namespaceAllowed(dep.Namespace) {
			continue
		}
		apps = append(apps, d.scoreWorkload("Deployment", dep.Namespace, dep.Name, dep.Image, now))
	}

	sts, err := d.k8s.ListStatefulSets(ctx, "")
	if err == nil {
		for _, st := range sts {
			if !d.namespaceAllowed(st.Namespace) {
				continue
			}
			apps = append(apps, d.scoreWorkload("StatefulSet", st.Namespace, st.Name, st.Image, now))
		}
	}

	dss, err := d.k8s.ListDaemonSets(ctx, "")
	if err == nil {
		for _, ds := range dss {
			if !d.namespaceAllowed(ds.Namespace) {
				continue
			}
			apps = append(apps, d.scoreWorkload("DaemonSet", ds.Namespace, ds.Name, ds.Image, now))
		}
	}

	return apps, nil
}

func (d *Discoverer) scoreWorkload(kind, namespace, name, image string, now time.Time) AppCoverage {
	runtime := detectRuntime(image)
	cap, reason, next, signals := d.scoreRuntime(runtime, image)
	id := fmt.Sprintf("%s/%s/%s", strings.ToLower(kind), namespace, name)
	enabled := false
	// Treat already-annotated-looking images / otel sidecars as enabled later;
	// for MVP, "enabled" is set by the controller after applying a plan.
	return AppCoverage{
		ID:          id,
		Kind:        kind,
		Name:        name,
		Namespace:   namespace,
		Runtime:     runtime,
		Capability:  cap,
		Reason:      reason,
		NextStep:    next,
		Enabled:     enabled,
		Signals:     signals,
		Plane:       PlaneApp,
		LastSeen:    now,
		ServiceName: name,
		Image:       image,
	}
}

func (d *Discoverer) scoreRuntime(runtime, image string) (Capability, string, string, []string) {
	signals := []string{"logs", "metrics"}
	lowerImg := strings.ToLower(image)

	// Collector can always scrape kube metadata + container logs.
	if runtime == "" || runtime == "unknown" {
		return CapabilityCollector, "Runtime not detected; collector-based logs/metrics still available",
			"Annotate the workload with language.opentelemetry.io/inject-<runtime>=true when ready for traces",
			[]string{"logs", "metrics"}
	}

	if _, ok := d.supportedRuntimes[runtime]; ok {
		// Go and many static binaries benefit most from eBPF first.
		if runtime == "go" || strings.Contains(lowerImg, "scratch") || strings.Contains(lowerImg, "distroless") {
			return CapabilityEBPF, "Supported via eBPF / collector for zero-code network and runtime signals",
				"OTLP Autopilot will prefer eBPF; auto-instr used when available",
				append(signals, "traces")
		}
		return CapabilityAutoInstr, fmt.Sprintf("Supported runtime %q — auto-instrumentation can inject OTEL without code changes", runtime),
			"OTLP Autopilot will apply language.opentelemetry.io inject annotations",
			append(signals, "traces")
	}

	if runtime == "nginx" || runtime == "redis" || runtime == "postgres" {
		return CapabilityCollector, "Infrastructure component — collector/eBPF metrics and logs, limited app traces",
			"Use collector scrapes; manual SDK only if you need deep app traces",
			signals
	}

	return CapabilityNeedsConfig, fmt.Sprintf("Runtime %q needs a minimal annotation or sidecar to enable traces", runtime),
		"Add opentelemetry.io language inject annotation or enable collector scrape",
		signals
}

func detectRuntime(image string) string {
	img := strings.ToLower(image)
	switch {
	case strings.Contains(img, "java"), strings.Contains(img, "jdk"), strings.Contains(img, "jre"), strings.Contains(img, "openjdk"), strings.Contains(img, "temurin"):
		return "java"
	case strings.Contains(img, "node"), strings.Contains(img, "nodejs"):
		return "nodejs"
	case strings.Contains(img, "python"), strings.Contains(img, "django"), strings.Contains(img, "flask"):
		return "python"
	case strings.Contains(img, "dotnet"), strings.Contains(img, "aspnet"):
		return "dotnet"
	case strings.Contains(img, "golang"), strings.Contains(img, "go-"):
		return "go"
	case strings.Contains(img, "nginx"):
		return "nginx"
	case strings.Contains(img, "redis"):
		return "redis"
	case strings.Contains(img, "postgres"), strings.Contains(img, "pgsql"):
		return "postgres"
	default:
		return "unknown"
	}
}
