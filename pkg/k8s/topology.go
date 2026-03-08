package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceDependency represents a discovered dependency between services.
type ServiceDependency struct {
	Source      string `json:"source"`      // "namespace/service-name"
	Target      string `json:"target"`      // "namespace/service-name" or "external"
	Port        int32  `json:"port"`
	Protocol    string `json:"protocol"`
}

// DiscoverServiceDependencies attempts to discover service dependencies
// by examining environment variables and service endpoints.
// This is a heuristic approach — service mesh integration provides more accurate results.
func (c *Client) DiscoverServiceDependencies(ctx context.Context, namespace string) ([]ServiceDependency, error) {
	podList, err := c.Core.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pods for dependency discovery: %w", err)
	}

	svcList, err := c.Core.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing services for dependency discovery: %w", err)
	}

	// Build a set of known service names in the namespace.
	knownServices := make(map[string]bool, len(svcList.Items))
	for _, svc := range svcList.Items {
		knownServices[svc.Name] = true
	}

	var deps []ServiceDependency
	seen := make(map[string]bool)

	// Scan pod environment variables for references to other services.
	// Kubernetes DNS names follow: <service>.<namespace>.svc.cluster.local
	for _, pod := range podList.Items {
		// Determine which service owns this pod by matching labels.
		ownerService := ""
		for _, svc := range svcList.Items {
			if matchLabels(pod.Labels, svc.Spec.Selector) {
				ownerService = svc.Name
				break
			}
		}
		if ownerService == "" {
			continue
		}

		for _, container := range pod.Spec.Containers {
			for _, env := range container.Env {
				// Look for environment variable values that reference other services.
				for targetSvc := range knownServices {
					if targetSvc == ownerService {
						continue
					}
					// Check if the env value contains a reference to another service.
					if containsServiceRef(env.Value, targetSvc, namespace) {
						key := ownerService + "->" + targetSvc
						if !seen[key] {
							seen[key] = true
							deps = append(deps, ServiceDependency{
								Source:   fmt.Sprintf("%s/%s", namespace, ownerService),
								Target:   fmt.Sprintf("%s/%s", namespace, targetSvc),
								Protocol: "tcp",
							})
						}
					}
				}
			}
		}
	}

	return deps, nil
}

// matchLabels checks if all selector labels are present in the target labels.
func matchLabels(labels, selector map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	for key, val := range selector {
		if labels[key] != val {
			return false
		}
	}
	return true
}

// containsServiceRef checks if a string references a Kubernetes service.
func containsServiceRef(value, serviceName, namespace string) bool {
	// Check for common service reference patterns.
	patterns := []string{
		serviceName + "." + namespace,                              // service.namespace
		serviceName + "." + namespace + ".svc",                    // service.namespace.svc
		serviceName + "." + namespace + ".svc.cluster.local",      // FQDN
		serviceName + ":" ,                                         // service:port
	}
	for _, p := range patterns {
		if len(value) >= len(p) && containsSubstring(value, p) {
			return true
		}
	}
	return false
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
