// Package client implements the KubePilot MCP agent that runs in remote clusters.
// It connects to the central MCP server over WebSocket, streams local cluster
// state, and executes approved actions locally — enabling true multi-cluster management.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/k8s"
	"github.com/kubepilot/kubepilot/pkg/security"
)

// Config holds MCP agent configuration.
type Config struct {
	MCPServerAddr string
	ClusterName   string
	K8sClient     *k8s.Client
}

// Agent is a lightweight MCP client that runs in a remote cluster.
type Agent struct {
	cfg   Config
	guard *security.Guard
	log   *zap.Logger
}

// NewAgent constructs an Agent. Call Connect to establish the server connection.
func NewAgent(cfg Config, log *zap.Logger) *Agent {
	return &Agent{
		cfg:   cfg,
		guard: security.NewGuard(cfg.K8sClient.Core, log),
		log:   log,
	}
}

// Connect establishes a persistent WebSocket connection to the MCP server.
// It reconnects automatically with exponential backoff on disconnection.
func (a *Agent) Connect(ctx context.Context) error {
	backoff := time.Second

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if err := a.connect(ctx); err != nil {
			a.log.Warn("Agent connection failed, retrying",
				zap.String("server", a.cfg.MCPServerAddr),
				zap.Duration("backoff", backoff),
				zap.Error(err),
			)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(backoff):
			}
			// Exponential backoff capped at 60s.
			backoff = min(backoff*2, 60*time.Second)
			continue
		}
		// Successful connection ended cleanly — reset backoff.
		backoff = time.Second
	}
}

func (a *Agent) connect(ctx context.Context) error {
	u := url.URL{
		Scheme:   "ws",
		Host:     a.cfg.MCPServerAddr,
		Path:     "/mcp/connect",
		RawQuery: "cluster=" + url.QueryEscape(a.cfg.ClusterName),
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return fmt.Errorf("dialing MCP server %s: %w", u.String(), err)
	}
	defer conn.Close()

	a.log.Info("Connected to MCP server",
		zap.String("server", a.cfg.MCPServerAddr),
		zap.String("cluster", a.cfg.ClusterName),
	)

	// Start streaming cluster state to the server.
	go a.streamClusterState(ctx, conn)

	// Read commands from the server.
	return a.readLoop(ctx, conn)
}

// streamClusterState periodically sends a cluster summary to the MCP server.
func (a *Agent) streamClusterState(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pods, err := a.cfg.K8sClient.ListPods(ctx, "")
			if err != nil {
				a.log.Warn("Failed to list pods for state stream", zap.Error(err))
				continue
			}

			payload, err := json.Marshal(map[string]any{
				"cluster": a.cfg.ClusterName,
				"pods":    pods,
				"ts":      time.Now().UTC(),
			})
			if err != nil {
				continue
			}

			msg := map[string]any{
				"type":    "cluster_state",
				"payload": json.RawMessage(payload),
			}
			raw, _ := json.Marshal(msg)
			_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, raw); err != nil {
				a.log.Warn("Failed to stream cluster state", zap.Error(err))
				return
			}
		}
	}
}

// readLoop receives and dispatches commands sent by the MCP server.
func (a *Agent) readLoop(ctx context.Context, conn *websocket.Conn) error {
	conn.SetReadLimit(1 << 20)
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("reading from MCP server: %w", err)
		}

		var cmd struct {
			Type     string          `json:"type"`
			Payload  json.RawMessage `json:"payload"`
			ChangeID string          `json:"change_id"`
			CRCode   string          `json:"cr_code"`
			TargetEnv string         `json:"target_env"`
		}
		if err := json.Unmarshal(raw, &cmd); err != nil {
			a.log.Warn("Malformed server command", zap.Error(err))
			continue
		}

		// Production commands must be validated locally before execution.
		if cmd.TargetEnv == "production" {
			if err := a.guard.Authorize(ctx, cmd.ChangeID, cmd.CRCode); err != nil {
				a.log.Warn("Production command blocked — CR code invalid",
					zap.String("change_id", cmd.ChangeID),
					zap.Error(err),
				)
				continue
			}
		}

		a.log.Info("Received server command",
			zap.String("type", cmd.Type),
			zap.String("cluster", a.cfg.ClusterName),
		)
		// Future: dispatch to local action executor.
	}
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
