/**
 * RCA page — List, filter, and view Root Cause Analysis reports.
 */
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { FileSearch, ArrowLeft, Filter, Lock } from "lucide-react";
import { listRCAReports, getRCAReport, type RCAReport, type Severity } from "@/lib/api";
import { useNamespaceLock } from "@/lib/useNamespaceLock";
import { RCADetail } from "@/components/RCADetail";
import { AnomalyTimeline } from "@/components/AnomalyTimeline";

const SEVERITY_BADGE: Record<Severity, string> = {
  critical: "bg-pilot-danger/12 text-pilot-danger border-pilot-danger/25",
  high: "bg-pilot-danger/10 text-pilot-danger border-pilot-danger/20",
  medium: "bg-pilot-warning/12 text-pilot-warning border-pilot-warning/25",
  low: "bg-pilot-info/12 text-pilot-info border-pilot-info/25",
  info: "bg-pilot-surface-2 text-pilot-muted border-pilot-border",
};

export default function RCAPage() {
  const { locked, namespace: lockedNamespace } = useNamespaceLock();
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [severityFilter, setSeverityFilter] = useState("");
  const [namespaceInput, setNamespaceInput] = useState("");
  const namespaceFilter = locked ? lockedNamespace! : namespaceInput;

  const { data: reports = [], isLoading } = useQuery({
    queryKey: ["rca-reports", severityFilter, namespaceFilter],
    queryFn: () =>
      listRCAReports({
        severity: severityFilter || undefined,
        namespace: namespaceFilter || undefined,
        since: "24h",
      }),
    refetchInterval: 15_000,
  });

  const { data: selectedReport } = useQuery({
    queryKey: ["rca-report", selectedId],
    queryFn: () => getRCAReport(selectedId!),
    enabled: !!selectedId,
  });

  if (selectedReport) {
    return (
      <div className="min-h-screen bg-pilot-bg text-pilot-text-primary p-6">
        <button
          onClick={() => setSelectedId(null)}
          className="flex items-center gap-1.5 text-sm text-pilot-accent hover:text-pilot-text-primary mb-5 font-medium"
        >
          <ArrowLeft className="w-4 h-4" /> Back to reports
        </button>
        <RCADetail report={selectedReport} />
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-pilot-bg text-pilot-text-primary p-6 space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <h1 className="text-2xl font-bold font-display tracking-tight text-pilot-text-primary flex items-center gap-2.5">
          <FileSearch className="w-6 h-6 text-pilot-accent" />
          RCA Reports
        </h1>
        <div className="flex items-center gap-2">
          <Filter className="w-4 h-4 text-pilot-muted" />
          <select
            value={severityFilter}
            onChange={(e) => setSeverityFilter(e.target.value)}
            className="bg-pilot-surface border border-pilot-border rounded-lg px-3 py-2 text-sm text-pilot-text-primary focus:outline-none focus:border-pilot-accent/60 focus:ring-2 focus:ring-pilot-accent/25"
          >
            <option value="">All Severities</option>
            <option value="critical">Critical</option>
            <option value="high">High</option>
            <option value="medium">Medium</option>
            <option value="low">Low</option>
          </select>
          {locked ? (
            <span
              className="inline-flex items-center gap-1.5 bg-pilot-accent/15 text-pilot-accent-light border border-pilot-accent/40 rounded-lg px-3 py-2 text-sm font-medium w-36"
              title="Locked to this namespace via URL parameter"
            >
              <Lock className="w-3.5 h-3.5 shrink-0" />
              <span className="truncate">{namespaceFilter}</span>
            </span>
          ) : (
            <input
              type="text"
              value={namespaceInput}
              onChange={(e) => setNamespaceInput(e.target.value)}
              placeholder="Namespace"
              className="bg-pilot-surface border border-pilot-border rounded-lg px-3 py-2 text-sm text-pilot-text-primary placeholder:text-pilot-muted w-36 focus:outline-none focus:border-pilot-accent/60 focus:ring-2 focus:ring-pilot-accent/25"
            />
          )}
        </div>
      </div>

      {isLoading ? (
        <div className="text-pilot-muted text-sm py-10 text-center">Loading RCA reports...</div>
      ) : reports.length === 0 ? (
        <div className="text-pilot-muted text-sm py-10 text-center bg-pilot-surface border border-pilot-border rounded-xl">
          No RCA reports found. The watcher will generate reports as anomalies are detected.
        </div>
      ) : (
        <div className="space-y-2">
          {reports.map((report) => (
            <ReportRow
              key={report.id}
              report={report}
              onClick={() => setSelectedId(report.id)}
            />
          ))}
        </div>
      )}

      <div className="border-t border-pilot-border pt-6">
        <AnomalyTimeline />
      </div>
    </div>
  );
}

function ReportRow({
  report,
  onClick,
}: {
  report: RCAReport;
  onClick: () => void;
}) {
  const ts = new Date(report.timestamp);
  const badgeClass = SEVERITY_BADGE[report.severity] || SEVERITY_BADGE.info;
  const confidence = Math.round(report.confidence * 100);

  return (
    <button
      onClick={onClick}
      className="w-full text-left bg-pilot-surface border border-pilot-border rounded-xl p-4 hover:border-pilot-border-hover hover:shadow-card-hover transition-all"
    >
      <div className="flex items-center gap-2.5 min-w-0">
        <span className={`text-xs font-bold uppercase px-2 py-0.5 rounded-md shrink-0 border ${badgeClass}`}>
          {report.severity}
        </span>
        <span className="text-[0.95rem] text-pilot-text-primary font-semibold truncate min-w-0">
          {report.root_cause.summary}
        </span>
        <span className="ml-auto shrink-0 font-semibold text-sm text-pilot-accent-light tabular-nums">{confidence}%</span>
      </div>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 mt-2 text-sm text-pilot-muted">
        <span className="font-mono text-xs truncate max-w-full">
          {report.target_resource.namespace}/{report.target_resource.name}
        </span>
        <span className="text-pilot-border-hover" aria-hidden="true">·</span>
        <span>{report.root_cause.category}</span>
        <span className="text-pilot-border-hover" aria-hidden="true">·</span>
        <span>{ts.toLocaleString()}</span>
        <span className="sm:ml-auto font-semibold text-pilot-text-secondary">{report.status}</span>
      </div>
    </button>
  );
}
