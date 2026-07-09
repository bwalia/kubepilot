import SwiftUI

struct NodeIPLabels: View {
    let lanIPs: [String]
    let wanIPs: [String]
    let tunnelIPs: [String]
    let fallbackIPs: [String]

    init(node: NodeSummary) {
        lanIPs = node.lanIPs ?? []
        wanIPs = node.wanIPs ?? []
        tunnelIPs = node.tunnelIPs ?? []
        fallbackIPs = node.allIPs
    }

    init(health: NodeHealthRow) {
        lanIPs = health.lanIPs ?? []
        wanIPs = health.wanIPs ?? []
        tunnelIPs = health.tunnelIPs ?? []
        fallbackIPs = health.ips ?? []
    }

    var body: some View {
        if !lanIPs.isEmpty || !wanIPs.isEmpty || !tunnelIPs.isEmpty {
            VStack(alignment: .leading, spacing: 4) {
                ipRow(label: "LAN", ips: lanIPs)
                ipRow(label: "WAN", ips: wanIPs)
                ipRow(label: "Tunnel", ips: tunnelIPs)
            }
        } else if fallbackIPs.isEmpty {
            Text("—")
                .foregroundStyle(.secondary)
        } else {
            VStack(alignment: .leading, spacing: 2) {
                ForEach(fallbackIPs, id: \.self) { ip in
                    Text(ip).font(.caption.monospaced())
                }
            }
        }
    }

    @ViewBuilder
    private func ipRow(label: String, ips: [String]) -> some View {
        if !ips.isEmpty {
            HStack(alignment: .top, spacing: 6) {
                Text(label)
                    .font(.caption2.weight(.semibold))
                    .foregroundStyle(.secondary)
                    .frame(width: 44, alignment: .leading)
                Text(ips.joined(separator: ", "))
                    .font(.caption.monospaced())
            }
        }
    }
}
