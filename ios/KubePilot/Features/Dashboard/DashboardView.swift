import SwiftUI

struct DashboardView: View {
    @Environment(AppState.self) private var appState
    @State private var viewModel = DashboardViewModel()

    private let columns = [GridItem(.flexible()), GridItem(.flexible())]

    var body: some View {
        NavigationStack {
            ZStack {
                BrandScreenBackground()

                ScrollView {
                    VStack(alignment: .leading, spacing: Theme.spacingLG) {
                        ClusterContextBanner()

                        if let error = viewModel.errorMessage {
                            ErrorBanner(message: error) {
                                Task { await viewModel.refresh(namespace: appState.clusterManager.selectedNamespace) }
                            }
                        }

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
                    .padding(Theme.spacingMD)
                }
            }
            .navigationTitle("Dashboard")
            .navigationBarTitleDisplayMode(.large)
            .themedScreen()
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

    private var healthScoreCard: some View {
        SurfaceCard {
            HStack {
                VStack(alignment: .leading, spacing: Theme.spacingXS) {
                    Text("Cluster Health")
                        .font(Theme.Typography.cardLabel)
                        .foregroundStyle(Theme.muted)
                        .textCase(.uppercase)
                    Text(viewModel.healthScore.label)
                        .font(.system(.largeTitle, design: .rounded, weight: .bold))
                        .foregroundStyle(Theme.healthColor(for: viewModel.healthScore))
                        .contentTransition(.numericText())
                }
                Spacer()
                if viewModel.isLoading {
                    ProgressView()
                        .tint(Theme.accent)
                } else {
                    Image(systemName: healthSymbol)
                        .font(.title)
                        .foregroundStyle(Theme.healthColor(for: viewModel.healthScore))
                        .symbolRenderingMode(.hierarchical)
                        .symbolEffect(.pulse, options: .repeating, value: viewModel.healthScore)
                }
            }
        }
    }

    private var healthSymbol: String {
        switch viewModel.healthScore {
        case .healthy: "checkmark.seal.fill"
        case .degraded: "exclamationmark.triangle.fill"
        case .critical: "xmark.octagon.fill"
        }
    }

    private var metricsGrid: some View {
        LazyVGrid(columns: columns, spacing: Theme.spacingSM) {
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
        VStack(alignment: .leading, spacing: Theme.spacingSM) {
            SectionHeader(title: "AI Insights")
            ForEach(viewModel.insights.prefix(3)) { insight in
                SurfaceCard(cornerRadius: Theme.cornerRadiusSmall) {
                    VStack(alignment: .leading, spacing: Theme.spacingXS) {
                        HStack {
                            StatusBadge(
                                text: insight.severity.uppercased(),
                                color: Theme.severityColor(insight.severity)
                            )
                            Text(insight.title)
                                .font(.subheadline.weight(.semibold))
                                .foregroundStyle(Theme.textPrimary)
                        }
                        Text(insight.summary)
                            .font(.caption)
                            .foregroundStyle(Theme.textSecondary)
                    }
                }
            }
        }
    }

    private var problemPodsSection: some View {
        VStack(alignment: .leading, spacing: Theme.spacingSM) {
            SectionHeader(title: "Problem Pods")
            ForEach(viewModel.problemPods.prefix(5)) { pod in
                NavigationLink(value: DashboardDestination.pod(pod.namespace, pod.name)) {
                    SurfaceCard(cornerRadius: Theme.cornerRadiusSmall) {
                        HStack {
                            VStack(alignment: .leading, spacing: Theme.spacingXS) {
                                Text(pod.name)
                                    .font(.subheadline.weight(.medium))
                                    .foregroundStyle(Theme.textPrimary)
                                Text(pod.namespace)
                                    .font(.caption)
                                    .foregroundStyle(Theme.muted)
                            }
                            Spacer()
                            StatusBadge(
                                text: pod.reason.isEmpty ? pod.status : pod.reason,
                                color: Theme.danger
                            )
                        }
                    }
                }
                .buttonStyle(.plain)
            }
        }
    }

    private var clusterNodesSection: some View {
        VStack(alignment: .leading, spacing: Theme.spacingSM) {
            HStack {
                SectionHeader(title: "Cluster Nodes")
                if !viewModel.clusterNodes.isEmpty {
                    NavigationLink(value: DashboardDestination.nodes) {
                        Text("View all")
                            .font(.caption.weight(.semibold))
                            .foregroundStyle(Theme.accentLight)
                    }
                }
            }

            if viewModel.clusterNodes.isEmpty {
                EmptyStateView(
                    title: "No Nodes",
                    systemImage: "server.rack",
                    description: "The cluster did not report any node health data."
                )
                .frame(minHeight: 120)
            } else {
                ForEach(viewModel.clusterNodes) { node in
                    DashboardNodeHealthRow(node: node)
                }
            }
        }
    }

    @ToolbarContentBuilder
    private var toolbarContent: some ToolbarContent {
        ToolbarItem(placement: .topBarLeading) {
            KubePilotMark(size: 28)
        }
        ToolbarItem(placement: .topBarTrailing) {
            NamespacePicker()
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

        var errors: [String] = []

        do {
            summary = try await KubePilotService.shared.fetchTroubleshootingSummary(namespace: namespace)
        } catch {
            errors.append(error.localizedDescription)
        }

        do {
            pods = try await KubePilotService.shared.fetchPods(namespace: namespace)
        } catch {
            errors.append(error.localizedDescription)
        }

        do {
            anomalies = try await KubePilotService.shared.fetchAnomalies()
        } catch {
            anomalies = []
        }

        if summary == nil && pods.isEmpty {
            errorMessage = errors.first
        }
    }
}

struct DashboardNodeHealthRow: View {
    let node: NodeHealthRow

    var body: some View {
        SurfaceCard(cornerRadius: Theme.cornerRadiusSmall) {
            VStack(alignment: .leading, spacing: Theme.spacingSM) {
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

                NodeRoleBadge(health: node)

                NodeIPLabels(health: node)

                HStack(spacing: Theme.spacingMD) {
                    if let cpu = node.cpuUsagePercent {
                        Label("CPU \(cpu)%", systemImage: "cpu")
                    } else {
                        Label("CPU \(node.cpuCapacity)", systemImage: "cpu")
                    }
                    if let mem = node.memoryUsagePercent {
                        Label("Mem \(mem)%", systemImage: "memorychip")
                    } else {
                        Label("Mem \(node.memoryCapacity)", systemImage: "memorychip")
                    }
                }
                .font(.caption2)
                .foregroundStyle(Theme.muted)
                .labelStyle(.titleAndIcon)

                if node.diskPressure || node.memoryPressure || node.pidPressure {
                    HStack(spacing: Theme.spacingXS) {
                        if node.memoryPressure { StatusBadge(text: "Mem pressure", color: Theme.warning) }
                        if node.diskPressure { StatusBadge(text: "Disk pressure", color: Theme.warning) }
                        if node.pidPressure { StatusBadge(text: "PID pressure", color: Theme.warning) }
                    }
                }
            }
        }
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
            .labelStyle(.iconOnly)
            .foregroundStyle(Theme.accentLight)
        }
        .accessibilityLabel("Namespace filter")
        .task { try? await appState.clusterManager.refresh() }
    }
}
