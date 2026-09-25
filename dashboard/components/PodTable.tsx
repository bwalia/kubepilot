/**
 * PodTable — the pod list, rendered through the shared ResourceTable so it
 * gets the same fuzzy filter, sortable columns, sticky header and phone-sized
 * card layout as every other list in the dashboard.
 *
 * The status column deliberately shows plain English ("Crash looping") rather
 * than the raw phase, with the Kubernetes wording one keystroke away under the
 * "?" — a pod list is the first thing a non-operator is shown when they ask
 * "is my app up?", and "CrashLoopBackOff" does not answer that question.
 */
import { useEffect, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import type { PodSummary } from "@/lib/api";
import { troubleshootPod } from "@/lib/api";
import { RefreshCw, X, Stethoscope } from "lucide-react";
import { PortForwardButton } from "@/components/PortForwardButton";
import { AIReportActions } from "@/components/AIReportActions";
import { ResourceTable, type Column } from "@/components/dashboard/ResourceTable";
import { StatusPill } from "@/components/ui/StatusPill";
import { explainPod } from "@/lib/k8sExplain";

interface Props {
  pods: PodSummary[];
  loading: boolean;
  // When provided, clicking a pod row (outside the action buttons) invokes this
  // callback — used by the Kubernetes Dashboard to open the pod detail drawer.
  onRowClick?: (namespace: string, name: string) => void;
  // Enables the per-row Port Forward control when the server allows mutations.
  mutationsEnabled?: boolean;
  // Optional extra filter control rendered right-aligned on the toolbar.
  filterSlot?: ReactNode;
}

/** Kubernetes-style age ("4d15h", "12m") to minutes, for sorting. */
function ageToMinutes(uptime: string): number {
  if (!uptime) return Number.MAX_SAFE_INTEGER;
  const part = (unit: string) => {
    const m = uptime.match(new RegExp(`(\\d+)\\s*${unit}`));
    return m ? parseInt(m[1], 10) : 0;
  };
  const mins = part("d") * 1440 + part("h") * 60 + part("m") + part("s") / 60;
  return mins || Number.MAX_SAFE_INTEGER;
}

export function PodTable({ pods, loading, onRowClick, mutationsEnabled = false, filterSlot }: Props) {
  const [troubleshootTarget, setTroubleshootTarget] = useState<{
    namespace: string;
    pod: string;
  } | null>(null);

  const columns: Column<PodSummary>[] = [
    {
      header: "Pod",
      cell: (pod) => (
        <span className="font-mono font-semibold text-pilot-text-primary">{pod.Name}</span>
      ),
      sortValue: (pod) => pod.Name,
    },
    {
      header: "Namespace",
      cell: (pod) => <span className="text-pilot-text-secondary">{pod.Namespace}</span>,
      sortValue: (pod) => pod.Namespace,
    },
    {
      header: "Status",
      cell: (pod) => (
        <StatusPill
          explanation={explainPod(pod)}
          raw={pod.Reason || (pod.Phase && !pod.Ready ? `${pod.Phase} (not ready)` : pod.Phase)}
        />
      ),
      // Sort by severity, not alphabetically — "show me what is broken" is the
      // only reason anyone sorts a status column.
      sortValue: (pod) => {
        const rank = { bad: 0, warn: 1, info: 2, idle: 3, ok: 4 } as const;
        return rank[explainPod(pod).tone];
      },
    },
    {
      header: "Restarts",
      align: "right",
      cell: (pod) => (
        <span
          className={
            pod.Restarts > 5
              ? "font-bold tabular-nums text-pilot-warning"
              : "tabular-nums text-pilot-text-secondary"
          }
        >
          {pod.Restarts}
        </span>
      ),
      sortValue: (pod) => pod.Restarts,
    },
    {
      header: "Age",
      cell: (pod) => <span className="font-mono text-pilot-text-secondary">{pod.Uptime || "\u2014"}</span>,
      sortValue: (pod) => ageToMinutes(pod.Uptime),
      hideBelow: "lg",
    },
    {
      header: "Machine",
      cell: (pod) => <span className="text-pilot-text-secondary">{pod.NodeName || "\u2014"}</span>,
      sortValue: (pod) => pod.NodeName,
      hideBelow: "lg",
    },
    {
      header: "Actions",
      align: "right",
      cell: (pod) => (
        <div className="flex items-center justify-end gap-2">
          <button
            onClick={(e) => {
              e.stopPropagation();
              setTroubleshootTarget({ namespace: pod.Namespace, pod: pod.Name });
            }}
            className="inline-flex items-center gap-1.5 rounded-lg bg-pilot-accent/10 px-3 py-1.5 text-sm font-medium text-pilot-accent-light transition-colors hover:bg-pilot-accent/20"
            title="Ask the AI what is wrong with this pod"
          >
            <Stethoscope className="h-3.5 w-3.5" aria-hidden="true" />
            Diagnose
          </button>
          <PortForwardButton
            kind="pod"
            namespace={pod.Namespace}
            name={pod.Name}
            mutationsEnabled={mutationsEnabled}
          />
        </div>
      ),
    },
  ];

  return (
    <div>
      <ResourceTable
        columns={columns}
        items={pods}
        rowKey={(pod) => `${pod.Namespace}/${pod.Name}`}
        loading={loading}
        noun="pod"
        searchText={(pod) => `${pod.Namespace}/${pod.Name} ${pod.NodeName} ${pod.Phase} ${pod.Reason}`}
        searchPlaceholder="Find a pod by name…"
        filterSlot={filterSlot}
        onRowClick={onRowClick ? (pod) => onRowClick(pod.Namespace, pod.Name) : undefined}
        emptyMessage="No pods are running here."
      />

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
            <div className="flex justify-end">
              <AIReportActions report={report} />
            </div>
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
