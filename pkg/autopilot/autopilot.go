// Package autopilot implements KubePilot's closed-loop, AI-driven
// self-healing controller.
//
// The cluster watcher (pkg/observability) already detects anomalies and runs
// AI root-cause analysis (RCA) for failing pods. Autopilot is the missing
// final link: it consumes each finished RCAReport, decides — under a strict,
// auditable safety policy — whether KubePilot may resolve the issue itself,
// and (in active mode) executes the safe remediation step via the existing
// ai.RemediationExecutor.
//
// Safety model (defence in depth):
//   - Off by default. Operators must explicitly enable it.
//   - Three modes: off, dry-run (decide + log, never mutate), active (mutate).
//   - Confidence floor: low-confidence RCA never auto-acts.
//   - Action allow-list + max risk: only whitelisted, low-risk actions run.
//   - Namespace guardrails: system namespaces are blocked by default.
//   - Per-resource cooldown: prevents fighting a controller in a tight loop.
//   - Global rate limit (blast radius): caps actions per rolling hour.
//   - CR-code boundary: any step flagged RequiresCR is escalated to a human,
//     never auto-executed — autopilot never bypasses change control.
//
// Every evaluation — executed, dry-run, skipped, escalated or failed — is
// recorded as a Decision in an in-memory ledger for full auditability.
package autopilot

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/ai"
)

// Mode controls how aggressively the controller acts.
type Mode string

const (
	// ModeOff disables the controller entirely.
	ModeOff Mode = "off"
	// ModeDryRun evaluates every report and records what it *would* do,
	// but never mutates the cluster. This is the safe default for rollout.
	ModeDryRun Mode = "dry-run"
	// ModeActive evaluates and actually executes safe remediation steps.
	ModeActive Mode = "active"
)

// Verdict is the outcome of evaluating a single RCA report.
type Verdict string

const (
	VerdictExecuted  Verdict = "executed"  // action applied to the cluster
	VerdictDryRun    Verdict = "dry-run"   // would act, but mode is dry-run
	VerdictSkipped   Verdict = "skipped"   // policy declined to act
	VerdictEscalated Verdict = "escalated" // needs a human (CR code / manual)
	VerdictFailed    Verdict = "failed"    // attempted but the action errored
)

// riskRank orders remediation risk levels so the policy can enforce a ceiling.
var riskRank = map[string]int{"safe": 0, "moderate": 1, "high": 2}

// Policy is the set of guardrails the controller enforces. The zero value is
// not useful; build one with DefaultPolicy and override fields as needed.
type Policy struct {
	Mode Mode `json:"mode"`

	// MinConfidence is the lowest RCA confidence (0.0–1.0) eligible for action.
	MinConfidence float64 `json:"min_confidence"`

	// AllowedActions is the whitelist of remediation actions the controller may
	// auto-apply (e.g. "delete_pod", "restart", "scale").
	AllowedActions []string `json:"allowed_actions"`

	// MaxRisk is the highest step risk the controller may auto-apply:
	// "safe", "moderate" or "high".
	MaxRisk string `json:"max_risk"`

	// AllowedNamespaces, when non-empty, restricts action to these namespaces.
	AllowedNamespaces []string `json:"allowed_namespaces,omitempty"`
	// BlockedNamespaces are never touched (defaults to cluster system namespaces).
	BlockedNamespaces []string `json:"blocked_namespaces"`

	// Cooldown is the minimum time between autopilot actions on the same resource.
	Cooldown time.Duration `json:"cooldown"`

	// MaxActionsPerHour caps the number of real actions in any rolling hour.
	MaxActionsPerHour int `json:"max_actions_per_hour"`
}

// DefaultPolicy returns a conservative, production-safe policy. It is disabled
// (ModeOff) so that simply constructing a controller never changes behaviour.
func DefaultPolicy() Policy {
	return Policy{
		Mode:              ModeOff,
		MinConfidence:     0.80,
		AllowedActions:    []string{"delete_pod", "restart"},
		MaxRisk:           "safe",
		BlockedNamespaces: []string{"kube-system", "kube-public", "kube-node-lease", "kubepilot-system"},
		Cooldown:          10 * time.Minute,
		MaxActionsPerHour: 10,
	}
}

// Decision is a single, auditable record of an autopilot evaluation.
type Decision struct {
	Time       time.Time      `json:"time"`
	ReportID   string         `json:"report_id"`
	Resource   ai.ResourceRef `json:"resource"`
	Severity   string         `json:"severity"`
	Confidence float64        `json:"confidence"`
	RootCause  string         `json:"root_cause"`
	Action     string         `json:"action,omitempty"`
	Verdict    Verdict        `json:"verdict"`
	Reason     string         `json:"reason"`
	Output     string         `json:"output,omitempty"`
}

// executor is the subset of ai.RemediationExecutor the controller relies on.
// Declared as an interface so tests can substitute a fake without a cluster.
type executor interface {
	ExecuteStep(ctx context.Context, step ai.RemediationStep, changeID, crCode string) (*ai.RemediationResult, error)
}

// Controller is the closed-loop self-healing engine.
type Controller struct {
	exec executor
	log  *zap.Logger
	now  func() time.Time

	mu          sync.Mutex
	policy      Policy
	resumeMode  Mode       // mode to restore after a Pause (kill switch)
	decisions   []Decision // ring buffer, newest last
	maxLedger   int
	lastAction  map[string]time.Time // resource key -> last real action
	actionTimes []time.Time          // timestamps of real actions, for rate limiting
}

// Config configures a new Controller.
type Config struct {
	Policy   Policy
	Executor *ai.RemediationExecutor
}

// New creates a Controller. The executor should be configured with DryRun=false;
// the controller itself decides whether a step is actually applied based on Mode.
func New(cfg Config, log *zap.Logger) *Controller {
	return &Controller{
		exec:       cfg.Executor,
		log:        log,
		now:        time.Now,
		policy:     cfg.Policy,
		maxLedger:  200,
		lastAction: make(map[string]time.Time),
	}
}

// Policy returns a copy of the current policy.
func (c *Controller) Policy() Policy {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.policy
}

// Pause is the global kill switch: it immediately stops all self-healing by
// switching to ModeOff, remembering the previous mode so Resume can restore it.
// It is idempotent and safe to call at any time.
func (c *Controller) Pause() Policy {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.policy.Mode != ModeOff {
		c.resumeMode = c.policy.Mode
		c.policy.Mode = ModeOff
		c.log.Warn("autopilot paused (kill switch engaged)", zap.String("previous_mode", string(c.resumeMode)))
	}
	return c.policy
}

// Resume restores the mode that was active before the last Pause. If autopilot
// was never running, it conservatively resumes into dry-run rather than active.
func (c *Controller) Resume() Policy {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.policy.Mode == ModeOff {
		m := c.resumeMode
		if m == "" || m == ModeOff {
			m = ModeDryRun
		}
		c.policy.Mode = m
		c.log.Info("autopilot resumed", zap.String("mode", string(m)))
	}
	return c.policy
}

// SetMode explicitly sets the operating mode (off, dry-run or active).
func (c *Controller) SetMode(m Mode) Policy {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.policy.Mode = m
	c.log.Info("autopilot mode changed", zap.String("mode", string(m)))
	return c.policy
}

// Decisions returns the most recent decisions, newest first, capped at limit
// (limit <= 0 returns all retained decisions).
func (c *Controller) Decisions(limit int) []Decision {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := len(c.decisions)
	out := make([]Decision, 0, n)
	for i := n - 1; i >= 0; i-- {
		out = append(out, c.decisions[i])
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// Stats summarises the ledger by verdict for the dashboard.
func (c *Controller) Stats() map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	stats := map[string]int{}
	for _, d := range c.decisions {
		stats[string(d.Verdict)]++
	}
	stats["actions_last_hour"] = c.countRecentActionsLocked()
	return stats
}

// HandleReport is the watcher hook: it evaluates one finished RCA report and,
// in active mode, executes the chosen remediation. It never blocks the watcher
// for long and never panics out into the caller.
func (c *Controller) HandleReport(ctx context.Context, report *ai.RCAReport) {
	if report == nil {
		return
	}

	decision := c.evaluate(report)

	// Execute only when the evaluation says we may act in active mode.
	if decision.Verdict == VerdictExecuted {
		decision = c.execute(ctx, report, decision)
	}

	c.record(decision)
	c.log.Info("autopilot decision",
		zap.String("report_id", decision.ReportID),
		zap.String("resource", decision.Resource.Namespace+"/"+decision.Resource.Name),
		zap.String("action", decision.Action),
		zap.String("verdict", string(decision.Verdict)),
		zap.String("reason", decision.Reason),
	)
}

// evaluate applies the policy to a report and returns a provisional Decision.
// A Verdict of VerdictExecuted here means "cleared to execute in active mode";
// HandleReport then performs the side effect via execute. This function is pure
// with respect to the cluster (no mutations) so it is straightforward to test.
func (c *Controller) evaluate(report *ai.RCAReport) Decision {
	c.mu.Lock()
	defer c.mu.Unlock()

	d := Decision{
		Time:       c.now().UTC(),
		ReportID:   report.ID,
		Resource:   report.TargetResource,
		Severity:   string(report.Severity),
		Confidence: report.Confidence,
		RootCause:  report.RootCause.Category,
	}

	if c.policy.Mode == ModeOff {
		d.Verdict, d.Reason = VerdictSkipped, "autopilot disabled"
		return d
	}

	if !c.namespaceAllowed(report.TargetResource.Namespace) {
		d.Verdict = VerdictSkipped
		d.Reason = fmt.Sprintf("namespace %q is not eligible for autopilot", report.TargetResource.Namespace)
		return d
	}

	if report.Confidence < c.policy.MinConfidence {
		d.Verdict = VerdictSkipped
		d.Reason = fmt.Sprintf("confidence %.2f below threshold %.2f", report.Confidence, c.policy.MinConfidence)
		return d
	}

	step, why := c.selectStep(report)
	if step == nil {
		// Distinguish "nothing applicable" from "needs a human".
		if why == reasonNeedsHuman {
			d.Verdict = VerdictEscalated
			d.Reason = "remediation requires change-control approval or manual action — escalated"
		} else {
			d.Verdict = VerdictSkipped
			d.Reason = string(why)
		}
		return d
	}
	d.Action = step.Action

	// Per-resource cooldown to avoid fighting a reconciling controller.
	key := resourceKey(report.TargetResource)
	if last, ok := c.lastAction[key]; ok {
		if since := c.now().Sub(last); since < c.policy.Cooldown {
			d.Verdict = VerdictSkipped
			d.Reason = fmt.Sprintf("cooldown active (%s since last action, need %s)", since.Round(time.Second), c.policy.Cooldown)
			return d
		}
	}

	// Global blast-radius cap.
	if c.policy.MaxActionsPerHour > 0 && c.countRecentActionsLocked() >= c.policy.MaxActionsPerHour {
		d.Verdict = VerdictSkipped
		d.Reason = fmt.Sprintf("rate limit reached (%d actions/hour)", c.policy.MaxActionsPerHour)
		return d
	}

	if c.policy.Mode == ModeDryRun {
		d.Verdict = VerdictDryRun
		d.Reason = fmt.Sprintf("would %s %s/%s (dry-run mode)", step.Action, report.TargetResource.Namespace, report.TargetResource.Name)
		return d
	}

	// Active mode: cleared to execute. Reserve a rate-limit slot and cooldown now
	// so concurrent reports can't race past the caps before execute() runs.
	d.Verdict = VerdictExecuted
	d.Reason = fmt.Sprintf("auto-remediating: %s", step.Description)
	c.actionTimes = append(c.actionTimes, c.now())
	c.lastAction[key] = c.now()
	return d
}

// skipReason is a typed reason used to distinguish escalation from a plain skip.
type skipReason string

const reasonNeedsHuman skipReason = "needs-human"

// selectStep picks the first remediation step that satisfies the policy, or nil.
// It returns reasonNeedsHuman when the only available fixes require a human
// (CR code or manual), so the caller can escalate rather than silently skip.
func (c *Controller) selectStep(report *ai.RCAReport) (*ai.RemediationStep, skipReason) {
	steps := make([]ai.RemediationStep, len(report.Remediation))
	copy(steps, report.Remediation)
	sort.SliceStable(steps, func(i, j int) bool { return steps[i].Order < steps[j].Order })

	sawHumanGated := false
	for i := range steps {
		step := steps[i]
		if step.RequiresCR || step.Action == "manual" || step.Action == "rollback" {
			sawHumanGated = true
			continue
		}
		if !step.AutoApply {
			continue
		}
		if !c.actionAllowed(step.Action) {
			continue
		}
		if !c.riskWithinCeiling(step.Risk) {
			continue
		}
		resolved, ok := c.resolveTarget(report.TargetResource, step)
		if !ok {
			// We can't safely determine the target — treat as needing a human.
			sawHumanGated = true
			continue
		}
		return &resolved, ""
	}

	if sawHumanGated {
		return nil, reasonNeedsHuman
	}
	return nil, "no auto-applicable remediation step"
}

// resolveTarget fills in step.Command with a concrete "namespace/name" target
// so the executor can act. For pod-scoped actions the RCA target pod is used;
// for workload actions an explicit "ns/name" command from the LLM is required.
func (c *Controller) resolveTarget(target ai.ResourceRef, step ai.RemediationStep) (ai.RemediationStep, bool) {
	switch step.Action {
	case "delete_pod":
		if target.Namespace == "" || target.Name == "" {
			return step, false
		}
		step.Command = target.Namespace + "/" + target.Name
		return step, true
	case "restart", "scale":
		// These act on a workload (Deployment), not the pod. Only proceed when the
		// RCA supplied an explicit "namespace/name" target we can trust.
		if ns, name, ok := parseNamespacedName(step.Command); ok {
			step.Command = ns + "/" + name
			return step, true
		}
		return step, false
	default:
		return step, false
	}
}

// execute applies the selected step. cleared must be a decision with
// VerdictExecuted produced by evaluate (which already reserved the rate slot).
func (c *Controller) execute(ctx context.Context, report *ai.RCAReport, cleared Decision) Decision {
	step, _ := c.selectStep(report) // re-resolve target; policy already passed
	if step == nil {
		cleared.Verdict = VerdictSkipped
		cleared.Reason = "remediation step no longer applicable at execution time"
		return cleared
	}

	res, err := c.exec.ExecuteStep(ctx, *step, "", "")
	if err != nil || (res != nil && !res.Success) {
		cleared.Verdict = VerdictFailed
		if err != nil {
			cleared.Reason = "remediation failed: " + err.Error()
		} else {
			cleared.Reason = "remediation failed: " + res.Error
		}
		return cleared
	}

	cleared.Verdict = VerdictExecuted
	if res != nil {
		cleared.Output = res.Output
	}
	cleared.Reason = fmt.Sprintf("auto-remediated %s/%s via %s", report.TargetResource.Namespace, report.TargetResource.Name, step.Action)
	return cleared
}

// ── policy helpers ──────────────────────────────────────────────────────────

func (c *Controller) namespaceAllowed(ns string) bool {
	for _, b := range c.policy.BlockedNamespaces {
		if strings.EqualFold(b, ns) {
			return false
		}
	}
	if len(c.policy.AllowedNamespaces) == 0 {
		return true
	}
	for _, a := range c.policy.AllowedNamespaces {
		if strings.EqualFold(a, ns) {
			return true
		}
	}
	return false
}

func (c *Controller) actionAllowed(action string) bool {
	for _, a := range c.policy.AllowedActions {
		if strings.EqualFold(a, action) {
			return true
		}
	}
	return false
}

func (c *Controller) riskWithinCeiling(risk string) bool {
	ceiling, ok := riskRank[strings.ToLower(c.policy.MaxRisk)]
	if !ok {
		ceiling = 0 // default to "safe" if misconfigured
	}
	r, ok := riskRank[strings.ToLower(risk)]
	if !ok {
		r = riskRank["moderate"] // unknown risk treated as moderate, not safe
	}
	return r <= ceiling
}

// countRecentActionsLocked counts real actions within the last hour and prunes
// older entries. Caller must hold c.mu.
func (c *Controller) countRecentActionsLocked() int {
	cutoff := c.now().Add(-time.Hour)
	kept := c.actionTimes[:0]
	for _, t := range c.actionTimes {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	c.actionTimes = kept
	return len(c.actionTimes)
}

func (c *Controller) record(d Decision) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.decisions = append(c.decisions, d)
	if len(c.decisions) > c.maxLedger {
		c.decisions = c.decisions[len(c.decisions)-c.maxLedger:]
	}
}

func resourceKey(r ai.ResourceRef) string {
	return r.Kind + "/" + r.Namespace + "/" + r.Name
}

// parseNamespacedName parses "namespace/name", rejecting empty parts.
func parseNamespacedName(s string) (namespace, name string, ok bool) {
	s = strings.TrimSpace(s)
	i := strings.IndexByte(s, '/')
	if i <= 0 || i == len(s)-1 {
		return "", "", false
	}
	ns, n := s[:i], s[i+1:]
	if strings.ContainsAny(n, "/ ") {
		return "", "", false
	}
	return ns, n, true
}
