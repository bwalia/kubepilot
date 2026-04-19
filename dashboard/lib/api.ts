/**
 * KubePilot API client — typed wrappers around the Go backend REST API.
 * All requests target /api/v1/* which is proxied to the Go server in dev,
 * or served directly by the embedded Go dashboard server in production.
 */
import axios from "axios";

const BASE = "/api/v1";

const http = axios.create({ baseURL: BASE, timeout: 180_000 });

// ─────────────────────────────────────────
// Types mirroring Go backend structs
// ─────────────────────────────────────────

export interface PodSummary {
  Name: string;
  Namespace: string;
  Phase: string;
  Reason: string;
  NodeName: string;
  Restarts: number;
  Ready: boolean;
}

export interface DeploymentSummary {
  Name: string;
  Namespace: string;
  Replicas: number;
  ReadyReplicas: number;
  AvailableReplicas: number;
  Image: string;
}

export interface NodeSummary {
  Name: string;
  Ready: boolean;
  MemoryPressure: boolean;
  DiskPressure: boolean;
  PIDPressure: boolean;
  CPUCapacity: string;
  MemoryCapacity: string;
  KubeletVersion: string;
  Unschedulable: boolean;
}

export interface KubeEvent {
  reason: string;
  message: string;
  type: string;
  count: number;
  first_seen: string;
  last_seen: string;
  involved_object: {
    kind: string;
    name: string;
    namespace: string;
  };
  source: string;
}

export interface EventListResponse {
  items: KubeEvent[];
  total: number;
}

export interface TroubleshootingInsight {
  id: string;
  category: string;
  severity: string;
  title: string;
  summary: string;
  suggestions: string[];
  affected_resources?: string[];
}

export interface NodeHealthRow {
  name: string;
  ready: boolean;
  cpu_capacity: string;
  memory_capacity: string;
  cpu_usage?: string;
  memory_usage?: string;
  cpu_usage_percent?: number;
  memory_usage_percent?: number;
  disk_pressure: boolean;
  memory_pressure: boolean;
  pid_pressure: boolean;
  unschedulable: boolean;
  kubelet_version: string;
}

export interface ResourcePressureSummary {
  metrics_available: boolean;
  cpu_usage_percent?: number;
  memory_usage_percent?: number;
  memory_pressure_nodes: number;
  disk_pressure_nodes: number;
  pid_pressure_nodes: number;
  cpu_usage_milli?: number;
  cpu_capacity_milli?: number;
  memory_usage_bytes?: number;
  memory_capacity_bytes?: number;
  storage_usage_percent?: number;
  storage_bound_bytes?: number;
  storage_capacity_bytes?: number;
  storage_pvc_count?: number;
  storage_pvc_bound?: number;
}

export interface ProblemPod {
  name: string;
  namespace: string;
  status: string;
  restarts: number;
  node: string;
  reason: string;
  age_minutes: number;
  message?: string;
}

export interface HealthSummary {
  not_ready_nodes: number;
  crashloop_pods: number;
  failed_mount_events: number;
  pending_pods: number;
  warning_events: number;
  recommended_actions: string[];
}

export interface ClusterTroubleshootingSummary {
  namespace: string;
  generated_at: string;
  health_summary: HealthSummary;
  insights: TroubleshootingInsight[];
  nodes: NodeHealthRow[];
  resource_pressure: ResourcePressureSummary;
  problem_pods: ProblemPod[];
}

export interface PodCondition {
  type: string;
  status: string;
  reason?: string;
  message?: string;
}

export interface ContainerDiag {
  name: string;
  image: string;
  ready: boolean;
  restart_count: number;
  state: string;
  state_reason: string;
  state_message: string;
  exit_code: number;
  last_terminated_reason?: string;
}

export interface ResourceMetrics {
  cpu_request: string;
  cpu_limit: string;
  mem_request: string;
  mem_limit: string;
  cpu_usage: string;
  mem_usage: string;
}

export interface PodDiagnostics {
  name: string;
  namespace: string;
  phase: string;
  node_name: string;
  service_account: string;
  created_at: string;
  conditions: PodCondition[];
  container_statuses: ContainerDiag[];
  events: KubeEvent[];
  resource_usage?: ResourceMetrics;
  owner_chain: Array<{ kind: string; name: string; uid: string }>;
  labels: Record<string, string>;
  annotations: Record<string, string>;
  tolerations?: string[];
  node_selector?: Record<string, string>;
  volumes?: string[];
}

export interface PodDiagnosticsResponse {
  diagnostics: PodDiagnostics;
  logs: string;
}

export interface ContextInfo {
  name: string;
  cluster: string;
  user: string;
}

export interface KubeconfigState {
  active_path: string;
  active_context: string;
  paths: string[];
  contexts?: ContextInfo[];
}

export type ActionType =
  | "scale"
  | "restart"
  | "delete_pod"
  | "investigate"
  | "noop"
  | "custom_job";

export interface SuggestedAction {
  type: ActionType;
  namespace?: string;
  resource?: string;
  replicas?: number;
  command?: string;
  explanation: string;
  requires_cr_code: boolean;
}

export interface ExecuteActionResult {
  status: "executed" | "skipped";
  message: string;
}

export interface TroubleshootReport {
  PodName: string;
  Namespace: string;
  RootCause: string;
  Analysis: string;
  Actions: SuggestedAction[];
}

interface TroubleshootReportWire {
  pod_name?: string;
  namespace?: string;
  root_cause?: string;
  analysis?: string;
  actions?: SuggestedAction[];
}

export type JobStatus =
  | "pending"
  | "running"
  | "done"
  | "failed"
  | "blocked";

export interface Job {
  ID: string;
  Name: string;
  Command: string;
  Schedule: string;
  TargetEnv: string;
  ChangeID: string;
  Status: JobStatus;
  CreatedAt: string;
  LastRunAt?: string;
  LastResult: string;
  Actions: SuggestedAction[];
}

export interface SubmitJobRequest {
  name: string;
  command: string;
  schedule?: string;
  target_environment: string;
  change_id?: string;
  cr_code?: string;
}

// ─────────────────────────────────────────
// Cluster API
// ─────────────────────────────────────────

export const listPods = (namespace = ""): Promise<PodSummary[]> =>
  http.get("/clusters/pods", { params: { namespace } }).then((r) => r.data);

export const listCrashingPods = (namespace = ""): Promise<PodSummary[]> =>
  http
    .get("/clusters/crashing-pods", { params: { namespace } })
    .then((r) => r.data);

export const listDeployments = (namespace = ""): Promise<DeploymentSummary[]> =>
  http
    .get("/clusters/deployments", { params: { namespace } })
    .then((r) => r.data);

export const listNodes = (): Promise<NodeSummary[]> =>
  http.get("/clusters/nodes").then((r) => r.data);

export const listClusterEvents = (params?: {
  namespace?: string;
  kind?: string;
  type?: string;
  search?: string;
  sort?: "asc" | "desc";
  limit?: number;
  since?: string;
}): Promise<EventListResponse> =>
  http.get("/events", { params }).then((r) => r.data);

export const getClusterTroubleshootingSummary = (
  namespace = ""
): Promise<ClusterTroubleshootingSummary> =>
  http.get("/troubleshooting/summary", { params: { namespace } }).then((r) => r.data);

export const getPodDiagnostics = (
  namespace: string,
  pod: string,
  tailLines = 200
): Promise<PodDiagnosticsResponse> =>
  http
    .get(`/clusters/pods/${encodeURIComponent(namespace)}/${encodeURIComponent(pod)}/diagnostics`, {
      params: { tail_lines: tailLines },
    })
    .then((r) => r.data);

export const listKubeconfigs = (): Promise<KubeconfigState> =>
  http.get("/clusters/kubeconfigs").then((r) => r.data);

export const addKubeconfigPath = (
  path: string,
  activate = true
): Promise<KubeconfigState> =>
  http.post("/clusters/kubeconfigs", { path, activate }).then((r) => r.data);

export const switchCluster = (path: string): Promise<KubeconfigState> =>
  http.post("/clusters/switch", { path }).then((r) => r.data);

export const switchContext = (context: string): Promise<KubeconfigState> =>
  http.post("/clusters/switch-context", { context }).then((r) => r.data);

export const uploadKubeconfig = async (
  file: File
): Promise<KubeconfigState> => {
  const form = new FormData();
  form.append("kubeconfig", file);
  const resp = await http.post("/clusters/kubeconfigs/upload", form, {
    headers: { "Content-Type": "multipart/form-data" },
  });
  return resp.data;
};

export const uploadKubeconfigBase64 = (
  contentBase64: string,
  name?: string
): Promise<KubeconfigState> =>
  http
    .post("/clusters/kubeconfigs/base64", {
      content_base64: contentBase64,
      name,
    })
    .then((r) => r.data);

// ─────────────────────────────────────────
// AI API
// ─────────────────────────────────────────

export interface AIHealthStatus {
  healthy: boolean;
  model: string;
  base_url: string;
  latency_ms: number;
  error?: string;
}

export const getAIHealth = (): Promise<AIHealthStatus> =>
  http.get("/ai/health").then((r) => r.data);

export const interpretCommand = (
  command: string
): Promise<{ actions: SuggestedAction[] }> =>
  http.post("/ai/interpret", { command }).then((r) => r.data);

export const troubleshootPod = (
  namespace: string,
  pod: string
): Promise<TroubleshootReport> =>
  http.get(`/ai/troubleshoot/${namespace}/${pod}`).then((r) => {
    const wire = (r.data || {}) as TroubleshootReportWire;
    return {
      PodName: wire.pod_name ?? pod,
      Namespace: wire.namespace ?? namespace,
      RootCause: wire.root_cause ?? "",
      Analysis: wire.analysis ?? "",
      Actions: Array.isArray(wire.actions) ? wire.actions : [],
    };
  });

export const executeSuggestedAction = (
  action: SuggestedAction,
  changeId?: string,
  crCode?: string
): Promise<ExecuteActionResult> =>
  http
    .post("/ai/execute-action", {
      action,
      change_id: changeId,
      cr_code: crCode,
    })
    .then((r) => r.data);

// ─────────────────────────────────────────
// Jobs API
// ─────────────────────────────────────────

export const listJobs = (): Promise<Job[]> =>
  http.get("/jobs").then((r) => r.data);

export const getJob = (id: string): Promise<Job> =>
  http.get(`/jobs/${id}`).then((r) => r.data);

export const submitJob = (req: SubmitJobRequest): Promise<Job> =>
  http.post("/jobs", req).then((r) => r.data);

export const cancelJob = (id: string): Promise<void> =>
  http.post(`/jobs/${id}/cancel`).then(() => undefined);

// ─────────────────────────────────────────
// CR Code API
// ─────────────────────────────────────────

export const authorizeCRCode = (changeId: string, crCode: string) =>
  http.post("/crcode/authorize", { change_id: changeId, cr_code: crCode }).then((r) => r.data);

export const registerCRCode = (
  changeId: string,
  crCode: string,
  expiresAt?: string
) =>
  http
    .post("/crcode/register", { change_id: changeId, cr_code: crCode, expires_at: expiresAt })
    .then((r) => r.data);

export const revokeCRCode = (changeId: string) =>
  http.post("/crcode/revoke", { change_id: changeId }).then((r) => r.data);

// ─────────────────────────────────────────
// RCA & Anomaly types
// ─────────────────────────────────────────

export type Severity = "critical" | "high" | "medium" | "low" | "info";
export type RCAStatus = "pending" | "analyzing" | "complete" | "action_taken" | "failed";

export interface ResourceRef {
  kind: string;
  name: string;
  namespace: string;
}

export interface RootCause {
  category: string;
  summary: string;
  detail: string;
  affected_components?: string[];
}

export interface Evidence {
  source: string;
  data: string;
  relevance: string;
  timestamp: string;
}

export interface RemediationStep {
  order: number;
  action: string;
  description: string;
  command?: string;
  risk: string;
  auto_apply: boolean;
  requires_cr: boolean;
}

export interface RCAReport {
  id: string;
  timestamp: string;
  target_resource: ResourceRef;
  severity: Severity;
  root_cause: RootCause;
  evidence_chain: Evidence[];
  remediation: RemediationStep[];
  confidence: number;
  status: RCAStatus;
}

export interface Anomaly {
  id: string;
  detected_at: string;
  rule: string;
  resource: ResourceRef;
  severity: Severity;
  description: string;
  rca_report_id?: string;
}

export interface ServiceNode {
  name: string;
  namespace: string;
  pods: string[];
  healthy: boolean;
  anomalies?: string[];
}

export interface DependencyEdge {
  from: string;
  to: string;
  type: string;
  port: number;
}

export interface ServiceTopology {
  services: ServiceNode[];
  edges: DependencyEdge[];
}

  // ─────────────────────────────────────────
  // Service Graph (ArgoCD-style canvas)
  // ─────────────────────────────────────────

  export type SGNodeKind =
    | "Ingress"
    | "Service"
    | "Deployment"
    | "StatefulSet"
    | "DaemonSet"
    | "Pod";

  export interface SGPortInfo {
    name: string;
    port: number;
    target_port: string;
    protocol: string;
    node_port?: number;
  }

  export interface SGNode {
    id: string;
    kind: SGNodeKind;
    name: string;
    namespace: string;
    status: "healthy" | "degraded" | "pending" | "unknown";
    labels?: Record<string, string>;
    // service
    service_type?: string;
    ports?: SGPortInfo[];
    cluster_ip?: string;
    external_ips?: string[];
    // workload
    replicas?: number;
    ready_replicas?: number;
    image?: string;
    // pod
    phase?: string;
    ready?: boolean;
    restarts?: number;
    node_name?: string;
    // ingress
    host?: string;
    ingress_url?: string;
    tls?: boolean;
  }

  export interface SGEdge {
    from: string;
    to: string;
  }

  export interface ServiceGraph {
    namespace: string;
    nodes: SGNode[];
    edges: SGEdge[];
  }


export interface RemediationResult {
  step_index: number;
  action: string;
  success: boolean;
  output: string;
  error?: string;
  executed_at: string;
  dry_run: boolean;
}

// ─────────────────────────────────────────
// RCA & Anomaly API
// ─────────────────────────────────────────

export const listRCAReports = (params?: {
  severity?: string;
  namespace?: string;
  since?: string;
}): Promise<RCAReport[]> =>
  http.get("/rca", { params }).then((r) => r.data);

export const getRCAReport = (id: string): Promise<RCAReport> =>
  http.get(`/rca/${id}`).then((r) => r.data);

export const listAnomalies = (params?: {
  severity?: string;
  namespace?: string;
  since?: string;
}): Promise<Anomaly[]> =>
  http.get("/anomalies", { params }).then((r) => r.data);

export const getTopology = (namespace: string): Promise<ServiceTopology> =>
  http.get(`/topology/${namespace}`).then((r) => r.data);

  export const getServiceGraph = (namespace: string): Promise<ServiceGraph> =>
    http
      .get("/clusters/service-graph", { params: { namespace } })
      .then((r) => r.data);

// ─────────────────────────────────────────
// Runbooks API
// ─────────────────────────────────────────

export interface Runbook {
  id: string;
  name: string;
  description: string;
  category: string;
  steps: string[];
  actions: string[];
  risk: string;
}

export interface RunbookStepResult {
  status: "ok" | "error" | "manual";
  message: string;
}

export const listRunbooks = (): Promise<Runbook[]> =>
  http.get("/runbooks").then((r) => r.data);

export const executeRunbookStep = (
  runbookId: string,
  step: number,
  params?: Record<string, string>
): Promise<{ runbook_id: string; step: number; result: RunbookStepResult }> =>
  http
    .post("/runbooks/execute", {
      runbook_id: runbookId,
      step,
      params: params || {},
    })
    .then((r) => r.data);

export const executeRemediation = (
  reportId: string,
  step: number,
  changeId?: string,
  crCode?: string
): Promise<RemediationResult> =>
  http
    .post("/remediate", {
      report_id: reportId,
      step,
      change_id: changeId,
      cr_code: crCode,
    })
    .then((r) => r.data);
