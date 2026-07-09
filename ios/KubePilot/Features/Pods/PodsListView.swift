import SwiftUI

enum PodFilter: String, CaseIterable {
    case all, healthy, failing, crashloop, pending, oom

    var title: String {
        switch self {
        case .all: "All Pods"
        case .healthy: "Healthy"
        case .failing: "Failing"
        case .crashloop: "CrashLoopBackOff"
        case .pending: "Pending"
        case .oom: "OOMKilled"
        }
    }
}

struct PodsListView: View {
    @Environment(AppState.self) private var appState
    @State private var viewModel = PodsListViewModel()
    var initialFilter: PodFilter = .all

    var body: some View {
        NavigationStack {
            VStack(spacing: 12) {
                SearchBar(text: $viewModel.searchText, placeholder: "Search pods…")
                    .padding(.horizontal)

                Picker("Filter", selection: $viewModel.filter) {
                    ForEach(PodFilter.allCases, id: \.self) { filter in
                        Text(filter.title).tag(filter)
                    }
                }
                .pickerStyle(.segmented)
                .padding(.horizontal)

                if viewModel.isLoading && viewModel.pods.isEmpty {
                    LoadingOverlay(message: "Loading pods…")
                } else if let error = viewModel.errorMessage, viewModel.pods.isEmpty {
                    ErrorBanner(message: error) {
                        Task { await viewModel.load(namespace: appState.clusterManager.selectedNamespace) }
                    }
                } else {
                    List(viewModel.filteredPods) { pod in
                        NavigationLink {
                            PodDetailView(namespace: pod.namespace, podName: pod.name)
                        } label: {
                            PodRow(pod: pod)
                        }
                    }
                    .listStyle(.plain)
                }
            }
            .navigationTitle("Pods")
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
}

struct PodRow: View {
    let pod: PodSummary

    var body: some View {
        HStack(spacing: 12) {
            Circle()
                .fill(Theme.healthColor(for: pod.severity))
                .frame(width: 10, height: 10)
            VStack(alignment: .leading, spacing: 4) {
                Text(pod.name)
                    .font(.subheadline.weight(.semibold))
                Text(pod.namespace)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Spacer()
            VStack(alignment: .trailing, spacing: 4) {
                StatusBadge(
                    text: pod.displayStatus,
                    color: Theme.healthColor(for: pod.severity)
                )
                Text("\(pod.restarts) restarts · \(pod.uptime)")
                    .font(.caption2)
                    .foregroundStyle(.tertiary)
            }
        }
        .padding(.vertical, 4)
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
