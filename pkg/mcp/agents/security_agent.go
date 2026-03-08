package agents

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/k8s"
)

// SecurityAgent audits RBAC and security posture.
type SecurityAgent struct {
	k8s *k8s.Client
	log *zap.Logger
}

// NewSecurityAgent creates a security auditing agent.
func NewSecurityAgent(k8sClient *k8s.Client, log *zap.Logger) *SecurityAgent {
	return &SecurityAgent{
		k8s: k8sClient,
		log: log,
	}
}

func (a *SecurityAgent) ID() string   { return "security-auditor" }
func (a *SecurityAgent) Name() string { return "Security Auditor Agent" }
func (a *SecurityAgent) Capabilities() []string {
	return []string{"list_roles", "list_cluster_roles", "list_role_bindings", "audit_rbac"}
}

// SecurityRequest is the payload for a security analysis request.
type SecurityRequest struct {
	Action    string `json:"action"` // "roles", "cluster_roles", "role_bindings", "cluster_role_bindings"
	Namespace string `json:"namespace,omitempty"`
}

func (a *SecurityAgent) Handle(ctx context.Context, msg AgentMessage) (AgentMessage, error) {
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

func (a *SecurityAgent) handleQuery(ctx context.Context, msg AgentMessage) (AgentMessage, error) {
	var req SecurityRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return AgentMessage{
			Type:    "error",
			AgentID: a.ID(),
			Payload: mustMarshal(map[string]string{"error": fmt.Sprintf("invalid payload: %v", err)}),
		}, nil
	}

	switch req.Action {
	case "roles":
		roles, err := a.k8s.ListRoles(ctx, req.Namespace)
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
			Payload: mustMarshal(roles),
			ReplyTo: msg.ReplyTo,
		}, nil

	case "cluster_roles":
		roles, err := a.k8s.ListClusterRoles(ctx)
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
			Payload: mustMarshal(roles),
			ReplyTo: msg.ReplyTo,
		}, nil

	case "role_bindings":
		bindings, err := a.k8s.ListRoleBindings(ctx, req.Namespace)
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
			Payload: mustMarshal(bindings),
			ReplyTo: msg.ReplyTo,
		}, nil

	case "cluster_role_bindings":
		bindings, err := a.k8s.ListClusterRoleBindings(ctx)
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
			Payload: mustMarshal(bindings),
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
