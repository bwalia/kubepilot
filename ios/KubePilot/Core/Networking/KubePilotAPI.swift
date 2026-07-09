import Foundation

/// Typed KubePilot REST API surface consumed by the mobile app.
enum KubePilotAPI {
    // MARK: - Health

    static func healthCheck() -> APIRequest {
        APIRequest(path: "../healthz") // handled specially in service layer
    }

    static func version() -> APIRequest {
        APIRequest(path: "version")
    }

    static func serverConfig() -> APIRequest {
        APIRequest(path: "config")
    }

    // MARK: - Clusters

    static func kubeconfigs() -> APIRequest {
        APIRequest(path: "clusters/kubeconfigs")
    }

    static func switchContext(_ context: String) -> APIRequest {
        let body = try? JSONEncoder.kubePilot.encode(["context": context])
        return APIRequest(path: "clusters/switch-context", method: .post, body: body)
    }

    static func uploadKubeconfigBase64(name: String, contentBase64: String) -> APIRequest {
        struct Payload: Encodable {
            let name: String
            let content_base64: String
        }
        let body = try? JSONEncoder.kubePilot.encode(Payload(name: name, content_base64: contentBase64))
        return APIRequest(path: "clusters/kubeconfigs/base64", method: .post, body: body)
    }

    // MARK: - Resources

    static func pods(namespace: String = "") -> APIRequest {
        APIRequest(path: "clusters/pods", query: ["namespace": namespace])
    }

    static func crashingPods(namespace: String = "") -> APIRequest {
        APIRequest(path: "clusters/crashing-pods", query: ["namespace": namespace])
    }

    static func deployments(namespace: String = "") -> APIRequest {
        APIRequest(path: "clusters/deployments", query: ["namespace": namespace])
    }

    static func nodes() -> APIRequest {
        APIRequest(path: "clusters/nodes")
    }

    static func namespaces() -> APIRequest {
        APIRequest(path: "namespaces")
    }

    static func events(
        namespace: String = "",
        type: String = "",
        search: String = "",
        limit: Int = 100,
        since: String = "24h"
    ) -> APIRequest {
        APIRequest(
            path: "events",
            query: [
                "namespace": namespace,
                "type": type,
                "search": search,
                "limit": String(limit),
                "since": since,
                "sort": "desc"
            ]
        )
    }

    static func troubleshootingSummary(namespace: String = "") -> APIRequest {
        APIRequest(path: "troubleshooting/summary", query: ["namespace": namespace])
    }

    static func podDiagnostics(namespace: String, pod: String, tailLines: Int = 200) -> APIRequest {
        APIRequest(
            path: "clusters/pods/\(namespace)/\(pod)/diagnostics",
            query: ["tail_lines": String(tailLines)]
        )
    }

    static func podLogs(namespace: String, pod: String, container: String = "", tail: Int = 500) -> APIRequest {
        APIRequest(
            path: "clusters/pods/\(namespace)/\(pod)/logs",
            query: ["container": container, "tail": String(tail)],
            acceptsText: true
        )
    }

    static func resourceYAML(kind: String, namespace: String, name: String) -> APIRequest {
        APIRequest(path: "resource/\(kind)/\(namespace)/\(name)/yaml", acceptsText: true)
    }

    static func services(namespace: String = "") -> APIRequest {
        APIRequest(path: "clusters/services", query: ["namespace": namespace])
    }

    // MARK: - AI & RCA

    static func aiHealth() -> APIRequest {
        APIRequest(path: "ai/health")
    }

    static func troubleshootPod(namespace: String, pod: String) -> APIRequest {
        APIRequest(path: "ai/troubleshoot/\(namespace)/\(pod)", timeout: 180)
    }

    static func interpretCommand(_ command: String) -> APIRequest {
        struct Payload: Encodable { let command: String }
        let body = try? JSONEncoder.kubePilot.encode(Payload(command: command))
        return APIRequest(path: "ai/interpret", method: .post, body: body, timeout: 180)
    }

    static func rcaReports(severity: String = "", namespace: String = "", since: String = "7d") -> APIRequest {
        APIRequest(path: "rca", query: ["severity": severity, "namespace": namespace, "since": since])
    }

    static func rcaReport(id: String) -> APIRequest {
        APIRequest(path: "rca/\(id)")
    }

    static func anomalies(namespace: String = "", since: String = "24h") -> APIRequest {
        APIRequest(path: "anomalies", query: ["namespace": namespace, "since": since])
    }

    static func autopilotStatus(limit: Int = 50) -> APIRequest {
        APIRequest(path: "autopilot", query: ["limit": String(limit)])
    }
}

/// High-level service wrapping APIClient with domain methods.
actor KubePilotService {
    static let shared = KubePilotService()
    private let client = APIClient.shared

    func configure(account: ServerAccount) async {
        await client.configure(baseURL: account.baseURL, authHeader: account.authorizationHeader)
    }

    func ping() async throws -> Bool {
        try await client.healthCheck()
    }

    func fetchTroubleshootingSummary(namespace: String = "") async throws -> ClusterTroubleshootingSummary {
        try await client.send(KubePilotAPI.troubleshootingSummary(namespace: namespace))
    }

    func fetchPods(namespace: String = "") async throws -> [PodSummary] {
        try await client.send(KubePilotAPI.pods(namespace: namespace))
    }

    func fetchCrashingPods(namespace: String = "") async throws -> [PodSummary] {
        try await client.send(KubePilotAPI.crashingPods(namespace: namespace))
    }

    func fetchNodes() async throws -> [NodeSummary] {
        try await client.send(KubePilotAPI.nodes())
    }

    func fetchNamespaces() async throws -> [NamespaceSummary] {
        try await client.send(KubePilotAPI.namespaces())
    }

    func fetchEvents(namespace: String = "", type: String = "", limit: Int = 100) async throws -> EventListResponse {
        try await client.send(KubePilotAPI.events(namespace: namespace, type: type, limit: limit))
    }

    func fetchPodDiagnostics(namespace: String, pod: String) async throws -> PodDiagnosticsResponse {
        try await client.send(KubePilotAPI.podDiagnostics(namespace: namespace, pod: pod))
    }

    func fetchPodLogs(namespace: String, pod: String, container: String = "") async throws -> String {
        try await client.sendText(KubePilotAPI.podLogs(namespace: namespace, pod: pod, container: container))
    }

    func fetchResourceYAML(kind: String, namespace: String, name: String) async throws -> String {
        try await client.sendText(KubePilotAPI.resourceYAML(kind: kind, namespace: namespace, name: name))
    }

    func troubleshootPod(namespace: String, pod: String) async throws -> TroubleshootReport {
        try await client.send(KubePilotAPI.troubleshootPod(namespace: namespace, pod: pod))
    }

    func interpretCommand(_ command: String) async throws -> AIInterpretResponse {
        try await client.send(KubePilotAPI.interpretCommand(command))
    }

    func fetchRCAReports() async throws -> [RCAReport] {
        try await client.send(KubePilotAPI.rcaReports())
    }

    func fetchRCAReport(id: String) async throws -> RCAReport {
        try await client.send(KubePilotAPI.rcaReport(id: id))
    }

    func fetchAnomalies() async throws -> [Anomaly] {
        try await client.send(KubePilotAPI.anomalies())
    }

    func fetchKubeconfigState() async throws -> KubeconfigState {
        try await client.send(KubePilotAPI.kubeconfigs())
    }

    func switchContext(_ context: String) async throws -> KubeconfigState {
        try await client.send(KubePilotAPI.switchContext(context))
    }

    func fetchDeployments(namespace: String = "") async throws -> [DeploymentSummary] {
        try await client.send(KubePilotAPI.deployments(namespace: namespace))
    }

    func fetchAIHealth() async throws -> AIHealthResponse {
        try await client.send(KubePilotAPI.aiHealth())
    }
}
