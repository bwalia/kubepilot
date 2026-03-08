package ai

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/k8s"
)

// TroubleshootReport contains a structured root-cause analysis for a failing pod.
type TroubleshootReport struct {
	PodName   string `json:"pod_name"`
	Namespace string `json:"namespace"`
	// RootCause is the AI-determined cause: CrashLoopBackOff, OOMKilled, etc.
	RootCause string `json:"root_cause"`
	// Analysis is the LLM's natural language explanation of the failure.
	Analysis string `json:"analysis"`
	// Actions are the suggested remediations.
	Actions []SuggestedAction `json:"actions"`
	// RCA is the full structured RCA report (nil if RCA engine was not used).
	RCA *RCAReport `json:"rca,omitempty"`
}

// TroubleshootPod performs root cause analysis on a specific pod.
// It delegates to the structured RCA engine for deep analysis, then builds
// both a legacy TroubleshootReport and attaches the full RCAReport.
func (e *Engine) TroubleshootPod(ctx context.Context, namespace, podName string) (*TroubleshootReport, error) {
	// Use the structured RCA engine for deep analysis.
	rcaReport, err := e.rca.AnalyzePod(ctx, namespace, podName)
	if err != nil {
		e.log.Warn("RCA engine failed, falling back to simple troubleshoot",
			zap.String("pod", podName),
			zap.Error(err),
		)
		return e.simpleTroubleshoot(ctx, namespace, podName)
	}

	// Build the legacy TroubleshootReport from the RCA output.
	report := &TroubleshootReport{
		PodName:   podName,
		Namespace: namespace,
		RootCause: rcaReport.RootCause.Summary,
		Analysis:  rcaReport.RootCause.Detail,
		RCA:       rcaReport,
	}

	// Convert RCA remediation steps to SuggestedActions for backward compatibility.
	for _, rem := range rcaReport.Remediation {
		action := SuggestedAction{
			Type:           ActionType(rem.Action),
			Namespace:      namespace,
			Explanation:    rem.Description,
			RequiresCRCode: rem.RequiresCR,
		}
		report.Actions = append(report.Actions, action)
	}

	e.log.Info("Troubleshoot report generated via RCA engine",
		zap.String("pod", podName),
		zap.String("namespace", namespace),
		zap.String("root_cause", rcaReport.RootCause.Category),
		zap.Float64("confidence", rcaReport.Confidence),
		zap.Int("remediation_steps", len(rcaReport.Remediation)),
	)

	return report, nil
}

// simpleTroubleshoot is the legacy fallback when the RCA engine fails.
func (e *Engine) simpleTroubleshoot(ctx context.Context, namespace, podName string) (*TroubleshootReport, error) {
	logs, err := e.k8s.GetPodLogs(ctx, namespace, podName, "", 200)
	if err != nil {
		e.log.Warn("could not get pod logs for troubleshooting",
			zap.String("pod", podName),
			zap.String("namespace", namespace),
			zap.Error(err),
		)
		logs = "(logs unavailable)"
	}

	pods, err := e.k8s.ListPods(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("listing pods for troubleshoot context: %w", err)
	}

	var targetPod *k8s.PodSummary
	for _, p := range pods {
		if p.Name == podName {
			cp := p
			targetPod = &cp
			break
		}
	}
	if targetPod == nil {
		return nil, fmt.Errorf("pod %s/%s not found", namespace, podName)
	}

	command := fmt.Sprintf(
		"Perform root cause analysis for pod %s in namespace %s. "+
			"Pod reason: %s. Restart count: %d. "+
			"Recent logs:\n%s",
		podName, namespace,
		targetPod.Reason,
		targetPod.Restarts,
		truncateLogs(logs, 3000),
	)

	actions, err := e.Interpret(ctx, command)
	if err != nil {
		return nil, fmt.Errorf("AI interpretation for pod troubleshoot: %w", err)
	}

	report := &TroubleshootReport{
		PodName:   podName,
		Namespace: namespace,
		RootCause: targetPod.Reason,
		Actions:   actions,
	}

	if len(actions) > 0 {
		report.Analysis = actions[0].Explanation
	}

	return report, nil
}

// TroubleshootNamespace runs a bulk analysis of all crashing pods in a namespace,
// returning one report per failing pod. Suitable for the dashboard's health overview.
func (e *Engine) TroubleshootNamespace(ctx context.Context, namespace string) ([]*TroubleshootReport, error) {
	crashing, err := e.k8s.ListCrashingPods(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("listing crashing pods in namespace %q: %w", namespace, err)
	}

	reports := make([]*TroubleshootReport, 0, len(crashing))
	for _, pod := range crashing {
		report, err := e.TroubleshootPod(ctx, pod.Namespace, pod.Name)
		if err != nil {
			e.log.Warn("skipping pod troubleshoot",
				zap.String("pod", pod.Name),
				zap.Error(err),
			)
			continue
		}
		reports = append(reports, report)
	}

	return reports, nil
}

// truncateLogs trims log output to maxChars to stay within LLM token limits.
// It preserves the tail of the log (most recent lines) which has the highest signal.
func truncateLogs(logs string, maxChars int) string {
	if len(logs) <= maxChars {
		return logs
	}
	// Keep the last maxChars characters — most recent events are most relevant.
	truncated := logs[len(logs)-maxChars:]
	// Trim to a clean line boundary.
	if idx := strings.Index(truncated, "\n"); idx >= 0 {
		truncated = truncated[idx+1:]
	}
	return "[...truncated...]\n" + truncated
}
