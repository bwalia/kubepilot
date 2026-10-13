/**
 * OTLP Observability — trace-derived service map, trace explorer and
 * AI incident insights, plus the Autopilot coverage cockpit.
 *
 * The page answers three questions in order: is anything wrong right now
 * (insights), where is it in the system (map), and what exactly happened
 * (traces → spans → logs).
 */
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Activity, Map as MapIcon, Radio, ScrollText, Waypoints } from "lucide-react";
import { ClusterStatusBar } from "@/components/ClusterStatusBar";
import { OTELAutopilotPanel } from "@/components/OTELAutopilotPanel";
import { InsightsFeed } from "@/components/otel/InsightsFeed";
import { ServiceMap } from "@/components/otel/ServiceMap";
import { EdgePanel, NodePanel } from "@/components/otel/DetailPanel";
import { TraceExplorer, type TraceFilter } from "@/components/otel/TraceExplorer";
import { HEALTH_LABEL, HEALTH_STROKE, HEALTH_TEXT, fmtMs, fmtPct, fmtRate } from "@/components/otel/format";
import { getOTELInsights, getOTELServiceGraph, type OTELGraphEdge, type OTELGraphNode } from "@/lib/api";

type Tab = "map" | "traces" | "coverage";
const WINDOWS = ["5m", "15m", "1h", "6h"] as const;

export default function OTELPage() {
  const [tab, setTab] = useState<Tab>("map");
  const [timeWindow, setTimeWindow] = useState<string>("15m");
  const [namespace, setNamespace] = useState("");
  const [onlyUnhealthy, setOnlyUnhealthy] = useState(false);
  const [selNode, setSelNode] = useState<string | null>(null);
  const [selEdge, setSelEdge] = useState<OTELGraphEdge | null>(null);
  const [traceFilter, setTraceFilter] = useState<TraceFilter>({});
  const [traceId, setTraceId] = useState<string | null>(null);

  const { data: graph, isLoading, error } = useQuery({
    queryKey: ["otel-graph", timeWindow, namespace],
    queryFn: () => getOTELServiceGraph({ window: timeWindow, namespace: namespace || undefined }),
    refetchInterval: 10_000,
  });
  const { data: insightData } = useQuery({
    queryKey: ["otel-insights"],
    queryFn: getOTELInsights,
    refetchInterval: 5_000,
  });
  // Namespaces come from an unfiltered graph so the selector never empties itself.
  const { data: allGraph } = useQuery({
    queryKey: ["otel-graph", timeWindow, ""],
    queryFn: () => getOTELServiceGraph({ window: timeWindow }),
    refetchInterval: 30_000,
    enabled: namespace !== "",
  });

  const insights = insightData?.insights ?? [];
  const nodes = graph?.nodes ?? [];
  const services = nodes.filter((n) => n.kind === "service");
  const namespaces = useMemo(
    () => Array.from(new Set((allGraph ?? graph)?.nodes.map((n) => n.namespace).filter(Boolean) as string[])).sort(),
    [allGraph, graph],
  );
  const node = nodes.find((n) => n.id === selNode) || null;
  const hasData = (graph?.span_count ?? 0) > 0;

  const showNode = (id: string) => {
    setTab("map");
    setSelEdge(null);
    setSelNode(id);
    setOnlyUnhealthy(false);
  };
  const showTraces = (f: TraceFilter) => {
    setTraceFilter(f);
    setTraceId(null);
    setTab("traces");
  };
  const showTrace = (id: string) => {
    setTraceFilter({});
    setTraceId(id);
    setTab("traces");
  };

  const counts = {
    critical: services.filter((n) => n.health === "critical").length,
    degraded: services.filter((n) => n.health === "degraded").length,
    healthy: services.filter((n) => n.health === "healthy").length,
  };

  return (
    <div className="min-h-screen bg-pilot-bg text-pilot-text-primary">
      <header className="bg-pilot-surface border-b border-pilot-border px-4 sm:px-6 lg:px-8 py-4">
        <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-3">
          <div className="min-w-0">
            <h1 className="font-display text-2xl font-bold tracking-tight text-pilot-text-primary flex items-center gap-2">
              <Waypoints className="w-6 h-6 text-pilot-accent" /> Observability
            </h1>
            <p className="text-sm text-pilot-muted mt-1">
              Live service map from OpenTelemetry traces, with AI that spots incidents and explains them before you ask.
            </p>
          </div>
          <ClusterStatusBar />
        </div>
      </header>

      <main className="px-4 sm:px-6 lg:px-8 py-6 space-y-6">
        <InsightsFeed insights={insights} serviceCount={services.length} hasData={hasData} onShowNode={showNode} onShowTrace={showTrace} />

        <nav className="flex flex-wrap items-center gap-2 border-b border-pilot-border" aria-label="Observability views">
          {(
            [
              ["map", "Service map", MapIcon],
              ["traces", "Traces", ScrollText],
              ["coverage", "Coverage & signals", Radio],
            ] as const
          ).map(([id, label, Icon]) => (
            <button
              key={id}
              type="button"
              onClick={() => setTab(id)}
              className={`inline-flex items-center gap-1.5 px-3 py-2 text-sm font-medium border-b-2 -mb-px transition-colors ${
                tab === id ? "border-pilot-accent text-pilot-accent" : "border-transparent text-pilot-muted hover:text-pilot-text-primary"
              }`}
            >
              <Icon className="w-4 h-4" />
              {label}
            </button>
          ))}
          {tab !== "coverage" && (
            <div className="ml-auto flex items-center gap-2 pb-1.5">
              <div className="inline-flex rounded-lg border border-pilot-border bg-pilot-surface p-0.5" role="group" aria-label="Time window">
                {WINDOWS.map((w) => (
                  <button
                    key={w}
                    type="button"
                    onClick={() => setTimeWindow(w)}
                    className={`px-2.5 py-1 text-xs rounded-md font-mono ${timeWindow === w ? "bg-pilot-accent text-white" : "text-pilot-muted hover:text-pilot-text-primary"}`}
                  >
                    {w}
                  </button>
                ))}
              </div>
            </div>
          )}
        </nav>

        {tab === "map" && (
          <>
            {error && <p className="text-sm text-pilot-warning">Could not load the service map: {(error as Error).message}</p>}
            {isLoading && <p className="text-sm text-pilot-muted">Building service map…</p>}
            {graph && !hasData && <ConnectCollector />}
            {graph && hasData && (
              <>
                <div className="flex flex-wrap items-center gap-3 text-sm">
                  <select
                    value={namespace}
                    onChange={(e) => {
                      setNamespace(e.target.value);
                      setSelNode(null);
                    }}
                    className="bg-pilot-surface border border-pilot-border rounded-lg px-2.5 py-1.5 text-sm"
                    aria-label="Namespace"
                  >
                    <option value="">All namespaces</option>
                    {namespaces.map((ns) => (
                      <option key={ns} value={ns}>
                        {ns}
                      </option>
                    ))}
                  </select>
                  <label className="inline-flex items-center gap-1.5 text-pilot-muted">
                    <input type="checkbox" checked={onlyUnhealthy} onChange={(e) => setOnlyUnhealthy(e.target.checked)} />
                    Only problems
                  </label>
                  <div className="flex items-center gap-3 text-xs">
                    {(["critical", "degraded", "healthy"] as const).map((h) => (
                      <span key={h} className="inline-flex items-center gap-1.5">
                        <span className="w-2.5 h-2.5 rounded-full" style={{ background: HEALTH_STROKE[h] }} />
                        <span className={HEALTH_TEXT[h]}>{counts[h]}</span>
                        <span className="text-pilot-muted">{HEALTH_LABEL[h].toLowerCase()}</span>
                      </span>
                    ))}
                  </div>
                  <span className="ml-auto text-xs text-pilot-muted inline-flex items-center gap-1">
                    <Activity className="w-3.5 h-3.5" />
                    {graph.trace_count.toLocaleString()} traces · {graph.span_count.toLocaleString()} spans in {timeWindow} · edge width = traffic
                  </span>
                </div>

                <div className={`grid gap-4 ${node || selEdge ? "xl:grid-cols-[minmax(0,1fr)_22rem]" : ""}`}>
                  <div className="rounded-xl border border-pilot-border bg-pilot-surface-2/30 min-w-0">
                    <ServiceMap
                      graph={graph}
                      selected={selNode}
                      onlyUnhealthy={onlyUnhealthy}
                      onSelectNode={(n) => {
                        setSelEdge(null);
                        setSelNode(n.id === selNode ? null : n.id);
                      }}
                      onSelectEdge={(e) => {
                        setSelNode(null);
                        setSelEdge(e);
                      }}
                    />
                  </div>
                  {node && (
                    <NodePanel
                      node={node}
                      onClose={() => setSelNode(null)}
                      onTraces={(f) => showTraces(f)}
                      onInsight={() => document.querySelector('[aria-label="AI incident insights"]')?.scrollIntoView({ behavior: "smooth" })}
                    />
                  )}
                  {selEdge && <EdgePanel edge={selEdge} onClose={() => setSelEdge(null)} onTraces={(f) => showTraces(f)} />}
                </div>

                <ServicesTable nodes={nodes} onSelect={showNode} />
              </>
            )}
          </>
        )}

        {tab === "traces" && (
          <TraceExplorer window={timeWindow} filter={traceFilter} setFilter={setTraceFilter} traceId={traceId} setTraceId={setTraceId} />
        )}

        {tab === "coverage" && <OTELAutopilotPanel />}
      </main>
    </div>
  );
}

/** Sortable table: the scannable, accessible companion to the map. */
function ServicesTable({ nodes, onSelect }: { nodes: OTELGraphNode[]; onSelect: (id: string) => void }) {
  const [sort, setSort] = useState<"error_rate" | "p95_ms" | "rate_per_min">("error_rate");
  const rows = nodes
    .filter((n) => n.kind !== "clients" && n.stats.requests > 0)
    .sort((a, b) => b.stats[sort] - a.stats[sort] || a.name.localeCompare(b.name));
  const th = (key: typeof sort, label: string) => (
    <th className="px-3 py-2 font-medium text-right">
      <button type="button" onClick={() => setSort(key)} className={sort === key ? "text-pilot-accent" : "hover:text-pilot-text-primary"}>
        {label}
        {sort === key ? " ↓" : ""}
      </button>
    </th>
  );
  return (
    <div className="overflow-x-auto rounded-xl border border-pilot-border">
      <table className="w-full text-sm">
        <thead className="bg-pilot-surface-2 text-pilot-muted text-left">
          <tr>
            <th className="px-3 py-2 font-medium">Service / dependency</th>
            <th className="px-3 py-2 font-medium">Health</th>
            {th("rate_per_min", "Rate")}
            {th("error_rate", "Errors")}
            {th("p95_ms", "p95")}
            <th className="px-3 py-2 font-medium text-right">Pods</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((n) => (
            <tr key={n.id} onClick={() => onSelect(n.id)} className="border-t border-pilot-border/70 hover:bg-pilot-surface-2/40 cursor-pointer">
              <td className="px-3 py-2">
                <div className="font-medium text-pilot-text-primary">{n.name}</div>
                <div className="text-2xs text-pilot-muted">{n.kind === "service" ? n.namespace || "—" : n.system || n.kind}</div>
              </td>
              <td className={`px-3 py-2 ${HEALTH_TEXT[n.health]}`}>{HEALTH_LABEL[n.health]}</td>
              <td className="px-3 py-2 text-right font-mono">{fmtRate(n.stats.rate_per_min)}</td>
              <td className={`px-3 py-2 text-right font-mono ${n.stats.errors ? "text-pilot-danger" : ""}`}>{fmtPct(n.stats.error_rate)}</td>
              <td className="px-3 py-2 text-right font-mono">{fmtMs(n.stats.p95_ms)}</td>
              <td className="px-3 py-2 text-right font-mono text-pilot-muted">
                {n.pods?.length ? `${n.pods.filter((p) => p.known && p.ready).length}/${n.pods.length}` : "—"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/** First-run guidance: how to get traces flowing into KubePilot. */
function ConnectCollector() {
  const yaml = `exporters:
  otlphttp/kubepilot:
    endpoint: http://kubepilot.kubepilot-system.svc:8080/api/v1/otel
    headers:
      Authorization: "Bearer \${env:KUBEPILOT_OTEL_INGEST_TOKEN}"

service:
  pipelines:
    traces:  { receivers: [otlp], processors: [batch], exporters: [otlphttp/kubepilot] }
    logs:    { receivers: [otlp], processors: [batch], exporters: [otlphttp/kubepilot] }
    metrics: { receivers: [otlp], processors: [batch], exporters: [otlphttp/kubepilot] }`;
  return (
    <div className="rounded-xl border border-dashed border-pilot-border bg-pilot-surface p-6 space-y-4">
      <div>
        <h2 className="font-display font-bold text-lg">No traces yet — connect a collector</h2>
        <p className="text-sm text-pilot-muted mt-1 max-w-3xl">
          KubePilot speaks standard OTLP/HTTP (protobuf or JSON, gzip OK). Add it as an extra exporter on any OpenTelemetry collector you already run —
          existing exporters keep working. Set <code className="font-mono">--otel-ingest-token</code> (or <code className="font-mono">KUBEPILOT_OTEL_INGEST_TOKEN</code>) on KubePilot
          so the collector can push without dashboard credentials. The map, traces and AI incident detection light up within a minute.
        </p>
      </div>
      <pre className="text-xs font-mono bg-pilot-surface-2 rounded-lg p-4 overflow-x-auto">{yaml}</pre>
      <p className="text-xs text-pilot-muted">
        Apps without an SDK: switch OTLP Autopilot to <em>Enable</em> under <strong>Coverage &amp; signals</strong> for auto-instrumentation, or run Grafana Beyla for eBPF spans.
      </p>
    </div>
  );
}
