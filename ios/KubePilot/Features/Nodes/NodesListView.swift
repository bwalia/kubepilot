import SwiftUI

struct NodesListView: View {
    @State private var nodes: [NodeSummary] = []
    @State private var isLoading = false
    @State private var errorMessage: String?

    var body: some View {
        Group {
            if isLoading && nodes.isEmpty {
                LoadingOverlay(message: "Loading nodes…")
            } else if let errorMessage, nodes.isEmpty {
                ErrorBanner(message: errorMessage) { Task { await load() } }
            } else if nodes.isEmpty {
                EmptyStateView(
                    title: "No Nodes",
                    systemImage: "server.rack",
                    description: "The cluster did not return any node records."
                )
            } else {
                List(nodes) { node in
                    NavigationLink {
                        NodeDetailView(node: node)
                    } label: {
                        NodeRow(node: node)
                    }
                    .listRowBackground(Theme.surface)
                }
                .themedList()
            }
        }
        .navigationTitle("Nodes")
        .navigationBarTitleDisplayMode(.large)
        .themedScreen()
        .refreshable { await load() }
        .task { await load() }
    }

    private func load() async {
        isLoading = true
        errorMessage = nil
        defer { isLoading = false }
        do {
            nodes = try await KubePilotService.shared.fetchNodes()
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}

struct NodeRow: View {
    let node: NodeSummary

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.spacingXS) {
            HStack {
                Text(node.name)
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(Theme.textPrimary)
                Spacer()
                StatusBadge(
                    text: node.ready ? "Ready" : "NotReady",
                    color: node.ready ? Theme.success : Theme.danger
                )
            }
            NodeRoleBadge(node: node)
            if !node.allIPs.isEmpty {
                NodeIPLabels(node: node)
            }
            HStack(spacing: Theme.spacingSM) {
                Label("CPU \(node.cpuCapacity)", systemImage: "cpu")
                Label("Mem \(node.memoryCapacity)", systemImage: "memorychip")
            }
            .font(.caption2)
            .foregroundStyle(Theme.muted)
            .labelStyle(.titleAndIcon)
        }
        .padding(.vertical, Theme.spacingXS)
    }
}

struct NodeDetailView: View {
    let node: NodeSummary

    var body: some View {
        List {
            Section("Status") {
                LabeledContent("Ready", value: node.ready ? "Yes" : "No")
                LabeledContent("Role") {
                    NodeRoleBadge(node: node)
                }
                LabeledContent("Kubelet", value: node.kubeletVersion)
                LabeledContent("Unschedulable", value: node.unschedulable ? "Yes" : "No")
            }
            .listRowBackground(Theme.surface)

            Section("IP Addresses") {
                NodeIPLabels(node: node)
            }
            .listRowBackground(Theme.surface)

            Section("Capacity") {
                LabeledContent("CPU", value: node.cpuCapacity)
                LabeledContent("Memory", value: node.memoryCapacity)
            }
            .listRowBackground(Theme.surface)

            Section("Pressure") {
                pressureRow("Memory", node.memoryPressure)
                pressureRow("Disk", node.diskPressure)
                pressureRow("PID", node.pidPressure)
            }
            .listRowBackground(Theme.surface)
        }
        .themedList()
        .navigationTitle(node.name)
        .themedScreen()
    }

    private func pressureRow(_ label: String, _ active: Bool) -> some View {
        LabeledContent(label) {
            StatusBadge(text: active ? "Active" : "None", color: active ? Theme.warning : Theme.success)
        }
    }
}
