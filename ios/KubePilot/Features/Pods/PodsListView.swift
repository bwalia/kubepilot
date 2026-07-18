import SwiftUI

enum PodFilter: String, CaseIterable {
    case all, healthy, failing, crashloop, pending, oom

    var title: String {
        switch self {
        case .all: "All"
        case .healthy: "Healthy"
        case .failing: "Failing"
        case .crashloop: "CrashLoop"
        case .pending: "Pending"
        case .oom: "OOM"
        }
    }
}

struct PodsListView: View {
    @Environment(AppState.self) private var appState
    @State private var viewModel = PodsListViewModel()
    var initialFilter: PodFilter = .all

    var body: some View {
        NavigationStack {
            ZStack {
                BrandScreenBackground()

                VStack(spacing: Theme.spacingSM) {
                    SearchBar(text: $viewModel.searchText, placeholder: "Search pods…")
                        .padding(.horizontal, Theme.spacingMD)

                    FilterChipBar(
                        items: Array(PodFilter.allCases),
                        selection: $viewModel.filter,
                        title: { $0.title }
                    )

                    content
                }
                .padding(.top, Theme.spacingSM)
            }
            .navigationTitle("Pods")
            .navigationBarTitleDisplayMode(.large)
            .themedScreen()
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) { NamespacePicker() }
            }
            .refreshable {
                await viewModel.load(namespace: appState.clusterManager.selectedNamespace)
            }
            .task(id: appState.clusterManager.selectedNamespace) {
                viewModel.filter = initialFilter
                await viewModel.load(namespace: appState.clusterManager.selectedNamespace)
            }
        }
    }

    @ViewBuilder
    private var content: some View {
        if viewModel.isLoading && viewModel.pods.isEmpty {
            LoadingOverlay(message: "Loading pods…")
        } else if let error = viewModel.errorMessage, viewModel.pods.isEmpty {
            ErrorBanner(message: error) {
                Task { await viewModel.load(namespace: appState.clusterManager.selectedNamespace) }
            }
        } else if viewModel.filteredPods.isEmpty {
            EmptyStateView(
                title: viewModel.searchText.isEmpty ? "No Pods" : "No Matches",
                systemImage: "cube.box",
                description: viewModel.searchText.isEmpty
                    ? "No pods were returned for the current namespace."
                    : "Try a different search term or filter."
            )
        } else {
            List(viewModel.filteredPods) { pod in
                NavigationLink {
                    PodDetailView(namespace: pod.namespace, podName: pod.name)
                } label: {
                    PodRow(pod: pod)
                }
                .listRowBackground(Theme.surface)
            }
            .themedList()
        }
    }
}

struct PodRow: View {
    let pod: PodSummary

    var body: some View {
        HStack(spacing: Theme.spacingSM) {
            HealthIndicator(
                color: Theme.healthColor(for: pod.severity),
                label: pod.displayStatus
            )
            VStack(alignment: .leading, spacing: Theme.spacingXS) {
                Text(pod.name)
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(Theme.textPrimary)
                Text(pod.namespace)
                    .font(.caption)
                    .foregroundStyle(Theme.muted)
            }
            Spacer()
            VStack(alignment: .trailing, spacing: Theme.spacingXS) {
                StatusBadge(
                    text: pod.displayStatus,
                    color: Theme.healthColor(for: pod.severity)
                )
                Text("\(pod.restarts) restarts · \(pod.uptime)")
                    .font(.caption2)
                    .foregroundStyle(Theme.muted)
            }
        }
        .padding(.vertical, Theme.spacingXS)
    }
}

@MainActor
@Observable
final class PodsListViewModel {
    var pods: [PodSummary] = []
    var searchText = ""
    var filter: PodFilter = .all
    var isLoading = false
    var errorMessage: String?

    var filteredPods: [PodSummary] {
        pods.filter { pod in
            let matchesSearch = searchText.isEmpty
                || pod.name.localizedCaseInsensitiveContains(searchText)
                || pod.namespace.localizedCaseInsensitiveContains(searchText)
            guard matchesSearch else { return false }
            switch filter {
            case .all: return true
            case .healthy: return pod.ready && pod.reason.isEmpty
            case .failing: return !pod.ready || !pod.reason.isEmpty
            case .crashloop: return pod.reason == "CrashLoopBackOff"
            case .pending: return pod.phase == "Pending"
            case .oom: return pod.reason == "OOMKilled"
            }
        }
    }

    func load(namespace: String) async {
        isLoading = true
        errorMessage = nil
        defer { isLoading = false }
        do {
            pods = try await KubePilotService.shared.fetchPods(namespace: namespace)
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}
