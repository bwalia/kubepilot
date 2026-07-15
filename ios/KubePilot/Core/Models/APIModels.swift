import Foundation

// MARK: - Cluster Resources (PascalCase from Go)

struct PodSummary: Codable, Identifiable, Hashable, Sendable {
    var id: String { "\(namespace)/\(name)" }
    let name: String
    let namespace: String
    let phase: String
    let reason: String
    let nodeName: String
    let restarts: Int
    let ready: Bool
    let uptime: String

    enum CodingKeys: String, CodingKey {
        case name = "Name"
        case namespace = "Namespace"
        case phase = "Phase"
        case reason = "Reason"
        case nodeName = "NodeName"
        case restarts = "Restarts"
        case ready = "Ready"
        case uptime = "Uptime"
    }

    var displayStatus: String {
        if !reason.isEmpty { return reason }
        return phase
    }

    var severity: HealthScore {
        switch reason {
        case "CrashLoopBackOff", "OOMKilled", "Error", "ImagePullBackOff", "ErrImagePull":
            return .critical
        case "Pending":
            return .degraded
        default:
            return ready ? .healthy : .degraded
        }
    }
}

struct DeploymentSummary: Codable, Identifiable, Hashable, Sendable {
    var id: String { "\(namespace)/\(name)" }
    let name: String
    let namespace: String
    let replicas: Int
    let readyReplicas: Int
    let availableReplicas: Int
    let image: String

    enum CodingKeys: String, CodingKey {
        case name = "Name"
        case namespace = "Namespace"
        case replicas = "Replicas"
        case readyReplicas = "ReadyReplicas"
        case availableReplicas = "AvailableReplicas"
        case image = "Image"
    }
}

struct NodeSummary: Codable, Identifiable, Hashable, Sendable {
    var id: String { name }
    let name: String
    let ready: Bool
    let memoryPressure: Bool
    let diskPressure: Bool
    let pidPressure: Bool
    let cpuCapacity: String
    let memoryCapacity: String
    let kubeletVersion: String
    let internalIP: String
    let ips: [String]?
    let lanIPs: [String]?
    let wanIPs: [String]?
    let tunnelIPs: [String]?
    let unschedulable: Bool

    enum CodingKeys: String, CodingKey {
        case name = "Name"
        case ready = "Ready"
        case memoryPressure = "MemoryPressure"
        case diskPressure = "DiskPressure"
        case pidPressure = "PIDPressure"
        case cpuCapacity = "CPUCapacity"
        case memoryCapacity = "MemoryCapacity"
        case kubeletVersion = "KubeletVersion"
        case internalIP = "InternalIP"
        case ips = "IPs"
        case lanIPs = "LANIPs"
        case wanIPs = "WANIPs"
        case tunnelIPs = "TunnelIPs"
        case unschedulable = "Unschedulable"
    }

    var allIPs: [String] {
        if let lanIPs, let wanIPs, let tunnelIPs, !(lanIPs + wanIPs + tunnelIPs).isEmpty {
            return Array(Set(lanIPs + wanIPs + tunnelIPs)).sorted()
        }
        if let ips, !ips.isEmpty { return ips }
        if !internalIP.isEmpty { return internalIP.components(separatedBy: ", ") }
        return []
    }

    var healthScore: HealthScore {
        if !ready || memoryPressure || diskPressure || pidPressure { return .critical }
        return .healthy
    }
}

struct NamespaceSummary: Codable, Identifiable, Hashable, Sendable {
    var id: String { name }
    let name: String
    let status: String
    let labels: [String: String]?

    enum CodingKeys: String, CodingKey {
        case name = "Name"
        case status = "Status"
        case labels = "Labels"
    }
}

// MARK: - Events & Troubleshooting (snake_case)

struct KubeEvent: Codable, Identifiable, Hashable, Sendable {
    var id: String { "\(involvedObject.namespace)/\(involvedObject.kind)/\(involvedObject.name)/\(reason)/\(lastSeen)" }
    let reason: String
    let message: String
    let type: String
    let count: Int
    let firstSeen: String
    let lastSeen: String
    let involvedObject: InvolvedObject
    let source: String

    enum CodingKeys: String, CodingKey {
        case reason, message, type, count, source
        case firstSeen = "first_seen"
        case lastSeen = "last_seen"
        case involvedObject = "involved_object"
    }
}

struct InvolvedObject: Codable, Hashable, Sendable {
    let kind: String
    let name: String
    let namespace: String
}

struct EventListResponse: Codable, Sendable {
    let items: [KubeEvent]
    let total: Int
}

struct TroubleshootingInsight: Codable, Identifiable, Hashable, Sendable {
    var id: String
    let category: String
    let severity: String
    let title: String
    let summary: String
    let suggestions: [String]
    let affectedResources: [String]?

    enum CodingKeys: String, CodingKey {
        case id, category, severity, title, summary, suggestions
        case affectedResources = "affected_resources"
    }
}

struct NodeHealthRow: Codable, Identifiable, Hashable, Sendable {
    var id: String { name }
    let name: String
    let ready: Bool
    let cpuCapacity: String
    let memoryCapacity: String
    let cpuUsage: String?
    let memoryUsage: String?
    let cpuUsagePercent: Int?
    let memoryUsagePercent: Int?
    let diskPressure: Bool
    let memoryPressure: Bool
    let pidPressure: Bool
    let unschedulable: Bool
    let kubeletVersion: String
    let ips: [String]?
    let lanIPs: [String]?
    let wanIPs: [String]?
    let tunnelIPs: [String]?

    enum CodingKeys: String, CodingKey {
        case name, ready, unschedulable
        case cpuCapacity = "cpu_capacity"
        case memoryCapacity = "memory_capacity"
        case cpuUsage = "cpu_usage"
        case memoryUsage = "memory_usage"
        case cpuUsagePercent = "cpu_usage_percent"
        case memoryUsagePercent = "memory_usage_percent"
        case diskPressure = "disk_pressure"
        case memoryPressure = "memory_pressure"
        case pidPressure = "pid_pressure"
        case kubeletVersion = "kubelet_version"
        case ips
        case lanIPs = "lan_ips"
        case wanIPs = "wan_ips"
        case tunnelIPs = "tunnel_ips"
    }
}

struct ProblemPod: Codable, Identifiable, Hashable, Sendable {
    var id: String { "\(namespace)/\(name)" }
    let name: String
    let namespace: String
    let status: String
    let restarts: Int
    let node: String
    let reason: String
    let ageMinutes: Int
    let message: String?

    enum CodingKeys: String, CodingKey {
        case name, namespace, status, restarts, node, reason, message
        case ageMinutes = "age_minutes"
    }
}

struct HealthSummary: Codable, Sendable {
    let notReadyNodes: Int
    let crashloopPods: Int
    let failedMountEvents: Int
    let pendingPods: Int
    let warningEvents: Int
    let recommendedActions: [String]

    enum CodingKeys: String, CodingKey {
        case notReadyNodes = "not_ready_nodes"
        case crashloopPods = "crashloop_pods"
        case failedMountEvents = "failed_mount_events"
        case pendingPods = "pending_pods"
        case warningEvents = "warning_events"
        case recommendedActions = "recommended_actions"
    }
}

struct ResourcePressureSummary: Codable, Sendable {
    let metricsAvailable: Bool
    let cpuUsagePercent: Int?
    let memoryUsagePercent: Int?
    let memoryPressureNodes: Int
    let diskPressureNodes: Int
    let pidPressureNodes: Int

    enum CodingKeys: String, CodingKey {
        case metricsAvailable = "metrics_available"
        case cpuUsagePercent = "cpu_usage_percent"
        case memoryUsagePercent = "memory_usage_percent"
        case memoryPressureNodes = "memory_pressure_nodes"
        case diskPressureNodes = "disk_pressure_nodes"
        case pidPressureNodes = "pid_pressure_nodes"
    }
}

struct ClusterTroubleshootingSummary: Codable, Sendable {
    let namespace: String
    let generatedAt: Date
    let healthSummary: HealthSummary
    let insights: [TroubleshootingInsight]
    let nodes: [NodeHealthRow]
    let resourcePressure: ResourcePressureSummary
    let problemPods: [ProblemPod]

    enum CodingKeys: String, CodingKey {
        case namespace, insights, nodes
        case generatedAt = "generated_at"
        case healthSummary = "health_summary"
        case resourcePressure = "resource_pressure"
        case problemPods = "problem_pods"
    }

    var overallHealth: HealthScore {
        let h = healthSummary
        if h.crashloopPods > 0 || h.notReadyNodes > 0 { return .critical }
        if h.pendingPods > 0 || h.warningEvents > 5 { return .degraded }
        return .healthy
    }
}

// MARK: - Pod Diagnostics

struct PodCondition: Codable, Hashable, Sendable {
    let type: String
    let status: String
    let reason: String?
    let message: String?
}

struct ContainerDiag: Codable, Identifiable, Hashable, Sendable {
    var id: String { name }
    let name: String
    let image: String
    let ready: Bool
    let restartCount: Int
    let state: String
    let stateReason: String
    let stateMessage: String
    let exitCode: Int

    enum CodingKeys: String, CodingKey {
        case name, image, ready, state
        case restartCount = "restart_count"
        case stateReason = "state_reason"
        case stateMessage = "state_message"
        case exitCode = "exit_code"
    }
}

struct PodDiagnostics: Codable, Sendable {
    let name: String
    let namespace: String
    let phase: String
    let nodeName: String
    let createdAt: String
    let conditions: [PodCondition]
    let containerStatuses: [ContainerDiag]
    let events: [KubeEvent]
    let labels: [String: String]
    let ownerChain: [OwnerRef]

    enum CodingKeys: String, CodingKey {
        case name, namespace, phase, conditions, events, labels
        case nodeName = "node_name"
        case createdAt = "created_at"
        case containerStatuses = "container_statuses"
        case ownerChain = "owner_chain"
    }
}

struct OwnerRef: Codable, Hashable, Sendable {
    let kind: String
    let name: String
    let uid: String
}

struct PodDiagnosticsResponse: Codable, Sendable {
    let diagnostics: PodDiagnostics
    let logs: String
}

// MARK: - AI & RCA

struct AIHealthResponse: Codable, Sendable {
    let healthy: Bool
    let model: String
    let baseURL: String
    let latencyMs: Int?
    let error: String?

    enum CodingKeys: String, CodingKey {
        case healthy, model, error
        case baseURL = "base_url"
        case latencyMs = "latency_ms"
    }
}

struct SuggestedAction: Codable, Identifiable, Hashable, Sendable {
    var id: String { "\(type)-\(resource ?? "")-\(explanation)" }
    let type: String
    let namespace: String?
    let resource: String?
    let replicas: Int?
    let command: String?
    let explanation: String
    let requiresCrCode: Bool

    enum CodingKeys: String, CodingKey {
        case type, namespace, resource, replicas, command, explanation
        case requiresCrCode = "requires_cr_code"
    }
}

struct TroubleshootReport: Codable, Sendable {
    let podName: String
    let namespace: String
    let rootCause: String
    let analysis: String
    let actions: [SuggestedAction]

    enum CodingKeys: String, CodingKey {
        case actions
        case podName = "pod_name"
        case namespace
        case rootCause = "root_cause"
        case analysis
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        podName = try c.decodeIfPresent(String.self, forKey: .podName)
            ?? (try? decoder.container(keyedBy: AltKeys.self).decode(String.self, forKey: .podName)) ?? ""
        namespace = try c.decodeIfPresent(String.self, forKey: .namespace) ?? ""
        rootCause = try c.decodeIfPresent(String.self, forKey: .rootCause) ?? ""
        analysis = try c.decodeIfPresent(String.self, forKey: .analysis) ?? ""
        actions = try c.decodeIfPresent([SuggestedAction].self, forKey: .actions) ?? []
    }

    private enum AltKeys: String, CodingKey { case podName = "PodName" }
}

struct AIInterpretResponse: Codable, Sendable {
    let actions: [SuggestedAction]
}

struct RCAReport: Codable, Identifiable, Hashable, Sendable {
    let id: String
    let timestamp: Date
    let targetResource: String
    let severity: String
    let rootCause: String
    let evidenceChain: [String]
    let remediation: [String]
    let confidence: Double
    let status: String

    enum CodingKeys: String, CodingKey {
        case id, timestamp, severity, confidence, status, remediation
        case targetResource = "target_resource"
        case rootCause = "root_cause"
        case evidenceChain = "evidence_chain"
    }
}

struct Anomaly: Codable, Identifiable, Hashable, Sendable {
    let id: String
    let detectedAt: Date
    let rule: String
    let resource: String
    let severity: String
    let description: String
    let rcaReportId: String?

    enum CodingKeys: String, CodingKey {
        case id, rule, resource, severity, description
        case detectedAt = "detected_at"
        case rcaReportId = "rca_report_id"
    }
}

// MARK: - Cluster Config

struct ContextInfo: Codable, Hashable, Sendable {
    let name: String
    let cluster: String
    let user: String
}

struct KubeconfigState: Codable, Sendable {
    let activePath: String
    let activeContext: String
    let paths: [String]
    let contexts: [ContextInfo]?

    enum CodingKeys: String, CodingKey {
        case paths, contexts
        case activePath = "active_path"
        case activeContext = "active_context"
    }
}

struct ServerConfigResponse: Codable, Sendable {
    let mutationsEnabled: Bool

    enum CodingKeys: String, CodingKey {
        case mutationsEnabled = "mutations_enabled"
    }
}

// MARK: - Mobile-specific models

struct ClusterProfile: Codable, Identifiable, Hashable, Sendable {
    var id: String
    var name: String
    var serverURL: URL
    var provider: String
    var region: String
    var colorHex: String
    var environment: ClusterEnvironment
    var isFavorite: Bool
    var lastConnectedAt: Date?

    enum ClusterEnvironment: String, Codable, CaseIterable, Sendable {
        case production, staging, development

        var label: String {
            switch self {
            case .production: "Production"
            case .staging: "Staging"
            case .development: "Development"
            }
        }
    }
}

struct ServerAccount: Codable, Identifiable, Hashable, Sendable {
    var id: String
    var displayName: String
    var baseURL: URL
    var authMethod: AuthMethod
    var bearerToken: String?
    var username: String?
    var password: String?
    var clusters: [ClusterProfile]
    var activeClusterID: String?
    var biometricLockEnabled: Bool
    var createdAt: Date

    enum AuthMethod: String, Codable, CaseIterable, Sendable {
        case bearer
        case basic
        case oauthGitHub
        case oauthGitLab
        case oauthGoogle
        case oauthMicrosoft
        case oidc

        var label: String {
            switch self {
            case .bearer: "API Token"
            case .basic: "Username & Password"
            case .oauthGitHub: "GitHub"
            case .oauthGitLab: "GitLab"
            case .oauthGoogle: "Google"
            case .oauthMicrosoft: "Microsoft"
            case .oidc: "OIDC / SAML"
            }
        }
    }

    var authorizationHeader: String? {
        switch authMethod {
        case .bearer:
            guard let bearerToken, !bearerToken.isEmpty else { return nil }
            return "Bearer \(bearerToken)"
        case .basic:
            guard let username, let password else { return nil }
            let creds = Data("\(username):\(password)".utf8).base64EncodedString()
            return "Basic \(creds)"
        case .oauthGitHub, .oauthGitLab, .oauthGoogle, .oauthMicrosoft, .oidc:
            guard let bearerToken, !bearerToken.isEmpty else { return nil }
            return "Bearer \(bearerToken)"
        }
    }

    var activeCluster: ClusterProfile? {
        guard let activeClusterID else { return clusters.first }
        return clusters.first { $0.id == activeClusterID } ?? clusters.first
    }
}

struct AIChatMessage: Identifiable, Hashable, Sendable {
    let id: UUID
    let role: Role
    let content: String
    let timestamp: Date
    var actions: [SuggestedAction]
    var isStreaming: Bool

    enum Role: String, Sendable {
        case user, assistant, system
    }

    init(
        id: UUID = UUID(),
        role: Role,
        content: String,
        timestamp: Date = .now,
        actions: [SuggestedAction] = [],
        isStreaming: Bool = false
    ) {
        self.id = id
        self.role = role
        self.content = content
        self.timestamp = timestamp
        self.actions = actions
        self.isStreaming = isStreaming
    }
}
