/**
 * AutopilotLiveStatus — the "is it actually working?" banner for the Autopilot
 * cockpit. When autopilot is enabled it shows a spinning engine ball, a live
 * status message, a stacked status bar of the verdict mix, and a thermometer
 * gauging how much of the hourly action budget has been consumed. When paused
 * (off) it renders the same widgets in a static, dimmed state so it's obvious
 * autopilot is NOT working.
 */
import { RefreshCw } from "lucide-react";
import type { AutopilotStatus, AutopilotVerdict } from "@/lib/api";

interface Props {
  status?: AutopilotStatus;
  isFetching?: boolean;
}

const VERDICT_BAR: Record<AutopilotVerdict, { label: string; color: string }> = {
  executed: { label: "Executed", color: "bg-pilot-success" },
  "dry-run": { label: "Dry-run", color: "bg-pilot-accent" },
  escalated: { label: "Escalated", color: "bg-pilot-warning" },
  skipped: { label: "Skipped", color: "bg-pilot-muted" },
  failed: { label: "Failed", color: "bg-pilot-danger" },
};

const VERDICTS: AutopilotVerdict[] = [
  "executed",
  "dry-run",
  "escalated",
  "skipped",
  "failed",
];

function relativeTime(iso?: string): string {
  if (!iso) return "—";
  const then = new Date(iso).getTime();
  const diff = Date.now() - then;
  if (Number.isNaN(diff)) return "—";
  const s = Math.round(diff / 1000);
  if (s < 60) return `${s}s ago`;
  const m = Math.round(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.round(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.round(h / 24)}d ago`;
}

export function AutopilotLiveStatus({ status, isFetching }: Props) {
  const mode = status?.policy?.mode ?? "off";
  const active = mode !== "off";
  const stats = status?.stats ?? {};

  const cap = status?.policy?.max_actions_per_hour ?? 0;
  const used = stats["actions_last_hour"] ?? 0;
  const util = cap > 0 ? Math.min(used / cap, 1) : 0;

  const total = VERDICTS.reduce((acc, v) => acc + (stats[v] ?? 0), 0);
  const lastDecision = status?.decisions?.[0];

  // Thermometer heat colour rises with budget utilisation.
  const heat =
    !active || util < 0.5
      ? active
        ? "bg-pilot-success"
        : "bg-pilot-muted"
      : util < 0.8
        ? "bg-pilot-warning"
        : "bg-pilot-danger";

  const message = !status
    ? "Connecting to autopilot…"
    : mode === "active"
      ? "Autopilot is active — watching for anomalies and applying safe fixes automatically."
      : mode === "dry-run"
        ? "Autopilot is in dry-run — previewing what it would do, without applying any changes."
        : "Autopilot is paused. The kill switch is engaged; no actions will run.";

  return (
    <div
      className={`rounded-xl border shadow-card p-4 sm:p-5 ${
        active
          ? "border-pilot-accent/40 bg-pilot-accent/5"
          : "border-pilot-border bg-pilot-surface"
      }`}
    >
      <div className="flex items-start gap-4 sm:gap-5">
        {/* ── Spinning engine ball ── */}
        <div className="relative w-14 h-14 shrink-0" aria-hidden>
          <div
            className={`absolute inset-0 rounded-full border-4 border-pilot-border ${
              active ? "border-t-pilot-accent animate-spin" : ""
            }`}
            style={active ? { animationDuration: "1.4s" } : undefined}
          />
          <div
            className={`absolute inset-[10px] rounded-full ${
              active ? "bg-pilot-accent animate-pulse shadow-glow-blue" : "bg-pilot-muted"
            }`}
          />
        </div>

        {/* ── Message + status bar ── */}
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 flex-wrap">
            <span
              className={`eyebrow px-2 py-0.5 rounded-full ${
                active
                  ? "bg-pilot-success/15 text-pilot-success"
                  : "bg-pilot-surface-2 text-pilot-muted"
              }`}
            >
              <span className="relative inline-flex items-center gap-1.5">
                {active && (
                  <span className="relative flex h-2 w-2">
                    <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-pilot-success opacity-75" />
                    <span className="relative inline-flex rounded-full h-2 w-2 bg-pilot-success" />
                  </span>
                )}
                {active ? "Working" : "Idle"}
              </span>
            </span>
            {isFetching && (
              <span className="flex items-center gap-1 text-2xs text-pilot-muted">
                <RefreshCw className="w-3 h-3 animate-spin" /> syncing
              </span>
            )}
          </div>

          <p className="text-sm text-pilot-text-secondary mt-1.5 leading-relaxed">
            {message}
          </p>

          {/* Stacked status bar of the verdict mix */}
          <div className="mt-3">
            <div className="h-3 w-full rounded-full bg-pilot-bg border border-pilot-border overflow-hidden flex">
              {total === 0 ? (
                <div
                  className={`h-full w-full ${
                    active ? "bg-pilot-accent/30 animate-pulse" : "bg-pilot-surface-2"
                  }`}
                />
              ) : (
                VERDICTS.map((v) => {
                  const n = stats[v] ?? 0;
                  if (!n) return null;
                  return (
                    <div
                      key={v}
                      className={`${VERDICT_BAR[v].color} h-full transition-all duration-700`}
                      style={{ width: `${(n / total) * 100}%` }}
                      title={`${VERDICT_BAR[v].label}: ${n}`}
                    />
                  );
                })
              )}
            </div>
            <div className="flex items-center justify-between mt-1.5 text-2xs text-pilot-muted">
              <span>
                {total === 0
                  ? active
                    ? "Awaiting the first decision…"
                    : "No decisions recorded"
                  : `${total} decision${total === 1 ? "" : "s"} tracked`}
              </span>
              <span>Last: {relativeTime(lastDecision?.time)}</span>
            </div>
          </div>
        </div>

        {/* ── Thermometer: hourly action-budget heat ── */}
        <div className="flex flex-col items-center gap-1 shrink-0" title="Hourly action budget used">
          <div className="relative w-3.5 h-24 rounded-full bg-pilot-bg border border-pilot-border overflow-hidden flex items-end">
            <div
              className={`w-full ${heat} transition-all duration-700`}
              style={{ height: `${Math.max(util * 100, active ? 8 : 4)}%` }}
            />
          </div>
          <div
            className={`w-6 h-6 rounded-full ${heat} -mt-2.5 border-2 border-pilot-bg z-10`}
          />
          <span className="text-2xs text-pilot-muted font-mono mt-0.5">
            {used}/{cap || "∞"}
          </span>
          <span className="text-2xs text-pilot-muted">per hr</span>
        </div>
      </div>
    </div>
  );
}
