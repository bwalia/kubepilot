import SwiftUI
import UniformTypeIdentifiers

/// Switch between the KubePilot servers you've added and the clusters each
/// one manages, and add more of either. Picking a cluster changes only this
/// device's view; the server default (what Autopilot watches) moves only via
/// "Make server default".
struct ClusterPickerView: View {
    @Environment(AppState.self) private var appState
    @Environment(\.dismiss) private var dismiss
    @State private var showAddServer = false
    @State private var showImporter = false
    @State private var errorMessage: String?
    @State private var isWorking = false

    private var clusterManager: ClusterManager { appState.clusterManager }
    private var auth: AuthManager { appState.authManager }

    var body: some View {
        NavigationStack {
            ZStack {
                BrandScreenBackground()

                List {
                    if let errorMessage {
                        Section {
                            ErrorBanner(message: errorMessage, retry: nil)
                                .listRowBackground(Color.clear)
                                .listRowInsets(EdgeInsets())
                        }
                    }

                    clustersSection
                    serversSection
                }
                .themedForm()
                .refreshable { await reload() }
            }
            .navigationTitle("Clusters")
            .navigationBarTitleDisplayMode(.inline)
            .themedScreen()
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Done") { dismiss() }
                }
            }
            .task { await reload() }
            .sheet(isPresented: $showAddServer) {
                OnboardingView()
            }
            .onChange(of: auth.accounts.count) { old, new in
                // Onboarding saved a new server and switched to it.
                if new > old { showAddServer = false }
            }
            .fileImporter(
                isPresented: $showImporter,
                allowedContentTypes: [.yaml, .data, .text],
                allowsMultipleSelection: false
            ) { result in
                Task { await importKubeconfig(result) }
            }
        }
    }

    // MARK: Sections

    private var clustersSection: some View {
        Section {
            if clusterManager.clusters.isEmpty {
                HStack {
                    if clusterManager.isLoadingClusters { ProgressView().padding(.trailing, Theme.spacingXS) }
                    Text(clusterManager.isLoadingClusters ? "Loading clusters…" : clusterManager.activeContextName)
                        .foregroundStyle(Theme.muted)
                }
            }
            ForEach(clusterManager.clusters) { cluster in
                Button {
                    Task {
                        await clusterManager.select(cluster.id)
                        dismiss()
                    }
                } label: {
                    ClusterRow(cluster: cluster, isSelected: cluster.id == clusterManager.activeCluster?.id)
                }
                .disabled(clusterManager.isSwitching)
                .contextMenu {
                    if clusterManager.canAddClusters && !cluster.isDefault {
                        Button("Make server default", systemImage: "star") {
                            Task { await run { try await clusterManager.makeDefault(cluster) } }
                        }
                    }
                }
                .accessibilityHint(cluster.isDefault ? "Server default cluster" : "")
            }
            if clusterManager.canAddClusters {
                Button {
                    showImporter = true
                } label: {
                    Label("Add cluster from kubeconfig…", systemImage: "plus.circle")
                }
                .disabled(isWorking)
            }
        } header: {
            Text("Clusters on \(auth.activeAccount?.displayName ?? "this server")")
        } footer: {
            Text("Switching changes only this device. Long-press a cluster to make it the server default that Autopilot watches.")
        }
        .listRowBackground(Theme.surface)
    }

    private var serversSection: some View {
        Section {
            ForEach(auth.accounts) { account in
                Button {
                    Task { await auth.switchAccount(account) }
                } label: {
                    HStack {
                        VStack(alignment: .leading, spacing: 2) {
                            Text(account.displayName)
                                .foregroundStyle(Theme.textPrimary)
                            Text(account.baseURL.host ?? account.baseURL.absoluteString)
                                .font(Theme.Typography.identifierSmall)
                                .foregroundStyle(Theme.muted)
                        }
                        Spacer()
                        if account.id == auth.activeAccount?.id {
                            Image(systemName: "checkmark")
                                .foregroundStyle(Theme.accentLight)
                        }
                    }
                }
                .swipeActions {
                    if auth.accounts.count > 1 {
                        Button("Remove", role: .destructive) {
                            Task { await auth.removeAccount(account) }
                        }
                    }
                }
            }
            Button {
                showAddServer = true
            } label: {
                Label("Add KubePilot server…", systemImage: "plus.circle")
            }
        } header: {
            Text("Servers")
        } footer: {
            Text("Each server can manage several clusters. Add another server to reach clusters it manages.")
        }
        .listRowBackground(Theme.surface)
    }

    // MARK: Actions

    private func reload() async {
        await run { try await clusterManager.loadClusters(probe: true) }
    }

    private func importKubeconfig(_ result: Result<[URL], Error>) async {
        await run {
            guard let url = try result.get().first else { return }
            let scoped = url.startAccessingSecurityScopedResource()
            defer { if scoped { url.stopAccessingSecurityScopedResource() } }
            let data = try Data(contentsOf: url)
            try await clusterManager.addCluster(named: url.lastPathComponent, contents: data)
            dismiss()
        }
    }

    private func run(_ work: () async throws -> Void) async {
        isWorking = true
        defer { isWorking = false }
        do {
            try await work()
            errorMessage = nil
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}

private struct ClusterRow: View {
    let cluster: ClusterInfo
    let isSelected: Bool

    private var statusColor: Color {
        switch cluster.reachable {
        case .some(true): Theme.success
        case .some(false): Theme.danger
        case .none: Theme.muted
        }
    }

    var body: some View {
        HStack(spacing: Theme.spacingSM) {
            Circle()
                .fill(statusColor)
                .frame(width: 8, height: 8)
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 2) {
                HStack(spacing: Theme.spacingXS) {
                    Text(cluster.name)
                        .font(Theme.Typography.identifier)
                        .foregroundStyle(Theme.textPrimary)
                        .lineLimit(1)
                    if cluster.isDefault {
                        StatusBadge(text: "Default", color: Theme.amber)
                    }
                }
                Text(detail)
                    .font(.caption)
                    .foregroundStyle(cluster.reachable == false ? Theme.danger : Theme.muted)
                    .lineLimit(1)
            }
            Spacer()
            if isSelected {
                Image(systemName: "checkmark")
                    .foregroundStyle(Theme.accentLight)
            }
        }
        .contentShape(Rectangle())
        .accessibilityElement(children: .combine)
        .accessibilityAddTraits(isSelected ? .isSelected : [])
    }

    private var detail: String {
        if cluster.reachable == false { return "Unreachable" }
        return [cluster.serverHost, cluster.version].compactMap { $0 }.joined(separator: " · ")
    }
}
