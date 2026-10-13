/**
 * InsightsFeed — incidents KubePilot detected from traces on its own, each
 * ranked to a likely root cause and explained by the AI before anyone asks.
 */
import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  AlertOctagon,
  AlertTriangle,
  ArrowRight,
  Bot,
  CheckCircle2,
  ChevronDown,
  Copy,
  Loader2,
  MapPin,
  RefreshCw,
  ScrollText,
  ShieldCheck,
} from "lucide-react";
import { reanalyzeOTELInsight, type OTELInsight } from "@/lib/api";
import { timeAgo } from "./format";

const SEV: Record<OTELInsight["severity"], { cls: string; Icon: typeof AlertOctagon; label: string }> = {
  critical: { cls: "border-pilot-danger/50 bg-pilot-danger/[0.06]", Icon: AlertOctagon, label: "Critical" },
  high: { cls: "border-pilot-warning/50 bg-pilot-warning/[0.06]", Icon: AlertTriangle, label: "High" },
  medium: { cls: "border-pilot-border bg-pilot-surface", Icon: AlertTriangle, label: "Medium" },
};

const KIND_LABEL: Record<OTELInsight["kind"], string> = {
  error_spike: "Error spike",
  latency_regression: "Latency regression",
  new_error: "New error",
};

const nodeLabel = (id: string) => {
  if (id.startsWith("db:") || id.startsWith("mq:")) return id.split(":").slice(2).join(":") + ` (${id.split(":")[1]})`;
  if (id.startsWith("ext:")) return id.slice(4);
  return id.includes("/") ? id.split("/").pop()! : id;
};

export function InsightsFeed({
  insights,
  serviceCount,
  hasData,
  onShowNode,
  onShowTrace,
}: {
  insights: OTELInsight[];
  serviceCount: number;
  hasData: boolean;
  onShowNode: (id: string) => void;
  onShowTrace: (traceId: string) => void;
}) {
  const open = insights.filter((i) => i.status === "open");
  const resolved = insights.filter((i) => i.status === "resolved").slice(0, 5);
  const [showResolved, setShowResolved] = useState(false);

  return (
    <section aria-label="AI incident insights" className="space-y-3">
      <div className="flex flex-wrap items-center gap-3">
        <div className="flex items-center gap-2">
          <span className="relative flex h-2.5 w-2.5">
            <span className={`absolute inline-flex h-full w-full rounded-full opacity-60 animate-ping ${open.length ? "bg-pilot-danger" : "bg-pilot-success"}`} />
            <span className={`relative inline-flex h-2.5 w-2.5 rounded-full ${open.length ? "bg-pilot-danger" : "bg-pilot-success"}`} />
          </span>
          <h2 className="font-display font-bold text-lg text-pilot-text-primary">
            {open.length === 0 ? "All quiet" : `${open.length} active incident${open.length > 1 ? "s" : ""}`}
          </h2>
        </div>
        <p className="text-sm text-pilot-muted">
          {hasData
            ? `KubePilot AI is watching ${serviceCount} service${serviceCount === 1 ? "" : "s"} — comparing the last 5 min to the previous 30 for error spikes, latency regressions and new errors.`
            : "Waiting for traces. Point an OpenTelemetry collector at KubePilot to start automatic incident detection."}
        </p>
      </div>

      {open.length === 0 && hasData && (
        <div className="rounded-xl border border-pilot-success/30 bg-pilot-success/[0.05] px-4 py-3 flex items-center gap-3 text-sm">
          <ShieldCheck className="w-5 h-5 text-pilot-success shrink-0" />
          <span className="text-pilot-text-secondary">
            No anomalies detected. If one appears, KubePilot pins it here with its root cause and an AI explanation, and posts it to the anomaly timeline (and Slack, if configured).
          </span>
        </div>
      )}

      <div className="grid gap-3 xl:grid-cols-2">
        {open.map((i) => (
          <InsightCard key={i.id} insight={i} onShowNode={onShowNode} onShowTrace={onShowTrace} />
        ))}
      </div>

      {resolved.length > 0 && (
        <div>
          <button type="button" onClick={() => setShowResolved((v) => !v)} className="text-xs text-pilot-muted hover:text-pilot-text-primary inline-flex items-center gap-1">
            <ChevronDown className={`w-3.5 h-3.5 transition-transform ${showResolved ? "" : "-rotate-90"}`} />
            {resolved.length} recently resolved
          </button>
          {showResolved && (
            <ul className="mt-2 space-y-1">
              {resolved.map((i) => (
                <li key={i.id} className="text-sm text-pilot-muted flex items-center gap-2">
                  <CheckCircle2 className="w-4 h-4 text-pilot-success" />
                  <span className="text-pilot-text-secondary">{i.title}</span>
                  <span className="text-xs">resolved {i.resolved_at ? timeAgo(i.resolved_at) : ""}</span>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </section>
  );
}

function InsightCard({
  insight: i,
  onShowNode,
  onShowTrace,
}: {
  insight: OTELInsight;
  onShowNode: (id: string) => void;
  onShowTrace: (traceId: string) => void;
}) {
  const qc = useQueryClient();
  const reanalyze = useMutation({
    mutationFn: () => reanalyzeOTELInsight(i.id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["otel-insights"] }),
  });
  const sev = SEV[i.severity];
  const path = i.cause_path && i.cause_path.length > 1 ? i.cause_path : null;
  const ai = i.ai;
  const thinking = ai.status === "pending" || ai.status === "running";

  return (
    <article className={`rounded-xl border ${sev.cls} p-4 space-y-3`}>
      <header className="flex items-start gap-3">
        <sev.Icon className={`w-5 h-5 mt-0.5 shrink-0 ${i.severity === "critical" ? "text-pilot-danger" : "text-pilot-warning"}`} />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2 text-2xs">
            <span className="eyebrow">{sev.label}</span>
            <span className="text-pilot-muted">· {KIND_LABEL[i.kind]}</span>
            <span className="text-pilot-muted">· since {timeAgo(i.first_seen)}</span>
          </div>
          <h3 className="font-semibold text-pilot-text-primary mt-0.5">{i.title}</h3>
          <p className="text-sm text-pilot-text-secondary mt-1">{i.summary}</p>
        </div>
      </header>

      {path && (
        <div className="flex flex-wrap items-center gap-1.5 text-xs">
          <span className="text-pilot-muted mr-1">Failure path</span>
          {path.map((id, idx) => (
            <span key={id} className="inline-flex items-center gap-1.5">
              <button
                type="button"
                onClick={() => onShowNode(id)}
                className={`px-2 py-0.5 rounded-md border font-mono ${
                  id === i.root_cause_node ? "border-pilot-danger/60 text-pilot-danger bg-pilot-danger/10" : "border-pilot-border text-pilot-text-secondary"
                }`}
                title={id === i.root_cause_node ? "Likely origin" : "Affected caller"}
              >
                {nodeLabel(id)}
              </button>
              {idx < path.length - 1 && <ArrowRight className="w-3 h-3 text-pilot-muted" />}
            </span>
          ))}
        </div>
      )}

      <div className="grid grid-cols-2 gap-2">
        {i.signals.map((s) => (
          <div key={s.label} className="rounded-lg bg-pilot-surface-2/60 px-3 py-2">
            <div className="text-2xs text-pilot-muted">{s.label}</div>
            <div className="font-mono text-sm">
              <span className="text-pilot-text-primary font-semibold">{s.current}</span>
              <span className="text-pilot-muted"> vs {s.baseline}</span>
            </div>
          </div>
        ))}
      </div>

      {i.errors && i.errors.length > 0 && (
        <ul className="space-y-1">
          {i.errors.slice(0, 3).map((e) => (
            <li key={e} className="font-mono text-xs text-pilot-danger/90 bg-pilot-danger/[0.05] rounded px-2 py-1 break-all">
              {e}
            </li>
          ))}
        </ul>
      )}

      <div className="rounded-lg border border-pilot-accent/25 bg-pilot-accent/[0.05] p-3 space-y-2">
        <div className="flex items-center gap-2 text-xs">
          <Bot className="w-4 h-4 text-pilot-accent" />
          <span className="font-semibold text-pilot-text-primary">AI analysis</span>
          {ai.status === "done" && ai.confidence !== undefined && ai.confidence > 0 && (
            <span className="text-pilot-muted">· {Math.round(ai.confidence * 100)}% confidence</span>
          )}
          {ai.model && <span className="text-pilot-muted">· {ai.model}</span>}
          <button
            type="button"
            onClick={() => reanalyze.mutate()}
            disabled={thinking || reanalyze.isPending}
            className="ml-auto inline-flex items-center gap-1 text-pilot-muted hover:text-pilot-text-primary disabled:opacity-40"
            title="Ask the AI again with the latest evidence"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${reanalyze.isPending ? "animate-spin" : ""}`} />
            Re-analyse
          </button>
        </div>
        {thinking && (
          <p className="text-sm text-pilot-muted flex items-center gap-2">
            <Loader2 className="w-4 h-4 animate-spin" /> Correlating traces, logs and pod state…
          </p>
        )}
        {ai.status === "failed" && (
          <p className="text-sm text-pilot-warning">
            AI attempt {ai.attempts ?? 1} failed ({ai.error}).{" "}
            {(ai.attempts ?? 1) < 3 ? "Retrying automatically — the model may be busy with other analyses." : "The heuristic summary above still applies."}
          </p>
        )}
        {ai.status === "unavailable" && <p className="text-sm text-pilot-muted">No AI engine configured — showing heuristic analysis only.</p>}
        {ai.status === "done" && (
          <div className="space-y-2 text-sm">
            {ai.summary && <p className="text-pilot-text-primary">{ai.summary}</p>}
            {ai.root_cause && (
              <p>
                <span className="text-pilot-muted">Root cause: </span>
                <span className="text-pilot-text-secondary">{ai.root_cause}</span>
              </p>
            )}
            {ai.impact && (
              <p>
                <span className="text-pilot-muted">Impact: </span>
                <span className="text-pilot-text-secondary">{ai.impact}</span>
              </p>
            )}
            {ai.next_steps && ai.next_steps.length > 0 && (
              <ol className="list-decimal pl-5 space-y-0.5 text-pilot-text-secondary">
                {ai.next_steps.map((s) => (
                  <li key={s}>{s}</li>
                ))}
              </ol>
            )}
            {ai.commands && ai.commands.length > 0 && (
              <div className="space-y-1">
                {ai.commands.map((c) => (
                  <CommandLine key={c} cmd={c} />
                ))}
              </div>
            )}
          </div>
        )}
      </div>

      <footer className="flex flex-wrap gap-2">
        <button
          type="button"
          onClick={() => onShowNode(i.root_cause_node || i.node_id)}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium border border-pilot-border bg-pilot-surface hover:border-pilot-accent/50"
        >
          <MapPin className="w-3.5 h-3.5" /> Show on map
        </button>
        {i.trace_ids && i.trace_ids.length > 0 && (
          <button
            type="button"
            onClick={() => onShowTrace(i.trace_ids![0])}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium border border-pilot-border bg-pilot-surface hover:border-pilot-accent/50"
          >
            <ScrollText className="w-3.5 h-3.5" /> Open failing trace
          </button>
        )}
        {i.namespace && i.pods && i.pods.length > 0 && (
          <a
            href={`/dashboard?pod=${encodeURIComponent(`${i.namespace}/${i.pods[0].name}`)}`}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium border border-pilot-border bg-pilot-surface hover:border-pilot-accent/50"
          >
            <Bot className="w-3.5 h-3.5" /> Pod RCA
          </a>
        )}
      </footer>
    </article>
  );
}

function CommandLine({ cmd }: { cmd: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <div className="flex items-center gap-2 rounded-md bg-pilot-surface-2 px-2 py-1 font-mono text-xs">
      <span className="text-pilot-muted">$</span>
      <code className="flex-1 break-all text-pilot-text-primary">{cmd}</code>
      <button
        type="button"
        aria-label="Copy command"
        onClick={() => {
          navigator.clipboard?.writeText(cmd).then(() => {
            setCopied(true);
            setTimeout(() => setCopied(false), 1200);
          });
        }}
        className="text-pilot-muted hover:text-pilot-text-primary"
      >
        {copied ? <CheckCircle2 className="w-3.5 h-3.5 text-pilot-success" /> : <Copy className="w-3.5 h-3.5" />}
      </button>
    </div>
  );
}
