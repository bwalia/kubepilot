import { useQuery } from "@tanstack/react-query";
import { qk } from "@/lib/queryKeys";
import { getClusterTroubleshootingSummary, type NodeHealthRow, type ProblemPod } from "@/lib/api";
import { NodeIPDisplay } from "@/components/NodeIPDisplay";
import { NodeRoleBadge } from "@/components/NodeRoleBadge";
import { ResourceMeters } from "@/components/dashboard/ResourceMeters";
import { ResourceTable, type Column } from "./ResourceTable";
import { Badge } from "@/components/ui/badge";
import { AlertTriangle, Activity } from "lucide-react";
import { InsightActions } from "@/components/AIReportActions";
import { StatusPill } from "@/components/ui/StatusPill";
import { explainPod, explainNode } from "@/lib/k8sExplain";

export function ClusterHealthSection({ namespace }: { namespace: string }) {
  const { data, isLoading } = useQuery({
    queryKey: qk.health(namespace),
    queryFn: () => getClusterTroubleshootingSummary(namespace),
  });

  const health = data?.health_summary;

  return (
    <div className="space-y-6">
      <ResourceMeters />

      {/* Health KPI strip. Each tile is labelled by what it MEANS — the
          Kubernetes term is the sub-line, not the headline. */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
        <HealthStat
          label="Machines offline"
          hint="Nodes not Ready"
          value={health?.not_ready_nodes ?? 0}
          alert={(health?.not_ready_nodes ?? 0) > 0}
          loading={isLoading}
        />
        <HealthStat
          label="Apps crash looping"
          hint="Pods in CrashLoopBackOff"
          value={health?.crashloop_pods ?? 0}
          alert={(health?.crashloop_pods ?? 0) > 0}
          loading={isLoading}
        />
        <HealthStat
          label="Apps stuck starting"
          hint="Pods in Pending"
          value={health?.pending_pods ?? 0}
          alert={(health?.pending_pods ?? 0) > 0}
          loading={isLoading}
        />
        <HealthStat
          label="Disks that won't attach"
          hint="Failed mount events"
          value={health?.failed_mount_events ?? 0}
          alert={(health?.failed_mount_events ?? 0) > 0}
          loading={isLoading}
        />
        <HealthStat
          label="Recent warnings"
          hint="Warning events"
          value={health?.warning_events ?? 0}
          alert={(health?.warning_events ?? 0) > 0}
          loading={isLoading}
        />
      </div>

      {/* Insights */}
      {data?.insights && data.insights.length > 0 && (
        <section>
          <h3 className="text-sm font-bold font-display text-pilot-text-primary mb-3 flex items-center gap-2">
            <Activity className="w-4 h-4 text-pilot-accent" /> Insights
          </h3>
          <div className="space-y-2">
            {data.insights.map((insight) => (
              <div key={insight.id} className="bg-pilot-surface border border-pilot-border rounded-xl p-4 shadow-card">
                <div className="flex items-start justify-between gap-3 mb-1.5">
                  <div className="flex items-center gap-2 min-w-0">
                    <SeverityBadge severity={insight.severity} />
                    <span className="text-sm font-semibold font-display text-pilot-text-primary">{insight.title}</span>
                  </div>
                  <InsightActions insight={insight} />
                </div>
                <p className="text-sm text-pilot-text-secondary leading-relaxed">{insight.summary}</p>
                {insight.suggestions && insight.suggestions.length > 0 && (
                  <ul className="mt-2 space-y-1">
                    {insight.suggestions.map((s, i) => (
                      <li key={i} className="text-xs text-pilot-muted flex gap-2">
                        <span className="text-pilot-accent">→</span> {s}
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            ))}
          </div>
        </section>
      )}

      {/* Problem pods */}
      <section>
        <h3 className="text-sm font-bold font-display text-pilot-text-primary mb-3 flex items-center gap-2">
          <AlertTriangle className="h-4 w-4 text-pilot-danger" /> Apps that need attention
        </h3>
        <ProblemPodsTable pods={data?.problem_pods ?? []} loading={isLoading} />
      </section>

      {/* Node health */}
      <section>
        <h3 className="text-sm font-bold font-display text-pilot-text-primary mb-3 flex items-center gap-2">
          <Activity className="h-4 w-4 text-pilot-accent" /> Machines
        </h3>
        <NodeHealthTable nodes={data?.nodes ?? []} loading={isLoading} />
      </section>
    </div>
  );
}

function HealthStat({
  label,
  hint,
  value,
  alert,
  loading,
}: {
  label: string;
  hint: string;
  value: number;
  alert?: boolean;
  loading?: boolean;
}) {
  return (
    <div
      className={`rounded-xl border bg-pilot-surface p-4 shadow-card ${
        alert && !loading ? "border-pilot-danger/40" : "border-pilot-border"
      }`}
    >
      {/* A "0" while the scan is still running reads as "all clear" on a
          cluster that may have nine pods crash looping. Show that the number
          is not known yet instead of asserting a reassuring one. */}
      {loading ? (
        <div className="h-8 w-10 animate-pulse rounded bg-pilot-surface-2" aria-label="Loading" />
      ) : (
        <div
          className={`font-display text-2xl font-bold tabular-nums ${
            alert ? "text-pilot-danger" : "text-pilot-text-primary"
          }`}
        >
          {value}
        </div>
      )}
      <div className="mt-1 text-sm font-medium leading-snug text-pilot-text-secondary">{label}</div>
      <div className="mt-0.5 text-xs text-pilot-muted" title="The Kubernetes term for this">
        {hint}
      </div>
    </div>
  );
}

function ProblemPodsTable({ pods, loading }: { pods: ProblemPod[]; loading: boolean }) {
  const columns: Column<ProblemPod>[] = [
    {
      header: "Pod",
      cell: (p) => <span className="font-mono font-semibold text-pilot-text-primary">{p.name}</span>,
      sortValue: (p) => p.name,
    },
    {
      header: "Namespace",
      cell: (p) => <span className="text-pilot-text-secondary">{p.namespace}</span>,
      sortValue: (p) => p.namespace,
    },
    {
      header: "What's wrong",
      cell: (p) => (
        <StatusPill
          explanation={explainPod({ Phase: p.status, Reason: p.reason, Restarts: p.restarts })}
          raw={p.reason || p.status}
        />
      ),
      sortValue: (p) => p.reason || p.status,
    },
    {
      header: "Restarts",
      align: "right",
      cell: (p) => <span className="tabular-nums text-pilot-warning">{p.restarts}</span>,
      sortValue: (p) => p.restarts,
    },
  ];
  return (
    <ResourceTable
      columns={columns}
      items={pods}
      rowKey={(p) => `${p.namespace}/${p.name}`}
      loading={loading}
      noun="problem pod"
      searchText={(p) => `${p.namespace}/${p.name} ${p.status} ${p.reason}`}
      emptyMessage="Nothing is failing right now."
    />
  );
}

function NodeHealthTable({ nodes, loading }: { nodes: NodeHealthRow[]; loading: boolean }) {
  const columns: Column<NodeHealthRow>[] = [
    {
      header: "Node",
      // Hardware (vendor + model) only shows when the node carries the
      // kubepilot.io/hardware annotation; OS details ride along in the tooltip.
      cell: (n) => (
        <div title={[n.os_image, n.kernel_version, n.architecture, n.serial && `S/N ${n.serial}`].filter(Boolean).join(" · ")}>
          <span className="text-pilot-text-primary font-mono">{n.name}</span>
          {n.hardware && <div className="text-xs text-pilot-muted mt-0.5">{n.hardware}</div>}
        </div>
      ),
    },
    {
      header: "Role",
      cell: (n) => <NodeRoleBadge node={n} />,
    },
    {
      header: "IP Address",
      cell: (n) => <NodeIPDisplay node={n} />,
    },
    {
      header: "Status",
      cell: (n) => (
        <StatusPill
          explanation={explainNode({
            Ready: n.ready,
            MemoryPressure: n.memory_pressure,
            DiskPressure: n.disk_pressure,
            PIDPressure: n.pid_pressure,
          })}
          raw={n.ready ? "Ready" : "NotReady"}
        />
      ),
      // Broken machines first — the only ordering anyone wants here.
      sortValue: (n) => (n.ready ? 1 : 0),
    },
    {
      header: "CPU",
      align: "right",
      cell: (n) => (
        <span className="tabular-nums text-pilot-text-secondary">
          {n.cpu_usage_percent ? `${n.cpu_usage_percent}%` : "\u2014"}
        </span>
      ),
      sortValue: (n) => n.cpu_usage_percent ?? -1,
    },
    {
      header: "Memory",
      align: "right",
      cell: (n) => (
        <span className="tabular-nums text-pilot-text-secondary">
          {n.memory_usage_percent ? `${n.memory_usage_percent}%` : "\u2014"}
        </span>
      ),
      sortValue: (n) => n.memory_usage_percent ?? -1,
    },
    {
      header: "Pressure",
      cell: (n) => {
        const pressures = [
          n.memory_pressure && "Mem",
          n.disk_pressure && "Disk",
          n.pid_pressure && "PID",
        ].filter(Boolean) as string[];
        return pressures.length ? (
          <Badge variant="warning">{pressures.join(", ")}</Badge>
        ) : (
          <span className="text-pilot-muted text-sm">None</span>
        );
      },
    },
    {
      header: "Version",
      cell: (n) => <span className="font-mono text-xs text-pilot-muted">{n.kubelet_version}</span>,
      sortValue: (n) => n.kubelet_version,
      hideBelow: "lg",
    },
  ];
  return (
    <ResourceTable
      columns={columns}
      items={nodes}
      rowKey={(n) => n.name}
      loading={loading}
      noun="machine"
      searchText={(n) => `${n.name} ${n.hardware ?? ""} ${n.os_image ?? ""}`}
      searchPlaceholder="Find a machine…"
      emptyMessage="No machine data available."
    />
  );
}

function SeverityBadge({ severity }: { severity: string }) {
  const variant =
    severity === "critical" || severity === "high"
      ? "danger"
      : severity === "medium"
      ? "warning"
      : "muted";
  return <Badge variant={variant}>{severity}</Badge>;
}
