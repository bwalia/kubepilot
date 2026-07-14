/**
 * AutopilotPanel — live view of KubePilot's closed-loop self-healing.
 *
 * Shows the current policy, aggregate stats, a global pause/resume kill switch,
 * and the audit ledger of recent decisions (executed / dry-run / skipped /
 * escalated / failed).
 */
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Bot,
  Power,
  Eye,
  Zap,
  ShieldCheck,
  CheckCircle2,
  XCircle,
  SkipForward,
  UserCog,
  ChevronRight,
  Wrench,
  ArrowRight,
  Clock,
} from "lucide-react";
import {
  getAutopilotStatus,
  setAutopilotMode,
  type AutopilotVerdict,
  type AutopilotStatus,
  type AutopilotMode,
} from "@/lib/api";
import { AutopilotLiveStatus } from "@/components/AutopilotLiveStatus";
import { AutopilotDecisionDetail } from "@/components/AutopilotDecisionDetail";
import type { AutopilotDecision } from "@/lib/api";

const VERDICT_META: Record<
  AutopilotVerdict,
  { label: string; badge: string; Icon: typeof CheckCircle2 }
> = {
  executed: {
    label: "Executed",
    badge: "bg-pilot-success/12 text-pilot-success border-pilot-success/25",
    Icon: CheckCircle2,
  },
  "dry-run": {
    label: "Dry-run",
    badge: "bg-pilot-accent/12 text-pilot-accent border-pilot-accent/25",
    Icon: Eye,
  },
  skipped: {
    label: "Skipped",
    badge: "bg-pilot-surface-2 text-pilot-muted border-pilot-border",
    Icon: SkipForward,
  },
  escalated: {
    label: "Escalated",
    badge: "bg-pilot-warning/12 text-pilot-warning border-pilot-warning/25",
    Icon: UserCog,
  },
  failed: {
    label: "Failed",
    badge: "bg-pilot-danger/12 text-pilot-danger border-pilot-danger/25",
    Icon: XCircle,
  },
};

const MODE_BADGE: Record<string, string> = {
  off: "bg-pilot-surface-2 text-pilot-muted border-pilot-border",
  "dry-run": "bg-pilot-accent/12 text-pilot-accent border-pilot-accent/25",
  active: "bg-pilot-success/12 text-pilot-success border-pilot-success/25",
};

function formatCooldown(ns?: number): string {
  if (!ns) return "—";
  const minutes = Math.round(ns / 1e9 / 60);
  return minutes >= 1 ? `${minutes}m` : `${Math.round(ns / 1e9)}s`;
}

export function AutopilotPanel() {
  const qc = useQueryClient();
  const [pendingActive, setPendingActive] = useState(false);
  const [selected, setSelected] = useState<AutopilotDecision | null>(null);

  const { data, isLoading, isFetching } = useQuery<AutopilotStatus>({
    queryKey: ["autopilot-status"],
    // Fetch the full ledger (matches the server's maxLedger) so the verdict
    // stats and the Executed section never disagree — a single executed
    // decision can otherwise sit older than the latest N on a noisy cluster.
    queryFn: () => getAutopilotStatus(200),
    refetchInterval: 10_000,
  });

  // Live mode switch — applied in-memory by the server, no restart/rebuild.
  const setMode = useMutation({
    mutationFn: setAutopilotMode,
    onSuccess: (s) => {
      qc.setQueryData(["autopilot-status"], s);
      setPendingActive(false);
    },
  });
  const changeMode = (m: AutopilotMode) => {
    setPendingActive(false);
    setMode.mutate(m);
  };

  const policy = data?.policy;
  const mode = policy?.mode ?? "off";
  const stats = data?.stats ?? {};
  const decisions = data?.decisions ?? [];
  const executed = decisions.filter((d) => d.verdict === "executed");

  return (
    <div className="space-y-6">
      {/* Header + kill switch */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div className="flex items-center gap-2.5">
          <Bot className="w-6 h-6 text-pilot-accent" />
          <div>
            <h1 className="text-xl font-display font-bold leading-tight text-pilot-text-primary">Autopilot</h1>
            <p className="text-sm text-pilot-muted">AI-driven self-healing</p>
          </div>
          <span
            className={`ml-2 eyebrow px-2 py-0.5 rounded-md border ${
              MODE_BADGE[mode] || MODE_BADGE.off
            }`}
          >
            {mode}
          </span>
        </div>

        {/* Live mode switch — changes take effect instantly, no restart/rebuild. */}
        <div className="flex flex-col items-end gap-1">
          <div className="inline-flex rounded-lg border border-pilot-border bg-pilot-surface p-0.5">
            <ModeButton
              label="Off"
              Icon={Power}
              active={mode === "off"}
              tone="muted"
              disabled={setMode.isPending}
              onClick={() => changeMode("off")}
            />
            <ModeButton
              label="Dry-run"
              Icon={Eye}
              active={mode === "dry-run"}
              tone="sky"
              disabled={setMode.isPending}
              onClick={() => changeMode("dry-run")}
            />
            <ModeButton
              label={pendingActive && mode !== "active" ? "Confirm active" : "Active"}
              Icon={Zap}
              active={mode === "active"}
              tone={pendingActive && mode !== "active" ? "danger" : "emerald"}
              disabled={setMode.isPending}
              onClick={() => {
                if (mode === "active") return;
                if (pendingActive) changeMode("active");
                else setPendingActive(true);
              }}
            />
          </div>
          <span className="text-[10px] text-pilot-muted">
            {setMode.isPending ? "applying…" : "switches instantly — no restart"}
          </span>
        </div>
      </div>

      {/* Live "is it working?" banner: spinning ball, status bar, message, thermometer */}
      <AutopilotLiveStatus status={data} isFetching={isFetching} />

      {/* Stats */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
        {(["executed", "dry-run", "escalated", "skipped", "failed"] as AutopilotVerdict[]).map(
          (v) => {
            const meta = VERDICT_META[v];
            return (
              <div
                key={v}
                className="bg-pilot-surface border border-pilot-border rounded-xl shadow-card p-3"
              >
                <div className="flex items-center gap-1.5 eyebrow">
                  <meta.Icon className="w-3.5 h-3.5" /> {meta.label}
                </div>
                <div className="text-2xl font-display font-bold tabular-nums text-pilot-text-primary mt-1">{stats[v] ?? 0}</div>
              </div>
            );
          },
        )}
        <div className="bg-pilot-surface border border-pilot-border rounded-xl shadow-card p-3">
          <div className="flex items-center gap-1.5 eyebrow">
            <ShieldCheck className="w-3.5 h-3.5" /> Last hour
          </div>
          <div className="text-2xl font-display font-bold tabular-nums text-pilot-text-primary mt-1">
            {stats["actions_last_hour"] ?? 0}
            {policy ? (
              <span className="text-sm text-pilot-muted font-normal">
                /{policy.max_actions_per_hour}
              </span>
            ) : null}
          </div>
        </div>
      </div>

      {/* Policy summary */}
      {policy && (
        <div className="bg-pilot-surface border border-pilot-border rounded-xl shadow-card p-4">
          <h2 className="text-sm font-display font-semibold text-pilot-text-primary mb-3 flex items-center gap-1.5">
            <ShieldCheck className="w-4 h-4 text-pilot-accent" /> Active policy
          </h2>
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-x-6 gap-y-2 text-sm">
            <Field label="Min confidence" value={`${Math.round(policy.min_confidence * 100)}%`} />
            <Field label="Max risk" value={policy.max_risk} />
            <Field label="Cooldown / resource" value={formatCooldown(policy.cooldown)} />
            <Field label="Rate limit" value={`${policy.max_actions_per_hour}/hour`} />
            <Field
              label="Allowed actions"
              value={policy.allowed_actions?.join(", ") || "—"}
            />
            <Field
              label="Namespaces"
              value={
                policy.allowed_namespaces && policy.allowed_namespaces.length > 0
                  ? policy.allowed_namespaces.join(", ")
                  : "all (except blocked)"
              }
            />
            <Field
              label="Blocked namespaces"
              value={policy.blocked_namespaces?.join(", ") || "—"}
            />
          </div>
        </div>
      )}

      {/* Auto-remediated pods — dedicated "what did Autopilot actually fix?" section */}
      <div>
        <h2 className="text-sm font-display font-semibold text-pilot-text-primary mb-3 flex items-center gap-2">
          <CheckCircle2 className="w-4 h-4 text-pilot-success" />
          Auto-remediated pods
          <span className="text-xs font-medium px-2 py-0.5 rounded-md border border-pilot-success/25 bg-pilot-success/12 text-pilot-success">
            {executed.length} executed
          </span>
          {executed.length > 0 ? (
            <span className="text-xs font-normal text-pilot-muted">— click for live logs &amp; what was fixed</span>
          ) : null}
        </h2>
        {executed.length === 0 ? (
          <div className="text-pilot-muted text-sm py-8 text-center bg-pilot-surface border border-pilot-border rounded-xl">
            No pods auto-remediated yet. When Autopilot executes a fix (e.g. recycling a
            crash-looping pod), it appears here with full details.
          </div>
        ) : (
          <div className="grid gap-2 sm:grid-cols-2">
            {executed.map((d, i) => (
              <ExecutedCard key={`exec-${d.report_id}-${i}`} d={d} onClick={() => setSelected(d)} />
            ))}
          </div>
        )}
      </div>

      {/* Decision ledger */}
      <div>
        <h2 className="text-sm font-display font-semibold text-pilot-text-primary mb-3 flex items-center gap-2">
          All recent decisions
          {decisions.length > 0 ? (
            <span className="text-xs font-normal text-pilot-muted">
              — showing latest {Math.min(decisions.length, 50)} of {decisions.length}; click any row for live logs &amp; full details
            </span>
          ) : null}
        </h2>
        {isLoading ? (
          <div className="text-pilot-muted text-sm py-10 text-center">
            Loading autopilot activity...
          </div>
        ) : !data?.enabled && decisions.length === 0 ? (
          <div className="text-pilot-muted text-sm py-10 text-center bg-pilot-surface border border-pilot-border rounded-xl">
            Autopilot is off. Start it in <span className="font-mono">dry-run</span> mode
            (<span className="font-mono">--autopilot-mode=dry-run</span>) to preview what it
            would do as anomalies are detected.
          </div>
        ) : decisions.length === 0 ? (
          <div className="text-pilot-muted text-sm py-10 text-center bg-pilot-surface border border-pilot-border rounded-xl">
            No decisions yet. Autopilot acts when the watcher produces a new RCA report.
          </div>
        ) : (
          <div className="space-y-2">
            {decisions.slice(0, 50).map((d, i) => (
              <DecisionRow key={`${d.report_id}-${i}`} d={d} onClick={() => setSelected(d)} />
            ))}
          </div>
        )}
      </div>

      {/* Click-through detail drawer: live logs + action + RCA */}
      <AutopilotDecisionDetail decision={selected} onClose={() => setSelected(null)} />
    </div>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <div className="eyebrow">{label}</div>
      <div className="font-medium text-pilot-text-primary truncate">{value}</div>
    </div>
  );
}

// ModeButton is one segment of the live off/dry-run/active mode switch.
function ModeButton({
  label,
  Icon,
  active,
  tone,
  disabled,
  onClick,
}: {
  label: string;
  Icon: typeof Power;
  active: boolean;
  tone: "muted" | "sky" | "emerald" | "danger";
  disabled?: boolean;
  onClick: () => void;
}) {
  const activeTone: Record<string, string> = {
    muted: "bg-pilot-surface-2 text-pilot-text-secondary border-pilot-border",
    sky: "bg-pilot-accent/15 text-pilot-accent border-pilot-accent/40",
    emerald: "bg-pilot-success/15 text-pilot-success border-pilot-success/40",
    danger: "bg-pilot-danger/15 text-pilot-danger border-pilot-danger/50 animate-pulse",
  };
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-pressed={active}
      className={`flex items-center gap-1.5 eyebrow px-3 py-1.5 rounded-md border transition-colors disabled:opacity-50 ${
        active || tone === "danger"
          ? activeTone[tone]
          : "border-transparent text-pilot-muted hover:text-pilot-text-primary hover:bg-pilot-bg/60"
      }`}
    >
      <Icon className="w-3.5 h-3.5" />
      {label}
    </button>
  );
}

// describeFix turns an autopilot action into a plain-English "what was fixed".
function describeFix(action?: string): string {
  switch (action) {
    case "delete_pod":
      return "Recycled the pod — deleted the failing pod; its controller recreated a fresh one";
    case "restart":
      return "Rolling-restarted the workload to clear the failure";
    case "scale":
      return "Scaled the workload to recover capacity";
    default:
      return action ? `Applied action: ${action}` : "Applied remediation";
  }
}

// ExecutedCard is the rich, green-themed card shown in the "Auto-remediated pods"
// section: what was wrong → what Autopilot did → the result, click for full detail.
function ExecutedCard({ d, onClick }: { d: AutopilotDecision; onClick: () => void }) {
  const confidence = Math.round((d.confidence ?? 0) * 100);
  return (
    <button
      type="button"
      onClick={onClick}
      className="w-full text-left bg-pilot-success/[0.06] border border-pilot-success/25 rounded-xl shadow-card p-4 cursor-pointer transition-all hover:border-pilot-success/50 hover:shadow-card-hover hover:bg-pilot-success/[0.1] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-pilot-success/50"
    >
      <div className="flex items-center justify-between gap-2">
        <span className="flex items-center gap-1.5 eyebrow px-2 py-0.5 rounded-md border border-pilot-success/25 bg-pilot-success/12 text-pilot-success shrink-0">
          <CheckCircle2 className="w-3.5 h-3.5" /> Executed
        </span>
        <span className="flex items-center gap-0.5 text-xs text-pilot-success font-medium shrink-0">
          Full detail <ChevronRight className="w-4 h-4" />
        </span>
      </div>

      <div className="mt-2 font-mono text-sm text-pilot-text-primary font-semibold truncate">
        {d.resource.namespace}/{d.resource.name}
      </div>

      {/* what was wrong → what was done */}
      <div className="flex items-center gap-2 mt-2 text-xs flex-wrap">
        <span className="px-1.5 py-0.5 rounded bg-pilot-bg border border-pilot-border text-pilot-muted">
          Problem: <span className="text-pilot-text-primary font-semibold">{d.root_cause || "—"}</span>
        </span>
        <ArrowRight className="w-3.5 h-3.5 text-pilot-muted" />
        <span className="px-1.5 py-0.5 rounded bg-pilot-bg border border-pilot-success/25 font-mono text-pilot-success">
          {d.action || "remediated"}
        </span>
      </div>

      <p className="flex items-start gap-1.5 text-sm text-pilot-muted mt-2">
        <Wrench className="w-3.5 h-3.5 mt-0.5 text-pilot-success shrink-0" />
        {describeFix(d.action)}
      </p>

      {d.output ? (
        <pre className="text-xs text-pilot-success/80 mt-2 whitespace-pre-wrap break-all font-mono bg-pilot-bg rounded-md p-2 border border-pilot-border">
          {d.output}
        </pre>
      ) : null}

      <div className="flex items-center gap-3 mt-2 text-xs text-pilot-muted">
        <span className="font-medium tabular-nums">{confidence}% confidence</span>
        <span className="flex items-center gap-1">
          <Clock className="w-3 h-3" /> {new Date(d.time).toLocaleString()}
        </span>
      </div>
    </button>
  );
}

function DecisionRow({ d, onClick }: { d: AutopilotDecision; onClick: () => void }) {
  const meta = VERDICT_META[d.verdict] ?? VERDICT_META.skipped;
  const ts = new Date(d.time);
  const confidence = Math.round((d.confidence ?? 0) * 100);

  return (
    <button
      type="button"
      onClick={onClick}
      className="w-full text-left bg-pilot-surface border border-pilot-border rounded-xl shadow-card p-4 cursor-pointer transition-all hover:border-pilot-border-hover hover:shadow-card-hover hover:bg-pilot-surface/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-pilot-accent/50"
    >
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2.5 min-w-0">
          <span
            className={`flex items-center gap-1 eyebrow px-2 py-0.5 rounded-md shrink-0 border ${meta.badge}`}
          >
            <meta.Icon className="w-3 h-3" /> {meta.label}
          </span>
          {d.action ? (
            <span className="text-xs font-mono px-2 py-0.5 rounded-md bg-pilot-bg border border-pilot-border shrink-0">
              {d.action}
            </span>
          ) : null}
          <span className="text-sm text-pilot-text-primary font-semibold truncate">
            {d.root_cause || "—"}
          </span>
        </div>
        <div className="flex items-center gap-3 text-sm text-pilot-muted shrink-0">
          <span className="font-medium tabular-nums">{confidence}%</span>
          <span className="hidden sm:inline">{ts.toLocaleString()}</span>
          <span className="flex items-center gap-0.5 text-pilot-accent font-medium">
            Details <ChevronRight className="w-4 h-4" />
          </span>
        </div>
      </div>
      <div className="flex items-center gap-2 mt-2 text-sm text-pilot-muted">
        <span className="font-mono text-xs">
          {d.resource.namespace}/{d.resource.name}
        </span>
      </div>
      <p className="text-sm text-pilot-muted mt-1.5">{d.reason}</p>
      {d.output ? (
        <pre className="text-xs text-pilot-muted/80 mt-1.5 whitespace-pre-wrap font-mono">
          {d.output}
        </pre>
      ) : null}
    </button>
  );
}
