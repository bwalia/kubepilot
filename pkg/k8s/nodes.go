package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeSummary represents a cluster node's health and resource pressure state.
type NodeSummary struct {
	Name                     string
	Ready                    bool
	MemoryPressure           bool
	DiskPressure             bool
	PIDPressure              bool
	CPUCapacity              string
	MemoryCapacity           string
	EphemeralStorageCapacity string
	KubeletVersion           string
	Unschedulable            bool
}

// ListNodes returns a summary of all nodes in the cluster.
func (c *Client) ListNodes(ctx context.Context) ([]NodeSummary, error) {
	list, err := c.Core.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing cluster nodes: %w", err)
	}

	summaries := make([]NodeSummary, 0, len(list.Items))
	for _, node := range list.Items {
		summaries = append(summaries, toNodeSummary(node))
	}
	return summaries, nil
}

// ListPressureNodes returns only nodes experiencing resource pressure.
// These are candidates for AI-driven right-sizing recommendations.
func (c *Client) ListPressureNodes(ctx context.Context) ([]NodeSummary, error) {
	all, err := c.ListNodes(ctx)
	if err != nil {
		return nil, err
	}

	pressure := make([]NodeSummary, 0)
	for _, n := range all {
		if n.MemoryPressure || n.DiskPressure || n.PIDPressure || !n.Ready {
			pressure = append(pressure, n)
		}
	}
	return pressure, nil
}

func toNodeSummary(node corev1.Node) NodeSummary {
	s := NodeSummary{
		Name:           node.Name,
		KubeletVersion: node.Status.NodeInfo.KubeletVersion,
		Unschedulable:  node.Spec.Unschedulable,
	}

	if cpu, ok := node.Status.Capacity[corev1.ResourceCPU]; ok {
		s.CPUCapacity = cpu.String()
	}
	if mem, ok := node.Status.Capacity[corev1.ResourceMemory]; ok {
		s.MemoryCapacity = mem.String()
	}
	if eph, ok := node.Status.Capacity[corev1.ResourceEphemeralStorage]; ok {
		s.EphemeralStorageCapacity = eph.String()
	}

	for _, cond := range node.Status.Conditions {
		switch cond.Type {
		case corev1.NodeReady:
			s.Ready = cond.Status == corev1.ConditionTrue
		case corev1.NodeMemoryPressure:
			s.MemoryPressure = cond.Status == corev1.ConditionTrue
		case corev1.NodeDiskPressure:
			s.DiskPressure = cond.Status == corev1.ConditionTrue
		case corev1.NodePIDPressure:
			s.PIDPressure = cond.Status == corev1.ConditionTrue
		}
	}

	return s
}
