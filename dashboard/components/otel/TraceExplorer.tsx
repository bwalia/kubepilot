/**
 * TraceExplorer — filterable trace list + waterfall with span details and
 * logs correlated by trace ID.
 */
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { AlertCircle, ChevronDown, ChevronRight, Copy, X } from "lucide-react";
import { getOTELTrace, getOTELTraceSummaries, type OTELSpan, type OTELTraceDetail } from "@/lib/api";
import { fmtMs, serviceColor, timeAgo } from "./format";

export interface TraceFilter {
  service?: string;
  edge?: string;
  errors?: boolean;
  min_ms?: number;
}

export function TraceExplorer({
  window,
  filter,
  setFilter,
  traceId,
  setTraceId,
}: {
  window: string;
  filter: TraceFilter;
  setFilter: (f: TraceFilter) => void;
  traceId: string | null;
  setTraceId: (id: string | null) => void;
}) {
  const { data, isLoading } = useQuery({
    queryKey: ["otel-trace-summaries", window, filter],
    queryFn: () => getOTELTraceSummaries({ window, ...filter, limit: 150 }),
    refetchInterval: 10_000,
  });
  const traces = data?.traces ?? [];
  const maxDur = Math.max(1, ...traces.map((t) => t.duration_ms));

  return (
    <div className="grid gap-4 2xl:grid-cols-[minmax(0,5fr)_minmax(0,7fr)]">
      <section className="rounded-xl border border-pilot-border bg-pilot-surface overflow-hidden">
        <div className="flex flex-wrap items-center gap-2 p-3 border-b border-pilot-border">
          {(filter.service || filter.edge) && (
            <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bg-pilot-accent/10 text-pilot-accent text-xs font-mono">
              {filter.edge ? filter.edge.replace(/^.*\//, "") : filter.service}
              <button type="button" aria-label="Clear filter" onClick={() => setFilter({ ...filter, service: undefined, edge: undefined })}>
                <X className="w-3 h-3" />
              </button>
            </span>
          )}
          <label className="inline-flex items-center gap-1.5 text-xs text-pilot-muted">
            <input type="checkbox" checked={!!filter.errors} onChange={(e) => setFilter({ ...filter, errors: e.target.checked })} />
            Errors only
          </label>
          <select
            value={filter.min_ms ?? 0}
            onChange={(e) => setFilter({ ...filter, min_ms: Number(e.target.value) || undefined })}
            className="text-xs bg-pilot-surface-2 border border-pilot-border rounded-md px-2 py-1"
            aria-label="Minimum duration"
          >
            <option value={0}>Any duration</option>
            <option value={100}>≥ 100ms</option>
            <option value={500}>≥ 500ms</option>
            <option value={1000}>≥ 1s</option>
            <option value={5000}>≥ 5s</option>
          </select>
          <span className="ml-auto text-xs text-pilot-muted">{traces.length} traces</span>
        </div>
        <ul className="max-h-[65vh] overflow-y-auto divide-y divide-pilot-border/60">
          {isLoading && <li className="p-6 text-sm text-pilot-muted">Loading traces…</li>}
          {!isLoading && traces.length === 0 && <li className="p-6 text-sm text-pilot-muted text-center">No traces match in this window.</li>}
          {traces.map((t) => (
            <li key={t.trace_id}>
              <button
                type="button"
                onClick={() => setTraceId(t.trace_id)}
                className={`w-full text-left px-3 py-2.5 hover:bg-pilot-surface-2/50 ${traceId === t.trace_id ? "bg-pilot-accent/[0.08]" : ""}`}
              >
                <div className="flex items-center gap-2">
                  {t.error_count > 0 ? <AlertCircle className="w-4 h-4 text-pilot-danger shrink-0" /> : <span className="w-2 h-2 rounded-full shrink-0" style={{ background: serviceColor(t.root_service) }} />}
                  <span className="text-sm font-medium text-pilot-text-primary truncate">{t.root_name || "(unnamed)"}</span>
                  <span className="ml-auto font-mono text-xs text-pilot-text-secondary">{fmtMs(t.duration_ms)}</span>
                </div>
                <div className="mt-1 h-1 rounded-full bg-pilot-surface-2 overflow-hidden">
                  <div className={`h-full ${t.error_count ? "bg-pilot-danger" : "bg-pilot-accent"}`} style={{ width: `${Math.max(2, (t.duration_ms / maxDur) * 100)}%` }} />
                </div>
                <div className="mt-1 flex items-center gap-2 text-2xs text-pilot-muted">
                  <span className="truncate">{t.services.join(" → ")}</span>
                  <span className="ml-auto whitespace-nowrap">
                    {t.span_count} spans · {timeAgo(t.start)}
                  </span>
                </div>
                {t.error && <div className="mt-0.5 text-2xs font-mono text-pilot-danger truncate">{t.error}</div>}
              </button>
            </li>
          ))}
        </ul>
      </section>

      <section className="rounded-xl border border-pilot-border bg-pilot-surface min-h-[20rem]">
        {traceId ? (
          <TraceWaterfall traceId={traceId} onClose={() => setTraceId(null)} />
        ) : (
          <div className="h-full flex items-center justify-center text-sm text-pilot-muted p-8 text-center">Select a trace to see its waterfall, span attributes and logs.</div>
        )}
      </section>
    </div>
  );
}

interface Row {
  span: OTELSpan;
  depth: number;
  hasChildren: boolean;
}

function buildRows(spans: OTELSpan[], collapsed: Set<string>): Row[] {
  const ids = new Set(spans.map((s) => s.span_id));
  const children = new Map<string, OTELSpan[]>();
  const roots: OTELSpan[] = [];
  spans.forEach((s) => {
    if (s.parent_id && ids.has(s.parent_id)) {
      if (!children.has(s.parent_id)) children.set(s.parent_id, []);
      children.get(s.parent_id)!.push(s);
    } else roots.push(s);
  });
  const byStart = (a: OTELSpan, b: OTELSpan) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime();
  const rows: Row[] = [];
  const walk = (s: OTELSpan, depth: number) => {
    const kids = (children.get(s.span_id) || []).sort(byStart);
    rows.push({ span: s, depth, hasChildren: kids.length > 0 });
    if (!collapsed.has(s.span_id)) kids.forEach((k) => walk(k, depth + 1));
  };
  roots.sort(byStart).forEach((r) => walk(r, 0));
  return rows;
}

function TraceWaterfall({ traceId, onClose }: { traceId: string; onClose: () => void }) {
  const { data, isLoading, error } = useQuery<OTELTraceDetail>({
    queryKey: ["otel-trace", traceId],
    queryFn: () => getOTELTrace(traceId),
  });
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const [sel, setSel] = useState<OTELSpan | null>(null);

  const rows = useMemo(() => (data ? buildRows(data.spans, collapsed) : []), [data, collapsed]);
  if (isLoading) return <div className="p-6 text-sm text-pilot-muted">Loading trace…</div>;
  if (error || !data) return <div className="p-6 text-sm text-pilot-warning">Trace not found — it may have aged out of the managed store.</div>;

  const t0 = Math.min(...data.spans.map((s) => new Date(s.timestamp).getTime()));
  const total = Math.max(0.001, data.duration_ms);
  const services = Array.from(new Set(data.spans.map((s) => s.service || "?")));

  return (
    <div className="flex flex-col h-full">
      <header className="flex flex-wrap items-center gap-2 p-3 border-b border-pilot-border">
        <div className="min-w-0">
          <div className="font-semibold text-pilot-text-primary truncate">{data.root_name}</div>
          <div className="text-2xs text-pilot-muted font-mono flex items-center gap-1">
            {traceId}
            <button type="button" aria-label="Copy trace ID" onClick={() => navigator.clipboard?.writeText(traceId)}>
              <Copy className="w-3 h-3" />
            </button>
          </div>
        </div>
        <div className="ml-auto flex items-center gap-3 text-xs text-pilot-muted">
          <span className="font-mono text-pilot-text-primary">{fmtMs(data.duration_ms)}</span>
          <span>{data.span_count} spans</span>
          {data.error_count > 0 && <span className="text-pilot-danger">{data.error_count} errors</span>}
          <button type="button" aria-label="Close trace" onClick={onClose} className="hover:text-pilot-text-primary">
            <X className="w-4 h-4" />
          </button>
        </div>
        <div className="w-full flex flex-wrap gap-3 text-2xs">
          {services.map((s) => (
            <span key={s} className="inline-flex items-center gap-1 text-pilot-text-secondary">
              <span className="w-2.5 h-2.5 rounded-sm" style={{ background: serviceColor(s) }} />
              {s}
            </span>
          ))}
        </div>
      </header>

      <div className="overflow-auto max-h-[50vh]">
        <div className="min-w-[640px]">
          <div className="grid grid-cols-[minmax(14rem,2fr)_3fr] text-2xs text-pilot-muted border-b border-pilot-border/60 sticky top-0 bg-pilot-surface z-10">
            <div className="px-3 py-1">Span</div>
            <div className="relative py-1">
              {[0, 0.25, 0.5, 0.75].map((f) => (
                <span key={f} className="absolute font-mono" style={{ left: `${f * 100}%` }}>
                  {fmtMs(total * f)}
                </span>
              ))}
              &nbsp;
            </div>
          </div>
          {rows.map(({ span: s, depth, hasChildren }) => {
            const start = new Date(s.timestamp).getTime() - t0;
            const dur = s.duration / 1e6;
            const left = (start / total) * 100;
            const width = Math.max(0.4, (dur / total) * 100);
            const err = s.status === "ERROR";
            return (
              <div
                key={s.span_id}
                onClick={() => setSel(s)}
                className={`grid grid-cols-[minmax(14rem,2fr)_3fr] items-center text-xs cursor-pointer hover:bg-pilot-surface-2/50 ${sel?.span_id === s.span_id ? "bg-pilot-accent/[0.08]" : ""}`}
              >
                <div className="flex items-center gap-1 px-3 py-1 min-w-0" style={{ paddingLeft: 12 + depth * 14 }}>
                  {hasChildren ? (
                    <button
                      type="button"
                      aria-label="Toggle children"
                      onClick={(e) => {
                        e.stopPropagation();
                        const next = new Set(collapsed);
                        next.has(s.span_id) ? next.delete(s.span_id) : next.add(s.span_id);
                        setCollapsed(next);
                      }}
                      className="text-pilot-muted"
                    >
                      {collapsed.has(s.span_id) ? <ChevronRight className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
                    </button>
                  ) : (
                    <span className="w-3" />
                  )}
                  <span className="w-1.5 h-4 rounded-sm shrink-0" style={{ background: serviceColor(s.service) }} />
                  <span className={`truncate ${err ? "text-pilot-danger font-medium" : "text-pilot-text-primary"}`} title={s.name}>
                    {s.name}
                  </span>
                  <span className="text-pilot-muted truncate">· {s.service}</span>
                </div>
                <div className="relative h-6 mr-3">
                  <div
                    className={`absolute top-1.5 h-3 rounded-sm ${err ? "ring-1 ring-pilot-danger" : ""}`}
                    style={{ left: `${left}%`, width: `${width}%`, background: err ? "rgb(var(--p-danger))" : serviceColor(s.service), opacity: 0.85 }}
                  />
                  <span className="absolute top-1 text-2xs font-mono text-pilot-muted" style={{ left: `calc(${Math.min(left + width, 88)}% + 4px)` }}>
                    {fmtMs(dur)}
                  </span>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {sel && <SpanDetail span={sel} onClose={() => setSel(null)} />}

      {data.logs && data.logs.length > 0 && (
        <section className="border-t border-pilot-border p-3">
          <h4 className="eyebrow mb-1">Logs in this trace</h4>
          <ul className="space-y-0.5 max-h-48 overflow-y-auto font-mono text-2xs">
            {data.logs.map((l, i) => (
              <li key={i} className="flex gap-2">
                <span className={/err|fatal/i.test(l.severity || "") ? "text-pilot-danger" : /warn/i.test(l.severity || "") ? "text-pilot-warning" : "text-pilot-muted"}>
                  {(l.severity || "LOG").slice(0, 5)}
                </span>
                <span className="text-pilot-muted">{l.service}</span>
                <span className="text-pilot-text-secondary break-all">{l.body}</span>
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}

function SpanDetail({ span, onClose }: { span: OTELSpan; onClose: () => void }) {
  const labels = Object.entries(span.labels || {}).sort(([a], [b]) => a.localeCompare(b));
  const resource = labels.filter(([k]) => k.startsWith("k8s.") || k.startsWith("service.") || k.startsWith("deployment.") || k.startsWith("telemetry."));
  const attrs = labels.filter(([k]) => !resource.some(([r]) => r === k));
  return (
    <section className="border-t border-pilot-border p-3 space-y-2">
      <div className="flex items-center gap-2">
        <h4 className="font-semibold text-sm text-pilot-text-primary truncate">{span.name}</h4>
        <span className="text-2xs text-pilot-muted">
          {span.kind} · {fmtMs(span.duration / 1e6)} · {span.status}
        </span>
        <button type="button" aria-label="Close span" onClick={onClose} className="ml-auto text-pilot-muted hover:text-pilot-text-primary">
          <X className="w-4 h-4" />
        </button>
      </div>
      {span.status_message && <div className="font-mono text-xs text-pilot-danger bg-pilot-danger/[0.05] rounded px-2 py-1 break-all">{span.status_message}</div>}
      <div className="grid md:grid-cols-2 gap-3">
        {[
          ["Attributes", attrs],
          ["Resource", resource],
        ].map(([title, rows]) => (
          <div key={title as string}>
            <div className="eyebrow mb-1">{title as string}</div>
            <dl className="text-2xs font-mono space-y-0.5">
              {(rows as [string, string][]).map(([k, v]) => (
                <div key={k} className="flex gap-2">
                  <dt className="text-pilot-muted shrink-0">{k}</dt>
                  <dd className="text-pilot-text-secondary break-all">{v}</dd>
                </div>
              ))}
              {(rows as [string, string][]).length === 0 && <div className="text-pilot-muted">—</div>}
            </dl>
          </div>
        ))}
      </div>
    </section>
  );
}
