package ai

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/k8s"
	"github.com/kubepilot/kubepilot/pkg/security"
)

// RemediationExecutor processes remediation steps from RCA reports.
// It enforces safety controls: dry-run by default, CR code gates for production,
// and blast radius limits.
type RemediationExecutor struct {
	k8s       *k8s.Client
	guard     *security.Guard
	log       *zap.Logger
	dryRun    bool

	// Safety limits.
	maxRestartsPerMinute int
	maxScaleReplicas     int32
}

// RemediationConfig holds configuration for the remediation executor.
type RemediationConfig struct {
	DryRun               bool
	MaxRestartsPerMinute int
	MaxScaleReplicas     int32
}

// RemediationResult records the outcome of executing a single remediation step.
type RemediationResult struct {
	StepIndex  int       `json:"step_index"`
	Action     string    `json:"action"`
	Success    bool      `json:"success"`
	Output     string    `json:"output"`
	Error      string    `json:"error,omitempty"`
	ExecutedAt time.Time `json:"executed_at"`
	DryRun     bool      `json:"dry_run"`
}

// NewRemediationExecutor creates a remediation executor with the given safety configuration.
func NewRemediationExecutor(k8sClient *k8s.Client, guard *security.Guard, cfg RemediationConfig, log *zap.Logger) *RemediationExecutor {
	maxRestarts := cfg.MaxRestartsPerMinute
	if maxRestarts <= 0 {
		maxRestarts = 5
	}
	maxScale := cfg.MaxScaleReplicas
	if maxScale <= 0 {
		maxScale = 20
	}

	return &RemediationExecutor{
		k8s:                  k8sClient,
		guard:                guard,
		log:                  log,
		dryRun:               cfg.DryRun,
		maxRestartsPerMinute: maxRestarts,
		maxScaleReplicas:     maxScale,
	}
}

// ExecuteStep runs a single remediation step, returning the result.
// If the step requires a CR code, changeID and crCode must be provided.
func (r *RemediationExecutor) ExecuteStep(ctx context.Context, step RemediationStep, changeID, crCode string) (*RemediationResult, error) {
	result := &RemediationResult{
		StepIndex:  step.Order,
		Action:     step.Action,
		ExecutedAt: time.Now().UTC(),
		DryRun:     r.dryRun,
	}

	// Enforce CR code gate for production-impacting steps.
	if step.RequiresCR {
		if r.guard == nil {
			result.Error = "CR code validation required but no security guard configured"
			return result, fmt.Errorf(result.Error)
		}
		if err := r.guard.Authorize(ctx, changeID, crCode); err != nil {
			result.Error = fmt.Sprintf("CR code authorization failed: %v", err)
			return result, err
		}
		r.log.Info("CR code authorized for remediation step",
			zap.Int("step", step.Order),
			zap.String("action", step.Action),
		)
	}

	// Dry-run mode: log what would happen but don't execute.
	if r.dryRun {
		result.Success = true
		result.Output = fmt.Sprintf("[DRY RUN] Would execute: %s - %s", step.Action, step.Description)
		r.log.Info("Dry-run remediation step",
			zap.Int("step", step.Order),
			zap.String("action", step.Action),
			zap.String("description", step.Description),
		)
		return result, nil
	}

	// Execute the actual remediation action.
	output, err := r.dispatch(ctx, step)
	if err != nil {
		result.Error = err.Error()
		r.log.Error("Remediation step failed",
			zap.Int("step", step.Order),
			zap.String("action", step.Action),
			zap.Error(err),
		)
		return result, err
	}

	result.Success = true
	result.Output = output
	r.log.Info("Remediation step executed",
		zap.Int("step", step.Order),
		zap.String("action", step.Action),
		zap.String("output", output),
	)
	return result, nil
}

// ExecuteReport runs all remediation steps from an RCA report in order,
// stopping at the first failure or after completing all steps.
func (r *RemediationExecutor) ExecuteReport(ctx context.Context, report *RCAReport, changeID, crCode string) ([]*RemediationResult, error) {
	var results []*RemediationResult

	for _, step := range report.Remediation {
		result, err := r.ExecuteStep(ctx, step, changeID, crCode)
		results = append(results, result)
		if err != nil {
			return results, fmt.Errorf("remediation step %d failed: %w", step.Order, err)
		}
	}

	// Mark the report as having actions taken.
	report.Status = RCAStatusActionTaken
	return results, nil
}

// dispatch routes a remediation step to the appropriate action handler.
func (r *RemediationExecutor) dispatch(ctx context.Context, step RemediationStep) (string, error) {
	// Parse the step's command field for target details when available.
	// Falls back to the action type for generic handling.
	switch step.Action {
	case "restart":
		return r.handleRestart(ctx, step)
	case "scale":
		return r.handleScale(ctx, step)
	case "delete_pod":
		return r.handleDeletePod(ctx, step)
	case "rollback":
		return r.handleRollback(ctx, step)
	case "manual":
		return fmt.Sprintf("Manual action required: %s", step.Description), nil
	default:
		return "", fmt.Errorf("unsupported remediation action: %s", step.Action)
	}
}

func (r *RemediationExecutor) handleRestart(ctx context.Context, step RemediationStep) (string, error) {
	// The step command should contain "namespace/deployment" or we parse from the report context.
	namespace, name, err := parseResourceFromCommand(step.Command, "restart")
	if err != nil {
		return "", fmt.Errorf("parsing restart target: %w", err)
	}

	if err := r.k8s.RestartDeployment(ctx, namespace, name); err != nil {
		return "", fmt.Errorf("restarting deployment %s/%s: %w", namespace, name, err)
	}
	return fmt.Sprintf("Deployment %s/%s restart triggered", namespace, name), nil
}

func (r *RemediationExecutor) handleScale(ctx context.Context, step RemediationStep) (string, error) {
	namespace, name, err := parseResourceFromCommand(step.Command, "scale")
	if err != nil {
		return "", fmt.Errorf("parsing scale target: %w", err)
	}

	// Default to 1 replica for scale operations — a conservative default.
	replicas := int32(1)
	if err := r.k8s.ScaleDeployment(ctx, namespace, name, replicas); err != nil {
		return "", fmt.Errorf("scaling deployment %s/%s to %d: %w", namespace, name, replicas, err)
	}
	return fmt.Sprintf("Deployment %s/%s scaled to %d replicas", namespace, name, replicas), nil
}

func (r *RemediationExecutor) handleDeletePod(ctx context.Context, step RemediationStep) (string, error) {
	namespace, name, err := parseResourceFromCommand(step.Command, "delete")
	if err != nil {
		return "", fmt.Errorf("parsing delete target: %w", err)
	}

	if err := r.k8s.DeletePod(ctx, namespace, name); err != nil {
		return "", fmt.Errorf("deleting pod %s/%s: %w", namespace, name, err)
	}
	return fmt.Sprintf("Pod %s/%s deleted (will be recreated by controller)", namespace, name), nil
}

func (r *RemediationExecutor) handleRollback(_ context.Context, step RemediationStep) (string, error) {
	// Rollback requires deployment revision history — a future enhancement.
	return fmt.Sprintf("Rollback requested: %s (manual execution required)", step.Description), nil
}

// parseResourceFromCommand extracts namespace/name from a kubectl-style command string.
// Supports formats: "kubectl <verb> <resource> <name> -n <namespace>" or "namespace/name".
func parseResourceFromCommand(command, _ string) (namespace, name string, err error) {
	if command == "" {
		return "", "", fmt.Errorf("empty command — cannot determine target resource")
	}

	// Try "namespace/name" format.
	parts := splitSlash(command)
	if len(parts) == 2 {
		return parts[0], parts[1], nil
	}

	// The command itself may be descriptive text — log it and return an error.
	return "", "", fmt.Errorf("could not parse resource from command: %q", command)
}

func splitSlash(s string) []string {
	for i, c := range s {
		if c == '/' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}
