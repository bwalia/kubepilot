import { useState } from "react";
import { Copy, Check, Terminal } from "lucide-react";
import type { TroubleshootReport } from "@/lib/api";
import { formatReportText, buildFixPrompt, copyText } from "@/lib/aiPrompt";

// Two developer utilities on an AI analysis result:
//  - Copy output: the raw analysis text.
//  - Copy as prompt: a ready-to-run prompt to paste into a terminal LLM (Claude)
//    to analyse further or produce a concrete fix.
export function AIReportActions({ report }: { report: TroubleshootReport }) {
  const [copied, setCopied] = useState<"output" | "prompt" | null>(null);

  const doCopy = async (kind: "output" | "prompt") => {
    const text = kind === "output" ? formatReportText(report) : buildFixPrompt(report);
    if (await copyText(text)) {
      setCopied(kind);
      setTimeout(() => setCopied((c) => (c === kind ? null : c)), 1800);
    }
  };

  const base =
    "inline-flex items-center gap-1.5 text-xs font-semibold px-3 py-2 rounded-lg border transition-colors";

  return (
    <div className="flex flex-wrap gap-2">
      <button
        onClick={() => doCopy("output")}
        className={`${base} bg-pilot-surface border-pilot-border text-pilot-text-secondary hover:text-pilot-text-primary hover:border-pilot-border-hover`}
        title="Copy the analysis text"
      >
        {copied === "output" ? <Check className="w-3.5 h-3.5 text-pilot-success" /> : <Copy className="w-3.5 h-3.5" />}
        {copied === "output" ? "Copied" : "Copy output"}
      </button>
      <button
        onClick={() => doCopy("prompt")}
        className={`${base} bg-pilot-accent/15 border-pilot-accent/30 text-pilot-accent hover:bg-pilot-accent/25`}
        title="Copy a prompt to paste into Claude / an LLM to fix this"
      >
        {copied === "prompt" ? <Check className="w-3.5 h-3.5" /> : <Terminal className="w-3.5 h-3.5" />}
        {copied === "prompt" ? "Copied prompt" : "Copy as prompt"}
      </button>
    </div>
  );
}
