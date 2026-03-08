package observability

import (
	"context"
	"testing"

	"github.com/kubepilot/kubepilot/pkg/ai"
	"github.com/kubepilot/kubepilot/pkg/k8s"
)

func TestCrashLoopDetector_Detects(t *testing.T) {
	detector := &CrashLoopDetector{MinRestarts: 3}

	snapshot := &k8s.ClusterSnapshot{
		Pods: []k8s.PodSummary{
			{Name: "healthy-pod", Namespace: "default", Ready: true, Restarts: 0},
			{Name: "crashing-pod", Namespace: "default", Ready: false, Restarts: 5, Reason: "CrashLoopBackOff"},
			{Name: "restarting-pod", Namespace: "prod", Ready: false, Restarts: 4},
		},
	}

	anomalies, err := detector.Evaluate(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(anomalies) != 2 {
		t.Errorf("expected 2 anomalies, got %d", len(anomalies))
	}

	for _, a := range anomalies {
		if a.Severity != ai.SeverityHigh {
			t.Errorf("expected severity 'high', got %q", a.Severity)
		}
		if a.Resource.Kind != "Pod" {
			t.Errorf("expected kind 'Pod', got %q", a.Resource.Kind)
		}
	}
}

func TestCrashLoopDetector_NoAnomalies(t *testing.T) {
	detector := &CrashLoopDetector{MinRestarts: 3}
	snapshot := &k8s.ClusterSnapshot{
		Pods: []k8s.PodSummary{
			{Name: "healthy", Namespace: "default", Ready: true, Restarts: 0},
		},
	}

	anomalies, err := detector.Evaluate(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(anomalies) != 0 {
		t.Errorf("expected 0 anomalies for healthy pod, got %d", len(anomalies))
	}
}

func TestOOMDetector(t *testing.T) {
	detector := &OOMDetector{}
	snapshot := &k8s.ClusterSnapshot{
		Pods: []k8s.PodSummary{
			{Name: "normal", Namespace: "default", Reason: ""},
			{Name: "oom-pod", Namespace: "default", Reason: "OOMKilled"},
		},
	}

	anomalies, err := detector.Evaluate(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(anomalies) != 1 {
		t.Fatalf("expected 1 OOM anomaly, got %d", len(anomalies))
	}
	if anomalies[0].Severity != ai.SeverityCritical {
		t.Errorf("expected severity 'critical', got %q", anomalies[0].Severity)
	}
}

func TestPendingPodDetector(t *testing.T) {
	detector := &PendingPodDetector{}
	snapshot := &k8s.ClusterSnapshot{
		Pods: []k8s.PodSummary{
			{Name: "running", Namespace: "default", Phase: "Running"},
			{Name: "stuck", Namespace: "default", Phase: "Pending"},
		},
	}

	anomalies, err := detector.Evaluate(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(anomalies) != 1 {
		t.Fatalf("expected 1 pending anomaly, got %d", len(anomalies))
	}
	if anomalies[0].Severity != ai.SeverityMedium {
		t.Errorf("expected severity 'medium', got %q", anomalies[0].Severity)
	}
}

func TestNodePressureDetector(t *testing.T) {
	detector := &NodePressureDetector{}
	snapshot := &k8s.ClusterSnapshot{
		Nodes: []k8s.NodeSummary{
			{Name: "healthy-node", Ready: true},
			{Name: "pressure-node", Ready: true, MemoryPressure: true, DiskPressure: true},
			{Name: "not-ready", Ready: false},
		},
	}

	anomalies, err := detector.Evaluate(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(anomalies) != 2 {
		t.Errorf("expected 2 anomalies (pressure + not-ready), got %d", len(anomalies))
	}
}

func TestFailedEventDetector(t *testing.T) {
	detector := &FailedEventDetector{MinCount: 5}
	snapshot := &k8s.ClusterSnapshot{
		Events: []k8s.Event{
			{Type: "Warning", Reason: "FailedPull", Message: "image not found", Count: 10,
				InvolvedObject: k8s.ObjectRef{Kind: "Pod", Name: "bad-pod", Namespace: "default"}},
			{Type: "Warning", Reason: "BackOff", Message: "back-off", Count: 2,
				InvolvedObject: k8s.ObjectRef{Kind: "Pod", Name: "ok-pod"}},
			{Type: "Normal", Reason: "Pulled", Message: "pulled image", Count: 20,
				InvolvedObject: k8s.ObjectRef{Kind: "Pod", Name: "good-pod"}},
		},
	}

	anomalies, err := detector.Evaluate(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(anomalies) != 1 {
		t.Fatalf("expected 1 failed event anomaly, got %d", len(anomalies))
	}
	if anomalies[0].Resource.Name != "bad-pod" {
		t.Errorf("expected resource 'bad-pod', got %q", anomalies[0].Resource.Name)
	}
}

func TestImagePullDetector(t *testing.T) {
	detector := &ImagePullDetector{}
	snapshot := &k8s.ClusterSnapshot{
		Pods: []k8s.PodSummary{
			{Name: "running", Namespace: "default", Reason: ""},
			{Name: "pull-err", Namespace: "default", Reason: "ImagePullBackOff"},
			{Name: "err-pull", Namespace: "default", Reason: "ErrImagePull"},
		},
	}

	anomalies, err := detector.Evaluate(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(anomalies) != 2 {
		t.Errorf("expected 2 image pull anomalies, got %d", len(anomalies))
	}
}

func TestDefaultRules(t *testing.T) {
	rules := DefaultRules()
	if len(rules) != 6 {
		t.Errorf("expected 6 default rules, got %d", len(rules))
	}

	names := make(map[string]bool)
	for _, r := range rules {
		name := r.Name()
		if name == "" {
			t.Error("rule has empty name")
		}
		if names[name] {
			t.Errorf("duplicate rule name: %s", name)
		}
		names[name] = true
	}
}
