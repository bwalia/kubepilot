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
	// OSImage, KernelVersion, Architecture and ContainerRuntime come from the
	// node's status.nodeInfo — enough to tell one machine from another without
	// installing anything on it.
	OSImage          string
	KernelVersion    string
	Architecture     string
	ContainerRuntime string
	// Hardware is the physical machine's vendor and model, e.g.
	// "HP ProLiant DL380 Gen9". Kubernetes doesn't know it, so it comes from the
	// kubepilot.io/hardware annotation written by the KubePilot node agent,
	// falling back to the cloud instance-type label. Empty when neither exists.
	Hardware string
	// Serial is the machine's chassis serial, from the kubepilot.io/serial
	// annotation — the only way to tell two identical boxes apart.
	Serial string
	// HardwareInfo is every kubepilot.io/hw.* annotation the node agent wrote,
	// keyed without the prefix ("cpu", "memory", "disks", "bios", ...). It is a
	// map rather than named fields so the agent can publish a new fact without
	// a matching change here, in the JSON, and in the dashboard's types.
	HardwareInfo map[string]string
	// InternalIP is kept for backward compatibility; human-readable LAN/WAN/Tunnel summary.
	InternalIP string
	// IPs lists every unique address (flat union).
	IPs []string
	// LANIPs are private / on-prem / VPC internal addresses.
	LANIPs []string
	// WANIPs are public / external addresses.
	WANIPs []string
	// TunnelIPs are WireGuard, flannel, or other overlay endpoint addresses.
	TunnelIPs []string
	// Roles are the node's roles from its node-role.kubernetes.io/* labels,
	// e.g. ["control-plane", "master"] or ["worker"]. Nodes with no role label
	// (typical k3s agents) report ["worker"].
	Roles []string
	// ControlPlane is true when the node runs the control plane (master).
	ControlPlane bool
	// Labels are the node's Kubernetes labels, surfaced verbatim for the UI.
	Labels        map[string]string
	Unschedulable bool
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
		Name:             node.Name,
		KubeletVersion:   node.Status.NodeInfo.KubeletVersion,
		OSImage:          node.Status.NodeInfo.OSImage,
		KernelVersion:    node.Status.NodeInfo.KernelVersion,
		Architecture:     node.Status.NodeInfo.Architecture,
		ContainerRuntime: node.Status.NodeInfo.ContainerRuntimeVersion,
		Hardware:         nodeHardware(node),
		Serial:           node.Annotations[AnnotationSerial],
		HardwareInfo:     nodeHardwareInfo(node),
		Unschedulable:    node.Spec.Unschedulable,
		Labels:           node.Labels,
	}

	s.Roles = nodeRoles(node)
	for _, r := range s.Roles {
		if r == "control-plane" || r == "master" {
			s.ControlPlane = true
		}
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

// nodeRoles extracts a node's roles from its labels. Roles come from
// node-role.kubernetes.io/<role> labels (the value is usually empty or "true")
// and the legacy kubernetes.io/role label. A node with no role label — the
// normal case for a k3s agent — is reported as ["worker"] so the UI can always
// show a definite master/worker type.
func nodeRoles(node corev1.Node) []string {
	const rolePrefix = "node-role.kubernetes.io/"
	seen := make(map[string]struct{})
	roles := make([]string, 0, 2)
	add := func(role string) {
		role = strings.TrimSpace(role)
		if role == "" {
			return
		}
		if _, ok := seen[role]; ok {
			return
		}
		seen[role] = struct{}{}
		roles = append(roles, role)
	}

	for label, value := range node.Labels {
		if strings.HasPrefix(label, rolePrefix) {
			if role := strings.TrimPrefix(label, rolePrefix); role != "" {
				add(role)
			} else {
				add(value)
			}
		}
	}
	add(node.Labels["kubernetes.io/role"])

	sortStrings(roles)
	if len(roles) == 0 {
		return []string{"worker"}
	}
	return roles
}

// Keys the KubePilot node agent (`kubepilot node-agent`) writes onto every
// node, and the dashboard reads back. Kubernetes has no field for any of this:
// status.nodeInfo stops at the OS and kernel.
const (
	// AnnotationHardware is the machine's vendor and model, e.g. "HP ZBook 17 G5".
	AnnotationHardware = "kubepilot.io/hardware"
	// AnnotationSerial is the chassis serial — the only thing that tells two
	// identical models apart.
	AnnotationSerial = "kubepilot.io/serial"
	// AnnotationHWPrefix namespaces the rest of the physical facts:
	// kubepilot.io/hw.cpu, hw.memory, hw.disks, hw.bios, hw.chassis, ...
	AnnotationHWPrefix = "kubepilot.io/hw."
	// LabelLANIP is the node's real physical LAN IP. It exists because k3s on a
	// WireGuard mesh reports only the tunnel address as the internal IP.
	LabelLANIP = "kubepilot.io/lan-ip"
)

// nodeHardware reports the machine's vendor and model. The KubePilot node
// agent reads it from DMI and writes the kubepilot.io/hardware annotation;
// on cloud nodes the instance-type label is the closest equivalent.
func nodeHardware(node corev1.Node) string {
	for _, v := range []string{
		node.Annotations[AnnotationHardware],
		node.Labels[AnnotationHardware],
		node.Labels["node.kubernetes.io/instance-type"],
		node.Labels["beta.kubernetes.io/instance-type"],
	} {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

// nodeHardwareInfo pulls the kubepilot.io/hw.* annotations off a node, stripped
// of their prefix. Returns nil when the node agent has never run on that node,
// so the dashboard can tell "no data" from "collected nothing".
func nodeHardwareInfo(node corev1.Node) map[string]string {
	var info map[string]string
	for key, value := range node.Annotations {
		name, ok := strings.CutPrefix(key, AnnotationHWPrefix)
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		if info == nil {
			info = make(map[string]string)
		}
		info[name] = value
	}
	return info
}
