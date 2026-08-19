package telemetry

import (
	"net/http"
	"strings"
)

// KubernetesSpanName names a span for one Kubernetes API request.
//
// otelhttp would otherwise name every one of them "HTTP GET", which makes the
// API-server calls in a trace indistinguishable. Naming them by the resource
// they act on ("k8s GET pods/log") is what makes a slow dashboard page legible
// at a glance.
func KubernetesSpanName(r *http.Request) string {
	resource := KubernetesResource(r.URL.Path)
	if resource == "" {
		return "k8s " + r.Method
	}
	return "k8s " + r.Method + " " + resource
}

// KubernetesResource reduces an API-server path to the resource type it targets,
// discarding namespace and object names.
//
// That discarding is the point: the names are unbounded, and putting them in a
// span name or a metric label is how a metrics backend gets a cardinality
// explosion. "/api/v1/namespaces/kube-system/pods/coredns-abc123/log" collapses
// to "pods/log".
//
// It returns "" for a path it does not recognise, so callers can fall back to a
// plain method name rather than emitting something misleading.
func KubernetesResource(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}

	// Core group is served at /api/v1/..., named groups at /apis/<group>/<version>/...
	var i int
	switch parts[0] {
	case "api":
		i = 2 // skip "api" and the version
	case "apis":
		i = 3 // skip "apis", the group, and the version
	default:
		return ""
	}
	if i >= len(parts) {
		return ""
	}

	// A namespaced request inserts "namespaces/<name>" before the resource. The
	// exception is a request for the namespace objects themselves, where
	// "namespaces" is the resource.
	if parts[i] == "namespaces" && i+2 < len(parts) {
		i += 2
	}
	if i >= len(parts) {
		return ""
	}

	resource := parts[i]
	// A subresource follows the object name: .../pods/<name>/log -> "pods/log".
	if i+2 < len(parts) {
		return resource + "/" + parts[i+2]
	}
	return resource
}
