package k8s

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodCondition holds a single pod condition for diagnostics.
type PodCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// ContainerDiag holds diagnostic information for a single container.
type ContainerDiag struct {
	Name         string `json:"name"`
	Image        string `json:"image"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restart_count"`
	State        string `json:"state"`           // Running, Waiting, Terminated
	StateReason  string `json:"state_reason"`     // CrashLoopBackOff, OOMKilled, etc.
	StateMessage string `json:"state_message"`
	ExitCode     int32  `json:"exit_code"`
	LastTerminatedReason string `json:"last_terminated_reason,omitempty"`
}

// OwnerRef traces the ownership chain of a resource.
type OwnerRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	UID  string `json:"uid"`
}

// PodDiagnostics provides comprehensive pod information for AI troubleshooting.
type PodDiagnostics struct {
	Name              string           `json:"name"`
	Namespace         string           `json:"namespace"`
	Phase             string           `json:"phase"`
	NodeName          string           `json:"node_name"`
	ServiceAccount    string           `json:"service_account"`
	CreatedAt         time.Time        `json:"created_at"`
	Conditions        []PodCondition   `json:"conditions"`
	ContainerStatuses []ContainerDiag  `json:"container_statuses"`
	Events            []Event          `json:"events"`
	ResourceUsage     *ResourceMetrics `json:"resource_usage,omitempty"`
	OwnerChain        []OwnerRef       `json:"owner_chain"`
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	Tolerations       []string         `json:"tolerations,omitempty"`
	NodeSelector      map[string]string `json:"node_selector,omitempty"`
	Volumes           []string         `json:"volumes,omitempty"`
}

// DeploymentDiagnostics provides comprehensive deployment information.
type DeploymentDiagnostics struct {
	Name              string           `json:"name"`
	Namespace         string           `json:"namespace"`
	Replicas          int32            `json:"replicas"`
	ReadyReplicas     int32            `json:"ready_replicas"`
	UpdatedReplicas   int32            `json:"updated_replicas"`
	AvailableReplicas int32            `json:"available_replicas"`
	Strategy          string           `json:"strategy"`
	Events            []Event          `json:"events"`
	Conditions        []string         `json:"conditions"`
	Labels            map[string]string `json:"labels"`
	PodTemplate       string           `json:"pod_template_hash,omitempty"`
}

// GetPodDiagnostics returns full diagnostic information for a pod, including
// events, resource usage, conditions, container statuses, and ownership chain.
func (c *Client) GetPodDiagnostics(ctx context.Context, namespace, podName string) (*PodDiagnostics, error) {
	pod, err := c.Core.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting pod %s/%s: %w", namespace, podName, err)
	}

	diag := &PodDiagnostics{
		Name:           pod.Name,
		Namespace:      pod.Namespace,
		Phase:          string(pod.Status.Phase),
		NodeName:       pod.Spec.NodeName,
		ServiceAccount: pod.Spec.ServiceAccountName,
		CreatedAt:      pod.CreationTimestamp.Time,
		Labels:         pod.Labels,
		Annotations:    pod.Annotations,
		NodeSelector:   pod.Spec.NodeSelector,
	}

	// Pod conditions.
	for _, cond := range pod.Status.Conditions {
		diag.Conditions = append(diag.Conditions, PodCondition{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  cond.Reason,
			Message: cond.Message,
		})
	}

	// Container statuses — both init and regular containers.
	allStatuses := append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...)
	for _, cs := range allStatuses {
		cd := ContainerDiag{
			Name:         cs.Name,
			Image:        cs.Image,
			Ready:        cs.Ready,
			RestartCount: cs.RestartCount,
		}

		if cs.State.Running != nil {
			cd.State = "Running"
		} else if cs.State.Waiting != nil {
			cd.State = "Waiting"
			cd.StateReason = cs.State.Waiting.Reason
			cd.StateMessage = cs.State.Waiting.Message
		} else if cs.State.Terminated != nil {
			cd.State = "Terminated"
			cd.StateReason = cs.State.Terminated.Reason
			cd.StateMessage = cs.State.Terminated.Message
			cd.ExitCode = cs.State.Terminated.ExitCode
		}

		if cs.LastTerminationState.Terminated != nil {
			cd.LastTerminatedReason = cs.LastTerminationState.Terminated.Reason
		}

		// Determine image from container spec if not in status.
		if cd.Image == "" {
			for _, container := range pod.Spec.Containers {
				if container.Name == cs.Name {
					cd.Image = container.Image
					break
				}
			}
		}

		diag.ContainerStatuses = append(diag.ContainerStatuses, cd)
	}

	// Owner chain — follow the ownership hierarchy.
	diag.OwnerChain = buildOwnerChain(pod.OwnerReferences)

	// Tolerations.
	for _, t := range pod.Spec.Tolerations {
		tolStr := fmt.Sprintf("%s=%s:%s", t.Key, t.Value, t.Effect)
		if t.Key == "" {
			tolStr = fmt.Sprintf("*:%s", t.Effect)
		}
		diag.Tolerations = append(diag.Tolerations, tolStr)
	}

	// Volume names for awareness of persistent storage.
	for _, v := range pod.Spec.Volumes {
		diag.Volumes = append(diag.Volumes, v.Name)
	}

	// Fetch related events — non-fatal if this fails.
	events, err := c.GetEventsForResource(ctx, namespace, podName, "Pod")
	if err == nil {
		diag.Events = events
	}

	// Fetch resource metrics — non-fatal if metrics-server is unavailable.
	metrics, err := c.GetPodResourceMetrics(ctx, namespace, podName)
	if err == nil {
		diag.ResourceUsage = metrics
	}

	return diag, nil
}

// GetDeploymentDiagnostics returns comprehensive deployment diagnostics.
func (c *Client) GetDeploymentDiagnostics(ctx context.Context, namespace, name string) (*DeploymentDiagnostics, error) {
	dep, err := c.Core.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting deployment %s/%s: %w", namespace, name, err)
	}

	diag := &DeploymentDiagnostics{
		Name:              dep.Name,
		Namespace:         dep.Namespace,
		Replicas:          *dep.Spec.Replicas,
		ReadyReplicas:     dep.Status.ReadyReplicas,
		UpdatedReplicas:   dep.Status.UpdatedReplicas,
		AvailableReplicas: dep.Status.AvailableReplicas,
		Strategy:          string(dep.Spec.Strategy.Type),
		Labels:            dep.Labels,
	}

	for _, cond := range dep.Status.Conditions {
		diag.Conditions = append(diag.Conditions,
			fmt.Sprintf("%s=%s (reason=%s)", cond.Type, cond.Status, cond.Reason))
	}

	if hash, ok := dep.Spec.Template.Labels["pod-template-hash"]; ok {
		diag.PodTemplate = hash
	}

	events, err := c.GetEventsForResource(ctx, namespace, name, "Deployment")
	if err == nil {
		diag.Events = events
	}

	return diag, nil
}

func buildOwnerChain(refs []metav1.OwnerReference) []OwnerRef {
	chain := make([]OwnerRef, 0, len(refs))
	for _, ref := range refs {
		chain = append(chain, OwnerRef{
			Kind: ref.Kind,
			Name: ref.Name,
			UID:  string(ref.UID),
		})
	}
	return chain
}

// ClusterSnapshot captures a point-in-time view of the entire cluster for anomaly detection.
type ClusterSnapshot struct {
	Timestamp   time.Time            `json:"timestamp"`
	Pods        []PodSummary         `json:"pods"`
	Nodes       []NodeSummary        `json:"nodes"`
	Deployments []DeploymentSummary  `json:"deployments"`
	Events      []Event              `json:"events"`
}

// TakeClusterSnapshot collects a comprehensive snapshot of the cluster state.
func (c *Client) TakeClusterSnapshot(ctx context.Context) (*ClusterSnapshot, error) {
	snapshot := &ClusterSnapshot{
		Timestamp: time.Now().UTC(),
	}

	pods, err := c.ListPods(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("snapshot: listing pods: %w", err)
	}
	snapshot.Pods = pods

	nodes, err := c.ListNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("snapshot: listing nodes: %w", err)
	}
	snapshot.Nodes = nodes

	deps, err := c.ListDeployments(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("snapshot: listing deployments: %w", err)
	}
	snapshot.Deployments = deps

	events, err := c.GetWarningEvents(ctx, "", 15*time.Minute)
	if err != nil {
		// Warning events are useful but not critical to the snapshot.
		snapshot.Events = []Event{}
	} else {
		snapshot.Events = events
	}

	return snapshot, nil
}
