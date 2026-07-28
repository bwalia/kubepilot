import { useState } from "react";
import { Copy, Check, Terminal } from "lucide-react";
import type { RCAReport, TroubleshootReport, TroubleshootingInsight } from "@/lib/api";
import {
  formatReportText,
  buildFixPrompt,
  formatRCAText,
  buildRCAFixPrompt,
  formatInsightText,
  buildInsightFixPrompt,
  copyText,
} from "@/lib/aiPrompt";

type CopyKind = "output" | "prompt";

/**
 * Universal copy actions for any AI analysis surface:
 *  - Copy output: the raw analysis text.
 *  - Copy Prompt: a ready-to-run prompt to paste into a terminal LLM (Claude)
 *    to analyse further or produce a concrete fix.
 */
export function CopyPromptActions({
  outputText,
  promptText,
  className = "",
}: {
  outputText: string;
  promptText: string;
  className?: string;
}) {
  const [copied, setCopied] = useState<CopyKind | null>(null);

  const doCopy = async (kind: CopyKind) => {
    const text = kind === "output" ? outputText : promptText;
    if (await copyText(text)) {
      setCopied(kind);
      setTimeout(() => setCopied((c) => (c === kind ? null : c)), 1800);
    }
  };

  const base =
    "inline-flex items-center gap-1.5 text-xs font-semibold px-3 py-2 rounded-lg border transition-colors";

  return (
    <div className={`flex flex-wrap gap-2 ${className}`.trim()}>
      <button
        type="button"
        onClick={() => doCopy("output")}
        className={`${base} bg-pilot-surface border-pilot-border text-pilot-text-secondary hover:text-pilot-text-primary hover:border-pilot-border-hover`}
        title="Copy the analysis text"
      >
        {copied === "output" ? <Check className="w-3.5 h-3.5 text-pilot-success" /> : <Copy className="w-3.5 h-3.5" />}
        {copied === "output" ? "Copied" : "Copy output"}
      </button>
      <button
        type="button"
        onClick={() => doCopy("prompt")}
        className={`${base} bg-pilot-accent/15 border-pilot-accent/30 text-pilot-accent hover:bg-pilot-accent/25`}
        title="Copy a prompt to paste into Claude / an LLM to fix this"
      >
        {copied === "prompt" ? <Check className="w-3.5 h-3.5" /> : <Terminal className="w-3.5 h-3.5" />}
        {copied === "prompt" ? "Copied prompt" : "Copy Prompt"}
      </button>
    </div>
  );
}

/** Convenience wrapper for pod TroubleshootReport surfaces. */
export function AIReportActions({ report }: { report: TroubleshootReport }) {
  return (
    <CopyPromptActions
      outputText={formatReportText(report)}
      promptText={buildFixPrompt(report)}
    />
  );
}

/** Convenience wrapper for RCA report surfaces. */
export function RCAReportActions({ report }: { report: RCAReport }) {
  return (
    <CopyPromptActions
      outputText={formatRCAText(report)}
      promptText={buildRCAFixPrompt(report)}
    />
  );
}

/** Convenience wrapper for cluster troubleshooting insights. */
export function InsightActions({ insight }: { insight: TroubleshootingInsight }) {
  return (
    <CopyPromptActions
      outputText={formatInsightText(insight)}
      promptText={buildInsightFixPrompt(insight)}
    />
  );
}
