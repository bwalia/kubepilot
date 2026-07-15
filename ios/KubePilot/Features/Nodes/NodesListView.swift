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
            } else {
                List(nodes) { node in
                    NavigationLink {
                        NodeDetailView(node: node)
                    } label: {
                        NodeRow(node: node)
                    }
                }
            }
        }
        .navigationTitle("Nodes")
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
        VStack(alignment: .leading, spacing: 6) {
            HStack {
                Text(node.name).font(.subheadline.weight(.semibold))
                Spacer()
                StatusBadge(
                    text: node.ready ? "Ready" : "NotReady",
                    color: node.ready ? Theme.success : Theme.danger
                )
            }
            if !node.allIPs.isEmpty {
                NodeIPLabels(node: node)
            }
            HStack(spacing: 8) {
                Text("CPU \(node.cpuCapacity)")
                Text("Mem \(node.memoryCapacity)")
            }
            .font(.caption2)
            .foregroundStyle(.tertiary)
        }
        .padding(.vertical, 4)
    }
}

struct NodeDetailView: View {
    let node: NodeSummary

    var body: some View {
        List {
            Section("Status") {
                LabeledContent("Ready", value: node.ready ? "Yes" : "No")
                LabeledContent("Kubelet", value: node.kubeletVersion)
                LabeledContent("Unschedulable", value: node.unschedulable ? "Yes" : "No")
            }
            Section("IP Addresses") {
                NodeIPLabels(node: node)
            }
            Section("Capacity") {
                LabeledContent("CPU", value: node.cpuCapacity)
                LabeledContent("Memory", value: node.memoryCapacity)
            }
            Section("Pressure") {
                pressureRow("Memory", node.memoryPressure)
                pressureRow("Disk", node.diskPressure)
                pressureRow("PID", node.pidPressure)
            }
        }
        .navigationTitle(node.name)
    }

    private func pressureRow(_ label: String, _ active: Bool) -> some View {
        LabeledContent(label) {
            StatusBadge(text: active ? "Active" : "None", color: active ? Theme.warning : Theme.success)
        }
    }
}
