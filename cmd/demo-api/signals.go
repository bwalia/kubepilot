package main

// signals.go — events, logs and YAML derived from the simulated cluster.

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type event struct {
	Reason, Message, Type, Source string
	Count                         int
	FirstSeen, LastSeen           time.Time
	Kind, Name, Namespace         string
}

func (e event) JSON() map[string]any {
	return map[string]any{
		"reason": e.Reason, "message": e.Message, "type": e.Type, "count": e.Count,
		"first_seen": e.FirstSeen.Format(time.RFC3339), "last_seen": e.LastSeen.Format(time.RFC3339),
		"involved_object": map[string]string{"kind": e.Kind, "name": e.Name, "namespace": e.Namespace},
		"source":          e.Source,
	}
}

func (s snapshot) events() []event {
	now := s.Now
	var out []event
	add := func(e event) {
		if e.Count == 0 {
			e.Count = 1
		}
		if e.LastSeen.IsZero() {
			e.LastSeen = e.FirstSeen
		}
		if e.LastSeen.After(now) || e.FirstSeen.After(now) {
			return
		}
		out = append(out, e)
	}

	for _, p := range s.Pods {
		in := p.Incident
		if in == nil {
			continue
		}
		since := now.Sub(in.Started)
		switch {
		case in.Kind == "nodenotready":
			continue // node-level events below
		case in.Kind == "crashloop":
			add(event{Reason: "BackOff", Type: "Warning", Source: "kubelet", Kind: "Pod", Name: p.Name, Namespace: p.Namespace,
				Message: "Back-off restarting failed container checkout in pod " + p.Name + "_payments",
				Count:   p.Restarts + 2, FirstSeen: in.Started.Add(40 * time.Second), LastSeen: now.Add(-time.Duration(hash64(now.Truncate(time.Minute).String())%50) * time.Second)})
			add(event{Reason: "Unhealthy", Type: "Warning", Source: "kubelet", Kind: "Pod", Name: p.Name, Namespace: p.Namespace,
				Message: "Readiness probe failed: Get \"http://" + p.IP + ":8080/readyz\": dial tcp " + p.IP + ":8080: connect: connection refused",
				Count:   p.Restarts*3 + 1, FirstSeen: in.Started.Add(15 * time.Second), LastSeen: now.Add(-95 * time.Second)})
			add(event{Reason: "Pulled", Type: "Normal", Source: "kubelet", Kind: "Pod", Name: p.Name, Namespace: p.Namespace,
				Message: "Container image \"" + p.Image + "\" already present on machine",
				Count:   p.Restarts + 1, FirstSeen: in.Started, LastSeen: now.Add(-4 * time.Minute)})
			add(event{Reason: "Updated", Type: "Normal", Source: "argocd-application-controller", Kind: "ConfigMap", Name: "checkout-api-config", Namespace: "payments",
				Message: "Updated by sync 41f2c9e (REDIS_HOST: redis-cart.payments.svc → localhost)", FirstSeen: in.Started.Add(-2 * time.Minute)})
		case in.Kind == "oom":
			add(event{Reason: "OOMKilling", Type: "Warning", Source: "kernel-monitor", Kind: "Pod", Name: p.Name, Namespace: p.Namespace,
				Message: "Memory cgroup out of memory: Killed process 1 (search-indexer) total-vm:1384220kB, anon-rss:524112kB",
				Count:   p.Restarts - 1, FirstSeen: in.Started.Add(90 * time.Second), LastSeen: in.Started.Add(time.Duration(int(since.Minutes()/6)) * 6 * time.Minute).Add(90 * time.Second)})
			add(event{Reason: "BackOff", Type: "Warning", Source: "kubelet", Kind: "Pod", Name: p.Name, Namespace: p.Namespace,
				Message: "Back-off restarting failed container search-indexer in pod " + p.Name + "_catalog",
				Count:   p.Restarts, FirstSeen: in.Started.Add(2 * time.Minute), LastSeen: now.Add(-50 * time.Second)})
		case in.Kind == "imagepull":
			add(event{Reason: "Failed", Type: "Warning", Source: "kubelet", Kind: "Pod", Name: p.Name, Namespace: p.Namespace,
				Message: "Failed to pull image \"ghcr.io/acme-shop/web-frontend:2.9.0\": rpc error: code = NotFound desc = failed to pull and unpack image: manifest unknown",
				Count:   int(since.Minutes()/2) + 3, FirstSeen: in.Started.Add(5 * time.Second), LastSeen: now.Add(-70 * time.Second)})
			add(event{Reason: "BackOff", Type: "Normal", Source: "kubelet", Kind: "Pod", Name: p.Name, Namespace: p.Namespace,
				Message: "Back-off pulling image \"ghcr.io/acme-shop/web-frontend:2.9.0\"",
				Count:   int(since.Minutes()) + 4, FirstSeen: in.Started.Add(20 * time.Second), LastSeen: now.Add(-20 * time.Second)})
			add(event{Reason: "ScalingReplicaSet", Type: "Normal", Source: "deployment-controller", Kind: "Deployment", Name: "web-frontend", Namespace: "storefront",
				Message: "Scaled up replica set " + p.ReplicaSet + " to 1", FirstSeen: in.Started})
		case in.Kind == "diskpressure":
			add(event{Reason: "Evicted", Type: "Warning", Source: "kubelet", Kind: "Pod", Name: p.Name, Namespace: p.Namespace,
				Message: "The node was low on resource: ephemeral-storage. Threshold quantity: 15%, available: 11%.", FirstSeen: in.Started.Add(time.Minute)})
			add(event{Reason: "FailedScheduling", Type: "Warning", Source: "default-scheduler", Kind: "Pod", Name: p.Name, Namespace: p.Namespace,
				Message: p.StateMessage + ". preemption: 0/8 nodes are available: 8 Preemption is not helpful for scheduling.",
				Count:   int(since.Minutes()) + 1, FirstSeen: in.Started.Add(2 * time.Minute), LastSeen: now.Add(-40 * time.Second)})
			add(event{Reason: "EvictionThresholdMet", Type: "Warning", Source: "kubelet", Kind: "Node", Name: in.Node, Namespace: "",
				Message: "Attempting to reclaim ephemeral-storage", Count: int(since.Minutes()/3) + 1, FirstSeen: in.Started, LastSeen: now.Add(-3 * time.Minute)})
			add(event{Reason: "NodeHasDiskPressure", Type: "Normal", Source: "kubelet", Kind: "Node", Name: in.Node, Namespace: "",
				Message: "Node " + in.Node + " status is now: NodeHasDiskPressure", FirstSeen: in.Started})
		}
	}
	for _, in := range s.Active {
		if in.Kind == "nodenotready" {
			add(event{Reason: "NodeNotReady", Type: "Normal", Source: "node-controller", Kind: "Node", Name: in.Node, Namespace: "",
				Message: "Node " + in.Node + " status is now: NodeNotReady", FirstSeen: in.Started})
			add(event{Reason: "SystemOOM", Type: "Warning", Source: "kubelet", Kind: "Node", Name: in.Node, Namespace: "",
				Message: "System OOM encountered, victim process: k3s-agent, pid: 1184", FirstSeen: in.Started.Add(-40 * time.Second)})
			for _, p := range s.Pods {
				if p.Node == in.Node && p.Workload.Kind != "DaemonSet" {
					add(event{Reason: "TaintManagerEviction", Type: "Normal", Source: "taint-eviction-controller", Kind: "Pod", Name: p.Name, Namespace: p.Namespace,
						Message: "Marking for deletion Pod " + p.Namespace + "/" + p.Name, FirstSeen: in.Started.Add(5 * time.Minute)})
				}
			}
		}
	}

	// Routine cluster activity, recurring so the timeline always looks current.
	hpa := now.Truncate(17 * time.Minute)
	replicas := 3 + int(hash64(hpa.String())%3)
	add(event{Reason: "SuccessfulRescale", Type: "Normal", Source: "horizontal-pod-autoscaler", Kind: "HorizontalPodAutoscaler", Name: "recommendations", Namespace: "storefront",
		Message: fmt.Sprintf("New size: %d; reason: cpu resource utilization (percentage of request) above target", replicas), FirstSeen: hpa.Add(-4 * time.Hour), LastSeen: hpa, Count: 14})
	backup := now.Truncate(time.Hour)
	add(event{Reason: "SuccessfulCreate", Type: "Normal", Source: "cronjob-controller", Kind: "CronJob", Name: "postgres-backup", Namespace: "data",
		Message: fmt.Sprintf("Created job postgres-backup-%d", backup.Unix()/60), FirstSeen: backup})
	add(event{Reason: "SawCompletedJob", Type: "Normal", Source: "cronjob-controller", Kind: "CronJob", Name: "postgres-backup", Namespace: "data",
		Message: fmt.Sprintf("Saw completed job: postgres-backup-%d, status: Complete", backup.Unix()/60), FirstSeen: backup.Add(3*time.Minute + 12*time.Second)})
	sync := now.Truncate(3 * time.Minute)
	add(event{Reason: "ResourceUpdated", Type: "Normal", Source: "argocd-application-controller", Kind: "Application", Name: "storefront", Namespace: "argocd",
		Message: "Updated sync status: Synced -> Synced; health: Healthy", FirstSeen: sync.Add(-6 * time.Hour), LastSeen: sync, Count: 120})
	cert := periodStart(now, 24*time.Hour, "cert")
	add(event{Reason: "Issuing", Type: "Normal", Source: "cert-manager-certificates-trigger", Kind: "Certificate", Name: "shop-acme-example-tls", Namespace: "ingress-nginx",
		Message: "Renewing certificate as renewal was scheduled at " + cert.Format(time.RFC3339), FirstSeen: cert})
	add(event{Reason: "Issued", Type: "Normal", Source: "cert-manager-certificates-issuing", Kind: "Certificate", Name: "shop-acme-example-tls", Namespace: "ingress-nginx",
		Message: "The certificate has been successfully issued", FirstSeen: cert.Add(41 * time.Second)})
	for _, w := range workloads {
		if w.Kind != "Deployment" {
			continue
		}
		start := periodStart(now, rolloutPeriod, w.Namespace+w.Name)
		if now.Sub(start) > 36*time.Hour {
			continue
		}
		rs := w.Name + "-" + k8sSuffix(10, w.Name, w.Image, start.String())
		add(event{Reason: "ScalingReplicaSet", Type: "Normal", Source: "deployment-controller", Kind: "Deployment", Name: w.Name, Namespace: w.Namespace,
			Message: fmt.Sprintf("Scaled up replica set %s to %d", rs, w.Replicas), FirstSeen: start})
	}
	for _, p := range s.Pods {
		if p.Incident != nil || now.Sub(p.Started) > 6*time.Hour || p.Node == "" {
			continue
		}
		add(event{Reason: "Scheduled", Type: "Normal", Source: "default-scheduler", Kind: "Pod", Name: p.Name, Namespace: p.Namespace,
			Message: "Successfully assigned " + p.Namespace + "/" + p.Name + " to " + p.Node, FirstSeen: p.Started.Add(-3 * time.Second)})
		add(event{Reason: "Pulled", Type: "Normal", Source: "kubelet", Kind: "Pod", Name: p.Name, Namespace: p.Namespace,
			Message: fmt.Sprintf("Successfully pulled image %q in 2.%03ds", p.Image, hash64(p.Name)%900), FirstSeen: p.Started.Add(-time.Second)})
		add(event{Reason: "Started", Type: "Normal", Source: "kubelet", Kind: "Pod", Name: p.Name, Namespace: p.Namespace,
			Message: "Started container " + p.Workload.Name, FirstSeen: p.Started})
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].LastSeen.After(out[j].LastSeen) })
	return out
}

// ---------------------------------------------------------------------------
// Logs

var httpPaths = map[string][]string{
	"checkout-api":    {"POST /v1/checkout", "GET /v1/cart/totals", "POST /v1/checkout/confirm", "GET /healthz"},
	"payment-gateway": {"POST /v2/payments/authorize", "POST /v2/payments/capture", "GET /v2/payments/status"},
	"cart-service":    {"GET /cart", "POST /cart/items", "DELETE /cart/items"},
	"search-indexer":  {"GET /metrics"},
	"web-frontend":    {"GET /", "GET /products/[slug]", "GET /api/cart", "GET /_next/static/chunks/main.js", "GET /checkout"},
	"catalog-api":     {"GET /api/products", "GET /api/products/{id}", "GET /api/categories"},
	"fraud-scorer":    {"POST /score", "GET /health"},
	"recommendations": {"POST /recommend", "GET /health"},
}

// logLines returns tail lines ending at now. Lines are keyed to fixed time
// slots, so a refresh shows new lines arriving rather than a reshuffle.
func (s snapshot) logLines(p pod, tail int) []string {
	if tail <= 0 || tail > 1000 {
		tail = 200
	}
	now := s.Now
	if p.Incident != nil && p.Incident.Kind == "imagepull" {
		return []string{fmt.Sprintf(`container "%s" in pod "%s" is waiting to start: trying and failing to pull image`, p.Workload.Name, p.Name)}
	}
	if p.Incident != nil && (p.Incident.Kind == "diskpressure" || p.Incident.Kind == "nodenotready") {
		return []string{fmt.Sprintf("Unable to retrieve container logs: pod %s is not running on a reachable node", p.Name)}
	}
	if p.Incident != nil && p.Incident.Kind == "crashloop" {
		return crashLogs(p, now)
	}

	step := 4*time.Second + time.Duration(hash64(p.Name)%9)*time.Second
	end := now.Truncate(step)
	lines := make([]string, 0, tail)
	for i := tail - 1; i >= 0; i-- {
		t := end.Add(-time.Duration(i) * step)
		if t.Before(p.Started) {
			continue
		}
		lines = append(lines, logLine(p, t))
	}
	if p.Incident != nil && p.Incident.Kind == "oom" {
		lines = append(lines,
			now.Add(-2*time.Second).Format(time.RFC3339Nano)+` {"level":"info","msg":"reading batch","batch":43,"docs":50000,"heap_mb":498}`,
			now.Add(-time.Second).Format(time.RFC3339Nano)+` {"level":"warn","msg":"heap above 90% of GOMEMLIMIT","heap_mb":507,"limit_mb":512}`)
	}
	return lines
}

func crashLogs(p pod, now time.Time) []string {
	start := now.Add(-time.Duration(hash64(now.Truncate(time.Minute).String())%40+5) * time.Second)
	ts := func(d time.Duration) string { return start.Add(d).Format(time.RFC3339Nano) }
	return []string{
		ts(0) + ` {"level":"info","msg":"starting checkout-api","version":"1.14.2","commit":"9c1e7d2","go":"go1.23.2"}`,
		ts(12*time.Millisecond) + ` {"level":"info","msg":"config loaded","source":"/etc/checkout/config.yaml","redis_host":"localhost","redis_port":6379}`,
		ts(31*time.Millisecond) + ` {"level":"info","msg":"connecting to postgres","host":"postgres-0.postgres.data.svc","db":"orders"}`,
		ts(88*time.Millisecond) + ` {"level":"info","msg":"postgres pool ready","max_conns":20}`,
		ts(90*time.Millisecond) + ` {"level":"info","msg":"connecting to redis","addr":"localhost:6379"}`,
		ts(1092*time.Millisecond) + ` {"level":"error","msg":"redis ping failed","addr":"localhost:6379","error":"dial tcp 127.0.0.1:6379: connect: connection refused","attempt":1}`,
		ts(3095*time.Millisecond) + ` {"level":"error","msg":"redis ping failed","addr":"localhost:6379","error":"dial tcp 127.0.0.1:6379: connect: connection refused","attempt":2}`,
		ts(7101*time.Millisecond) + ` {"level":"error","msg":"redis ping failed","addr":"localhost:6379","error":"dial tcp 127.0.0.1:6379: connect: connection refused","attempt":3}`,
		ts(7102*time.Millisecond) + ` {"level":"fatal","msg":"cannot start without session store","error":"redis unavailable after 3 attempts"}`,
	}
}

func logLine(p pod, t time.Time) string {
	r := hash64(p.Name, t.String())
	ts := t.Add(time.Duration(r%999) * time.Millisecond)
	paths := httpPaths[p.Workload.Name]
	path := "GET /"
	if len(paths) > 0 {
		path = paths[r%uint64(len(paths))]
	}
	method, route, _ := strings.Cut(path, " ")
	status := 200
	switch r % 53 {
	case 0:
		status = 500
	case 1, 2:
		status = 404
	case 3:
		status = 429
	}
	if strings.HasPrefix(method, "POST") && status == 200 {
		status = 201
	}
	ms := 3 + r%180
	trace := fmt.Sprintf("%016x", r)
	switch p.Workload.LogStyle {
	case "go":
		lvl := "info"
		if status >= 500 {
			lvl = "error"
		}
		return fmt.Sprintf(`%s {"level":%q,"msg":"request","method":%q,"path":%q,"status":%d,"duration_ms":%d,"trace_id":%q}`,
			ts.Format(time.RFC3339Nano), lvl, method, route, status, ms, trace)
	case "node":
		return fmt.Sprintf(`%s  %s %s %d in %dms`, ts.Format("2006-01-02T15:04:05.000Z"), method, route, status, ms)
	case "python":
		lvl := "INFO"
		if status >= 500 {
			lvl = "ERROR"
		}
		return fmt.Sprintf(`%s %s uvicorn.access: 10.42.%d.%d:%d - "%s %s HTTP/1.1" %d`,
			ts.Format("2006-01-02 15:04:05,000"), lvl, r%8+1, r%250+3, 30000+r%30000, method, route, status)
	case "java":
		return fmt.Sprintf(`%s  INFO 1 --- [nio-8080-exec-%d] c.a.catalog.web.ProductController : %s %s -> %d (%d ms)`,
			ts.Format("2006-01-02T15:04:05.000Z07:00"), r%10+1, method, route, status, ms)
	case "nginx":
		hosts := []string{"shop.acme.example", "api.acme.example"}
		return fmt.Sprintf(`203.0.113.%d - - [%s] "%s %s HTTP/2.0" %d %d "-" "Mozilla/5.0" %d %.3f [storefront-web-frontend-3000] 10.42.%d.%d:3000 %d %.3f %d %s`,
			r%250+3, ts.Format("02/Jan/2006:15:04:05 -0700"), method, route, status, 512+r%40000, 380+r%200,
			float64(ms)/1000, r%8+1, r%250+3, 512+r%40000, float64(ms)/1000, status, hosts[r%2])
	case "redis":
		return fmt.Sprintf(`1:M %s * %d changes in 60 seconds. Saving...`, ts.Format("02 Jan 2006 15:04:05.000"), 100+r%9000)
	case "postgres":
		msgs := []string{
			"LOG:  checkpoint starting: time",
			fmt.Sprintf("LOG:  checkpoint complete: wrote %d buffers (%.1f%%); 0 WAL file(s) added, 0 removed, 1 recycled", 200+r%2000, float64(r%40)/10),
			fmt.Sprintf("LOG:  duration: %d.%03d ms  statement: SELECT * FROM orders WHERE customer_id = $1 ORDER BY created_at DESC LIMIT 20", 1+r%40, r%1000),
			"LOG:  automatic vacuum of table \"orders.public.cart_items\": index scans: 1",
		}
		return ts.Format("2006-01-02 15:04:05.000 UTC") + " [" + fmt.Sprint(100+r%900) + "] " + msgs[r%uint64(len(msgs))]
	case "kafka":
		return fmt.Sprintf(`[%s] INFO [ReplicaFetcher replicaId=%d, leaderId=%d, fetcherId=0] Truncating partition orders-%d to offset %d (kafka.server.ReplicaFetcherThread)`,
			ts.Format("2006-01-02 15:04:05,000"), p.Index, (p.Index+1)%3, r%12, 1_000_000+r%9_000_000)
	case "elastic":
		return fmt.Sprintf(`{"@timestamp":%q,"log.level":"INFO","message":"[products-v7][%d] took [%dms] for bulk of [%d] docs","service.name":"ES_ECS","elasticsearch.node.name":%q}`,
			ts.Format(time.RFC3339Nano), r%5, ms, 500+r%4500, p.Name)
	default:
		return fmt.Sprintf(`%s level=info component=%s msg="reconcile complete" duration=%dms`, ts.Format(time.RFC3339), p.Workload.Name, ms)
	}
}

// ---------------------------------------------------------------------------
// YAML

func (s snapshot) yaml(kind, ns, name string) string {
	switch strings.ToLower(kind) {
	case "pod", "pods":
		if p, ok := s.findPod(ns, name); ok {
			return podYAML(p, s.Now)
		}
	case "deployment", "deployments", "statefulset", "statefulsets", "daemonset", "daemonsets":
		if w, ok := findWorkload(ns, name); ok {
			return workloadYAML(w, s)
		}
	case "service", "services":
		for _, svc := range s.services() {
			if svc["namespace"] == ns && svc["name"] == name {
				return serviceYAML(svc)
			}
		}
	case "node", "nodes":
		if n, ok := findNode(name); ok {
			return nodeYAML(n, s.NodeStats[n.Name])
		}
	}
	return fmt.Sprintf("# KubePilot demo: %s %s/%s is not part of the simulated cluster\napiVersion: v1\nkind: %s\nmetadata:\n  name: %s\n  namespace: %s\n", kind, ns, name, kind, name, ns)
}

func podYAML(p pod, now time.Time) string {
	w := p.Workload
	var b strings.Builder
	owner := "ReplicaSet\n    name: " + p.ReplicaSet
	if w.Kind != "Deployment" {
		owner = w.Kind + "\n    name: " + w.Name
	}
	fmt.Fprintf(&b, "# Simulated by the KubePilot demo API — not a real cluster\napiVersion: v1\nkind: Pod\nmetadata:\n  name: %s\n  namespace: %s\n  labels:\n    app.kubernetes.io/name: %s\n    app.kubernetes.io/part-of: acme-shop\n  creationTimestamp: \"%s\"\n  ownerReferences:\n  - apiVersion: apps/v1\n    kind: %s\n    controller: true\nspec:\n  nodeName: %s\n  serviceAccountName: %s\n  containers:\n  - name: %s\n    image: %s\n",
		p.Name, p.Namespace, w.Name, p.Started.Format(time.RFC3339), owner, p.Node, w.Name, w.Name, p.Image)
	if w.Port > 0 {
		fmt.Fprintf(&b, "    ports:\n    - containerPort: %d\n      protocol: TCP\n", w.Port)
	}
	if w.Name == "checkout-api" {
		b.WriteString("    envFrom:\n    - configMapRef:\n        name: checkout-api-config\n")
	}
	fmt.Fprintf(&b, "    resources:\n      requests:\n        cpu: 100m\n        memory: 128Mi\n      limits:\n        memory: %s\n", w.MemLimit)
	if w.Port > 0 && w.LogStyle != "infra" {
		fmt.Fprintf(&b, "    readinessProbe:\n      httpGet:\n        path: /readyz\n        port: %d\n      periodSeconds: 5\n", w.Port)
	}
	fmt.Fprintf(&b, "status:\n  phase: %s\n  podIP: %s\n  startTime: \"%s\"\n  containerStatuses:\n  - name: %s\n    ready: %t\n    restartCount: %d\n", p.Phase, p.IP, p.Started.Format(time.RFC3339), w.Name, p.Ready, p.Restarts)
	if p.Reason != "" {
		fmt.Fprintf(&b, "    state:\n      waiting:\n        reason: %s\n        message: %q\n", p.Reason, p.StateMessage)
	}
	return b.String()
}

func workloadYAML(w workload, s snapshot) string {
	desired, ready := s.workloadReady(w)
	api := "apps/v1"
	var b strings.Builder
	fmt.Fprintf(&b, "# Simulated by the KubePilot demo API — not a real cluster\napiVersion: %s\nkind: %s\nmetadata:\n  name: %s\n  namespace: %s\n  labels:\n    app.kubernetes.io/name: %s\n    app.kubernetes.io/managed-by: argocd\nspec:\n", api, w.Kind, w.Name, w.Namespace, w.Name)
	if w.Kind != "DaemonSet" {
		fmt.Fprintf(&b, "  replicas: %d\n", w.Replicas)
	}
	if w.Kind == "Deployment" {
		b.WriteString("  strategy:\n    type: RollingUpdate\n    rollingUpdate:\n      maxSurge: 1\n      maxUnavailable: 1\n")
	}
	fmt.Fprintf(&b, "  selector:\n    matchLabels:\n      app.kubernetes.io/name: %s\n  template:\n    spec:\n      containers:\n      - name: %s\n        image: %s\n        resources:\n          limits:\n            memory: %s\nstatus:\n  replicas: %d\n  readyReplicas: %d\n", w.Name, w.Name, w.Image, w.MemLimit, desired, ready)
	return b.String()
}

func serviceYAML(svc map[string]any) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Simulated by the KubePilot demo API — not a real cluster\napiVersion: v1\nkind: Service\nmetadata:\n  name: %s\n  namespace: %s\nspec:\n  type: %s\n  clusterIP: %s\n  ports:\n", svc["name"], svc["namespace"], svc["type"], svc["cluster_ip"])
	for _, p := range svc["ports"].([]map[string]any) {
		fmt.Fprintf(&b, "  - name: %s\n    port: %d\n    targetPort: %s\n    protocol: %s\n", p["name"], p["port"], p["target_port"], p["protocol"])
	}
	return b.String()
}

func nodeYAML(n node, st nodeStat) string {
	ready := "True"
	if !st.Ready {
		ready = "Unknown"
	}
	disk := "False"
	if st.DiskPressure {
		disk = "True"
	}
	return fmt.Sprintf("# Simulated by the KubePilot demo API — not a real cluster\napiVersion: v1\nkind: Node\nmetadata:\n  name: %s\n  labels:\n    kubernetes.io/hostname: %s\n    topology.kubernetes.io/zone: %s\n    node.kubernetes.io/instance-type: %s\nstatus:\n  capacity:\n    cpu: \"%d\"\n    memory: %dGi\n  addresses:\n  - type: InternalIP\n    address: %s\n  conditions:\n  - type: Ready\n    status: \"%s\"\n  - type: DiskPressure\n    status: \"%s\"\n  nodeInfo:\n    kubeletVersion: %s\n    osImage: Ubuntu 24.04.1 LTS\n    containerRuntimeVersion: containerd://1.7.22-k3s1\n",
		n.Name, n.Name, n.Zone, n.Instance, n.CPU, n.MemGi, n.LAN, ready, disk, kubeVersion)
}
