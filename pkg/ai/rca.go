package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/k8s"
)

// RCAEngine orchestrates Kubernetes root cause analysis using the AI engine
// and the rich data layer from pkg/k8s.
type RCAEngine struct {
	engine *Engine
	k8s    *k8s.Client
	log    *zap.Logger
	idSeq  int64
}

// NewRCAEngine creates an RCAEngine backed by the provided Engine and k8s client.
func NewRCAEngine(engine *Engine, k8sClient *k8s.Client, log *zap.Logger) *RCAEngine {
	return &RCAEngine{
		engine: engine,
		k8s:    k8sClient,
		log:    log,
	}
}

// AnalyzePod performs deep root cause analysis on a specific pod.
// It collects diagnostics, events, logs, resource metrics, and sends everything
// to the LLM for structured analysis.
func (r *RCAEngine) AnalyzePod(ctx context.Context, namespace, podName string) (*RCAReport, error) {
	r.log.Info("Starting RCA analysis",
		zap.String("namespace", namespace),
		zap.String("pod", podName),
	)

	// Collect pod diagnostics (conditions, container statuses, events, resource metrics, owner chain).
	diag, err := r.k8s.GetPodDiagnostics(ctx, namespace, podName)
	if err != nil {
		return nil, fmt.Errorf("getting pod diagnostics for %s/%s: %w", namespace, podName, err)
	}

	// Collect pod logs.
	logs, err := r.k8s.GetPodLogs(ctx, namespace, podName, "", 200)
	if err != nil {
		r.log.Warn("Could not get pod logs for RCA",
			zap.String("pod", podName),
			zap.Error(err),
		)
		logs = "(logs unavailable)"
	}

	// Build the prompt sections.
	podInfo := fmt.Sprintf("Name: %s\nNamespace: %s\nPhase: %s\nNode: %s\nServiceAccount: %s\nCreated: %s",
		diag.Name, diag.Namespace, diag.Phase, diag.NodeName,
		diag.ServiceAccount, diag.CreatedAt.Format(time.RFC3339))

	containerInfo := formatContainerStatuses(diag.ContainerStatuses)
	conditionInfo := formatConditions(diag.Conditions)
	eventInfo := formatEvents(diag.Events)
	logInfo := truncateLogs(logs, 4000)
	resourceInfo := formatResourceMetrics(diag.ResourceUsage)
	ownerInfo := formatOwnerChain(diag.OwnerChain)

	userPrompt := fmt.Sprintf(rcaPodUserPromptTemplate,
		podInfo, containerInfo, conditionInfo, eventInfo, logInfo, resourceInfo, ownerInfo)

	// Add scenario-specific hints if we can identify the failure mode.
	scenario := identifyScenario(diag)
	systemPrompt := rcaSystemPrompt
	if hint, ok := rcaScenarioHints[scenario]; ok {
		systemPrompt += "\n\n## Scenario Hint\n" + hint
	}

	// Call the LLM.
	resp, err := r.engine.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: r.engine.cfg.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		Temperature: 0.1, // Very low temp for structured factual analysis.
	})
	if err != nil {
		return nil, fmt.Errorf("calling LLM for RCA: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty LLM response for RCA")
	}

	raw := resp.Choices[0].Message.Content

	// Parse the LLM response into a structured report.
	report, parseErr := r.parseLLMResponse(raw, namespace, podName)
	if parseErr != nil {
		r.log.Warn("LLM response was not valid JSON, building low-confidence report",
			zap.Error(parseErr),
		)
		report = r.buildFallbackReport(namespace, podName, raw, diag)
	}

	r.log.Info("RCA analysis complete",
		zap.String("pod", podName),
		zap.String("severity", string(report.Severity)),
		zap.String("root_cause", report.RootCause.Category),
		zap.Float64("confidence", report.Confidence),
	)

	return report, nil
}

// parseLLMResponse attempts to parse the LLM output as a structured RCA response.
func (r *RCAEngine) parseLLMResponse(raw, namespace, podName string) (*RCAReport, error) {
	raw = strings.TrimSpace(raw)

	var llmResp LLMRCAResponse

	// Try direct parse.
	if err := json.Unmarshal([]byte(raw), &llmResp); err != nil {
		// Try extracting JSON from markdown fences or embedded objects.
		cleaned := extractJSON(raw)
		if err2 := json.Unmarshal([]byte(cleaned), &llmResp); err2 != nil {
			return nil, fmt.Errorf("failed to parse LLM RCA response: %w (raw length: %d)", err, len(raw))
		}
	}

	r.idSeq++
	report := &RCAReport{
		ID:        fmt.Sprintf("rca-%s-%s-%d", namespace, podName, time.Now().Unix()),
		Timestamp: time.Now().UTC(),
		TargetResource: ResourceRef{
			Kind:      "Pod",
			Name:      podName,
			Namespace: namespace,
		},
		Severity: parseSeverity(llmResp.Severity),
		RootCause: RootCause{
			Category:           llmResp.RootCause.Category,
			Summary:            llmResp.RootCause.Summary,
			Detail:             llmResp.RootCause.Detail,
			AffectedComponents: llmResp.RootCause.AffectedComponents,
		},
		Confidence: clampConfidence(llmResp.Confidence),
		Status:     RCAStatusComplete,
	}

	for _, e := range llmResp.Evidence {
		report.EvidenceChain = append(report.EvidenceChain, Evidence{
			Source:    e.Source,
			Data:      e.Data,
			Relevance: e.Relevance,
			Timestamp: time.Now().UTC(),
		})
	}

	for _, rem := range llmResp.Remediation {
		report.Remediation = append(report.Remediation, RemediationStep{
			Order:       rem.Order,
			Action:      rem.Action,
			Description: rem.Description,
			Command:     rem.Command,
			Risk:        rem.Risk,
			AutoApply:   rem.AutoApply,
			RequiresCR:  rem.RequiresCR,
		})
	}

	return report, nil
}

// buildFallbackReport creates a low-confidence report when the LLM produces
// non-parseable output. The raw response is included as analysis text.
func (r *RCAEngine) buildFallbackReport(namespace, podName, rawResponse string, diag *k8s.PodDiagnostics) *RCAReport {
	scenario := identifyScenario(diag)
	severity := SeverityMedium
	if scenario == "OOMKilled" || scenario == "CrashLoopBackOff" {
		severity = SeverityHigh
	}

	return &RCAReport{
		ID:        fmt.Sprintf("rca-%s-%s-%d", namespace, podName, time.Now().Unix()),
		Timestamp: time.Now().UTC(),
		TargetResource: ResourceRef{
			Kind:      "Pod",
			Name:      podName,
			Namespace: namespace,
		},
		Severity: severity,
		RootCause: RootCause{
			Category: scenario,
			Summary:  fmt.Sprintf("Pod %s/%s is in %s state", namespace, podName, scenario),
			Detail:   strings.TrimSpace(rawResponse),
		},
		EvidenceChain: []Evidence{
			{
				Source:    "status",
				Data:      fmt.Sprintf("Phase: %s, Scenario: %s", diag.Phase, scenario),
				Relevance: "Primary failure indicator from pod status",
				Timestamp: time.Now().UTC(),
			},
		},
		Remediation: []RemediationStep{
			{
				Order:       1,
				Action:      "manual",
				Description: "Review the AI analysis above and investigate further",
				Risk:        "safe",
				AutoApply:   false,
				RequiresCR:  false,
			},
		},
		Confidence: 0.3,
		Status:     RCAStatusComplete,
	}
}

// identifyScenario determines the primary failure mode from pod diagnostics.
func identifyScenario(diag *k8s.PodDiagnostics) string {
	for _, cs := range diag.ContainerStatuses {
		switch {
		case cs.StateReason == "CrashLoopBackOff":
			return "CrashLoopBackOff"
		case cs.StateReason == "OOMKilled" || cs.LastTerminatedReason == "OOMKilled":
			return "OOMKilled"
		case cs.StateReason == "ImagePullBackOff" || cs.StateReason == "ErrImagePull":
			return "ImagePullBackOff"
		case cs.StateReason == "Error" || cs.ExitCode != 0:
			return "Error"
		}
	}
	if diag.Phase == "Pending" {
		return "Pending"
	}
	return "Unknown"
}

// Helper functions for formatting diagnostic data into prompt sections.

func formatContainerStatuses(statuses []k8s.ContainerDiag) string {
	if len(statuses) == 0 {
		return "(no container statuses)"
	}
	var sb strings.Builder
	for _, cs := range statuses {
		fmt.Fprintf(&sb, "- %s: image=%s, state=%s, reason=%s, restarts=%d, exit_code=%d",
			cs.Name, cs.Image, cs.State, cs.StateReason, cs.RestartCount, cs.ExitCode)
		if cs.LastTerminatedReason != "" {
			fmt.Fprintf(&sb, ", last_terminated=%s", cs.LastTerminatedReason)
		}
		if cs.StateMessage != "" {
			fmt.Fprintf(&sb, ", message=%s", cs.StateMessage)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func formatConditions(conditions []k8s.PodCondition) string {
	if len(conditions) == 0 {
		return "(no conditions)"
	}
	var sb strings.Builder
	for _, c := range conditions {
		fmt.Fprintf(&sb, "- %s=%s", c.Type, c.Status)
		if c.Reason != "" {
			fmt.Fprintf(&sb, " (reason=%s)", c.Reason)
		}
		if c.Message != "" {
			fmt.Fprintf(&sb, " message=%q", c.Message)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func formatEvents(events []k8s.Event) string {
	if len(events) == 0 {
		return "(no events)"
	}
	var sb strings.Builder
	for _, e := range events {
		fmt.Fprintf(&sb, "- [%s] %s: %s (count=%d, last=%s)\n",
			e.Type, e.Reason, e.Message, e.Count, e.LastSeen.Format(time.RFC3339))
	}
	return sb.String()
}

func formatResourceMetrics(rm *k8s.ResourceMetrics) string {
	if rm == nil {
		return "(resource metrics unavailable)"
	}
	return fmt.Sprintf("CPU: request=%s limit=%s usage=%s\nMemory: request=%s limit=%s usage=%s",
		rm.CPURequest, rm.CPULimit, rm.CPUUsage,
		rm.MemRequest, rm.MemLimit, rm.MemUsage)
}

func formatOwnerChain(owners []k8s.OwnerRef) string {
	if len(owners) == 0 {
		return "(no owner references)"
	}
	var parts []string
	for _, o := range owners {
		parts = append(parts, fmt.Sprintf("%s/%s", o.Kind, o.Name))
	}
	return strings.Join(parts, " → ")
}

// extractJSON attempts to extract a JSON object from a string that may contain
// markdown fences or surrounding prose.
func extractJSON(raw string) string {
	// Strip markdown code fences.
	if idx := strings.Index(raw, "```"); idx >= 0 {
		cleaned := raw[idx+3:]
		if nl := strings.Index(cleaned, "\n"); nl >= 0 {
			cleaned = cleaned[nl+1:]
		}
		if end := strings.Index(cleaned, "```"); end >= 0 {
			cleaned = cleaned[:end]
		}
		return strings.TrimSpace(cleaned)
	}

	// Find the first '{' and last '}' for an embedded JSON object.
	if start := strings.Index(raw, "{"); start >= 0 {
		if end := strings.LastIndex(raw, "}"); end > start {
			return raw[start : end+1]
		}
	}

	return raw
}

func parseSeverity(s string) Severity {
	switch strings.ToLower(s) {
	case "critical":
		return SeverityCritical
	case "high":
		return SeverityHigh
	case "medium":
		return SeverityMedium
	case "low":
		return SeverityLow
	case "info":
		return SeverityInfo
	default:
		return SeverityMedium
	}
}

func clampConfidence(c float64) float64 {
	if c < 0 {
		return 0
	}
	if c > 1 {
		return 1
	}
	return c
}
