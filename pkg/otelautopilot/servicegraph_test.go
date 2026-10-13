package otelautopilot

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

// span builds a test span; parent "" makes it a root.
func span(trace, id, parent, svc, ns, kind, name, status string, start time.Time, dur time.Duration, extra map[string]string) SpanRecord {
	labels := map[string]string{"k8s.namespace.name": ns, "k8s.pod.name": svc + "-7d9f8b6c4-abcde"}
	for k, v := range extra {
		labels[k] = v
	}
	return SpanRecord{
		TraceID: trace, SpanID: id, ParentID: parent, Service: svc, Kind: kind, Name: name,
		Status: status, Duration: dur, Timestamp: start, Labels: labels, Plane: PlaneApp,
	}
}

// request emits one frontend → checkout → payments → postgres call chain.
func request(i int, at time.Time, dbFails bool, dbLatency time.Duration) []SpanRecord {
	tr := fmt.Sprintf("t%04d", i)
	status := "OK"
	dbExtra := map[string]string{"db.system": "postgresql", "db.name": "payments", "server.address": "pg.shop.svc"}
	if dbFails {
		status = "ERROR"
		dbExtra["exception.message"] = fmt.Sprintf("connection refused to 10.0.0.%d:5432", i%250)
	}
	return []SpanRecord{
		span(tr, "a", "", "frontend", "shop", "server", "GET /checkout", status, at, 120*time.Millisecond, map[string]string{"http.request.method": "GET"}),
		span(tr, "b", "a", "frontend", "shop", "client", "POST checkout", status, at, 100*time.Millisecond, map[string]string{"http.request.method": "POST"}),
		span(tr, "c", "b", "checkout", "shop", "server", "POST /checkout", status, at, 90*time.Millisecond, map[string]string{"http.request.method": "POST"}),
		span(tr, "d", "c", "checkout", "shop", "client", "POST charge", status, at, 80*time.Millisecond, map[string]string{"http.request.method": "POST"}),
		span(tr, "e", "d", "payments", "shop", "server", "POST /charge", status, at, 70*time.Millisecond, map[string]string{"http.request.method": "POST"}),
		span(tr, "f", "e", "payments", "shop", "client", "INSERT charges", status, at, dbLatency, dbExtra),
	}
}

func TestServiceGraphDerivesEdgesAndVirtualNodes(t *testing.T) {
	store := NewStore(time.Hour)
	now := time.Now().UTC()
	for i := 0; i < 20; i++ {
		store.IngestSpans(request(i, now.Add(-time.Duration(i)*time.Second), false, 5*time.Millisecond)...)
	}
	g := BuildServiceGraph(store, now, 5*time.Minute, 0, "")

	edges := map[string]GraphEdge{}
	for _, e := range g.Edges {
		edges[e.ID] = e
	}
	for _, want := range []string{
		"clients→shop/frontend",
		"shop/frontend→shop/checkout",
		"shop/checkout→shop/payments",
		"shop/payments→db:postgresql:payments",
	} {
		e, ok := edges[want]
		if !ok {
			t.Fatalf("missing edge %s; have %v", want, keys(edges))
		}
		if e.Stats.Requests != 20 {
			t.Errorf("%s requests = %d, want 20", want, e.Stats.Requests)
		}
	}
	if len(edges) != 4 {
		t.Errorf("got %d edges, want 4: %v", len(edges), keys(edges))
	}
	for _, n := range g.Nodes {
		if n.ID == "shop/checkout" {
			if n.Stats.Requests != 20 || n.Workload != "checkout" || len(n.Pods) != 1 {
				t.Errorf("checkout node = %+v", n)
			}
		}
		if n.ID == "db:postgresql:payments" && n.Kind != NodeDatabase {
			t.Errorf("db node kind = %s", n.Kind)
		}
	}
}

type fakeAnalyzer struct {
	mu      sync.Mutex
	prompts []string
}

func (f *fakeAnalyzer) Complete(_ context.Context, _, _, user string) (string, error) {
	f.mu.Lock()
	f.prompts = append(f.prompts, user)
	f.mu.Unlock()
	return "Here you go:\n```json\n{\"summary\":\"Postgres refuses connections\",\"root_cause\":\"payments database down\",\"impact\":\"checkout fails\",\"confidence\":0.9,\"next_steps\":[\"check postgres pod\"],\"commands\":[\"kubectl -n shop get pods\"]}\n```", nil
}

func (f *fakeAnalyzer) ModelName() string { return "fake" }

func TestInsightRanksDatabaseAsRootCauseAndRunsAI(t *testing.T) {
	store := NewStore(time.Hour)
	now := time.Now().UTC()
	// Healthy baseline 10–30 minutes ago, then the database starts failing.
	for i := 0; i < 60; i++ {
		store.IngestSpans(request(i, now.Add(-10*time.Minute-time.Duration(i)*15*time.Second), false, 5*time.Millisecond)...)
	}
	for i := 100; i < 140; i++ {
		store.IngestSpans(request(i, now.Add(-time.Duration(i-100)*5*time.Second), i%2 == 0, 5*time.Millisecond)...)
	}

	eng := newInsightEngine(store, zap.NewNop())
	fa := &fakeAnalyzer{}
	eng.analyzer = fa
	var hooked []Insight
	var hookMu sync.Mutex
	eng.hook = func(_ context.Context, in Insight, analysed bool) {
		hookMu.Lock()
		defer hookMu.Unlock()
		if !analysed {
			hooked = append(hooked, in)
		}
	}
	eng.tick(context.Background())

	list := eng.list()
	var spike *Insight
	for i := range list {
		if list[i].Kind == InsightErrorSpike {
			spike = &list[i]
		}
	}
	if spike == nil {
		t.Fatalf("no error-spike insight; got %+v", list)
	}
	if spike.RootCause != "db:postgresql:payments" {
		t.Errorf("root cause = %q, want the postgres node (path %v)", spike.RootCause, spike.CausePath)
	}
	if !contains(spike.Affected, "shop/frontend") || !contains(spike.Affected, "shop/checkout") {
		t.Errorf("blast radius = %v", spike.Affected)
	}
	if spike.Caller != "shop/payments" || spike.Namespace != "shop" || len(spike.Pods) != 1 {
		t.Errorf("db incident should borrow its caller: caller=%q ns=%q pods=%v", spike.Caller, spike.Namespace, spike.Pods)
	}
	if len(spike.TraceIDs) == 0 || len(spike.Errors) == 0 {
		t.Errorf("missing evidence: traces=%v errors=%v", spike.TraceIDs, spike.Errors)
	}
	if !strings.Contains(spike.Errors[0], "connection refused to <n>.<n>.<n>.<n>:<n>") || len(spike.Errors) != 1 {
		t.Errorf("error signature not normalised: %q", spike.Errors[0])
	}
	// One incident for the whole failing chain, not one per caller.
	spikes := 0
	for _, in := range list {
		if in.Kind == InsightErrorSpike {
			spikes++
		}
	}
	if spikes != 1 || len(list) != 1 {
		t.Errorf("got %d insights (%d error spikes), want 1 grouped incident: %+v", len(list), spikes, list)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if in := eng.byID(spike.ID); in != nil && in.AI.Status == "done" {
			if in.AI.RootCause != "payments database down" || in.AI.Confidence != 0.9 {
				t.Errorf("AI analysis = %+v", in.AI)
			}
			fa.mu.Lock()
			prompt := strings.Join(fa.prompts, "\n---\n")
			fa.mu.Unlock()
			if len(fa.prompts) != 1 {
				t.Errorf("AI called %d times, want once for one incident", len(fa.prompts))
			}
			if !strings.Contains(prompt, "connection refused") || !strings.Contains(prompt, "db:postgresql:payments") {
				t.Errorf("prompt lacks evidence:\n%s", prompt)
			}
			hookMu.Lock()
			defer hookMu.Unlock()
			if len(hooked) == 0 {
				t.Error("insight hook not called")
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("AI analysis did not complete")
}

func TestOTLPJSONIngest(t *testing.T) {
	store := NewStore(time.Hour)
	now := time.Now().UnixNano()
	body := fmt.Sprintf(`{"resourceSpans":[{"resource":{"attributes":[
		{"key":"service.name","value":{"stringValue":"checkout"}},
		{"key":"k8s.namespace.name","value":{"stringValue":"shop"}}]},
	  "scopeSpans":[{"spans":[{"traceId":"5b8efff798038103d269b633813fc60c","spanId":"eee19b7ec3c1b174",
		"name":"POST /checkout","kind":2,"startTimeUnixNano":"%d","endTimeUnixNano":"%d",
		"status":{"code":2,"message":"boom"},
		"attributes":[{"key":"http.response.status_code","value":{"intValue":"500"}}]}]}]}]}`, now, now+int64(30*time.Millisecond))
	n, err := store.IngestOTLPTraces([]byte(body), true)
	if err != nil || n != 1 {
		t.Fatalf("ingest = %d, %v", n, err)
	}
	sp := store.Trace("5b8efff798038103d269b633813fc60c")
	if len(sp) != 1 {
		t.Fatalf("trace not stored by hex id")
	}
	got := sp[0]
	if got.Service != "checkout" || got.Kind != "server" || got.Status != "ERROR" || got.StatusMessage != "boom" ||
		got.Duration != 30*time.Millisecond || got.Labels["http.response.status_code"] != "500" || got.SpanID != "eee19b7ec3c1b174" {
		t.Errorf("span = %+v", got)
	}
}

// Early in an outage only a few requests have failed: callers report "new
// error" before any threshold for a spike is met. That must still be one
// incident at the failing dependency, not one card per caller.
func TestEarlyOutageGroupsSymptomsAtOrigin(t *testing.T) {
	store := NewStore(time.Hour)
	now := time.Now().UTC()
	for i := 0; i < 60; i++ {
		store.IngestSpans(request(i, now.Add(-10*time.Minute-time.Duration(i)*15*time.Second), false, 5*time.Millisecond)...)
	}
	for i := 100; i < 130; i++ {
		store.IngestSpans(request(i, now.Add(-time.Duration(i-100)*5*time.Second), i%10 == 0, 5*time.Millisecond)...)
	}
	// Healthy extra frontend traffic dilutes its error rate below the edge
	// threshold, so its own downstream walk stops at the frontend.
	for i := 200; i < 600; i++ {
		tr := fmt.Sprintf("h%04d", i)
		at := now.Add(-time.Duration(i-200) * 500 * time.Millisecond)
		store.IngestSpans(
			span(tr, "a", "", "frontend", "shop", "server", "GET /", "OK", at, 5*time.Millisecond, nil),
			span(tr, "b", "a", "frontend", "shop", "client", "POST checkout", "OK", at, 4*time.Millisecond, nil),
			span(tr, "c", "b", "checkout", "shop", "server", "GET /status", "OK", at, 3*time.Millisecond, nil),
		)
	}
	eng := newInsightEngine(store, zap.NewNop())
	eng.tick(context.Background())
	list := eng.list()
	if len(list) != 1 {
		t.Fatalf("got %d insights, want 1: %+v", len(list), list)
	}
	if list[0].RootCause != "db:postgresql:payments" {
		t.Errorf("root = %s", list[0].RootCause)
	}
}

func keys(m map[string]GraphEdge) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
