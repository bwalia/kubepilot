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
  explanation: string;
  requires_cr_code: boolean;
}

export interface TroubleshootReport {
  PodName: string;
  Namespace: string;
  RootCause: string;
  Analysis: string;
  Actions: SuggestedAction[];
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

// ─────────────────────────────────────────
// AI API
// ─────────────────────────────────────────

export const interpretCommand = (
  command: string
): Promise<{ actions: SuggestedAction[] }> =>
  http.post("/ai/interpret", { command }).then((r) => r.data);

export const troubleshootPod = (
  namespace: string,
  pod: string
): Promise<TroubleshootReport> =>
  http.get(`/ai/troubleshoot/${namespace}/${pod}`).then((r) => r.data);

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
