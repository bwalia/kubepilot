package otelautopilot

import "testing"

func TestDetectRuntimeAndScore(t *testing.T) {
	d := NewDiscoverer(nil, []string{"java", "nodejs", "python", "dotnet", "go"}, nil, []string{"kube-system"})

	cap, _, _, signals := d.scoreRuntime("java", "openjdk:17")
	if cap != CapabilityAutoInstr {
		t.Fatalf("java expected auto-instr, got %s", cap)
	}
	if !contains(signals, "traces") {
		t.Fatalf("java should include traces")
	}

	cap, _, _, _ = d.scoreRuntime("go", "golang:1.22")
	if cap != CapabilityEBPF {
		t.Fatalf("go expected ebpf, got %s", cap)
	}

	cap, _, _, _ = d.scoreRuntime("unknown", "busybox:latest")
	if cap != CapabilityCollector {
		t.Fatalf("unknown expected collector-only, got %s", cap)
	}
}

func TestBuildEnablementPlanAutoInstr(t *testing.T) {
	app := AppCoverage{
		ID: "deployment/demo/api", Kind: "Deployment", Name: "api", Namespace: "demo",
		Runtime: "nodejs", Capability: CapabilityAutoInstr, Signals: []string{"logs", "metrics", "traces"},
	}
	plan := BuildEnablementPlan(app, true)
	if !plan.Applied {
		t.Fatal("expected plan applied in enable mode")
	}
	if plan.Annotations["instrumentation.opentelemetry.io/inject-nodejs"] != "true" {
		t.Fatalf("missing inject annotation: %#v", plan.Annotations)
	}
}

func TestStoreIngestAndEvidence(t *testing.T) {
	s := NewStore(0)
	s.IngestMetrics(MetricSample{Name: "http.server.request.duration", Value: 10, Service: "api", Plane: PlaneApp, Labels: map[string]string{"namespace": "demo", "pod": "api-1"}})
	s.IngestLogs(LogRecord{Body: "boom", Service: "api", Plane: PlaneApp, Labels: map[string]string{"k8s.namespace.name": "demo", "k8s.pod.name": "api-1"}})
	s.IngestSpans(SpanRecord{TraceID: "t1", SpanID: "s1", Name: "GET /", Service: "api", Plane: PlaneApp})

	m, l, tr := s.EvidenceForResource("demo", "api", 3)
	if len(m) == 0 || len(l) == 0 || len(tr) == 0 {
		t.Fatalf("expected evidence metrics=%d logs=%d traces=%d", len(m), len(l), len(tr))
	}
}

func TestControllerModes(t *testing.T) {
	c := New(Config{Policy: DefaultPolicy()}, nil)
	if c.Policy().Mode != ModeOff {
		t.Fatal("default should be off")
	}
	if err := c.SetMode(ModeObserve); err != nil {
		t.Fatal(err)
	}
	c.Pause()
	if c.Policy().Mode != ModeOff {
		t.Fatal("pause should force off")
	}
	c.Resume()
	if c.Policy().Mode != ModeObserve {
		t.Fatalf("resume expected observe, got %s", c.Policy().Mode)
	}
}

func TestAggregateCoverage(t *testing.T) {
	apps := []AppCoverage{
		{Capability: CapabilityAutoInstr, Enabled: true},
		{Capability: CapabilityEBPF},
		{Capability: CapabilityCollector},
		{Capability: CapabilityUnsupported},
	}
	r := AggregateCoverage(ModeEnable, apps)
	if r.Totals.Total != 4 || r.Totals.Enabled != 1 || r.Totals.AutoInstr != 1 {
		t.Fatalf("unexpected totals: %+v", r.Totals)
	}
}

func TestExportActive(t *testing.T) {
	p := DefaultPolicy()
	p.Export.MetricsRemoteWriteURL = "http://thanos:10908/api/v1/receive"
	c := New(Config{Policy: p}, nil)
	if !c.ExportActive() {
		t.Fatal("expected export active when remote_write configured")
	}
}
