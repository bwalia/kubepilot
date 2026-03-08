import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import type { PodSummary } from "@/lib/api";
import { troubleshootPod } from "@/lib/api";
import { AlertTriangle, CheckCircle, RefreshCw, Search } from "lucide-react";

interface Props {
  pods: PodSummary[];
  loading: boolean;
}

export function PodTable({ pods, loading }: Props) {
  const [search, setSearch] = useState("");
  const [troubleshootTarget, setTroubleshootTarget] = useState<{
    namespace: string;
    pod: string;
  } | null>(null);

  const filtered = pods.filter(
    (p) =>
      p.Name.toLowerCase().includes(search.toLowerCase()) ||
      p.Namespace.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div>
      {/* Search bar */}
      <div className="flex items-center gap-2 mb-3">
        <Search className="w-4 h-4 text-pilot-muted" />
        <input
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Filter pods…"
          className="bg-pilot-surface border border-pilot-border rounded px-3 py-1.5 text-sm text-white placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent w-64"
        />
      </div>

      {loading ? (
        <div className="animate-pulse space-y-2">
          {[...Array(5)].map((_, i) => (
            <div key={i} className="h-10 bg-pilot-surface rounded" />
          ))}
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm border border-pilot-border rounded-lg overflow-hidden">
            <thead className="bg-pilot-surface text-pilot-muted text-xs uppercase tracking-widest">
              <tr>
                <th className="text-left px-4 py-3">Namespace</th>
                <th className="text-left px-4 py-3">Pod</th>
                <th className="text-left px-4 py-3">Phase</th>
                <th className="text-left px-4 py-3">Reason</th>
                <th className="text-left px-4 py-3">Restarts</th>
                <th className="text-left px-4 py-3">Node</th>
                <th className="text-left px-4 py-3">Actions</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((pod) => (
                <tr
                  key={`${pod.Namespace}/${pod.Name}`}
                  className="border-t border-pilot-border hover:bg-pilot-surface/50"
                >
                  <td className="px-4 py-3 text-xs text-pilot-muted">{pod.Namespace}</td>
                  <td className="px-4 py-3 text-xs text-white font-mono">{pod.Name}</td>
                  <td className="px-4 py-3">
                    <PhaseChip phase={pod.Phase} ready={pod.Ready} />
                  </td>
                  <td className="px-4 py-3">
                    {pod.Reason ? (
                      <span className="text-xs text-pilot-danger font-bold">{pod.Reason}</span>
                    ) : (
                      <span className="text-xs text-pilot-muted">—</span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-xs text-center">
                    <span
                      className={pod.Restarts > 5 ? "text-pilot-warning font-bold" : "text-pilot-muted"}
                    >
                      {pod.Restarts}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-xs text-pilot-muted">{pod.NodeName || "—"}</td>
                  <td className="px-4 py-3">
                    <button
                      onClick={() =>
                        setTroubleshootTarget({ namespace: pod.Namespace, pod: pod.Name })
                      }
                      className="text-xs bg-pilot-accent text-white px-2 py-1 rounded hover:bg-blue-500"
                    >
                      AI Diagnose
                    </button>
                  </td>
                </tr>
              ))}
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={7} className="px-4 py-8 text-center text-pilot-muted text-xs">
                    No pods match filter.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {/* Troubleshoot slide-over panel */}
      {troubleshootTarget && (
        <TroubleshootPanel
          namespace={troubleshootTarget.namespace}
          pod={troubleshootTarget.pod}
          onClose={() => setTroubleshootTarget(null)}
        />
      )}
    </div>
  );
}

function PhaseChip({ phase, ready }: { phase: string; ready: boolean }) {
  const color =
    phase === "Running" && ready
      ? "text-pilot-success"
      : phase === "Pending"
      ? "text-pilot-warning"
      : "text-pilot-danger";

  const Icon = phase === "Running" && ready ? CheckCircle : AlertTriangle;

  return (
    <span className={`flex items-center gap-1 text-xs ${color}`}>
      <Icon className="w-3.5 h-3.5" />
      {phase}
    </span>
  );
}

function TroubleshootPanel({
  namespace,
  pod,
  onClose,
}: {
  namespace: string;
  pod: string;
  onClose: () => void;
}) {
  const { data: report, isLoading } = useQuery({
    queryKey: ["troubleshoot", namespace, pod],
    queryFn: () => troubleshootPod(namespace, pod),
  });

  return (
    <div className="fixed inset-0 bg-black/60 z-50 flex justify-end">
      <div className="w-full max-w-xl bg-pilot-bg border-l border-pilot-border h-full overflow-y-auto p-6">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-bold text-white text-sm">
            AI Diagnosis — {namespace}/{pod}
          </h3>
          <button onClick={onClose} className="text-pilot-muted hover:text-white text-xs">
            Close
          </button>
        </div>

        {isLoading && (
          <div className="flex items-center gap-2 text-pilot-muted text-sm">
            <RefreshCw className="w-4 h-4 animate-spin" />
            Analyzing pod…
          </div>
        )}

        {report && (
          <div className="space-y-4">
            <div>
              <p className="text-xs text-pilot-muted mb-1">Root Cause</p>
              <p className="text-sm font-bold text-pilot-danger">{report.RootCause || "Unknown"}</p>
            </div>
            <div>
              <p className="text-xs text-pilot-muted mb-1">Analysis</p>
              <p className="text-sm text-white leading-relaxed">{report.Analysis}</p>
            </div>
            {report.Actions.length > 0 && (
              <div>
                <p className="text-xs text-pilot-muted mb-2">Suggested Actions</p>
                <div className="space-y-2">
                  {report.Actions.map((action, i) => (
                    <div
                      key={i}
                      className="bg-pilot-surface border border-pilot-border rounded p-3"
                    >
                      <span className="text-xs font-bold text-pilot-accent uppercase">
                        {action.type}
                      </span>
                      <p className="text-xs text-pilot-muted mt-1">{action.explanation}</p>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
