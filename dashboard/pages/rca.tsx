/**
 * RCA page — List, filter, and view Root Cause Analysis reports.
 */
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { FileSearch, ArrowLeft, Filter } from "lucide-react";
import { listRCAReports, getRCAReport, type RCAReport, type Severity } from "@/lib/api";
import { RCADetail } from "@/components/RCADetail";
import { AnomalyTimeline } from "@/components/AnomalyTimeline";

const SEVERITY_BADGE: Record<Severity, string> = {
  critical: "bg-red-900/40 text-red-400",
  high: "bg-orange-900/40 text-orange-400",
  medium: "bg-yellow-900/40 text-yellow-400",
  low: "bg-blue-900/40 text-blue-400",
  info: "bg-gray-800 text-gray-400",
};

export default function RCAPage() {
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [severityFilter, setSeverityFilter] = useState("");
  const [namespaceFilter, setNamespaceFilter] = useState("");

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
      <div className="min-h-screen bg-pilot-bg text-white font-mono p-6">
        <button
          onClick={() => setSelectedId(null)}
          className="flex items-center gap-1 text-sm text-pilot-accent hover:text-white mb-4"
        >
          <ArrowLeft className="w-4 h-4" /> Back to reports
        </button>
        <RCADetail report={selectedReport} />
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-pilot-bg text-white font-mono p-6 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-bold flex items-center gap-2">
          <FileSearch className="w-5 h-5 text-pilot-accent" />
          RCA Reports
        </h1>
        <div className="flex items-center gap-2">
          <Filter className="w-4 h-4 text-pilot-muted" />
          <select
            value={severityFilter}
            onChange={(e) => setSeverityFilter(e.target.value)}
            className="bg-pilot-surface border border-pilot-border rounded px-2 py-1 text-xs text-white"
          >
            <option value="">All Severities</option>
            <option value="critical">Critical</option>
            <option value="high">High</option>
            <option value="medium">Medium</option>
            <option value="low">Low</option>
          </select>
          <input
            type="text"
            value={namespaceFilter}
            onChange={(e) => setNamespaceFilter(e.target.value)}
            placeholder="Namespace"
            className="bg-pilot-surface border border-pilot-border rounded px-2 py-1 text-xs text-white placeholder:text-pilot-muted w-32"
          />
        </div>
      </div>

      {isLoading ? (
        <div className="text-pilot-muted text-sm py-8 text-center">Loading RCA reports...</div>
      ) : reports.length === 0 ? (
        <div className="text-pilot-muted text-sm py-8 text-center">
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
      className="w-full text-left bg-pilot-surface border border-pilot-border rounded p-3 hover:border-pilot-accent transition-colors"
    >
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2 min-w-0">
          <span className={`text-[10px] font-bold uppercase px-1.5 py-0.5 rounded shrink-0 ${badgeClass}`}>
            {report.severity}
          </span>
          <span className="text-sm text-white font-semibold truncate">
            {report.root_cause.summary}
          </span>
        </div>
        <div className="flex items-center gap-3 text-xs text-pilot-muted shrink-0">
          <span>{report.root_cause.category}</span>
          <span>{confidence}%</span>
          <span>{ts.toLocaleString()}</span>
        </div>
      </div>
      <div className="flex items-center gap-2 mt-1 text-xs text-pilot-muted">
        <span>
          {report.target_resource.namespace}/{report.target_resource.name}
        </span>
        <span className="ml-auto">{report.status}</span>
      </div>
    </button>
  );
}
