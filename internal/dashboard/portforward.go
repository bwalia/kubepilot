package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/k8s"
)

// PortForwardSession describes an active or terminated port-forward session.
type PortForwardSession struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"` // "pod" | "service"
	Namespace  string    `json:"namespace"`
	Name       string    `json:"name"`
	LocalPort  uint16    `json:"local_port"`
	RemotePort uint16    `json:"remote_port"`
	Address    string    `json:"address"`    // "localhost:<local_port>" (on the server host)
	ProxyPath  string    `json:"proxy_path"` // reachable HTTP proxy path via the dashboard port
	Status     string    `json:"status"`     // "active" | "stopped" | "error"
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"created_at"`

	stop func()
}

// PortForwardManager owns all port-forward sessions for the lifetime of the
// dashboard server process.
type PortForwardManager struct {
	mu       sync.Mutex
	sessions map[string]*PortForwardSession
	log      *zap.Logger
}

// NewPortForwardManager creates an empty manager.
func NewPortForwardManager(log *zap.Logger) *PortForwardManager {
	return &PortForwardManager{
		sessions: make(map[string]*PortForwardSession),
		log:      log,
	}
}

// Start resolves the target (service → backing pod when needed), begins a
// kubectl-style port-forward, registers the session, and watches for the
// forward to terminate so the session status is kept current.
func (m *PortForwardManager) Start(ctx context.Context, k8sClient *k8s.Client, kind, namespace, name string, remotePort int32) (*PortForwardSession, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("namespace and name are required")
	}

	// Dedupe: return an existing active session for the same target+port.
	m.mu.Lock()
	for _, s := range m.sessions {
		if s.Status == "active" && s.Kind == kind && s.Namespace == namespace &&
			s.Name == name && int32(s.RemotePort) == remotePort {
			m.mu.Unlock()
			return s, nil
		}
	}
	m.mu.Unlock()

	var target k8s.ForwardTarget
	switch kind {
	case "service":
		resolved, err := k8sClient.ResolveServiceForwardTarget(ctx, namespace, name, remotePort)
		if err != nil {
			return nil, err
		}
		target = *resolved
	case "pod":
		port := uint16(remotePort)
		if remotePort == 0 {
			first, err := k8sClient.FirstPodContainerPort(ctx, namespace, name)
			if err != nil {
				return nil, err
			}
			port = first
		}
		target = k8s.ForwardTarget{Namespace: namespace, PodName: name, RemotePort: port}
	default:
		return nil, fmt.Errorf("unsupported port-forward kind %q (want pod or service)", kind)
	}

	result, err := k8sClient.StartPodPortForward(ctx, target, 10*time.Second)
	if err != nil {
		return nil, err
	}

	session := &PortForwardSession{
		ID:         uuid.NewString(),
		Kind:       kind,
		Namespace:  namespace,
		Name:       name,
		LocalPort:  result.LocalPort,
		RemotePort: result.RemotePort,
		Address:    fmt.Sprintf("localhost:%d", result.LocalPort),
		Status:     "active",
		CreatedAt:  time.Now().UTC(),
		stop:       result.Stop,
	}
	// Expose an HTTP reverse-proxy path reachable through the dashboard port —
	// this works even when KubePilot runs in a container where the bound local
	// port itself is not published to the host.
	session.ProxyPath = fmt.Sprintf("/api/v1/forward/%s/", session.ID)

	m.mu.Lock()
	m.sessions[session.ID] = session
	m.mu.Unlock()

	// Watch for the forward to end (clean stop or connection drop).
	go func() {
		fwdErr := <-result.ForwardErr
		m.mu.Lock()
		defer m.mu.Unlock()
		s, ok := m.sessions[session.ID]
		if !ok {
			return
		}
		if fwdErr != nil && fwdErr.Error() != "" {
			s.Status = "error"
			s.Error = fwdErr.Error()
		} else if s.Status == "active" {
			s.Status = "stopped"
		}
	}()

	return session, nil
}

// List returns a snapshot of all known sessions.
func (m *PortForwardManager) List() []*PortForwardSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*PortForwardSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s)
	}
	return out
}

// Get returns a session by ID, or nil if it does not exist.
func (m *PortForwardManager) Get(id string) *PortForwardSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[id]
}

// Stop terminates a session and removes it from the registry.
func (m *PortForwardManager) Stop(id string) error {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("port-forward session %q not found", id)
	}
	delete(m.sessions, id)
	m.mu.Unlock()

	if s.stop != nil {
		s.stop()
	}
	return nil
}

// StopAll terminates every active session; called on server shutdown.
func (m *PortForwardManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		if s.Status == "active" && s.stop != nil {
			s.stop()
			s.Status = "stopped"
		}
	}
}

// ── HTTP handlers ────────────────────────────────────────────────────────────

func (s *Server) handleStartPortForward(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind       string `json:"kind"`
		Namespace  string `json:"namespace"`
		Name       string `json:"name"`
		RemotePort int32  `json:"remote_port,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, fmt.Errorf("invalid request body: %w", err), http.StatusBadRequest)
		return
	}

	session, err := s.pfManager.Start(r.Context(), s.currentK8sClient(), req.Kind, req.Namespace, req.Name, req.RemotePort)
	if err != nil {
		httpError(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, session)
}

func (s *Server) handleListPortForwards(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, s.pfManager.List())
}

func (s *Server) handleStopPortForward(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := s.pfManager.Stop(id); err != nil {
		httpError(w, err, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleForwardProxy reverse-proxies HTTP requests through the dashboard port to
// a session's local forward listener (127.0.0.1:<local_port>). This makes a
// forwarded HTTP service reachable even when KubePilot runs in a container where
// the auto-assigned local port is not published to the host. It is HTTP-only —
// raw TCP protocols (databases, DNS) cannot be tunnelled through a browser.
func (s *Server) handleForwardProxy(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	session := s.pfManager.Get(id)
	if session == nil {
		httpError(w, fmt.Errorf("port-forward session %q not found", id), http.StatusNotFound)
		return
	}
	if session.Status != "active" {
		httpError(w, fmt.Errorf("port-forward session %q is %s", id, session.Status), http.StatusBadGateway)
		return
	}

	target := &url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", session.LocalPort)}
	prefix := fmt.Sprintf("/api/v1/forward/%s", id)

	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		// Strip the proxy prefix so the upstream sees a normal path.
		req.URL.Path = strings.TrimPrefix(req.URL.Path, prefix)
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}
		req.Host = target.Host
	}
	proxy.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, err error) {
		httpError(rw, fmt.Errorf("forwarding to %s failed: %w", target.Host, err), http.StatusBadGateway)
	}
	proxy.ServeHTTP(w, r)
}
