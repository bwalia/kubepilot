/**
 * TopConsumers — the heaviest pods by CPU, memory and storage, side by side.
 *
 * Sits directly under the cluster resource meters: the meters say how full the
 * cluster is, this says who is filling it. Each row opens the pod drawer on
 * the dashboard; "View all" jumps to the full ranked list.
 */
import { useRouter } from "next/router";
import { Cpu, MemoryStick, HardDrive, ArrowRight, Flame, Info } from "lucide-react";
import { requestDashboardTarget } from "@/lib/dashboardNav";
import { useThemeColors } from "@/lib/useThemeColors";
import {
  usePodUsage,
  groupUsage,
  topBy,
  metricValue,
  formatMetric,
  pressure,
  type UsageMetric,
  type UsageRow,
} from "./usage";

const METRICS: { key: UsageMetric; label: string; icon: typeof Cpu }[] = [
  { key: "cpu", label: "CPU", icon: Cpu },
  { key: "memory", label: "Memory", icon: MemoryStick },
  { key: "storage", label: "Storage", icon: HardDrive },
];

interface Props {
  namespace: string;
  /** Rows per metric. */
  limit?: number;
  /** Called by "View all"; defaults to opening the dashboard's usage section. */
  onViewAll?: () => void;
}

export function TopConsumers({ namespace, limit = 5, onViewAll }: Props) {
  const router = useRouter();
  const { data, isLoading, error } = usePodUsage(namespace);
  const tc = useThemeColors(["accent", "warning", "danger", "info"]);
  const colors: Record<UsageMetric, string> = {
    cpu: tc.accent || "#0D6F8A",
    memory: tc.info || "#6B28D6",
    storage: tc.warning || "#B45309",
  };

  const rows = groupUsage(data?.pods ?? [], "pod");

  const openPod = (row: UsageRow) => {
    requestDashboardTarget({ type: "pod", namespace: row.namespace, name: row.name });
    if (router.pathname !== "/dashboard") void router.push("/dashboard");
  };
  const viewAll =
    onViewAll ??
    (() => {
      requestDashboardTarget({ type: "section", section: "usage" });
      if (router.pathname !== "/dashboard") void router.push("/dashboard");
    });

  const scope = namespace === "" ? "all namespaces" : namespace;

  return (
    <section>
      <div className="flex flex-wrap items-center justify-between gap-2 mb-3">
        <h2 className="text-base font-bold text-pilot-text-primary flex items-center gap-2">
          <Flame className="w-5 h-5 text-pilot-warning" />
          Top consumers
          <span className="font-mono text-sm font-medium text-pilot-muted">· {scope}</span>
        </h2>
        <button
          type="button"
          onClick={viewAll}
          className="inline-flex items-center gap-1.5 text-sm font-semibold text-pilot-accent hover:text-pilot-accent-light"
        >
          View full usage list <ArrowRight className="w-4 h-4" />
        </button>
      </div>

      {data && (!data.metrics_available || !data.storage_stats_available) && (
        <p className="mb-3 flex items-start gap-2 text-xs text-pilot-muted">
          <Info className="w-3.5 h-3.5 mt-0.5 shrink-0" />
          {!data.metrics_available && "metrics-server is not answering, so CPU and memory show zero. "}
          {!data.storage_stats_available && "Kubelet storage stats are unavailable, so storage shows claimed PVC size instead of bytes used."}
        </p>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        {METRICS.map(({ key, label, icon: Icon }) => {
          const top = topBy(rows, key, limit);
          const max = top.length ? metricValue(top[0], key) : 0;
          const color = colors[key];
          return (
            <div key={key} className="bg-pilot-surface border border-pilot-border rounded-xl p-4 shadow-card">
              <div className="flex items-center gap-2.5 mb-3">
                <span
                  className="inline-flex items-center justify-center w-8 h-8 rounded-lg bg-pilot-surface-2 border border-pilot-border"
                  style={{ color }}
                >
                  <Icon className="w-4 h-4" />
                </span>
                <span className="eyebrow">Top {label}</span>
              </div>

              {isLoading ? (
                <div className="space-y-2">
                  {[...Array(limit)].map((_, i) => (
                    <div key={i} className="h-9 bg-pilot-surface-2 rounded-lg animate-pulse" />
                  ))}
                </div>
              ) : error ? (
                <p className="text-sm text-pilot-danger py-4">Could not load usage.</p>
              ) : top.length === 0 ? (
                <p className="text-sm text-pilot-muted py-4">No {label.toLowerCase()} usage reported.</p>
              ) : (
                <ol className="space-y-1">
                  {top.map((row, i) => {
                    const v = metricValue(row, key);
                    const p = pressure(row, key);
                    const hot = p && p.of === "limit" && p.pct >= 85;
                    return (
                      <li key={row.key}>
                        <button
                          type="button"
                          onClick={() => openPod(row)}
                          title={`${row.namespace}/${row.name}`}
                          className="w-full text-left rounded-lg px-2 py-1.5 hover:bg-pilot-surface-2 transition-colors"
                        >
                          <div className="flex items-baseline justify-between gap-3">
                            <span className="min-w-0 truncate text-sm">
                              <span className="text-pilot-muted tabular-nums mr-1.5">{i + 1}.</span>
                              <span className="font-mono text-pilot-text-primary">{row.name}</span>
                            </span>
                            <span className="shrink-0 font-mono text-sm font-semibold tabular-nums" style={{ color }}>
                              {formatMetric(key, v)}
                            </span>
                          </div>
                          <div className="mt-1 flex items-center gap-2">
                            <div className="h-1.5 flex-1 rounded-full bg-pilot-surface-2 overflow-hidden">
                              <div
                                className="h-full rounded-full"
                                style={{ width: `${max ? Math.max(3, (v / max) * 100) : 0}%`, background: color }}
                              />
                            </div>
                            <span className="w-28 shrink-0 truncate text-right text-2xs text-pilot-muted font-mono">
                              {row.namespace}
                            </span>
                          </div>
                          {p && (
                            <div className={`mt-0.5 text-2xs ${hot ? "text-pilot-danger font-semibold" : "text-pilot-muted"}`}>
                              {Math.round(p.pct)}% of {p.of}
                            </div>
                          )}
                          {key === "storage" && row.storageEstimated && (
                            <div className="mt-0.5 text-2xs text-pilot-muted">includes claimed PVC size (live usage not measurable)</div>
                          )}
                        </button>
                      </li>
                    );
                  })}
                </ol>
              )}
            </div>
          );
        })}
      </div>
    </section>
  );
}
