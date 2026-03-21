import { useDeferredValue, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  AlertTriangle,
  CheckCircle2,
  Clock3,
  Cpu,
  HardDrive,
  RefreshCw,
  Search,
  Server,
  ShieldAlert,
  X,
  FileWarning,
  Activity,
  Sparkles,
} from "lucide-react";
import {
  getClusterTroubleshootingSummary,
  getPodDiagnostics,
  listClusterEvents,
  troubleshootPod,
  type ClusterTroubleshootingSummary,
  type KubeEvent,
} from "@/lib/api";
import { LogViewer } from "@/components/LogViewer";

const IMPORTANT_REASONS = new Set([
  "FailedScheduling",
  "CrashLoopBackOff",
  "ImagePullBackOff",
  "FailedMount",
  "OOMKilled",
  "BackOff",
  "NodeNotReady",
  "Evicted",
  "VolumeAttachTimeout",
  "ContainerCannotRun",
  "ErrImagePull",
]);

const PAGE_SIZE = 25;

export function ClusterEventsTroubleshooting() {
  const [namespaceInput, setNamespaceInput] = useState("all");
  const [namespace, setNamespace] = useState("");
  const [kind, setKind] = useState("");
  const [severity, setSeverity] = useState("");
  const [search, setSearch] = useState("");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");
  const [page, setPage] = useState(1);
  const [selectedPod, setSelectedPod] = useState<{
    namespace: string;
    name: string;
    initialView?: "inspect" | "ai";
  } | null>(null);
  const deferredSearch = useDeferredValue(search);

  const { data: summary, isLoading: summaryLoading, refetch: refetchSummary } = useQuery<ClusterTroubleshootingSummary>({
    queryKey: ["cluster-troubleshooting-summary", namespace],
    queryFn: () => getClusterTroubleshootingSummary(namespace),
    refetchInterval: 10_000,
  });

  const { data: eventsResp, isLoading: eventsLoading, refetch: refetchEvents } = useQuery({
    queryKey: ["cluster-events", namespace, kind, severity, deferredSearch, sortDir],
    queryFn: () =>
      listClusterEvents({
        namespace,
        kind,
        type: severity,
        search: deferredSearch,
        sort: sortDir,
        limit: 500,
        since: "24h",
      }),
    refetchInterval: 10_000,
  });

  const events = Array.isArray(eventsResp?.items) ? eventsResp.items : [];
  const pagedEvents = useMemo(() => {
    const start = (page - 1) * PAGE_SIZE;
    return events.slice(start, start + PAGE_SIZE);
  }, [events, page]);
  const totalPages = Math.max(1, Math.ceil(events.length / PAGE_SIZE));

  const submitNamespace = (value: string) => {
    const normalized = value.trim().toLowerCase();
    const next = normalized === "" || normalized === "all" || normalized === "*" ? "" : value.trim();
    setNamespace(next);
    setNamespaceInput(next === "" ? "all" : next);
    setPage(1);
  };

  const handleRefresh = () => {
    void refetchSummary();
    void refetchEvents();
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <section className="bg-pilot-surface border border-pilot-border rounded-xl p-5 shadow-card">
        <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4">
          <div>
            <h2 className="text-base font-bold text-white flex items-center gap-2">
              <ShieldAlert className="w-5 h-5 text-pilot-warning" />
              Cluster Events & Troubleshooting
            </h2>
            <p className="text-sm text-pilot-muted mt-1">
              Live event stream, node health diagnostics, resource pressure, and problematic pod analysis.
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <button
              onClick={() => submitNamespace("all")}
              className={`px-3.5 py-2 text-sm rounded-lg border font-medium ${
                namespace === ""
                  ? "bg-pilot-accent text-white border-pilot-accent"
                  : "bg-pilot-bg text-pilot-muted border-pilot-border hover:text-white hover:border-pilot-border-hover"
              }`}
            >
              All Namespaces
            </button>
            <div className="flex items-center gap-2">
              <input
                value={namespaceInput}
                onChange={(e) => setNamespaceInput(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") submitNamespace(namespaceInput);
                }}
                placeholder="Namespace or all"
                className="bg-pilot-bg border border-pilot-border rounded-lg px-3.5 py-2 text-sm text-white placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent focus:ring-1 focus:ring-pilot-accent/30 w-44"
              />
              <button
                onClick={() => submitNamespace(namespaceInput)}
                className="px-3.5 py-2 text-sm rounded-lg bg-pilot-accent text-white hover:bg-blue-500 font-medium"
              >
                Apply
              </button>
            </div>
            <button
              onClick={handleRefresh}
              className="p-2 rounded-lg border border-pilot-border text-pilot-muted hover:text-white hover:bg-pilot-surface-2"
              title="Refresh diagnostics"
            >
              <RefreshCw className={`w-4 h-4 ${(summaryLoading || eventsLoading) ? "animate-spin" : ""}`} />
            </button>
          </div>
        </div>
      </section>

      <HealthSummaryCards summary={summary} loading={summaryLoading} />

      {/* Cluster health + insights */}
      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
        <section className="xl:col-span-2 bg-pilot-surface border border-pilot-border rounded-xl p-5 shadow-card">
          <div className="flex items-center justify-between mb-5">
            <h3 className="text-base font-bold text-white">Cluster Health Summary</h3>
            <span className="text-xs text-pilot-muted font-medium bg-pilot-surface-2 px-2.5 py-1 rounded-md">Auto-refresh 10s</span>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="bg-pilot-bg border border-pilot-border rounded-xl p-5">
              <p className="text-xs uppercase tracking-wider text-pilot-muted mb-3 font-semibold">Recommended Actions</p>
              <div className="space-y-3 text-sm text-white">
                {(summary?.health_summary.recommended_actions || []).length > 0 ? (
                  (summary?.health_summary.recommended_actions || []).map((action) => (
                    <div key={action} className="flex items-start gap-2.5">
                      <AlertTriangle className="w-4 h-4 text-pilot-warning mt-0.5 shrink-0" />
                      <span className="leading-relaxed">{action}</span>
                    </div>
                  ))
                ) : (
                  <div className="text-pilot-muted">No urgent actions inferred from the current cluster state.</div>
                )}
              </div>
            </div>
            <ResourcePressurePanel summary={summary} />
          </div>
        </section>

        <section className="bg-pilot-surface border border-pilot-border rounded-xl p-5 shadow-card">
          <h3 className="text-base font-bold text-white mb-4">Troubleshooting Insights</h3>
          <div className="space-y-3 max-h-[400px] overflow-y-auto pr-1">
            {(summary?.insights || []).length > 0 ? (
              (summary?.insights || []).map((insight) => (
                <InsightCard key={insight.id} insight={insight} />
              ))
            ) : (
              <div className="text-sm text-pilot-muted">No major cluster failure patterns detected in the current sample.</div>
            )}
          </div>
        </section>
      </div>

      {/* Node health table */}
      <section className="bg-pilot-surface border border-pilot-border rounded-xl p-5 shadow-card">
        <div className="flex items-center justify-between mb-5">
          <h3 className="text-base font-bold text-white flex items-center gap-2">
            <Server className="w-5 h-5 text-pilot-accent" />
            Node Health
          </h3>
          {summary?.resource_pressure.metrics_available ? (
            <span className="text-xs text-pilot-success font-medium bg-emerald-500/10 px-2.5 py-1 rounded-md">Metrics server detected</span>
          ) : (
            <span className="text-xs text-pilot-muted bg-pilot-surface-2 px-2.5 py-1 rounded-md">Metrics unavailable</span>
          )}
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-pilot-surface-2 text-pilot-muted text-xs uppercase tracking-wider">
                <th className="text-left px-5 py-3.5 font-semibold">Node</th>
                <th className="text-left px-5 py-3.5 font-semibold">Status</th>
                <th className="text-left px-5 py-3.5 font-semibold">CPU</th>
                <th className="text-left px-5 py-3.5 font-semibold">Memory</th>
                <th className="text-left px-5 py-3.5 font-semibold">Pressure</th>
                <th className="text-left px-5 py-3.5 font-semibold">Kubelet</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-pilot-border">
              {(summary?.nodes || []).map((node) => (
                <tr key={node.name} className="hover:bg-pilot-surface-2/50">
                  <td className="px-5 py-3.5 font-mono text-sm text-white">{node.name}</td>
                  <td className="px-5 py-3.5">
                    {node.ready ? (
                      <span className="inline-flex items-center gap-1.5 text-pilot-success text-sm font-medium"><CheckCircle2 className="w-4 h-4" /> Ready</span>
                    ) : (
                      <span className="inline-flex items-center gap-1.5 text-pilot-danger text-sm font-medium"><AlertTriangle className="w-4 h-4" /> NotReady</span>
                    )}
                  </td>
                  <td className="px-5 py-3.5 text-sm text-pilot-text-secondary">
                    {node.cpu_usage ? `${node.cpu_usage} / ${node.cpu_capacity}` : node.cpu_capacity}
                    {node.cpu_usage_percent ? <span className="ml-2 text-pilot-accent font-medium">({node.cpu_usage_percent}%)</span> : null}
                  </td>
                  <td className="px-5 py-3.5 text-sm text-pilot-text-secondary">
                    {node.memory_usage ? `${node.memory_usage} / ${node.memory_capacity}` : node.memory_capacity}
                    {node.memory_usage_percent ? <span className="ml-2 text-pilot-accent font-medium">({node.memory_usage_percent}%)</span> : null}
                  </td>
                  <td className="px-5 py-3.5"><PressureBadges node={node} /></td>
                  <td className="px-5 py-3.5 text-sm text-pilot-text-secondary font-mono">{node.kubelet_version}</td>
                </tr>
              ))}
              {(summary?.nodes || []).length === 0 && (
                <tr>
                  <td colSpan={6} className="px-5 py-10 text-center text-sm text-pilot-muted">No node diagnostics available.</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>

      {/* Problematic pods */}
      <section className="bg-pilot-surface border border-pilot-border rounded-xl p-5 shadow-card">
        <div className="flex items-center justify-between mb-5">
          <h3 className="text-base font-bold text-white flex items-center gap-2">
            <FileWarning className="w-5 h-5 text-pilot-danger" />
            Problematic Pods
          </h3>
          <span className="text-xs text-pilot-muted bg-pilot-surface-2 px-2.5 py-1 rounded-md">Crash loops, evictions, pull failures</span>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-pilot-surface-2 text-pilot-muted text-xs uppercase tracking-wider">
                <th className="text-left px-5 py-3.5 font-semibold">Pod</th>
                <th className="text-left px-5 py-3.5 font-semibold">Namespace</th>
                <th className="text-left px-5 py-3.5 font-semibold">Status</th>
                <th className="text-left px-5 py-3.5 font-semibold">Restarts</th>
                <th className="text-left px-5 py-3.5 font-semibold">Node</th>
                <th className="text-left px-5 py-3.5 font-semibold">Reason</th>
                <th className="text-left px-5 py-3.5 font-semibold">Age</th>
                <th className="text-left px-5 py-3.5 font-semibold">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-pilot-border">
              {(summary?.problem_pods || []).map((pod) => (
                <tr key={`${pod.namespace}/${pod.name}`} className="hover:bg-pilot-surface-2/50">
                  <td className="px-5 py-3.5 font-mono text-sm text-white">{pod.name}</td>
                  <td className="px-5 py-3.5 text-sm text-pilot-text-secondary">{pod.namespace}</td>
                  <td className="px-5 py-3.5 text-sm text-pilot-text-secondary">{pod.status}</td>
                  <td className="px-5 py-3.5 text-sm">
                    <span className={pod.restarts > 5 ? "text-pilot-warning font-bold" : "text-pilot-text-secondary"}>{pod.restarts}</span>
                  </td>
                  <td className="px-5 py-3.5 text-sm text-pilot-text-secondary">{pod.node || "\u2014"}</td>
                  <td className="px-5 py-3.5"><ReasonBadge reason={pod.reason} eventType="Warning" /></td>
                  <td className="px-5 py-3.5 text-sm text-pilot-text-secondary">{formatAgeMinutes(pod.age_minutes)}</td>
                  <td className="px-5 py-3.5">
                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => setSelectedPod({ namespace: pod.namespace, name: pod.name, initialView: "inspect" })}
                        className="text-sm bg-pilot-accent/10 text-pilot-accent-light px-3 py-1.5 rounded-lg hover:bg-pilot-accent/20 font-medium"
                      >
                        Inspect
                      </button>
                      <button
                        onClick={() => setSelectedPod({ namespace: pod.namespace, name: pod.name, initialView: "ai" })}
                        className="text-sm bg-amber-500/10 text-pilot-warning px-3 py-1.5 rounded-lg hover:bg-amber-500/20 font-medium flex items-center gap-1.5"
                      >
                        <Sparkles className="w-3.5 h-3.5" />
                        AI Analyze
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
              {(summary?.problem_pods || []).length === 0 && (
                <tr>
                  <td colSpan={8} className="px-5 py-10 text-center text-sm text-pilot-muted">No problematic pods detected.</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>

      {/* Events table */}
      <section className="bg-pilot-surface border border-pilot-border rounded-xl p-5 shadow-card">
        <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4 mb-5">
          <div>
            <h3 className="text-base font-bold text-white">Kubernetes Events</h3>
            <p className="text-sm text-pilot-muted mt-1">Recent events with server-side filtering and 10s refresh.</p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <div className="flex items-center gap-2 bg-pilot-bg border border-pilot-border rounded-lg px-3.5 py-2 focus-within:border-pilot-accent focus-within:ring-1 focus-within:ring-pilot-accent/30">
              <Search className="w-4 h-4 text-pilot-muted" />
              <input
                value={search}
                onChange={(e) => {
                  setSearch(e.target.value);
                  setPage(1);
                }}
                placeholder="Search reason, message, resource"
                className="bg-transparent text-sm text-white placeholder:text-pilot-muted focus:outline-none w-56"
              />
            </div>
            <select value={kind} onChange={(e) => { setKind(e.target.value); setPage(1); }} className="bg-pilot-bg border border-pilot-border rounded-lg px-3.5 py-2 text-sm text-white">
              <option value="">All Resources</option>
              <option value="Pod">Pod</option>
              <option value="Node">Node</option>
              <option value="Deployment">Deployment</option>
              <option value="PersistentVolumeClaim">PVC</option>
            </select>
            <select value={severity} onChange={(e) => { setSeverity(e.target.value); setPage(1); }} className="bg-pilot-bg border border-pilot-border rounded-lg px-3.5 py-2 text-sm text-white">
              <option value="">All Severities</option>
              <option value="Warning">Warning</option>
              <option value="Normal">Normal</option>
            </select>
            <button
              onClick={() => setSortDir((current) => current === "desc" ? "asc" : "desc")}
              className="px-3.5 py-2 text-sm rounded-lg border border-pilot-border bg-pilot-bg text-white hover:bg-pilot-surface-2 font-medium"
            >
              {sortDir === "desc" ? "Newest first" : "Oldest first"}
            </button>
          </div>
        </div>

        {eventsLoading ? (
          <div className="space-y-2">
            {Array.from({ length: 6 }).map((_, index) => (
              <div key={index} className="h-12 rounded-lg bg-pilot-bg animate-pulse" />
            ))}
          </div>
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="bg-pilot-surface-2 text-pilot-muted text-xs uppercase tracking-wider">
                    <th className="text-left px-5 py-3.5 font-semibold">Time</th>
                    <th className="text-left px-5 py-3.5 font-semibold">Namespace</th>
                    <th className="text-left px-5 py-3.5 font-semibold">Object</th>
                    <th className="text-left px-5 py-3.5 font-semibold">Type</th>
                    <th className="text-left px-5 py-3.5 font-semibold">Reason</th>
                    <th className="text-left px-5 py-3.5 font-semibold">Message</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-pilot-border">
                  {pagedEvents.map((event, index) => (
                    <tr key={`${event.involved_object.namespace}-${event.involved_object.name}-${event.reason}-${index}`} className="hover:bg-pilot-surface-2/50">
                      <td className="px-5 py-3.5 text-sm text-pilot-muted whitespace-nowrap">{formatTimestamp(event.last_seen)}</td>
                      <td className="px-5 py-3.5 text-sm text-pilot-text-secondary">{event.involved_object.namespace || "cluster"}</td>
                      <td className="px-5 py-3.5 text-sm text-white">
                        <span className="font-mono">{event.involved_object.kind}/{event.involved_object.name}</span>
                      </td>
                      <td className="px-5 py-3.5 text-sm">
                        <span className={event.type === "Warning" ? "text-pilot-warning font-semibold" : "text-pilot-success"}>{event.type}</span>
                      </td>
                      <td className="px-5 py-3.5"><ReasonBadge reason={event.reason} eventType={event.type} /></td>
                      <td className="px-5 py-3.5 text-sm text-pilot-muted max-w-[520px] truncate" title={event.message}>{event.message}</td>
                    </tr>
                  ))}
                  {pagedEvents.length === 0 && (
                    <tr>
                      <td colSpan={6} className="px-5 py-10 text-center text-sm text-pilot-muted">No events match the selected filters.</td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>

            {/* Pagination */}
            <div className="flex items-center justify-between mt-5 text-sm text-pilot-muted">
              <span>Showing <span className="font-medium text-white">{pagedEvents.length}</span> of <span className="font-medium text-white">{events.length}</span> events</span>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => setPage((current) => Math.max(1, current - 1))}
                  disabled={page === 1}
                  className="px-3 py-1.5 rounded-lg border border-pilot-border disabled:opacity-30 hover:bg-pilot-surface-2 font-medium"
                >
                  Prev
                </button>
                <span className="px-2 font-medium">Page {page} / {totalPages}</span>
                <button
                  onClick={() => setPage((current) => Math.min(totalPages, current + 1))}
                  disabled={page === totalPages}
                  className="px-3 py-1.5 rounded-lg border border-pilot-border disabled:opacity-30 hover:bg-pilot-surface-2 font-medium"
                >
                  Next
                </button>
              </div>
            </div>
          </>
        )}
      </section>

      {selectedPod && (
        <PodDiagnosticsModal
          namespace={selectedPod.namespace}
          pod={selectedPod.name}
          initialView={selectedPod.initialView}
          onClose={() => setSelectedPod(null)}
        />
      )}
    </div>
  );
}

function HealthSummaryCards({
  summary,
  loading,
}: {
  summary: ClusterTroubleshootingSummary | undefined;
  loading: boolean;
}) {
  const health = summary?.health_summary;
  const cards = [
    { label: "Nodes NotReady", value: health?.not_ready_nodes ?? 0, icon: <Server className="w-5 h-5" />, alert: (health?.not_ready_nodes ?? 0) > 0 },
    { label: "Pod Failures", value: health?.crashloop_pods ?? 0, icon: <AlertTriangle className="w-5 h-5" />, alert: (health?.crashloop_pods ?? 0) > 0 },
    { label: "Failed Mounts", value: health?.failed_mount_events ?? 0, icon: <HardDrive className="w-5 h-5" />, alert: (health?.failed_mount_events ?? 0) > 0 },
    { label: "Warning Events", value: health?.warning_events ?? 0, icon: <Activity className="w-5 h-5" />, alert: (health?.warning_events ?? 0) > 0 },
  ];

  if (loading) {
    return (
      <div className="grid grid-cols-2 xl:grid-cols-4 gap-4">
        {cards.map((card) => (
          <div key={card.label} className="h-28 rounded-xl bg-pilot-surface border border-pilot-border animate-pulse" />
        ))}
      </div>
    );
  }

  return (
    <div className="grid grid-cols-2 xl:grid-cols-4 gap-4">
      {cards.map((card) => (
        <div key={card.label} className={`bg-pilot-surface border rounded-xl p-5 shadow-card ${card.alert ? "border-pilot-danger/40" : "border-pilot-border"}`}>
          <div className={`flex items-center gap-2 mb-2 ${card.alert ? "text-pilot-danger" : "text-pilot-accent"}`}>
            {card.icon}
            <span className="text-xs uppercase tracking-wider text-pilot-muted font-semibold">{card.label}</span>
          </div>
          <div className={`text-3xl font-bold tracking-tight ${card.alert ? "text-pilot-danger" : "text-white"}`}>{card.value}</div>
        </div>
      ))}
    </div>
  );
}

function ResourcePressurePanel({ summary }: { summary: ClusterTroubleshootingSummary | undefined }) {
  const resource = summary?.resource_pressure;
  return (
    <div className="bg-pilot-bg border border-pilot-border rounded-xl p-5">
      <p className="text-xs uppercase tracking-wider text-pilot-muted mb-4 font-semibold">Resource Pressure</p>
      <div className="space-y-4 text-sm">
        <div className="flex items-center justify-between text-white">
          <span className="flex items-center gap-2"><Cpu className="w-4 h-4 text-pilot-accent" /> Cluster CPU</span>
          <span className="font-semibold">{resource?.metrics_available ? `${resource.cpu_usage_percent ?? 0}%` : "n/a"}</span>
        </div>
        <div className="flex items-center justify-between text-white">
          <span className="flex items-center gap-2"><HardDrive className="w-4 h-4 text-pilot-accent" /> Cluster Memory</span>
          <span className="font-semibold">{resource?.metrics_available ? `${resource.memory_usage_percent ?? 0}%` : "n/a"}</span>
        </div>
        <div className="text-sm text-pilot-muted pt-3 border-t border-pilot-border space-y-1.5">
          <div>{resource?.memory_pressure_nodes ?? 0} node(s) under MemoryPressure</div>
          <div>{resource?.disk_pressure_nodes ?? 0} node(s) under DiskPressure</div>
          <div>{resource?.pid_pressure_nodes ?? 0} node(s) under PIDPressure</div>
        </div>
      </div>
    </div>
  );
}

function InsightCard({ insight }: { insight: ClusterTroubleshootingSummary["insights"][number] }) {
  const severityClass =
    insight.severity === "high" || insight.severity === "critical"
      ? "border-pilot-danger/60"
      : insight.severity === "medium"
      ? "border-pilot-warning/60"
      : "border-pilot-border";

  return (
    <div className={`bg-pilot-bg border ${severityClass} rounded-xl p-4`}>
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-sm font-bold text-white">{insight.title}</p>
          <p className="text-sm text-pilot-muted mt-1.5 leading-relaxed">{insight.summary}</p>
        </div>
        <span className="text-xs uppercase rounded-md px-2.5 py-1 bg-pilot-surface text-pilot-muted font-medium shrink-0">{insight.category}</span>
      </div>
      {(insight.suggestions || []).length > 0 && (
        <div className="mt-3 space-y-2 text-sm text-white">
          {insight.suggestions.map((suggestion) => (
            <div key={suggestion} className="flex items-start gap-2">
              <Clock3 className="w-4 h-4 text-pilot-accent mt-0.5 shrink-0" />
              <span className="leading-relaxed">{suggestion}</span>
            </div>
          ))}
        </div>
      )}
      {(insight.affected_resources || []).length > 0 && (
        <div className="mt-3 text-xs text-pilot-muted font-mono break-all">
          {(insight.affected_resources || []).slice(0, 4).join(", ")}
        </div>
      )}
    </div>
  );
}

function PressureBadges({ node }: { node: ClusterTroubleshootingSummary["nodes"][number] }) {
  const active = [
    node.memory_pressure ? "Memory" : null,
    node.disk_pressure ? "Disk" : null,
    node.pid_pressure ? "PID" : null,
    node.unschedulable ? "Unschedulable" : null,
  ].filter(Boolean) as string[];

  if (active.length === 0) {
    return <span className="text-sm text-pilot-muted">None</span>;
  }

  return (
    <div className="flex gap-1.5 flex-wrap">
      {active.map((label) => (
        <span key={label} className="text-xs bg-pilot-warning/20 text-pilot-warning px-2 py-0.5 rounded-md font-semibold border border-pilot-warning/30">
          {label}
        </span>
      ))}
    </div>
  );
}

function ReasonBadge({ reason, eventType }: { reason: string; eventType: string }) {
  const isCritical = IMPORTANT_REASONS.has(reason);
  const cls = isCritical
    ? "bg-red-900/40 text-red-300 border-red-700/50"
    : eventType === "Warning"
    ? "bg-yellow-900/30 text-yellow-300 border-yellow-700/50"
    : "bg-pilot-bg text-pilot-muted border-pilot-border";

  return <span className={`text-xs px-2.5 py-1 rounded-md border font-medium ${cls}`}>{reason || "\u2014"}</span>;
}

function PodDiagnosticsModal({
  namespace,
  pod,
  initialView = "inspect",
  onClose,
}: {
  namespace: string;
  pod: string;
  initialView?: "inspect" | "ai";
  onClose: () => void;
}) {
  const [activeView, setActiveView] = useState<"inspect" | "ai">(initialView);
  const { data, isLoading } = useQuery({
    queryKey: ["pod-diagnostics", namespace, pod],
    queryFn: () => getPodDiagnostics(namespace, pod),
  });

  const { data: aiReport, isLoading: aiLoading, refetch: refetchAI, isFetching: aiFetching } = useQuery({
    queryKey: ["pod-ai-diagnose", namespace, pod],
    queryFn: () => troubleshootPod(namespace, pod),
    enabled: activeView === "ai",
  });

  const diag = data?.diagnostics;

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex justify-end">
      <div className="w-full max-w-3xl bg-pilot-bg border-l border-pilot-border h-full overflow-y-auto p-6 animate-slide-in-right">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h3 className="font-bold text-white text-lg">Pod Troubleshooting</h3>
            <p className="text-sm text-pilot-muted mt-0.5 font-mono">{namespace}/{pod}</p>
          </div>
          <button onClick={onClose} className="text-pilot-muted hover:text-white p-2 rounded-lg hover:bg-pilot-surface">
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="flex items-center gap-2 mb-5">
          <button
            onClick={() => setActiveView("inspect")}
            className={`px-4 py-2 text-sm rounded-lg border font-medium ${
              activeView === "inspect"
                ? "bg-pilot-accent border-pilot-accent text-white"
                : "bg-pilot-surface border-pilot-border text-pilot-muted hover:text-white"
            }`}
          >
            Inspect
          </button>
          <button
            onClick={() => setActiveView("ai")}
            className={`px-4 py-2 text-sm rounded-lg border font-medium flex items-center gap-1.5 ${
              activeView === "ai"
                ? "bg-pilot-warning border-pilot-warning text-black"
                : "bg-pilot-surface border-pilot-border text-pilot-muted hover:text-white"
            }`}
          >
            <Sparkles className="w-4 h-4" />
            AI Analysis
          </button>
          {activeView === "ai" && (
            <button
              onClick={() => void refetchAI()}
              className="ml-auto px-4 py-2 text-sm rounded-lg bg-pilot-surface border border-pilot-border text-white hover:bg-pilot-surface-2 flex items-center gap-1.5 font-medium"
            >
              <RefreshCw className={`w-4 h-4 ${aiFetching ? "animate-spin" : ""}`} />
              Re-run AI
            </button>
          )}
        </div>

        {isLoading ? (
          <div className="flex items-center gap-3 text-pilot-muted text-sm py-8">
            <RefreshCw className="w-5 h-5 animate-spin" /> Loading pod diagnostics...
          </div>
        ) : diag ? (
          <div className="space-y-5 animate-fade-in">
            {activeView === "inspect" ? (
              <>
                <div className="bg-pilot-surface border border-pilot-border rounded-xl p-5 space-y-3 text-sm">
                  <Row label="Phase" value={diag.phase} />
                  <Row label="Node" value={diag.node_name || "\u2014"} />
                  <Row label="Service Account" value={diag.service_account || "default"} />
                  <Row label="Created" value={formatTimestamp(diag.created_at)} />
                  <Row label="Volumes" value={(diag.volumes || []).join(", ") || "\u2014"} />
                </div>

                <section className="bg-pilot-surface border border-pilot-border rounded-xl p-5">
                  <h4 className="text-xs font-semibold text-white uppercase tracking-wider mb-4">Container Status</h4>
                  <div className="space-y-3">
                    {(diag.container_statuses || []).map((container) => (
                      <div key={container.name} className="bg-pilot-bg border border-pilot-border rounded-xl p-4 text-sm">
                        <div className="flex items-center justify-between gap-3">
                          <span className="font-mono text-white font-medium">{container.name}</span>
                          <span className={container.ready ? "text-pilot-success font-medium" : "text-pilot-warning font-medium"}>{container.state || "Unknown"}</span>
                        </div>
                        <div className="mt-2 text-pilot-muted break-all text-sm">{container.image}</div>
                        <div className="mt-1.5 text-pilot-muted">Reason: {container.state_reason || container.last_terminated_reason || "\u2014"} &bull; Restarts: {container.restart_count}</div>
                        {container.state_message && <div className="mt-1.5 text-pilot-warning">{container.state_message}</div>}
                      </div>
                    ))}
                  </div>
                </section>

                <section className="bg-pilot-surface border border-pilot-border rounded-xl p-5">
                  <h4 className="text-xs font-semibold text-white uppercase tracking-wider mb-4">Related Events</h4>
                  <div className="space-y-2 max-h-80 overflow-y-auto">
                    {(diag.events || []).length > 0 ? (
                      (diag.events || []).map((event, index) => (
                        <div key={`${event.reason}-${index}`} className="bg-pilot-bg border border-pilot-border rounded-xl p-4 text-sm">
                          <div className="flex items-center justify-between gap-2">
                            <ReasonBadge reason={event.reason} eventType={event.type} />
                            <span className="text-pilot-muted text-sm">{formatTimestamp(event.last_seen)}</span>
                          </div>
                          <div className="text-pilot-text-secondary mt-2 leading-relaxed">{event.message}</div>
                        </div>
                      ))
                    ) : (
                      <div className="text-sm text-pilot-muted">No related events found for this pod.</div>
                    )}
                  </div>
                </section>

                <LogViewer title="Recent Logs" content={data?.logs || ""} maxHeight="360px" />
              </>
            ) : (
              <section className="bg-pilot-surface border border-pilot-border rounded-xl p-5 space-y-5">
                <div>
                  <p className="text-xs uppercase tracking-wider text-pilot-muted mb-2 font-semibold">AI Troubleshooting</p>
                  <p className="text-sm text-pilot-muted">
                    Analyze pod status, recent events, and logs to infer likely root cause and next remediation steps.
                  </p>
                </div>

                {aiLoading || aiFetching ? (
                  <div className="flex items-center gap-3 text-pilot-muted text-sm py-6">
                    <RefreshCw className="w-5 h-5 animate-spin" /> Analyzing error signals with AI...
                  </div>
                ) : aiReport ? (
                  <div className="space-y-4 animate-fade-in">
                    <div className="bg-pilot-bg border border-pilot-border rounded-xl p-5">
                      <p className="text-xs uppercase tracking-wider text-pilot-muted mb-2 font-semibold">Root Cause</p>
                      <p className="text-base font-bold text-pilot-danger leading-relaxed">{aiReport.RootCause || "Unknown root cause"}</p>
                    </div>
                    <div className="bg-pilot-bg border border-pilot-border rounded-xl p-5">
                      <p className="text-xs uppercase tracking-wider text-pilot-muted mb-2 font-semibold">Analysis</p>
                      <p className="text-sm text-white leading-relaxed whitespace-pre-wrap">{aiReport.Analysis}</p>
                    </div>
                    {(aiReport.Actions || []).length > 0 && (
                      <div className="bg-pilot-bg border border-pilot-border rounded-xl p-5">
                        <p className="text-xs uppercase tracking-wider text-pilot-muted mb-3 font-semibold">Suggested Actions</p>
                        <div className="space-y-3">
                          {(aiReport.Actions || []).map((action, index) => (
                            <div key={`${action.type}-${index}`} className="border border-pilot-border rounded-xl p-4 bg-pilot-surface/40">
                              <div className="text-xs font-bold text-pilot-accent uppercase">{action.type}</div>
                              <div className="text-sm text-pilot-text-secondary mt-1.5 leading-relaxed">{action.explanation}</div>
                              {action.namespace || action.resource ? (
                                <div className="text-xs text-white mt-2 font-mono">
                                  {[action.namespace, action.resource].filter(Boolean).join("/")}
                                </div>
                              ) : null}
                            </div>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                ) : (
                  <div className="text-pilot-muted text-sm">No AI analysis available.</div>
                )}
              </section>
            )}
          </div>
        ) : (
          <div className="text-pilot-danger text-sm py-8">Pod diagnostics unavailable.</div>
        )}
      </div>
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex gap-3">
      <span className="text-pilot-muted w-32 shrink-0 font-medium">{label}</span>
      <span className="text-white break-all">{value}</span>
    </div>
  );
}

function formatTimestamp(value: string) {
  if (!value) return "\u2014";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
}

function formatAgeMinutes(minutes: number) {
  if (minutes < 60) return `${minutes}m`;
  if (minutes < 24 * 60) return `${Math.floor(minutes / 60)}h ${minutes % 60}m`;
  const days = Math.floor(minutes / (24 * 60));
  const hours = Math.floor((minutes % (24 * 60)) / 60);
  return `${days}d ${hours}h`;
}
