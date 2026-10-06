package main

// cluster.go — a deterministic simulation of a small production cluster.
//
// Everything is derived from the wall clock, not from process state, so every
// demo-api replica returns the same cluster at the same moment and a restart
// never resets it. Pods roll weekly, metrics drift on slow waves, restart
// counts climb with real back-off timing, and a secondary incident rotates
// every incidentSlot so the demo looks different from one visit to the next.

import (
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	clusterName    = "kp-demo-prod-eu-west-2"
	kubeVersion    = "v1.31.4+k3s1"
	incidentSlot   = 30 * time.Minute
	primaryCycle   = 2 * time.Hour
	rolloutPeriod  = 7 * 24 * time.Hour
	statefulPeriod = 30 * 24 * time.Hour
	daemonPeriod   = 14 * 24 * time.Hour
)

// hash64 gives a stable pseudo-random number for any combination of strings.
func hash64(parts ...string) uint64 {
	h := fnv.New64a()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return h.Sum64()
}

// k8sSuffix mimics the random suffixes Kubernetes appends to generated names.
func k8sSuffix(n int, parts ...string) string {
	const alphabet = "bcdfghjklmnpqrstvwxz2456789"
	v := hash64(parts...)
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(alphabet[v%uint64(len(alphabet))])
		v /= uint64(len(alphabet))
		if v == 0 {
			v = hash64(append(parts, fmt.Sprint(i))...)
		}
	}
	return b.String()
}

// periodStart returns the most recent start of a repeating window, offset per
// key so different workloads roll at different times.
func periodStart(now time.Time, period time.Duration, key string) time.Time {
	off := time.Duration(hash64(key) % uint64(period))
	return now.Add(-off).Truncate(period).Add(off)
}

// wave is a smooth value in [-1, 1] that drifts with time.
func wave(now time.Time, period time.Duration, key string) float64 {
	phase := float64(hash64(key)%1000) / 1000 * 2 * math.Pi
	return math.Sin(2*math.Pi*float64(now.UnixNano())/float64(period) + phase)
}

// jitter is a per-minute value in [-1, 1].
func jitter(now time.Time, key string) float64 {
	return float64(hash64(key, now.Truncate(time.Minute).String())%2001)/1000 - 1
}

func humanAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// ---------------------------------------------------------------------------
// Nodes

type node struct {
	Name, Zone, Instance string
	CPU                  int // cores
	MemGi                int
	LAN, WAN, Tunnel     string
	ControlPlane         bool
	baseCPU, baseMem     float64
}

var nodes = []node{
	{Name: "kp-cp-1", Zone: "eu-west-2a", Instance: "m6i.xlarge", CPU: 4, MemGi: 16, LAN: "10.20.0.11", WAN: "203.0.113.11", Tunnel: "100.64.0.1", ControlPlane: true, baseCPU: 31, baseMem: 54},
	{Name: "kp-cp-2", Zone: "eu-west-2b", Instance: "m6i.xlarge", CPU: 4, MemGi: 16, LAN: "10.20.0.12", Tunnel: "100.64.0.2", ControlPlane: true, baseCPU: 27, baseMem: 51},
	{Name: "kp-cp-3", Zone: "eu-west-2c", Instance: "m6i.xlarge", CPU: 4, MemGi: 16, LAN: "10.20.0.13", Tunnel: "100.64.0.3", ControlPlane: true, baseCPU: 25, baseMem: 49},
	{Name: "kp-worker-1", Zone: "eu-west-2a", Instance: "m6i.2xlarge", CPU: 8, MemGi: 32, LAN: "10.20.1.21", Tunnel: "100.64.1.21", baseCPU: 46, baseMem: 63},
	{Name: "kp-worker-2", Zone: "eu-west-2b", Instance: "m6i.2xlarge", CPU: 8, MemGi: 32, LAN: "10.20.1.22", Tunnel: "100.64.1.22", baseCPU: 52, baseMem: 68},
	{Name: "kp-worker-3", Zone: "eu-west-2c", Instance: "m6i.2xlarge", CPU: 8, MemGi: 32, LAN: "10.20.1.23", Tunnel: "100.64.1.23", baseCPU: 41, baseMem: 59},
	{Name: "kp-worker-4", Zone: "eu-west-2a", Instance: "r6i.2xlarge", CPU: 8, MemGi: 64, LAN: "10.20.1.24", Tunnel: "100.64.1.24", baseCPU: 38, baseMem: 71},
	{Name: "kp-worker-5", Zone: "eu-west-2b", Instance: "c6i.2xlarge", CPU: 8, MemGi: 16, LAN: "10.20.1.25", Tunnel: "100.64.1.25", baseCPU: 57, baseMem: 62},
}

func workerNames() []string {
	var out []string
	for _, n := range nodes {
		if !n.ControlPlane {
			out = append(out, n.Name)
		}
	}
	return out
}

func findNode(name string) (node, bool) {
	for _, n := range nodes {
		if n.Name == name {
			return n, true
		}
	}
	return node{}, false
}

// ---------------------------------------------------------------------------
// Workloads

type workload struct {
	Kind      string // Deployment | StatefulSet | DaemonSet
	Name      string
	Namespace string
	Image     string
	Replicas  int
	Port      int
	LogStyle  string // go | node | python | java | nginx | redis | postgres | kafka | elastic | infra
	MemLimit  string
	OnControl bool // DaemonSets and system pods may run on control-plane nodes
}

var workloads = []workload{
	{Kind: "Deployment", Name: "checkout-api", Namespace: "payments", Image: "ghcr.io/acme-shop/checkout-api:1.14.2", Replicas: 3, Port: 8080, LogStyle: "go", MemLimit: "512Mi"},
	{Kind: "Deployment", Name: "payment-gateway", Namespace: "payments", Image: "ghcr.io/acme-shop/payment-gateway:3.2.0", Replicas: 2, Port: 8443, LogStyle: "go", MemLimit: "384Mi"},
	{Kind: "Deployment", Name: "fraud-scorer", Namespace: "payments", Image: "ghcr.io/acme-shop/fraud-scorer:0.19.4", Replicas: 2, Port: 8000, LogStyle: "python", MemLimit: "1Gi"},
	{Kind: "StatefulSet", Name: "redis-cart", Namespace: "payments", Image: "redis:7.2.5-alpine", Replicas: 1, Port: 6379, LogStyle: "redis", MemLimit: "1Gi"},
	{Kind: "Deployment", Name: "web-frontend", Namespace: "storefront", Image: "ghcr.io/acme-shop/web-frontend:2.8.3", Replicas: 3, Port: 3000, LogStyle: "node", MemLimit: "768Mi"},
	{Kind: "Deployment", Name: "cart-service", Namespace: "storefront", Image: "ghcr.io/acme-shop/cart-service:1.6.1", Replicas: 2, Port: 8080, LogStyle: "go", MemLimit: "256Mi"},
	{Kind: "Deployment", Name: "recommendations", Namespace: "storefront", Image: "ghcr.io/acme-shop/recommendations:4.0.2", Replicas: 2, Port: 8000, LogStyle: "python", MemLimit: "2Gi"},
	{Kind: "Deployment", Name: "catalog-api", Namespace: "catalog", Image: "ghcr.io/acme-shop/catalog-api:5.3.0", Replicas: 2, Port: 8080, LogStyle: "java", MemLimit: "1536Mi"},
	{Kind: "Deployment", Name: "search-indexer", Namespace: "catalog", Image: "ghcr.io/acme-shop/search-indexer:2.1.7", Replicas: 1, Port: 9090, LogStyle: "go", MemLimit: "512Mi"},
	{Kind: "StatefulSet", Name: "elasticsearch", Namespace: "catalog", Image: "docker.elastic.co/elasticsearch/elasticsearch:8.15.1", Replicas: 3, Port: 9200, LogStyle: "elastic", MemLimit: "4Gi"},
	{Kind: "StatefulSet", Name: "postgres", Namespace: "data", Image: "bitnami/postgresql:16.4.0", Replicas: 2, Port: 5432, LogStyle: "postgres", MemLimit: "4Gi"},
	{Kind: "StatefulSet", Name: "kafka", Namespace: "data", Image: "bitnami/kafka:3.8.0", Replicas: 3, Port: 9092, LogStyle: "kafka", MemLimit: "3Gi"},
	{Kind: "StatefulSet", Name: "prometheus-k8s", Namespace: "observability", Image: "quay.io/prometheus/prometheus:v2.54.1", Replicas: 1, Port: 9090, LogStyle: "infra", MemLimit: "6Gi"},
	{Kind: "Deployment", Name: "grafana", Namespace: "observability", Image: "grafana/grafana:11.2.2", Replicas: 1, Port: 3000, LogStyle: "infra", MemLimit: "512Mi"},
	{Kind: "StatefulSet", Name: "loki", Namespace: "observability", Image: "grafana/loki:3.2.0", Replicas: 1, Port: 3100, LogStyle: "infra", MemLimit: "2Gi"},
	{Kind: "DaemonSet", Name: "node-exporter", Namespace: "observability", Image: "quay.io/prometheus/node-exporter:v1.8.2", Port: 9100, LogStyle: "infra", MemLimit: "64Mi", OnControl: true},
	{Kind: "DaemonSet", Name: "fluent-bit", Namespace: "observability", Image: "cr.fluentbit.io/fluent/fluent-bit:3.1.9", Port: 2020, LogStyle: "infra", MemLimit: "128Mi", OnControl: true},
	{Kind: "Deployment", Name: "ingress-nginx-controller", Namespace: "ingress-nginx", Image: "registry.k8s.io/ingress-nginx/controller:v1.11.3", Replicas: 2, Port: 443, LogStyle: "nginx", MemLimit: "512Mi"},
	{Kind: "Deployment", Name: "cert-manager", Namespace: "cert-manager", Image: "quay.io/jetstack/cert-manager-controller:v1.16.1", Replicas: 1, Port: 9402, LogStyle: "infra", MemLimit: "256Mi"},
	{Kind: "Deployment", Name: "cert-manager-webhook", Namespace: "cert-manager", Image: "quay.io/jetstack/cert-manager-webhook:v1.16.1", Replicas: 1, Port: 10250, LogStyle: "infra", MemLimit: "128Mi"},
	{Kind: "Deployment", Name: "argocd-server", Namespace: "argocd", Image: "quay.io/argoproj/argocd:v2.12.4", Replicas: 1, Port: 8080, LogStyle: "infra", MemLimit: "256Mi"},
	{Kind: "Deployment", Name: "argocd-repo-server", Namespace: "argocd", Image: "quay.io/argoproj/argocd:v2.12.4", Replicas: 1, Port: 8081, LogStyle: "infra", MemLimit: "512Mi"},
	{Kind: "StatefulSet", Name: "argocd-application-controller", Namespace: "argocd", Image: "quay.io/argoproj/argocd:v2.12.4", Replicas: 1, Port: 8082, LogStyle: "infra", MemLimit: "1Gi"},
	{Kind: "Deployment", Name: "coredns", Namespace: "kube-system", Image: "rancher/mirrored-coredns-coredns:1.11.3", Replicas: 2, Port: 53, LogStyle: "infra", MemLimit: "170Mi", OnControl: true},
	{Kind: "Deployment", Name: "metrics-server", Namespace: "kube-system", Image: "rancher/mirrored-metrics-server:v0.7.2", Replicas: 1, Port: 10250, LogStyle: "infra", MemLimit: "200Mi", OnControl: true},
	{Kind: "Deployment", Name: "local-path-provisioner", Namespace: "kube-system", Image: "rancher/local-path-provisioner:v0.0.30", Replicas: 1, Port: 0, LogStyle: "infra", MemLimit: "128Mi", OnControl: true},
}

var namespaceList = []struct {
	Name   string
	Labels map[string]string
}{
	{"payments", map[string]string{"team": "payments", "env": "production", "kubepilot.io/tier": "critical"}},
	{"storefront", map[string]string{"team": "web", "env": "production"}},
	{"catalog", map[string]string{"team": "search", "env": "production"}},
	{"data", map[string]string{"team": "platform-data", "env": "production"}},
	{"observability", map[string]string{"team": "sre"}},
	{"ingress-nginx", map[string]string{"app.kubernetes.io/name": "ingress-nginx"}},
	{"cert-manager", map[string]string{"app.kubernetes.io/name": "cert-manager"}},
	{"argocd", map[string]string{"app.kubernetes.io/part-of": "argocd"}},
	{"kube-system", map[string]string{}},
	{"default", map[string]string{}},
}

func findWorkload(ns, name string) (workload, bool) {
	for _, w := range workloads {
		if w.Namespace == ns && w.Name == name {
			return w, true
		}
	}
	return workload{}, false
}

// ---------------------------------------------------------------------------
// Pods

type pod struct {
	Name, Namespace, Node string
	Workload              workload
	Index                 int
	Image                 string
	Phase, Reason         string
	Ready                 bool
	Restarts              int
	Started               time.Time
	ReplicaSet            string
	IP                    string
	Incident              *incident
	ExitCode              int
	StateMessage          string
}

func (p pod) Age(now time.Time) time.Duration { return now.Sub(p.Started) }

func (p pod) Key() string { return p.Namespace + "/" + p.Name }

// ---------------------------------------------------------------------------
// Incidents

type incident struct {
	ID          string
	Kind        string // crashloop | oom | imagepull | diskpressure | nodenotready
	Severity    string
	Namespace   string
	Workload    string
	PodIndex    int    // -1 for node incidents
	Node        string // node involved, if any
	Started     time.Time
	Resolved    bool
	ResolvedAt  time.Time
	Title       string
	Summary     string
	RootCause   string
	Analysis    string
	Rule        string
	Confidence  float64
	Evidence    []string
	Remediation []string
	Suggestions []string
	Action      map[string]any // SuggestedAction for the AI fixer
	Autopilot   autopilotPlan
}

type autopilotPlan struct {
	Action  string
	Verdict string // decision when the mode allows it; escalated/skipped are mode-independent
	Reason  string
}

// primaryIncident is always active: one checkout-api replica crash loops
// because a config change pointed REDIS_HOST at localhost.
func primaryIncident(now time.Time) incident {
	start := now.Truncate(primaryCycle).Add(-11 * time.Minute)
	return incident{
		ID: "rca-" + start.Format("0102-1504") + "-checkout", Kind: "crashloop", Severity: "critical",
		Namespace: "payments", Workload: "checkout-api", PodIndex: 0, Started: start,
		Title:     "checkout-api is crash looping",
		Summary:   "One checkout-api replica exits with code 1 seconds after start: it cannot reach Redis at 127.0.0.1:6379. The other two replicas still serve traffic, so checkout capacity is down by a third.",
		RootCause: "REDIS_HOST was changed to localhost in the checkout-api-config ConfigMap; the replica that restarted after the change can no longer reach the redis-cart Service.",
		Analysis:  "The two healthy replicas started before ConfigMap checkout-api-config was edited and still hold REDIS_HOST=redis-cart.payments.svc in their environment. The replica scheduled after the edit reads REDIS_HOST=localhost, fails its Redis ping, and exits with code 1. Kubelet back-off is now at 5 minutes. Rolling the Deployment would take down the healthy replicas too, so fix the ConfigMap first.",
		Rule:      "crashloop_threshold", Confidence: 0.93,
		Evidence: []string{
			"Logs: dial tcp 127.0.0.1:6379: connect: connection refused (every start)",
			"Events: Back-off restarting failed container checkout (count climbing)",
			"ConfigMap checkout-api-config: REDIS_HOST changed to localhost by argocd (sync 41f2c9e)",
			"Sibling replicas healthy with REDIS_HOST=redis-cart.payments.svc",
		},
		Remediation: []string{
			"Revert REDIS_HOST to redis-cart.payments.svc in checkout-api-config (git revert 41f2c9e)",
			"kubectl -n payments rollout restart deploy/checkout-api once the ConfigMap is fixed",
			"Add a startup probe so a bad Redis address fails the rollout instead of one replica",
		},
		Suggestions: []string{"Check REDIS_HOST in checkout-api-config", "Compare env of healthy and failing replicas", "kubectl -n payments logs -p <failing pod>"},
		Action: map[string]any{
			"type": "edit_env", "namespace": "payments", "resource": "checkout-api", "replicas": nil,
			"command":          "kubectl -n payments set env deploy/checkout-api REDIS_HOST=redis-cart.payments.svc",
			"explanation":      "Point checkout-api back at the redis-cart Service, then roll the Deployment",
			"requires_cr_code": true,
		},
		Autopilot: autopilotPlan{Action: "edit_env", Verdict: "escalated", Reason: "edit_env is not in the allowed action list — paged on-call with the RCA"},
	}
}

// rotatingIncident returns the secondary incident for the slot containing t.
func rotatingIncident(slotStart time.Time) incident {
	n := int(slotStart.Unix()/int64(incidentSlot.Seconds())) % 4
	if n < 0 {
		n += 4
	}
	stamp := slotStart.Format("0102-1504")
	switch n {
	case 0:
		return incident{
			ID: "rca-" + stamp + "-indexer", Kind: "oom", Severity: "high",
			Namespace: "catalog", Workload: "search-indexer", PodIndex: 0, Started: slotStart,
			Title:     "search-indexer is OOMKilled during reindex",
			Summary:   "search-indexer hits its 512Mi memory limit during the nightly full reindex and is killed by the kernel (exit code 137).",
			RootCause: "The catalog grew past 2.1M products; the indexer buffers a full batch of 50k documents in memory, which no longer fits in the 512Mi limit.",
			Analysis:  "Memory climbs linearly while the indexer reads a batch from catalog-api and drops back after each bulk request to Elasticsearch. Since the catalog import on Monday, a batch peaks at about 610Mi. The container is OOMKilled before the bulk request is sent, so the reindex restarts from the last checkpoint and never completes.",
			Rule:      "oom_killed", Confidence: 0.89,
			Evidence: []string{
				"Container lastState: OOMKilled, exit code 137",
				"Memory working set peaks at 611Mi against a 512Mi limit",
				"Catalog size 2.14M products (was 1.6M last week)",
			},
			Remediation: []string{
				"Lower INDEX_BATCH_SIZE from 50000 to 20000",
				"Raise the memory limit to 1Gi for search-indexer",
			},
			Suggestions: []string{"Check memory limit vs working set", "Reduce INDEX_BATCH_SIZE", "Look at recent catalog growth"},
			Action: map[string]any{
				"type": "patch_resources", "namespace": "catalog", "resource": "search-indexer", "replicas": nil,
				"command":          `kubectl -n catalog set resources deploy/search-indexer --limits=memory=1Gi`,
				"explanation":      "Give the indexer enough memory for a full batch",
				"requires_cr_code": false,
			},
			Autopilot: autopilotPlan{Action: "restart_pod", Verdict: "act", Reason: "OOM loop with confidence 0.89 ≥ 0.80; restart resumes from checkpoint"},
		}
	case 1:
		return incident{
			ID: "rca-" + stamp + "-frontend", Kind: "imagepull", Severity: "high",
			Namespace: "storefront", Workload: "web-frontend", PodIndex: 2, Started: slotStart,
			Title:     "web-frontend rollout stuck on ImagePullBackOff",
			Summary:   "The rollout of web-frontend 2.9.0 is stuck: the new pod cannot pull ghcr.io/acme-shop/web-frontend:2.9.0 and the rollout has stopped with 2 of 3 replicas available.",
			RootCause: "The CI pipeline pushed the image as 2.9.0-rc1, but the Helm values reference 2.9.0, which does not exist in the registry.",
			Analysis:  "Kubelet reports \"manifest unknown\" for tag 2.9.0. The registry has 2.9.0-rc1, pushed 14 minutes before the Argo CD sync. maxUnavailable=1 kept two old replicas serving, so customers are unaffected but capacity is reduced. This will not resolve itself: the tag will never appear.",
			Rule:      "image_pull_failure", Confidence: 0.96,
			Evidence: []string{
				"Event: Failed to pull image \"ghcr.io/acme-shop/web-frontend:2.9.0\": manifest unknown",
				"Registry has tag 2.9.0-rc1 only",
				"Deployment progressing condition: ReplicaSetUpdated, 2/3 available",
			},
			Remediation: []string{
				"Fix the image tag in storefront values (2.9.0-rc1) or promote the rc tag to 2.9.0",
				"Or roll back: kubectl -n storefront rollout undo deploy/web-frontend",
			},
			Suggestions: []string{"Compare the image tag with the registry", "kubectl -n storefront rollout status deploy/web-frontend"},
			Action: map[string]any{
				"type": "rollback", "namespace": "storefront", "resource": "web-frontend", "replicas": nil,
				"command":          "kubectl -n storefront rollout undo deploy/web-frontend",
				"explanation":      "Return to 2.8.3 until the correct image is published",
				"requires_cr_code": true,
			},
			Autopilot: autopilotPlan{Action: "rollback", Verdict: "escalated", Reason: "rollback requires a change-request code — waiting for approval"},
		}
	case 2:
		return incident{
			ID: "rca-" + stamp + "-postgres", Kind: "diskpressure", Severity: "high",
			Namespace: "data", Workload: "postgres", PodIndex: 1, Node: "kp-worker-4", Started: slotStart,
			Title:     "postgres-1 evicted by disk pressure on kp-worker-4",
			Summary:   "kp-worker-4 crossed the kubelet's 85% disk threshold. The replica postgres-1 was evicted and is Pending: its local volume is pinned to that node.",
			RootCause: "Container images and unrotated WAL archives filled the root disk of kp-worker-4; the local-path PVC for postgres-1 can only be scheduled back onto that node.",
			Analysis:  "Image garbage collection freed only 1.2GiB because most layers are in use. /var/lib/rancher/k3s/storage holds 61GiB of WAL archives that the archive_command copies but never deletes. Until disk is freed, the node keeps the disk-pressure taint and postgres-1 cannot reschedule. The primary, postgres-0, is healthy, but replication lag is growing.",
			Rule:      "node_disk_pressure", Confidence: 0.87,
			Evidence: []string{
				"Node condition DiskPressure=True on kp-worker-4 (nodefs.available 11%)",
				"Event: The node was low on resource: ephemeral-storage — evicted postgres-1",
				"FailedScheduling: node(s) had untolerated taint {node.kubernetes.io/disk-pressure}",
				"PVC data-postgres-1 bound to a local-path volume on kp-worker-4",
			},
			Remediation: []string{
				"Prune old WAL archives on kp-worker-4 (keep the last 24h)",
				"Fix archive_command retention in the postgres chart",
				"Consider network storage so replicas can move between nodes",
			},
			Suggestions: []string{"Check node disk usage", "Inspect WAL archive retention", "kubectl describe node kp-worker-4"},
			Action: map[string]any{
				"type": "kubectl", "namespace": "data", "resource": "postgres-1", "replicas": nil,
				"command":          "kubectl describe node kp-worker-4 | grep -A6 Conditions",
				"explanation":      "Confirm the disk-pressure condition before freeing space",
				"requires_cr_code": false,
			},
			Autopilot: autopilotPlan{Action: "cordon_node", Verdict: "skipped", Reason: "namespace data is not in the allowed namespaces"},
		}
	default:
		return incident{
			ID: "rca-" + stamp + "-worker5", Kind: "nodenotready", Severity: "critical",
			Namespace: "", Workload: "", PodIndex: -1, Node: "kp-worker-5", Started: slotStart,
			Title:     "kp-worker-5 is NotReady",
			Summary:   "kp-worker-5 stopped posting status 4 minutes ago. Its pods show Unknown and are being rescheduled to the other workers.",
			RootCause: "The k3s agent on kp-worker-5 was OOM-killed by the kernel after a fraud-scorer batch job used all of the node's 16Gi of memory.",
			Analysis:  "The node's last heartbeat came right after a memory spike to 99%. kp-worker-5 is a c6i.2xlarge with only 16Gi, and fraud-scorer has no memory limit on its batch sidecar. The node controller has tainted the node, and evictions start after the 5-minute toleration.",
			Rule:      "node_not_ready", Confidence: 0.84,
			Evidence: []string{
				"Node condition Ready=Unknown: Kubelet stopped posting node status",
				"Last node memory sample 99.1% (fraud-scorer batch)",
				"Taint node.kubernetes.io/unreachable:NoExecute applied by node-controller",
			},
			Remediation: []string{
				"Restart the k3s agent on kp-worker-5 (systemctl restart k3s-agent)",
				"Set a memory limit on the fraud-scorer batch sidecar",
				"Reserve memory for system daemons (--system-reserved=memory=1Gi)",
			},
			Suggestions: []string{"Check the node heartbeat", "Look for memory exhaustion before the outage", "kubectl get pods -A -o wide --field-selector spec.nodeName=kp-worker-5"},
			Action: map[string]any{
				"type": "cordon_node", "namespace": "", "resource": "kp-worker-5", "replicas": nil,
				"command":          "kubectl cordon kp-worker-5 && kubectl drain kp-worker-5 --ignore-daemonsets --delete-emptydir-data",
				"explanation":      "Keep new pods off the node while the agent is restarted",
				"requires_cr_code": true,
			},
			Autopilot: autopilotPlan{Action: "cordon_node", Verdict: "escalated", Reason: "node actions are above the low-risk ceiling — paged on-call"},
		}
	}
}

// ---------------------------------------------------------------------------
// Snapshot

type snapshot struct {
	Now       time.Time
	Pods      []pod
	Active    []incident
	History   []incident
	NodeStats map[string]nodeStat
}

type nodeStat struct {
	Ready          bool
	CPUPercent     int
	MemPercent     int
	DiskPressure   bool
	MemoryPressure bool
}

func (s snapshot) incidentFor(ns, workloadName string, index int) *incident {
	for i := range s.Active {
		in := &s.Active[i]
		if in.Namespace == ns && in.Workload == workloadName && in.PodIndex == index {
			return in
		}
	}
	return nil
}

func (s snapshot) nodeIncident(name string) *incident {
	for i := range s.Active {
		if s.Active[i].Kind == "nodenotready" && s.Active[i].Node == name {
			return &s.Active[i]
		}
	}
	return nil
}

// simulate builds the whole cluster for one instant.
func simulate(now time.Time) snapshot {
	now = now.UTC()
	slot := now.Truncate(incidentSlot)
	s := snapshot{Now: now, NodeStats: map[string]nodeStat{}}
	s.Active = []incident{primaryIncident(now), rotatingIncident(slot)}
	for i := 1; i <= 4; i++ {
		prev := rotatingIncident(slot.Add(-time.Duration(i) * incidentSlot))
		prev.Resolved = true
		prev.ResolvedAt = slot.Add(-time.Duration(i-1) * incidentSlot).Add(-time.Duration(hash64(prev.ID)%9+3) * time.Minute)
		s.History = append(s.History, prev)
	}

	for _, n := range nodes {
		cpu := n.baseCPU + 11*wave(now, 47*time.Minute, n.Name+"cpu") + 4*jitter(now, n.Name+"cpu")
		mem := n.baseMem + 5*wave(now, 3*time.Hour, n.Name+"mem") + 1.5*jitter(now, n.Name+"mem")
		st := nodeStat{Ready: true}
		if in := s.nodeIncident(n.Name); in != nil {
			st.Ready = false
			cpu, mem = 0, 0
		}
		for _, in := range s.Active {
			if in.Kind == "diskpressure" && in.Node == n.Name {
				st.DiskPressure = true
			}
		}
		st.CPUPercent = int(math.Round(math.Max(2, math.Min(97, cpu))))
		st.MemPercent = int(math.Round(math.Max(5, math.Min(97, mem))))
		if !st.Ready {
			st.CPUPercent, st.MemPercent = 0, 0
		}
		s.NodeStats[n.Name] = st
	}

	workers := workerNames()
	for _, w := range workloads {
		switch w.Kind {
		case "DaemonSet":
			start := periodStart(now, daemonPeriod, w.Namespace+w.Name)
			for i, n := range nodes {
				p := pod{
					Name: w.Name + "-" + k8sSuffix(5, w.Name, n.Name, start.String()), Namespace: w.Namespace,
					Node: n.Name, Workload: w, Index: i, Image: w.Image, Phase: "Running", Ready: true,
					Started: start.Add(time.Duration(i) * 7 * time.Second),
				}
				s.Pods = append(s.Pods, p)
			}
		case "StatefulSet":
			start := periodStart(now, statefulPeriod, w.Namespace+w.Name)
			for i := 0; i < w.Replicas; i++ {
				p := pod{
					Name: fmt.Sprintf("%s-%d", w.Name, i), Namespace: w.Namespace, Workload: w, Index: i,
					Image: w.Image, Phase: "Running", Ready: true,
					Node:    workers[int(hash64(w.Name, fmt.Sprint(i))%uint64(len(workers)))],
					Started: start.Add(time.Duration(i) * 95 * time.Second),
				}
				if w.Name == "postgres" && i == 1 {
					p.Node = "kp-worker-4"
				}
				s.Pods = append(s.Pods, p)
			}
		default:
			start := periodStart(now, rolloutPeriod, w.Namespace+w.Name)
			rs := w.Name + "-" + k8sSuffix(10, w.Name, w.Image, start.String())
			for i := 0; i < w.Replicas; i++ {
				candidates := workers
				if w.OnControl {
					candidates = append([]string{"kp-cp-1", "kp-cp-2", "kp-cp-3"}, workers...)
				}
				p := pod{
					Name: rs + "-" + k8sSuffix(5, rs, fmt.Sprint(i)), Namespace: w.Namespace, Workload: w, Index: i,
					Image: w.Image, Phase: "Running", Ready: true, ReplicaSet: rs,
					Node:    candidates[int(hash64(rs, fmt.Sprint(i))%uint64(len(candidates)))],
					Started: start.Add(time.Duration(i)*23*time.Second + time.Duration(hash64(rs, "s", fmt.Sprint(i))%40)*time.Second),
				}
				s.Pods = append(s.Pods, p)
			}
		}
	}

	// Background restarts: a few long-lived pods have restarted once or twice.
	for i := range s.Pods {
		p := &s.Pods[i]
		p.IP = fmt.Sprintf("10.42.%d.%d", hash64(p.Node)%8+1, hash64(p.Name)%250+3)
		if hash64(p.Name, "restarts")%9 == 0 && now.Sub(p.Started) > 48*time.Hour {
			p.Restarts = int(hash64(p.Name, "count")%2) + 1
		}
	}

	// Apply incidents.
	for i := range s.Pods {
		p := &s.Pods[i]
		if in := s.nodeIncident(p.Node); in != nil {
			p.Phase, p.Reason, p.Ready, p.Incident = "Unknown", "NodeLost", false, in
			p.StateMessage = "Node " + p.Node + " which was running pod " + p.Name + " is unresponsive"
			continue
		}
		in := s.incidentFor(p.Namespace, p.Workload.Name, p.Index)
		if in == nil {
			continue
		}
		p.Incident = in
		since := now.Sub(in.Started)
		switch in.Kind {
		case "crashloop":
			p.Started = in.Started
			p.Phase, p.Reason, p.Ready = "Running", "CrashLoopBackOff", false
			p.Restarts = crashRestarts(since)
			p.ExitCode = 1
			p.StateMessage = "back-off 5m0s restarting failed container=checkout pod=" + p.Name
		case "oom":
			p.Started = in.Started.Add(-3 * time.Hour)
			p.Phase, p.Reason, p.Ready = "Running", "OOMKilled", int(since.Minutes())%6 >= 2
			if !p.Ready {
				p.Reason = "CrashLoopBackOff"
			}
			p.Restarts = 2 + int(since.Minutes()/6)
			p.ExitCode = 137
			p.StateMessage = "container search-indexer was OOMKilled (limit 512Mi)"
		case "imagepull":
			p.Started = in.Started
			p.Image = strings.Replace(p.Image, "2.8.3", "2.9.0", 1)
			p.ReplicaSet = p.Workload.Name + "-" + k8sSuffix(10, p.Workload.Name, p.Image, in.Started.String())
			p.Name = p.ReplicaSet + "-" + k8sSuffix(5, p.ReplicaSet, "0")
			p.Phase, p.Reason, p.Ready = "Pending", "ImagePullBackOff", false
			p.StateMessage = `Back-off pulling image "ghcr.io/acme-shop/web-frontend:2.9.0"`
		case "diskpressure":
			p.Started = in.Started.Add(2 * time.Minute)
			p.Node = ""
			p.IP = ""
			p.Phase, p.Reason, p.Ready = "Pending", "Unschedulable", false
			p.StateMessage = "0/8 nodes are available: 1 node(s) had untolerated taint {node.kubernetes.io/disk-pressure: }, 7 node(s) didn't match PersistentVolume's node affinity"
		}
	}
	sort.SliceStable(s.Pods, func(i, j int) bool {
		if s.Pods[i].Namespace != s.Pods[j].Namespace {
			return s.Pods[i].Namespace < s.Pods[j].Namespace
		}
		return s.Pods[i].Name < s.Pods[j].Name
	})
	return s
}

// crashRestarts follows kubelet's exponential back-off (10s doubling, capped at 5m).
func crashRestarts(since time.Duration) int {
	n, t, delay := 0, time.Duration(0), 10*time.Second
	for t+delay <= since {
		t += delay
		n++
		if delay < 5*time.Minute {
			delay *= 2
			if delay > 5*time.Minute {
				delay = 5 * time.Minute
			}
		}
	}
	return n
}

func (s snapshot) findPod(ns, name string) (pod, bool) {
	for _, p := range s.Pods {
		if p.Namespace == ns && p.Name == name {
			return p, true
		}
	}
	// Accept a bare workload name (or an old fixture name) and map it to the
	// workload's first pod, so stale links from the app still resolve.
	for _, p := range s.Pods {
		if p.Namespace == ns && strings.HasPrefix(name, p.Workload.Name) {
			return p, true
		}
	}
	return pod{}, false
}

func (s snapshot) workloadReady(w workload) (desired, ready int) {
	for _, p := range s.Pods {
		if p.Namespace == w.Namespace && p.Workload.Name == w.Name {
			desired++
			if p.Ready {
				ready++
			}
		}
	}
	if w.Kind != "DaemonSet" {
		desired = w.Replicas
	}
	return desired, ready
}
