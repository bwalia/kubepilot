package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodSummary is a concise representation of a pod for dashboard and AI use.
type PodSummary struct {
	Name      string
	Namespace string
	Phase     corev1.PodPhase
	// Reason captures CrashLoopBackOff, OOMKilled, ImagePullBackOff, etc.
	Reason    string
	NodeName  string
	Restarts  int32
	Ready     bool
}

// ListPods returns all pods across all namespaces (or a specific namespace).
// Passing an empty namespace string lists pods from all namespaces.
func (c *Client) ListPods(ctx context.Context, namespace string) ([]PodSummary, error) {
	list, err := c.Core.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pods in namespace %q: %w", namespace, err)
	}

	summaries := make([]PodSummary, 0, len(list.Items))
	for _, pod := range list.Items {
		summaries = append(summaries, toPodSummary(pod))
	}
	return summaries, nil
}

// GetPodLogs retrieves the last tailLines log lines from the named container.
// If containerName is empty the first container in the pod spec is used.
// For multi-container pods this avoids the "a container name must be specified" error.
func (c *Client) GetPodLogs(ctx context.Context, namespace, podName, containerName string, tailLines int64) (string, error) {
	// When no container is specified, look up the pod and pick the first container.
	if containerName == "" {
		pod, err := c.Core.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("getting pod %s/%s to resolve container name: %w", namespace, podName, err)
		}
		if len(pod.Spec.Containers) > 0 {
			containerName = pod.Spec.Containers[0].Name
		}
	}

	opts := &corev1.PodLogOptions{TailLines: &tailLines}
	if containerName != "" {
		opts.Container = containerName
	}

	req := c.Core.CoreV1().Pods(namespace).GetLogs(podName, opts)
	result := req.Do(ctx)
	if err := result.Error(); err != nil {
		return "", fmt.Errorf("getting logs for pod %s/%s: %w", namespace, podName, err)
	}

	raw, err := result.Raw()
	if err != nil {
		return "", fmt.Errorf("reading log response for pod %s/%s: %w", namespace, podName, err)
	}
	return string(raw), nil
}

// DeletePod deletes a specific pod, triggering controller-managed restart.
func (c *Client) DeletePod(ctx context.Context, namespace, podName string) error {
	err := c.Core.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("deleting pod %s/%s: %w", namespace, podName, err)
	}
	return nil
}

// ListCrashingPods returns pods in CrashLoopBackOff, OOMKilled, or Error states.
// These are the primary targets for AI-driven troubleshooting.
func (c *Client) ListCrashingPods(ctx context.Context, namespace string) ([]PodSummary, error) {
	all, err := c.ListPods(ctx, namespace)
	if err != nil {
		return nil, err
	}

	crashing := make([]PodSummary, 0)
	for _, p := range all {
		switch p.Reason {
		case "CrashLoopBackOff", "OOMKilled", "Error", "ImagePullBackOff", "ErrImagePull":
			crashing = append(crashing, p)
		}
		// Also capture pods that are not ready and have high restart counts.
		if !p.Ready && p.Restarts > 5 {
			crashing = append(crashing, p)
		}
	}
	return crashing, nil
}

func toPodSummary(pod corev1.Pod) PodSummary {
	summary := PodSummary{
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Phase:     pod.Status.Phase,
		NodeName:  pod.Spec.NodeName,
	}

	// Extract the waiting reason from the first container status if present.
	for _, cs := range pod.Status.ContainerStatuses {
		summary.Restarts += cs.RestartCount
		if cs.State.Waiting != nil {
			summary.Reason = cs.State.Waiting.Reason
		}
		if cs.State.Terminated != nil && summary.Reason == "" {
			summary.Reason = cs.State.Terminated.Reason
		}
		if cs.Ready {
			summary.Ready = true
		}
	}

	return summary
}
