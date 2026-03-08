package agents

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/ai"
	"github.com/kubepilot/kubepilot/pkg/k8s"
)

// LogAgent is a specialized agent for log analysis.
type LogAgent struct {
	analyzer *ai.LogAnalyzer
	log      *zap.Logger
}

// NewLogAgent creates a log analysis agent.
func NewLogAgent(engine *ai.Engine, k8sClient *k8s.Client, log *zap.Logger) *LogAgent {
	return &LogAgent{
		analyzer: ai.NewLogAnalyzer(engine, k8sClient, log),
		log:      log,
	}
}

func (a *LogAgent) ID() string   { return "log-analyzer" }
func (a *LogAgent) Name() string { return "Log Analyzer Agent" }
func (a *LogAgent) Capabilities() []string {
	return []string{"analyze_pod_logs", "detect_error_patterns", "summarize_logs"}
}

// LogAnalyzeRequest is the payload for a log analysis request.
type LogAnalyzeRequest struct {
	Namespace string `json:"namespace"`
	PodName   string `json:"pod_name"`
	Container string `json:"container,omitempty"`
	TailLines int64  `json:"tail_lines,omitempty"`
}

func (a *LogAgent) Handle(ctx context.Context, msg AgentMessage) (AgentMessage, error) {
	switch msg.Type {
	case "analyze":
		return a.handleAnalyze(ctx, msg)
	default:
		return AgentMessage{
			Type:    "error",
			AgentID: a.ID(),
			Payload: mustMarshal(map[string]string{"error": fmt.Sprintf("unsupported message type: %s", msg.Type)}),
		}, nil
	}
}

func (a *LogAgent) handleAnalyze(ctx context.Context, msg AgentMessage) (AgentMessage, error) {
	var req LogAnalyzeRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return AgentMessage{
			Type:    "error",
			AgentID: a.ID(),
			Payload: mustMarshal(map[string]string{"error": fmt.Sprintf("invalid payload: %v", err)}),
		}, nil
	}

	analysis, err := a.analyzer.AnalyzePodLogs(ctx, req.Namespace, req.PodName, req.Container, req.TailLines)
	if err != nil {
		return AgentMessage{
			Type:    "error",
			AgentID: a.ID(),
			Payload: mustMarshal(map[string]string{"error": err.Error()}),
		}, nil
	}

	return AgentMessage{
		Type:    "report",
		AgentID: a.ID(),
		Payload: mustMarshal(analysis),
		ReplyTo: msg.ReplyTo,
	}, nil
}
