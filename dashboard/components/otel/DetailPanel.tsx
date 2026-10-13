/** Side panel for a selected service-map node or edge. */
import { X, ScrollText, AlertTriangle, ExternalLink } from "lucide-react";
import type { OTELGraphEdge, OTELGraphNode, OTELRed } from "@/lib/api";
import { HEALTH_LABEL, HEALTH_TEXT, delta, fmtMs, fmtPct, fmtRate } from "./format";

function RedGrid({ stats, baseline }: { stats: OTELRed; baseline?: OTELRed }) {
  const cells: Array<{ label: string; value: string; d: ReturnType<typeof delta>; base?: string }> = [
    { label: "Throughput", value: fmtRate(stats.rate_per_min), d: delta(stats.rate_per_min, baseline?.rate_per_min, "neutral"), base: baseline && fmtRate(baseline.rate_per_min) },
    { label: "Error rate", value: fmtPct(stats.error_rate), d: delta(stats.error_rate, baseline?.error_rate), base: baseline && fmtPct(baseline.error_rate) },
    { label: "p50", value: fmtMs(stats.p50_ms), d: delta(stats.p50_ms, baseline?.p50_ms), base: baseline && fmtMs(baseline.p50_ms) },
    { label: "p95", value: fmtMs(stats.p95_ms), d: delta(stats.p95_ms, baseline?.p95_ms), base: baseline && fmtMs(baseline.p95_ms) },
  ];
  return (
    <div className="grid grid-cols-2 gap-2">
      {cells.map((c) => (
        <div key={c.label} className="rounded-lg bg-pilot-surface-2/60 px-3 py-2">
          <div className="text-2xs text-pilot-muted">{c.label}</div>
          <div className="flex items-baseline gap-2">
            <span className="font-mono font-semibold text-pilot-text-primary">{c.value}</span>
            {c.d && <span className={`text-2xs font-mono ${c.d.cls}`}>{c.d.text}</span>}
          </div>
          {c.base && <div className="text-2xs text-pilot-muted">baseline {c.base}</div>}
        </div>
      ))}
    </div>
  );
}

export function NodePanel({
  node,
  onClose,
  onTraces,
  onInsight,
}: {
  node: OTELGraphNode;
  onClose: () => void;
  onTraces: (opts: { service: string; errors?: boolean }) => void;
  onInsight?: () => void;
}) {
  const pods = node.pods || [];
  return (
    <aside className="rounded-xl border border-pilot-border bg-pilot-surface p-4 space-y-4">
      <header className="flex items-start gap-2">
        <div className="min-w-0 flex-1">
          <div className="eyebrow">{node.kind === "service" ? "Service" : node.system || node.kind}</div>
          <h3 className="font-display font-bold text-lg text-pilot-text-primary truncate">{node.name}</h3>
          <div className="text-xs text-pilot-muted">
            {[node.namespace, node.workload && `deploy/${node.workload}`, node.language, node.version && `v${node.version}`].filter(Boolean).join(" · ")}
          </div>
          <div className={`text-sm font-medium mt-1 ${HEALTH_TEXT[node.health]}`}>{HEALTH_LABEL[node.health]}</div>
        </div>
        <button type="button" onClick={onClose} aria-label="Close" className="text-pilot-muted hover:text-pilot-text-primary">
          <X className="w-4 h-4" />
        </button>
      </header>

      {node.insight_id && onInsight && (
        <button type="button" onClick={onInsight} className="w-full text-left rounded-lg border border-pilot-danger/40 bg-pilot-danger/[0.06] px-3 py-2 text-sm flex items-center gap-2">
          <AlertTriangle className="w-4 h-4 text-pilot-danger" />
          <span className="text-pilot-text-primary">Part of an active incident — view AI analysis</span>
        </button>
      )}

      {node.stats.requests > 0 && <RedGrid stats={node.stats} baseline={node.baseline} />}

      <div className="flex flex-wrap gap-2">
        <button type="button" onClick={() => onTraces({ service: node.id })} className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium border border-pilot-border hover:border-pilot-accent/50">
          <ScrollText className="w-3.5 h-3.5" /> Traces
        </button>
        {node.stats.errors > 0 && (
          <button type="button" onClick={() => onTraces({ service: node.id, errors: true })} className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium border border-pilot-danger/40 text-pilot-danger hover:bg-pilot-danger/10">
            <ScrollText className="w-3.5 h-3.5" /> Failing traces ({node.stats.errors})
          </button>
        )}
      </div>

      {node.top_errors && node.top_errors.length > 0 && (
        <section>
          <h4 className="eyebrow mb-1">Top errors</h4>
          <ul className="space-y-1">
            {node.top_errors.map((e) => (
              <li key={e} className="font-mono text-xs text-pilot-danger/90 bg-pilot-danger/[0.05] rounded px-2 py-1 break-all">{e}</li>
            ))}
          </ul>
        </section>
      )}

      {pods.length > 0 && (
        <section>
          <h4 className="eyebrow mb-1">Pods ({pods.length})</h4>
          <ul className="space-y-1">
            {pods.map((p) => (
              <li key={p.name} className="flex items-center gap-2 text-xs">
                <span className={`w-2 h-2 rounded-full ${!p.known ? "bg-pilot-muted" : p.ready && p.restarts === 0 ? "bg-pilot-success" : p.ready ? "bg-pilot-warning" : "bg-pilot-danger"}`} />
                <a href={`/dashboard?pod=${encodeURIComponent(`${node.namespace}/${p.name}`)}`} className="font-mono truncate text-pilot-text-secondary hover:text-pilot-accent">
                  {p.name}
                </a>
                <span className="ml-auto text-pilot-muted whitespace-nowrap">
                  {p.known ? `${p.reason || p.phase}${p.restarts ? ` · ${p.restarts}↻` : ""}` : "gone"}
                </span>
                {p.known && <ExternalLink className="w-3 h-3 text-pilot-muted" />}
              </li>
            ))}
          </ul>
        </section>
      )}

      {node.operations && node.operations.length > 0 && (
        <section>
          <h4 className="eyebrow mb-1">Operations</h4>
          <table className="w-full text-xs">
            <thead className="text-pilot-muted text-left">
              <tr>
                <th className="font-medium py-1">Endpoint</th>
                <th className="font-medium py-1 text-right">Rate</th>
                <th className="font-medium py-1 text-right">Err</th>
                <th className="font-medium py-1 text-right">p95</th>
              </tr>
            </thead>
            <tbody className="font-mono">
              {node.operations.map((o) => (
                <tr key={o.name} className="border-t border-pilot-border/60">
                  <td className="py-1 pr-2 truncate max-w-[10rem] text-pilot-text-secondary" title={o.name}>{o.name}</td>
                  <td className="py-1 text-right">{fmtRate(o.rate_per_min)}</td>
                  <td className={`py-1 text-right ${o.errors ? "text-pilot-danger" : ""}`}>{fmtPct(o.error_rate)}</td>
                  <td className="py-1 text-right">{fmtMs(o.p95_ms)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      )}
    </aside>
  );
}

export function EdgePanel({
  edge,
  onClose,
  onTraces,
}: {
  edge: OTELGraphEdge;
  onClose: () => void;
  onTraces: (opts: { edge: string; errors?: boolean }) => void;
}) {
  const short = (id: string) => (id.includes("/") ? id.split("/").pop() : id.replace(/^(db|mq|ext):/, ""));
  return (
    <aside className="rounded-xl border border-pilot-border bg-pilot-surface p-4 space-y-4">
      <header className="flex items-start gap-2">
        <div className="min-w-0 flex-1">
          <div className="eyebrow">Dependency {edge.protocol ? `· ${edge.protocol}` : ""}</div>
          <h3 className="font-display font-bold text-pilot-text-primary break-all">
            {short(edge.source)} → {short(edge.target)}
          </h3>
          <div className={`text-sm font-medium mt-1 ${HEALTH_TEXT[edge.health]}`}>{HEALTH_LABEL[edge.health]}</div>
        </div>
        <button type="button" onClick={onClose} aria-label="Close" className="text-pilot-muted hover:text-pilot-text-primary">
          <X className="w-4 h-4" />
        </button>
      </header>
      <RedGrid stats={edge.stats} baseline={edge.baseline} />
      <div className="flex flex-wrap gap-2">
        <button type="button" onClick={() => onTraces({ edge: edge.id })} className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium border border-pilot-border hover:border-pilot-accent/50">
          <ScrollText className="w-3.5 h-3.5" /> Traces across this call
        </button>
        {edge.stats.errors > 0 && (
          <button type="button" onClick={() => onTraces({ edge: edge.id, errors: true })} className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium border border-pilot-danger/40 text-pilot-danger hover:bg-pilot-danger/10">
            <ScrollText className="w-3.5 h-3.5" /> Failing ({edge.stats.errors})
          </button>
        )}
      </div>
    </aside>
  );
}
