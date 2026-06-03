package k8s

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

// knownGVR maps the lowercase resource kind exposed by the dashboard to its
// GroupVersionResource so the dynamic client can fetch any browsed resource.
var knownGVR = map[string]schema.GroupVersionResource{
	"pod":                   {Group: "", Version: "v1", Resource: "pods"},
	"service":               {Group: "", Version: "v1", Resource: "services"},
	"configmap":             {Group: "", Version: "v1", Resource: "configmaps"},
	"secret":                {Group: "", Version: "v1", Resource: "secrets"},
	"persistentvolumeclaim": {Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
	"pvc":                   {Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
	"namespace":             {Group: "", Version: "v1", Resource: "namespaces"},
	"node":                  {Group: "", Version: "v1", Resource: "nodes"},
	"deployment":            {Group: "apps", Version: "v1", Resource: "deployments"},
	"statefulset":           {Group: "apps", Version: "v1", Resource: "statefulsets"},
	"daemonset":             {Group: "apps", Version: "v1", Resource: "daemonsets"},
	"replicaset":            {Group: "apps", Version: "v1", Resource: "replicasets"},
	"job":                   {Group: "batch", Version: "v1", Resource: "jobs"},
	"cronjob":               {Group: "batch", Version: "v1", Resource: "cronjobs"},
	"ingress":               {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
	"storageclass":          {Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses"},
}

// clusterScopedKinds are resource kinds that are not namespaced.
var clusterScopedKinds = map[string]bool{
	"namespace":    true,
	"node":         true,
	"storageclass": true,
}

// GetResourceYAML fetches any supported resource via the dynamic client and
// returns sanitized YAML suitable for a read-only viewer.
//
// SECURITY: for Secret resources the data and stringData blocks are stripped so
// secret values never reach the client. The noisy managedFields block is removed
// for every resource (matching `kubectl get -o yaml` ergonomics).
func (c *Client) GetResourceYAML(ctx context.Context, kind, namespace, name string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(kind))
	gvr, ok := knownGVR[key]
	if !ok {
		return "", fmt.Errorf("unsupported resource kind %q", kind)
	}

	var raw map[string]interface{}

	if clusterScopedKinds[key] {
		u, err := c.Dynamic.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("getting %s %q: %w", kind, name, err)
		}
		raw = u.Object
	} else {
		u, err := c.Dynamic.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("getting %s %s/%s: %w", kind, namespace, name, err)
		}
		raw = u.Object
	}

	sanitizeResource(raw, key)

	out, err := yaml.Marshal(raw)
	if err != nil {
		return "", fmt.Errorf("marshalling %s %q to yaml: %w", kind, name, err)
	}
	return string(out), nil
}

// sanitizeResource removes noisy and sensitive fields from an unstructured
// resource map prior to YAML marshalling.
func sanitizeResource(obj map[string]interface{}, kind string) {
	// Strip managedFields from metadata for every resource.
	if meta, ok := obj["metadata"].(map[string]interface{}); ok {
		delete(meta, "managedFields")
	}

	// Never expose Secret values.
	if kind == "secret" {
		delete(obj, "data")
		delete(obj, "stringData")
	}
}
