import { useQuery } from "@tanstack/react-query";
import { getClusterTroubleshootingSummary, type NodeHealthRow, type ProblemPod } from "@/lib/api";
import { ClusterResourceCharts } from "@/components/ClusterResourceCharts";
import { ResourceTable, type Column } from "./ResourceTable";
import { Badge } from "@/components/ui/badge";
import { AlertTriangle, CheckCircle, Activity } from "lucide-react";

export function ClusterHealthSection({ namespace }: { namespace: string }) {
  const { data, isLoading } = useQuery({
    queryKey: ["dash-health", namespace],
    queryFn: () => getClusterTroubleshootingSummary(namespace),
  });

  const health = data?.health_summary;

  return (
    <div className="space-y-6">
      <ClusterResourceCharts />

      {/* Health KPI strip */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
        <HealthStat label="Not Ready Nodes" value={health?.not_ready_nodes ?? 0} alert={(health?.not_ready_nodes ?? 0) > 0} />
        <HealthStat label="CrashLoop Pods" value={health?.crashloop_pods ?? 0} alert={(health?.crashloop_pods ?? 0) > 0} />
        <HealthStat label="Pending Pods" value={health?.pending_pods ?? 0} alert={(health?.pending_pods ?? 0) > 0} />
        <HealthStat label="Failed Mounts" value={health?.failed_mount_events ?? 0} alert={(health?.failed_mount_events ?? 0) > 0} />
        <HealthStat label="Warning Events" value={health?.warning_events ?? 0} alert={(health?.warning_events ?? 0) > 0} />
      </div>

      {/* Insights */}
      {data?.insights && data.insights.length > 0 && (
        <section>
          <h3 className="text-sm font-bold text-white mb-3 flex items-center gap-2">
            <Activity className="w-4 h-4 text-pilot-accent" /> Insights
          </h3>
          <div className="space-y-2">
            {data.insights.map((insight) => (
              <div key={insight.id} className="bg-pilot-surface border border-pilot-border rounded-xl p-4">
                <div className="flex items-center gap-2 mb-1.5">
                  <SeverityBadge severity={insight.severity} />
                  <span className="text-sm font-semibold text-white">{insight.title}</span>
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
        <h3 className="text-sm font-bold text-white mb-3 flex items-center gap-2">
          <AlertTriangle className="w-4 h-4 text-pilot-danger" /> Problem Pods
        </h3>
        <ProblemPodsTable pods={data?.problem_pods ?? []} loading={isLoading} />
      </section>

      {/* Node health */}
      <section>
        <h3 className="text-sm font-bold text-white mb-3 flex items-center gap-2">
          <Activity className="w-4 h-4 text-pilot-accent" /> Node Health
        </h3>
        <NodeHealthTable nodes={data?.nodes ?? []} loading={isLoading} />
      </section>
    </div>
  );
}

function HealthStat({ label, value, alert }: { label: string; value: number; alert?: boolean }) {
  return (
    <div className={`bg-pilot-surface border rounded-xl p-4 ${alert ? "border-pilot-danger/40" : "border-pilot-border"}`}>
      <div className="text-xs uppercase tracking-wider text-pilot-muted font-medium mb-1">{label}</div>
      <div className={`text-2xl font-bold ${alert ? "text-pilot-danger" : "text-white"}`}>{value}</div>
    </div>
  );
}

function ProblemPodsTable({ pods, loading }: { pods: ProblemPod[]; loading: boolean }) {
  const columns: Column<ProblemPod>[] = [
    { header: "Namespace", cell: (p) => <span className="text-pilot-text-secondary">{p.namespace}</span> },
    { header: "Pod", cell: (p) => <span className="text-white font-mono">{p.name}</span> },
    { header: "Status", cell: (p) => <span className="text-pilot-danger font-semibold">{p.status}</span> },
    { header: "Reason", cell: (p) => <span className="text-pilot-text-secondary">{p.reason || "—"}</span> },
    { header: "Restarts", align: "center", cell: (p) => <span className="text-pilot-warning">{p.restarts}</span> },
  ];
  return (
    <ResourceTable
      columns={columns}
      items={pods}
      rowKey={(p) => `${p.namespace}/${p.name}`}
      loading={loading}
      emptyMessage="No problem pods detected."
    />
  );
}

function NodeHealthTable({ nodes, loading }: { nodes: NodeHealthRow[]; loading: boolean }) {
  const columns: Column<NodeHealthRow>[] = [
    { header: "Node", cell: (n) => <span className="text-white font-mono">{n.name}</span> },
    {
      header: "Status",
      cell: (n) =>
        n.ready ? (
          <span className="inline-flex items-center gap-1.5 text-pilot-success text-sm">
            <CheckCircle className="w-3.5 h-3.5" /> Ready
          </span>
        ) : (
          <span className="inline-flex items-center gap-1.5 text-pilot-danger text-sm">
            <AlertTriangle className="w-3.5 h-3.5" /> NotReady
          </span>
        ),
    },
    { header: "CPU", align: "center", cell: (n) => <span className="text-pilot-text-secondary">{n.cpu_usage_percent ? `${n.cpu_usage_percent}%` : "—"}</span> },
    { header: "Memory", align: "center", cell: (n) => <span className="text-pilot-text-secondary">{n.memory_usage_percent ? `${n.memory_usage_percent}%` : "—"}</span> },
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
    { header: "Version", cell: (n) => <span className="text-pilot-muted font-mono text-xs">{n.kubelet_version}</span> },
  ];
  return (
    <ResourceTable
      columns={columns}
      items={nodes}
      rowKey={(n) => n.name}
      loading={loading}
      emptyMessage="No node data available."
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
