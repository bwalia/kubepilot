package autopilot

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/ai"
)

// fakeExecutor records the steps it was asked to run and returns a canned result.
type fakeExecutor struct {
	mu     sync.Mutex
	calls  []ai.RemediationStep
	result *ai.RemediationResult
	err    error
}

func (f *fakeExecutor) ExecuteStep(_ context.Context, step ai.RemediationStep, _, _ string) (*ai.RemediationResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, step)
	if f.result != nil {
		return f.result, f.err
	}
	return &ai.RemediationResult{Success: f.err == nil, Output: "ok"}, f.err
}

func (f *fakeExecutor) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func newController(t *testing.T, p Policy, fe *fakeExecutor) *Controller {
	t.Helper()
	c := &Controller{
		exec:       fe,
		log:        zap.NewNop(),
		now:        time.Now,
		policy:     p,
		maxLedger:  200,
		lastAction: make(map[string]time.Time),
	}
	return c
}

// crashLoopReport builds a high-confidence report whose safe, auto-applicable
// fix is to delete the failing pod (recreated by its controller).
func crashLoopReport(ns, pod string, confidence float64) *ai.RCAReport {
	return &ai.RCAReport{
		ID:             "rca-" + pod,
		TargetResource: ai.ResourceRef{Kind: "Pod", Namespace: ns, Name: pod},
		Severity:       ai.SeverityHigh,
		Confidence:     confidence,
		RootCause:      ai.RootCause{Category: "CrashLoop"},
		Remediation: []ai.RemediationStep{
			{Order: 1, Action: "delete_pod", Description: "recreate the crashing pod", Risk: "safe", AutoApply: true, RequiresCR: false},
		},
	}
}

func TestActiveModeExecutesSafeStep(t *testing.T) {
	fe := &fakeExecutor{}
	p := DefaultPolicy()
	p.Mode = ModeActive
	c := newController(t, p, fe)

	c.HandleReport(context.Background(), crashLoopReport("apps", "web-1", 0.95))

	if fe.count() != 1 {
		t.Fatalf("expected 1 execution, got %d", fe.count())
	}
	got := c.Decisions(1)
	if len(got) != 1 || got[0].Verdict != VerdictExecuted {
		t.Fatalf("expected executed verdict, got %+v", got)
	}
	if got := fe.calls[0].Command; got != "apps/web-1" {
		t.Fatalf("expected resolved command apps/web-1, got %q", got)
	}
}

// A small local model often marks the deployment-restart fix requires_cr=true
// (and gives an unresolvable free-text command). For a transient crash on an
// allow-listed namespace this must still recycle the offending pod via the
// safe delete_pod fallback rather than escalate — the namespace allow-list is
// the production boundary, not the model's per-step CR guess.
func TestRequiresCRRestartFallsBackToPodRecycle(t *testing.T) {
	fe := &fakeExecutor{}
	p := DefaultPolicy()
	p.Mode = ModeActive // delete_pod is in the default allow-list
	c := newController(t, p, fe)

	report := crashLoopReport("apps", "web-1", 0.95)
	report.Remediation = []ai.RemediationStep{
		{Order: 1, Action: "restart", Description: "rollout restart", Risk: "safe",
			AutoApply: false, RequiresCR: true, Command: "kubectl rollout restart deployment web"},
	}
	c.HandleReport(context.Background(), report)

	if fe.count() != 1 {
		t.Fatalf("expected 1 execution (pod recycle), got %d", fe.count())
	}
	if got := fe.calls[0].Action; got != "delete_pod" {
		t.Fatalf("expected delete_pod fallback, got %q", got)
	}
	if got := fe.calls[0].Command; got != "apps/web-1" {
		t.Fatalf("expected recycle target apps/web-1, got %q", got)
	}
	if v := c.Decisions(1)[0].Verdict; v != VerdictExecuted {
		t.Fatalf("expected executed, got %s", v)
	}
}

func TestDryRunModeNeverExecutes(t *testing.T) {
	fe := &fakeExecutor{}
	p := DefaultPolicy()
	p.Mode = ModeDryRun
	c := newController(t, p, fe)

	c.HandleReport(context.Background(), crashLoopReport("apps", "web-1", 0.95))

	if fe.count() != 0 {
		t.Fatalf("dry-run must not execute, got %d calls", fe.count())
	}
	if v := c.Decisions(1)[0].Verdict; v != VerdictDryRun {
		t.Fatalf("expected dry-run verdict, got %s", v)
	}
}

func TestOffModeSkips(t *testing.T) {
	fe := &fakeExecutor{}
	c := newController(t, DefaultPolicy(), fe) // DefaultPolicy is Off

	c.HandleReport(context.Background(), crashLoopReport("apps", "web-1", 0.99))

	if fe.count() != 0 {
		t.Fatalf("off mode must not execute")
	}
	if v := c.Decisions(1)[0].Verdict; v != VerdictSkipped {
		t.Fatalf("expected skipped, got %s", v)
	}
}

func TestLowConfidenceSkipped(t *testing.T) {
	fe := &fakeExecutor{}
	p := DefaultPolicy()
	p.Mode = ModeActive
	p.MinConfidence = 0.8
	c := newController(t, p, fe)

	c.HandleReport(context.Background(), crashLoopReport("apps", "web-1", 0.5))

	if fe.count() != 0 {
		t.Fatalf("low confidence must not execute")
	}
	if v := c.Decisions(1)[0].Verdict; v != VerdictSkipped {
		t.Fatalf("expected skipped, got %s", v)
	}
}

func TestBlockedNamespaceSkipped(t *testing.T) {
	fe := &fakeExecutor{}
	p := DefaultPolicy()
	p.Mode = ModeActive
	c := newController(t, p, fe)

	c.HandleReport(context.Background(), crashLoopReport("kube-system", "coredns-1", 0.99))

	if fe.count() != 0 {
		t.Fatalf("blocked namespace must not execute")
	}
}

func TestAllowedNamespacesRestricts(t *testing.T) {
	fe := &fakeExecutor{}
	p := DefaultPolicy()
	p.Mode = ModeActive
	p.AllowedNamespaces = []string{"prod"}
	c := newController(t, p, fe)

	c.HandleReport(context.Background(), crashLoopReport("staging", "web-1", 0.99))
	if fe.count() != 0 {
		t.Fatalf("namespace not in allow-list must not execute")
	}
	c.HandleReport(context.Background(), crashLoopReport("prod", "web-1", 0.99))
	if fe.count() != 1 {
		t.Fatalf("allow-listed namespace should execute, got %d", fe.count())
	}
}

func TestCRRequiredStepIsEscalated(t *testing.T) {
	fe := &fakeExecutor{}
	p := DefaultPolicy()
	p.Mode = ModeActive
	c := newController(t, p, fe)

	report := crashLoopReport("apps", "web-1", 0.99)
	report.Remediation = []ai.RemediationStep{
		{Order: 1, Action: "scale", Description: "scale down", Risk: "safe", AutoApply: true, RequiresCR: true, Command: "apps/web"},
	}
	c.HandleReport(context.Background(), report)

	if fe.count() != 0 {
		t.Fatalf("CR-required step must never auto-execute")
	}
	if v := c.Decisions(1)[0].Verdict; v != VerdictEscalated {
		t.Fatalf("expected escalated, got %s", v)
	}
}

func TestRiskCeilingEnforced(t *testing.T) {
	fe := &fakeExecutor{}
	p := DefaultPolicy()
	p.Mode = ModeActive
	p.MaxRisk = "safe"
	c := newController(t, p, fe)

	report := crashLoopReport("apps", "web-1", 0.99)
	report.Remediation[0].Risk = "high"
	c.HandleReport(context.Background(), report)

	if fe.count() != 0 {
		t.Fatalf("high-risk step must be skipped under safe ceiling")
	}
}

func TestActionAllowListEnforced(t *testing.T) {
	fe := &fakeExecutor{}
	p := DefaultPolicy()
	p.Mode = ModeActive
	p.AllowedActions = []string{"restart"} // delete_pod not allowed
	c := newController(t, p, fe)

	c.HandleReport(context.Background(), crashLoopReport("apps", "web-1", 0.99))
	if fe.count() != 0 {
		t.Fatalf("non-allow-listed action must be skipped")
	}
}

func TestCooldownPerResource(t *testing.T) {
	fe := &fakeExecutor{}
	p := DefaultPolicy()
	p.Mode = ModeActive
	p.Cooldown = time.Hour
	c := newController(t, p, fe)

	c.HandleReport(context.Background(), crashLoopReport("apps", "web-1", 0.99))
	c.HandleReport(context.Background(), crashLoopReport("apps", "web-1", 0.99))

	if fe.count() != 1 {
		t.Fatalf("second action within cooldown must be skipped, got %d", fe.count())
	}
	// A different resource is unaffected by the first one's cooldown.
	c.HandleReport(context.Background(), crashLoopReport("apps", "web-2", 0.99))
	if fe.count() != 2 {
		t.Fatalf("distinct resource should execute, got %d", fe.count())
	}
}

func TestRateLimitBlastRadius(t *testing.T) {
	fe := &fakeExecutor{}
	p := DefaultPolicy()
	p.Mode = ModeActive
	p.MaxActionsPerHour = 2
	p.Cooldown = 0
	c := newController(t, p, fe)

	for i := 0; i < 5; i++ {
		c.HandleReport(context.Background(), crashLoopReport("apps", "web-"+string(rune('a'+i)), 0.99))
	}
	if fe.count() != 2 {
		t.Fatalf("rate limit should cap at 2 actions, got %d", fe.count())
	}
}

func TestFailedExecutionRecorded(t *testing.T) {
	fe := &fakeExecutor{result: &ai.RemediationResult{Success: false, Error: "boom"}}
	p := DefaultPolicy()
	p.Mode = ModeActive
	c := newController(t, p, fe)

	c.HandleReport(context.Background(), crashLoopReport("apps", "web-1", 0.99))
	if v := c.Decisions(1)[0].Verdict; v != VerdictFailed {
		t.Fatalf("expected failed verdict, got %s", v)
	}
}

func TestWorkloadActionWithoutTargetEscalates(t *testing.T) {
	fe := &fakeExecutor{}
	p := DefaultPolicy()
	p.Mode = ModeActive
	p.AllowedActions = []string{"restart"}
	c := newController(t, p, fe)

	report := crashLoopReport("apps", "web-1", 0.99)
	report.Remediation = []ai.RemediationStep{
		// restart with no resolvable "ns/name" command -> can't safely target.
		{Order: 1, Action: "restart", Description: "rollout restart", Risk: "safe", AutoApply: true},
	}
	c.HandleReport(context.Background(), report)

	if fe.count() != 0 {
		t.Fatalf("unresolvable workload target must not execute")
	}
	if v := c.Decisions(1)[0].Verdict; v != VerdictEscalated {
		t.Fatalf("expected escalated, got %s", v)
	}
}

func TestPauseResumeKillSwitch(t *testing.T) {
	fe := &fakeExecutor{}
	p := DefaultPolicy()
	p.Mode = ModeActive
	c := newController(t, p, fe)

	// Pausing must halt execution immediately.
	c.Pause()
	if c.Policy().Mode != ModeOff {
		t.Fatalf("pause should set mode to off, got %s", c.Policy().Mode)
	}
	c.HandleReport(context.Background(), crashLoopReport("apps", "web-1", 0.99))
	if fe.count() != 0 {
		t.Fatalf("paused autopilot must not execute, got %d", fe.count())
	}

	// Resuming must restore the previous (active) mode and act again.
	c.Resume()
	if c.Policy().Mode != ModeActive {
		t.Fatalf("resume should restore active mode, got %s", c.Policy().Mode)
	}
	c.HandleReport(context.Background(), crashLoopReport("apps", "web-2", 0.99))
	if fe.count() != 1 {
		t.Fatalf("resumed autopilot should execute, got %d", fe.count())
	}
}

func TestResumeFromNeverRunningIsDryRun(t *testing.T) {
	c := newController(t, DefaultPolicy(), &fakeExecutor{}) // mode off, never paused
	if c.Resume().Mode != ModeDryRun {
		t.Fatalf("resume with no prior mode should default to dry-run, got %s", c.Policy().Mode)
	}
}

func TestSetMode(t *testing.T) {
	c := newController(t, DefaultPolicy(), &fakeExecutor{})
	if c.SetMode(ModeActive).Mode != ModeActive {
		t.Fatalf("SetMode(active) failed")
	}
	if c.Policy().Mode != ModeActive {
		t.Fatalf("mode not persisted")
	}
}

func TestParseNamespacedName(t *testing.T) {
	cases := []struct {
		in       string
		ns, name string
		ok       bool
	}{
		{"apps/web", "apps", "web", true},
		{" apps/web ", "apps", "web", true},
		{"web", "", "", false},
		{"/web", "", "", false},
		{"apps/", "", "", false},
		{"a/b/c", "", "", false},
	}
	for _, tc := range cases {
		ns, name, ok := parseNamespacedName(tc.in)
		if ok != tc.ok || ns != tc.ns || name != tc.name {
			t.Errorf("parseNamespacedName(%q) = (%q,%q,%v), want (%q,%q,%v)", tc.in, ns, name, ok, tc.ns, tc.name, tc.ok)
		}
	}
}
