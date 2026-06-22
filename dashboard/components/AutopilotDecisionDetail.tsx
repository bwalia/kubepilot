/**
 * AutopilotDecisionDetail — right-side slide-over opened when an operator clicks
 * an Autopilot decision row. Built for ops: it leads with the live pod logs so
 * you can immediately see "is it actually working?", then the action Autopilot
 * took, then the full AI root-cause analysis behind the decision.
 */
import { useQuery } from "@tanstack/react-query";
import {
  RefreshCw,
  ScrollText,
  Bot,
  Stethoscope,
  X,
  CheckCircle2,
  XCircle,
  SkipForward,
  Eye,
  UserCog,
  AlertTriangle,
  Boxes,
} from "lucide-react";
import { Dialog, DrawerContent, DialogTitle } from "@/components/ui/dialog";
import { LogViewer } from "@/components/LogViewer";
import {
  getRCAReport,
  getPodLogs,
  type AutopilotDecision,
  type AutopilotVerdict,
} from "@/lib/api";

const VERDICT_META: Record<
  AutopilotVerdict,
  { label: string; badge: string; Icon: typeof CheckCircle2 }
> = {
  executed: { label: "Executed", badge: "bg-emerald-900/30 text-emerald-400 border-emerald-700/50", Icon: CheckCircle2 },
  "dry-run": { label: "Dry-run", badge: "bg-sky-900/30 text-sky-400 border-sky-700/50", Icon: Eye },
  skipped: { label: "Skipped", badge: "bg-gray-800/50 text-gray-400 border-gray-700/50", Icon: SkipForward },
  escalated: { label: "Escalated", badge: "bg-amber-900/30 text-amber-400 border-amber-700/50", Icon: UserCog },
  failed: { label: "Failed", badge: "bg-red-900/30 text-red-400 border-red-700/50", Icon: XCircle },
};

const RISK_BADGE: Record<string, string> = {
  safe: "bg-emerald-900/30 text-emerald-400 border-emerald-700/50",
  moderate: "bg-amber-900/30 text-amber-400 border-amber-700/50",
  high: "bg-red-900/30 text-red-400 border-red-700/50",
};

const LOG_REFRESH_MS = 5000;

export function AutopilotDecisionDetail({
  decision,
  onClose,
}: {
  decision: AutopilotDecision | null;
  onClose: () => void;
}) {
  const open = !!decision;
  const isPod = (decision?.resource.kind ?? "").toLowerCase() === "pod";
  const ns = decision?.resource.namespace ?? "";
  const name = decision?.resource.name ?? "";

  const rca = useQuery({
    queryKey: ["rca-report", decision?.report_id],
    queryFn: () => getRCAReport(decision!.report_id),
    enabled: open && !!decision?.report_id,
  });

  // Live logs: poll while the drawer is open so ops sees the pod's current state.
  const logs = useQuery({
    queryKey: ["pod-logs", ns, name],
    queryFn: () => getPodLogs(ns, name, "", 300),
    enabled: open && isPod && !!ns && !!name,
    refetchInterval: LOG_REFRESH_MS,
    refetchOnWindowFocus: true,
  });

  const meta = decision ? VERDICT_META[decision.verdict] ?? VERDICT_META.skipped : VERDICT_META.skipped;
  const confidence = decision ? Math.round((decision.confidence ?? 0) * 100) : 0;

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DrawerContent className="max-w-3xl">
        {decision && (
          <>
            {/* ── Sticky header ─────────────────────────────────────── */}
            <div className="sticky top-0 z-10 bg-pilot-bg border-b border-pilot-border px-5 py-4">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <span className={`flex items-center gap-1 text-xs font-bold uppercase px-2 py-0.5 rounded-md border ${meta.badge}`}>
                      <meta.Icon className="w-3.5 h-3.5" /> {meta.label}
                    </span>
                    {decision.action ? (
                      <span className="text-xs font-mono px-2 py-0.5 rounded-md bg-pilot-surface border border-pilot-border">
                        {decision.action}
                      </span>
                    ) : null}
                  </div>
                  <DialogTitle className="mt-2 flex items-center gap-2 truncate">
                    <Boxes className="w-4 h-4 text-pilot-accent shrink-0" />
                    <span className="font-mono text-base truncate">{ns}/{name}</span>
                  </DialogTitle>
                  <p className="text-xs text-pilot-muted mt-1">
                    {decision.resource.kind} · severity {decision.severity} · {new Date(decision.time).toLocaleString()}
                  </p>
                </div>
                <button
                  onClick={onClose}
                  className="shrink-0 text-pilot-muted hover:text-white p-1 rounded-md hover:bg-pilot-surface"
                  aria-label="Close"
                >
                  <X className="w-5 h-5" />
                </button>
              </div>
            </div>

            {/* ── Scrollable body ───────────────────────────────────── */}
            <div className="flex-1 overflow-y-auto px-5 py-5 space-y-6">
              {/* What Autopilot did */}
              <section>
                <h3 className="flex items-center gap-1.5 text-sm font-bold text-white mb-3">
                  <Bot className="w-4 h-4 text-pilot-accent" /> What Autopilot did
                </h3>
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-3">
                  <Stat label="Verdict" value={meta.label} />
                  <Stat label="Action" value={decision.action || "—"} mono />
                  <Stat label="Confidence" value={`${confidence}%`} />
                  <Stat label="Root cause" value={decision.root_cause || "—"} />
                </div>
                <div className="bg-pilot-surface border border-pilot-border rounded-lg p-3">
                  <div className="text-xs text-pilot-muted mb-1">Reason</div>
                  <p className="text-sm text-white">{decision.reason}</p>
                  {decision.output ? (
                    <>
                      <div className="text-xs text-pilot-muted mt-3 mb-1">Action output</div>
                      <pre className="text-xs font-mono text-emerald-300/90 whitespace-pre-wrap break-all bg-pilot-bg rounded-md p-2 border border-pilot-border">
                        {decision.output}
                      </pre>
                    </>
                  ) : null}
                </div>
              </section>

              {/* Live pod logs — the prominent, ops-facing section */}
              {isPod && (
                <section>
                  <div className="flex items-center justify-between mb-2">
                    <h3 className="flex items-center gap-1.5 text-sm font-bold text-white">
                      <ScrollText className="w-4 h-4 text-pilot-accent" /> Live pod logs
                      <span className="flex items-center gap-1 ml-2 text-[10px] font-bold uppercase text-emerald-400">
                        <span className="relative flex h-2 w-2">
                          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
                          <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500" />
                        </span>
                        Live · {LOG_REFRESH_MS / 1000}s
                      </span>
                    </h3>
                    <button
                      onClick={() => logs.refetch()}
                      className="flex items-center gap-1 text-xs text-pilot-muted hover:text-white px-2 py-1 rounded-md border border-pilot-border hover:bg-pilot-surface"
                    >
                      <RefreshCw className={`w-3.5 h-3.5 ${logs.isFetching ? "animate-spin" : ""}`} /> Refresh
                    </button>
                  </div>
                  {logs.isLoading ? (
                    <div className="text-pilot-muted text-sm py-8 text-center bg-pilot-surface border border-pilot-border rounded-lg">
                      Loading logs…
                    </div>
                  ) : logs.isError ? (
                    <div className="flex items-center gap-2 text-amber-400 text-sm py-6 px-3 bg-amber-900/10 border border-amber-700/40 rounded-lg">
                      <AlertTriangle className="w-4 h-4 shrink-0" />
                      Could not fetch logs (pod may have been recreated or has no logs yet). Retrying every {LOG_REFRESH_MS / 1000}s.
                    </div>
                  ) : (logs.data ?? "").trim() === "" ? (
                    <div className="text-pilot-muted text-sm py-8 text-center bg-pilot-surface border border-pilot-border rounded-lg">
                      No log output.
                    </div>
                  ) : (
                    <LogViewer title={`${ns}/${name} — last 300 lines`} content={logs.data ?? ""} maxHeight="420px" />
                  )}
                </section>
              )}

              {/* AI Root Cause Analysis */}
              <section>
                <h3 className="flex items-center gap-1.5 text-sm font-bold text-white mb-3">
                  <Stethoscope className="w-4 h-4 text-pilot-accent" /> AI root cause analysis
                </h3>
                {rca.isLoading ? (
                  <div className="text-pilot-muted text-sm py-8 text-center bg-pilot-surface border border-pilot-border rounded-lg">
                    Loading RCA report…
                  </div>
                ) : rca.isError || !rca.data ? (
                  <div className="text-pilot-muted text-sm py-6 px-3 bg-pilot-surface border border-pilot-border rounded-lg">
                    RCA report <span className="font-mono">{decision.report_id}</span> is no longer available
                    (the in-memory store keeps only recent reports).
                  </div>
                ) : (
                  <div className="space-y-4">
                    <div className="bg-pilot-surface border border-pilot-border rounded-lg p-3">
                      <div className="flex items-center gap-2 mb-1">
                        <span className="text-xs font-bold uppercase px-2 py-0.5 rounded-md bg-pilot-bg border border-pilot-border">
                          {rca.data.root_cause?.category}
                        </span>
                        <span className="text-xs text-pilot-muted">{Math.round((rca.data.confidence ?? 0) * 100)}% confidence</span>
                      </div>
                      <p className="text-sm text-white font-medium">{rca.data.root_cause?.summary}</p>
                      {rca.data.root_cause?.detail ? (
                        <p className="text-sm text-pilot-muted mt-1.5 whitespace-pre-wrap">{rca.data.root_cause.detail}</p>
                      ) : null}
                      {rca.data.root_cause?.affected_components?.length ? (
                        <div className="flex flex-wrap gap-1.5 mt-2">
                          {rca.data.root_cause.affected_components.map((c) => (
                            <span key={c} className="text-xs font-mono px-1.5 py-0.5 rounded bg-pilot-bg border border-pilot-border text-pilot-muted">{c}</span>
                          ))}
                        </div>
                      ) : null}
                    </div>

                    {rca.data.evidence_chain?.length ? (
                      <div>
                        <div className="text-xs text-pilot-muted mb-1.5">Evidence</div>
                        <div className="space-y-2">
                          {rca.data.evidence_chain.map((e, i) => (
                            <div key={i} className="bg-pilot-surface border border-pilot-border rounded-lg p-3">
                              <div className="text-xs font-bold text-pilot-accent uppercase mb-1">{e.source}</div>
                              <pre className="text-xs font-mono text-pilot-muted whitespace-pre-wrap break-all">{e.data}</pre>
                              {e.relevance ? <p className="text-xs text-pilot-muted/80 mt-1 italic">{e.relevance}</p> : null}
                            </div>
                          ))}
                        </div>
                      </div>
                    ) : null}

                    {rca.data.remediation?.length ? (
                      <div>
                        <div className="text-xs text-pilot-muted mb-1.5">Remediation steps</div>
                        <div className="space-y-2">
                          {rca.data.remediation.map((s, i) => (
                            <div key={i} className="bg-pilot-surface border border-pilot-border rounded-lg p-3">
                              <div className="flex items-center gap-2 flex-wrap">
                                <span className="text-xs font-mono px-1.5 py-0.5 rounded bg-pilot-bg border border-pilot-border">#{s.order} {s.action}</span>
                                <span className={`text-[10px] font-bold uppercase px-1.5 py-0.5 rounded border ${RISK_BADGE[s.risk?.toLowerCase()] || RISK_BADGE.moderate}`}>{s.risk}</span>
                                {s.auto_apply ? <span className="text-[10px] font-bold uppercase px-1.5 py-0.5 rounded border border-emerald-700/50 text-emerald-400">auto-apply</span> : null}
                                {s.requires_cr ? <span className="text-[10px] font-bold uppercase px-1.5 py-0.5 rounded border border-amber-700/50 text-amber-400">needs CR code</span> : null}
                              </div>
                              <p className="text-sm text-white mt-1.5">{s.description}</p>
                              {s.command ? (
                                <pre className="text-xs font-mono text-sky-300/90 whitespace-pre-wrap break-all bg-pilot-bg rounded-md p-2 border border-pilot-border mt-1.5">{s.command}</pre>
                              ) : null}
                            </div>
                          ))}
                        </div>
                      </div>
                    ) : null}
                  </div>
                )}
              </section>
            </div>
          </>
        )}
      </DrawerContent>
    </Dialog>
  );
}

function Stat({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="bg-pilot-surface border border-pilot-border rounded-lg p-2.5">
      <div className="text-xs text-pilot-muted">{label}</div>
      <div className={`text-sm font-semibold text-white mt-0.5 truncate ${mono ? "font-mono" : ""}`}>{value}</div>
    </div>
  );
}
