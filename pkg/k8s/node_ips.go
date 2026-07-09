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

func classifyNodeIPs(node corev1.Node) nodeIPBuckets {
	lan := newIPSet()
	wan := newIPSet()
	tunnel := newIPSet()

	k3sInternal := firstIP(node.Annotations[annoK3sInternalIP])
	k3sExternal := parseIPList(node.Annotations[annoK3sExternalIP])
	flannelPublic := firstIP(node.Annotations[annoFlannelPublicIP])
	flannelOverwrite := firstIP(node.Annotations[annoFlannelPublicIPOverwrite])
	if flannelOverwrite == "" {
		flannelOverwrite = firstIP(node.Annotations[annoFlannelPublicIPv6Overwrite])
	}

	if k3sInternal != "" {
		lan.add(k3sInternal)
	}
	for _, ip := range k3sExternal {
		wan.add(ip)
	}
	if flannelOverwrite != "" {
		wan.add(flannelOverwrite)
	}

	for _, addr := range node.Status.Addresses {
		ip := strings.TrimSpace(addr.Address)
		if ip == "" || net.ParseIP(ip) == nil {
			continue
		}
		switch addr.Type {
		case corev1.NodeExternalIP:
			wan.add(ip)
		case corev1.NodeInternalIP:
			classifyInternalAddress(ip, k3sInternal, flannelPublic, lan, wan, tunnel)
		}
	}

	if flannelPublic != "" {
		classifyFlannelPublicIP(flannelPublic, k3sInternal, lan, wan, tunnel)
	}

	rebalancePrivateInternalIPs(k3sInternal, node.Status.Addresses, lan, tunnel)

	return nodeIPBuckets{
		lan:    lan.values(),
		wan:    wan.values(),
		tunnel: tunnel.values(),
	}
}

func classifyInternalAddress(ip, k3sInternal, flannelPublic string, lan, wan, tunnel *ipSet) {
	if k3sInternal != "" && ip == k3sInternal {
		lan.add(ip)
		return
	}
	if isPublicIP(ip) {
		wan.add(ip)
		return
	}
	if k3sInternal != "" && ip != k3sInternal {
		// Extra private internal address while k3s reports a different LAN IP — typical WireGuard / flannel endpoint.
		tunnel.add(ip)
		return
	}
	if flannelPublic != "" && ip == flannelPublic && k3sInternal != "" && ip != k3sInternal {
		tunnel.add(ip)
		return
	}
	if isPrivateIP(ip) {
		lan.add(ip)
	}
}

func rebalancePrivateInternalIPs(k3sInternal string, addresses []corev1.NodeAddress, lan, tunnel *ipSet) {
	if k3sInternal != "" {
		return
	}
	var tenDot []string
	var homeLAN []string
	for _, addr := range addresses {
		if addr.Type != corev1.NodeInternalIP {
			continue
		}
		ip := strings.TrimSpace(addr.Address)
		if !isPrivateIP(ip) {
			continue
		}
		if strings.HasPrefix(ip, "10.") {
			tenDot = append(tenDot, ip)
		} else {
			homeLAN = append(homeLAN, ip)
		}
	}
	if len(tenDot) == 0 || len(homeLAN) == 0 {
		return
	}
	for _, ip := range tenDot {
		lan.remove(ip)
		tunnel.add(ip)
	}
	for _, ip := range homeLAN {
		lan.add(ip)
	}
}

func classifyFlannelPublicIP(ip, k3sInternal string, lan, wan, tunnel *ipSet) {
	if k3sInternal != "" && ip == k3sInternal {
		lan.add(ip)
		return
	}
	if isPublicIP(ip) {
		wan.add(ip)
		return
	}
	if k3sInternal != "" && ip != k3sInternal {
		tunnel.add(ip)
		return
	}
	if isPrivateIP(ip) {
		lan.add(ip)
	}
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

func (s *ipSet) remove(ip string) {
	delete(s.seen, strings.TrimSpace(ip))
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
