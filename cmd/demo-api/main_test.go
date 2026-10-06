package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The iOS app decodes these responses with strict Codable models, so the mock
// cluster must keep every field and JSON type the original fixtures had.
// testdata/baseline holds responses captured from the pre-simulation server.
var baselineRoutes = map[string]struct{ method, path string }{
	"version":         {"GET", "/api/v1/version"},
	"config":          {"GET", "/api/v1/config"},
	"kubeconfigs":     {"GET", "/api/v1/clusters/kubeconfigs"},
	"pods":            {"GET", "/api/v1/clusters/pods"},
	"crashing-pods":   {"GET", "/api/v1/clusters/crashing-pods"},
	"deployments":     {"GET", "/api/v1/clusters/deployments"},
	"nodes":           {"GET", "/api/v1/clusters/nodes"},
	"namespaces":      {"GET", "/api/v1/namespaces"},
	"events":          {"GET", "/api/v1/events"},
	"troubleshooting": {"GET", "/api/v1/troubleshooting/summary"},
	"diagnostics":     {"GET", "/api/v1/clusters/pods/payments/checkout-api-7d9f8c/diagnostics"},
	"ai-health":       {"GET", "/api/v1/ai/health"},
	"ai-troubleshoot": {"GET", "/api/v1/ai/troubleshoot/payments/checkout-api-7d9f8c"},
	"ai-interpret":    {"POST", "/api/v1/ai/interpret"},
	"ai-execute":      {"POST", "/api/v1/ai/execute-action"},
	"rca-list":        {"GET", "/api/v1/rca"},
	"rca-get":         {"GET", "/api/v1/rca/rca-demo-checkout"},
	"anomalies":       {"GET", "/api/v1/anomalies"},
	"autopilot":       {"GET", "/api/v1/autopilot?limit=50"},
	"ok":              {"POST", "/api/v1/autopilot/pause"},
}

func testServer(at time.Time) http.Handler {
	s := &server{user: "apple", password: "review", started: at, clock: func() time.Time { return at }}
	return withCORS(s.routes())
}

func get(t *testing.T, h http.Handler, method, path string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(`{"command":"why is checkout crashing"}`))
	req.SetBasicAuth("apple", "review")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body, _ := io.ReadAll(rec.Result().Body)
	return rec.Code, body
}

// sampleTimes covers every rotating incident (4 slots) plus slot boundaries.
func sampleTimes() []time.Time {
	base := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	var out []time.Time
	for i := 0; i < 4; i++ {
		slot := base.Add(time.Duration(i) * incidentSlot)
		out = append(out, slot, slot.Add(90*time.Second), slot.Add(17*time.Minute), slot.Add(incidentSlot-time.Second))
	}
	return append(out, time.Now())
}

// compatible reports where got breaks the shape of want.
func compatible(path string, want, got any) []string {
	switch w := want.(type) {
	case nil:
		return nil // optional in the app's models
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return []string{fmt.Sprintf("%s: want object, got %T", path, got)}
		}
		var errs []string
		for k, wv := range w {
			gv, present := g[k]
			if !present {
				errs = append(errs, path+"."+k+": missing")
				continue
			}
			errs = append(errs, compatible(path+"."+k, wv, gv)...)
		}
		return errs
	case []any:
		g, ok := got.([]any)
		if !ok {
			return []string{fmt.Sprintf("%s: want array, got %T", path, got)}
		}
		if len(w) == 0 {
			return nil
		}
		var errs []string
		for i, gv := range g {
			// Elements can differ in optional fields; compare against the
			// baseline element of the same index when there is one.
			errs = append(errs, compatible(fmt.Sprintf("%s[%d]", path, i), w[min(i, len(w)-1)], gv)...)
		}
		return errs
	case float64:
		g, ok := got.(float64)
		if !ok {
			return []string{fmt.Sprintf("%s: want number, got %T", path, got)}
		}
		if w == math.Trunc(w) && strings.Contains(path, "confidence") == false && g != math.Trunc(g) {
			return []string{fmt.Sprintf("%s: want integer, got %v", path, g)}
		}
		return nil
	default:
		if reflect.TypeOf(want) != reflect.TypeOf(got) {
			return []string{fmt.Sprintf("%s: want %T, got %T", path, want, got)}
		}
		return nil
	}
}

func TestResponsesKeepBaselineShape(t *testing.T) {
	for _, at := range sampleTimes() {
		h := testServer(at)
		for name, rt := range baselineRoutes {
			raw, err := os.ReadFile(filepath.Join("testdata", "baseline", name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var want, got any
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s baseline: %v", name, err)
			}
			code, body := get(t, h, rt.method, rt.path)
			if code != http.StatusOK {
				t.Errorf("%s at %s: HTTP %d: %s", name, at.Format(time.Kitchen), code, body)
				continue
			}
			if err := json.Unmarshal(body, &got); err != nil {
				t.Errorf("%s: invalid JSON: %v", name, err)
				continue
			}
			for _, e := range compatible(name, want, got) {
				t.Errorf("at %s: %s", at.Format(time.RFC3339), e)
			}
		}
	}
}

// Diagnostics events were empty in the baseline; the app decodes them as
// KubeEvent, the same shape as /events items.
func TestDiagnosticEventsMatchEventShape(t *testing.T) {
	raw, _ := os.ReadFile(filepath.Join("testdata", "baseline", "events.json"))
	var events struct{ Items []any }
	_ = json.Unmarshal(raw, &events)
	for _, at := range sampleTimes() {
		h := testServer(at)
		snap := simulate(at)
		for _, p := range snap.Pods {
			if p.Incident == nil {
				continue
			}
			_, body := get(t, h, "GET", "/api/v1/clusters/pods/"+p.Namespace+"/"+p.Name+"/diagnostics")
			var d struct{ Diagnostics struct{ Events []any } }
			if err := json.Unmarshal(body, &d); err != nil {
				t.Fatalf("%s: %v", p.Name, err)
			}
			for i, e := range d.Diagnostics.Events {
				for _, msg := range compatible(fmt.Sprintf("%s.events[%d]", p.Name, i), events.Items[0], e) {
					t.Error(msg)
				}
			}
		}
	}
}

func TestEveryPodResolvesForDetailEndpoints(t *testing.T) {
	at := sampleTimes()[2]
	h := testServer(at)
	for _, p := range simulate(at).Pods {
		for _, suffix := range []string{"/diagnostics", "/logs?tail=50"} {
			if code, body := get(t, h, "GET", "/api/v1/clusters/pods/"+p.Namespace+"/"+p.Name+suffix); code != 200 || len(body) == 0 {
				t.Errorf("%s%s: HTTP %d", p.Key(), suffix, code)
			}
		}
		if code, _ := get(t, h, "GET", "/api/v1/ai/troubleshoot/"+p.Namespace+"/"+p.Name); code != 200 {
			t.Errorf("troubleshoot %s: HTTP %d", p.Key(), code)
		}
	}
}

func TestSimulationIsDeterministicAndRotates(t *testing.T) {
	at := time.Date(2026, 10, 6, 10, 7, 0, 0, time.UTC)
	a, b := simulate(at), simulate(at)
	if !reflect.DeepEqual(a.Pods, b.Pods) {
		t.Fatal("same instant produced different clusters")
	}
	kinds := map[string]bool{}
	for _, ts := range sampleTimes() {
		s := simulate(ts)
		if len(s.Active) != 2 || s.Active[0].Kind != "crashloop" {
			t.Fatalf("at %s: want the crashloop plus one rotating incident, got %d", ts, len(s.Active))
		}
		kinds[s.Active[1].Kind] = true
		failing := 0
		for _, p := range s.Pods {
			if !p.Ready {
				failing++
			}
		}
		if failing == 0 {
			t.Errorf("at %s: no failing pods to troubleshoot", ts)
		}
	}
	for _, k := range []string{"oom", "imagepull", "diskpressure", "nodenotready"} {
		if !kinds[k] {
			t.Errorf("incident %q never rotated in", k)
		}
	}
}

func TestCrashRestartsFollowBackoff(t *testing.T) {
	cases := map[time.Duration]int{0: 0, 10 * time.Second: 1, 30 * time.Second: 2, 5 * time.Minute: 4, 20 * time.Minute: 7}
	for since, want := range cases {
		if got := crashRestarts(since); got != want {
			t.Errorf("crashRestarts(%s) = %d, want %d", since, got, want)
		}
	}
}

func TestPublicPagesNeedNoAuth(t *testing.T) {
	h := testServer(time.Now())
	for _, path := range []string{"/", "/healthz", "/demo/status.json"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 200 {
			t.Errorf("%s: HTTP %d", path, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/clusters/pods", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("/api/v1 without auth: HTTP %d, want 401", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if !strings.Contains(rec.Body.String(), "mock API") {
		t.Error("landing page does not say it is a mock API")
	}
}
