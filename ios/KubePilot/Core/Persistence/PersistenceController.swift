import Foundation
import Observation
import SwiftData

@MainActor
@Observable
final class ClusterManager {
    private(set) var kubeconfig: KubeconfigState?
    private(set) var selectedNamespace: String = ""
    private(set) var namespaces: [NamespaceSummary] = []
    private(set) var isSwitching = false

    func reset() {
        kubeconfig = nil
        selectedNamespace = ""
        namespaces = []
    }

    func refresh() async throws {
        async let kube = KubePilotService.shared.fetchKubeconfigState()
        async let ns = KubePilotService.shared.fetchNamespaces()
        kubeconfig = try await kube
        namespaces = try await ns
    }

    func selectNamespace(_ namespace: String) {
        selectedNamespace = namespace
    }

    func switchContext(_ context: String) async throws {
        isSwitching = true
        defer { isSwitching = false }
        kubeconfig = try await KubePilotService.shared.switchContext(context)
    }

    var activeContextName: String {
        kubeconfig?.activeContext ?? "Unknown"
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
