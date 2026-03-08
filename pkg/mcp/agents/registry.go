// Package agents implements specialized AI agents for KubePilot's MCP protocol.
// Each agent handles a specific domain: log analysis, metrics, network debugging, security auditing.
package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// AgentMessage is the wire format for inter-agent communication.
type AgentMessage struct {
	Type    string          `json:"type"`    // "analyze", "query", "report", "error"
	AgentID string          `json:"agent_id"`
	Payload json.RawMessage `json:"payload"`
	ReplyTo string          `json:"reply_to,omitempty"` // For response routing.
}

// Agent is the interface for all specialized AI agents.
type Agent interface {
	ID() string
	Name() string
	Capabilities() []string
	Handle(ctx context.Context, msg AgentMessage) (AgentMessage, error)
}

// Registry manages the set of available agents and routes messages to them.
type Registry struct {
	agents map[string]Agent
	mu     sync.RWMutex
	log    *zap.Logger
}

// NewRegistry creates an empty agent registry.
func NewRegistry(log *zap.Logger) *Registry {
	return &Registry{
		agents: make(map[string]Agent),
		log:    log,
	}
}

// Register adds an agent to the registry.
func (r *Registry) Register(agent Agent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[agent.ID()] = agent
	r.log.Info("Agent registered",
		zap.String("id", agent.ID()),
		zap.String("name", agent.Name()),
		zap.Strings("capabilities", agent.Capabilities()),
	)
}

// Get returns an agent by ID.
func (r *Registry) Get(id string) (Agent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, ok := r.agents[id]
	return agent, ok
}

// List returns all registered agents.
func (r *Registry) List() []AgentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	infos := make([]AgentInfo, 0, len(r.agents))
	for _, agent := range r.agents {
		infos = append(infos, AgentInfo{
			ID:           agent.ID(),
			Name:         agent.Name(),
			Capabilities: agent.Capabilities(),
		})
	}
	return infos
}

// AgentInfo is a summary of an agent for API responses.
type AgentInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Capabilities []string `json:"capabilities"`
}

// Dispatch routes a message to the appropriate agent based on AgentID.
func (r *Registry) Dispatch(ctx context.Context, msg AgentMessage) (AgentMessage, error) {
	agent, ok := r.Get(msg.AgentID)
	if !ok {
		return AgentMessage{
			Type:    "error",
			AgentID: "registry",
			Payload: mustMarshal(map[string]string{"error": fmt.Sprintf("agent %q not found", msg.AgentID)}),
		}, fmt.Errorf("agent %q not found", msg.AgentID)
	}

	r.log.Debug("Dispatching message to agent",
		zap.String("agent", msg.AgentID),
		zap.String("type", msg.Type),
	)

	return agent.Handle(ctx, msg)
}

func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
