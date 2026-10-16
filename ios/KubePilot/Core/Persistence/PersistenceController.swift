import Foundation
import Observation
import SwiftData

/// The clusters on the connected KubePilot server and which one this session
/// is looking at. The choice is sent with every request, so it never changes
/// the cluster anyone else sees.
@MainActor
@Observable
final class ClusterManager {
    private(set) var clusters: [ClusterInfo] = []
    private(set) var defaultClusterID: String = ""
    /// Cluster picked in this app; nil follows the server default.
    private(set) var selectedClusterID: String?
    /// False when the server forbids adding kubeconfigs.
    private(set) var canAddClusters = false
    private(set) var selectedNamespace: String = ""
    private(set) var namespaces: [NamespaceSummary] = []
    private(set) var isSwitching = false
    private(set) var isLoadingClusters = false
    /// Bumped on every switch; views key on it to drop state from the old cluster.
    private(set) var epoch = 0
    /// Context name reported by servers that predate the cluster list.
    private var legacyContext: String?

    func reset() {
        clusters = []
        defaultClusterID = ""
        selectedClusterID = nil
        canAddClusters = false
        selectedNamespace = ""
        namespaces = []
        legacyContext = nil
        epoch += 1
    }

    /// Call after the active server account changes.
    func accountChanged() async {
        reset()
        if let key = await KubePilotService.shared.currentAccountKey() {
            selectedClusterID = KubePilotService.storedCluster(forAccount: key)
        }
        try? await refresh()
    }

    func refresh() async throws {
        async let list = loadClusters(probe: false)
        async let ns = KubePilotService.shared.fetchNamespaces()
        try await list
        namespaces = try await ns
    }

    /// Reloads the cluster list; `probe` also checks each API server.
    func loadClusters(probe: Bool) async throws {
        isLoadingClusters = true
        defer { isLoadingClusters = false }
        do {
            let list = try await KubePilotService.shared.fetchClusters(probe: probe)
            apply(list)
        } catch APIError.notFound {
            // Older servers have no cluster list; show their single context.
            let kube = try await KubePilotService.shared.fetchKubeconfigState()
            legacyContext = kube.activeContext
            clusters = []
            canAddClusters = false
        } catch APIError.clusterGone {
            await select(nil)
        }
    }

    private func apply(_ list: ClusterListResponse) {
        clusters = list.clusters
        defaultClusterID = list.defaultID
        canAddClusters = list.mutations ?? false
        if let selectedClusterID, !list.clusters.contains(where: { $0.id == selectedClusterID }) {
            Task { await select(nil) }
        }
    }

    /// Switches this session to `id` (nil: the server default).
    func select(_ id: String?) async {
        let next = id
        guard next != selectedClusterID else { return }
        isSwitching = true
        defer { isSwitching = false }
        await KubePilotService.shared.selectCluster(next)
        selectedClusterID = next
        selectedNamespace = ""
        namespaces = []
        epoch += 1
        namespaces = (try? await KubePilotService.shared.fetchNamespaces()) ?? []
    }

    /// Uploads a kubeconfig to the server and switches to the first cluster in it.
    func addCluster(named name: String, contents: Data) async throws {
        let list = try await KubePilotService.shared.addKubeconfig(name: name, contents: contents)
        apply(list)
        if let first = list.addedIDs?.first {
            await select(first)
        }
    }

    func makeDefault(_ cluster: ClusterInfo) async throws {
        apply(try await KubePilotService.shared.setDefaultCluster(id: cluster.id))
    }

    func selectNamespace(_ namespace: String) {
        selectedNamespace = namespace
    }

    var activeCluster: ClusterInfo? {
        let id = selectedClusterID ?? defaultClusterID
        return clusters.first { $0.id == id } ?? clusters.first { $0.isDefault }
    }

    var activeContextName: String {
        activeCluster?.name ?? legacyContext ?? "Unknown"
    }
}

struct PersistenceController {
    static let shared = PersistenceController()

    let container: ModelContainer

    private init() {
        let schema = Schema([
            CachedTroubleshootingSummary.self,
            CachedRCAReport.self,
            RecentResource.self,
            FavoriteCluster.self
        ])
        let config = ModelConfiguration("KubePilot", isStoredInMemoryOnly: false)
        do {
            container = try ModelContainer(for: schema, configurations: [config])
        } catch {
            fatalError("SwiftData container failed: \(error)")
        }
    }
}

@Model
final class CachedTroubleshootingSummary {
    @Attribute(.unique) var id: String
    var namespace: String
    var payloadJSON: Data
    var fetchedAt: Date

    init(namespace: String, payloadJSON: Data, fetchedAt: Date = .now) {
        self.id = namespace.isEmpty ? "all" : namespace
        self.namespace = namespace
        self.payloadJSON = payloadJSON
        self.fetchedAt = fetchedAt
    }
}

@Model
final class CachedRCAReport {
    @Attribute(.unique) var reportID: String
    var payloadJSON: Data
    var fetchedAt: Date

    init(reportID: String, payloadJSON: Data, fetchedAt: Date = .now) {
        self.reportID = reportID
        self.payloadJSON = payloadJSON
        self.fetchedAt = fetchedAt
    }
}

@Model
final class RecentResource {
    @Attribute(.unique) var key: String
    var kind: String
    var namespace: String
    var name: String
    var viewedAt: Date

    init(kind: String, namespace: String, name: String, viewedAt: Date = .now) {
        self.key = "\(kind)/\(namespace)/\(name)"
        self.kind = kind
        self.namespace = namespace
        self.name = name
        self.viewedAt = viewedAt
    }
}

@Model
final class FavoriteCluster {
    @Attribute(.unique) var clusterID: String
    var name: String
    var environment: String
    var colorHex: String

    init(clusterID: String, name: String, environment: String, colorHex: String) {
        self.clusterID = clusterID
        self.name = name
        self.environment = environment
        self.colorHex = colorHex
    }
}
