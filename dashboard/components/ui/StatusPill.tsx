/**
 * StatusPill — one status surface for the whole dashboard.
 *
 * Three rules it exists to enforce:
 *  1. Never colour alone. Every pill carries an icon and a word, so it reads
 *     correctly in greyscale and to a colour-blind viewer.
 *  2. Plain English by default. The pill shows "Crash looping", not
 *     "CrashLoopBackOff"; the raw Kubernetes wording is still available under
 *     the "why" affordance for anyone who wants it.
 *  3. The explanation is reachable by keyboard and by touch, not hover-only —
 *     it hangs off a real <button>, not a title attribute.
 */
import * as React from "react";
import { CheckCircle2, AlertTriangle, XCircle, Loader2, CircleDashed, HelpCircle } from "lucide-react";
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { TONE_CLASS, type Explanation, type Tone } from "@/lib/k8sExplain";
import { cn } from "@/lib/utils";

const TONE_ICON: Record<Tone, typeof CheckCircle2> = {
  ok: CheckCircle2,
  warn: AlertTriangle,
  bad: XCircle,
  info: Loader2,
  idle: CircleDashed,
};

interface Props {
  explanation: Explanation;
  /** The raw Kubernetes wording, shown alongside the plain text in the tooltip. */
  raw?: string;
  /** Force-hide the "?" affordance — for dense cells where space is tight. */
  hideWhy?: boolean;
  className?: string;
}

export function StatusPill({ explanation, raw, hideWhy, className }: Props) {
  const Icon = TONE_ICON[explanation.tone];
  const tone = TONE_CLASS[explanation.tone];
  // A healthy "Running" explains itself. Offering a "?" on every green row is
  // noise on a 700-row table and trains people to ignore the one that matters.
  const showWhy = !hideWhy && (explanation.tone !== "ok" || Boolean(explanation.advice));

  return (
    <span className="inline-flex items-center gap-1.5">
      <span
        className={cn(
          "inline-flex items-center gap-1.5 rounded-md border px-2 py-1 text-xs font-semibold whitespace-nowrap",
          tone.pill,
          className
        )}
      >
        <Icon
          className={cn("h-3.5 w-3.5 shrink-0", explanation.tone === "info" && "animate-spin [animation-duration:2s]")}
          aria-hidden="true"
        />
        {explanation.label}
      </span>

      {showWhy && (
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                // Stop the click reaching a clickable table row — asking what a
                // status means should not also open the detail drawer.
                onClick={(e) => e.stopPropagation()}
                className="inline-flex h-6 w-6 items-center justify-center rounded-md text-pilot-muted transition-colors hover:bg-pilot-surface-2 hover:text-pilot-text-secondary"
                aria-label={`What does "${explanation.label}" mean?`}
              >
                <HelpCircle className="h-3.5 w-3.5" aria-hidden="true" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="top" className="max-w-xs space-y-1.5 px-3 py-2.5">
              <p className="text-sm font-semibold text-pilot-text-primary">{explanation.label}</p>
              <p className="text-xs leading-relaxed text-pilot-text-secondary">{explanation.plain}</p>
              {explanation.advice && (
                <p className="text-xs leading-relaxed text-pilot-accent-light">{explanation.advice}</p>
              )}
              {raw && raw !== explanation.label && (
                <p className="border-t border-pilot-border pt-1.5 font-mono text-[0.7rem] text-pilot-muted">
                  Kubernetes says: {raw}
                </p>
              )}
            </TooltipContent>
          </Tooltip>
      )}
    </span>
  );
}

/**
 * A compact one-line hint shown above a resource list — what this kind of
 * thing actually is. Cheap to read, easy to ignore once you know.
 */
export function KindHint({ title, plain }: { title: string; plain: string }) {
  return (
    <p className="mb-4 max-w-3xl text-sm leading-relaxed text-pilot-muted">
      <span className="font-semibold text-pilot-text-secondary">{title}:</span> {plain}
    </p>
  );
}
