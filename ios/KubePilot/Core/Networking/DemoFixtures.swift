import Foundation

/// Offline fixture responses for App Review when demo.kubepilot.org is unreachable.
/// Shapes match `cmd/demo-api` / iOS `APIModels` (PascalCase pods/nodes, snake_case summaries).
enum DemoFixtures {
    static let offlineHost = "offline.demo.kubepilot.local"
    static let hostedURL = URL(string: "https://demo.kubepilot.org")!
    static let username = "apple"
    static let password = "review"

    static var offlineBaseURL: URL { URL(string: "https://\(offlineHost)")! }

    static func isOfflineURL(_ url: URL) -> Bool {
        url.host == offlineHost
    }

    static let healthOK = Data("ok".utf8)

    /// Returns fixture body for `/api/v1/<path>`.
    static func data(forAPIPath path: String, query: [String: String] = [:]) -> Data? {
        let normalized = path.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
        switch normalized {
        case "clusters/pods":
            return utf8(podsJSON)
        case "clusters/crashing-pods":
            return utf8(crashingPodsJSON)
        case "clusters/nodes":
            return utf8(nodesJSON)
        case "clusters/deployments":
            return utf8(deploymentsJSON)
        case "namespaces":
            return utf8(namespacesJSON)
        case "events":
            return utf8(eventsJSON)
        case "troubleshooting/summary":
            return utf8(troubleshootingJSON)
        case "ai/health":
            return utf8(aiHealthJSON)
        case "rca":
            return utf8(rcaListJSON)
        case "anomalies":
            return utf8(anomaliesJSON)
        case "autopilot":
            return utf8(autopilotJSON)
        case "clusters/kubeconfigs":
            return utf8(kubeconfigsJSON)
        case "version":
            return utf8(#"{"version":"1.0.0-demo","commit":"app-review","built":"2026-09-29T00:00:00Z"}"#)
        case "config":
            return utf8(#"{"mutations_enabled":false}"#)
        case "ai/interpret", "ai/execute-action", "autopilot/mode", "autopilot/pause", "autopilot/resume",
             "clusters/switch-context", "crcode/authorize":
            return utf8(#"{"status":"ok","message":"demo stub accepted"}"#)
        default:
            if normalized.hasPrefix("clusters/pods/") && normalized.hasSuffix("/diagnostics") {
                return utf8(diagnosticsJSON)
            }
            if normalized.hasPrefix("clusters/pods/") && normalized.hasSuffix("/logs") {
                return Data(logText.utf8)
            }
            if normalized.hasPrefix("ai/troubleshoot/") {
                return utf8(troubleshootJSON)
            }
            if normalized.hasPrefix("rca/") {
                return utf8(rcaOneJSON)
            }
            if normalized.hasPrefix("resource/") {
                return Data("apiVersion: v1\nkind: Pod\nmetadata:\n  name: checkout-api-7d9f8c\n  namespace: payments\n# demo fixture\n".utf8)
            }
            _ = query
            return utf8(#"{}"#)
        }
    }

    private static func utf8(_ s: String) -> Data { Data(s.utf8) }

    private static let podsJSON = """
    [{"Name":"checkout-api-7d9f8c","Namespace":"payments","Phase":"Running","Reason":"CrashLoopBackOff","NodeName":"demo-worker-1","Restarts":3,"Ready":false,"Uptime":"12m"},{"Name":"checkout-api-9ab12e","Namespace":"payments","Phase":"Running","Reason":"","NodeName":"demo-worker-2","Restarts":0,"Ready":true,"Uptime":"4h"},{"Name":"redis-cart-0","Namespace":"payments","Phase":"Running","Reason":"","NodeName":"demo-worker-1","Restarts":0,"Ready":true,"Uptime":"2d"},{"Name":"coredns-5d4f","Namespace":"kube-system","Phase":"Running","Reason":"","NodeName":"demo-control-1","Restarts":0,"Ready":true,"Uptime":"14d"},{"Name":"metrics-server","Namespace":"kube-system","Phase":"Pending","Reason":"ImagePullBackOff","NodeName":"","Restarts":2,"Ready":false,"Uptime":"8m"}]
    """

    private static let crashingPodsJSON = """
    [{"Name":"checkout-api-7d9f8c","Namespace":"payments","Phase":"Running","Reason":"CrashLoopBackOff","NodeName":"demo-worker-1","Restarts":3,"Ready":false,"Uptime":"12m"},{"Name":"metrics-server","Namespace":"kube-system","Phase":"Pending","Reason":"ImagePullBackOff","NodeName":"","Restarts":2,"Ready":false,"Uptime":"8m"}]
    """

    private static let nodesJSON = """
    [{"Name":"demo-control-1","Ready":true,"MemoryPressure":false,"DiskPressure":false,"PIDPressure":false,"CPUCapacity":"4","MemoryCapacity":"8Gi","KubeletVersion":"v1.29.4","InternalIP":"LAN: 10.0.1.10","IPs":["10.0.1.10"],"LANIPs":["10.0.1.10"],"WANIPs":[],"TunnelIPs":[],"Roles":["control-plane"],"ControlPlane":true,"Labels":{"node-role.kubernetes.io/control-plane":""},"Unschedulable":false},{"Name":"demo-worker-1","Ready":true,"MemoryPressure":false,"DiskPressure":false,"PIDPressure":false,"CPUCapacity":"8","MemoryCapacity":"16Gi","KubeletVersion":"v1.29.4","InternalIP":"LAN: 10.0.1.21","IPs":["10.0.1.21"],"LANIPs":["10.0.1.21"],"WANIPs":[],"TunnelIPs":[],"Roles":["worker"],"ControlPlane":false,"Labels":{"kubepilot.io/lan-ip":"10.0.1.21"},"Unschedulable":false},{"Name":"demo-worker-2","Ready":true,"MemoryPressure":false,"DiskPressure":false,"PIDPressure":false,"CPUCapacity":"8","MemoryCapacity":"16Gi","KubeletVersion":"v1.29.4","InternalIP":"LAN: 10.0.1.22","IPs":["10.0.1.22"],"LANIPs":["10.0.1.22"],"WANIPs":[],"TunnelIPs":[],"Roles":["worker"],"ControlPlane":false,"Labels":{"kubepilot.io/lan-ip":"10.0.1.22"},"Unschedulable":false}]
    """

    private static let deploymentsJSON = """
    [{"Name":"checkout-api","Namespace":"payments","Replicas":2,"ReadyReplicas":1,"AvailableReplicas":1,"Image":"ghcr.io/demo/checkout:1.4.2"},{"Name":"redis-cart","Namespace":"payments","Replicas":1,"ReadyReplicas":1,"AvailableReplicas":1,"Image":"redis:7.2"}]
    """

    private static let namespacesJSON = """
    [{"Name":"payments","Status":"Active","Labels":{"env":"demo"}},{"Name":"kube-system","Status":"Active","Labels":{}},{"Name":"default","Status":"Active","Labels":{}}]
    """

    private static let eventsJSON = """
    {"items":[{"reason":"BackOff","message":"Back-off restarting failed container checkout","type":"Warning","count":12,"first_seen":"2026-09-28T23:20:00Z","last_seen":"2026-09-29T00:00:00Z","involved_object":{"kind":"Pod","name":"checkout-api-7d9f8c","namespace":"payments"},"source":"kubelet"},{"reason":"Unhealthy","message":"Readiness probe failed: connection refused","type":"Warning","count":4,"first_seen":"2026-09-28T23:45:00Z","last_seen":"2026-09-29T00:00:00Z","involved_object":{"kind":"Pod","name":"checkout-api-7d9f8c","namespace":"payments"},"source":"kubelet"}],"total":2}
    """

    private static let troubleshootingJSON = """
    {"namespace":"payments","generated_at":"2026-09-29T00:00:00.000Z","health_summary":{"not_ready_nodes":0,"crashloop_pods":1,"failed_mount_events":0,"pending_pods":1,"warning_events":8,"recommended_actions":["Inspect checkout-api CrashLoopBackOff","Fix metrics-server image pull"]},"insights":[{"id":"insight-crashloop","category":"workload","severity":"critical","title":"checkout-api is crash looping","summary":"Container exits with code 1 after failing to reach redis-cart on the readiness probe window.","suggestions":["Check REDIS_URL","Review recent config change","kubectl logs -p checkout-api-7d9f8c"],"affected_resources":["payments/checkout-api-7d9f8c"]}],"nodes":[{"name":"demo-worker-1","ready":true,"cpu_capacity":"8","memory_capacity":"16Gi","cpu_usage":"1.2","memory_usage":"6Gi","cpu_usage_percent":15,"memory_usage_percent":38,"disk_pressure":false,"memory_pressure":false,"pid_pressure":false,"unschedulable":false,"kubelet_version":"v1.29.4","roles":["worker"],"control_plane":false}],"resource_pressure":{"metrics_available":true,"cpu_usage_percent":22,"memory_usage_percent":41,"memory_pressure_nodes":0,"disk_pressure_nodes":0,"pid_pressure_nodes":0},"problem_pods":[{"name":"checkout-api-7d9f8c","namespace":"payments","status":"CrashLoopBackOff","restarts":3,"node":"demo-worker-1","reason":"CrashLoopBackOff","age_minutes":12,"message":"container exit code 1"},{"name":"metrics-server","namespace":"kube-system","status":"Pending","restarts":2,"node":"","reason":"ImagePullBackOff","age_minutes":8,"message":"Failed to pull image"}]}
    """

    private static let aiHealthJSON = """
    {"healthy":true,"model":"demo-llama3.2:3b","base_url":"http://demo-llm.local/v1","latency_ms":42,"error":null}
    """

    private static let rcaListJSON = """
    [{"id":"rca-demo-checkout","timestamp":"2026-09-28T23:40:00.000Z","target_resource":"payments/checkout-api","severity":"critical","root_cause":"Misconfigured REDIS_HOST causes CrashLoopBackOff on one checkout replica","evidence_chain":["Pod logs: connection refused on 127.0.0.1:6379","Events: BackOff restarting failed container","Sibling replica healthy with Service DNS"],"remediation":["Update REDIS_HOST to redis-cart.payments.svc","Rollout restart deploy/checkout-api"],"confidence":0.91,"status":"open"}]
    """

    private static let rcaOneJSON = """
    {"id":"rca-demo-checkout","timestamp":"2026-09-28T23:40:00.000Z","target_resource":"payments/checkout-api","severity":"critical","root_cause":"Misconfigured REDIS_HOST causes CrashLoopBackOff on one checkout replica","evidence_chain":["Pod logs: connection refused on 127.0.0.1:6379","Events: BackOff restarting failed container","Sibling replica healthy with Service DNS"],"remediation":["Update REDIS_HOST to redis-cart.payments.svc","Rollout restart deploy/checkout-api"],"confidence":0.91,"status":"open"}
    """

    private static let anomaliesJSON = """
    [{"id":"anom-demo-1","detected_at":"2026-09-28T23:42:00.000Z","rule":"crashloop_threshold","severity":"critical","description":"Pod restart rate exceeded threshold","rca_report_id":"rca-demo-checkout","resource":{"kind":"Pod","name":"checkout-api-7d9f8c","namespace":"payments"}}]
    """

    private static let autopilotJSON = """
    {"enabled":true,"policy":{"mode":"dry-run","min_confidence":0.8,"allowed_actions":["restart_pod","scale"],"max_risk":"low","allowed_namespaces":["payments"],"blocked_namespaces":["kube-system"],"cooldown":60000000000,"max_actions_per_hour":10},"decisions":[{"time":"2026-09-28T23:50:00.000Z","report_id":"rca-demo-checkout","resource":{"kind":"Pod","name":"checkout-api-7d9f8c","namespace":"payments"},"severity":"critical","confidence":0.88,"root_cause":"CrashLoopBackOff from bad REDIS_HOST","action":"restart_pod","verdict":"dry-run","reason":"Demo mode — would restart after env fix","output":null}],"stats":{"executed":0,"dry-run":3,"skipped":1}}
    """

    private static let kubeconfigsJSON = """
    {"active_path":"/demo/kubeconfig","active_context":"demo-payments","paths":["/demo/kubeconfig"],"contexts":[{"name":"demo-payments","cluster":"demo","user":"apple-review"}]}
    """

    private static let diagnosticsJSON = """
    {"diagnostics":{"name":"checkout-api-7d9f8c","namespace":"payments","phase":"Running","node_name":"demo-worker-1","created_at":"2026-09-28T23:48:00Z","conditions":[{"type":"Ready","status":"False","reason":"ContainersNotReady","message":"containers with unready status: [checkout]"}],"container_statuses":[{"name":"checkout","image":"ghcr.io/demo/checkout:1.4.2","ready":false,"restart_count":3,"state":"waiting","state_reason":"CrashLoopBackOff","state_message":"back-off 5m0s restarting failed container","exit_code":1}],"events":[],"labels":{"app":"checkout-api"},"owner_chain":[{"kind":"ReplicaSet","name":"checkout-api-7d9f8c","uid":"demo-rs"}]},"logs":"ERROR redis: connection refused (127.0.0.1:6379)\\nINFO retrying in 2s…\\nFATAL unable to start checkout listener\\n"}
    """

    private static let troubleshootJSON = """
    {"pod_name":"checkout-api-7d9f8c","namespace":"payments","root_cause":"Checkout container cannot reach redis-cart; readiness probes fail and kubelet restarts the pod.","analysis":"Demo RCA: REDIS_HOST still points at localhost inside the pod network namespace. Other replica on worker-2 is healthy because it uses the Service DNS name.","actions":[{"type":"edit_env","namespace":"payments","resource":"checkout-api","replicas":null,"command":"kubectl -n payments set env deploy/checkout-api REDIS_HOST=redis-cart","explanation":"Point checkout at the redis-cart Service","requires_cr_code":false}]}
    """

    private static let logText = """
    2026-09-29T00:00:00Z ERROR redis: connection refused (127.0.0.1:6379)
    2026-09-29T00:00:00Z INFO retrying in 2s…
    2026-09-29T00:00:00Z FATAL unable to start checkout listener
    # demo fixture logs for checkout-api-7d9f8c
    """
}
