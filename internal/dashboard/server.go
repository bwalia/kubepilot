// Package dashboard serves the KubePilot "Kubernetes Cockpit" UI and its REST/WebSocket API.
// The Go server embeds the pre-built Next.js static export and exposes all
// backend API endpoints that the dashboard UI consumes.
package dashboard

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/ai"
	"github.com/kubepilot/kubepilot/pkg/jobs"
	"github.com/kubepilot/kubepilot/pkg/k8s"
	"github.com/kubepilot/kubepilot/pkg/observability"
	"github.com/kubepilot/kubepilot/pkg/security"
)

// Config holds dashboard server configuration.
type Config struct {
	Port           int
	AIEngine       *ai.Engine
	Scheduler      *jobs.Scheduler
	K8sClient      *k8s.Client
	RCAStore       *observability.RCAStore
	KubeconfigPath string
}

// Server serves the Kubernetes Cockpit dashboard and REST API.
type Server struct {
	mu    sync.RWMutex
	cfg   Config
	guard *security.Guard
	log   *zap.Logger

	k8sClient            *k8s.Client
	activeKubeconfigPath string
	knownKubeconfigPaths map[string]struct{}
	stateFilePath        string
	uploadDir            string
}

// NewServer creates a dashboard Server.
func NewServer(cfg Config, log *zap.Logger) *Server {
	srv := &Server{
		cfg:                  cfg,
		guard:                security.NewGuard(cfg.K8sClient.Core, log),
		log:                  log,
		k8sClient:            cfg.K8sClient,
		activeKubeconfigPath: cfg.KubeconfigPath,
		knownKubeconfigPaths: map[string]struct{}{},
	}

	if cfg.KubeconfigPath != "" {
		srv.knownKubeconfigPaths[cfg.KubeconfigPath] = struct{}{}
	}

	stateFile, uploadDir, err := defaultStatePaths()
	if err == nil {
		srv.stateFilePath = stateFile
		srv.uploadDir = uploadDir
		if st, loadErr := loadKubeconfigState(stateFile); loadErr == nil {
			for _, p := range st.Paths {
				srv.knownKubeconfigPaths[p] = struct{}{}
			}
			if srv.activeKubeconfigPath == "" && st.ActivePath != "" {
				srv.activeKubeconfigPath = st.ActivePath
			}
		} else {
			log.Warn("Failed to load persisted kubeconfig state", zap.Error(loadErr))
		}
	} else {
		log.Warn("Failed to initialize kubeconfig state paths", zap.Error(err))
	}

	if srv.activeKubeconfigPath != "" {
		srv.knownKubeconfigPaths[srv.activeKubeconfigPath] = struct{}{}
		srv.persistKubeconfigStateLocked()
	}

	return srv
}

// Start begins serving on the configured port until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	router := mux.NewRouter()

	// ── API routes ──────────────────────────────────────────────────────────
	api := router.PathPrefix("/api/v1").Subrouter()

	// Cluster overview
	api.HandleFunc("/clusters/kubeconfigs", s.handleListKubeconfigs).Methods(http.MethodGet)
	api.HandleFunc("/clusters/kubeconfigs", s.handleAddKubeconfig).Methods(http.MethodPost)
	api.HandleFunc("/clusters/kubeconfigs/upload", s.handleUploadKubeconfig).Methods(http.MethodPost)
	api.HandleFunc("/clusters/kubeconfigs/base64", s.handleUploadKubeconfigBase64).Methods(http.MethodPost)
	api.HandleFunc("/clusters/switch", s.handleSwitchCluster).Methods(http.MethodPost)

	api.HandleFunc("/clusters/pods", s.handleListPods).Methods(http.MethodGet)
	api.HandleFunc("/clusters/pods/{namespace}/{pod}/diagnostics", s.handlePodDiagnostics).Methods(http.MethodGet)
	api.HandleFunc("/clusters/deployments", s.handleListDeployments).Methods(http.MethodGet)
	api.HandleFunc("/clusters/nodes", s.handleListNodes).Methods(http.MethodGet)
	api.HandleFunc("/clusters/crashing-pods", s.handleCrashingPods).Methods(http.MethodGet)
	api.HandleFunc("/clusters/service-graph", s.handleServiceGraph).Methods(http.MethodGet)
	api.HandleFunc("/events", s.handleListEvents).Methods(http.MethodGet)
	api.HandleFunc("/troubleshooting/summary", s.handleClusterTroubleshooting).Methods(http.MethodGet)

	// AI
	api.HandleFunc("/ai/interpret", s.handleAIInterpret).Methods(http.MethodPost)
	api.HandleFunc("/ai/troubleshoot/{namespace}/{pod}", s.handleTroubleshoot).Methods(http.MethodGet)
	api.HandleFunc("/ai/execute-action", s.handleExecuteSuggestedAction).Methods(http.MethodPost)

	// RCA & Anomalies
	api.HandleFunc("/rca", s.handleListRCAReports).Methods(http.MethodGet)
	api.HandleFunc("/rca/{id}", s.handleGetRCAReport).Methods(http.MethodGet)
	api.HandleFunc("/anomalies", s.handleListAnomalies).Methods(http.MethodGet)
	api.HandleFunc("/topology/{namespace}", s.handleTopology).Methods(http.MethodGet)
	api.HandleFunc("/remediate", s.handleRemediate).Methods(http.MethodPost)

	// Jobs (Jira-style job management)
	api.HandleFunc("/jobs", s.handleListJobs).Methods(http.MethodGet)
	api.HandleFunc("/jobs", s.handleSubmitJob).Methods(http.MethodPost)
	api.HandleFunc("/jobs/{id}", s.handleGetJob).Methods(http.MethodGet)
	api.HandleFunc("/jobs/{id}/cancel", s.handleCancelJob).Methods(http.MethodPost)

	// Production authorization
	api.HandleFunc("/crcode/authorize", s.handleCRCodeAuthorize).Methods(http.MethodPost)
	api.HandleFunc("/crcode/register", s.handleCRCodeRegister).Methods(http.MethodPost)
	api.HandleFunc("/crcode/revoke", s.handleCRCodeRevoke).Methods(http.MethodPost)

	// Health
	router.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// ── Static Next.js dashboard ─────────────────────────────────────────────
	// In production the dashboard is built to ./dashboard/out and served here.
	// During development, Next.js dev server runs separately on port 3000.
	router.PathPrefix("/").Handler(http.FileServer(http.Dir("./dashboard/out")))

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.cfg.Port),
		Handler:      withCORS(router),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute, // AI endpoints may take time with local LLMs.
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	s.log.Sugar().Infof("Dashboard listening on http://localhost:%d", s.cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("dashboard server error: %w", err)
	}
	return nil
}

// ─────────────────────────────────────────
// Cluster handlers
// ─────────────────────────────────────────

func (s *Server) handleListPods(w http.ResponseWriter, r *http.Request) {
	k8sClient := s.currentK8sClient()
	ns := r.URL.Query().Get("namespace")
	pods, err := k8sClient.ListPods(r.Context(), ns)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, pods)
}

func (s *Server) handleListDeployments(w http.ResponseWriter, r *http.Request) {
	k8sClient := s.currentK8sClient()
	ns := r.URL.Query().Get("namespace")
	deps, err := k8sClient.ListDeployments(r.Context(), ns)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, deps)
}

func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	k8sClient := s.currentK8sClient()
	nodes, err := k8sClient.ListNodes(r.Context())
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, nodes)
}

func (s *Server) handleCrashingPods(w http.ResponseWriter, r *http.Request) {
	k8sClient := s.currentK8sClient()
	ns := r.URL.Query().Get("namespace")
	pods, err := k8sClient.ListCrashingPods(r.Context(), ns)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, pods)
}

func (s *Server) handleServiceGraph(w http.ResponseWriter, r *http.Request) {
	k8sClient := s.currentK8sClient()
	ns := strings.TrimSpace(r.URL.Query().Get("namespace"))
	// Empty namespace (or "all") means list resources across all namespaces.
	if strings.EqualFold(ns, "all") {
		ns = ""
	}
	graph, err := k8sClient.GetServiceGraph(r.Context(), ns)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, graph)
}

// ─────────────────────────────────────────
// AI handlers
// ─────────────────────────────────────────

func (s *Server) handleAIInterpret(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Command string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, fmt.Errorf("invalid request body: %w", err), http.StatusBadRequest)
		return
	}
	if req.Command == "" {
		httpError(w, fmt.Errorf("command is required"), http.StatusBadRequest)
		return
	}

	actions, err := s.cfg.AIEngine.Interpret(r.Context(), req.Command)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"actions": actions})
}

func (s *Server) handleTroubleshoot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	report, err := s.cfg.AIEngine.TroubleshootPod(r.Context(), vars["namespace"], vars["pod"])
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, report)
}

func (s *Server) handleExecuteSuggestedAction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action   ai.SuggestedAction `json:"action"`
		ChangeID string             `json:"change_id,omitempty"`
		CRCode   string             `json:"cr_code,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, fmt.Errorf("invalid request body: %w", err), http.StatusBadRequest)
		return
	}

	if req.Action.Type == "" {
		httpError(w, fmt.Errorf("action.type is required"), http.StatusBadRequest)
		return
	}

	if req.Action.RequiresCRCode {
		guard := s.currentGuard()
		if err := guard.Authorize(r.Context(), req.ChangeID, req.CRCode); err != nil {
			httpError(w, err, http.StatusForbidden)
			return
		}
	}

	if req.Action.Command != "" {
		output, err := s.executeSuggestedCommand(r.Context(), req.Action.Command)
		if err != nil {
			httpError(w, fmt.Errorf("executing suggested command: %w", err), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{
			"status":  "executed",
			"message": output,
		})
		return
	}

	k8sClient := s.currentK8sClient()
	switch req.Action.Type {
	case ai.ActionRestart:
		if req.Action.Namespace == "" || req.Action.Resource == "" {
			httpError(w, fmt.Errorf("restart action requires namespace and resource"), http.StatusBadRequest)
			return
		}
		if err := k8sClient.RestartDeployment(r.Context(), req.Action.Namespace, req.Action.Resource); err != nil {
			httpError(w, err, http.StatusBadRequest)
			return
		}
	case ai.ActionScale:
		if req.Action.Namespace == "" || req.Action.Resource == "" {
			httpError(w, fmt.Errorf("scale action requires namespace and resource"), http.StatusBadRequest)
			return
		}
		if err := k8sClient.ScaleDeployment(r.Context(), req.Action.Namespace, req.Action.Resource, req.Action.Replicas); err != nil {
			httpError(w, err, http.StatusBadRequest)
			return
		}
	case ai.ActionDeletePod:
		if req.Action.Namespace == "" || req.Action.Resource == "" {
			httpError(w, fmt.Errorf("delete_pod action requires namespace and resource"), http.StatusBadRequest)
			return
		}
		if err := k8sClient.DeletePod(r.Context(), req.Action.Namespace, req.Action.Resource); err != nil {
			httpError(w, err, http.StatusBadRequest)
			return
		}
	case ai.ActionInvestigate, ai.ActionNoOp:
		writeJSON(w, map[string]any{
			"status":  "skipped",
			"message": "Action is informational and does not require execution",
		})
		return
	default:
		httpError(w, fmt.Errorf("unsupported action type %q; provide action.command for command-based fixes", req.Action.Type), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]any{
		"status":  "executed",
		"message": "Action executed successfully",
	})
}

// ─────────────────────────────────────────
// Job handlers
// ─────────────────────────────────────────

func (s *Server) handleListJobs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, s.cfg.Scheduler.ListJobs())
}

func (s *Server) handleSubmitJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		Command   string `json:"command"`
		Schedule  string `json:"schedule,omitempty"`
		TargetEnv string `json:"target_environment"`
		ChangeID  string `json:"change_id,omitempty"`
		CRCode    string `json:"cr_code,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, fmt.Errorf("invalid request body: %w", err), http.StatusBadRequest)
		return
	}

	job := &jobs.Job{
		Name:      req.Name,
		Command:   req.Command,
		Schedule:  req.Schedule,
		TargetEnv: req.TargetEnv,
		ChangeID:  req.ChangeID,
		CRCode:    req.CRCode,
	}

	if err := s.cfg.Scheduler.Submit(r.Context(), job); err != nil {
		httpError(w, err, http.StatusForbidden)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, job)
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	job, ok := s.cfg.Scheduler.GetJob(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, job)
}

func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := s.cfg.Scheduler.CancelJob(id); err != nil {
		httpError(w, err, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─────────────────────────────────────────
// CR Code handlers
// ─────────────────────────────────────────

func (s *Server) handleCRCodeAuthorize(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChangeID string `json:"change_id"`
		CRCode   string `json:"cr_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, fmt.Errorf("invalid request: %w", err), http.StatusBadRequest)
		return
	}
	if err := s.guard.Authorize(r.Context(), req.ChangeID, req.CRCode); err != nil {
		httpError(w, err, http.StatusForbidden)
		return
	}
	writeJSON(w, map[string]string{"status": "authorized"})
}

func (s *Server) handleCRCodeRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChangeID  string     `json:"change_id"`
		CRCode    string     `json:"cr_code"`
		ExpiresAt *time.Time `json:"expires_at,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, fmt.Errorf("invalid request: %w", err), http.StatusBadRequest)
		return
	}
	if err := s.guard.RegisterCRCode(r.Context(), req.ChangeID, req.CRCode, req.ExpiresAt); err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]string{"status": "registered"})
}

func (s *Server) handleCRCodeRevoke(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChangeID string `json:"change_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, fmt.Errorf("invalid request: %w", err), http.StatusBadRequest)
		return
	}
	if err := s.guard.RevokeCRCode(r.Context(), req.ChangeID); err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "revoked"})
}

// ─────────────────────────────────────────
// RCA & Anomaly handlers
// ─────────────────────────────────────────

func (s *Server) handleListRCAReports(w http.ResponseWriter, r *http.Request) {
	if s.cfg.RCAStore == nil {
		writeJSON(w, []struct{}{})
		return
	}
	filter := observability.ReportFilter{
		Severity:  r.URL.Query().Get("severity"),
		Namespace: r.URL.Query().Get("namespace"),
		Status:    r.URL.Query().Get("status"),
	}
	if since := r.URL.Query().Get("since"); since != "" {
		if d, err := time.ParseDuration(since); err == nil {
			filter.Since = time.Now().Add(-d)
		}
	}
	reports := s.cfg.RCAStore.ListReports(filter)
	if reports == nil {
		reports = make([]*ai.RCAReport, 0)
	}
	writeJSON(w, reports)
}

func (s *Server) handleGetRCAReport(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if s.cfg.RCAStore == nil {
		http.NotFound(w, r)
		return
	}
	report := s.cfg.RCAStore.GetReport(id)
	if report == nil {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, report)
}

func (s *Server) handleListAnomalies(w http.ResponseWriter, r *http.Request) {
	if s.cfg.RCAStore == nil {
		writeJSON(w, []struct{}{})
		return
	}
	filter := observability.AnomalyFilter{
		Severity:  r.URL.Query().Get("severity"),
		Namespace: r.URL.Query().Get("namespace"),
	}
	if since := r.URL.Query().Get("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			filter.Since = t
		} else if d, err := time.ParseDuration(since); err == nil {
			filter.Since = time.Now().Add(-d)
		}
	}
	anomalies := s.cfg.RCAStore.ListAnomalies(filter)
	if anomalies == nil {
		anomalies = make([]*observability.Anomaly, 0)
	}
	writeJSON(w, anomalies)
}

func (s *Server) handleTopology(w http.ResponseWriter, r *http.Request) {
	ns := mux.Vars(r)["namespace"]
	k8sClient := s.currentK8sClient()
	correlationEngine := ai.NewCorrelationEngine(s.cfg.AIEngine, k8sClient, s.log)
	topology, err := correlationEngine.BuildTopology(r.Context(), ns)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, topology)
}

func (s *Server) handleRemediate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ReportID string `json:"report_id"`
		Step     int    `json:"step"`
		CRCode   string `json:"cr_code,omitempty"`
		ChangeID string `json:"change_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, fmt.Errorf("invalid request: %w", err), http.StatusBadRequest)
		return
	}

	if s.cfg.RCAStore == nil {
		httpError(w, fmt.Errorf("RCA store not available"), http.StatusServiceUnavailable)
		return
	}

	report := s.cfg.RCAStore.GetReport(req.ReportID)
	if report == nil {
		httpError(w, fmt.Errorf("RCA report %q not found", req.ReportID), http.StatusNotFound)
		return
	}

	if req.Step < 0 || req.Step >= len(report.Remediation) {
		httpError(w, fmt.Errorf("step index %d out of range", req.Step), http.StatusBadRequest)
		return
	}

	step := report.Remediation[req.Step]
	k8sClient := s.currentK8sClient()
	guard := s.currentGuard()
	executor := ai.NewRemediationExecutor(k8sClient, guard, ai.RemediationConfig{DryRun: true}, s.log)
	result, err := executor.ExecuteStep(r.Context(), step, req.ChangeID, req.CRCode)
	if err != nil {
		httpError(w, err, http.StatusForbidden)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleListKubeconfigs(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	active := s.activeKubeconfigPath
	paths := make([]string, 0, len(s.knownKubeconfigPaths))
	for p := range s.knownKubeconfigPaths {
		paths = append(paths, p)
	}
	s.mu.RUnlock()
	sort.Strings(paths)
	writeJSON(w, map[string]any{
		"active_path": active,
		"paths":       paths,
	})
}

func (s *Server) handleAddKubeconfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path     string `json:"path"`
		Activate *bool  `json:"activate,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, fmt.Errorf("invalid request body: %w", err), http.StatusBadRequest)
		return
	}

	path := expandKubeconfigPath(req.Path)
	if path == "" {
		httpError(w, fmt.Errorf("path is required"), http.StatusBadRequest)
		return
	}
	if !filepath.IsAbs(path) {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}
	if _, err := os.Stat(path); err != nil {
		httpError(w, fmt.Errorf("kubeconfig path is not accessible: %w", err), http.StatusBadRequest)
		return
	}

	activate := true
	if req.Activate != nil {
		activate = *req.Activate
	}

	if activate {
		if err := s.switchCluster(path); err != nil {
			httpError(w, err, http.StatusBadRequest)
			return
		}
	} else {
		s.mu.Lock()
		s.knownKubeconfigPaths[path] = struct{}{}
		s.persistKubeconfigStateLocked()
		s.mu.Unlock()
	}

	s.handleListKubeconfigs(w, r)
}

func (s *Server) handleUploadKubeconfig(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		httpError(w, fmt.Errorf("invalid multipart form: %w", err), http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("kubeconfig")
	if err != nil {
		httpError(w, fmt.Errorf("kubeconfig file is required: %w", err), http.StatusBadRequest)
		return
	}
	defer file.Close()

	if s.uploadDir == "" {
		httpError(w, fmt.Errorf("kubeconfig upload directory is not configured"), http.StatusServiceUnavailable)
		return
	}

	baseName := filepath.Base(strings.TrimSpace(header.Filename))
	if baseName == "." || baseName == string(filepath.Separator) || baseName == "" {
		baseName = fmt.Sprintf("uploaded-%d.kubeconfig", time.Now().Unix())
	}
	raw, err := io.ReadAll(file)
	if err != nil {
		httpError(w, fmt.Errorf("reading kubeconfig file: %w", err), http.StatusBadRequest)
		return
	}
	dstPath, err := s.saveKubeconfigBytes(baseName, raw)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}

	if err := s.switchCluster(dstPath); err != nil {
		httpError(w, err, http.StatusBadRequest)
		return
	}

	s.handleListKubeconfigs(w, r)
}

func (s *Server) handleUploadKubeconfigBase64(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name          string `json:"name,omitempty"`
		ContentBase64 string `json:"content_base64"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, fmt.Errorf("invalid request body: %w", err), http.StatusBadRequest)
		return
	}

	encoded := strings.TrimSpace(req.ContentBase64)
	if encoded == "" {
		httpError(w, fmt.Errorf("content_base64 is required"), http.StatusBadRequest)
		return
	}

	if idx := strings.Index(encoded, ","); strings.HasPrefix(encoded, "data:") && idx > -1 {
		encoded = encoded[idx+1:]
	}
	encoded = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\n', '\r', '\t':
			return -1
		default:
			return r
		}
	}, encoded)

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(encoded)
		if err != nil {
			httpError(w, fmt.Errorf("invalid base64 kubeconfig content: %w", err), http.StatusBadRequest)
			return
		}
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = fmt.Sprintf("base64-%d.kubeconfig", time.Now().Unix())
	}
	dstPath, err := s.saveKubeconfigBytes(name, raw)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}

	if err := s.switchCluster(dstPath); err != nil {
		httpError(w, err, http.StatusBadRequest)
		return
	}

	s.handleListKubeconfigs(w, r)
}

func (s *Server) handleSwitchCluster(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, fmt.Errorf("invalid request body: %w", err), http.StatusBadRequest)
		return
	}
	if err := s.switchCluster(req.Path); err != nil {
		httpError(w, err, http.StatusBadRequest)
		return
	}
	s.handleListKubeconfigs(w, r)
}

func (s *Server) currentK8sClient() *k8s.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.k8sClient
}

func (s *Server) currentGuard() *security.Guard {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.guard
}

func (s *Server) switchCluster(rawPath string) error {
	path := expandKubeconfigPath(rawPath)
	if path == "" {
		return fmt.Errorf("path is required")
	}
	if !filepath.IsAbs(path) {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("kubeconfig path is not accessible: %w", err)
	}

	client, err := k8s.NewClient(path)
	if err != nil {
		return fmt.Errorf("connecting with kubeconfig %q: %w", path, err)
	}

	s.mu.Lock()
	s.k8sClient = client
	s.guard = security.NewGuard(client.Core, s.log)
	s.activeKubeconfigPath = path
	s.knownKubeconfigPaths[path] = struct{}{}
	if s.cfg.AIEngine != nil {
		s.cfg.AIEngine.SetK8sClient(client)
	}
	if s.cfg.Scheduler != nil {
		s.cfg.Scheduler.SetK8sClient(client)
	}
	s.persistKubeconfigStateLocked()
	s.mu.Unlock()

	s.log.Info("Switched active Kubernetes cluster", zap.String("kubeconfig", path))
	return nil
}

func (s *Server) saveKubeconfigBytes(name string, content []byte) (string, error) {
	if s.uploadDir == "" {
		return "", fmt.Errorf("kubeconfig upload directory is not configured")
	}
	baseName := filepath.Base(strings.TrimSpace(name))
	if baseName == "" || baseName == "." || baseName == string(filepath.Separator) {
		baseName = fmt.Sprintf("uploaded-%d.kubeconfig", time.Now().Unix())
	}
	if !strings.HasSuffix(baseName, ".yaml") && !strings.HasSuffix(baseName, ".yml") && !strings.HasSuffix(baseName, ".kubeconfig") && !strings.HasSuffix(baseName, ".conf") {
		baseName += ".kubeconfig"
	}

	dstPath := filepath.Join(s.uploadDir, baseName)
	if _, err := os.Stat(dstPath); err == nil {
		dstPath = filepath.Join(s.uploadDir, fmt.Sprintf("%d-%s", time.Now().Unix(), baseName))
	}

	if err := os.WriteFile(dstPath, content, 0o600); err != nil {
		return "", fmt.Errorf("saving kubeconfig file: %w", err)
	}
	return dstPath, nil
}

func (s *Server) executeSuggestedCommand(ctx context.Context, command string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("command is empty")
	}
	if strings.ContainsAny(command, "\n\r;&|><`") {
		return "", fmt.Errorf("command contains unsupported shell operators")
	}

	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("command is empty")
	}
	if parts[0] != "kubectl" {
		return "", fmt.Errorf("only kubectl commands are allowed")
	}

	s.mu.RLock()
	active := s.activeKubeconfigPath
	s.mu.RUnlock()

	if active != "" {
		hasKubeconfig := false
		for i := 1; i < len(parts); i++ {
			if parts[i] == "--kubeconfig" || strings.HasPrefix(parts[i], "--kubeconfig=") {
				hasKubeconfig = true
				break
			}
		}
		if !hasKubeconfig {
			parts = append([]string{"kubectl", "--kubeconfig", active}, parts[1:]...)
		}
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(out))
		if trimmed == "" {
			trimmed = err.Error()
		}
		return "", fmt.Errorf(trimmed)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		trimmed = "kubectl command executed"
	}
	return trimmed, nil
}

func (s *Server) persistKubeconfigStateLocked() {
	if s.stateFilePath == "" {
		return
	}
	paths := make([]string, 0, len(s.knownKubeconfigPaths))
	for p := range s.knownKubeconfigPaths {
		paths = append(paths, p)
	}
	st := &kubeconfigState{
		ActivePath: s.activeKubeconfigPath,
		Paths:      paths,
	}
	if err := saveKubeconfigState(s.stateFilePath, st); err != nil {
		s.log.Warn("Failed to persist kubeconfig state", zap.Error(err))
	}
}

// ─────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Encoding failure after headers sent — log but cannot change status code.
		_ = err
	}
}

func httpError(w http.ResponseWriter, err error, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// withCORS adds permissive CORS headers for the local dashboard.
// Tighten this for any network-accessible deployment.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
