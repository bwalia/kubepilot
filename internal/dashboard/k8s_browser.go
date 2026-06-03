package dashboard

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// This file implements the read-only Kubernetes Dashboard browser endpoints.
// Every handler follows the established pattern: resolve the active k8s client,
// read an optional ?namespace= filter, call the client, and write JSON.

func (s *Server) handleListNamespaces(w http.ResponseWriter, r *http.Request) {
	namespaces, err := s.currentK8sClient().ListNamespaces(r.Context())
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, namespaces)
}

func (s *Server) handleListStatefulSets(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	result, err := s.currentK8sClient().ListStatefulSets(r.Context(), ns)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleListDaemonSets(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	result, err := s.currentK8sClient().ListDaemonSets(r.Context(), ns)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleListK8sJobs(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	result, err := s.currentK8sClient().ListJobs(r.Context(), ns)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleListCronJobs(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	result, err := s.currentK8sClient().ListCronJobs(r.Context(), ns)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleListIngresses(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	result, err := s.currentK8sClient().ListIngresses(r.Context(), ns)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleListServices(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	result, err := s.currentK8sClient().ListServices(r.Context(), ns)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleListConfigMaps(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	result, err := s.currentK8sClient().ListConfigMaps(r.Context(), ns)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, result)
}

// handleListSecrets returns Secret metadata only — values never leave the cluster.
func (s *Server) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	result, err := s.currentK8sClient().ListSecrets(r.Context(), ns)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleListPVCs(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	result, err := s.currentK8sClient().ListPVCs(r.Context(), ns)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleListStorageClasses(w http.ResponseWriter, r *http.Request) {
	result, err := s.currentK8sClient().ListStorageClasses(r.Context())
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, result)
}

// handleGetPodLogs returns plain-text logs for a pod container. The optional
// ?container= selects a specific container; ?tail= limits the number of lines.
func (s *Server) handleGetPodLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	container := r.URL.Query().Get("container")

	tail := int64(200)
	if v := r.URL.Query().Get("tail"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil && parsed > 0 {
			tail = parsed
		}
	}

	logs, err := s.currentK8sClient().GetPodLogs(r.Context(), vars["namespace"], vars["pod"], container, tail)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(logs))
}

// handleGetResourceYAML returns sanitized YAML for any supported resource kind.
// Secret values and managedFields are stripped by the k8s layer.
func (s *Server) handleGetResourceYAML(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	yamlText, err := s.currentK8sClient().GetResourceYAML(r.Context(), vars["kind"], vars["namespace"], vars["name"])
	if err != nil {
		httpError(w, err, http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(yamlText))
}

// handleGetServerConfig reports server capability flags the dashboard UI uses to
// decide whether to enable mutation controls.
func (s *Server) handleGetServerConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"mutations_enabled": s.cfg.EnableActionMutationEndpoints,
	})
}
