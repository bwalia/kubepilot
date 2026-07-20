import type { SuggestedAction, TroubleshootReport } from "@/lib/api";

// Shared helpers for turning an AI troubleshooting report into copy-pasteable
// text — either the raw analysis, or a ready-to-run prompt a developer can hand
// to an LLM (e.g. Claude in a terminal) to analyse further or produce a fix.

function formatAction(a: SuggestedAction): string {
  const bits = [`[${a.type}]`];
  if (a.resource) bits.push(a.resource);
  if (a.namespace) bits.push(`ns=${a.namespace}`);
  if (typeof a.replicas === "number") bits.push(`replicas=${a.replicas}`);
  if (a.command) bits.push(`cmd: ${a.command}`);
  const head = bits.join(" ");
  return a.explanation ? `- ${head} — ${a.explanation}` : `- ${head}`;
}

/** Plain-text rendering of the report — for a raw copy of what's on screen. */
export function formatReportText(report: TroubleshootReport): string {
  const lines: string[] = [];
  lines.push(`KubePilot AI analysis — pod ${report.PodName} (namespace ${report.Namespace})`);
  lines.push("");
  lines.push("Root cause:");
  lines.push(report.RootCause || "Unknown");
  lines.push("");
  lines.push("Analysis:");
  lines.push(report.Analysis || "(none)");
  if (report.Actions && report.Actions.length > 0) {
    lines.push("");
    lines.push("Suggested actions:");
    report.Actions.forEach((a) => lines.push(formatAction(a)));
  }
  return lines.join("\n");
}

/**
 * A prompt a developer can paste into a terminal LLM to get a concrete fix.
 * It embeds the full analysis and asks for copy-paste kubectl/manifest steps.
 */
export function buildFixPrompt(report: TroubleshootReport): string {
  return [
    "I'm troubleshooting a Kubernetes workload. Below is an AI root-cause analysis from KubePilot.",
    "Please help me fix it: confirm the likely root cause, then give me exact copy-paste kubectl",
    "commands and/or manifest changes to resolve it, and call out anything risky before I run it.",
    "",
    `Pod: ${report.PodName}`,
    `Namespace: ${report.Namespace}`,
    "",
    "Root cause:",
    report.RootCause || "Unknown",
    "",
    "Analysis:",
    report.Analysis || "(none)",
    ...(report.Actions && report.Actions.length > 0
      ? ["", "Suggested actions from KubePilot:", ...report.Actions.map(formatAction)]
      : []),
    "",
    "Deliverables:",
    "1. The most likely root cause in one line.",
    "2. Exact kubectl commands (and any YAML) to fix it, ready to paste.",
    "3. A quick verification step to confirm the fix worked.",
  ].join("\n");
}

/** Copy text to the clipboard, with a legacy fallback. Returns success. */
export async function copyText(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch {
    /* fall through to legacy path */
  }
  try {
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.select();
    const ok = document.execCommand("copy");
    document.body.removeChild(ta);
    return ok;
  } catch {
    return false;
  }
}
