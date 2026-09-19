package otelautopilot

import (
	"fmt"
	"strings"
)

// BuildEnablementPlan creates the zero/low-code actions for a covered app.
func BuildEnablementPlan(app AppCoverage, apply bool) EnablementPlan {
	plan := EnablementPlan{
		AppID:       app.ID,
		Capability:  app.Capability,
		Actions:     []string{},
		Annotations: map[string]string{},
		Applied:     false,
	}

	switch app.Capability {
	case CapabilityAutoInstr:
		key := fmt.Sprintf("instrumentation.opentelemetry.io/inject-%s", runtimeInjectKey(app.Runtime))
		plan.Annotations[key] = "true"
		plan.Annotations["kubepilot.io/otel-autopilot"] = "auto-instr"
		plan.Actions = append(plan.Actions,
			"annotate workload for OpenTelemetry auto-instrumentation",
			"ensure OTEL collector endpoint is reachable from the pod",
		)
		plan.Message = "Auto-instrumentation annotations ready for supported runtime"
	case CapabilityEBPF:
		plan.Annotations["kubepilot.io/otel-autopilot"] = "ebpf"
		plan.Actions = append(plan.Actions,
			"cover with eBPF agent DaemonSet (kernel/network/HTTP)",
			"forward spans/metrics to in-cluster OTEL collector",
		)
		plan.Message = "eBPF path preferred for this workload"
	case CapabilityCollector:
		plan.Annotations["kubepilot.io/otel-autopilot"] = "collector-only"
		plan.Actions = append(plan.Actions,
			"collect container logs via OTEL filelog receiver",
			"scrape kubelet/cAdvisor and service metrics when available",
		)
		plan.Message = "Collector-based logs/metrics without app code changes"
	case CapabilityNeedsConfig:
		plan.Actions = append(plan.Actions,
			"add minimal language inject annotation or OTEL_EXPORTER_OTLP_ENDPOINT env",
		)
		plan.Message = "Minimal configuration required before full traces"
	default:
		plan.Actions = append(plan.Actions, "manual SDK instrumentation is the last resort")
		plan.Message = "Unsupported for automatic enablement in this release"
	}

	if apply && (app.Capability == CapabilityAutoInstr || app.Capability == CapabilityEBPF || app.Capability == CapabilityCollector) {
		plan.Applied = true
		if plan.Message != "" {
			plan.Message = plan.Message + " (recorded as enabled in Autopilot ledger)"
		}
	}
	return plan
}

func runtimeInjectKey(runtime string) string {
	switch strings.ToLower(runtime) {
	case "nodejs", "node":
		return "nodejs"
	case "dotnet":
		return "dotnet"
	case "python":
		return "python"
	case "java":
		return "java"
	case "go":
		return "go"
	default:
		return runtime
	}
}

// AggregateCoverage builds CoverageReport totals from apps.
func AggregateCoverage(mode Mode, apps []AppCoverage) CoverageReport {
	report := CoverageReport{
		Mode: mode,
		Apps: apps,
	}
	for _, app := range apps {
		report.Totals.Total++
		if app.Enabled {
			report.Totals.Enabled++
		}
		switch app.Capability {
		case CapabilityEBPF:
			report.Totals.EBPF++
		case CapabilityAutoInstr:
			report.Totals.AutoInstr++
		case CapabilityCollector:
			report.Totals.Collector++
		case CapabilityNeedsConfig:
			report.Totals.NeedsConfig++
		default:
			report.Totals.Unsupported++
		}
	}
	return report
}
