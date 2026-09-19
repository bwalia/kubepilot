/**
 * OTELAutopilotPanel — coverage cockpit + Metrics/Logs/Traces overview for
 * cluster and application planes. Hybrid: managed store by default, optional export.
 */
import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Activity, Eye, Power, Radio, Waves } from "lucide-react";
import {
  getOTELAutopilotStatus,
  getOTELLogs,
  getOTELMetrics,
  getOTELTraces,
  setOTELAutopilotMode,
  type OTELAutopilotMode,
  type OTELCapability,
  type OTELPlane,
} from "@/lib/api";

const MODE_BADGE: Record<OTELAutopilotMode, string> = {
  off: "bg-pilot-surface-2 text-pilot-muted border-pilot-border",
  observe: "bg-sky-500/10 text-sky-300 border-sky-500/30",
  enable: "bg-pilot-success/10 text-pilot-success border-pilot-success/30",
};

const CAP_BADGE: Record<OTELCapability, string> = {
  ebpf: "bg-violet-500/10 text-violet-300 border-violet-500/30",
  "auto-instr": "bg-pilot-accent/10 text-pilot-accent border-pilot-accent/30",
  "collector-only": "bg-amber-500/10 text-amber-300 border-amber-500/30",
  "needs-config": "bg-orange-500/10 text-orange-300 border-orange-500/30",
  unsupported: "bg-pilot-surface-2 text-pilot-muted border-pilot-border",
};

export function OTELAutopilotPanel() {
  const qc = useQueryClient();
  const [plane, setPlane] = useState<OTELPlane>("");
  const [tab, setTab] = useState<"coverage" | "metrics" | "logs" | "traces">("coverage");

  const { data, isLoading, isFetching } = useQuery({
    queryKey: ["otel-autopilot"],
    queryFn: getOTELAutopilotStatus,
    refetchInterval: 15_000,
  });

  const { data: metrics } = useQuery({
    queryKey: ["otel-metrics", plane],
    queryFn: () => getOTELMetrics({ plane, limit: 40 }),
    enabled: tab === "metrics",
    refetchInterval: 15_000,
  });
  const { data: logs } = useQuery({
    queryKey: ["otel-logs", plane],
    queryFn: () => getOTELLogs({ plane, limit: 40 }),
    enabled: tab === "logs",
    refetchInterval: 15_000,
  });
  const { data: traces } = useQuery({
    queryKey: ["otel-traces", plane],
    queryFn: () => getOTELTraces({ plane, limit: 40 }),
    enabled: tab === "traces",
    refetchInterval: 15_000,
  });

  const setMode = useMutation({
    mutationFn: setOTELAutopilotMode,
    onSuccess: () => qc.invalidateQueries({ queryKey: ["otel-autopilot"] }),
  });

  const mode = data?.policy.mode ?? "off";
  const totals = data?.coverage.totals;
  const apps = data?.coverage.apps ?? [];

  return (
    <div className="space-y-6">
      <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-4">
        <div className="flex items-start gap-3">
          <Radio className="w-6 h-6 text-pilot-accent mt-0.5" />
          <div>
            <div className="flex items-center gap-2 flex-wrap">
              <h2 className="text-xl font-display font-bold text-pilot-text-primary">
                Automatic observability engine
              </h2>
              <span className={`eyebrow px-2 py-0.5 rounded-md border ${MODE_BADGE[mode]}`}>{mode}</span>
              {isFetching && <span className="text-xs text-pilot-muted">refreshing…</span>}
            </div>
            <p className="text-sm text-pilot-muted mt-1 max-w-2xl">
              Discover every app, enable telemetry without code changes when possible, and inspect
              cluster + application OTEL in KubePilot. Optional Prometheus/Thanos/Cortex export is Hybrid dual-write.
            </p>
          </div>
        </div>
        <div className="inline-flex rounded-lg border border-pilot-border bg-pilot-surface p-0.5">
          <ModeButton label="Off" Icon={Power} active={mode === "off"} disabled={setMode.isPending} onClick={() => setMode.mutate("off")} />
          <ModeButton label="Observe" Icon={Eye} active={mode === "observe"} disabled={setMode.isPending} onClick={() => setMode.mutate("observe")} />
          <ModeButton label="Enable" Icon={Activity} active={mode === "enable"} disabled={setMode.isPending} onClick={() => setMode.mutate("enable")} />
        </div>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 xl:grid-cols-7 gap-3">
        <Stat label="Apps" value={totals?.total ?? 0} />
        <Stat label="Enabled" value={totals?.enabled ?? 0} />
        <Stat label="Auto-instr" value={totals?.auto_instr ?? 0} />
        <Stat label="eBPF" value={totals?.ebpf ?? 0} />
        <Stat label="Collector" value={totals?.collector_only ?? 0} />
        <Stat label="Metrics" value={data?.signals.metrics_count ?? 0} />
        <Stat label="Traces" value={data?.signals.traces_count ?? 0} />
      </div>

      <div className="rounded-xl border border-pilot-border bg-pilot-surface p-4 text-sm text-pilot-muted flex flex-wrap gap-x-6 gap-y-2">
        <span>
          Managed store:{" "}
          <strong className="text-pilot-text-primary">
            {data?.signals.managed_store_enabled ? "on" : "off"}
          </strong>
        </span>
        <span>
          Optional export:{" "}
          <strong className="text-pilot-text-primary">
            {data?.signals.export_active ? "dual-write active" : "not configured"}
          </strong>
        </span>
        {data?.signals.export.metrics_remote_write_url && (
          <span className="font-mono text-xs truncate max-w-full">
            remote_write: {data.signals.export.metrics_remote_write_url}
          </span>
        )}
      </div>

      <div className="flex flex-wrap items-center gap-2">
        {(["coverage", "metrics", "logs", "traces"] as const).map((t) => (
          <button
            key={t}
            type="button"
            onClick={() => setTab(t)}
            className={`px-3 py-1.5 rounded-lg text-sm font-medium border transition-colors ${
              tab === t
                ? "bg-pilot-accent/15 border-pilot-accent/40 text-pilot-accent"
                : "bg-pilot-surface border-pilot-border text-pilot-muted hover:text-pilot-text-primary"
            }`}
          >
            {t[0].toUpperCase() + t.slice(1)}
          </button>
        ))}
        {tab !== "coverage" && (
          <div className="ml-auto inline-flex rounded-lg border border-pilot-border p-0.5">
            {([
              ["", "All"],
              ["cluster", "Cluster"],
              ["application", "Apps"],
            ] as const).map(([value, label]) => (
              <button
                key={value || "all"}
                type="button"
                onClick={() => setPlane(value)}
                className={`px-2.5 py-1 text-xs rounded-md ${
                  plane === value ? "bg-pilot-surface-2 text-pilot-text-primary" : "text-pilot-muted"
                }`}
              >
                {label}
              </button>
            ))}
          </div>
        )}
      </div>

      {isLoading && <p className="text-pilot-muted text-sm">Loading OTLP Autopilot…</p>}

      {tab === "coverage" && (
        <div className="overflow-x-auto rounded-xl border border-pilot-border">
          <table className="w-full text-sm">
            <thead className="bg-pilot-surface-2 text-pilot-muted text-left">
              <tr>
                <th className="px-3 py-2 font-medium">Workload</th>
                <th className="px-3 py-2 font-medium">Runtime</th>
                <th className="px-3 py-2 font-medium">Capability</th>
                <th className="px-3 py-2 font-medium">Signals</th>
                <th className="px-3 py-2 font-medium">Status</th>
                <th className="px-3 py-2 font-medium">Next step</th>
              </tr>
            </thead>
            <tbody>
              {apps.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-3 py-8 text-center text-pilot-muted">
                    {mode === "off"
                      ? "OTLP Autopilot is off. Switch to Observe to discover apps, or Enable to auto-instrument the supported subset."
                      : "No workloads discovered yet (or all namespaces are blocked)."}
                  </td>
                </tr>
              )}
              {apps.map((app) => (
                <tr key={app.id} className="border-t border-pilot-border/80 hover:bg-pilot-surface-2/40">
                  <td className="px-3 py-2">
                    <div className="font-medium text-pilot-text-primary">{app.namespace}/{app.name}</div>
                    <div className="text-xs text-pilot-muted">{app.kind}</div>
                  </td>
                  <td className="px-3 py-2 text-pilot-muted">{app.runtime || "—"}</td>
                  <td className="px-3 py-2">
                    <span className={`eyebrow px-2 py-0.5 rounded-md border ${CAP_BADGE[app.capability]}`}>
                      {app.capability}
                    </span>
                    <div className="text-xs text-pilot-muted mt-1 max-w-xs">{app.reason}</div>
                  </td>
                  <td className="px-3 py-2 text-pilot-muted">{app.signals?.join(", ")}</td>
                  <td className="px-3 py-2">
                    {app.enabled ? (
                      <span className="text-pilot-success">enabled</span>
                    ) : (
                      <span className="text-pilot-muted">pending</span>
                    )}
                  </td>
                  <td className="px-3 py-2 text-xs text-pilot-muted max-w-sm">{app.next_step || "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {tab === "metrics" && (
        <SignalList
          empty="No metrics in the managed store yet. Enable Autopilot or push via /api/v1/otel/ingest."
          rows={(metrics?.metrics ?? []).map((m: any) => ({
            key: `${m.name}-${m.timestamp}`,
            title: m.name,
            meta: `${m.service || "—"} · ${m.plane} · ${m.value}${m.unit || ""}`,
            body: m.timestamp,
          }))}
        />
      )}
      {tab === "logs" && (
        <SignalList
          empty="No logs in the managed store yet."
          rows={(logs?.logs ?? []).map((l: any, i: number) => ({
            key: `${l.timestamp}-${i}`,
            title: l.severity || "LOG",
            meta: `${l.service || "—"} · ${l.plane}`,
            body: l.body,
          }))}
        />
      )}
      {tab === "traces" && (
        <SignalList
          empty="No traces in the managed store yet."
          rows={(traces?.traces ?? []).map((t: any) => ({
            key: `${t.trace_id}-${t.span_id}`,
            title: t.name,
            meta: `${t.service || "—"} · ${t.status || "—"} · ${t.trace_id}`,
            body: t.timestamp,
          }))}
        />
      )}
    </div>
  );
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-xl border border-pilot-border bg-pilot-surface px-3 py-3">
      <div className="text-xs text-pilot-muted">{label}</div>
      <div className="text-xl font-display font-bold text-pilot-text-primary mt-0.5">{value}</div>
    </div>
  );
}

function ModeButton({
  label,
  Icon,
  active,
  disabled,
  onClick,
}: {
  label: string;
  Icon: typeof Power;
  active: boolean;
  disabled?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
        active ? "bg-pilot-accent text-white" : "text-pilot-muted hover:text-pilot-text-primary"
      }`}
    >
      <Icon className="w-3.5 h-3.5" />
      {label}
    </button>
  );
}

function SignalList({
  rows,
  empty,
}: {
  rows: Array<{ key: string; title: string; meta: string; body: string }>;
  empty: string;
}) {
  if (!rows.length) {
    return (
      <div className="rounded-xl border border-pilot-border bg-pilot-surface p-8 text-center text-pilot-muted text-sm">
        <Waves className="w-5 h-5 mx-auto mb-2 opacity-60" />
        {empty}
      </div>
    );
  }
  return (
    <ul className="space-y-2">
      {rows.map((row) => (
        <li key={row.key} className="rounded-xl border border-pilot-border bg-pilot-surface px-4 py-3">
          <div className="flex flex-wrap items-baseline justify-between gap-2">
            <div className="font-medium text-pilot-text-primary">{row.title}</div>
            <div className="text-xs text-pilot-muted">{row.meta}</div>
          </div>
          <div className="text-sm text-pilot-muted mt-1 break-all">{row.body}</div>
        </li>
      ))}
    </ul>
  );
}
