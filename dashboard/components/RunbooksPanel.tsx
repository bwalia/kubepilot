/**
 * RunbooksPanel — Pre-built Kubernetes automation runbooks.
 * Left panel: list of runbooks with risk/category badges.
 * Right panel: selected runbook's steps with inline execute + result display.
 */
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { BookOpen, Play, CheckCircle2, XCircle, AlertCircle, ChevronRight, Loader2 } from "lucide-react";
import { listRunbooks, executeRunbookStep, type Runbook, type RunbookStepResult } from "@/lib/api";

const RISK_STYLES: Record<string, string> = {
  low: "bg-emerald-500/10 text-pilot-success border-emerald-500/30",
  medium: "bg-amber-500/10 text-pilot-warning border-amber-500/30",
  high: "bg-red-500/10 text-pilot-danger border-red-500/30",
};

const CATEGORY_ICONS: Record<string, string> = {
  diagnostic: "🔍",
  recovery: "🚑",
  rollback: "↩️",
  health: "❤️",
  migration: "📦",
};

export function RunbooksPanel() {
  const { data: runbooks = [], isLoading } = useQuery({
    queryKey: ["runbooks"],
    queryFn: listRunbooks,
  });

  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [stepResults, setStepResults] = useState<Record<number, RunbookStepResult>>({});
  const [executingStep, setExecutingStep] = useState<number | null>(null);
  const [params, setParams] = useState<{ namespace: string; pod: string; deployment: string }>({
    namespace: "",
    pod: "",
    deployment: "",
  });

  const selected = runbooks.find((r) => r.id === selectedId) || null;

  const handleSelect = (id: string) => {
    setSelectedId(id);
    setStepResults({});
  };

  const handleExecute = async (rb: Runbook, stepIdx: number) => {
    setExecutingStep(stepIdx);
    try {
      const resp = await executeRunbookStep(rb.id, stepIdx, {
        namespace: params.namespace,
        pod: params.pod,
        deployment: params.deployment,
      });
      setStepResults((prev) => ({ ...prev, [stepIdx]: resp.result }));
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : "execution failed";
      setStepResults((prev) => ({
        ...prev,
        [stepIdx]: { status: "error", message },
      }));
    } finally {
      setExecutingStep(null);
    }
  };

  if (isLoading) {
    return <div className="text-pilot-muted text-sm py-10 text-center">Loading runbooks...</div>;
  }

  return (
    <div className="grid grid-cols-1 lg:grid-cols-[380px_1fr] gap-6">
      {/* ── Runbook List ───────────────────────────────────── */}
      <aside className="space-y-2">
        <h2 className="text-base font-bold flex items-center gap-2 mb-3">
          <BookOpen className="w-5 h-5 text-pilot-accent" />
          Runbooks
          <span className="text-xs text-pilot-muted font-normal">({runbooks.length})</span>
        </h2>
        {runbooks.map((rb) => {
          const riskClass = RISK_STYLES[rb.risk] ?? RISK_STYLES.low;
          const icon = CATEGORY_ICONS[rb.category] ?? "📋";
          const isActive = selectedId === rb.id;
          return (
            <button
              key={rb.id}
              onClick={() => handleSelect(rb.id)}
              className={`w-full text-left bg-pilot-surface border rounded-lg p-3 hover:border-pilot-accent/50 transition-colors ${
                isActive ? "border-pilot-accent shadow-glow-blue" : "border-pilot-border"
              }`}
            >
              <div className="flex items-start justify-between gap-2 mb-1">
                <div className="flex items-center gap-2 min-w-0">
                  <span className="text-lg leading-none">{icon}</span>
                  <span className="font-semibold text-sm text-white truncate">{rb.name}</span>
                </div>
                <span className={`text-2xs font-bold uppercase px-2 py-0.5 rounded-full border ${riskClass}`}>
                  {rb.risk}
                </span>
              </div>
              <p className="text-xs text-pilot-muted line-clamp-2 leading-relaxed mb-2">
                {rb.description}
              </p>
              <div className="flex items-center gap-3 text-2xs text-pilot-muted">
                <span className="uppercase tracking-wider">{rb.category}</span>
                <span>{rb.steps.length} steps</span>
              </div>
            </button>
          );
        })}
      </aside>

      {/* ── Runbook Detail ─────────────────────────────────── */}
      <section>
        {!selected ? (
          <div className="bg-pilot-surface border border-pilot-border rounded-xl p-10 text-center">
            <BookOpen className="w-12 h-12 text-pilot-muted mx-auto mb-3 opacity-50" />
            <p className="text-pilot-muted text-sm">Select a runbook to view steps and execute actions.</p>
          </div>
        ) : (
          <div className="bg-pilot-surface border border-pilot-border rounded-xl p-6 space-y-5">
            <div>
              <div className="flex items-center gap-3 mb-2">
                <span className="text-2xl">{CATEGORY_ICONS[selected.category] ?? "📋"}</span>
                <h3 className="text-xl font-bold">{selected.name}</h3>
                <span
                  className={`text-2xs font-bold uppercase px-2 py-0.5 rounded-full border ${
                    RISK_STYLES[selected.risk] ?? RISK_STYLES.low
                  }`}
                >
                  {selected.risk} risk
                </span>
              </div>
              <p className="text-sm text-pilot-text-secondary leading-relaxed">{selected.description}</p>
            </div>

            {/* Parameter inputs — used by actions that need namespace/pod/deployment */}
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 pt-2 border-t border-pilot-border">
              <div>
                <label className="text-2xs uppercase tracking-wider text-pilot-muted block mb-1">
                  Namespace
                </label>
                <input
                  type="text"
                  value={params.namespace}
                  onChange={(e) => setParams((p) => ({ ...p, namespace: e.target.value }))}
                  placeholder="e.g. dev"
                  className="w-full bg-pilot-bg border border-pilot-border rounded-md px-3 py-1.5 text-sm placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent"
                />
              </div>
              <div>
                <label className="text-2xs uppercase tracking-wider text-pilot-muted block mb-1">
                  Pod (for AI analyze)
                </label>
                <input
                  type="text"
                  value={params.pod}
                  onChange={(e) => setParams((p) => ({ ...p, pod: e.target.value }))}
                  placeholder="e.g. api-abc123"
                  className="w-full bg-pilot-bg border border-pilot-border rounded-md px-3 py-1.5 text-sm placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent"
                />
              </div>
              <div>
                <label className="text-2xs uppercase tracking-wider text-pilot-muted block mb-1">
                  Deployment (for restart)
                </label>
                <input
                  type="text"
                  value={params.deployment}
                  onChange={(e) => setParams((p) => ({ ...p, deployment: e.target.value }))}
                  placeholder="e.g. api"
                  className="w-full bg-pilot-bg border border-pilot-border rounded-md px-3 py-1.5 text-sm placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent"
                />
              </div>
            </div>

            {/* Steps */}
            <div className="space-y-2 pt-2">
              <h4 className="text-2xs uppercase tracking-wider text-pilot-muted font-medium">
                Steps
              </h4>
              {selected.steps.map((step, i) => {
                const result = stepResults[i];
                const action = selected.actions[i] || "manual";
                const isManual = action === "manual" || action === "";
                const isExecuting = executingStep === i;
                const borderClass = result?.status === "ok"
                  ? "border-pilot-success"
                  : result?.status === "error"
                  ? "border-pilot-danger"
                  : result?.status === "manual"
                  ? "border-pilot-warning"
                  : "border-pilot-border";

                return (
                  <div
                    key={i}
                    className={`bg-pilot-bg border rounded-lg p-3 ${borderClass}`}
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2 mb-1">
                          <span className="text-2xs font-mono text-pilot-muted">STEP {i + 1}</span>
                          {isManual && (
                            <span className="text-2xs text-pilot-warning bg-amber-500/10 px-1.5 py-0.5 rounded">
                              manual
                            </span>
                          )}
                          {!isManual && (
                            <span className="text-2xs text-pilot-accent bg-blue-500/10 px-1.5 py-0.5 rounded font-mono">
                              {action}
                            </span>
                          )}
                        </div>
                        <p className="text-sm text-white leading-relaxed">{step}</p>
                      </div>
                      <button
                        onClick={() => handleExecute(selected, i)}
                        disabled={isExecuting}
                        className="shrink-0 flex items-center gap-1.5 text-xs font-semibold px-3 py-1.5 rounded-md bg-pilot-accent hover:bg-blue-500 text-white disabled:opacity-50"
                      >
                        {isExecuting ? (
                          <Loader2 className="w-3.5 h-3.5 animate-spin" />
                        ) : (
                          <Play className="w-3.5 h-3.5" />
                        )}
                        {isExecuting ? "Running" : "Execute"}
                      </button>
                    </div>

                    {result && (
                      <div
                        className={`mt-3 flex items-start gap-2 text-xs p-2 rounded-md font-mono leading-relaxed ${
                          result.status === "ok"
                            ? "bg-emerald-500/10 text-pilot-success"
                            : result.status === "error"
                            ? "bg-red-500/10 text-pilot-danger"
                            : "bg-amber-500/10 text-pilot-warning"
                        }`}
                      >
                        {result.status === "ok" && <CheckCircle2 className="w-3.5 h-3.5 shrink-0 mt-0.5" />}
                        {result.status === "error" && <XCircle className="w-3.5 h-3.5 shrink-0 mt-0.5" />}
                        {result.status === "manual" && <AlertCircle className="w-3.5 h-3.5 shrink-0 mt-0.5" />}
                        <span className="break-words">{result.message}</span>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>

            <div className="flex items-center gap-2 text-2xs text-pilot-muted pt-2 border-t border-pilot-border">
              <ChevronRight className="w-3 h-3" />
              Runbook execution requires <code className="font-mono text-pilot-text-secondary">--enable-action-mutations</code> on the KubePilot server.
            </div>
          </div>
        )}
      </section>
    </div>
  );
}
