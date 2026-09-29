// Command demo-api serves a fixture KubePilot API for Apple App Review.
//
// It speaks the same /healthz + /api/v1/* surface the iOS app expects, with
// Basic auth (DEMO_USER / DEMO_PASSWORD) and lightly dynamic timestamps so the
// UI feels alive without exposing a real cluster.
package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

func main() {
	addr := env("DEMO_API_ADDR", ":8383")
	user := env("DEMO_USER", "apple")
	pass := env("DEMO_PASSWORD", "review")

	s := &server{
		user:     user,
		password: pass,
		started:  time.Now().UTC(),
	}

	r := mux.NewRouter()
	r.HandleFunc("/healthz", s.handleHealthz).Methods(http.MethodGet)

	api := r.PathPrefix("/api/v1").Subrouter()
	api.Use(s.authMiddleware)
	api.HandleFunc("/version", s.handleVersion).Methods(http.MethodGet)
	api.HandleFunc("/config", s.handleConfig).Methods(http.MethodGet)
	api.HandleFunc("/clusters/kubeconfigs", s.handleKubeconfigs).Methods(http.MethodGet)
	api.HandleFunc("/clusters/switch-context", s.handleOK).Methods(http.MethodPost)
	api.HandleFunc("/clusters/pods", s.handlePods).Methods(http.MethodGet)
	api.HandleFunc("/clusters/crashing-pods", s.handleCrashingPods).Methods(http.MethodGet)
	api.HandleFunc("/clusters/deployments", s.handleDeployments).Methods(http.MethodGet)
	api.HandleFunc("/clusters/nodes", s.handleNodes).Methods(http.MethodGet)
	api.HandleFunc("/clusters/services", s.handleEmptyList).Methods(http.MethodGet)
	api.HandleFunc("/namespaces", s.handleNamespaces).Methods(http.MethodGet)
	api.HandleFunc("/events", s.handleEvents).Methods(http.MethodGet)
	api.HandleFunc("/troubleshooting/summary", s.handleTroubleshooting).Methods(http.MethodGet)
	api.HandleFunc("/clusters/pods/{ns}/{pod}/diagnostics", s.handleDiagnostics).Methods(http.MethodGet)
	api.HandleFunc("/clusters/pods/{ns}/{pod}/logs", s.handleLogs).Methods(http.MethodGet)
	api.HandleFunc("/resource/{kind}/{ns}/{name}/yaml", s.handleYAML).Methods(http.MethodGet)
	api.HandleFunc("/ai/health", s.handleAIHealth).Methods(http.MethodGet)
	api.HandleFunc("/ai/troubleshoot/{ns}/{pod}", s.handleTroubleshoot).Methods(http.MethodGet)
	api.HandleFunc("/ai/interpret", s.handleInterpret).Methods(http.MethodPost)
	api.HandleFunc("/ai/execute-action", s.handleExecuteAction).Methods(http.MethodPost)
	api.HandleFunc("/rca", s.handleRCAList).Methods(http.MethodGet)
	api.HandleFunc("/rca/{id}", s.handleRCAGet).Methods(http.MethodGet)
	api.HandleFunc("/anomalies", s.handleAnomalies).Methods(http.MethodGet)
	api.HandleFunc("/autopilot", s.handleAutopilot).Methods(http.MethodGet)
	api.HandleFunc("/autopilot/mode", s.handleOK).Methods(http.MethodPost)
	api.HandleFunc("/autopilot/pause", s.handleOK).Methods(http.MethodPost)
	api.HandleFunc("/autopilot/resume", s.handleOK).Methods(http.MethodPost)
	api.HandleFunc("/crcode/authorize", s.handleOK).Methods(http.MethodPost)

	log.Printf("kubepilot demo-api listening on %s (user=%s)", addr, user)
	log.Fatal(http.ListenAndServe(addr, withCORS(r)))
}

type server struct {
	user     string
	password string
	started  time.Time
	mu       sync.Mutex
	mode     string
}

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(u), []byte(s.user)) != 1 ||
			subtle.ConstantTimeCompare([]byte(p), []byte(s.password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="KubePilot"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func (s *server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *server) handleOK(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"status": "ok", "message": "demo stub accepted"})
}

func (s *server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"version": "1.0.0-demo",
		"commit":  "app-review",
		"built":   s.started.Format(time.RFC3339),
	})
}

func (s *server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"mutations_enabled": false})
}

func (s *server) handleKubeconfigs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"active_path":    "/demo/kubeconfig",
		"active_context": "demo-payments",
		"paths":          []string{"/demo/kubeconfig"},
		"contexts": []map[string]string{
			{"name": "demo-payments", "cluster": "demo", "user": "apple-review"},
		},
	})
}

func (s *server) tickRestarts() int {
	// Slow climb so screenshots/review feel dynamic without chaos.
	return 3 + int(time.Since(s.started).Minutes())%7
}

func (s *server) handlePods(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	pods := s.pods()
	if ns != "" {
		filtered := make([]map[string]any, 0)
		for _, p := range pods {
			if p["Namespace"] == ns {
				filtered = append(filtered, p)
			}
		}
		writeJSON(w, filtered)
		return
	}
	writeJSON(w, pods)
}

func (s *server) handleCrashingPods(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	out := make([]map[string]any, 0)
	for _, p := range s.pods() {
		if p["Reason"] == "CrashLoopBackOff" || p["Reason"] == "ImagePullBackOff" {
			if ns == "" || p["Namespace"] == ns {
				out = append(out, p)
			}
		}
	}
	writeJSON(w, out)
}

func (s *server) pods() []map[string]any {
	restarts := s.tickRestarts()
	return []map[string]any{
		{"Name": "checkout-api-7d9f8c", "Namespace": "payments", "Phase": "Running", "Reason": "CrashLoopBackOff", "NodeName": "demo-worker-1", "Restarts": restarts, "Ready": false, "Uptime": "12m"},
		{"Name": "checkout-api-9ab12e", "Namespace": "payments", "Phase": "Running", "Reason": "", "NodeName": "demo-worker-2", "Restarts": 0, "Ready": true, "Uptime": "4h"},
		{"Name": "redis-cart-0", "Namespace": "payments", "Phase": "Running", "Reason": "", "NodeName": "demo-worker-1", "Restarts": 0, "Ready": true, "Uptime": "2d"},
		{"Name": "coredns-5d4f", "Namespace": "kube-system", "Phase": "Running", "Reason": "", "NodeName": "demo-control-1", "Restarts": 0, "Ready": true, "Uptime": "14d"},
		{"Name": "metrics-server", "Namespace": "kube-system", "Phase": "Pending", "Reason": "ImagePullBackOff", "NodeName": "", "Restarts": 2, "Ready": false, "Uptime": "8m"},
	}
}

func (s *server) handleDeployments(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	deps := []map[string]any{
		{"Name": "checkout-api", "Namespace": "payments", "Replicas": 2, "ReadyReplicas": 1, "AvailableReplicas": 1, "Image": "ghcr.io/demo/checkout:1.4.2"},
		{"Name": "redis-cart", "Namespace": "payments", "Replicas": 1, "ReadyReplicas": 1, "AvailableReplicas": 1, "Image": "redis:7.2"},
	}
	if ns != "" {
		filtered := make([]map[string]any, 0)
		for _, d := range deps {
			if d["Namespace"] == ns {
				filtered = append(filtered, d)
			}
		}
		writeJSON(w, filtered)
		return
	}
	writeJSON(w, deps)
}

func (s *server) handleNodes(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, []map[string]any{
		{
			"Name": "demo-control-1", "Ready": true, "MemoryPressure": false, "DiskPressure": false, "PIDPressure": false,
			"CPUCapacity": "4", "MemoryCapacity": "8Gi", "KubeletVersion": "v1.29.4",
			"InternalIP": "LAN: 10.0.1.10", "IPs": []string{"10.0.1.10"}, "LANIPs": []string{"10.0.1.10"},
			"WANIPs": []string{}, "TunnelIPs": []string{}, "Roles": []string{"control-plane"}, "ControlPlane": true,
			"Labels": map[string]string{"node-role.kubernetes.io/control-plane": ""}, "Unschedulable": false,
		},
		{
			"Name": "demo-worker-1", "Ready": true, "MemoryPressure": false, "DiskPressure": false, "PIDPressure": false,
			"CPUCapacity": "8", "MemoryCapacity": "16Gi", "KubeletVersion": "v1.29.4",
			"InternalIP": "LAN: 10.0.1.21", "IPs": []string{"10.0.1.21"}, "LANIPs": []string{"10.0.1.21"},
			"WANIPs": []string{}, "TunnelIPs": []string{}, "Roles": []string{"worker"}, "ControlPlane": false,
			"Labels": map[string]string{"kubepilot.io/lan-ip": "10.0.1.21"}, "Unschedulable": false,
		},
		{
			"Name": "demo-worker-2", "Ready": true, "MemoryPressure": false, "DiskPressure": false, "PIDPressure": false,
			"CPUCapacity": "8", "MemoryCapacity": "16Gi", "KubeletVersion": "v1.29.4",
			"InternalIP": "LAN: 10.0.1.22", "IPs": []string{"10.0.1.22"}, "LANIPs": []string{"10.0.1.22"},
			"WANIPs": []string{}, "TunnelIPs": []string{}, "Roles": []string{"worker"}, "ControlPlane": false,
			"Labels": map[string]string{"kubepilot.io/lan-ip": "10.0.1.22"}, "Unschedulable": false,
		},
	})
}

func (s *server) handleNamespaces(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, []map[string]any{
		{"Name": "payments", "Status": "Active", "Labels": map[string]string{"env": "demo"}},
		{"Name": "kube-system", "Status": "Active", "Labels": map[string]string{}},
		{"Name": "default", "Status": "Active", "Labels": map[string]string{}},
	})
}

func (s *server) handleEmptyList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, []any{})
}

func (s *server) handleEvents(w http.ResponseWriter, _ *http.Request) {
	now := time.Now().UTC()
	items := []map[string]any{
		{
			"reason": "BackOff", "message": "Back-off restarting failed container checkout", "type": "Warning",
			"count": 12, "first_seen": now.Add(-40 * time.Minute).Format(time.RFC3339),
			"last_seen": now.Add(-90 * time.Second).Format(time.RFC3339),
			"involved_object": map[string]string{"kind": "Pod", "name": "checkout-api-7d9f8c", "namespace": "payments"},
			"source": "kubelet",
		},
		{
			"reason": "Unhealthy", "message": "Readiness probe failed: connection refused", "type": "Warning",
			"count": 4, "first_seen": now.Add(-15 * time.Minute).Format(time.RFC3339),
			"last_seen": now.Add(-2 * time.Minute).Format(time.RFC3339),
			"involved_object": map[string]string{"kind": "Pod", "name": "checkout-api-7d9f8c", "namespace": "payments"},
			"source": "kubelet",
		},
		{
			"reason": "Scheduled", "message": "Successfully assigned payments/checkout-api-9ab12e to demo-worker-2", "type": "Normal",
			"count": 1, "first_seen": now.Add(-4 * time.Hour).Format(time.RFC3339),
			"last_seen": now.Add(-4 * time.Hour).Format(time.RFC3339),
			"involved_object": map[string]string{"kind": "Pod", "name": "checkout-api-9ab12e", "namespace": "payments"},
			"source": "default-scheduler",
		},
	}
	writeJSON(w, map[string]any{"items": items, "total": len(items)})
}

func (s *server) handleTroubleshooting(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	if ns == "" {
		ns = "payments"
	}
	writeJSON(w, map[string]any{
		"namespace":    ns,
		"generated_at": time.Now().UTC().Format(time.RFC3339Nano),
		"health_summary": map[string]any{
			"not_ready_nodes":      0,
			"crashloop_pods":       1,
			"failed_mount_events":  0,
			"pending_pods":         1,
			"warning_events":       8,
			"recommended_actions":  []string{"Inspect checkout-api CrashLoopBackOff", "Fix metrics-server image pull"},
		},
		"insights": []map[string]any{
			{
				"id": "insight-crashloop", "category": "workload", "severity": "critical",
				"title": "checkout-api is crash looping",
				"summary": "Container exits with code 1 after failing to reach redis-cart on the readiness probe window.",
				"suggestions": []string{"Check REDIS_URL", "Review recent config change", "kubectl logs -p checkout-api-7d9f8c"},
				"affected_resources": []string{"payments/checkout-api-7d9f8c"},
			},
		},
		"nodes": []map[string]any{
			{
				"name": "demo-worker-1", "ready": true, "cpu_capacity": "8", "memory_capacity": "16Gi",
				"cpu_usage": "1.2", "memory_usage": "6Gi", "cpu_usage_percent": 15, "memory_usage_percent": 38,
				"disk_pressure": false, "memory_pressure": false, "pid_pressure": false, "unschedulable": false,
				"kubelet_version": "v1.29.4", "roles": []string{"worker"}, "control_plane": false,
			},
		},
		"resource_pressure": map[string]any{
			"metrics_available": true, "cpu_usage_percent": 22, "memory_usage_percent": 41,
			"memory_pressure_nodes": 0, "disk_pressure_nodes": 0, "pid_pressure_nodes": 0,
		},
		"problem_pods": []map[string]any{
			{
				"name": "checkout-api-7d9f8c", "namespace": "payments", "status": "CrashLoopBackOff",
				"restarts": s.tickRestarts(), "node": "demo-worker-1", "reason": "CrashLoopBackOff",
				"age_minutes": 12, "message": "container exit code 1",
			},
			{
				"name": "metrics-server", "namespace": "kube-system", "status": "Pending",
				"restarts": 2, "node": "", "reason": "ImagePullBackOff",
				"age_minutes": 8, "message": "Failed to pull image",
			},
		},
	})
}

func (s *server) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ns, pod := vars["ns"], vars["pod"]
	writeJSON(w, map[string]any{
		"diagnostics": map[string]any{
			"name": pod, "namespace": ns, "phase": "Running", "node_name": "demo-worker-1",
			"created_at": time.Now().UTC().Add(-12 * time.Minute).Format(time.RFC3339),
			"conditions": []map[string]any{
				{"type": "Ready", "status": "False", "reason": "ContainersNotReady", "message": "containers with unready status: [checkout]"},
			},
			"container_statuses": []map[string]any{
				{
					"name": "checkout", "image": "ghcr.io/demo/checkout:1.4.2", "ready": false,
					"restart_count": s.tickRestarts(), "state": "waiting", "state_reason": "CrashLoopBackOff",
					"state_message": "back-off 5m0s restarting failed container", "exit_code": 1,
				},
			},
			"events": []any{}, "labels": map[string]string{"app": "checkout-api"},
			"owner_chain": []map[string]string{{"kind": "ReplicaSet", "name": "checkout-api-7d9f8c", "uid": "demo-rs"}},
		},
		"logs": s.logText(pod),
	})
}

func (s *server) handleLogs(w http.ResponseWriter, r *http.Request) {
	pod := mux.Vars(r)["pod"]
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(s.logText(pod)))
}

func (s *server) logText(pod string) string {
	ts := time.Now().UTC().Format(time.RFC3339)
	return fmt.Sprintf("%s ERROR redis: connection refused (127.0.0.1:6379)\n%s INFO retrying in 2s…\n%s FATAL unable to start checkout listener\n# demo fixture logs for %s\n", ts, ts, ts, pod)
}

func (s *server) handleYAML(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintf(w, "apiVersion: v1\nkind: %s\nmetadata:\n  name: %s\n  namespace: %s\n# demo fixture\n", vars["kind"], vars["name"], vars["ns"])
}

func (s *server) handleAIHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"healthy": true, "model": "demo-llama3.2:3b", "base_url": "http://demo-llm.local/v1",
		"latency_ms": 42, "error": nil,
	})
}

func (s *server) handleTroubleshoot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	writeJSON(w, map[string]any{
		"pod_name":   vars["pod"],
		"namespace":  vars["ns"],
		"root_cause": "Checkout container cannot reach redis-cart; readiness probes fail and kubelet restarts the pod.",
		"analysis":   "Demo RCA: REDIS_HOST still points at localhost inside the pod network namespace. Other replica on worker-2 is healthy because it uses the Service DNS name.",
		"actions": []map[string]any{
			{
				"type": "edit_env", "namespace": vars["ns"], "resource": "checkout-api",
				"replicas": nil, "command": "kubectl -n payments set env deploy/checkout-api REDIS_HOST=redis-cart",
				"explanation": "Point checkout at the redis-cart Service", "requires_cr_code": false,
			},
		},
	})
}

func (s *server) handleInterpret(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"actions": []map[string]any{
			{
				"type": "kubectl", "namespace": "payments", "resource": "checkout-api",
				"command": "kubectl -n payments get pods -l app=checkout-api",
				"explanation": "List checkout pods", "requires_cr_code": false,
			},
		},
	})
}

func (s *server) handleExecuteAction(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"status": "skipped", "message": "Demo mode — mutations are disabled"})
}

func (s *server) handleRCAList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, []map[string]any{s.rcaReport()})
}

func (s *server) handleRCAGet(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	rep := s.rcaReport()
	if id != "" && id != rep["id"] {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, rep)
}

func (s *server) rcaReport() map[string]any {
	return map[string]any{
		"id": "rca-demo-checkout", "timestamp": time.Now().UTC().Add(-20 * time.Minute).Format(time.RFC3339Nano),
		"target_resource": "payments/checkout-api", "severity": "critical",
		"root_cause": "Misconfigured REDIS_HOST causes CrashLoopBackOff on one checkout replica",
		"evidence_chain": []string{
			"Pod logs: connection refused on 127.0.0.1:6379",
			"Events: BackOff restarting failed container",
			"Sibling replica healthy with Service DNS",
		},
		"remediation": []string{
			"Update REDIS_HOST to redis-cart.payments.svc",
			"Rollout restart deploy/checkout-api",
		},
		"confidence": 0.91, "status": "open",
	}
}

func (s *server) handleAnomalies(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, []map[string]any{
		{
			"id": "anom-demo-1", "detected_at": time.Now().UTC().Add(-18 * time.Minute).Format(time.RFC3339Nano),
			"rule": "crashloop_threshold", "severity": "critical",
			"description": "Pod restart rate exceeded threshold",
			"rca_report_id": "rca-demo-checkout",
			"resource": map[string]any{"kind": "Pod", "name": "checkout-api-7d9f8c", "namespace": "payments"},
		},
	})
}

func (s *server) handleAutopilot(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	_ = limit
	s.mu.Lock()
	mode := s.mode
	s.mu.Unlock()
	if mode == "" {
		mode = "dry-run"
	}
	writeJSON(w, map[string]any{
		"enabled": true,
		"policy": map[string]any{
			"mode": mode, "min_confidence": 0.8,
			"allowed_actions": []string{"restart_pod", "scale"},
			"max_risk": "low", "allowed_namespaces": []string{"payments"},
			"blocked_namespaces": []string{"kube-system"},
			"cooldown": int64(60_000_000_000), "max_actions_per_hour": 10,
		},
		"decisions": []map[string]any{
			{
				"time": time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339Nano),
				"report_id": "rca-demo-checkout",
				"resource": map[string]any{"kind": "Pod", "name": "checkout-api-7d9f8c", "namespace": "payments"},
				"severity": "critical", "confidence": 0.88,
				"root_cause": "CrashLoopBackOff from bad REDIS_HOST",
				"action": "restart_pod", "verdict": "dry-run",
				"reason": "Demo mode — would restart after env fix", "output": nil,
			},
		},
		"stats": map[string]int{"executed": 0, "dry-run": 3, "skipped": 1},
	})
}
