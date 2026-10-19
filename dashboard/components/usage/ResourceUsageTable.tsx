/**
 * ResourceUsageTable — every pod (or workload, namespace, node) ranked by live
 * CPU, memory or storage consumption, with usage against requests and limits.
 *
 * Built for SREs hunting heavy hitters: pick a metric to rank by, roll pods up
 * to the level you care about, and spot the rows running hot against their
 * limits or with no limits at all.
 */
import { useMemo, useState } from "react";
import { Cpu, MemoryStick, HardDrive, Info } from "lucide-react";
import { ResourceTable, type Column } from "@/components/dashboard/ResourceTable";
import { cn } from "@/lib/utils";
import {
  usePodUsage,
  groupUsage,
  metricValue,
  pressure,
  formatCPU,
  formatBytes,
  type UsageGroup,
  type UsageMetric,
  type UsageRow,
} from "./usage";

const METRICS: { key: UsageMetric; label: string; icon: typeof Cpu }[] = [
  { key: "cpu", label: "CPU", icon: Cpu },
  { key: "memory", label: "Memory", icon: MemoryStick },
  { key: "storage", label: "Storage", icon: HardDrive },
];

const GROUPS: { key: UsageGroup; label: string }[] = [
  { key: "pod", label: "Pods" },
  { key: "workload", label: "Workloads" },
  { key: "namespace", label: "Namespaces" },
  { key: "node", label: "Nodes" },
];

interface Props {
  namespace: string;
  /** Opens a pod's details when a single-pod row is clicked. */
  onPodClick?: (namespace: string, name: string) => void;
}

export function ResourceUsageTable({ namespace, onPodClick }: Props) {
  const [metric, setMetric] = useState<UsageMetric>("cpu");
  const [group, setGroup] = useState<UsageGroup>("pod");
  const { data, isLoading, error } = usePodUsage(namespace);

  const rows = useMemo(() => {
    const grouped = groupUsage(data?.pods ?? [], group);
    return grouped.sort((a, b) => metricValue(b, metric) - metricValue(a, metric));
  }, [data, group, metric]);

  // Rank by the chosen metric, kept even when a column header re-sorts the view.
  const rank = useMemo(() => new Map(rows.map((r, i) => [r.key, i + 1])), [rows]);

  const totals = useMemo(() => {
    const t = { cpu: 0, mem: 0, storage: 0 };
    for (const r of rows) {
      t.cpu += r.cpu;
      t.mem += r.mem;
      t.storage += r.storage;
    }
    return t;
  }, [rows]);

  const columns: Column<UsageRow>[] = [
    {
      header: "#",
      cell: (r) => <span className="text-pilot-muted tabular-nums">{rank.get(r.key)}</span>,
      hideBelow: "md",
    },
    {
      header: group === "pod" ? "Pod" : GROUPS.find((g) => g.key === group)!.label.replace(/s$/, ""),
      cell: (r) => (
        <div className="min-w-0">
          <div className="font-mono text-sm text-pilot-text-primary truncate max-w-[22rem]" title={r.name}>
            {r.name}
          </div>
          <div className="text-2xs text-pilot-muted truncate">
            {[
              r.namespace,
              group !== "namespace" && group !== "node" ? r.kind : null,
              group !== "pod" ? `${r.podCount} pod${r.podCount === 1 ? "" : "s"}` : null,
            ]
              .filter(Boolean)
              .join(" · ")}
          </div>
        </div>
      ),
      sortValue: (r) => r.name,
    },
    {
      header: "CPU",
      align: "right",
      cell: (r) => <UsageCell row={r} metric="cpu" emphasise={metric === "cpu"} />,
      sortValue: (r) => r.cpu,
    },
    {
      header: "Memory",
      align: "right",
      cell: (r) => <UsageCell row={r} metric="memory" emphasise={metric === "memory"} />,
      sortValue: (r) => r.mem,
    },
    {
      header: "Storage",
      align: "right",
      cell: (r) => <UsageCell row={r} metric="storage" emphasise={metric === "storage"} />,
      sortValue: (r) => r.storage,
    },
    {
      header: "Node",
      cell: (r) => <span className="font-mono text-xs text-pilot-text-secondary">{r.node || "—"}</span>,
      sortValue: (r) => r.node,
      hideBelow: "lg",
    },
    {
      header: "Restarts",
      align: "right",
      cell: (r) => (
        <span className={cn("tabular-nums", r.restarts > 0 ? "text-pilot-warning font-semibold" : "text-pilot-muted")}>
          {r.restarts}
        </span>
      ),
      sortValue: (r) => r.restarts,
      hideBelow: "md",
    },
  ];

  const toggle = (active: boolean) =>
    cn(
      "px-3 py-1.5 text-xs font-semibold rounded-md transition-colors inline-flex items-center gap-1.5",
      active
        ? "bg-pilot-accent text-pilot-bg"
        : "text-pilot-text-secondary hover:text-pilot-text-primary hover:bg-pilot-surface-2",
    );

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <Total label="CPU in use" value={formatCPU(totals.cpu)} />
        <Total label="Memory in use" value={formatBytes(totals.mem)} />
        <Total label="Storage in use" value={formatBytes(totals.storage)} />
      </div>

      {data && (!data.metrics_available || !data.storage_stats_available) && (
        <p className="flex items-start gap-2 text-xs text-pilot-muted">
          <Info className="w-3.5 h-3.5 mt-0.5 shrink-0" />
          {!data.metrics_available && "metrics-server is not answering, so CPU and memory show zero. "}
          {!data.storage_stats_available &&
            "Kubelet storage stats are unavailable (needs nodes/proxy access), so storage shows claimed PVC size."}
        </p>
      )}

      <div className="flex flex-wrap items-center gap-3">
        <div className="inline-flex rounded-lg border border-pilot-border bg-pilot-surface p-0.5" role="radiogroup" aria-label="Rank by">
          {METRICS.map(({ key, label, icon: Icon }) => (
            <button key={key} type="button" role="radio" aria-checked={metric === key} onClick={() => setMetric(key)} className={toggle(metric === key)}>
              <Icon className="w-3.5 h-3.5" /> {label}
            </button>
          ))}
        </div>
        <div className="inline-flex rounded-lg border border-pilot-border bg-pilot-surface p-0.5" role="radiogroup" aria-label="Group by">
          {GROUPS.map(({ key, label }) => (
            <button key={key} type="button" role="radio" aria-checked={group === key} onClick={() => setGroup(key)} className={toggle(group === key)}>
              {label}
            </button>
          ))}
        </div>
      </div>

      <ResourceTable
        key={`${group}-${metric}`}
        columns={columns}
        items={rows}
        rowKey={(r) => r.key}
        loading={isLoading}
        error={error}
        noun={GROUPS.find((g) => g.key === group)!.label.toLowerCase()}
        emptyMessage="No pods are running in this scope."
        searchText={(r) => `${r.name} ${r.namespace} ${r.kind} ${r.node}`}
        searchPlaceholder="Filter by name, namespace, kind or node…"
        onRowClick={onPodClick ? (r) => r.pod && onPodClick(r.namespace, r.name) : undefined}
      />
    </div>
  );
}

function Total({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-pilot-surface border border-pilot-border rounded-xl px-4 py-3 shadow-card">
      <div className="eyebrow">{label}</div>
      <div className="font-display text-2xl font-bold tabular-nums text-pilot-text-primary mt-1">{value}</div>
    </div>
  );
}

/** Usage figure plus a small bar against the limit (or request when unlimited). */
function UsageCell({ row, metric, emphasise }: { row: UsageRow; metric: UsageMetric; emphasise: boolean }) {
  const v = metricValue(row, metric);
  const p = pressure(row, metric);
  const fmt = metric === "cpu" ? formatCPU : formatBytes;
  const cap = metric === "cpu" ? row.cpuLimit : metric === "memory" ? row.memLimit : row.pvcCapacity;
  const tone =
    p && p.of === "limit" && p.pct >= 85
      ? "bg-pilot-danger"
      : p && p.pct >= 60
        ? "bg-pilot-warning"
        : "bg-pilot-success";
  const noLimit = metric !== "storage" && v > 0 && cap === 0;
  return (
    <div className="inline-flex flex-col items-end min-w-[6.5rem]">
      <span className={cn("font-mono tabular-nums", emphasise ? "text-sm font-bold text-pilot-text-primary" : "text-xs text-pilot-text-secondary")}>
        {v > 0 ? fmt(v) : "—"}
      </span>
      {p ? (
        <>
          <div className="mt-1 h-1 w-24 rounded-full bg-pilot-surface-2 overflow-hidden">
            <div className={cn("h-full rounded-full", tone)} style={{ width: `${Math.min(100, p.pct)}%` }} />
          </div>
          <span className="text-2xs text-pilot-muted mt-0.5">
            {Math.round(p.pct)}% of {p.of} {cap > 0 && p.of === "limit" ? `(${fmt(cap)})` : ""}
          </span>
        </>
      ) : noLimit ? (
        <span className="text-2xs text-pilot-warning mt-0.5">no limit set</span>
      ) : metric === "storage" && row.storageEstimated ? (
        <span className="text-2xs text-pilot-muted mt-0.5" title="Directory-backed volumes (e.g. local-path) cannot report their own usage">incl. claimed PVC size</span>
      ) : null}
    </div>
  );
}
