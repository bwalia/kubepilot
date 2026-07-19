package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestToNodeSummaryCollectsAllIPs(t *testing.T) {
	node := corev1.Node{}
	node.Name = "worker-1"
	node.Status.Addresses = []corev1.NodeAddress{
		{Type: corev1.NodeInternalIP, Address: "10.8.0.2"},
		{Type: corev1.NodeInternalIP, Address: "192.168.1.50"},
		{Type: corev1.NodeExternalIP, Address: "203.0.113.10"},
		{Type: corev1.NodeHostName, Address: "worker-1"},
	}

	summary := toNodeSummary(node)

	if len(summary.LANIPs) != 1 || summary.LANIPs[0] != "192.168.1.50" {
		t.Fatalf("LANIPs = %v, want [192.168.1.50]", summary.LANIPs)
	}
	if len(summary.WANIPs) != 1 || summary.WANIPs[0] != "203.0.113.10" {
		t.Fatalf("WANIPs = %v, want [203.0.113.10]", summary.WANIPs)
	}
	if len(summary.TunnelIPs) != 1 || summary.TunnelIPs[0] != "10.8.0.2" {
		t.Fatalf("TunnelIPs = %v, want [10.8.0.2]", summary.TunnelIPs)
	}
	wantAll := []string{"10.8.0.2", "192.168.1.50", "203.0.113.10"}
	if len(summary.IPs) != len(wantAll) {
		t.Fatalf("got %d IPs, want %d: %v", len(summary.IPs), len(wantAll), summary.IPs)
	}
}

func TestClassifyNodeIPsFromK3sAnnotations(t *testing.T) {
	node := corev1.Node{}
	node.Name = "k3s-agent"
	node.Annotations = map[string]string{
		annoK3sInternalIP:   "192.168.1.50",
		annoK3sExternalIP:   "203.0.113.10",
		annoFlannelPublicIP: "10.8.0.2",
	}
	node.Status.Addresses = []corev1.NodeAddress{
		{Type: corev1.NodeInternalIP, Address: "10.8.0.2"},
	}

	summary := toNodeSummary(node)

	if len(summary.LANIPs) != 1 || summary.LANIPs[0] != "192.168.1.50" {
		t.Fatalf("LANIPs = %v, want [192.168.1.50]", summary.LANIPs)
	}
	if len(summary.WANIPs) != 1 || summary.WANIPs[0] != "203.0.113.10" {
		t.Fatalf("WANIPs = %v, want [203.0.113.10]", summary.WANIPs)
	}
	if len(summary.TunnelIPs) != 1 || summary.TunnelIPs[0] != "10.8.0.2" {
		t.Fatalf("TunnelIPs = %v, want [10.8.0.2]", summary.TunnelIPs)
	}
}

// A WireGuard-meshed k3s node: k3s reports the WireGuard address (10.x) as its
// internal IP, but the physical LAN address (192.168.x) must still show as LAN.
func TestClassifyNodeIPsWireGuardInternalIP(t *testing.T) {
	node := corev1.Node{}
	node.Name = "homelab-1"
	node.Annotations = map[string]string{
		annoK3sInternalIP: "10.8.0.3", // WireGuard address
	}
	node.Status.Addresses = []corev1.NodeAddress{
		{Type: corev1.NodeInternalIP, Address: "10.8.0.3"},
		{Type: corev1.NodeInternalIP, Address: "192.168.1.42"},
	}

	summary := toNodeSummary(node)

	if len(summary.LANIPs) != 1 || summary.LANIPs[0] != "192.168.1.42" {
		t.Fatalf("LANIPs = %v, want [192.168.1.42]", summary.LANIPs)
	}
	if len(summary.TunnelIPs) != 1 || summary.TunnelIPs[0] != "10.8.0.3" {
		t.Fatalf("TunnelIPs = %v, want [10.8.0.3]", summary.TunnelIPs)
	}
}

// An AWS node whose only private address is a VPC IP (10.x) must show it as LAN,
// not tunnel — there is no physical LAN address to distinguish it from.
func TestClassifyNodeIPsAWSVPCOnly(t *testing.T) {
	node := corev1.Node{}
	node.Name = "aws-worker"
	node.Status.Addresses = []corev1.NodeAddress{
		{Type: corev1.NodeInternalIP, Address: "10.0.4.17"},
		{Type: corev1.NodeExternalIP, Address: "52.14.9.200"},
	}

	summary := toNodeSummary(node)

	if len(summary.LANIPs) != 1 || summary.LANIPs[0] != "10.0.4.17" {
		t.Fatalf("LANIPs = %v, want [10.0.4.17]", summary.LANIPs)
	}
	if len(summary.WANIPs) != 1 || summary.WANIPs[0] != "52.14.9.200" {
		t.Fatalf("WANIPs = %v, want [52.14.9.200]", summary.WANIPs)
	}
	if len(summary.TunnelIPs) != 0 {
		t.Fatalf("TunnelIPs = %v, want none", summary.TunnelIPs)
	}
}

// An AWS node on a default VPC (172.31.x) plus a WireGuard mesh (10.x): the VPC
// address is the LAN, the WireGuard address is the tunnel.
func TestClassifyNodeIPsAWSVPCWithWireGuard(t *testing.T) {
	node := corev1.Node{}
	node.Name = "aws-mesh"
	node.Status.Addresses = []corev1.NodeAddress{
		{Type: corev1.NodeInternalIP, Address: "172.31.20.5"},
		{Type: corev1.NodeInternalIP, Address: "10.8.0.9"},
	}

	summary := toNodeSummary(node)

	if len(summary.LANIPs) != 1 || summary.LANIPs[0] != "172.31.20.5" {
		t.Fatalf("LANIPs = %v, want [172.31.20.5]", summary.LANIPs)
	}
	if len(summary.TunnelIPs) != 1 || summary.TunnelIPs[0] != "10.8.0.9" {
		t.Fatalf("TunnelIPs = %v, want [10.8.0.9]", summary.TunnelIPs)
	}
}

func TestNodeRolesControlPlane(t *testing.T) {
	node := corev1.Node{}
	node.Name = "server-1"
	node.Labels = map[string]string{
		"node-role.kubernetes.io/control-plane": "true",
		"node-role.kubernetes.io/master":        "true",
	}

	summary := toNodeSummary(node)

	if !summary.ControlPlane {
		t.Fatalf("ControlPlane = false, want true")
	}
	want := []string{"control-plane", "master"}
	if len(summary.Roles) != len(want) || summary.Roles[0] != want[0] || summary.Roles[1] != want[1] {
		t.Fatalf("Roles = %v, want %v", summary.Roles, want)
	}
}

func TestNodeRolesAgentDefaultsToWorker(t *testing.T) {
	node := corev1.Node{}
	node.Name = "agent-1"
	node.Labels = map[string]string{"kubernetes.io/hostname": "agent-1"}

	summary := toNodeSummary(node)

	if summary.ControlPlane {
		t.Fatalf("ControlPlane = true, want false")
	}
	if len(summary.Roles) != 1 || summary.Roles[0] != "worker" {
		t.Fatalf("Roles = %v, want [worker]", summary.Roles)
	}
}

func TestClassifyNodeIPsFlannelExternalOverwrite(t *testing.T) {
	node := corev1.Node{}
	node.Annotations = map[string]string{
		annoK3sInternalIP:            "10.128.15.197",
		annoK3sExternalIP:           "34.171.97.10",
		annoFlannelPublicIP:         "10.128.15.197",
		annoFlannelPublicIPOverwrite: "34.171.97.10",
	}
	node.Status.Addresses = []corev1.NodeAddress{
		{Type: corev1.NodeInternalIP, Address: "10.128.15.197"},
		{Type: corev1.NodeExternalIP, Address: "34.171.97.10"},
	}

	summary := toNodeSummary(node)

	if len(summary.LANIPs) != 1 || summary.LANIPs[0] != "10.128.15.197" {
		t.Fatalf("LANIPs = %v", summary.LANIPs)
	}
	if len(summary.WANIPs) != 1 || summary.WANIPs[0] != "34.171.97.10" {
		t.Fatalf("WANIPs = %v", summary.WANIPs)
	}
	if len(summary.TunnelIPs) != 0 {
		t.Fatalf("TunnelIPs = %v, want none", summary.TunnelIPs)
	}
}
