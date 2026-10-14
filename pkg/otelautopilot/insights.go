package otelautopilot

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/k8s"
)

// Analyzer is the slice of the AI engine insights need. Keeping it an
// interface lets tests run without a model and keeps prompts in this package.
type Analyzer interface {
	Complete(ctx context.Context, task, system, user string) (string, error)
	ModelName() string
}

// Insight kinds.
const (
	InsightErrorSpike = "error_spike"
	InsightLatency    = "latency_regression"
	InsightNewError   = "new_error"
)

// Insight statuses.
const (
	InsightOpen     = "open"
	InsightResolved = "resolved"
)

// Detection windows. Current traffic is compared against the half hour
// before it; an insight resolves after a quiet period so a flapping
// dependency does not open a new incident every tick.
const (
	insightWindow       = 5 * time.Minute
	insightBaseline     = 30 * time.Minute
	insightResolveAfter = 10 * time.Minute
	insightMinRequests  = 10
	maxInsights         = 50
)

// InsightSignal is one before/after comparison that triggered an insight.
type InsightSignal struct {
	Label    string `json:"label"`
	Current  string `json:"current"`
	Baseline string `json:"baseline"`
}

// InsightAnalysis is the AI's explanation of an insight.
type InsightAnalysis struct {
	Status      string    `json:"status"` // pending | running | done | failed | unavailable
	Summary     string    `json:"summary,omitempty"`
	RootCause   string    `json:"root_cause,omitempty"`
	Impact      string    `json:"impact,omitempty"`
	Confidence  float64   `json:"confidence,omitempty"`
	NextSteps   []string  `json:"next_steps,omitempty"`
	Commands    []string  `json:"commands,omitempty"`
	Model       string    `json:"model,omitempty"`
	Error       string    `json:"error,omitempty"`
	GeneratedAt time.Time `json:"generated_at,omitempty"`
	Attempts    int       `json:"attempts,omitempty"`
}

// The local model is shared with pod RCA and can be busy for minutes during
// an incident storm, so a failed analysis is retried a few times.
const (
	maxAIAttempts  = 3
	aiRetryBackoff = time.Minute
)

// Insight is a proactively detected problem, ranked to a likely root cause.
type Insight struct {
	ID         string          `json:"id"`
	Key        string          `json:"key"`
	Kind       string          `json:"kind"`
	Status     string          `json:"status"`
	Severity   string          `json:"severity"` // critical | high | medium
	Title      string          `json:"title"`
	Summary    string          `json:"summary"`
	NodeID     string          `json:"node_id"`
	Service    string          `json:"service"`
	Namespace  string          `json:"namespace,omitempty"`
	Workload   string          `json:"workload,omitempty"`
	RootCause  string          `json:"root_cause_node,omitempty"`
	Caller     string          `json:"caller,omitempty"` // instrumented service calling an inferred dependency
	CausePath  []string        `json:"cause_path,omitempty"`
	Affected   []string        `json:"affected,omitempty"`
	Signals    []InsightSignal `json:"signals"`
	Errors     []string        `json:"errors,omitempty"`
	TraceIDs   []string        `json:"trace_ids,omitempty"`
	Pods       []GraphPod      `json:"pods,omitempty"`
	FirstSeen  time.Time       `json:"first_seen"`
	LastSeen   time.Time       `json:"last_seen"`
	ResolvedAt *time.Time      `json:"resolved_at,omitempty"`
	AI         InsightAnalysis `json:"ai"`
}

// InsightHook is notified when an insight opens (and again when its AI
// analysis lands) so it can reach Slack and the anomaly timeline.
type InsightHook func(ctx context.Context, in Insight, analysed bool)

type insightEngine struct {
	mu       sync.Mutex
	store    *Store
	analyzer Analyzer
	k8s      *k8s.Client
	hook     InsightHook
	log      *zap.Logger
	insights map[string]*Insight // by key
	sem      chan struct{}
}

func newInsightEngine(store *Store, log *zap.Logger) *insightEngine {
	return &insightEngine{store: store, log: log, insights: map[string]*Insight{}, sem: make(chan struct{}, 1)}
}

// candidate is a node or edge that crossed a detection threshold.
type candidate struct {
	kind     string
	nodeID   string
	severity string
	title    string
	signals  []InsightSignal
	score    float64
}

// displayName disambiguates inferred dependencies: a Postgres database
// named "payments" must not read like the payments service.
func displayName(n GraphNode) string {
	switch n.Kind {
	case NodeDatabase, NodeMessaging:
		return fmt.Sprintf("%s (%s)", n.Name, firstNonEmpty(n.System, n.Kind))
	case NodeExternal:
		return n.Name + " (external)"
	}
	return n.Name
}

// detect compares the current window against the baseline and returns one
// candidate per (node, kind).
func detect(g ServiceGraph, fresh map[string][]string) []candidate {
	var out []candidate
	for _, n := range g.Nodes {
		if n.Kind == NodeClients {
			continue
		}
		s, b := n.Stats, n.Baseline
		if s.Requests < insightMinRequests {
			continue
		}
		baseRate := 0.0
		if b != nil {
			baseRate = b.ErrorRate
		}
		if s.Errors >= 5 && s.ErrorRate >= 0.05 && (b == nil || b.Requests < insightMinRequests || s.ErrorRate >= 3*baseRate) {
			sev := "high"
			if s.ErrorRate >= 0.25 {
				sev = "critical"
			}
			out = append(out, candidate{
				kind: InsightErrorSpike, nodeID: n.ID, severity: sev,
				title: fmt.Sprintf("%s is failing %s of requests", displayName(n), pct(s.ErrorRate)),
				signals: []InsightSignal{
					{Label: "Error rate", Current: pct(s.ErrorRate), Baseline: baselineStr(b, func(r REDStats) string { return pct(r.ErrorRate) })},
					{Label: "Errors (5m)", Current: fmt.Sprint(s.Errors), Baseline: baselineStr(b, func(r REDStats) string { return fmt.Sprint(r.Errors) })},
				},
				score: s.ErrorRate*100 + float64(s.Errors)/10,
			})
		}
		if b != nil && b.Requests >= 20 && s.P95Ms >= 250 && s.P95Ms >= 2*b.P95Ms {
			sev := "medium"
			if s.P95Ms >= 4*b.P95Ms {
				sev = "high"
			}
			out = append(out, candidate{
				kind: InsightLatency, nodeID: n.ID, severity: sev,
				title: fmt.Sprintf("%s p95 latency is %.1f× slower", displayName(n), s.P95Ms/maxf(b.P95Ms, 1)),
				signals: []InsightSignal{
					{Label: "p95 latency", Current: ms(s.P95Ms), Baseline: ms(b.P95Ms)},
					{Label: "p50 latency", Current: ms(s.P50Ms), Baseline: ms(b.P50Ms)},
				},
				score: s.P95Ms / maxf(b.P95Ms, 1),
			})
		}
		if sigs := fresh[n.ID]; len(sigs) > 0 && b != nil && b.Requests >= insightMinRequests {
			out = append(out, candidate{
				kind: InsightNewError, nodeID: n.ID, severity: "medium",
				title:   fmt.Sprintf("New error in %s: %s", displayName(n), truncate(sigs[0], 80)),
				signals: []InsightSignal{{Label: "New error signatures", Current: fmt.Sprint(len(sigs)), Baseline: "0"}},
				score:   float64(len(sigs)),
			})
		}
	}
	return out
}

// freshSignatures finds error signatures with ≥3 occurrences now that never
// appeared in the baseline, per node.
func freshSignatures(cur, base []SpanRecord) map[string][]string {
	seen := map[string]bool{}
	for _, sp := range base {
		if sp.Status == "ERROR" {
			seen[spanNodeID(sp)+"|"+errorSignature(sp)] = true
		}
	}
	counts := map[string]map[string]int{}
	for _, sp := range cur {
		if sp.Status != "ERROR" {
			continue
		}
		id, sig := spanNodeID(sp), errorSignature(sp)
		if seen[id+"|"+sig] {
			continue
		}
		if counts[id] == nil {
			counts[id] = map[string]int{}
		}
		counts[id][sig]++
	}
	out := map[string][]string{}
	for id, m := range counts {
		for sig, n := range m {
			if n >= 3 {
				out[id] = append(out[id], sig)
			}
		}
		sort.Strings(out[id])
	}
	return out
}

// rootCause walks downstream from an unhealthy node along unhealthy edges
// and returns the deepest failing dependency plus the path to it. The
// deepest failing hop is where errors originate; everything above it is
// collateral.
func rootCause(g ServiceGraph, start string) (string, []string) {
	down := map[string][]GraphEdge{}
	for _, e := range g.Edges {
		down[e.Source] = append(down[e.Source], e)
	}
	path := []string{start}
	visited := map[string]bool{start: true}
	cur := start
	for {
		var next *GraphEdge
		for i, e := range down[cur] {
			if e.Health == HealthHealthy || e.Health == HealthUnknown || visited[e.Target] {
				continue
			}
			if next == nil || e.Stats.ErrorRate > next.Stats.ErrorRate ||
				(e.Stats.ErrorRate == next.Stats.ErrorRate && e.Stats.P95Ms > next.Stats.P95Ms) {
				next = &down[cur][i]
			}
		}
		if next == nil {
			return cur, path
		}
		visited[next.Target] = true
		path = append(path, next.Target)
		cur = next.Target
	}
}

// blastRadius lists every caller upstream of id.
func blastRadius(g ServiceGraph, id string) []string {
	up := map[string][]string{}
	for _, e := range g.Edges {
		up[e.Target] = append(up[e.Target], e.Source)
	}
	seen := map[string]bool{id: true}
	queue := []string{id}
	var out []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, src := range up[cur] {
			if !seen[src] {
				seen[src] = true
				if src != "clients" {
					out = append(out, src)
				}
				queue = append(queue, src)
			}
		}
	}
	sort.Strings(out)
	return out
}

// tick runs one detection pass. It is cheap enough to run every 30s.
func (e *insightEngine) tick(ctx context.Context) {
	now := time.Now().UTC()
	g := BuildServiceGraph(e.store, now, insightWindow, insightBaseline, "")
	if g.SpanCount == 0 {
		e.resolveStale(ctx, now, map[string]bool{})
		return
	}
	nodes := make(map[string]GraphNode, len(g.Nodes))
	for _, n := range g.Nodes {
		nodes[n.ID] = n
	}

	spans := e.store.SpansSince(now.Add(-insightWindow - insightBaseline))
	var cur, base []SpanRecord
	for _, sp := range spans {
		if sp.Timestamp.Before(now.Add(-insightWindow)) {
			base = append(base, sp)
		} else {
			cur = append(cur, sp)
		}
	}
	cands := detect(g, freshSignatures(cur, base))

	// Collapse candidates that share a root cause into one incident: a
	// failing database should be one card, not one per caller.
	type group struct {
		root  string
		path  []string
		cands []candidate
	}
	groups := map[string]*group{}
	// Every signal is grouped by the failure's ranked origin: a database
	// outage is one incident, however many callers report errors, slow
	// requests or new error messages because of it.
	for _, c := range cands {
		root, path := rootCause(g, c.nodeID)
		key := root + "|incident"
		gr := groups[key]
		if gr == nil {
			gr = &group{root: root, path: path}
			groups[key] = gr
		}
		gr.cands = append(gr.cands, c)
		if len(path) > len(gr.path) {
			gr.path = path
		}
	}

	// A symptom too weak to trace downstream on its own (say, 0.9% errors at
	// the frontend) still belongs to the incident whose origin it calls.
	for key, gr := range groups {
		for otherKey, other := range groups {
			if otherKey == key || groups[key] == nil {
				continue
			}
			if contains(blastRadius(g, other.root), gr.root) {
				other.cands = append(other.cands, gr.cands...)
				delete(groups, key)
				break
			}
		}
	}

	active := map[string]bool{}
	var opened []Insight
	for key, gr := range groups {
		sort.Slice(gr.cands, func(i, j int) bool {
			if kindRank(gr.cands[i].kind) != kindRank(gr.cands[j].kind) {
				return kindRank(gr.cands[i].kind) > kindRank(gr.cands[j].kind)
			}
			return gr.cands[i].score > gr.cands[j].score
		})
		top := gr.cands[0]
		// Describe the incident from the root cause when it has its own
		// signal (strongest kind first), otherwise from the top symptom.
		focus := top.nodeID
		for _, c := range gr.cands {
			if c.nodeID == gr.root {
				focus, top = c.nodeID, c
				break
			}
		}
		node := nodes[focus]
		rootNode := nodes[gr.root]
		active[key] = true

		e.mu.Lock()
		in := e.insights[key]
		isNew := in == nil || in.Status == InsightResolved
		if isNew {
			in = &Insight{
				ID: shortID(key + now.String()), Key: key, Kind: top.kind, Status: InsightOpen,
				FirstSeen: now, AI: InsightAnalysis{Status: "pending"},
			}
			e.insights[key] = in
		}
		in.LastSeen = now
		in.Severity = maxSeverity(gr.cands)
		in.Title = top.title
		in.NodeID, in.Service, in.Namespace, in.Workload = node.ID, displayName(node), node.Namespace, node.Workload
		in.Caller = ""
		if node.Kind != NodeService {
			// An inferred dependency has no namespace or pods of its own;
			// borrow its nearest instrumented caller's so commands and pod
			// links point somewhere real.
			if caller, ok := nearestCaller(g, gr.path, node.ID, nodes); ok {
				in.Caller = caller.ID
				in.Namespace, in.Workload = caller.Namespace, caller.Workload
				node.Pods = caller.Pods
			}
		}
		in.RootCause = gr.root
		in.CausePath = gr.path
		in.Affected = blastRadius(g, gr.root)
		in.Kind = top.kind
		in.Signals = mergeSignals(gr.cands)
		in.Errors = mergeErrors(rootNode.TopErrors, node.TopErrors)
		in.TraceIDs = sampleTraces(cur, gr.root, focus, 5)
		in.Pods = node.Pods
		in.Summary = heuristicSummary(*in, nodes)
		if isNew {
			opened = append(opened, *in)
		}
		e.mu.Unlock()
	}
	e.resolveStale(ctx, now, active)
	e.prune()

	var retry []string
	e.mu.Lock()
	for key, in := range e.insights {
		if in.Status == InsightOpen && in.AI.Status == "failed" && in.AI.Attempts < maxAIAttempts && now.Sub(in.AI.GeneratedAt) >= aiRetryBackoff {
			retry = append(retry, key)
		}
	}
	e.mu.Unlock()
	for _, key := range retry {
		e.analyseAsync(key)
	}

	for _, in := range opened {
		e.log.Info("OTLP insight opened", zap.String("title", in.Title), zap.String("severity", in.Severity), zap.String("root", in.RootCause))
		if e.hook != nil {
			e.hook(ctx, in, false)
		}
		e.analyseAsync(in.Key)
	}
}

func (e *insightEngine) resolveStale(_ context.Context, now time.Time, active map[string]bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for key, in := range e.insights {
		if in.Status == InsightOpen && !active[key] && now.Sub(in.LastSeen) >= insightResolveAfter {
			t := now
			in.Status, in.ResolvedAt = InsightResolved, &t
		}
	}
}

func (e *insightEngine) prune() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.insights) <= maxInsights {
		return
	}
	all := make([]*Insight, 0, len(e.insights))
	for _, in := range e.insights {
		all = append(all, in)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].LastSeen.After(all[j].LastSeen) })
	for _, in := range all[maxInsights:] {
		if in.Status == InsightResolved {
			delete(e.insights, in.Key)
		}
	}
}

func (e *insightEngine) list() []Insight {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]Insight, 0, len(e.insights))
	for _, in := range e.insights {
		out = append(out, *in)
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i].Status == InsightOpen) != (out[j].Status == InsightOpen) {
			return out[i].Status == InsightOpen
		}
		if severityRank(out[i].Severity) != severityRank(out[j].Severity) {
			return severityRank(out[i].Severity) > severityRank(out[j].Severity)
		}
		return out[i].LastSeen.After(out[j].LastSeen)
	})
	return out
}

func (e *insightEngine) byID(id string) *Insight {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, in := range e.insights {
		if in.ID == id {
			cp := *in
			return &cp
		}
	}
	return nil
}

// analyseAsync runs AI analysis in the background, one at a time so a burst
// of insights cannot queue a pile of slow local-LLM calls.
func (e *insightEngine) analyseAsync(key string) {
	e.mu.Lock()
	in := e.insights[key]
	if in == nil {
		e.mu.Unlock()
		return
	}
	if e.analyzer == nil {
		in.AI = InsightAnalysis{Status: "unavailable", Error: "AI engine not configured"}
		e.mu.Unlock()
		return
	}
	in.AI.Status = "pending"
	e.mu.Unlock()

	go func() {
		e.sem <- struct{}{}
		defer func() { <-e.sem }()

		e.mu.Lock()
		in := e.insights[key]
		if in == nil {
			e.mu.Unlock()
			return
		}
		in.AI.Status = "running"
		in.AI.Attempts++
		attempts := in.AI.Attempts
		snapshot := *in
		e.mu.Unlock()

		// Generous: this runs in the background and a shared local model can be
		// queued behind several pod RCAs during an incident storm.
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
		defer cancel()
		e.enrichPods(ctx, &snapshot)
		analysis := e.runAnalysis(ctx, snapshot)
		analysis.Attempts = attempts

		e.mu.Lock()
		if cur := e.insights[key]; cur != nil {
			cur.AI = analysis
			cur.Pods = snapshot.Pods
			snapshot = *cur
		}
		e.mu.Unlock()
		if e.hook != nil && analysis.Status == "done" {
			e.hook(context.Background(), snapshot, true)
		}
	}()
}

// enrichPods joins the span-reported pods with live cluster state, so the
// AI (and the operator) can see whether the failing service is also
// restarting or unready.
func (e *insightEngine) enrichPods(ctx context.Context, in *Insight) {
	if e.k8s == nil || in.Namespace == "" || len(in.Pods) == 0 {
		return
	}
	pods, err := e.k8s.ListPods(ctx, in.Namespace)
	if err != nil {
		return
	}
	live := make(map[string]k8s.PodSummary, len(pods))
	for _, p := range pods {
		live[p.Name] = p
	}
	for i := range in.Pods {
		if p, ok := live[in.Pods[i].Name]; ok {
			in.Pods[i] = GraphPod{Name: p.Name, Node: p.NodeName, Phase: string(p.Phase), Reason: p.Reason, Ready: p.Ready, Restarts: p.Restarts, Known: true}
		}
	}
}

const insightSystemPrompt = `You are KubePilot, an SRE assistant. You are given an incident that KubePilot detected automatically from OpenTelemetry traces on a Kubernetes cluster, before any operator asked.
Explain it for an on-call engineer. Be specific: name services, dependencies and error messages from the evidence. Do not invent facts that are not in the evidence.
Reply with ONLY a JSON object:
{"summary":"one or two sentences","root_cause":"most likely cause","impact":"who/what is affected","confidence":0.0-1.0,"next_steps":["short imperative steps"],"commands":["kubectl or curl commands to verify or fix"]}`

func (e *insightEngine) runAnalysis(ctx context.Context, in Insight) InsightAnalysis {
	prompt := e.buildPrompt(in)
	raw, err := e.analyzer.Complete(ctx, "otel-insight", insightSystemPrompt, prompt)
	res := InsightAnalysis{Model: e.analyzer.ModelName(), GeneratedAt: time.Now().UTC()}
	if err != nil {
		res.Status, res.Error = "failed", err.Error()
		return res
	}
	var parsed struct {
		Summary    string   `json:"summary"`
		RootCause  string   `json:"root_cause"`
		Impact     string   `json:"impact"`
		Confidence float64  `json:"confidence"`
		NextSteps  []string `json:"next_steps"`
		Commands   []string `json:"commands"`
	}
	if js := extractJSONObject(raw); js != "" && json.Unmarshal([]byte(js), &parsed) == nil && parsed.Summary != "" {
		res.Status = "done"
		res.Summary, res.RootCause, res.Impact = parsed.Summary, parsed.RootCause, parsed.Impact
		res.Confidence = clamp01(parsed.Confidence)
		res.NextSteps, res.Commands = capList(parsed.NextSteps, 6), capList(parsed.Commands, 5)
		return res
	}
	// Small local models sometimes ignore the JSON instruction; the prose
	// is still useful, so keep it rather than fail.
	res.Status, res.Summary = "done", truncate(raw, 1200)
	return res
}

// buildPrompt keeps the evidence compact: the dashboard model may run with a
// 4k context window.
func (e *insightEngine) buildPrompt(in Insight) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Incident: %s\nSeverity: %s\nDetected: %s (first seen %s)\n", in.Title, in.Severity, in.Kind, in.FirstSeen.Format(time.RFC3339))
	if in.Caller != "" {
		fmt.Fprintf(&sb, "Failing component: %s — an uninstrumented dependency inferred from client spans, NOT a Kubernetes service itself.\n", in.Service)
		fmt.Fprintf(&sb, "It is called by service %s (namespace %s, deployment %s).\n", in.Caller, dash(in.Namespace), dash(in.Workload))
	} else {
		fmt.Fprintf(&sb, "Service: %s (namespace %s, deployment %s)\n", in.Service, dash(in.Namespace), dash(in.Workload))
	}
	for _, s := range in.Signals {
		fmt.Fprintf(&sb, "- %s: now %s, baseline %s\n", s.Label, s.Current, s.Baseline)
	}
	if len(in.CausePath) > 1 {
		fmt.Fprintf(&sb, "Failing call path (caller → callee): %s\n", strings.Join(in.CausePath, " → "))
	}
	fmt.Fprintf(&sb, "Suspected origin: %s\n", in.RootCause)
	if len(in.Affected) > 0 {
		fmt.Fprintf(&sb, "Upstream callers affected: %s\n", strings.Join(in.Affected, ", "))
	}
	if len(in.Errors) > 0 {
		sb.WriteString("Top error messages:\n")
		for _, msg := range in.Errors {
			fmt.Fprintf(&sb, "- %s\n", msg)
		}
	}
	if len(in.Pods) > 0 {
		sb.WriteString("Pods:\n")
		for i, p := range in.Pods {
			if i == 6 {
				break
			}
			if p.Known {
				fmt.Fprintf(&sb, "- %s phase=%s ready=%t restarts=%d %s\n", p.Name, p.Phase, p.Ready, p.Restarts, p.Reason)
			} else {
				fmt.Fprintf(&sb, "- %s (not found in cluster now)\n", p.Name)
			}
		}
	}
	if len(in.TraceIDs) > 0 {
		if tr := e.store.Trace(in.TraceIDs[0]); len(tr) > 0 {
			sb.WriteString("Sample failing trace (service: span [status] duration):\n")
			for i, sp := range tr {
				if i == 15 {
					break
				}
				line := fmt.Sprintf("- %s: %s [%s] %s", sp.Service, sp.Name, sp.Status, sp.Duration.Round(time.Millisecond))
				if sp.Status == "ERROR" {
					line += " — " + errorSignature(sp)
				}
				sb.WriteString(truncate(line, 220) + "\n")
			}
		}
		logs := e.store.LogsForTraces(map[string]bool{in.TraceIDs[0]: true}, 8)
		if len(logs) > 0 {
			sb.WriteString("Logs from that trace:\n")
			for _, l := range logs {
				fmt.Fprintf(&sb, "- [%s] %s\n", l.Severity, truncate(l.Body, 200))
			}
		}
	}
	return sb.String()
}

// heuristicSummary is the explanation shown immediately, before (or
// without) the AI.
func heuristicSummary(in Insight, nodes map[string]GraphNode) string {
	node := nodes[in.NodeID]
	var sb strings.Builder
	switch in.Kind {
	case InsightErrorSpike:
		fmt.Fprintf(&sb, "%s is returning errors on %s of requests over the last 5 minutes", displayName(node), pct(node.Stats.ErrorRate))
		if node.Baseline != nil {
			fmt.Fprintf(&sb, " (baseline %s)", pct(node.Baseline.ErrorRate))
		}
		sb.WriteString(".")
	case InsightLatency:
		if node.Baseline != nil {
			fmt.Fprintf(&sb, "%s p95 latency rose to %s from %s.", displayName(node), ms(node.Stats.P95Ms), ms(node.Baseline.P95Ms))
		}
	case InsightNewError:
		fmt.Fprintf(&sb, "%s started failing with an error not seen in the previous 30 minutes.", displayName(node))
	}
	if in.RootCause != "" && in.RootCause != in.NodeID {
		root := nodes[in.RootCause]
		name := in.RootCause
		if root.ID != "" {
			name = displayName(root)
		}
		fmt.Fprintf(&sb, " The failures originate downstream in %s.", name)
	}
	if len(in.Errors) > 0 {
		fmt.Fprintf(&sb, " Most common error: %s.", strings.TrimSuffix(in.Errors[0], "."))
	}
	if len(in.Affected) > 0 {
		fmt.Fprintf(&sb, " %d upstream service(s) are affected.", len(in.Affected))
	}
	return strings.TrimSpace(sb.String())
}

// nearestCaller finds the instrumented service that calls id, preferring
// the hop just before it on the failure path.
func nearestCaller(g ServiceGraph, path []string, id string, nodes map[string]GraphNode) (GraphNode, bool) {
	for i := len(path) - 1; i > 0; i-- {
		if path[i] == id {
			if n, ok := nodes[path[i-1]]; ok && n.Kind == NodeService {
				return n, true
			}
		}
	}
	for _, e := range g.Edges {
		if e.Target == id {
			if n, ok := nodes[e.Source]; ok && n.Kind == NodeService {
				return n, true
			}
		}
	}
	return GraphNode{}, false
}

func sampleTraces(spans []SpanRecord, root, focus string, n int) []string {
	seen := map[string]bool{}
	var out []string
	pick := func(match func(SpanRecord) bool) {
		for i := len(spans) - 1; i >= 0 && len(out) < n; i-- {
			sp := spans[i]
			if sp.Status == "ERROR" && !seen[sp.TraceID] && match(sp) {
				seen[sp.TraceID] = true
				out = append(out, sp.TraceID)
			}
		}
	}
	pick(func(sp SpanRecord) bool {
		vid, _ := virtualNode(sp)
		return spanNodeID(sp) == root || vid == root
	})
	pick(func(sp SpanRecord) bool { return spanNodeID(sp) == focus })
	if len(out) == 0 {
		// Latency incidents may have no errors: fall back to the slowest spans.
		var slow []SpanRecord
		for _, sp := range spans {
			if spanNodeID(sp) == focus {
				slow = append(slow, sp)
			}
		}
		sort.Slice(slow, func(i, j int) bool { return slow[i].Duration > slow[j].Duration })
		for _, sp := range slow {
			if len(out) >= n {
				break
			}
			if !seen[sp.TraceID] {
				seen[sp.TraceID] = true
				out = append(out, sp.TraceID)
			}
		}
	}
	return out
}

// mergeSignals keeps the strongest candidate's signals first, then adds
// any other distinct signal (e.g. latency alongside an error spike).
func mergeSignals(cs []candidate) []InsightSignal {
	seen := map[string]bool{}
	out := []InsightSignal{}
	for _, c := range cs {
		for _, sig := range c.signals {
			if !seen[sig.Label] && len(out) < 4 {
				seen[sig.Label] = true
				out = append(out, sig)
			}
		}
	}
	return out
}

func mergeErrors(lists ...[]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, l := range lists {
		for _, e := range l {
			if !seen[e] && len(out) < 4 {
				seen[e] = true
				out = append(out, e)
			}
		}
	}
	return out
}

func maxSeverity(cs []candidate) string {
	best := "medium"
	for _, c := range cs {
		if severityRank(c.severity) > severityRank(best) {
			best = c.severity
		}
	}
	return best
}

// kindRank orders what an incident is "about": failing requests matter
// more than slow ones, which matter more than a new message alone.
func kindRank(k string) int {
	switch k {
	case InsightErrorSpike:
		return 3
	case InsightLatency:
		return 2
	}
	return 1
}

func severityRank(s string) int {
	switch s {
	case "critical":
		return 3
	case "high":
		return 2
	case "medium":
		return 1
	}
	return 0
}

func extractJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return ""
	}
	return raw[start : end+1]
}

func baselineStr(b *REDStats, f func(REDStats) string) string {
	if b == nil || b.Requests == 0 {
		return "no baseline"
	}
	return f(*b)
}

func pct(v float64) string {
	if v > 0 && v < 0.001 {
		return "<0.1%"
	}
	return fmt.Sprintf("%.1f%%", v*100)
}

func ms(v float64) string {
	if v >= 1000 {
		return fmt.Sprintf("%.2fs", v/1000)
	}
	return fmt.Sprintf("%.0fms", v)
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func clamp01(v float64) float64 {
	if v > 1 && v <= 100 {
		v /= 100
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func capList(in []string, n int) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if strings.TrimSpace(s) != "" && len(out) < n {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func shortID(s string) string {
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:6])
}
