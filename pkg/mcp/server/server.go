// Package server implements the KubePilot MCP (Multi-Cluster Protocol) server.
// Remote agents connect to this server over WebSocket to register clusters,
// stream cluster state, and receive orchestration commands.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/ai"
	"github.com/kubepilot/kubepilot/pkg/jobs"
	"github.com/kubepilot/kubepilot/pkg/k8s"
	"github.com/kubepilot/kubepilot/pkg/mcp/agents"
)

// Config holds MCP server configuration.
type Config struct {
	Port      int
	AIEngine  *ai.Engine
	Scheduler *jobs.Scheduler
	K8sClient *k8s.Client
}

// AgentConn represents a connected remote cluster agent.
type AgentConn struct {
	ClusterName string
	conn        *websocket.Conn
	send        chan []byte
}

// Message is the wire format for MCP protocol messages.
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Server is the central MCP coordination server.
type Server struct {
	cfg      Config
	agents   map[string]*AgentConn // keyed by cluster name
	mu       sync.RWMutex
	log      *zap.Logger
	upgrader websocket.Upgrader
	registry *agents.Registry
}

// New creates an MCP Server instance with all specialized agents registered.
func New(cfg Config, log *zap.Logger) *Server {
	// Initialize the agent registry with all specialized agents.
	registry := agents.NewRegistry(log)
	registry.Register(agents.NewLogAgent(cfg.AIEngine, cfg.K8sClient, log))
	registry.Register(agents.NewMetricAgent(cfg.K8sClient, log))
	registry.Register(agents.NewNetworkAgent(cfg.K8sClient, log))
	registry.Register(agents.NewSecurityAgent(cfg.K8sClient, log))

	return &Server{
		cfg:      cfg,
		agents:   make(map[string]*AgentConn),
		log:      log,
		registry: registry,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
			HandshakeTimeout: 10 * time.Second,
		},
	}
}

// Start begins listening for agent connections on the configured port.
func (s *Server) Start(ctx context.Context) error {
	router := mux.NewRouter()
	router.HandleFunc("/mcp/connect", s.handleAgentConnect)
	router.HandleFunc("/mcp/health", s.handleHealth)
	router.HandleFunc("/mcp/agents", s.handleListAgents)
	router.HandleFunc("/mcp/command", s.handleCommand).Methods(http.MethodPost)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.cfg.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	s.log.Sugar().Infof("MCP server listening on :%d", s.cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("MCP server error: %w", err)
	}
	return nil
}

// handleAgentConnect upgrades the HTTP connection to WebSocket and registers the agent.
func (s *Server) handleAgentConnect(w http.ResponseWriter, r *http.Request) {
	clusterName := r.URL.Query().Get("cluster")
	if clusterName == "" {
		http.Error(w, "cluster query parameter required", http.StatusBadRequest)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Error("WebSocket upgrade failed", zap.String("cluster", clusterName), zap.Error(err))
		return
	}

	agent := &AgentConn{
		ClusterName: clusterName,
		conn:        conn,
		send:        make(chan []byte, 256),
	}

	s.mu.Lock()
	s.agents[clusterName] = agent
	s.mu.Unlock()

	s.log.Info("Agent connected", zap.String("cluster", clusterName))

	go s.writePump(agent)
	s.readPump(agent) // Blocks until disconnected.

	s.mu.Lock()
	delete(s.agents, clusterName)
	s.mu.Unlock()
	s.log.Info("Agent disconnected", zap.String("cluster", clusterName))
}

func (s *Server) readPump(agent *AgentConn) {
	defer func() {
		agent.conn.Close()
		close(agent.send)
	}()
	agent.conn.SetReadLimit(1 << 20) // 1 MiB max message.
	_ = agent.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	agent.conn.SetPongHandler(func(string) error {
		return agent.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	for {
		_, raw, err := agent.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				s.log.Warn("Agent read error", zap.String("cluster", agent.ClusterName), zap.Error(err))
			}
			return
		}

		var msg Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			s.log.Warn("Malformed agent message", zap.String("cluster", agent.ClusterName), zap.Error(err))
			continue
		}
		s.handleAgentMessage(agent, msg)
	}
}

func (s *Server) writePump(agent *AgentConn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-agent.send:
			_ = agent.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = agent.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := agent.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			// Ping to keep the connection alive and detect stale agents.
			_ = agent.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := agent.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *Server) handleAgentMessage(agent *AgentConn, msg Message) {
	s.log.Debug("Agent message received",
		zap.String("cluster", agent.ClusterName),
		zap.String("type", msg.Type),
	)

	// Try to dispatch as an agent message if it has the right structure.
	var agentMsg agents.AgentMessage
	if err := json.Unmarshal(msg.Payload, &agentMsg); err == nil && agentMsg.AgentID != "" {
		ctx := context.Background()
		response, err := s.registry.Dispatch(ctx, agentMsg)
		if err != nil {
			s.log.Warn("Agent dispatch failed",
				zap.String("agent_id", agentMsg.AgentID),
				zap.Error(err),
			)
		}
		// Send response back to the originating agent connection.
		respData, _ := json.Marshal(response)
		select {
		case agent.send <- respData:
		default:
			s.log.Warn("Agent send buffer full", zap.String("cluster", agent.ClusterName))
		}
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleListAgents(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	clusters := make([]string, 0, len(s.agents))
	for name := range s.agents {
		clusters = append(clusters, name)
	}
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string][]string{"connected_clusters": clusters})
}

func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Command  string `json:"command"`
		Cluster  string `json:"cluster,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	actions, err := s.cfg.AIEngine.Interpret(r.Context(), req.Command)
	if err != nil {
		http.Error(w, fmt.Sprintf("AI error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"actions": actions,
	})
}
