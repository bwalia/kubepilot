package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// A WireGuard-only node where k3s reports just the WireGuard 10.x as internal IP:
// the node agent's kubepilot.io/lan-ip label is authoritative, so the
// labeled address shows as LAN and the WireGuard address is demoted to Tunnel.
func TestClassifyNodeIPsLANIPLabelOverride(t *testing.T) {
	node := corev1.Node{}
	node.Name = "wg-only"
	node.Labels = map[string]string{labelLANIP: "192.168.1.140"}
	node.Annotations = map[string]string{
		annoK3sInternalIP: "10.8.0.4", // WireGuard address, all k8s knows
	}
	node.Status.Addresses = []corev1.NodeAddress{
		{Type: corev1.NodeInternalIP, Address: "10.8.0.4"},
	}

	summary := toNodeSummary(node)

	if len(summary.LANIPs) != 1 || summary.LANIPs[0] != "192.168.1.140" {
		t.Fatalf("LANIPs = %v, want [192.168.1.140]", summary.LANIPs)
	}
	if len(summary.TunnelIPs) != 1 || summary.TunnelIPs[0] != "10.8.0.4" {
		t.Fatalf("TunnelIPs = %v, want [10.8.0.4]", summary.TunnelIPs)
	}
	if summary.Labels[labelLANIP] != "192.168.1.140" {
		t.Fatalf("Labels[%q] = %q, want 192.168.1.140", labelLANIP, summary.Labels[labelLANIP])
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
		annoK3sExternalIP:            "34.171.97.10",
		annoFlannelPublicIP:          "10.128.15.197",
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

func TestNodeHardwareSources(t *testing.T) {
	cases := []struct {
		name string
		node corev1.Node
		want string
	}{
		{
			name: "annotation wins",
			node: corev1.Node{ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{AnnotationHardware: "HP ProLiant DL380 Gen9"},
				Labels:      map[string]string{"node.kubernetes.io/instance-type": "k3s"},
			}},
			want: "HP ProLiant DL380 Gen9",
		},
		{
			name: "falls back to instance type",
			node: corev1.Node{ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"node.kubernetes.io/instance-type": "t3.medium"},
			}},
			want: "t3.medium",
		},
		{name: "unknown hardware", node: corev1.Node{}, want: ""},
	}

	for _, tc := range cases {
		if got := nodeHardware(tc.node); got != tc.want {
			t.Errorf("%s: nodeHardware() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestToNodeSummaryCarriesSystemInfo(t *testing.T) {
	node := corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name:        "worker-1",
		Annotations: map[string]string{AnnotationHardware: "Dell Inc. OptiPlex 7050", AnnotationSerial: "ABC123"},
	}}
	node.Status.NodeInfo.OSImage = "Ubuntu 22.04.4 LTS"
	node.Status.NodeInfo.KernelVersion = "5.15.0-100-generic"
	node.Status.NodeInfo.Architecture = "amd64"
	node.Status.NodeInfo.ContainerRuntimeVersion = "containerd://1.7.11"

	s := toNodeSummary(node)

	if s.Hardware != "Dell Inc. OptiPlex 7050" || s.Serial != "ABC123" {
		t.Fatalf("hardware = %q / serial = %q", s.Hardware, s.Serial)
	}
	if s.OSImage == "" || s.KernelVersion == "" || s.Architecture == "" || s.ContainerRuntime == "" {
		t.Fatalf("system info missing: %+v", s)
	}
}

func TestNodeHardwareInfoStripsPrefix(t *testing.T) {
	node := corev1.Node{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
		AnnotationHWPrefix + "cpu":    "Intel(R) Xeon(R) E-2176M CPU @ 2.70GHz",
		AnnotationHWPrefix + "memory": "15.5 GB",
		AnnotationHWPrefix + "blank":  "  ",
		"k3s.io/node-args":            "[\"agent\"]",
	}}}

	info := toNodeSummary(node).HardwareInfo

	if len(info) != 2 || info["cpu"] == "" || info["memory"] != "15.5 GB" {
		t.Fatalf("HardwareInfo = %v, want only the two non-blank hw.* keys", info)
	}
	if toNodeSummary(corev1.Node{}).HardwareInfo != nil {
		t.Error("HardwareInfo should be nil when the node agent has never run")
	}
}
