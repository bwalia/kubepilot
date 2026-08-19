package telemetry

import "testing"

func TestKubernetesResource(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		// Core group.
		{"cluster-scoped list", "/api/v1/pods", "pods"},
		{"cluster-scoped nodes", "/api/v1/nodes", "nodes"},
		{"namespaced list", "/api/v1/namespaces/default/pods", "pods"},
		{"namespaced get", "/api/v1/namespaces/default/pods/my-pod", "pods"},
		{"subresource", "/api/v1/namespaces/kube-system/pods/coredns-abc123/log", "pods/log"},
		{"namespaces themselves", "/api/v1/namespaces", "namespaces"},
		{"single namespace", "/api/v1/namespaces/default", "namespaces"},

		// Named groups.
		{"apps group", "/apis/apps/v1/namespaces/prod/deployments", "deployments"},
		{"apps group get", "/apis/apps/v1/namespaces/prod/deployments/api", "deployments"},
		{"scale subresource", "/apis/apps/v1/namespaces/prod/deployments/api/scale", "deployments/scale"},
		{"metrics group", "/apis/metrics.k8s.io/v1beta1/nodes", "nodes"},

		// Unrecognised shapes fall back to empty so the caller can degrade.
		{"root", "/", ""},
		{"version endpoint", "/version", ""},
		{"healthz", "/healthz", ""},
		{"truncated core", "/api/v1", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := KubernetesResource(tt.path); got != tt.want {
				t.Errorf("KubernetesResource(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// TestKubernetesResourceDropsIdentifiers is the cardinality guard: object and
// namespace names must never survive into a span name or metric label.
func TestKubernetesResourceDropsIdentifiers(t *testing.T) {
	paths := []string{
		"/api/v1/namespaces/tenant-8f3a/pods/checkout-7d9f8b-x2k4l",
		"/api/v1/namespaces/tenant-9c2b/pods/checkout-7d9f8b-q7m1p",
	}
	first := KubernetesResource(paths[0])
	for _, p := range paths[1:] {
		if got := KubernetesResource(p); got != first {
			t.Errorf("distinct object names produced distinct labels: %q vs %q", first, got)
		}
	}
	if first != "pods" {
		t.Errorf("got %q, want %q", first, "pods")
	}
}
