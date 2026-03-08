package agents

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/k8s"
)

// MetricAgent analyzes resource usage metrics for pods and nodes.
type MetricAgent struct {
	k8s *k8s.Client
	log *zap.Logger
}

// NewMetricAgent creates a metric analysis agent.
func NewMetricAgent(k8sClient *k8s.Client, log *zap.Logger) *MetricAgent {
	return &MetricAgent{
		k8s: k8sClient,
		log: log,
	}
}

func (a *MetricAgent) ID() string   { return "metric-analyzer" }
func (a *MetricAgent) Name() string { return "Metric Analyzer Agent" }
func (a *MetricAgent) Capabilities() []string {
	return []string{"pod_resource_usage", "node_resource_usage", "resource_recommendations"}
}

// MetricRequest is the payload for a metric analysis request.
type MetricRequest struct {
	Type      string `json:"type"` // "pod" or "node"
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
}

func (a *MetricAgent) Handle(ctx context.Context, msg AgentMessage) (AgentMessage, error) {
	switch msg.Type {
	case "query":
		return a.handleQuery(ctx, msg)
	default:
		return AgentMessage{
			Type:    "error",
			AgentID: a.ID(),
			Payload: mustMarshal(map[string]string{"error": fmt.Sprintf("unsupported message type: %s", msg.Type)}),
		}, nil
	}
}

func (a *MetricAgent) handleQuery(ctx context.Context, msg AgentMessage) (AgentMessage, error) {
	var req MetricRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return AgentMessage{
			Type:    "error",
			AgentID: a.ID(),
			Payload: mustMarshal(map[string]string{"error": fmt.Sprintf("invalid payload: %v", err)}),
		}, nil
	}

	switch req.Type {
	case "pod":
		metrics, err := a.k8s.GetPodResourceMetrics(ctx, req.Namespace, req.Name)
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
			Payload: mustMarshal(metrics),
			ReplyTo: msg.ReplyTo,
		}, nil

	case "node":
		metrics, err := a.k8s.GetNodeResourceMetrics(ctx)
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
			Payload: mustMarshal(metrics),
			ReplyTo: msg.ReplyTo,
		}, nil

	default:
		return AgentMessage{
			Type:    "error",
			AgentID: a.ID(),
			Payload: mustMarshal(map[string]string{"error": fmt.Sprintf("unknown metric type: %s", req.Type)}),
		}, nil
	}
}
