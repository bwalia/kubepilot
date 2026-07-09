package k8s

import (
	"context"
	"fmt"
	"strings"

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
	// InternalIP is kept for backward compatibility; human-readable LAN/WAN/Tunnel summary.
	InternalIP               string
	// IPs lists every unique address (flat union).
	IPs                      []string
	// LANIPs are private / on-prem / VPC internal addresses.
	LANIPs                   []string
	// WANIPs are public / external addresses.
	WANIPs                   []string
	// TunnelIPs are WireGuard, flannel, or other overlay endpoint addresses.
	TunnelIPs                []string
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

	buckets := classifyNodeIPs(node)
	s.LANIPs = buckets.lan
	s.WANIPs = buckets.wan
	s.TunnelIPs = buckets.tunnel
	s.IPs = mergeNodeIPBuckets(buckets)
	if summary := formatNodeIPSummary(buckets); summary != "" {
		s.InternalIP = summary
	} else if len(s.IPs) > 0 {
		s.InternalIP = strings.Join(s.IPs, ", ")
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
