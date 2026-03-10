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
      <section className="bg-pilot-surface border border-pilot-border rounded-lg p-4">
        <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4">
          <div>
            <h2 className="text-sm font-bold text-white flex items-center gap-2">
              <ShieldAlert className="w-4 h-4 text-pilot-warning" />
              Cluster Events & Troubleshooting
            </h2>
            <p className="text-xs text-pilot-muted mt-1">
              Live event stream, node health diagnostics, resource pressure, and problematic pod analysis.
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <button
              onClick={() => submitNamespace("all")}
              className={`px-3 py-1.5 text-xs rounded border ${
                namespace === ""
                  ? "bg-pilot-accent text-white border-pilot-accent"
                  : "bg-pilot-bg text-pilot-muted border-pilot-border hover:text-white"
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
                className="bg-pilot-bg border border-pilot-border rounded px-3 py-1.5 text-xs text-white placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent w-40"
              />
              <button
                onClick={() => submitNamespace(namespaceInput)}
                className="px-3 py-1.5 text-xs rounded bg-pilot-accent text-white hover:bg-blue-500"
              >
                Apply
              </button>
            </div>
            <button
              onClick={handleRefresh}
              className="p-2 rounded border border-pilot-border text-pilot-muted hover:text-white hover:bg-pilot-bg"
              title="Refresh diagnostics"
            >
              <RefreshCw className={`w-4 h-4 ${(summaryLoading || eventsLoading) ? "animate-spin" : ""}`} />
            </button>
          </div>
        </div>
      </section>

      <HealthSummaryCards summary={summary} loading={summaryLoading} />

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
        <section className="xl:col-span-2 bg-pilot-surface border border-pilot-border rounded-lg p-4">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-sm font-bold text-white">Cluster Health Summary</h3>
            <span className="text-xs text-pilot-muted">Auto-refresh every 10s</span>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="bg-pilot-bg border border-pilot-border rounded p-4">
              <p className="text-xs uppercase tracking-wider text-pilot-muted mb-2">Recommended Actions</p>
              <div className="space-y-2 text-sm text-white">
                {(summary?.health_summary.recommended_actions || []).length > 0 ? (
                  (summary?.health_summary.recommended_actions || []).map((action) => (
                    <div key={action} className="flex items-start gap-2">
                      <AlertTriangle className="w-4 h-4 text-pilot-warning mt-0.5 shrink-0" />
                      <span>{action}</span>
                    </div>
                  ))
                ) : (
                  <div className="text-pilot-muted text-sm">No urgent actions inferred from the current cluster state.</div>
                )}
              </div>
            </div>
            <ResourcePressurePanel summary={summary} />
          </div>
        </section>

        <section className="bg-pilot-surface border border-pilot-border rounded-lg p-4">
          <h3 className="text-sm font-bold text-white mb-4">Troubleshooting Insights</h3>
          <div className="space-y-3 max-h-[360px] overflow-y-auto pr-1">
            {(summary?.insights || []).length > 0 ? (
              (summary?.insights || []).map((insight) => (
                <InsightCard key={insight.id} insight={insight} />
              ))
            ) : (
              <div className="text-xs text-pilot-muted">No major cluster failure patterns detected in the current sample.</div>
            )}
          </div>
        </section>
      </div>

      <section className="bg-pilot-surface border border-pilot-border rounded-lg p-4">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-sm font-bold text-white flex items-center gap-2">
            <Server className="w-4 h-4 text-pilot-accent" />
            Node Health
          </h3>
          {summary?.resource_pressure.metrics_available ? (
            <span className="text-xs text-pilot-success">Metrics server detected</span>
          ) : (
            <span className="text-xs text-pilot-muted">Metrics unavailable</span>
          )}
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm border border-pilot-border rounded-lg overflow-hidden">
            <thead className="bg-pilot-bg text-pilot-muted text-xs uppercase tracking-widest">
              <tr>
                <th className="text-left px-4 py-3">Node</th>
                <th className="text-left px-4 py-3">Status</th>
                <th className="text-left px-4 py-3">CPU</th>
                <th className="text-left px-4 py-3">Memory</th>
                <th className="text-left px-4 py-3">Pressure</th>
                <th className="text-left px-4 py-3">Kubelet</th>
              </tr>
            </thead>
            <tbody>
              {(summary?.nodes || []).map((node) => (
                <tr key={node.name} className="border-t border-pilot-border hover:bg-pilot-bg/60">
                  <td className="px-4 py-3 text-xs font-mono text-white">{node.name}</td>
                  <td className="px-4 py-3">
                    {node.ready ? (
                      <span className="text-xs text-pilot-success flex items-center gap-1"><CheckCircle2 className="w-3.5 h-3.5" /> Ready</span>
                    ) : (
                      <span className="text-xs text-pilot-danger flex items-center gap-1"><AlertTriangle className="w-3.5 h-3.5" /> NotReady</span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-xs text-pilot-muted">
                    {node.cpu_usage ? `${node.cpu_usage} / ${node.cpu_capacity}` : node.cpu_capacity}
                    {node.cpu_usage_percent ? <span className="ml-2 text-pilot-accent">({node.cpu_usage_percent}%)</span> : null}
                  </td>
                  <td className="px-4 py-3 text-xs text-pilot-muted">
                    {node.memory_usage ? `${node.memory_usage} / ${node.memory_capacity}` : node.memory_capacity}
                    {node.memory_usage_percent ? <span className="ml-2 text-pilot-accent">({node.memory_usage_percent}%)</span> : null}
                  </td>
                  <td className="px-4 py-3"><PressureBadges node={node} /></td>
                  <td className="px-4 py-3 text-xs text-pilot-muted">{node.kubelet_version}</td>
                </tr>
              ))}
              {(summary?.nodes || []).length === 0 && (
                <tr>
                  <td colSpan={6} className="px-4 py-8 text-center text-xs text-pilot-muted">No node diagnostics available.</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>

      <section className="bg-pilot-surface border border-pilot-border rounded-lg p-4">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-sm font-bold text-white flex items-center gap-2">
            <FileWarning className="w-4 h-4 text-pilot-danger" />
            Problematic Pods
          </h3>
          <span className="text-xs text-pilot-muted">Crash loops, evictions, pull failures, long-pending pods</span>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm border border-pilot-border rounded-lg overflow-hidden">
            <thead className="bg-pilot-bg text-pilot-muted text-xs uppercase tracking-widest">
              <tr>
                <th className="text-left px-4 py-3">Pod</th>
                <th className="text-left px-4 py-3">Namespace</th>
                <th className="text-left px-4 py-3">Status</th>
                <th className="text-left px-4 py-3">Restarts</th>
                <th className="text-left px-4 py-3">Node</th>
                <th className="text-left px-4 py-3">Reason</th>
                <th className="text-left px-4 py-3">Age</th>
                <th className="text-left px-4 py-3">Actions</th>
              </tr>
            </thead>
            <tbody>
              {(summary?.problem_pods || []).map((pod) => (
                <tr key={`${pod.namespace}/${pod.name}`} className="border-t border-pilot-border hover:bg-pilot-bg/60">
                  <td className="px-4 py-3 text-xs font-mono text-white">{pod.name}</td>
                  <td className="px-4 py-3 text-xs text-pilot-muted">{pod.namespace}</td>
                  <td className="px-4 py-3 text-xs text-pilot-muted">{pod.status}</td>
                  <td className="px-4 py-3 text-xs">
                    <span className={pod.restarts > 5 ? "text-pilot-warning font-bold" : "text-pilot-muted"}>{pod.restarts}</span>
                  </td>
                  <td className="px-4 py-3 text-xs text-pilot-muted">{pod.node || "—"}</td>
                  <td className="px-4 py-3"><ReasonBadge reason={pod.reason} eventType="Warning" /></td>
                  <td className="px-4 py-3 text-xs text-pilot-muted">{formatAgeMinutes(pod.age_minutes)}</td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => setSelectedPod({ namespace: pod.namespace, name: pod.name, initialView: "inspect" })}
                        className="text-xs bg-pilot-accent text-white px-2 py-1 rounded hover:bg-blue-500"
                      >
                        Inspect
                      </button>
                      <button
                        onClick={() => setSelectedPod({ namespace: pod.namespace, name: pod.name, initialView: "ai" })}
                        className="text-xs bg-pilot-warning text-black px-2 py-1 rounded hover:brightness-110 flex items-center gap-1"
                      >
                        <Sparkles className="w-3 h-3" />
                        AI Analyze
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
              {(summary?.problem_pods || []).length === 0 && (
                <tr>
                  <td colSpan={8} className="px-4 py-8 text-center text-xs text-pilot-muted">No problematic pods detected.</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>

      <section className="bg-pilot-surface border border-pilot-border rounded-lg p-4">
        <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4 mb-4">
          <div>
            <h3 className="text-sm font-bold text-white">Kubernetes Events</h3>
            <p className="text-xs text-pilot-muted mt-1">Recent events with server-side filtering and 10s refresh.</p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <div className="flex items-center gap-2 bg-pilot-bg border border-pilot-border rounded px-3 py-1.5">
              <Search className="w-4 h-4 text-pilot-muted" />
              <input
                value={search}
                onChange={(e) => {
                  setSearch(e.target.value);
                  setPage(1);
                }}
                placeholder="Search reason, message, resource"
                className="bg-transparent text-xs text-white placeholder:text-pilot-muted focus:outline-none w-56"
              />
            </div>
            <select value={kind} onChange={(e) => { setKind(e.target.value); setPage(1); }} className="bg-pilot-bg border border-pilot-border rounded px-3 py-1.5 text-xs text-white">
              <option value="">All Resources</option>
              <option value="Pod">Pod</option>
              <option value="Node">Node</option>
              <option value="Deployment">Deployment</option>
              <option value="PersistentVolumeClaim">PVC</option>
            </select>
            <select value={severity} onChange={(e) => { setSeverity(e.target.value); setPage(1); }} className="bg-pilot-bg border border-pilot-border rounded px-3 py-1.5 text-xs text-white">
              <option value="">All Severities</option>
              <option value="Warning">Warning</option>
              <option value="Normal">Normal</option>
            </select>
            <button
              onClick={() => setSortDir((current) => current === "desc" ? "asc" : "desc")}
              className="px-3 py-1.5 text-xs rounded border border-pilot-border bg-pilot-bg text-white hover:bg-pilot-border"
            >
              Sort: {sortDir === "desc" ? "Newest" : "Oldest"}
            </button>
          </div>
        </div>

        {eventsLoading ? (
          <div className="animate-pulse space-y-2">
            {Array.from({ length: 6 }).map((_, index) => (
              <div key={index} className="h-10 rounded bg-pilot-bg" />
            ))}
          </div>
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="w-full text-sm border border-pilot-border rounded-lg overflow-hidden">
                <thead className="bg-pilot-bg text-pilot-muted text-xs uppercase tracking-widest">
                  <tr>
                    <th className="text-left px-4 py-3">Time</th>
                    <th className="text-left px-4 py-3">Namespace</th>
                    <th className="text-left px-4 py-3">Object</th>
                    <th className="text-left px-4 py-3">Type</th>
                    <th className="text-left px-4 py-3">Reason</th>
                    <th className="text-left px-4 py-3">Message</th>
                  </tr>
                </thead>
                <tbody>
                  {pagedEvents.map((event, index) => (
                    <tr key={`${event.involved_object.namespace}-${event.involved_object.name}-${event.reason}-${index}`} className="border-t border-pilot-border hover:bg-pilot-bg/60">
                      <td className="px-4 py-3 text-xs text-pilot-muted whitespace-nowrap">{formatTimestamp(event.last_seen)}</td>
                      <td className="px-4 py-3 text-xs text-pilot-muted">{event.involved_object.namespace || "cluster"}</td>
                      <td className="px-4 py-3 text-xs text-white">
                        <span className="font-mono">{event.involved_object.kind}/{event.involved_object.name}</span>
                      </td>
                      <td className="px-4 py-3 text-xs">
                        <span className={event.type === "Warning" ? "text-pilot-warning font-bold" : "text-pilot-success"}>{event.type}</span>
                      </td>
                      <td className="px-4 py-3"><ReasonBadge reason={event.reason} eventType={event.type} /></td>
                      <td className="px-4 py-3 text-xs text-pilot-muted max-w-[520px] truncate" title={event.message}>{event.message}</td>
                    </tr>
                  ))}
                  {pagedEvents.length === 0 && (
                    <tr>
                      <td colSpan={6} className="px-4 py-8 text-center text-xs text-pilot-muted">No events match the selected filters.</td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>

            <div className="flex items-center justify-between mt-4 text-xs text-pilot-muted">
              <span>Showing {pagedEvents.length} of {events.length} events</span>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => setPage((current) => Math.max(1, current - 1))}
                  disabled={page === 1}
                  className="px-2 py-1 rounded border border-pilot-border disabled:opacity-40"
                >
                  Prev
                </button>
                <span>Page {page} / {totalPages}</span>
                <button
                  onClick={() => setPage((current) => Math.min(totalPages, current + 1))}
                  disabled={page === totalPages}
                  className="px-2 py-1 rounded border border-pilot-border disabled:opacity-40"
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
    { label: "Nodes NotReady", value: health?.not_ready_nodes ?? 0, icon: <Server className="w-4 h-4" />, alert: (health?.not_ready_nodes ?? 0) > 0 },
    { label: "Pod Failures", value: health?.crashloop_pods ?? 0, icon: <AlertTriangle className="w-4 h-4" />, alert: (health?.crashloop_pods ?? 0) > 0 },
    { label: "Failed Mounts", value: health?.failed_mount_events ?? 0, icon: <HardDrive className="w-4 h-4" />, alert: (health?.failed_mount_events ?? 0) > 0 },
    { label: "Warning Events", value: health?.warning_events ?? 0, icon: <Activity className="w-4 h-4" />, alert: (health?.warning_events ?? 0) > 0 },
  ];

  if (loading) {
    return (
      <div className="grid grid-cols-2 xl:grid-cols-4 gap-4">
        {cards.map((card) => (
          <div key={card.label} className="h-24 rounded-lg bg-pilot-surface border border-pilot-border animate-pulse" />
        ))}
      </div>
    );
  }

  return (
    <div className="grid grid-cols-2 xl:grid-cols-4 gap-4">
      {cards.map((card) => (
        <div key={card.label} className="bg-pilot-surface border border-pilot-border rounded-lg p-4">
          <div className={`flex items-center gap-2 mb-2 ${card.alert ? "text-pilot-danger" : "text-pilot-accent"}`}>
            {card.icon}
            <span className="text-xs uppercase tracking-widest text-pilot-muted">{card.label}</span>
          </div>
          <div className={`text-2xl font-bold ${card.alert ? "text-pilot-danger" : "text-white"}`}>{card.value}</div>
        </div>
      ))}
    </div>
  );
}

function ResourcePressurePanel({ summary }: { summary: ClusterTroubleshootingSummary | undefined }) {
  const resource = summary?.resource_pressure;
  return (
    <div className="bg-pilot-bg border border-pilot-border rounded p-4">
      <p className="text-xs uppercase tracking-wider text-pilot-muted mb-3">Resource Pressure</p>
      <div className="space-y-3 text-sm">
        <div className="flex items-center justify-between text-white">
          <span className="flex items-center gap-2"><Cpu className="w-4 h-4 text-pilot-accent" /> Cluster CPU</span>
          <span>{resource?.metrics_available ? `${resource.cpu_usage_percent ?? 0}%` : "n/a"}</span>
        </div>
        <div className="flex items-center justify-between text-white">
          <span className="flex items-center gap-2"><HardDrive className="w-4 h-4 text-pilot-accent" /> Cluster Memory</span>
          <span>{resource?.metrics_available ? `${resource.memory_usage_percent ?? 0}%` : "n/a"}</span>
        </div>
        <div className="text-xs text-pilot-muted pt-2 border-t border-pilot-border space-y-1">
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
      ? "border-pilot-danger"
      : insight.severity === "medium"
      ? "border-pilot-warning"
      : "border-pilot-border";

  return (
    <div className={`bg-pilot-bg border ${severityClass} rounded p-3`}>
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-sm font-bold text-white">{insight.title}</p>
          <p className="text-xs text-pilot-muted mt-1 leading-relaxed">{insight.summary}</p>
        </div>
        <span className="text-[10px] uppercase rounded px-2 py-0.5 bg-pilot-surface text-pilot-muted">{insight.category}</span>
      </div>
      {(insight.suggestions || []).length > 0 && (
        <div className="mt-3 space-y-1.5 text-xs text-white">
          {insight.suggestions.map((suggestion) => (
            <div key={suggestion} className="flex items-start gap-2">
              <Clock3 className="w-3.5 h-3.5 text-pilot-accent mt-0.5 shrink-0" />
              <span>{suggestion}</span>
            </div>
          ))}
        </div>
      )}
      {(insight.affected_resources || []).length > 0 && (
        <div className="mt-3 text-[11px] text-pilot-muted font-mono break-all">
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
    return <span className="text-xs text-pilot-muted">—</span>;
  }

  return (
    <div className="flex gap-1 flex-wrap">
      {active.map((label) => (
        <span key={label} className="text-xs bg-pilot-warning text-black px-1.5 py-0.5 rounded font-bold">
          {label}
        </span>
      ))}
    </div>
  );
}

function ReasonBadge({ reason, eventType }: { reason: string; eventType: string }) {
  const isCritical = IMPORTANT_REASONS.has(reason);
  const cls = isCritical
    ? "bg-red-900/40 text-red-300 border-red-700"
    : eventType === "Warning"
    ? "bg-yellow-900/30 text-yellow-300 border-yellow-700"
    : "bg-pilot-bg text-pilot-muted border-pilot-border";

  return <span className={`text-xs px-2 py-0.5 rounded border ${cls}`}>{reason || "—"}</span>;
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
    <div className="fixed inset-0 bg-black/60 z-50 flex justify-end">
      <div className="w-full max-w-3xl bg-pilot-bg border-l border-pilot-border h-full overflow-y-auto p-6">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h3 className="font-bold text-white text-sm">Pod Troubleshooting — {namespace}/{pod}</h3>
            <p className="text-xs text-pilot-muted mt-1">Description, recent logs, related events, and AI diagnosis.</p>
          </div>
          <button onClick={onClose} className="text-pilot-muted hover:text-white p-1"><X className="w-4 h-4" /></button>
        </div>

        <div className="flex items-center gap-2 mb-4">
          <button
            onClick={() => setActiveView("inspect")}
            className={`px-3 py-1.5 text-xs rounded border ${
              activeView === "inspect"
                ? "bg-pilot-accent border-pilot-accent text-white"
                : "bg-pilot-surface border-pilot-border text-pilot-muted hover:text-white"
            }`}
          >
            Inspect
          </button>
          <button
            onClick={() => setActiveView("ai")}
            className={`px-3 py-1.5 text-xs rounded border flex items-center gap-1 ${
              activeView === "ai"
                ? "bg-pilot-warning border-pilot-warning text-black"
                : "bg-pilot-surface border-pilot-border text-pilot-muted hover:text-white"
            }`}
          >
            <Sparkles className="w-3 h-3" />
            AI Analysis
          </button>
          {activeView === "ai" && (
            <button
              onClick={() => void refetchAI()}
              className="ml-auto px-3 py-1.5 text-xs rounded bg-pilot-surface border border-pilot-border text-white hover:bg-pilot-border flex items-center gap-1"
            >
              <RefreshCw className={`w-3 h-3 ${aiFetching ? "animate-spin" : ""}`} />
              Re-run AI
            </button>
          )}
        </div>

        {isLoading ? (
          <div className="flex items-center gap-2 text-pilot-muted text-sm">
            <RefreshCw className="w-4 h-4 animate-spin" /> Loading pod diagnostics…
          </div>
        ) : diag ? (
          <div className="space-y-5">
            {activeView === "inspect" ? (
              <>
                <div className="bg-pilot-surface border border-pilot-border rounded p-4 space-y-2 text-xs">
                  <Row label="Phase" value={diag.phase} />
                  <Row label="Node" value={diag.node_name || "—"} />
                  <Row label="Service Account" value={diag.service_account || "default"} />
                  <Row label="Created" value={formatTimestamp(diag.created_at)} />
                  <Row label="Volumes" value={(diag.volumes || []).join(", ") || "—"} />
                </div>

                <section className="bg-pilot-surface border border-pilot-border rounded p-4">
                  <h4 className="text-xs font-bold text-white uppercase tracking-widest mb-3">Container Status</h4>
                  <div className="space-y-2">
                    {(diag.container_statuses || []).map((container) => (
                      <div key={container.name} className="bg-pilot-bg border border-pilot-border rounded p-3 text-xs">
                        <div className="flex items-center justify-between gap-3">
                          <span className="font-mono text-white">{container.name}</span>
                          <span className={container.ready ? "text-pilot-success" : "text-pilot-warning"}>{container.state || "Unknown"}</span>
                        </div>
                        <div className="mt-2 text-pilot-muted break-all">{container.image}</div>
                        <div className="mt-1 text-pilot-muted">Reason: {container.state_reason || container.last_terminated_reason || "—"} • Restarts: {container.restart_count}</div>
                        {container.state_message && <div className="mt-1 text-pilot-warning">{container.state_message}</div>}
                      </div>
                    ))}
                  </div>
                </section>

                <section className="bg-pilot-surface border border-pilot-border rounded p-4">
                  <h4 className="text-xs font-bold text-white uppercase tracking-widest mb-3">Related Events</h4>
                  <div className="space-y-2 max-h-72 overflow-y-auto">
                    {(diag.events || []).length > 0 ? (
                      (diag.events || []).map((event, index) => (
                        <div key={`${event.reason}-${index}`} className="bg-pilot-bg border border-pilot-border rounded p-3 text-xs">
                          <div className="flex items-center justify-between gap-2">
                            <ReasonBadge reason={event.reason} eventType={event.type} />
                            <span className="text-pilot-muted">{formatTimestamp(event.last_seen)}</span>
                          </div>
                          <div className="text-pilot-muted mt-2">{event.message}</div>
                        </div>
                      ))
                    ) : (
                      <div className="text-xs text-pilot-muted">No related events found for this pod.</div>
                    )}
                  </div>
                </section>

                <LogViewer title="Recent Logs" content={data?.logs || ""} maxHeight="360px" />
              </>
            ) : (
              <section className="bg-pilot-surface border border-pilot-border rounded p-4 space-y-4">
                <div>
                  <p className="text-xs uppercase tracking-wider text-pilot-muted mb-2">AI Troubleshooting</p>
                  <p className="text-xs text-pilot-muted mb-3">
                    Analyze pod status, recent events, and logs to infer likely root cause and next remediation steps.
                  </p>
                </div>

                {aiLoading || aiFetching ? (
                  <div className="flex items-center gap-2 text-pilot-muted text-sm">
                    <RefreshCw className="w-4 h-4 animate-spin" /> Analyzing error signals with AI…
                  </div>
                ) : aiReport ? (
                  <>
                    <div className="bg-pilot-bg border border-pilot-border rounded p-4">
                      <p className="text-xs uppercase tracking-wider text-pilot-muted mb-2">Root Cause</p>
                      <p className="text-sm font-bold text-pilot-danger">{aiReport.RootCause || "Unknown root cause"}</p>
                    </div>
                    <div className="bg-pilot-bg border border-pilot-border rounded p-4">
                      <p className="text-xs uppercase tracking-wider text-pilot-muted mb-2">Analysis</p>
                      <p className="text-sm text-white leading-relaxed whitespace-pre-wrap">{aiReport.Analysis}</p>
                    </div>
                    {(aiReport.Actions || []).length > 0 && (
                      <div className="bg-pilot-bg border border-pilot-border rounded p-4">
                        <p className="text-xs uppercase tracking-wider text-pilot-muted mb-3">Suggested Actions</p>
                        <div className="space-y-2">
                          {(aiReport.Actions || []).map((action, index) => (
                            <div key={`${action.type}-${index}`} className="border border-pilot-border rounded p-3 bg-pilot-surface/40">
                              <div className="text-xs font-bold text-pilot-accent uppercase">{action.type}</div>
                              <div className="text-xs text-pilot-muted mt-1">{action.explanation}</div>
                              {action.namespace || action.resource ? (
                                <div className="text-[11px] text-white mt-2 font-mono">
                                  {[action.namespace, action.resource].filter(Boolean).join("/")}
                                </div>
                              ) : null}
                            </div>
                          ))}
                        </div>
                      </div>
                    )}
                  </>
                ) : (
                  <div className="text-pilot-muted text-sm">No AI analysis available.</div>
                )}
              </section>
            )}
          </div>
        ) : (
          <div className="text-pilot-danger text-sm">Pod diagnostics unavailable.</div>
        )}
      </div>
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex gap-2">
      <span className="text-pilot-muted w-28 shrink-0">{label}</span>
      <span className="text-white break-all">{value}</span>
    </div>
  );
}

function formatTimestamp(value: string) {
  if (!value) return "—";
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
