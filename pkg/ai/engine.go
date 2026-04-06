// Package ai provides KubePilot's natural language AI engine.
// It supports local LLMs via Ollama (default) as well as any provider that
// exposes an OpenAI-compatible chat completions API.
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/k8s"
)

// AIHealthStatus reports the health of the AI/Ollama backend.
type AIHealthStatus struct {
	Healthy   bool   `json:"healthy"`
	Model     string `json:"model"`
	BaseURL   string `json:"base_url"`
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

const (
	// DefaultOllamaBaseURL is the default Ollama API endpoint.
	// Override with KUBEPILOT_OLLAMA_BASE_URL for remote or authenticated Ollama instances.
	DefaultOllamaBaseURL = "http://localhost:11434/v1"
	// DefaultOllamaModel is the default Ollama model. Override with --ollama-model.
	DefaultOllamaModel = "llama3"
)

// Config holds the AI engine configuration.
type Config struct {
	// OllamaAPIKey is optional — Ollama does not require a key for local instances.
	// Set KUBEPILOT_OLLAMA_API_KEY if your Ollama instance is behind an auth proxy.
	OllamaAPIKey string
	// OllamaBaseURL is the base URL of the Ollama API (OpenAI-compatible endpoint).
	// Defaults to http://localhost:11434/v1.
	OllamaBaseURL string
	// Model is the Ollama model name (e.g. "llama3", "mistral", "codellama").
	Model string
}

// ActionType categorises AI-suggested Kubernetes actions.
type ActionType string

const (
	ActionScale        ActionType = "scale"
	ActionRestart      ActionType = "restart"
	ActionDeletePod    ActionType = "delete_pod"
	ActionInvestigate  ActionType = "investigate"
	ActionNoOp         ActionType = "noop"
	ActionCustomJob    ActionType = "custom_job"
)

// SuggestedAction is a structured action plan returned by the AI engine.
// The dashboard presents these for approval; the scheduler executes them.
type SuggestedAction struct {
	Type        ActionType `json:"type"`
	Namespace   string     `json:"namespace,omitempty"`
	Resource    string     `json:"resource,omitempty"`
	Replicas    int32      `json:"replicas,omitempty"`
	Command     string     `json:"command,omitempty"`
	Explanation string     `json:"explanation"`
	// RequiresCRCode indicates this action must not run on production without a valid CR code.
	RequiresCRCode bool `json:"requires_cr_code"`
}

// Engine handles natural language interpretation of cluster management commands.
type Engine struct {
	mu       sync.RWMutex
	cfg       Config
	client    *openai.Client
	k8s       *k8s.Client
	log       *zap.Logger
	rca       *RCAEngine
}

// RCA returns the engine's RCA sub-engine for structured root cause analysis.
func (e *Engine) RCA() *RCAEngine {
	return e.rca
}

// Client returns the underlying OpenAI-compatible client (for sub-engines).
func (e *Engine) Client() *openai.Client {
	return e.client
}

// SetK8sClient swaps the active Kubernetes client at runtime.
// This allows the dashboard/server to switch target clusters without restart.
func (e *Engine) SetK8sClient(k8sClient *k8s.Client) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.k8s = k8sClient
	if e.rca != nil {
		e.rca.k8s = k8sClient
	}
}

// NewEngine constructs an Engine with the provided configuration.
// The go-openai client is pointed at the Ollama base URL so any model
// served by Ollama (llama3, mistral, codellama, etc.) can be used without
// changing any other code.
func NewEngine(cfg Config, k8sClient *k8s.Client, log *zap.Logger) *Engine {
	baseURL := cfg.OllamaBaseURL
	if baseURL == "" {
		baseURL = DefaultOllamaBaseURL
	}
	if cfg.Model == "" {
		cfg.Model = DefaultOllamaModel
	}

	// go-openai supports any OpenAI-compatible endpoint via ClientConfig.
	// Ollama exposes one at /v1 — no API key required for local instances.
	clientCfg := openai.DefaultConfig(cfg.OllamaAPIKey)
	clientCfg.BaseURL = baseURL

	e := &Engine{
		cfg:    cfg,
		client: openai.NewClientWithConfig(clientCfg),
		k8s:    k8sClient,
		log:    log,
	}
	e.rca = NewRCAEngine(e, k8sClient, log)
	return e
}

// CheckHealth tests the Ollama API connection by listing models.
// Returns health status including latency and any errors.
func (e *Engine) CheckHealth(ctx context.Context) AIHealthStatus {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	status := AIHealthStatus{
		Model:   e.cfg.Model,
		BaseURL: e.cfg.OllamaBaseURL,
	}
	if status.BaseURL == "" {
		status.BaseURL = DefaultOllamaBaseURL
	}

	_, err := e.client.ListModels(ctx)
	status.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		status.Healthy = false
		status.Error = err.Error()
		e.log.Warn("AI health check failed", zap.Error(err))
	} else {
		status.Healthy = true
	}
	return status
}

// Interpret takes a natural language command, builds cluster context,
// sends it to the LLM, and returns a list of suggested actions.
//
// Example input:  "Fix pods in CrashLoopBackOff in the production namespace"
// Example output: [{type: "restart", namespace: "production", resource: "api-server", ...}]
func (e *Engine) Interpret(ctx context.Context, command string) ([]SuggestedAction, error) {
	clusterCtx, err := e.buildClusterContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("building cluster context for AI prompt: %w", err)
	}

	systemPrompt := `You are KubePilot, an expert Kubernetes AI operations assistant.
You receive:
1. A natural language command from a cluster operator.
2. A JSON summary of the current cluster state.

You must respond with a JSON array of suggested actions. Each action must have:
- "type": one of: scale | restart | delete_pod | investigate | noop | custom_job
- "namespace": target Kubernetes namespace (omit for cluster-wide)
- "resource": deployment/pod/job name
- "replicas": new replica count (only for scale actions)
- "explanation": concise human-readable justification
- "requires_cr_code": true if the action would affect a production namespace or delete/scale resources

Respond ONLY with valid JSON array. No prose, no markdown fences.`

	userPrompt := fmt.Sprintf("Command: %s\n\nCluster state:\n%s", command, clusterCtx)

	resp, err := e.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: e.cfg.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		Temperature: 0.2, // Low temperature: deterministic, factual responses for ops context.
	})
	if err != nil {
		return nil, fmt.Errorf("calling Ollama API (%s, model: %s): %w", e.cfg.OllamaBaseURL, e.cfg.Model, err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from Ollama API (model: %s)", e.cfg.Model)
	}

	raw := resp.Choices[0].Message.Content
	actions, parseErr := parseActionsFromResponse(raw)
	if parseErr != nil {
		// Smaller models (e.g. tinyllama) may not produce valid JSON.
		// Fall back to wrapping the raw response as an investigate action
		// so the user still gets useful output from the LLM.
		e.log.Warn("LLM response was not valid JSON, wrapping as investigate action",
			zap.String("model", e.cfg.Model),
			zap.Error(parseErr),
		)
		actions = []SuggestedAction{{
			Type:        ActionInvestigate,
			Explanation: strings.TrimSpace(raw),
		}}
	}

	e.log.Info("AI interpreted command",
		zap.String("command", command),
		zap.Int("actions", len(actions)),
	)
	return actions, nil
}

// parseActionsFromResponse attempts to extract a JSON array of SuggestedAction
// from the LLM response. It tries the full response first, then searches for
// an embedded JSON array (models sometimes wrap JSON in markdown fences or prose).
func parseActionsFromResponse(raw string) ([]SuggestedAction, error) {
	raw = strings.TrimSpace(raw)

	// Try direct parse first.
	var actions []SuggestedAction
	if err := json.Unmarshal([]byte(raw), &actions); err == nil {
		return actions, nil
	}

	// Strip markdown code fences if present (```json ... ```).
	cleaned := raw
	if idx := strings.Index(cleaned, "```"); idx >= 0 {
		cleaned = cleaned[idx+3:]
		// Remove optional language tag on the opening fence.
		if nl := strings.Index(cleaned, "\n"); nl >= 0 {
			cleaned = cleaned[nl+1:]
		}
		if end := strings.Index(cleaned, "```"); end >= 0 {
			cleaned = cleaned[:end]
		}
		cleaned = strings.TrimSpace(cleaned)
		if err := json.Unmarshal([]byte(cleaned), &actions); err == nil {
			return actions, nil
		}
	}

	// Search for the first '[' ... last ']' substring (embedded JSON array).
	if start := strings.Index(raw, "["); start >= 0 {
		if end := strings.LastIndex(raw, "]"); end > start {
			fragment := raw[start : end+1]
			if err := json.Unmarshal([]byte(fragment), &actions); err == nil {
				return actions, nil
			}
		}
	}

	return nil, fmt.Errorf("no valid JSON array found in LLM response")
}

// buildClusterContext gathers live cluster state and serializes it as JSON
// for injection into the AI system prompt. Intentionally lightweight —
// only summaries are sent, not full YAML manifests, to stay within token limits.
func (e *Engine) buildClusterContext(ctx context.Context) (string, error) {
	e.mu.RLock()
	k8sClient := e.k8s
	e.mu.RUnlock()

	type clusterState struct {
		CrashingPods  []k8s.PodSummary        `json:"crashing_pods"`
		Deployments   []k8s.DeploymentSummary  `json:"deployments"`
		PressureNodes []k8s.NodeSummary        `json:"pressure_nodes"`
	}

	crashingPods, err := k8sClient.ListCrashingPods(ctx, "")
	if err != nil {
		return "", fmt.Errorf("listing crashing pods: %w", err)
	}

	deployments, err := k8sClient.ListDeployments(ctx, "")
	if err != nil {
		return "", fmt.Errorf("listing deployments: %w", err)
	}

	pressureNodes, err := k8sClient.ListPressureNodes(ctx)
	if err != nil {
		return "", fmt.Errorf("listing pressure nodes: %w", err)
	}

	state := clusterState{
		CrashingPods:  crashingPods,
		Deployments:   deployments,
		PressureNodes: pressureNodes,
	}

	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshalling cluster state: %w", err)
	}
	return string(raw), nil
}
