import { useEffect, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import type { PodSummary } from "@/lib/api";
import { troubleshootPod } from "@/lib/api";
import { AlertTriangle, CheckCircle, RefreshCw, Search, X } from "lucide-react";
import { PortForwardButton } from "@/components/PortForwardButton";

interface Props {
  pods: PodSummary[];
  loading: boolean;
  // When provided, clicking a pod row (outside the action buttons) invokes this
  // callback — used by the Kubernetes Dashboard to open the pod detail drawer.
  onRowClick?: (namespace: string, name: string) => void;
  // Enables the per-row Port Forward control when the server allows mutations.
  mutationsEnabled?: boolean;
  // Optional extra filter control rendered right-aligned on the search row.
  filterSlot?: ReactNode;
}

export function PodTable({ pods, loading, onRowClick, mutationsEnabled = false, filterSlot }: Props) {
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
      {/* Filters row: search (left) + optional filter slot (right) */}
      <div className="flex flex-wrap items-center justify-between gap-3 mb-4">
        <div className="flex items-center gap-2 bg-pilot-surface border border-pilot-border rounded-lg px-3 py-2 w-72 max-w-full focus-within:border-pilot-accent/60 focus-within:ring-2 focus-within:ring-pilot-accent/25">
          <Search className="w-4 h-4 text-pilot-muted shrink-0" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Filter pods..."
            className="bg-transparent text-sm text-pilot-text-primary placeholder:text-pilot-muted focus:outline-none w-full"
          />
        </div>
        {filterSlot}
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
                <tr>
                  <th className="text-left px-5 py-3.5 eyebrow">Namespace</th>
                  <th className="text-left px-5 py-3.5 eyebrow">Pod</th>
                  <th className="text-left px-5 py-3.5 eyebrow">Phase</th>
                  <th className="text-left px-5 py-3.5 eyebrow">Reason</th>
                  <th className="text-left px-5 py-3.5 eyebrow">Restarts</th>
                  <th className="text-left px-5 py-3.5 eyebrow">Uptime</th>
                  <th className="text-left px-5 py-3.5 eyebrow">Node</th>
                  <th className="text-left px-5 py-3.5 eyebrow">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-pilot-border">
                {filtered.map((pod) => (
                  <tr
                    key={`${pod.Namespace}/${pod.Name}`}
                    onClick={onRowClick ? () => onRowClick(pod.Namespace, pod.Name) : undefined}
                    className={`hover:bg-pilot-surface-2 ${onRowClick ? "cursor-pointer" : ""}`}
                  >
                    <td className="px-5 py-3.5 text-sm text-pilot-text-secondary">{pod.Namespace}</td>
                    <td className="px-5 py-3.5 text-sm text-pilot-text-primary font-mono font-semibold">{pod.Name}</td>
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
                    <td className="px-5 py-3.5 text-sm text-pilot-text-secondary font-mono">{pod.Uptime || "\u2014"}</td>
                    <td className="px-5 py-3.5 text-sm text-pilot-text-secondary">{pod.NodeName || "\u2014"}</td>
                    <td className="px-5 py-3.5">
                      <div className="flex items-center gap-2">
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            setTroubleshootTarget({ namespace: pod.Namespace, pod: pod.Name });
                          }}
                          className="text-sm bg-pilot-accent/10 text-pilot-accent-light px-3 py-1.5 rounded-lg hover:bg-pilot-accent/20 font-medium"
                        >
                          AI Diagnose
                        </button>
                        <PortForwardButton
                          kind="pod"
                          namespace={pod.Namespace}
                          name={pod.Name}
                          mutationsEnabled={mutationsEnabled}
                        />
                      </div>
                    </td>
                  </tr>
                ))}
                {filtered.length === 0 && (
                  <tr>
                    <td colSpan={8} className="px-5 py-10 text-center text-pilot-muted text-sm">
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
    ? "bg-pilot-success/10"
    : isPending
    ? "bg-pilot-warning/10"
    : "bg-pilot-danger/10";

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

  // Close on Escape, mirroring the backdrop click, so the panel is dismissable
  // even if a narrow viewport clips the corner close button.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    // z-[70] sits above the sticky GlobalNav (z-[60]) and mobile sidebar (z-[66])
    // so the header no longer overlaps the panel or hides its close button.
    <div
      className="fixed inset-0 bg-black/60 backdrop-blur-sm z-[70] flex justify-end"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label={`AI diagnosis for ${namespace}/${pod}`}
        onClick={(e) => e.stopPropagation()}
        className="w-full max-w-xl bg-pilot-bg border-l border-pilot-border h-full overflow-y-auto p-6 animate-slide-in-right"
      >
        <div className="flex items-center justify-between gap-3 mb-6">
          <div className="min-w-0">
            <h3 className="font-bold font-display text-pilot-text-primary text-base">AI Diagnosis</h3>
            <p className="text-sm text-pilot-muted mt-0.5 font-mono truncate">{namespace}/{pod}</p>
          </div>
          <button
            onClick={onClose}
            aria-label="Close diagnosis panel"
            className="shrink-0 text-pilot-muted hover:text-pilot-text-primary p-1.5 rounded-lg hover:bg-pilot-surface"
          >
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
              <p className="eyebrow mb-2">Root Cause</p>
              <p className="text-base font-bold text-pilot-danger leading-relaxed">{report.RootCause || "Unknown"}</p>
            </div>
            <div className="bg-pilot-surface border border-pilot-border rounded-xl p-5">
              <p className="eyebrow mb-2">Analysis</p>
              <p className="text-sm text-pilot-text-primary leading-relaxed">{report.Analysis}</p>
            </div>
            {(report.Actions || []).length > 0 && (
              <div>
                <p className="eyebrow mb-3">Suggested Actions</p>
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
