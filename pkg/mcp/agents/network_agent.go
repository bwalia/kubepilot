package agents

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/k8s"
)

// NetworkAgent debugs network connectivity issues.
type NetworkAgent struct {
	k8s *k8s.Client
	log *zap.Logger
}

// NewNetworkAgent creates a network debugging agent.
func NewNetworkAgent(k8sClient *k8s.Client, log *zap.Logger) *NetworkAgent {
	return &NetworkAgent{
		k8s: k8sClient,
		log: log,
	}
}

func (a *NetworkAgent) ID() string   { return "network-debugger" }
func (a *NetworkAgent) Name() string { return "Network Debugger Agent" }
func (a *NetworkAgent) Capabilities() []string {
	return []string{"list_network_policies", "list_services", "check_endpoints", "discover_dependencies"}
}

// NetworkRequest is the payload for a network analysis request.
type NetworkRequest struct {
	Action    string `json:"action"` // "policies", "services", "dependencies"
	Namespace string `json:"namespace"`
}

func (a *NetworkAgent) Handle(ctx context.Context, msg AgentMessage) (AgentMessage, error) {
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

func (a *NetworkAgent) handleQuery(ctx context.Context, msg AgentMessage) (AgentMessage, error) {
	var req NetworkRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return AgentMessage{
			Type:    "error",
			AgentID: a.ID(),
			Payload: mustMarshal(map[string]string{"error": fmt.Sprintf("invalid payload: %v", err)}),
		}, nil
	}

	switch req.Action {
	case "policies":
		policies, err := a.k8s.ListNetworkPolicies(ctx, req.Namespace)
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
			Payload: mustMarshal(policies),
			ReplyTo: msg.ReplyTo,
		}, nil

	case "services":
		services, err := a.k8s.ListServices(ctx, req.Namespace)
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
			Payload: mustMarshal(services),
			ReplyTo: msg.ReplyTo,
		}, nil

	case "dependencies":
		deps, err := a.k8s.DiscoverServiceDependencies(ctx, req.Namespace)
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
			Payload: mustMarshal(deps),
			ReplyTo: msg.ReplyTo,
		}, nil

	default:
		return AgentMessage{
			Type:    "error",
			AgentID: a.ID(),
			Payload: mustMarshal(map[string]string{"error": fmt.Sprintf("unknown action: %s", req.Action)}),
		}, nil
	}
}
