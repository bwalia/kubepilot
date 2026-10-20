package ai

import (
	"strings"
	"testing"

	"github.com/kubepilot/kubepilot/pkg/k8s"
)

func TestFormatResourceMetrics_IncludesUnitNoteAndPercents(t *testing.T) {
	cpuPct := 0.1
	memPct := 26.2
	out := formatResourceMetrics(&k8s.ResourceMetrics{
		CPURequest:         "50m",
		CPULimit:           "1000m",
		CPUUsage:           "1m",
		MemRequest:         "64Mi",
		MemLimit:           "256Mi",
		MemUsage:           "67Mi",
		CPUUsageOfLimitPct: &cpuPct,
		MemUsageOfLimitPct: &memPct,
	})
	if !strings.Contains(out, "limit=1000m") || !strings.Contains(out, "usage=1m") {
		t.Fatalf("expected millicore strings, got:\n%s", out)
	}
	if !strings.Contains(out, "1000m = 1 core") {
		t.Fatalf("expected unit note, got:\n%s", out)
	}
	if !strings.Contains(out, "usage_of_limit=0.1%") {
		t.Fatalf("expected cpu percent, got:\n%s", out)
	}
	if strings.Contains(out, "limit=1\n") || strings.Contains(out, "limit=1 ") {
		t.Fatalf("must not emit bare core count that LLMs misread as 1m:\n%s", out)
	}
}

func TestIdentifyScenario_ProbeRestartStorm(t *testing.T) {
	diag := &k8s.PodDiagnostics{
		Phase: "Running",
		ContainerStatuses: []k8s.ContainerDiag{
			{Name: "njs", RestartCount: 1474, State: "Running"},
		},
		Events: []k8s.Event{
			{Type: "Warning", Reason: "Unhealthy", Message: `Liveness probe failed: Get "http://10.0.0.1:80/hello": context deadline exceeded`},
		},
	}
	if got := identifyScenario(diag); got != "ProbeFailure" {
		t.Fatalf("identifyScenario=%q want ProbeFailure", got)
	}
}

func TestIdentifyScenario_OOMStillWins(t *testing.T) {
	diag := &k8s.PodDiagnostics{
		ContainerStatuses: []k8s.ContainerDiag{
			{Name: "app", RestartCount: 20, LastTerminatedReason: "OOMKilled"},
		},
		Events: []k8s.Event{
			{Reason: "Unhealthy", Message: "Liveness probe failed"},
		},
	}
	if got := identifyScenario(diag); got != "OOMKilled" {
		t.Fatalf("identifyScenario=%q want OOMKilled", got)
	}
}
