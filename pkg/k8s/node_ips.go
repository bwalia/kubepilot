package k8s

import (
	"net"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const (
	annoK3sInternalIP              = "k3s.io/internal-ip"
	annoK3sExternalIP              = "k3s.io/external-ip"
	annoFlannelPublicIP            = "flannel.alpha.coreos.com/public-ip"
	annoFlannelPublicIPOverwrite   = "flannel.alpha.coreos.com/public-ip-overwrite"
	annoFlannelPublicIPv6Overwrite = "flannel.alpha.coreos.com/public-ipv6-overwrite"
)

type nodeIPBuckets struct {
	lan    []string
	wan    []string
	tunnel []string
}

// classifyNodeIPs sorts a node's addresses into LAN / WAN / Tunnel buckets by IP
// range rather than by which address k3s happens to report as its internal IP.
// That distinction matters on WireGuard-meshed clusters, where k3s often sets
// its internal IP to the WireGuard address (a 10.x tunnel) while the real LAN
// address (192.168.x) is just another entry — the old "everything that isn't the
// k3s internal IP is a tunnel" rule buried the physical LAN IP under Tunnel.
//
// Rules:
//   - 192.168.0.0/16 and 172.16.0.0/12 are always LAN (home/office LAN, VPC).
//   - 10.0.0.0/8 is LAN when it is the node's only private range (AWS VPC), but
//     Tunnel when a real LAN address coexists (WireGuard / flannel overlay).
//   - 100.64.0.0/10 (CGNAT / Tailscale) is always Tunnel.
//   - Public addresses, and k3s/flannel public-IP annotations, are WAN.
func classifyNodeIPs(node corev1.Node) nodeIPBuckets {
	wan := newIPSet()
	overlay := newIPSet() // forced tunnel: CGNAT / Tailscale
	private := newIPSet() // LAN-vs-tunnel decided by range below

	// consider routes a single address to WAN, forced-tunnel, or the private pool.
	consider := func(raw string) {
		ip := strings.TrimSpace(raw)
		if ip == "" || net.ParseIP(ip) == nil {
			return
		}
		switch {
		case isCGNATIP(ip):
			overlay.add(ip)
		case isPublicIP(ip):
			wan.add(ip)
		case isPrivateIP(ip):
			private.add(ip)
		}
	}

	for _, addr := range node.Status.Addresses {
		switch addr.Type {
		case corev1.NodeExternalIP, corev1.NodeInternalIP:
			consider(addr.Address)
		}
	}

	// k3s external and flannel public-ip-overwrite are the node's public identity.
	for _, ip := range parseIPList(node.Annotations[annoK3sExternalIP]) {
		consider(ip)
	}
	if fo := firstIP(node.Annotations[annoFlannelPublicIPOverwrite]); fo != "" {
		consider(fo)
	} else if fo := firstIP(node.Annotations[annoFlannelPublicIPv6Overwrite]); fo != "" {
		consider(fo)
	}
	// k3s internal-ip and flannel public-ip are per-node addresses; classify by range.
	consider(firstIP(node.Annotations[annoK3sInternalIP]))
	consider(firstIP(node.Annotations[annoFlannelPublicIP]))

	privateIPs := private.values()
	hasRealLAN := false
	for _, ip := range privateIPs {
		if isStrongLANIP(ip) {
			hasRealLAN = true
			break
		}
	}

	lan := newIPSet()
	tunnel := newIPSet()
	for _, ip := range privateIPs {
		switch {
		case isStrongLANIP(ip):
			lan.add(ip)
		case hasRealLAN:
			// A 10.x address alongside a real LAN address is an overlay endpoint.
			tunnel.add(ip)
		default:
			// Only 10.x private addresses present — this is the LAN (e.g. AWS VPC).
			lan.add(ip)
		}
	}
	for _, ip := range overlay.values() {
		tunnel.add(ip)
	}

	return nodeIPBuckets{
		lan:    lan.values(),
		wan:    wan.values(),
		tunnel: tunnel.values(),
	}
}

// isStrongLANIP reports whether ip is in a range that is unambiguously a physical
// LAN or cloud VPC (never a WireGuard/overlay endpoint by convention).
func isStrongLANIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, cidr := range []string{"192.168.0.0/16", "172.16.0.0/12"} {
		if _, network, err := net.ParseCIDR(cidr); err == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

// isCGNATIP reports whether ip is in the 100.64.0.0/10 shared-address range used
// by carrier-grade NAT and Tailscale — always an overlay/tunnel address here.
func isCGNATIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	_, network, err := net.ParseCIDR("100.64.0.0/10")
	return err == nil && network.Contains(ip)
}

func mergeNodeIPBuckets(b nodeIPBuckets) []string {
	all := newIPSet()
	for _, ip := range b.lan {
		all.add(ip)
	}
	for _, ip := range b.wan {
		all.add(ip)
	}
	for _, ip := range b.tunnel {
		all.add(ip)
	}
	return all.values()
}

func formatNodeIPSummary(b nodeIPBuckets) string {
	parts := make([]string, 0, 3)
	if len(b.lan) > 0 {
		parts = append(parts, "LAN: "+strings.Join(b.lan, ", "))
	}
	if len(b.wan) > 0 {
		parts = append(parts, "WAN: "+strings.Join(b.wan, ", "))
	}
	if len(b.tunnel) > 0 {
		parts = append(parts, "Tunnel: "+strings.Join(b.tunnel, ", "))
	}
	return strings.Join(parts, " | ")
}

func parseIPList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	out := make([]string, 0)
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' }) {
		ip := strings.TrimSpace(part)
		if ip != "" && net.ParseIP(ip) != nil {
			out = append(out, ip)
		}
	}
	return out
}

func firstIP(raw string) string {
	ips := parseIPList(raw)
	if len(ips) == 0 {
		return ""
	}
	return ips[0]
}

func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return false
	}
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"100.64.0.0/10", // CGNAT / Tailscale
	}
	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

func isPublicIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsPrivate() {
		return false
	}
	return true
}

type ipSet struct {
	seen map[string]struct{}
}

func newIPSet() *ipSet {
	return &ipSet{seen: make(map[string]struct{})}
}

func (s *ipSet) add(ip string) {
	ip = strings.TrimSpace(ip)
	if ip == "" || net.ParseIP(ip) == nil {
		return
	}
	s.seen[ip] = struct{}{}
}

func (s *ipSet) values() []string {
	out := make([]string, 0, len(s.seen))
	for ip := range s.seen {
		out = append(out, ip)
	}
	sortStrings(out)
	return out
}

func sortStrings(values []string) {
	for i := 0; i < len(values); i++ {
		for j := i + 1; j < len(values); j++ {
			if strings.Compare(values[i], values[j]) > 0 {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
}

// collectNodeIPs is kept for backward compatibility — returns the flat union of all addresses.
func collectNodeIPs(node corev1.Node) []string {
	return mergeNodeIPBuckets(classifyNodeIPs(node))
}
