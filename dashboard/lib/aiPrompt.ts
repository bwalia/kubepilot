import type { RCAReport, SuggestedAction, TroubleshootReport, TroubleshootingInsight } from "@/lib/api";

// Shared helpers for turning AI analysis into copy-pasteable text —
// either the raw analysis, or a ready-to-run prompt a developer can hand
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

/** Plain-text rendering of a pod troubleshoot report. */
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
  return buildGenericFixPrompt({
    title: `Pod ${report.PodName} (namespace ${report.Namespace})`,
    rootCause: report.RootCause || "Unknown",
    analysis: report.Analysis || "(none)",
    actions: report.Actions?.map(formatAction),
  });
}

/** Plain-text rendering of an RCA report. */
export function formatRCAText(report: RCAReport): string {
  const resource = `${report.target_resource.namespace}/${report.target_resource.name} (${report.target_resource.kind})`;
  const lines: string[] = [
    `KubePilot RCA — ${resource}`,
    `Severity: ${report.severity} · Confidence: ${Math.round((report.confidence ?? 0) * 100)}%`,
    "",
    "Root cause:",
    report.root_cause?.summary || "Unknown",
  ];
  if (report.root_cause?.category) {
    lines.push(`Category: ${report.root_cause.category}`);
  }
  if (report.root_cause?.detail) {
    lines.push("", "Detail:", report.root_cause.detail);
  }
  if (report.root_cause?.affected_components?.length) {
    lines.push("", "Affected components:", ...report.root_cause.affected_components.map((c) => `- ${c}`));
  }
  if (report.evidence_chain?.length) {
    lines.push("", "Evidence:");
    report.evidence_chain.forEach((e, i) => {
      lines.push(`${i + 1}. [${e.source}] ${e.relevance || ""}`.trim());
      if (e.data) lines.push(e.data);
    });
  }
  if (report.remediation?.length) {
    lines.push("", "Remediation steps:");
    report.remediation.forEach((s) => {
      lines.push(`- #${s.order} [${s.action}] risk=${s.risk}${s.requires_cr ? " (needs CR)" : ""}`);
      if (s.description) lines.push(`  ${s.description}`);
      if (s.command) lines.push(`  cmd: ${s.command}`);
    });
  }
  return lines.join("\n");
}

/** Prompt built from an RCA report for pasting into Claude / an LLM. */
export function buildRCAFixPrompt(report: RCAReport): string {
  const resource = `${report.target_resource.namespace}/${report.target_resource.name} (${report.target_resource.kind})`;
  const actions = (report.remediation || []).map((s) => {
    const bits = [`#${s.order}`, `[${s.action}]`, `risk=${s.risk}`];
    if (s.command) bits.push(`cmd: ${s.command}`);
    const head = bits.join(" ");
    return s.description ? `- ${head} — ${s.description}` : `- ${head}`;
  });
  return buildGenericFixPrompt({
    title: resource,
    rootCause: report.root_cause?.summary || "Unknown",
    analysis: [
      report.root_cause?.detail || "",
      report.evidence_chain?.length
        ? `Evidence:\n${report.evidence_chain
            .map((e, i) => `${i + 1}. [${e.source}] ${e.data}`)
            .join("\n")}`
        : "",
    ]
      .filter(Boolean)
      .join("\n\n"),
    actions,
  });
}

/** Plain-text rendering of a cluster troubleshooting insight. */
export function formatInsightText(insight: TroubleshootingInsight): string {
  const lines: string[] = [
    `KubePilot insight — ${insight.title}`,
    `Category: ${insight.category} · Severity: ${insight.severity}`,
    "",
    insight.summary || "(none)",
  ];
  if (insight.suggestions?.length) {
    lines.push("", "Suggestions:", ...insight.suggestions.map((s) => `- ${s}`));
  }
  if (insight.affected_resources?.length) {
    lines.push("", "Affected resources:", ...insight.affected_resources.map((r) => `- ${r}`));
  }
  return lines.join("\n");
}

/** Prompt built from a cluster insight for pasting into Claude / an LLM. */
export function buildInsightFixPrompt(insight: TroubleshootingInsight): string {
  return buildGenericFixPrompt({
    title: insight.title,
    rootCause: `${insight.severity} · ${insight.category}`,
    analysis: [
      insight.summary || "(none)",
      insight.affected_resources?.length
        ? `Affected resources:\n${insight.affected_resources.map((r) => `- ${r}`).join("\n")}`
        : "",
    ]
      .filter(Boolean)
      .join("\n\n"),
    actions: insight.suggestions?.map((s) => `- ${s}`),
  });
}

function buildGenericFixPrompt({
  title,
  rootCause,
  analysis,
  actions,
}: {
  title: string;
  rootCause: string;
  analysis: string;
  actions?: string[];
}): string {
  return [
    "I'm troubleshooting a Kubernetes workload. Below is an AI root-cause analysis from KubePilot.",
    "Please help me fix it: confirm the likely root cause, then give me exact copy-paste kubectl",
    "commands and/or manifest changes to resolve it, and call out anything risky before I run it.",
    "",
    `Target: ${title}`,
    "",
    "Root cause:",
    rootCause,
    "",
    "Analysis:",
    analysis || "(none)",
    ...(actions && actions.length > 0 ? ["", "Suggested actions from KubePilot:", ...actions] : []),
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
