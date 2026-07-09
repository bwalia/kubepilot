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
