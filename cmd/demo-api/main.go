// Command demo-api serves a mock KubePilot API backed by a simulated cluster.
//
// It speaks the same /healthz + /api/v1/* surface the iOS app expects, with
// Basic auth (DEMO_USER / DEMO_PASSWORD). The data comes from cluster.go: a
// deterministic, clock-driven simulation of a small production cluster with
// live metrics and rotating incidents. Nothing here touches a real cluster.
package main

import (
	"crypto/subtle"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

func main() {
	addr := env("DEMO_API_ADDR", ":8383")
	user := env("DEMO_USER", "apple")
	pass := env("DEMO_PASSWORD", "review")

	// DEMO_CLOCK_OFFSET (e.g. "45m") shifts the simulation clock, so every
	// rotating incident can be checked without waiting for it.
	offset, _ := time.ParseDuration(env("DEMO_CLOCK_OFFSET", "0s"))
	clock := func() time.Time { return time.Now().Add(offset) }
	s := &server{user: user, password: pass, started: time.Now().UTC(), clock: clock}

	log.Printf("kubepilot demo-api listening on %s (user=%s)", addr, user)
	log.Fatal(http.ListenAndServe(addr, withCORS(s.routes())))
}

type server struct {
	user     string
	password string
	started  time.Time
	clock    func() time.Time
}

func (s *server) routes() http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/", s.handleRoot).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc("/healthz", s.handleHealthz).Methods(http.MethodGet)
	r.HandleFunc("/demo/status.json", s.handleStatus).Methods(http.MethodGet)

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
	api.HandleFunc("/clusters/services", s.handleServices).Methods(http.MethodGet)
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
	return r
}

func (s *server) snap() snapshot { return simulate(s.clock()) }

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
		w.Header().Set("X-KubePilot-Demo", "mock-data")
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

func nsMatch(filter, ns string) bool {
	return filter == "" || filter == "all" || filter == ns
}

//go:embed page.html
var rootPage []byte

// handleRoot serves the branded landing page, so anyone opening the demo URL
// in a browser learns it is a mock API and how to connect a real cluster.
func (s *server) handleRoot(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(rootPage)
}

func (s *server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *server) handleOK(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"status": "ok", "message": "demo stub accepted — the mock cluster is read-only"})
}

func (s *server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"version": "1.0.0-demo",
		"commit":  "mock-cluster",
		"built":   s.started.Format(time.RFC3339),
	})
}

func (s *server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"mutations_enabled": false})
}

func (s *server) handleKubeconfigs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"active_path":    "/demo/kubeconfig",
		"active_context": clusterName,
		"paths":          []string{"/demo/kubeconfig"},
		"contexts": []map[string]string{
			{"name": clusterName, "cluster": clusterName, "user": "demo-viewer"},
			{"name": "kp-demo-staging-eu-west-2", "cluster": "kp-demo-staging-eu-west-2", "user": "demo-viewer"},
		},
	})
}

// ---------------------------------------------------------------------------
// Workloads

func podJSON(p pod, now time.Time) map[string]any {
	return map[string]any{
		"Name": p.Name, "Namespace": p.Namespace, "Phase": p.Phase, "Reason": p.Reason, "NodeName": p.Node,
		"Restarts": p.Restarts, "Ready": p.Ready, "Uptime": humanAge(p.Age(now)),
	}
}

func (s *server) handlePods(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	snap := s.snap()
	out := make([]map[string]any, 0, len(snap.Pods))
	for _, p := range snap.Pods {
		if nsMatch(ns, p.Namespace) {
			out = append(out, podJSON(p, snap.Now))
		}
	}
	writeJSON(w, out)
}

func (s *server) handleCrashingPods(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	snap := s.snap()
	out := make([]map[string]any, 0)
	for _, p := range snap.Pods {
		if !p.Ready && p.Reason != "" && nsMatch(ns, p.Namespace) {
			out = append(out, podJSON(p, snap.Now))
		}
	}
	writeJSON(w, out)
}

func (s *server) handleDeployments(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	snap := s.snap()
	out := make([]map[string]any, 0)
	for _, wl := range workloads {
		if wl.Kind != "Deployment" || !nsMatch(ns, wl.Namespace) {
			continue
		}
		_, ready := snap.workloadReady(wl)
		image := wl.Image
		for _, in := range snap.Active {
			if in.Kind == "imagepull" && in.Workload == wl.Name {
				image = strings.Replace(image, "2.8.3", "2.9.0", 1)
			}
		}
		out = append(out, map[string]any{
			"Name": wl.Name, "Namespace": wl.Namespace, "Replicas": wl.Replicas,
			"ReadyReplicas": ready, "AvailableReplicas": ready, "Image": image,
		})
	}
	writeJSON(w, out)
}

func nodeLabels(n node) map[string]string {
	labels := map[string]string{
		"kubernetes.io/hostname":           n.Name,
		"kubernetes.io/os":                 "linux",
		"kubernetes.io/arch":               "amd64",
		"topology.kubernetes.io/region":    "eu-west-2",
		"topology.kubernetes.io/zone":      n.Zone,
		"node.kubernetes.io/instance-type": n.Instance,
		"kubepilot.io/lan-ip":              n.LAN,
	}
	if n.ControlPlane {
		labels["node-role.kubernetes.io/control-plane"] = "true"
		labels["node-role.kubernetes.io/etcd"] = "true"
	} else {
		labels["node-role.kubernetes.io/worker"] = "true"
	}
	return labels
}

func nodeIPs(n node) (all, lan, wan, tun []string) {
	lan = []string{n.LAN}
	wan = []string{}
	if n.WAN != "" {
		wan = []string{n.WAN}
	}
	tun = []string{n.Tunnel}
	all = append(append(append([]string{}, lan...), wan...), tun...)
	return
}

func roles(n node) []string {
	if n.ControlPlane {
		return []string{"control-plane", "etcd"}
	}
	return []string{"worker"}
}

func (s *server) handleNodes(w http.ResponseWriter, _ *http.Request) {
	snap := s.snap()
	out := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		st := snap.NodeStats[n.Name]
		all, lan, wan, tun := nodeIPs(n)
		out = append(out, map[string]any{
			"Name": n.Name, "Ready": st.Ready, "MemoryPressure": st.MemoryPressure, "DiskPressure": st.DiskPressure, "PIDPressure": false,
			"CPUCapacity": strconv.Itoa(n.CPU), "MemoryCapacity": fmt.Sprintf("%dGi", n.MemGi), "KubeletVersion": kubeVersion,
			"InternalIP": "LAN: " + n.LAN, "IPs": all, "LANIPs": lan, "WANIPs": wan, "TunnelIPs": tun,
			"Roles": roles(n), "ControlPlane": n.ControlPlane, "Labels": nodeLabels(n), "Unschedulable": !st.Ready,
		})
	}
	writeJSON(w, out)
}

func (s snapshot) services() []map[string]any {
	var out []map[string]any
	for _, wl := range workloads {
		if wl.Port == 0 || wl.Kind == "DaemonSet" {
			continue
		}
		svcType, ip := "ClusterIP", fmt.Sprintf("10.43.%d.%d", hash64(wl.Namespace)%200+1, hash64(wl.Name)%250+2)
		if wl.Name == "ingress-nginx-controller" {
			svcType = "LoadBalancer"
		}
		portName := "http"
		switch wl.Port {
		case 443, 8443:
			portName = "https"
		case 5432:
			portName = "postgresql"
		case 6379:
			portName = "redis"
		case 9092:
			portName = "kafka"
		case 53:
			portName = "dns"
		}
		var endpoints []string
		for _, p := range s.Pods {
			if p.Namespace == wl.Namespace && p.Workload.Name == wl.Name && p.Ready && p.IP != "" {
				endpoints = append(endpoints, p.IP)
			}
		}
		out = append(out, map[string]any{
			"name": wl.Name, "namespace": wl.Namespace, "type": svcType, "cluster_ip": ip,
			"ports":        []map[string]any{{"name": portName, "port": wl.Port, "target_port": strconv.Itoa(wl.Port), "protocol": "TCP"}},
			"selector":     map[string]string{"app.kubernetes.io/name": wl.Name},
			"endpoint_ips": endpoints,
		})
	}
	return out
}

func (s *server) handleServices(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	out := make([]map[string]any, 0)
	for _, svc := range s.snap().services() {
		if nsMatch(ns, svc["namespace"].(string)) {
			out = append(out, svc)
		}
	}
	writeJSON(w, out)
}

func (s *server) handleNamespaces(w http.ResponseWriter, _ *http.Request) {
	out := make([]map[string]any, 0, len(namespaceList))
	for _, ns := range namespaceList {
		out = append(out, map[string]any{"Name": ns.Name, "Status": "Active", "Labels": ns.Labels})
	}
	writeJSON(w, out)
}

// ---------------------------------------------------------------------------
// Events & health

func (s *server) handleEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ns, typ, search := q.Get("namespace"), q.Get("type"), strings.ToLower(q.Get("search"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	all := s.snap().events()
	items := make([]map[string]any, 0, len(all))
	total := 0
	for _, e := range all {
		if (ns != "" && ns != "all" && e.Namespace != ns) || (typ != "" && !strings.EqualFold(typ, e.Type)) {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(e.Reason+" "+e.Message+" "+e.Name), search) {
			continue
		}
		total++
		if limit <= 0 || len(items) < limit {
			items = append(items, e.JSON())
		}
	}
	writeJSON(w, map[string]any{"items": items, "total": total})
}

func (s *server) handleTroubleshooting(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	snap := s.snap()
	summary := map[string]any{}

	notReady, memP, diskP := 0, 0, 0
	cpuSum, memSum, live := 0, 0, 0
	nodeRows := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		st := snap.NodeStats[n.Name]
		if !st.Ready {
			notReady++
		} else {
			cpuSum += st.CPUPercent
			memSum += st.MemPercent
			live++
		}
		if st.DiskPressure {
			diskP++
		}
		if st.MemoryPressure {
			memP++
		}
		all, lan, wan, tun := nodeIPs(n)
		nodeRows = append(nodeRows, map[string]any{
			"name": n.Name, "ready": st.Ready, "cpu_capacity": strconv.Itoa(n.CPU), "memory_capacity": fmt.Sprintf("%dGi", n.MemGi),
			"cpu_usage":         fmt.Sprintf("%.1f", float64(n.CPU*st.CPUPercent)/100),
			"memory_usage":      fmt.Sprintf("%.1fGi", float64(n.MemGi*st.MemPercent)/100),
			"cpu_usage_percent": st.CPUPercent, "memory_usage_percent": st.MemPercent,
			"disk_pressure": st.DiskPressure, "memory_pressure": st.MemoryPressure, "pid_pressure": false, "unschedulable": !st.Ready,
			"kubelet_version": kubeVersion, "roles": roles(n), "control_plane": n.ControlPlane,
			"ips": all, "lan_ips": lan, "wan_ips": wan, "tunnel_ips": tun, "labels": nodeLabels(n),
		})
	}

	crash, pending := 0, 0
	problems := make([]map[string]any, 0)
	for _, p := range snap.Pods {
		if p.Ready || !nsMatch(ns, p.Namespace) {
			continue
		}
		switch p.Reason {
		case "CrashLoopBackOff", "OOMKilled":
			crash++
		}
		if p.Phase == "Pending" {
			pending++
		}
		problems = append(problems, map[string]any{
			"name": p.Name, "namespace": p.Namespace, "status": p.Reason, "restarts": p.Restarts, "node": p.Node,
			"reason": p.Reason, "age_minutes": int(p.Age(snap.Now).Minutes()), "message": p.StateMessage,
		})
	}
	warnings := 0
	for _, e := range snap.events() {
		if e.Type == "Warning" && (e.Namespace == "" || nsMatch(ns, e.Namespace)) {
			warnings++
		}
	}

	insights := make([]map[string]any, 0)
	var actions []string
	for _, in := range snap.Active {
		if in.Namespace != "" && !nsMatch(ns, in.Namespace) {
			continue
		}
		affected := []string{}
		for _, p := range snap.Pods {
			if p.Incident != nil && p.Incident.ID == in.ID && len(affected) < 6 {
				affected = append(affected, p.Namespace+"/"+p.Name)
			}
		}
		if in.Node != "" {
			affected = append([]string{"node/" + in.Node}, affected...)
		}
		category := "workload"
		if in.Kind == "diskpressure" || in.Kind == "nodenotready" {
			category = "node"
		}
		insights = append(insights, map[string]any{
			"id": "insight-" + in.ID, "category": category, "severity": in.Severity,
			"title": in.Title, "summary": in.Summary, "suggestions": in.Suggestions, "affected_resources": affected,
		})
		actions = append(actions, in.Remediation[0])
	}
	if actions == nil {
		actions = []string{}
	}

	summary["not_ready_nodes"] = notReady
	summary["crashloop_pods"] = crash
	summary["failed_mount_events"] = 0
	summary["pending_pods"] = pending
	summary["warning_events"] = warnings
	summary["recommended_actions"] = actions
	if live == 0 {
		live = 1
	}
	if ns == "" {
		ns = "all"
	}
	writeJSON(w, map[string]any{
		"namespace": ns, "generated_at": snap.Now.Format(time.RFC3339Nano),
		"health_summary": summary, "insights": insights, "nodes": nodeRows,
		"resource_pressure": map[string]any{
			"metrics_available": true, "cpu_usage_percent": cpuSum / live, "memory_usage_percent": memSum / live,
			"memory_pressure_nodes": memP, "disk_pressure_nodes": diskP, "pid_pressure_nodes": 0,
		},
		"problem_pods": problems,
	})
}

// ---------------------------------------------------------------------------
// Pod detail

func (s *server) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	snap := s.snap()
	p, ok := snap.findPod(vars["ns"], vars["pod"])
	if !ok {
		http.Error(w, `{"error":"pod not found in the demo cluster"}`, http.StatusNotFound)
		return
	}
	tail, _ := strconv.Atoi(r.URL.Query().Get("tail_lines"))
	conditions := []map[string]any{
		{"type": "PodScheduled", "status": boolStatus(p.Node != ""), "reason": "", "message": ""},
		{"type": "Initialized", "status": "True", "reason": "", "message": ""},
		{"type": "ContainersReady", "status": boolStatus(p.Ready), "reason": "", "message": ""},
		{"type": "Ready", "status": boolStatus(p.Ready), "reason": "", "message": ""},
	}
	state, stateReason := "running", ""
	if !p.Ready {
		conditions[3]["reason"] = "ContainersNotReady"
		conditions[3]["message"] = "containers with unready status: [" + p.Workload.Name + "]"
		state, stateReason = "waiting", p.Reason
	}
	if p.Node == "" {
		conditions[0]["reason"] = "Unschedulable"
		conditions[0]["message"] = p.StateMessage
	}
	podEvents := make([]map[string]any, 0)
	for _, e := range snap.events() {
		if e.Kind == "Pod" && e.Name == p.Name && e.Namespace == p.Namespace {
			podEvents = append(podEvents, e.JSON())
		}
	}
	owner := []map[string]string{}
	if p.ReplicaSet != "" {
		owner = append(owner,
			map[string]string{"kind": "ReplicaSet", "name": p.ReplicaSet, "uid": fmt.Sprintf("%x", hash64(p.ReplicaSet))},
			map[string]string{"kind": "Deployment", "name": p.Workload.Name, "uid": fmt.Sprintf("%x", hash64(p.Namespace, p.Workload.Name))})
	} else {
		owner = append(owner, map[string]string{"kind": p.Workload.Kind, "name": p.Workload.Name, "uid": fmt.Sprintf("%x", hash64(p.Namespace, p.Workload.Name))})
	}
	writeJSON(w, map[string]any{
		"diagnostics": map[string]any{
			"name": p.Name, "namespace": p.Namespace, "phase": p.Phase, "node_name": p.Node,
			"created_at": p.Started.Format(time.RFC3339),
			"conditions": conditions,
			"container_statuses": []map[string]any{{
				"name": p.Workload.Name, "image": p.Image, "ready": p.Ready, "restart_count": p.Restarts,
				"state": state, "state_reason": stateReason, "state_message": p.StateMessage, "exit_code": p.ExitCode,
			}},
			"events": podEvents,
			"labels": map[string]string{
				"app": p.Workload.Name, "app.kubernetes.io/name": p.Workload.Name, "app.kubernetes.io/part-of": "acme-shop",
				"pod-template-hash": strings.TrimPrefix(p.ReplicaSet, p.Workload.Name+"-"),
			},
			"owner_chain": owner,
		},
		"logs": strings.Join(snap.logLines(p, tail), "\n") + "\n",
	})
}

func boolStatus(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

func (s *server) handleLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	snap := s.snap()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	p, ok := snap.findPod(vars["ns"], vars["pod"])
	if !ok {
		http.Error(w, "pod not found in the demo cluster", http.StatusNotFound)
		return
	}
	tail, _ := strconv.Atoi(r.URL.Query().Get("tail"))
	_, _ = io.WriteString(w, strings.Join(snap.logLines(p, tail), "\n")+"\n")
}

func (s *server) handleYAML(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, s.snap().yaml(vars["kind"], vars["ns"], vars["name"]))
}

// ---------------------------------------------------------------------------
// AI

func (s *server) handleAIHealth(w http.ResponseWriter, _ *http.Request) {
	now := s.clock()
	writeJSON(w, map[string]any{
		"healthy": true, "model": "llama3.1:8b (simulated)", "base_url": "http://ollama.ai.svc:11434/v1",
		"latency_ms": 180 + int(hash64(now.Truncate(time.Minute).String())%140), "error": nil,
	})
}

func (s *server) handleTroubleshoot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	snap := s.snap()
	p, ok := snap.findPod(vars["ns"], vars["pod"])
	if !ok {
		http.Error(w, `{"error":"pod not found in the demo cluster"}`, http.StatusNotFound)
		return
	}
	if p.Incident == nil {
		st := snap.NodeStats[p.Node]
		writeJSON(w, map[string]any{
			"pod_name": p.Name, "namespace": p.Namespace,
			"root_cause": "No fault found — " + p.Name + " is healthy.",
			"analysis": fmt.Sprintf("All containers are ready, with %d restarts in %s. Recent logs show normal traffic and no errors above baseline. Node %s is at %d%% CPU and %d%% memory, within normal range. No action needed.",
				p.Restarts, humanAge(p.Age(snap.Now)), p.Node, st.CPUPercent, st.MemPercent),
			"actions": []map[string]any{{
				"type": "kubectl", "namespace": p.Namespace, "resource": p.Name, "replicas": nil,
				"command": "kubectl -n " + p.Namespace + " top pod " + p.Name, "explanation": "Check live resource usage", "requires_cr_code": false,
			}},
		})
		return
	}
	in := p.Incident
	writeJSON(w, map[string]any{
		"pod_name": p.Name, "namespace": p.Namespace, "root_cause": in.RootCause, "analysis": in.Analysis,
		"actions": []map[string]any{in.Action, {
			"type": "kubectl", "namespace": p.Namespace, "resource": p.Name, "replicas": nil,
			"command": "kubectl -n " + p.Namespace + " describe pod " + p.Name, "explanation": "Inspect conditions and recent events", "requires_cr_code": false,
		}},
	})
}

func (s *server) handleInterpret(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Command string `json:"command"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&body)
	q := strings.ToLower(body.Command)
	snap := s.snap()
	act := func(typ, ns, res, cmd, why string) map[string]any {
		return map[string]any{"type": typ, "namespace": ns, "resource": res, "command": cmd, "explanation": why, "requires_cr_code": false}
	}
	var actions []map[string]any
	for _, in := range snap.Active {
		if (in.Workload != "" && strings.Contains(q, strings.Split(in.Workload, "-")[0])) || (in.Node != "" && strings.Contains(q, in.Node)) {
			actions = append(actions, in.Action)
		}
	}
	switch {
	case strings.Contains(q, "crash") || strings.Contains(q, "restart") || strings.Contains(q, "fail"):
		actions = append(actions, act("kubectl", "", "pods", "kubectl get pods -A --field-selector=status.phase!=Succeeded | grep -vE 'Running.*1/1|Completed'", "List pods that are not healthy"))
	case strings.Contains(q, "node"):
		actions = append(actions, act("kubectl", "", "nodes", "kubectl get nodes -o wide", "Show node status, roles and IPs"), act("kubectl", "", "nodes", "kubectl top nodes", "Show live node CPU and memory"))
	case strings.Contains(q, "memory") || strings.Contains(q, "oom") || strings.Contains(q, "cpu"):
		actions = append(actions, act("kubectl", "", "pods", "kubectl top pods -A --sort-by=memory | head -15", "Find the heaviest pods"))
	case strings.Contains(q, "event") || strings.Contains(q, "warning"):
		actions = append(actions, act("kubectl", "", "events", "kubectl get events -A --field-selector type=Warning --sort-by=.lastTimestamp", "Recent warning events"))
	case strings.Contains(q, "log"):
		actions = append(actions, act("kubectl", "payments", "checkout-api", "kubectl -n payments logs deploy/checkout-api --tail=100", "Tail checkout-api logs"))
	}
	if len(actions) == 0 {
		actions = append(actions, act("kubectl", "", "cluster", "kubectl get pods -A -o wide", "Overview of every pod in the cluster"))
	}
	writeJSON(w, map[string]any{"actions": actions})
}

func (s *server) handleExecuteAction(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"status": "skipped", "message": "Demo mode — this is a simulated cluster, so actions are previewed but never run. Connect your own cluster to apply fixes."})
}

// ---------------------------------------------------------------------------
// RCA, anomalies, autopilot

func rcaJSON(in incident) map[string]any {
	target := in.Namespace + "/" + in.Workload
	if in.Workload == "" {
		target = "node/" + in.Node
	}
	status := "open"
	if in.Resolved {
		status = "resolved"
	}
	return map[string]any{
		"id": in.ID, "timestamp": in.Started.Add(3 * time.Minute).Format(time.RFC3339Nano),
		"target_resource": target, "severity": in.Severity, "root_cause": in.RootCause,
		"evidence_chain": in.Evidence, "remediation": in.Remediation, "confidence": in.Confidence, "status": status,
	}
}

func (snap snapshot) reports(now time.Time) []incident {
	var out []incident
	for _, in := range append(append([]incident{}, snap.Active...), snap.History...) {
		if !in.Started.Add(3 * time.Minute).After(now) {
			out = append(out, in)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Started.After(out[j].Started) })
	return out
}

func (s *server) handleRCAList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	snap := s.snap()
	out := make([]map[string]any, 0)
	for _, in := range snap.reports(snap.Now) {
		if (q.Get("severity") != "" && !strings.EqualFold(q.Get("severity"), in.Severity)) ||
			(q.Get("namespace") != "" && q.Get("namespace") != in.Namespace) {
			continue
		}
		out = append(out, rcaJSON(in))
	}
	writeJSON(w, out)
}

func (s *server) handleRCAGet(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	snap := s.snap()
	for _, in := range snap.reports(snap.Now) {
		if in.ID == id {
			writeJSON(w, rcaJSON(in))
			return
		}
	}
	// Old fixture id, still cached by app builds that predate the mock cluster.
	if id == "rca-demo-checkout" {
		writeJSON(w, rcaJSON(snap.Active[0]))
		return
	}
	http.NotFound(w, r)
}

func incidentResource(in incident, snap snapshot) map[string]any {
	if in.Workload == "" {
		return map[string]any{"kind": "Node", "name": in.Node, "namespace": ""}
	}
	name := in.Workload
	for _, p := range snap.Pods {
		if p.Incident != nil && p.Incident.ID == in.ID {
			name = p.Name
		}
	}
	return map[string]any{"kind": "Pod", "name": name, "namespace": in.Namespace}
}

func (s *server) handleAnomalies(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	snap := s.snap()
	out := make([]map[string]any, 0)
	for _, in := range snap.reports(snap.Now) {
		if ns != "" && ns != in.Namespace {
			continue
		}
		out = append(out, map[string]any{
			"id": "anom-" + strings.TrimPrefix(in.ID, "rca-"), "detected_at": in.Started.Add(time.Minute).Format(time.RFC3339Nano),
			"rule": in.Rule, "severity": in.Severity, "description": in.Title, "rca_report_id": in.ID,
			"resource": incidentResource(in, snap),
		})
	}
	writeJSON(w, out)
}

const demoMode = "dry-run"

func (s *server) handleAutopilot(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	snap := s.snap()
	stats := map[string]int{"executed": 0, "dry-run": 0, "skipped": 0, "escalated": 0, "failed": 0}
	decisions := make([]map[string]any, 0)
	for _, in := range snap.reports(snap.Now) {
		verdict := in.Autopilot.Verdict
		reason := in.Autopilot.Reason
		if verdict == "act" {
			verdict = demoMode
			reason = "Would " + in.Autopilot.Action + ": " + reason + " (dry-run mode)"
		}
		stats[verdict]++
		if len(decisions) >= limit {
			continue
		}
		var output any
		if verdict == "dry-run" {
			output = "kubectl -n " + in.Namespace + " delete pod -l app.kubernetes.io/name=" + in.Workload + " --dry-run=server"
		}
		decisions = append(decisions, map[string]any{
			"time": in.Started.Add(4 * time.Minute).Format(time.RFC3339Nano), "report_id": in.ID,
			"resource": incidentResource(in, snap), "severity": in.Severity, "confidence": in.Confidence,
			"root_cause": in.RootCause, "action": in.Autopilot.Action, "verdict": verdict, "reason": reason, "output": output,
		})
	}
	writeJSON(w, map[string]any{
		"enabled": true,
		"policy": map[string]any{
			"mode": demoMode, "min_confidence": 0.8,
			"allowed_actions": []string{"restart_pod", "scale"},
			"max_risk":        "low", "allowed_namespaces": []string{"payments", "storefront", "catalog"},
			"blocked_namespaces": []string{"kube-system", "data"},
			"cooldown":           int64(5 * time.Minute), "max_actions_per_hour": 10,
		},
		"decisions": decisions,
		"stats":     stats,
	})
}

// ---------------------------------------------------------------------------
// Public status for the landing page (no auth, aggregate only)

func (s *server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	snap := s.snap()
	readyNodes, running, failing := 0, 0, 0
	cpu, mem := 0, 0
	for _, n := range nodes {
		st := snap.NodeStats[n.Name]
		if st.Ready {
			readyNodes++
			cpu += st.CPUPercent
			mem += st.MemPercent
		}
	}
	for _, p := range snap.Pods {
		if p.Ready {
			running++
		} else {
			failing++
		}
	}
	incidents := make([]map[string]any, 0)
	for _, in := range snap.Active {
		res := incidentResource(in, snap)
		target := fmt.Sprintf("%s/%s", res["namespace"], res["name"])
		if in.Workload == "" {
			target = "node/" + in.Node
		}
		incidents = append(incidents, map[string]any{
			"severity": in.Severity, "title": in.Title, "resource": target,
			"since": in.Started.Format(time.RFC3339), "summary": in.Summary,
		})
	}
	recent := make([]map[string]any, 0, 6)
	for _, e := range snap.events() {
		if len(recent) == 6 {
			break
		}
		obj := e.Kind + "/" + e.Name
		if e.Namespace != "" {
			obj = e.Namespace + "/" + e.Name
		}
		recent = append(recent, map[string]any{"type": e.Type, "reason": e.Reason, "object": obj, "message": e.Message, "last_seen": e.LastSeen.Format(time.RFC3339)})
	}
	if readyNodes == 0 {
		readyNodes = 1
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, map[string]any{
		"mock": true, "cluster": clusterName, "kubernetes_version": kubeVersion,
		"generated_at": snap.Now.Format(time.RFC3339),
		"nodes":        map[string]int{"ready": countReady(snap), "total": len(nodes)},
		"pods":         map[string]int{"running": running, "failing": failing, "total": len(snap.Pods)},
		"namespaces":   len(namespaceList),
		"cpu_percent":  cpu / readyNodes, "memory_percent": mem / readyNodes,
		"incidents":     incidents,
		"events":        recent,
		"next_rotation": snap.Now.Truncate(incidentSlot).Add(incidentSlot).Format(time.RFC3339),
	})
}

func countReady(snap snapshot) int {
	n := 0
	for _, st := range snap.NodeStats {
		if st.Ready {
			n++
		}
	}
	return n
}
