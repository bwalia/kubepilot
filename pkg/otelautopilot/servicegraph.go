package otelautopilot

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Node kinds in the trace-derived service graph. Only "service" nodes emit
// spans themselves; the rest are inferred from client spans (or, for
// "clients", from root server spans) so uninstrumented dependencies and
// callers still show up on the map.
const (
	NodeService   = "service"
	NodeDatabase  = "database"
	NodeMessaging = "messaging"
	NodeExternal  = "external"
	NodeClients   = "clients"
)

// Health levels shared by nodes, edges and insights.
const (
	HealthHealthy  = "healthy"
	HealthDegraded = "degraded"
	HealthCritical = "critical"
	HealthUnknown  = "unknown"
)

// REDStats is request rate, errors and duration for a node or edge.
type REDStats struct {
	Requests   int     `json:"requests"`
	Errors     int     `json:"errors"`
	ErrorRate  float64 `json:"error_rate"`
	RatePerMin float64 `json:"rate_per_min"`
	P50Ms      float64 `json:"p50_ms"`
	P95Ms      float64 `json:"p95_ms"`
	P99Ms      float64 `json:"p99_ms"`
}

// OperationStat is RED for one span name on a service.
type OperationStat struct {
	Name string `json:"name"`
	REDStats
}

// GraphPod is a pod seen emitting spans for a node, joined with live state.
type GraphPod struct {
	Name     string `json:"name"`
	Node     string `json:"node,omitempty"`
	Phase    string `json:"phase,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Ready    bool   `json:"ready"`
	Restarts int32  `json:"restarts"`
	Known    bool   `json:"known"` // false when the pod is not (or no longer) in the cluster
}

// GraphNode is a service, workload or inferred dependency on the map.
type GraphNode struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Kind       string          `json:"kind"`
	Namespace  string          `json:"namespace,omitempty"`
	Workload   string          `json:"workload,omitempty"`
	Language   string          `json:"language,omitempty"`
	Version    string          `json:"version,omitempty"`
	System     string          `json:"system,omitempty"` // db.system / messaging.system
	Pods       []GraphPod      `json:"pods,omitempty"`
	Stats      REDStats        `json:"stats"`
	Baseline   *REDStats       `json:"baseline,omitempty"`
	Health     string          `json:"health"`
	Operations []OperationStat `json:"operations,omitempty"`
	TopErrors  []string        `json:"top_errors,omitempty"`
	Insight    string          `json:"insight_id,omitempty"`
}

// GraphEdge is caller → callee traffic.
type GraphEdge struct {
	ID       string    `json:"id"`
	Source   string    `json:"source"`
	Target   string    `json:"target"`
	Protocol string    `json:"protocol,omitempty"`
	Stats    REDStats  `json:"stats"`
	Baseline *REDStats `json:"baseline,omitempty"`
	Health   string    `json:"health"`
}

// ServiceGraph is the topology derived from spans in a time window.
type ServiceGraph struct {
	GeneratedAt time.Time   `json:"generated_at"`
	From        time.Time   `json:"from"`
	To          time.Time   `json:"to"`
	Window      string      `json:"window"`
	SpanCount   int         `json:"span_count"`
	TraceCount  int         `json:"trace_count"`
	Nodes       []GraphNode `json:"nodes"`
	Edges       []GraphEdge `json:"edges"`
}

type agg struct {
	requests, errors int
	durations        []float64
}

func (a *agg) add(sp SpanRecord) {
	a.requests++
	if sp.Status == "ERROR" {
		a.errors++
	}
	a.durations = append(a.durations, float64(sp.Duration)/float64(time.Millisecond))
}

func (a *agg) stats(window time.Duration) REDStats {
	if a == nil || a.requests == 0 {
		return REDStats{}
	}
	sort.Float64s(a.durations)
	mins := window.Minutes()
	if mins <= 0 {
		mins = 1
	}
	return REDStats{
		Requests:   a.requests,
		Errors:     a.errors,
		ErrorRate:  float64(a.errors) / float64(a.requests),
		RatePerMin: float64(a.requests) / mins,
		P50Ms:      percentile(a.durations, 0.50),
		P95Ms:      percentile(a.durations, 0.95),
		P99Ms:      percentile(a.durations, 0.99),
	}
}

func percentile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * q)
	return sorted[idx]
}

type nodeMeta struct {
	GraphNode
	pods map[string]bool
	ops  map[string]*agg
	errs map[string]int
}

// graphAgg is the raw aggregation of one window, before stats are computed.
type graphAgg struct {
	nodes  map[string]*nodeMeta
	nodeA  map[string]*agg
	edges  map[string]*GraphEdge
	edgeA  map[string]*agg
	traces map[string]bool
	spans  int
}

// spanNodeID keys a span's service by namespace so equal service names in
// different namespaces stay separate nodes.
func spanNodeID(sp SpanRecord) string {
	svc := sp.Service
	if svc == "" {
		svc = "unknown"
	}
	if ns := sp.Labels["k8s.namespace.name"]; ns != "" {
		return ns + "/" + svc
	}
	return svc
}

func aggregate(spans []SpanRecord, namespace string) *graphAgg {
	g := &graphAgg{
		nodes: map[string]*nodeMeta{}, nodeA: map[string]*agg{},
		edges: map[string]*GraphEdge{}, edgeA: map[string]*agg{},
		traces: map[string]bool{},
	}

	byID := make(map[string]*SpanRecord, len(spans))
	hasRemoteChild := make(map[string]bool, len(spans))
	for i := range spans {
		byID[spans[i].TraceID+"/"+spans[i].SpanID] = &spans[i]
	}
	for i := range spans {
		sp := &spans[i]
		if sp.ParentID == "" {
			continue
		}
		if parent := byID[sp.TraceID+"/"+sp.ParentID]; parent != nil && spanNodeID(*parent) != spanNodeID(*sp) {
			hasRemoteChild[parent.TraceID+"/"+parent.SpanID] = true
		}
	}

	ensureNode := func(id string, sp *SpanRecord) *nodeMeta {
		n := g.nodes[id]
		if n == nil {
			n = &nodeMeta{
				GraphNode: GraphNode{ID: id, Kind: NodeService},
				pods:      map[string]bool{}, ops: map[string]*agg{}, errs: map[string]int{},
			}
			g.nodes[id] = n
			g.nodeA[id] = &agg{}
		}
		if sp != nil {
			l := sp.Labels
			n.Name = firstNonEmpty(sp.Service, "unknown")
			n.Namespace = firstNonEmpty(n.Namespace, l["k8s.namespace.name"])
			n.Workload = firstNonEmpty(n.Workload, l["k8s.deployment.name"], l["k8s.statefulset.name"], l["k8s.daemonset.name"], workloadFromPod(l["k8s.pod.name"]))
			n.Language = firstNonEmpty(n.Language, l["telemetry.sdk.language"])
			n.Version = firstNonEmpty(l["service.version"], n.Version)
			if pod := l["k8s.pod.name"]; pod != "" {
				n.pods[pod] = true
			}
		}
		return n
	}
	addEdge := func(src, dst, protocol string, sp SpanRecord) {
		id := src + "→" + dst
		if g.edges[id] == nil {
			g.edges[id] = &GraphEdge{ID: id, Source: src, Target: dst, Protocol: protocol}
			g.edgeA[id] = &agg{}
		}
		g.edgeA[id].add(sp)
	}

	for i := range spans {
		sp := spans[i]
		if namespace != "" && sp.Labels["k8s.namespace.name"] != namespace {
			continue
		}
		g.spans++
		g.traces[sp.TraceID] = true
		id := spanNodeID(sp)
		node := ensureNode(id, &sp)

		var parent *SpanRecord
		if sp.ParentID != "" {
			parent = byID[sp.TraceID+"/"+sp.ParentID]
		}
		remoteParent := parent != nil && spanNodeID(*parent) != id
		entry := sp.Kind == "server" || sp.Kind == "consumer" || parent == nil || remoteParent

		if entry {
			g.nodeA[id].add(sp)
			if node.ops[sp.Name] == nil {
				node.ops[sp.Name] = &agg{}
			}
			node.ops[sp.Name].add(sp)
		}
		if sp.Status == "ERROR" {
			if msg := errorSignature(sp); msg != "" {
				node.errs[msg]++
			}
		}

		switch {
		case remoteParent:
			addEdge(spanNodeID(*parent), id, protocolOf(*parent, sp), sp)
		case parent == nil && sp.ParentID == "" && (sp.Kind == "server" || sp.Kind == "consumer"):
			ensureNode("clients", nil).GraphNode = GraphNode{ID: "clients", Name: "Clients", Kind: NodeClients}
			addEdge("clients", id, protocolOf(sp, sp), sp)
		}

		// A client/producer span that no other service picked up is a call to
		// something uninstrumented — a database, a queue or an external host.
		if (sp.Kind == "client" || sp.Kind == "producer") && !hasRemoteChild[sp.TraceID+"/"+sp.SpanID] {
			if vid, vnode := virtualNode(sp); vid != "" {
				if g.nodes[vid] == nil {
					g.nodes[vid] = &nodeMeta{GraphNode: vnode, pods: map[string]bool{}, ops: map[string]*agg{}, errs: map[string]int{}}
					g.nodeA[vid] = &agg{}
				}
				g.nodeA[vid].add(sp)
				if sp.Status == "ERROR" {
					if msg := errorSignature(sp); msg != "" {
						g.nodes[vid].errs[msg]++
					}
				}
				addEdge(id, vid, protocolOf(sp, sp), sp)
			}
		}
	}
	return g
}

// BuildServiceGraph derives the topology for [to-window, to], comparing each
// node and edge against the preceding baseline window.
func BuildServiceGraph(store *Store, to time.Time, window, baseline time.Duration, namespace string) ServiceGraph {
	from := to.Add(-window)
	all := store.SpansSince(from.Add(-baseline))
	var cur, base []SpanRecord
	for _, sp := range all {
		if sp.Timestamp.Before(from) {
			base = append(base, sp)
		} else if !sp.Timestamp.After(to) {
			cur = append(cur, sp)
		}
	}
	g := aggregate(cur, namespace)
	var b *graphAgg
	if baseline > 0 && len(base) > 0 {
		b = aggregate(base, namespace)
	}

	out := ServiceGraph{
		GeneratedAt: time.Now().UTC(), From: from, To: to, Window: window.String(),
		SpanCount: g.spans, TraceCount: len(g.traces),
		// Empty lists, never null: the dashboard maps over them.
		Nodes: []GraphNode{}, Edges: []GraphEdge{},
	}
	for id, n := range g.nodes {
		node := n.GraphNode
		node.Stats = g.nodeA[id].stats(window)
		if b != nil && b.nodeA[id] != nil && b.nodeA[id].requests > 0 {
			bs := b.nodeA[id].stats(baseline)
			node.Baseline = &bs
		}
		for pod := range n.pods {
			node.Pods = append(node.Pods, GraphPod{Name: pod})
		}
		sort.Slice(node.Pods, func(i, j int) bool { return node.Pods[i].Name < node.Pods[j].Name })
		for name, a := range n.ops {
			node.Operations = append(node.Operations, OperationStat{Name: name, REDStats: a.stats(window)})
		}
		sort.Slice(node.Operations, func(i, j int) bool {
			oi, oj := node.Operations[i], node.Operations[j]
			if oi.Errors != oj.Errors {
				return oi.Errors > oj.Errors
			}
			return oi.Requests > oj.Requests
		})
		if len(node.Operations) > 8 {
			node.Operations = node.Operations[:8]
		}
		node.TopErrors = topKeys(n.errs, 3)
		node.Health = classify(node.Stats, node.Baseline)
		out.Nodes = append(out.Nodes, node)
	}
	for id, e := range g.edges {
		edge := *e
		edge.Stats = g.edgeA[id].stats(window)
		if b != nil && b.edgeA[id] != nil && b.edgeA[id].requests > 0 {
			bs := b.edgeA[id].stats(baseline)
			edge.Baseline = &bs
		}
		edge.Health = classify(edge.Stats, edge.Baseline)
		out.Edges = append(out.Edges, edge)
	}
	// A namespace filter can leave an edge pointing at a caller outside it;
	// keep that caller on the map as a stub rather than drop the edge.
	seen := make(map[string]bool, len(out.Nodes))
	for _, n := range out.Nodes {
		seen[n.ID] = true
	}
	for _, e := range out.Edges {
		for _, id := range []string{e.Source, e.Target} {
			if !seen[id] {
				seen[id] = true
				name := id
				if i := strings.LastIndex(id, "/"); i >= 0 {
					name = id[i+1:]
				}
				out.Nodes = append(out.Nodes, GraphNode{ID: id, Name: name, Kind: NodeService, Health: HealthUnknown})
			}
		}
	}
	sort.Slice(out.Nodes, func(i, j int) bool { return out.Nodes[i].ID < out.Nodes[j].ID })
	sort.Slice(out.Edges, func(i, j int) bool { return out.Edges[i].ID < out.Edges[j].ID })
	return out
}

// classify turns RED (and its baseline, when known) into a health level.
// Small samples never go critical on their own: one failed request out of
// two is noise, not an incident.
func classify(s REDStats, base *REDStats) string {
	if s.Requests == 0 {
		return HealthUnknown
	}
	switch {
	case s.Errors >= 3 && s.ErrorRate >= 0.05:
		return HealthCritical
	case s.Errors >= 1 && s.ErrorRate >= 0.01:
		return HealthDegraded
	case base != nil && base.Requests >= 20 && s.Requests >= 10 && s.P95Ms >= 250 && s.P95Ms >= 2*base.P95Ms:
		return HealthDegraded
	}
	return HealthHealthy
}

func virtualNode(sp SpanRecord) (string, GraphNode) {
	l := sp.Labels
	host := firstNonEmpty(l["server.address"], l["net.peer.name"], hostOf(l["url.full"]), hostOf(l["http.url"]), l["http.host"])
	if sys := firstNonEmpty(l["db.system"], l["db.system.name"]); sys != "" {
		name := firstNonEmpty(l["peer.service"], l["db.name"], l["db.namespace"], host, sys)
		id := "db:" + sys + ":" + name
		return id, GraphNode{ID: id, Name: name, Kind: NodeDatabase, System: sys}
	}
	if sys := l["messaging.system"]; sys != "" {
		name := firstNonEmpty(l["peer.service"], l["messaging.destination.name"], host, sys)
		id := "mq:" + sys + ":" + name
		return id, GraphNode{ID: id, Name: name, Kind: NodeMessaging, System: sys}
	}
	if name := firstNonEmpty(l["peer.service"], host); name != "" {
		id := "ext:" + name
		return id, GraphNode{ID: id, Name: name, Kind: NodeExternal}
	}
	return "", GraphNode{}
}

func protocolOf(client, server SpanRecord) string {
	for _, l := range []map[string]string{client.Labels, server.Labels} {
		switch {
		case l["db.system"] != "" || l["db.system.name"] != "":
			return firstNonEmpty(l["db.system"], l["db.system.name"])
		case l["messaging.system"] != "":
			return l["messaging.system"]
		case l["rpc.system"] != "":
			return l["rpc.system"]
		case l["http.request.method"] != "" || l["http.method"] != "":
			return "http"
		}
	}
	return ""
}

// errorSignature is a short, de-noised description of why a span failed.
func errorSignature(sp SpanRecord) string {
	l := sp.Labels
	msg := firstNonEmpty(l["exception.message"], sp.StatusMessage)
	kind := firstNonEmpty(l["exception.type"], l["error.type"])
	if code := firstNonEmpty(l["http.response.status_code"], l["http.status_code"]); code != "" && kind == "" {
		kind = "HTTP " + code
	}
	sig := strings.TrimSpace(strings.Join(nonEmpty(kind, normaliseMessage(msg)), ": "))
	if sig == "" {
		sig = sp.Name + " failed"
	}
	return truncate(sig, 160)
}

// digitRuns matches the variable parts of an error message: IDs, IPs,
// ports, durations, counts.
var digitRuns = regexp.MustCompile(`[0-9]+`)

// normaliseMessage replaces every run of digits so the same failure from
// many requests ("timed out after 1692ms", "...1733ms") collapses into one
// signature.
func normaliseMessage(msg string) string {
	return strings.Join(strings.Fields(digitRuns.ReplaceAllString(msg, "<n>")), " ")
}

func workloadFromPod(pod string) string {
	parts := strings.Split(pod, "-")
	switch {
	case len(parts) >= 3:
		return strings.Join(parts[:len(parts)-2], "-")
	case len(parts) == 2:
		return parts[0]
	}
	return ""
}

func hostOf(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func topKeys(m map[string]int, n int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]] != m[keys[j]] {
			return m[keys[i]] > m[keys[j]]
		}
		return keys[i] < keys[j]
	})
	if len(keys) > n {
		keys = keys[:n]
	}
	return keys
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func nonEmpty(vals ...string) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}
