/**
 * DebugGuide — "this pod is broken, now what?"
 *
 * Two layers, in this order on purpose:
 *
 *  1. A deterministic checklist derived from the pod's actual state
 *     (lib/k8sExplain debugSteps). It needs no AI, no network beyond the pod
 *     itself, and it never invents a cause. Each step jumps to the tab that
 *     answers it, and carries the kubectl equivalent so an engineer who wants
 *     the terminal can copy it and someone learning can see what the UI just
 *     did for them.
 *
 *  2. An optional AI analysis via the configured Ollama model. It is opt-in
 *     per pod — a local 3B model takes seconds and a cluster-wide auto-analyse
 *     would hammer it — and when the model is unreachable the panel says
 *     exactly that and stays useful, because layer 1 does not depend on it.
 */
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Sparkles,
  Terminal,
  Check,
  Copy,
  ArrowRight,
  Loader2,
  AlertTriangle,
  ScrollText,
  CalendarClock,
  Boxes,
  FileCode,
} from "lucide-react";
import { getAIHealth, troubleshootPod, type SuggestedAction, type PodSummary } from "@/lib/api";
import { debugSteps, explainPod, type DebugTab, type DebugStep } from "@/lib/k8sExplain";
import { StatusPill } from "@/components/ui/StatusPill";
import { cn } from "@/lib/utils";

const TAB_META: Record<DebugTab, { label: string; icon: typeof ScrollText }> = {
  logs: { label: "Logs", icon: ScrollText },
  events: { label: "Events", icon: CalendarClock },
  containers: { label: "Containers", icon: Boxes },
  yaml: { label: "Config", icon: FileCode },
};

interface Props {
  namespace: string;
  pod: string;
  /** Pod state, so the guide matches the status badge exactly. */
  summary: Pick<PodSummary, "Phase" | "Reason" | "Ready" | "Restarts">;
  /** Jump the drawer to another tab. */
  onGoToTab: (tab: DebugTab) => void;
  onAuthorizeAction?: (action: SuggestedAction) => void;
}

export function DebugGuide({ namespace, pod, summary, onGoToTab, onAuthorizeAction }: Props) {
  const explanation = explainPod(summary);
  const steps = debugSteps(summary);
  const [done, setDone] = useState<Set<number>>(new Set());
  const [askAI, setAskAI] = useState(false);

  const toggle = (i: number) =>
    setDone((prev) => {
      const next = new Set(prev);
      if (next.has(i)) next.delete(i);
      else next.add(i);
      return next;
    });

  return (
    <div className="space-y-5">
      {/* What is wrong, in one sentence. */}
      <div
        className={cn(
          "rounded-xl border p-4",
          explanation.tone === "bad"
            ? "border-pilot-danger/30 bg-pilot-danger/10"
            : explanation.tone === "warn"
            ? "border-pilot-warning/30 bg-pilot-warning/10"
            : "border-pilot-border bg-pilot-surface"
        )}
      >
        <div className="mb-2 flex flex-wrap items-center gap-2">
          <StatusPill explanation={explanation} raw={summary.Reason || summary.Phase} hideWhy />
        </div>
        <p className="text-sm leading-relaxed text-pilot-text-primary">{explanation.plain}</p>
        {explanation.advice && (
          <p className="mt-1.5 text-sm leading-relaxed text-pilot-text-secondary">{explanation.advice}</p>
        )}
      </div>

      {/* The checklist. */}
      <section>
        <div className="mb-3 flex items-baseline justify-between gap-3">
          <h4 className="font-display text-sm font-bold text-pilot-text-primary">
            How to work out what happened
          </h4>
          <span className="text-xs tabular-nums text-pilot-muted">
            {done.size} of {steps.length} checked
          </span>
        </div>

        <ol className="space-y-2">
          {steps.map((step, i) => (
            <StepRow
              key={i}
              index={i}
              step={step}
              namespace={namespace}
              pod={pod}
              checked={done.has(i)}
              onToggle={() => toggle(i)}
              onGoToTab={onGoToTab}
            />
          ))}
        </ol>
      </section>

      {/* Optional AI layer. */}
      <AIAnalysis
        namespace={namespace}
        pod={pod}
        enabled={askAI}
        onEnable={() => setAskAI(true)}
        onAuthorizeAction={onAuthorizeAction}
      />
    </div>
  );
}

function StepRow({
  index,
  step,
  namespace,
  pod,
  checked,
  onToggle,
  onGoToTab,
}: {
  index: number;
  step: DebugStep;
  namespace: string;
  pod: string;
  checked: boolean;
  onToggle: () => void;
  onGoToTab: (tab: DebugTab) => void;
}) {
  const [copied, setCopied] = useState(false);
  const command = step.command?.replace(/\{ns\}/g, namespace).replace(/\{pod\}/g, pod);

  const copy = async () => {
    if (!command) return;
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
      setTimeout(() => setCopied(false), 1800);
    } catch {
      /* clipboard blocked (insecure origin, or denied) — the text is on screen
         and selectable either way, so there is nothing to recover from */
    }
  };

  const TabIcon = step.tab ? TAB_META[step.tab].icon : null;

  return (
    <li
      className={cn(
        "rounded-xl border bg-pilot-surface p-3.5 transition-colors",
        checked ? "border-pilot-border opacity-60" : "border-pilot-border"
      )}
    >
      <div className="flex items-start gap-3">
        <button
          type="button"
          onClick={onToggle}
          role="checkbox"
          aria-checked={checked}
          aria-label={`Mark step ${index + 1} as checked`}
          className={cn(
            "mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-md border text-xs font-bold transition-colors",
            checked
              ? "border-pilot-success bg-pilot-success/15 text-pilot-success"
              : "border-pilot-border text-pilot-muted hover:border-pilot-accent/60"
          )}
        >
          {checked ? <Check className="h-3.5 w-3.5" aria-hidden="true" /> : index + 1}
        </button>

        <div className="min-w-0 flex-1">
          <p className={cn("text-sm font-semibold text-pilot-text-primary", checked && "line-through")}>
            {step.title}
          </p>
          <p className="mt-1 text-sm leading-relaxed text-pilot-text-secondary">{step.detail}</p>

          <div className="mt-2.5 flex flex-wrap items-center gap-2">
            {step.tab && TabIcon && (
              <button
                type="button"
                onClick={() => onGoToTab(step.tab!)}
                className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-pilot-accent/12 px-2.5 text-xs font-semibold text-pilot-accent-light transition-colors hover:bg-pilot-accent/20"
              >
                <TabIcon className="h-3.5 w-3.5" aria-hidden="true" />
                Open {TAB_META[step.tab].label}
                <ArrowRight className="h-3 w-3" aria-hidden="true" />
              </button>
            )}

            {command && (
              <button
                type="button"
                onClick={copy}
                title="Copy the kubectl equivalent"
                className="inline-flex h-8 min-w-0 max-w-full items-center gap-1.5 rounded-lg border border-pilot-border bg-pilot-surface-2 px-2.5 font-mono text-xs text-pilot-text-secondary transition-colors hover:border-pilot-border-hover"
              >
                <Terminal className="h-3.5 w-3.5 shrink-0 text-pilot-muted" aria-hidden="true" />
                <span className="truncate">{command}</span>
                {copied ? (
                  <Check className="h-3.5 w-3.5 shrink-0 text-pilot-success" aria-hidden="true" />
                ) : (
                  <Copy className="h-3.5 w-3.5 shrink-0 text-pilot-muted" aria-hidden="true" />
                )}
                <span className="sr-only">{copied ? "Copied" : "Copy command"}</span>
              </button>
            )}
          </div>
        </div>
      </div>
    </li>
  );
}

/**
 * The AI half. Opt-in per pod: a local 3B model takes several seconds per
 * request, so analysing every pod the moment a drawer opens would make the
 * model the bottleneck for simply browsing.
 */
function AIAnalysis({
  namespace,
  pod,
  enabled,
  onEnable,
  onAuthorizeAction,
}: {
  namespace: string;
  pod: string;
  enabled: boolean;
  onEnable: () => void;
  onAuthorizeAction?: (action: SuggestedAction) => void;
}) {
  // Cheap and cached — tells us up front whether asking is even possible.
  const { data: health } = useQuery({
    queryKey: ["ai-health"],
    queryFn: getAIHealth,
    staleTime: 60_000,
    refetchInterval: false,
    retry: false,
  });

  const {
    data: report,
    isFetching,
    error,
  } = useQuery({
    queryKey: ["ai-troubleshoot", namespace, pod],
    queryFn: () => troubleshootPod(namespace, pod),
    enabled,
    refetchInterval: false,
    staleTime: 5 * 60_000,
    retry: false,
  });

  const unavailable = health && !health.healthy;

  return (
    <section className="rounded-xl border border-pilot-border bg-pilot-surface p-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Sparkles className="h-4 w-4 text-pilot-accent" aria-hidden="true" />
          <h4 className="font-display text-sm font-bold text-pilot-text-primary">Ask the AI</h4>
          {health?.model && (
            <span className="font-mono text-xs text-pilot-muted">{health.model}</span>
          )}
        </div>

        {!enabled && !unavailable && (
          <button
            type="button"
            onClick={onEnable}
            className="inline-flex h-9 items-center gap-2 rounded-lg bg-pilot-accent/12 px-3 text-sm font-semibold text-pilot-accent-light transition-colors hover:bg-pilot-accent/20"
          >
            <Sparkles className="h-4 w-4" aria-hidden="true" />
            Analyse this pod
          </button>
        )}
      </div>

      {/* Say plainly why the button is missing, rather than failing on click. */}
      {unavailable ? (
        <div className="mt-3 flex items-start gap-2.5 rounded-lg border border-pilot-border bg-pilot-surface-2 p-3">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-pilot-warning" aria-hidden="true" />
          <div className="min-w-0 text-sm">
            <p className="font-medium text-pilot-text-secondary">
              The AI model is not reachable, so analysis is unavailable.
            </p>
            <p className="mt-1 text-xs text-pilot-muted">
              KubePilot is configured to use{" "}
              <span className="font-mono">{health?.model || "a model"}</span> at{" "}
              <span className="font-mono break-all">{health?.base_url}</span>. The checklist above
              does not need it.
            </p>
          </div>
        </div>
      ) : !enabled ? (
        <p className="mt-2 text-sm text-pilot-muted">
          Sends this pod&rsquo;s state, recent events and log tail to the model and asks for a
          probable cause. It reads the cluster; it changes nothing on its own.
        </p>
      ) : isFetching ? (
        <div className="mt-3 flex items-center gap-3 py-4 text-sm text-pilot-muted">
          <Loader2 className="h-4 w-4 animate-spin text-pilot-accent" aria-hidden="true" />
          Reading the pod and asking the model&hellip;
        </div>
      ) : error ? (
        <div className="mt-3 rounded-lg border border-pilot-danger/30 bg-pilot-danger/10 p-3 text-sm">
          <p className="font-medium text-pilot-danger">The analysis did not complete.</p>
          <p className="mt-1 break-words text-xs text-pilot-text-secondary">
            {error instanceof Error ? error.message : String(error)}
          </p>
        </div>
      ) : report ? (
        <div className="mt-3 space-y-3">
          <div>
            <p className="eyebrow mb-1">Probable cause</p>
            <p className="text-sm font-semibold leading-relaxed text-pilot-text-primary">
              {report.RootCause || "The model did not identify a specific cause."}
            </p>
          </div>
          {report.Analysis && (
            <div>
              <p className="eyebrow mb-1">Reasoning</p>
              <p className="whitespace-pre-line text-sm leading-relaxed text-pilot-text-secondary">
                {report.Analysis}
              </p>
            </div>
          )}
          {report.Actions?.length > 0 && (
            <div>
              <p className="eyebrow mb-2">Suggested next steps</p>
              <div className="space-y-2">
                {report.Actions.map((action, i) => (
                  <div key={i} className="rounded-lg border border-pilot-border bg-pilot-surface-2 p-3">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <span className="text-xs font-bold uppercase tracking-wide text-pilot-accent">
                        {action.type.replace(/_/g, " ")}
                      </span>
                      {onAuthorizeAction && action.type !== "investigate" && action.type !== "noop" && (
                        <button
                          type="button"
                          onClick={() => onAuthorizeAction(action)}
                          className="h-8 rounded-lg border border-pilot-border px-2.5 text-xs font-semibold text-pilot-text-secondary transition-colors hover:border-pilot-accent/60 hover:text-pilot-text-primary"
                        >
                          Review &amp; authorise
                        </button>
                      )}
                    </div>
                    <p className="mt-1.5 text-sm leading-relaxed text-pilot-text-secondary">
                      {action.explanation}
                    </p>
                  </div>
                ))}
              </div>
            </div>
          )}
          <p className="border-t border-pilot-border pt-2.5 text-xs text-pilot-muted">
            Generated by a language model from this pod&rsquo;s state. Treat it as a lead to verify
            with the checklist above, not as a verdict.
          </p>
        </div>
      ) : null}
    </section>
  );
}
