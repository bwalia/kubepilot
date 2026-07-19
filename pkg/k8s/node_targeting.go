package k8s

import (
	"context"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeTargetingWorkload is a workload (grouped by its owner) whose nodeSelector
// pins it to a particular node — i.e. every key/value in its Spec.NodeSelector
// is present in the node's labels.
type NodeTargetingWorkload struct {
	// Kind is the owning workload kind (Deployment, DaemonSet, StatefulSet, ...)
	// or "Pod" for a bare pod with no controller.
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	// Selector is the nodeSelector that matched this node.
	Selector map[string]string `json:"selector"`
	// Pods is how many pods of this workload are pinned to the node by the selector.
	Pods int `json:"pods"`
}

// NodeTargeting returns the workloads whose nodeSelector targets the named node.
// It matches each pod's Spec.NodeSelector against the node's labels (subset match)
// and groups the results by owning workload, so the dashboard can show "what is
// pinned here, and by which selector". Pods without a nodeSelector are ignored —
// they are schedulable anywhere and do not "target" the node.
func (c *Client) NodeTargeting(ctx context.Context, nodeName string) ([]NodeTargetingWorkload, error) {
	node, err := c.Core.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting node %q: %w", nodeName, err)
	}

	pods, err := c.Core.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	// Group matching pods by owner so a 30-replica Deployment shows as one row.
	type key struct{ kind, ns, name string }
	groups := make(map[key]*NodeTargetingWorkload)
	for i := range pods.Items {
		pod := &pods.Items[i]
		if len(pod.Spec.NodeSelector) == 0 {
			continue
		}
		if !selectorMatchesLabels(pod.Spec.NodeSelector, node.Labels) {
			continue
		}
		kind, name := workloadOwner(pod)
		k := key{kind, pod.Namespace, name}
		if g, ok := groups[k]; ok {
			g.Pods++
			continue
		}
		groups[k] = &NodeTargetingWorkload{
			Kind:      kind,
			Name:      name,
			Namespace: pod.Namespace,
			Selector:  pod.Spec.NodeSelector,
			Pods:      1,
		}
	}

	out := make([]NodeTargetingWorkload, 0, len(groups))
	for _, g := range groups {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// selectorMatchesLabels reports whether every key/value in selector is present in
// labels — the same subset semantics Kubernetes uses to schedule a nodeSelector.
func selectorMatchesLabels(selector, labels map[string]string) bool {
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

// workloadOwner resolves the controller that owns a pod to a (kind, name) pair,
// collapsing a ReplicaSet to its Deployment-style hash-suffixed name. Falls back
// to ("Pod", pod name) for bare pods.
func workloadOwner(pod *corev1.Pod) (kind, name string) {
	for _, ref := range pod.OwnerReferences {
		if ref.Controller == nil || !*ref.Controller {
			continue
		}
		if ref.Kind == "ReplicaSet" {
			// nginx-7d8b49557c -> nginx: drop the trailing generated hash.
			if idx := lastDash(ref.Name); idx > 0 {
				return "Deployment", ref.Name[:idx]
			}
		}
		return ref.Kind, ref.Name
	}
	return "Pod", pod.Name
}

// lastDash returns the index of the final '-' in s, or -1 if none.
func lastDash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '-' {
			return i
		}
	}
	return -1
}
