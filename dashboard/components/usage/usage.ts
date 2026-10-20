/**
 * Shared helpers for the resource-usage views: formatting, ranking and
 * rolling pods up into workloads, namespaces and nodes.
 */
import { useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { listPodUsage, type PodUsage, type PodUsageReport } from "@/lib/api";
import { qk } from "@/lib/queryKeys";

export type UsageMetric = "cpu" | "memory" | "storage";
export type UsageGroup = "pod" | "workload" | "namespace" | "node";

/** One row of the usage table — a pod, or a roll-up of several. */
export interface UsageRow {
  key: string;
  name: string;
  namespace: string;
  /** Workload kind for pods/workloads, "Namespace"/"Node" for roll-ups. */
  kind: string;
  node: string;
  podCount: number;
  restarts: number;
  cpu: number;
  cpuRequest: number;
  cpuLimit: number;
  mem: number;
  memRequest: number;
  memLimit: number;
  storage: number;
  pvcUsed: number;
  pvcCapacity: number;
  storageEstimated: boolean;
  /** Set only for single-pod rows; lets the row open the pod drawer. */
  pod?: PodUsage;
}

/** Rows each "Top consumers" card shows; also the size of the light query. */
export const TOP_N = 5;

const fullUsageQuery = (namespace: string) => ({
  queryKey: qk.podUsage(namespace),
  queryFn: () => listPodUsage(namespace),
  staleTime: 15_000,
});

/**
 * The full per-pod list, for the usage table. Until it arrives the table shows
 * the top consumers already in cache, so opening it never starts blank.
 */
export function usePodUsage(namespace: string) {
  const qc = useQueryClient();
  return useQuery({
    ...fullUsageQuery(namespace),
    refetchInterval: 30_000,
    placeholderData: () => qc.getQueryData<PodUsageReport>(qk.podUsageTop(namespace, TOP_N)),
  });
}

/** Only the top consumers — a few KB instead of the whole cluster. */
export function useTopPodUsage(namespace: string) {
  const qc = useQueryClient();
  return useQuery({
    queryKey: qk.podUsageTop(namespace, TOP_N),
    queryFn: () => listPodUsage(namespace, TOP_N),
    refetchInterval: 30_000,
    staleTime: 15_000,
    // If the full list is already cached (the table was open), start from it.
    placeholderData: () => qc.getQueryData<PodUsageReport>(qk.podUsage(namespace)),
  });
}

/** Warm the full list before the user gets there (hover / focus on "View all"). */
export function prefetchPodUsage(qc: QueryClient, namespace: string) {
  void qc.prefetchQuery(fullUsageQuery(namespace));
}

export function formatCPU(milli: number): string {
  if (!milli || milli <= 0) return "0";
  if (milli >= 1000) return `${(milli / 1000).toFixed(milli >= 10_000 ? 0 : 2)} cores`;
  return `${milli}m`;
}

export function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return "0";
  const units = ["B", "KiB", "MiB", "GiB", "TiB", "PiB"];
  let v = bytes;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(v >= 100 || i === 0 ? 0 : 1)} ${units[i]}`;
}

export function formatMetric(metric: UsageMetric, value: number): string {
  return metric === "cpu" ? formatCPU(value) : formatBytes(value);
}

export function metricValue(row: UsageRow, metric: UsageMetric): number {
  return metric === "cpu" ? row.cpu : metric === "memory" ? row.mem : row.storage;
}

/** Usage as a percentage of the limit, or of the request when unlimited. */
export function pressure(row: UsageRow, metric: UsageMetric): { pct: number; of: "limit" | "request" } | null {
  if (metric === "storage") {
    if (!row.pvcCapacity || row.storageEstimated) return null;
    return { pct: (row.pvcUsed / row.pvcCapacity) * 100, of: "limit" };
  }
  const used = metric === "cpu" ? row.cpu : row.mem;
  const limit = metric === "cpu" ? row.cpuLimit : row.memLimit;
  const request = metric === "cpu" ? row.cpuRequest : row.memRequest;
  if (limit > 0) return { pct: (used / limit) * 100, of: "limit" };
  if (request > 0) return { pct: (used / request) * 100, of: "request" };
  return null;
}

function podRow(p: PodUsage): UsageRow {
  return {
    key: `${p.namespace}/${p.name}`,
    name: p.name,
    namespace: p.namespace,
    kind: p.owner_kind || "Pod",
    node: p.node,
    podCount: 1,
    restarts: p.restarts,
    cpu: p.cpu_usage_milli,
    cpuRequest: p.cpu_request_milli,
    cpuLimit: p.cpu_limit_milli,
    mem: p.mem_usage_bytes,
    memRequest: p.mem_request_bytes,
    memLimit: p.mem_limit_bytes,
    storage: p.storage_used_bytes,
    pvcUsed: p.pvc_used_bytes,
    pvcCapacity: p.pvc_capacity_bytes,
    storageEstimated: !!p.storage_estimated,
    pod: p,
  };
}

/** Rolls pods up by the chosen grouping. "pod" returns one row per pod. */
export function groupUsage(pods: PodUsage[], group: UsageGroup): UsageRow[] {
  if (group === "pod") return pods.map(podRow);
  const out = new Map<string, UsageRow>();
  for (const p of pods) {
    let key: string;
    let name: string;
    let namespace = p.namespace;
    let kind: string;
    if (group === "workload") {
      kind = p.owner_kind || "Pod";
      name = p.owner_name || p.name;
      key = `${p.namespace}/${kind}/${name}`;
    } else if (group === "namespace") {
      kind = "Namespace";
      name = p.namespace;
      key = p.namespace;
    } else {
      kind = "Node";
      name = p.node || "(unscheduled)";
      namespace = "";
      key = name;
    }
    const r = podRow(p);
    const acc = out.get(key);
    if (!acc) {
      out.set(key, { ...r, key, name, namespace, kind, node: group === "node" ? name : p.node, pod: undefined });
      continue;
    }
    acc.podCount += 1;
    acc.restarts += r.restarts;
    acc.cpu += r.cpu;
    acc.cpuRequest += r.cpuRequest;
    acc.cpuLimit += r.cpuLimit;
    acc.mem += r.mem;
    acc.memRequest += r.memRequest;
    acc.memLimit += r.memLimit;
    acc.storage += r.storage;
    acc.pvcUsed += r.pvcUsed;
    acc.pvcCapacity += r.pvcCapacity;
    acc.storageEstimated = acc.storageEstimated || r.storageEstimated;
    if (acc.node !== r.node && group !== "node") acc.node = "multiple";
  }
  return [...out.values()];
}

export function topBy(rows: UsageRow[], metric: UsageMetric, n: number): UsageRow[] {
  return [...rows]
    .filter((r) => metricValue(r, metric) > 0)
    .sort((a, b) => metricValue(b, metric) - metricValue(a, metric))
    .slice(0, n);
}
