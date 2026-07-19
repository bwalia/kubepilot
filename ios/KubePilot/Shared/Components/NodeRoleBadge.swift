import SwiftUI

/// Shows a node's role (control-plane / master vs worker) as coloured badges,
/// so the master/worker type is always clear in the cluster nodes view.
struct NodeRoleBadge: View {
    let roles: [String]
    let isControlPlane: Bool

    init(node: NodeSummary) {
        roles = node.displayRoles
        isControlPlane = node.isControlPlane
    }

    init(health: NodeHealthRow) {
        roles = health.displayRoles
        isControlPlane = health.isControlPlane
    }

    var body: some View {
        HStack(spacing: 6) {
            StatusBadge(text: primaryRole, color: isControlPlane ? Theme.accent : Theme.muted)
            ForEach(extraRoles, id: \.self) { role in
                StatusBadge(text: role, color: Theme.muted)
            }
        }
    }

    // Control-plane nodes carry both "control-plane" and "master" labels; collapse
    // them into one clear badge and surface any additional roles separately.
    private var primaryRole: String {
        isControlPlane ? "control-plane" : (roles.first ?? "worker")
    }

    private var extraRoles: [String] {
        roles.filter { $0 != primaryRole && $0 != "control-plane" && $0 != "master" }
    }
}
