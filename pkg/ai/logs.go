package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/k8s"
)

// LogEntry represents a single structured log line.
type LogEntry struct {
	Line      string    `json:"line"`
	Severity  string    `json:"severity"` // error, warn, info, debug
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// DetectedPattern represents a recurring error pattern found in logs.
type DetectedPattern struct {
	Name     string    `json:"name"`
	Count    int       `json:"count"`
	Examples []string  `json:"examples"`
	FirstSeen time.Time `json:"first_seen,omitempty"`
	LastSeen  time.Time `json:"last_seen,omitempty"`
}

// LogAnalysis is the result of analyzing a pod's logs.
type LogAnalysis struct {
	PodName     string            `json:"pod_name"`
	Namespace   string            `json:"namespace"`
	Container   string            `json:"container"`
	TotalLines  int               `json:"total_lines"`
	ErrorLines  []LogEntry        `json:"error_lines"`
	Patterns    []DetectedPattern `json:"patterns"`
	LLMSummary  string            `json:"llm_summary"`
	Suggestions []string          `json:"suggestions"`
}

// LogAnalyzer performs intelligent log analysis using pattern matching and LLM interpretation.
type LogAnalyzer struct {
	engine *Engine
	k8s    *k8s.Client
	log    *zap.Logger
}

// NewLogAnalyzer creates a LogAnalyzer.
func NewLogAnalyzer(engine *Engine, k8sClient *k8s.Client, log *zap.Logger) *LogAnalyzer {
	return &LogAnalyzer{
		engine: engine,
		k8s:    k8sClient,
		log:    log,
	}
}

// AnalyzePodLogs fetches and analyzes logs for a specific pod.
func (la *LogAnalyzer) AnalyzePodLogs(ctx context.Context, namespace, podName, container string, tailLines int64) (*LogAnalysis, error) {
	if tailLines <= 0 {
		tailLines = 500
	}

	logs, err := la.k8s.GetPodLogs(ctx, namespace, podName, container, tailLines)
	if err != nil {
		return nil, fmt.Errorf("fetching logs for %s/%s: %w", namespace, podName, err)
	}

	lines := strings.Split(logs, "\n")

	analysis := &LogAnalysis{
		PodName:    podName,
		Namespace:  namespace,
		Container:  container,
		TotalLines: len(lines),
	}

	// Extract error lines and detect patterns.
	analysis.ErrorLines = extractErrorLines(lines)
	analysis.Patterns = detectPatterns(lines)

	// Use LLM for interpretation if there are error signals.
	if len(analysis.ErrorLines) > 0 || len(analysis.Patterns) > 0 {
		summary, suggestions, err := la.llmInterpret(ctx, namespace, podName, logs, analysis.ErrorLines, analysis.Patterns)
		if err != nil {
			la.log.Warn("LLM log interpretation failed", zap.Error(err))
		} else {
			analysis.LLMSummary = summary
			analysis.Suggestions = suggestions
		}
	}

	return analysis, nil
}

// extractErrorLines filters log lines that appear to be errors or warnings.
func extractErrorLines(lines []string) []LogEntry {
	var errors []LogEntry
	errorIndicators := []string{
		"error", "ERROR", "Error",
		"panic", "PANIC", "Panic",
		"fatal", "FATAL", "Fatal",
		"exception", "Exception", "EXCEPTION",
		"fail", "FAIL", "Fail",
		"OOM", "oom", "OutOfMemory",
		"killed", "KILLED",
		"refused", "REFUSED",
		"timeout", "TIMEOUT",
		"denied", "DENIED",
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, indicator := range errorIndicators {
			if strings.Contains(line, indicator) {
				severity := "error"
				if strings.Contains(strings.ToLower(line), "warn") {
					severity = "warn"
				}
				if strings.Contains(strings.ToLower(line), "fatal") || strings.Contains(strings.ToLower(line), "panic") {
					severity = "fatal"
				}
				errors = append(errors, LogEntry{
					Line:     line,
					Severity: severity,
				})
				break
			}
		}
	}
	return errors
}

// detectPatterns identifies recurring error patterns in logs.
func detectPatterns(lines []string) []DetectedPattern {
	patternDefs := []struct {
		Name     string
		Keywords []string
	}{
		{"stack-trace", []string{"goroutine", "runtime.goexit", "panic:", "traceback", "Traceback", "at "}},
		{"connection-refused", []string{"connection refused", "Connection refused", "ECONNREFUSED"}},
		{"oom", []string{"OOMKilled", "out of memory", "OutOfMemoryError", "Cannot allocate memory"}},
		{"timeout", []string{"timeout", "Timeout", "TIMEOUT", "context deadline exceeded", "i/o timeout"}},
		{"auth-failure", []string{"unauthorized", "Unauthorized", "403", "forbidden", "Forbidden", "authentication failed"}},
		{"dns-failure", []string{"no such host", "DNS lookup failed", "NXDOMAIN", "name resolution failed"}},
		{"image-pull", []string{"ImagePullBackOff", "ErrImagePull", "manifest unknown", "pull access denied"}},
		{"crash-loop", []string{"CrashLoopBackOff", "back-off restarting", "restarting failed container"}},
		{"permission-denied", []string{"permission denied", "Permission denied", "EACCES", "read-only file system"}},
		{"resource-limit", []string{"resource limit", "quota exceeded", "LimitRange", "insufficient"}},
	}

	var detected []DetectedPattern
	for _, def := range patternDefs {
		var examples []string
		count := 0
		for _, line := range lines {
			for _, kw := range def.Keywords {
				if strings.Contains(line, kw) {
					count++
					if len(examples) < 3 {
						examples = append(examples, strings.TrimSpace(line))
					}
					break
				}
			}
		}
		if count > 0 {
			detected = append(detected, DetectedPattern{
				Name:     def.Name,
				Count:    count,
				Examples: examples,
			})
		}
	}
	return detected
}

// llmInterpret sends error lines and patterns to the LLM for natural language interpretation.
func (la *LogAnalyzer) llmInterpret(ctx context.Context, namespace, podName, rawLogs string, errorLines []LogEntry, patterns []DetectedPattern) (string, []string, error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Analyze logs for pod %s/%s.\n\n", namespace, podName))

	if len(patterns) > 0 {
		sb.WriteString("Detected patterns:\n")
		for _, p := range patterns {
			sb.WriteString(fmt.Sprintf("- %s: %d occurrences\n", p.Name, p.Count))
			for _, ex := range p.Examples {
				sb.WriteString(fmt.Sprintf("  Example: %s\n", truncateString(ex, 200)))
			}
		}
		sb.WriteString("\n")
	}

	if len(errorLines) > 0 {
		sb.WriteString("Error lines (sample):\n")
		maxErrors := 20
		if len(errorLines) < maxErrors {
			maxErrors = len(errorLines)
		}
		for _, e := range errorLines[:maxErrors] {
			sb.WriteString(fmt.Sprintf("  [%s] %s\n", e.Severity, truncateString(e.Line, 200)))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Recent log tail:\n")
	sb.WriteString(truncateLogs(rawLogs, 2000))

	systemPrompt := `You are a Kubernetes log analysis expert. Analyze the provided log data and respond with:
1. A concise summary of what's going wrong (2-3 sentences).
2. A JSON array of actionable suggestions (strings).

Format your response as:
SUMMARY: <your summary>
SUGGESTIONS: ["suggestion 1", "suggestion 2", ...]`

	resp, err := la.engine.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: la.engine.cfg.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: sb.String()},
		},
		Temperature: 0.2,
	})
	if err != nil {
		return "", nil, err
	}

	if len(resp.Choices) == 0 {
		return "", nil, fmt.Errorf("empty LLM response")
	}

	raw := resp.Choices[0].Message.Content
	summary, suggestions := parseLogAnalysisResponse(raw)
	return summary, suggestions, nil
}

// parseLogAnalysisResponse extracts the summary and suggestions from the LLM response.
func parseLogAnalysisResponse(raw string) (string, []string) {
	summary := raw
	var suggestions []string

	if idx := strings.Index(raw, "SUMMARY:"); idx >= 0 {
		afterSummary := raw[idx+8:]
		if sugIdx := strings.Index(afterSummary, "SUGGESTIONS:"); sugIdx >= 0 {
			summary = strings.TrimSpace(afterSummary[:sugIdx])
			sugStr := strings.TrimSpace(afterSummary[sugIdx+12:])
			// Try to parse the suggestions array.
			if start := strings.Index(sugStr, "["); start >= 0 {
				if end := strings.LastIndex(sugStr, "]"); end > start {
					// Simple split on commas within quotes.
					inner := sugStr[start+1 : end]
					for _, part := range strings.Split(inner, "\",") {
						s := strings.Trim(strings.TrimSpace(part), "\"")
						if s != "" {
							suggestions = append(suggestions, s)
						}
					}
				}
			}
		} else {
			summary = strings.TrimSpace(afterSummary)
		}
	}

	return summary, suggestions
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
