import SwiftUI

struct DashboardView: View {
    @Environment(AppState.self) private var appState
    @State private var viewModel = DashboardViewModel()

    private let columns = [GridItem(.flexible()), GridItem(.flexible())]

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 20) {
                    dashboardBrandHeader
                    clusterHeader
                    healthScoreCard
                    metricsGrid
                    if !viewModel.insights.isEmpty {
                        insightsSection
                    }
                    if !viewModel.problemPods.isEmpty {
                        problemPodsSection
                    }
                    clusterNodesSection
                }
                .padding()
            }
            .background(Theme.background)
            .navigationTitle("")
            .navigationBarTitleDisplayMode(.inline)
            .toolbarBackground(Theme.brandBg.opacity(0.95), for: .navigationBar)
            .toolbarBackground(.visible, for: .navigationBar)
            .toolbar { toolbarContent }
            .refreshable { await viewModel.refresh(namespace: appState.clusterManager.selectedNamespace) }
            .task(id: appState.clusterManager.selectedNamespace) {
                await viewModel.refresh(namespace: appState.clusterManager.selectedNamespace)
            }
            .navigationDestination(for: DashboardDestination.self) { dest in
                switch dest {
                case .pods(let filter):
                    PodsListView(initialFilter: filter)
                case .nodes:
                    NodesListView()
                case .events:
                    AlertsView()
                case .pod(let ns, let name):
                    PodDetailView(namespace: ns, podName: name)
                }
            }
        }
    }

    private var dashboardBrandHeader: some View {
        HStack(alignment: .center, spacing: 12) {
            KubePilotMark(size: 40)
            KubePilotWordmark(size: .nav)
            Spacer()
        }
        .padding(.top, 4)
    }

    private var clusterHeader: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(appState.authManager.activeAccount?.displayName ?? "Cluster")
                .font(.headline)
            Text(appState.clusterManager.activeContextName)
                .font(.caption)
                .foregroundStyle(Theme.muted)
        }
    }

    private var healthScoreCard: some View {
        HStack {
            VStack(alignment: .leading, spacing: 6) {
                Text("Cluster Health")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .textCase(.uppercase)
                Text(viewModel.healthScore.label)
                    .font(.system(.largeTitle, design: .rounded, weight: .bold))
                    .foregroundStyle(Theme.healthColor(for: viewModel.healthScore))
            }
            Spacer()
            if viewModel.isLoading {
                ProgressView()
            }
        }
        .padding()
        .background(Theme.surface, in: RoundedRectangle(cornerRadius: 16, style: .continuous))
    }

    private var metricsGrid: some View {
        LazyVGrid(columns: columns, spacing: 12) {
            NavigationLink(value: DashboardDestination.pods(.healthy)) {
                MetricCard(title: "Healthy Pods", value: "\(viewModel.healthyPods)", tint: Theme.success)
            }
            .buttonStyle(.plain)

            NavigationLink(value: DashboardDestination.pods(.failing)) {
                MetricCard(title: "Failing Pods", value: "\(viewModel.failingPods)", tint: Theme.danger)
            }
            .buttonStyle(.plain)

            NavigationLink(value: DashboardDestination.pods(.crashloop)) {
                MetricCard(title: "CrashLoop", value: "\(viewModel.crashloopPods)", tint: Theme.danger)
            }
            .buttonStyle(.plain)

            NavigationLink(value: DashboardDestination.pods(.pending)) {
                MetricCard(title: "Pending", value: "\(viewModel.pendingPods)", tint: Theme.warning)
            }
            .buttonStyle(.plain)

            NavigationLink(value: DashboardDestination.nodes) {
                MetricCard(title: "Node Problems", value: "\(viewModel.nodeProblems)", tint: Theme.warning)
            }
            .buttonStyle(.plain)

            NavigationLink(value: DashboardDestination.events) {
                MetricCard(title: "Alerts", value: "\(viewModel.alertCount)", tint: Theme.accent)
            }
            .buttonStyle(.plain)
        }
    }

    private var insightsSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("AI Insights")
                .font(.headline)
            ForEach(viewModel.insights.prefix(3)) { insight in
                VStack(alignment: .leading, spacing: 6) {
                    HStack {
                        StatusBadge(text: insight.severity.uppercased(), color: severityColor(insight.severity))
                        Text(insight.title).font(.subheadline.weight(.semibold))
                    }
                    Text(insight.summary)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                .padding()
                .background(Theme.surface, in: RoundedRectangle(cornerRadius: 12, style: .continuous))
            }
        }
    }

    private var problemPodsSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Problem Pods")
                .font(.headline)
            ForEach(viewModel.problemPods.prefix(5)) { pod in
                NavigationLink(value: DashboardDestination.pod(pod.namespace, pod.name)) {
                    HStack {
                        VStack(alignment: .leading) {
                            Text(pod.name).font(.subheadline.weight(.medium))
                            Text(pod.namespace).font(.caption).foregroundStyle(.secondary)
                        }
                        Spacer()
                        StatusBadge(text: pod.reason.isEmpty ? pod.status : pod.reason, color: Theme.danger)
                    }
                    .padding()
                    .background(Theme.surface, in: RoundedRectangle(cornerRadius: 12, style: .continuous))
                }
                .buttonStyle(.plain)
            }
        }
    }

    private var clusterNodesSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                Text("Cluster Nodes")
                    .font(.headline)
                Spacer()
                if !viewModel.clusterNodes.isEmpty {
                    NavigationLink(value: DashboardDestination.nodes) {
                        Text("View all")
                            .font(.caption)
                    }
                }
            }

            if viewModel.clusterNodes.isEmpty {
                Text("No nodes reported.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding()
                    .background(Theme.surface, in: RoundedRectangle(cornerRadius: 12, style: .continuous))
            } else {
                ForEach(viewModel.clusterNodes) { node in
                    DashboardNodeHealthRow(node: node)
                }
            }
        }
    }

    @ToolbarContentBuilder
    private var toolbarContent: some ToolbarContent {
        ToolbarItem(placement: .principal) {
            Text("Dashboard")
                .font(.headline)
                .foregroundStyle(Theme.textPrimary)
        }
        ToolbarItem(placement: .topBarTrailing) {
            NamespacePicker()
        }
    }

    private func severityColor(_ severity: String) -> Color {
        switch severity.lowercased() {
        case "critical", "high": Theme.danger
        case "medium": Theme.warning
        default: Theme.accent
        }
    }
}

enum DashboardDestination: Hashable {
    case pods(PodFilter)
    case nodes
    case events
    case pod(String, String)
}

@MainActor
@Observable
final class DashboardViewModel {
    var summary: ClusterTroubleshootingSummary?
    var pods: [PodSummary] = []
    var anomalies: [Anomaly] = []
    var isLoading = false
    var errorMessage: String?

    var healthScore: HealthScore { summary?.overallHealth ?? .healthy }
    var insights: [TroubleshootingInsight] { summary?.insights ?? [] }
    var problemPods: [ProblemPod] { summary?.problemPods ?? [] }
    var clusterNodes: [NodeHealthRow] { summary?.nodes ?? [] }
    var crashloopPods: Int { summary?.healthSummary.crashloopPods ?? 0 }
    var pendingPods: Int { summary?.healthSummary.pendingPods ?? 0 }
    var nodeProblems: Int { summary?.healthSummary.notReadyNodes ?? 0 }
    var alertCount: Int { anomalies.count + (summary?.healthSummary.warningEvents ?? 0) }

    var healthyPods: Int {
        pods.filter { $0.ready && $0.reason.isEmpty }.count
    }

    var failingPods: Int {
        pods.filter { !$0.ready || !$0.reason.isEmpty }.count
    }

    func refresh(namespace: String) async {
        isLoading = true
        errorMessage = nil
        defer { isLoading = false }
        do {
            async let summaryTask = KubePilotService.shared.fetchTroubleshootingSummary(namespace: namespace)
            async let podsTask = KubePilotService.shared.fetchPods(namespace: namespace)
            async let anomaliesTask = KubePilotService.shared.fetchAnomalies()
            summary = try await summaryTask
            pods = try await podsTask
            anomalies = try await anomaliesTask
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}

struct DashboardNodeHealthRow: View {
    let node: NodeHealthRow

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text(node.name)
                    .font(.subheadline.weight(.semibold))
                Spacer()
                StatusBadge(
                    text: node.ready ? "Ready" : "NotReady",
                    color: node.ready ? Theme.success : Theme.danger
                )
            }

            NodeIPLabels(health: node)

            HStack(spacing: 12) {
                if let cpu = node.cpuUsagePercent {
                    Text("CPU \(cpu)%")
                } else {
                    Text("CPU \(node.cpuCapacity)")
                }
                if let mem = node.memoryUsagePercent {
                    Text("Mem \(mem)%")
                } else {
                    Text("Mem \(node.memoryCapacity)")
                }
            }
            .font(.caption2)
            .foregroundStyle(.tertiary)

            if node.diskPressure || node.memoryPressure || node.pidPressure {
                HStack(spacing: 6) {
                    if node.memoryPressure { StatusBadge(text: "Mem pressure", color: Theme.warning) }
                    if node.diskPressure { StatusBadge(text: "Disk pressure", color: Theme.warning) }
                    if node.pidPressure { StatusBadge(text: "PID pressure", color: Theme.warning) }
                }
            }
        }
        .padding()
        .background(Theme.surface, in: RoundedRectangle(cornerRadius: 12, style: .continuous))
    }
}

struct NamespacePicker: View {
    @Environment(AppState.self) private var appState

    var body: some View {
        Menu {
            Button("All Namespaces") {
                appState.clusterManager.selectNamespace("")
            }
            ForEach(appState.clusterManager.namespaces, id: \.name) { ns in
                Button(ns.name) {
                    appState.clusterManager.selectNamespace(ns.name)
                }
            }
        } label: {
            Label(
                appState.clusterManager.selectedNamespace.isEmpty ? "All" : appState.clusterManager.selectedNamespace,
                systemImage: "line.3.horizontal.decrease.circle"
            )
        }
        .task { try? await appState.clusterManager.refresh() }
    }
}
