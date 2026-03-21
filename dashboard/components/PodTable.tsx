import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import type { PodSummary } from "@/lib/api";
import { troubleshootPod } from "@/lib/api";
import { AlertTriangle, CheckCircle, RefreshCw, Search, X } from "lucide-react";

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
      <div className="flex items-center gap-2 mb-4">
        <div className="flex items-center gap-2 bg-pilot-surface border border-pilot-border rounded-lg px-3 py-2 w-72 focus-within:border-pilot-accent focus-within:ring-1 focus-within:ring-pilot-accent/30">
          <Search className="w-4 h-4 text-pilot-muted shrink-0" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Filter pods..."
            className="bg-transparent text-sm text-white placeholder:text-pilot-muted focus:outline-none w-full"
          />
        </div>
      </div>

      {loading ? (
        <div className="space-y-2">
          {[...Array(5)].map((_, i) => (
            <div key={i} className="h-14 bg-pilot-surface rounded-xl animate-pulse" />
          ))}
        </div>
      ) : (
        <div className="bg-pilot-surface border border-pilot-border rounded-xl overflow-hidden shadow-card">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-pilot-surface-2 text-pilot-muted text-xs uppercase tracking-wider">
                  <th className="text-left px-5 py-3.5 font-semibold">Namespace</th>
                  <th className="text-left px-5 py-3.5 font-semibold">Pod</th>
                  <th className="text-left px-5 py-3.5 font-semibold">Phase</th>
                  <th className="text-left px-5 py-3.5 font-semibold">Reason</th>
                  <th className="text-left px-5 py-3.5 font-semibold">Restarts</th>
                  <th className="text-left px-5 py-3.5 font-semibold">Node</th>
                  <th className="text-left px-5 py-3.5 font-semibold">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-pilot-border">
                {filtered.map((pod) => (
                  <tr
                    key={`${pod.Namespace}/${pod.Name}`}
                    className="hover:bg-pilot-surface-2/50"
                  >
                    <td className="px-5 py-3.5 text-sm text-pilot-text-secondary">{pod.Namespace}</td>
                    <td className="px-5 py-3.5 text-sm text-white font-mono">{pod.Name}</td>
                    <td className="px-5 py-3.5">
                      <PhaseChip phase={pod.Phase} ready={pod.Ready} />
                    </td>
                    <td className="px-5 py-3.5">
                      {pod.Reason ? (
                        <span className="text-sm text-pilot-danger font-semibold">{pod.Reason}</span>
                      ) : (
                        <span className="text-sm text-pilot-muted">&mdash;</span>
                      )}
                    </td>
                    <td className="px-5 py-3.5 text-sm text-center">
                      <span
                        className={pod.Restarts > 5 ? "text-pilot-warning font-bold" : "text-pilot-text-secondary"}
                      >
                        {pod.Restarts}
                      </span>
                    </td>
                    <td className="px-5 py-3.5 text-sm text-pilot-text-secondary">{pod.NodeName || "\u2014"}</td>
                    <td className="px-5 py-3.5">
                      <button
                        onClick={() =>
                          setTroubleshootTarget({ namespace: pod.Namespace, pod: pod.Name })
                        }
                        className="text-sm bg-pilot-accent/10 text-pilot-accent-light px-3 py-1.5 rounded-lg hover:bg-pilot-accent/20 font-medium"
                      >
                        AI Diagnose
                      </button>
                    </td>
                  </tr>
                ))}
                {filtered.length === 0 && (
                  <tr>
                    <td colSpan={7} className="px-5 py-10 text-center text-pilot-muted text-sm">
                      No pods match filter.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
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
  const isOk = phase === "Running" && ready;
  const isPending = phase === "Pending";

  const color = isOk
    ? "text-pilot-success"
    : isPending
    ? "text-pilot-warning"
    : "text-pilot-danger";

  const bgColor = isOk
    ? "bg-emerald-500/10"
    : isPending
    ? "bg-amber-500/10"
    : "bg-red-500/10";

  const Icon = isOk ? CheckCircle : AlertTriangle;

  return (
    <span className={`inline-flex items-center gap-1.5 text-sm font-medium px-2 py-0.5 rounded-md ${color} ${bgColor}`}>
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
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex justify-end">
      <div className="w-full max-w-xl bg-pilot-bg border-l border-pilot-border h-full overflow-y-auto p-6 animate-slide-in-right">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h3 className="font-bold text-white text-base">AI Diagnosis</h3>
            <p className="text-sm text-pilot-muted mt-0.5 font-mono">{namespace}/{pod}</p>
          </div>
          <button onClick={onClose} className="text-pilot-muted hover:text-white p-1.5 rounded-lg hover:bg-pilot-surface">
            <X className="w-5 h-5" />
          </button>
        </div>

        {isLoading && (
          <div className="flex items-center gap-3 text-pilot-muted text-sm py-8">
            <RefreshCw className="w-5 h-5 animate-spin" />
            Analyzing pod...
          </div>
        )}

        {report && (
          <div className="space-y-5 animate-fade-in">
            <div className="bg-pilot-surface border border-pilot-border rounded-xl p-5">
              <p className="text-xs font-medium text-pilot-muted uppercase tracking-wider mb-2">Root Cause</p>
              <p className="text-base font-bold text-pilot-danger leading-relaxed">{report.RootCause || "Unknown"}</p>
            </div>
            <div className="bg-pilot-surface border border-pilot-border rounded-xl p-5">
              <p className="text-xs font-medium text-pilot-muted uppercase tracking-wider mb-2">Analysis</p>
              <p className="text-sm text-white leading-relaxed">{report.Analysis}</p>
            </div>
            {(report.Actions || []).length > 0 && (
              <div>
                <p className="text-xs font-medium text-pilot-muted uppercase tracking-wider mb-3">Suggested Actions</p>
                <div className="space-y-2">
                  {(report.Actions || []).map((action, i) => (
                    <div
                      key={i}
                      className="bg-pilot-surface border border-pilot-border rounded-xl p-4"
                    >
                      <span className="text-xs font-bold text-pilot-accent uppercase">
                        {action.type}
                      </span>
                      <p className="text-sm text-pilot-text-secondary mt-1.5 leading-relaxed">{action.explanation}</p>
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
