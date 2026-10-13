/** Shared formatting + colour helpers for the OTLP pages. */
import type { OTELHealth, OTELRed } from "@/lib/api";

export const fmtMs = (v: number) =>
  !Number.isFinite(v) || v <= 0 ? "—" : v >= 1000 ? `${(v / 1000).toFixed(2)}s` : v >= 10 ? `${Math.round(v)}ms` : `${v.toFixed(1)}ms`;

export const fmtPct = (v: number) => (v <= 0 ? "0%" : v < 0.001 ? "<0.1%" : `${(v * 100).toFixed(v < 0.1 ? 1 : 0)}%`);

export const fmtRate = (perMin: number) =>
  perMin >= 60 ? `${(perMin / 60).toFixed(perMin >= 600 ? 0 : 1)}/s` : `${perMin >= 10 ? Math.round(perMin) : perMin.toFixed(1)}/min`;

export const HEALTH_STROKE: Record<OTELHealth, string> = {
  healthy: "rgb(var(--p-success))",
  degraded: "rgb(var(--p-warning))",
  critical: "rgb(var(--p-danger))",
  unknown: "rgb(var(--p-border-hover))",
};

export const HEALTH_TEXT: Record<OTELHealth, string> = {
  healthy: "text-pilot-success",
  degraded: "text-pilot-warning",
  critical: "text-pilot-danger",
  unknown: "text-pilot-muted",
};

export const HEALTH_LABEL: Record<OTELHealth, string> = {
  healthy: "Healthy",
  degraded: "Degraded",
  critical: "Failing",
  unknown: "No traffic",
};

/** Arrow + colour for "now vs baseline" — up is bad for errors and latency. */
export function delta(cur: number, base: number | undefined, kind: "bad-up" | "neutral" = "bad-up") {
  if (base === undefined || base <= 0 || cur <= 0) return null;
  const ratio = cur / base;
  if (ratio > 0.8 && ratio < 1.25) return { text: "≈", cls: "text-pilot-muted" };
  const up = ratio >= 1;
  const text = up ? `▲ ${ratio >= 10 ? Math.round(ratio) : ratio.toFixed(1)}×` : `▼ ${(1 / ratio).toFixed(1)}×`;
  if (kind === "neutral") return { text, cls: "text-pilot-muted" };
  return { text, cls: up ? "text-pilot-danger" : "text-pilot-success" };
}

/** Stable colour per service for the waterfall. */
const PALETTE = ["#0ea5e9", "#8b5cf6", "#10b981", "#f59e0b", "#ec4899", "#14b8a6", "#6366f1", "#84cc16", "#f97316", "#06b6d4"];
export function serviceColor(name: string | undefined) {
  const s = name || "?";
  let h = 0;
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0;
  return PALETTE[h % PALETTE.length];
}

export const errorRateOf = (r?: OTELRed) => (r && r.requests > 0 ? r.error_rate : 0);

export function timeAgo(iso: string) {
  const s = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000);
  if (s < 60) return `${Math.round(s)}s ago`;
  if (s < 3600) return `${Math.round(s / 60)}m ago`;
  return `${Math.round(s / 3600)}h ago`;
}
