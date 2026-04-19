// Package runbooks provides pre-built, opinionated automation workflows
// for common Kubernetes operations. Each runbook is a sequence of human-readable
// steps paired with automated actions that map to k8s operations or AI analysis.
package runbooks

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubepilot/kubepilot/pkg/ai"
	"github.com/kubepilot/kubepilot/pkg/k8s"
)

// Runbook is a named automation workflow with ordered steps and corresponding actions.
type Runbook struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"` // diagnostic, recovery, rollback, health, migration
	Steps       []string `json:"steps"`
	Actions     []string `json:"actions"`
	Risk        string   `json:"risk"` // low, medium, high
}

// StepResult is returned from executing a single runbook step.
type StepResult struct {
	Status  string `json:"status"` // ok, error, manual
	Message string `json:"message"`
}

// Engine executes runbook actions against the cluster and AI subsystems.
type Engine struct {
	mu   sync.RWMutex
	k8s  *k8s.Client
	ai   *ai.Engine
	log  *zap.Logger
}

// NewEngine constructs a runbook execution engine.
func NewEngine(k8sClient *k8s.Client, aiEngine *ai.Engine, log *zap.Logger) *Engine {
	return &Engine{k8s: k8sClient, ai: aiEngine, log: log}
}

// SetK8sClient swaps the active k8s client (for cluster context switching).
func (e *Engine) SetK8sClient(c *k8s.Client) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.k8s = c
}

// List returns all registered runbooks.
func (e *Engine) List() []Runbook {
	return Builtin()
}

// Get returns a runbook by ID.
func (e *Engine) Get(id string) (Runbook, bool) {
	for _, rb := range Builtin() {
		if rb.ID == id {
			return rb, true
		}
	}
	return Runbook{}, false
}

// Execute runs the action for the given step index of the runbook.
// If the step has no associated action, returns a manual-completion result.
func (e *Engine) Execute(ctx context.Context, runbookID string, step int, params map[string]string) StepResult {
	rb, ok := e.Get(runbookID)
	if !ok {
		return StepResult{Status: "error", Message: fmt.Sprintf("runbook %q not found", runbookID)}
	}
	if step < 0 || step >= len(rb.Steps) {
		return StepResult{Status: "error", Message: fmt.Sprintf("step %d out of range (0-%d)", step, len(rb.Steps)-1)}
	}
	if step >= len(rb.Actions) || rb.Actions[step] == "" || rb.Actions[step] == "manual" {
		return StepResult{Status: "manual", Message: "No automated action for this step — complete manually."}
	}

	action := rb.Actions[step]
	e.log.Info("Executing runbook step", zap.String("runbook", runbookID), zap.Int("step", step), zap.String("action", action))

	return e.executeAction(ctx, action, params)
}

// executeAction dispatches an action string to the right handler.
func (e *Engine) executeAction(ctx context.Context, action string, params map[string]string) StepResult {
	e.mu.RLock()
	k8sClient := e.k8s
	e.mu.RUnlock()

	ns := params["namespace"]
	pod := params["pod"]
	deployment := params["deployment"]

	switch action {
	case "list_crashing_pods":
		pods, err := k8sClient.ListCrashingPods(ctx, ns)
		if err != nil {
			return StepResult{Status: "error", Message: err.Error()}
		}
		if len(pods) == 0 {
			return StepResult{Status: "ok", Message: "No crashing pods detected."}
		}
		msg := fmt.Sprintf("Found %d crashing pod(s): ", len(pods))
		for i, p := range pods {
			if i > 0 {
				msg += ", "
			}
			msg += fmt.Sprintf("%s/%s (restarts=%d)", p.Namespace, p.Name, p.Restarts)
		}
		return StepResult{Status: "ok", Message: msg}

	case "list_pressure_nodes":
		nodes, err := k8sClient.ListPressureNodes(ctx)
		if err != nil {
			return StepResult{Status: "error", Message: err.Error()}
		}
		if len(nodes) == 0 {
			return StepResult{Status: "ok", Message: "No nodes under pressure."}
		}
		msg := fmt.Sprintf("Found %d node(s) under pressure: ", len(nodes))
		for i, n := range nodes {
			if i > 0 {
				msg += ", "
			}
			msg += n.Name
		}
		return StepResult{Status: "ok", Message: msg}

	case "ai_analyze_pod":
		if ns == "" || pod == "" {
			return StepResult{Status: "error", Message: "action requires namespace and pod parameters"}
		}
		report, err := e.ai.RCA().AnalyzePod(ctx, ns, pod)
		if err != nil {
			return StepResult{Status: "error", Message: err.Error()}
		}
		return StepResult{
			Status:  "ok",
			Message: fmt.Sprintf("RCA: %s — %s (confidence %.0f%%)", report.RootCause.Category, report.RootCause.Summary, report.Confidence*100),
		}

	case "restart_deployment":
		if ns == "" || deployment == "" {
			return StepResult{Status: "error", Message: "action requires namespace and deployment parameters"}
		}
		dep, err := k8sClient.Core.AppsV1().Deployments(ns).Get(ctx, deployment, metav1.GetOptions{})
		if err != nil {
			return StepResult{Status: "error", Message: err.Error()}
		}
		if dep.Spec.Template.Annotations == nil {
			dep.Spec.Template.Annotations = map[string]string{}
		}
		dep.Spec.Template.Annotations["kubepilot.io/restartedAt"] = time.Now().Format(time.RFC3339)
		if _, err := k8sClient.Core.AppsV1().Deployments(ns).Update(ctx, dep, metav1.UpdateOptions{}); err != nil {
			return StepResult{Status: "error", Message: err.Error()}
		}
		return StepResult{Status: "ok", Message: fmt.Sprintf("Triggered restart of %s/%s", ns, deployment)}

	case "check_pvc_status":
		pvcs, err := k8sClient.Core.CoreV1().PersistentVolumeClaims(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return StepResult{Status: "error", Message: err.Error()}
		}
		pending := 0
		for _, p := range pvcs.Items {
			if string(p.Status.Phase) != "Bound" {
				pending++
			}
		}
		return StepResult{
			Status:  "ok",
			Message: fmt.Sprintf("PVCs in %s: %d total, %d not Bound", nsOrAll(ns), len(pvcs.Items), pending),
		}

	case "check_image_pull_errors":
		pods, err := k8sClient.ListCrashingPods(ctx, ns)
		if err != nil {
			return StepResult{Status: "error", Message: err.Error()}
		}
		imagePullIssues := 0
		for _, p := range pods {
			if p.Reason == "ErrImagePull" || p.Reason == "ImagePullBackOff" {
				imagePullIssues++
			}
		}
		return StepResult{
			Status:  "ok",
			Message: fmt.Sprintf("Pods with image pull issues: %d of %d crashing pods", imagePullIssues, len(pods)),
		}

	case "cluster_health_summary":
		nodes, err := k8sClient.ListNodes(ctx)
		if err != nil {
			return StepResult{Status: "error", Message: err.Error()}
		}
		ready := 0
		pressure := 0
		for _, n := range nodes {
			if n.Ready {
				ready++
			}
			if n.MemoryPressure || n.DiskPressure || n.PIDPressure {
				pressure++
			}
		}
		crashing, _ := k8sClient.ListCrashingPods(ctx, "")
		return StepResult{
			Status: "ok",
			Message: fmt.Sprintf("Cluster: %d/%d nodes Ready, %d under pressure, %d crashing pods",
				ready, len(nodes), pressure, len(crashing)),
		}

	case "noop":
		return StepResult{Status: "ok", Message: "No-op step complete."}

	default:
		return StepResult{Status: "error", Message: fmt.Sprintf("unknown action %q", action)}
	}
}

func nsOrAll(ns string) string {
	if ns == "" {
		return "all namespaces"
	}
	return ns
}
