/**
 * RCADetail — Full RCA report view with evidence chain, remediation steps,
 * and execute buttons for remediation.
 */
import { useState } from "react";
import {
  Shield,
  ChevronDown,
  ChevronRight,
  AlertTriangle,
  CheckCircle,
  XCircle,
  Play,
} from "lucide-react";
import {
  executeRemediation,
  type RCAReport,
  type Severity,
  type RemediationResult,
} from "@/lib/api";

const SEVERITY_COLORS: Record<Severity, string> = {
  critical: "text-red-400 bg-red-900/40",
  high: "text-orange-400 bg-orange-900/40",
  medium: "text-yellow-400 bg-yellow-900/40",
  low: "text-blue-400 bg-blue-900/40",
  info: "text-gray-400 bg-gray-800",
};

export function RCADetail({ report }: { report: RCAReport }) {
  const [expandedEvidence, setExpandedEvidence] = useState(false);
  const [remediationResults, setRemediationResults] = useState<
    Record<number, RemediationResult>
  >({});
  const [executing, setExecuting] = useState<number | null>(null);

  const severityClass = SEVERITY_COLORS[report.severity] || SEVERITY_COLORS.info;
  const ts = new Date(report.timestamp);
  const confidence = Math.round(report.confidence * 100);

  const handleExecuteStep = async (stepIndex: number) => {
    setExecuting(stepIndex);
    try {
      const result = await executeRemediation(report.id, stepIndex);
      setRemediationResults((prev) => ({ ...prev, [stepIndex]: result }));
    } catch (err) {
      console.error("Remediation failed:", err);
    } finally {
      setExecuting(null);
    }
  };

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <div className="flex items-center gap-2 mb-1">
            <span className={`text-xs font-bold uppercase px-2 py-0.5 rounded ${severityClass}`}>
              {report.severity}
            </span>
            <span className="text-xs text-pilot-muted">{report.id}</span>
          </div>
          <h2 className="text-lg font-bold text-white">
            {report.root_cause.summary}
          </h2>
          <p className="text-sm text-pilot-muted mt-1">
            {report.target_resource.namespace}/{report.target_resource.name} ({report.target_resource.kind})
          </p>
        </div>
        <div className="text-right text-xs text-pilot-muted">
          <div>{ts.toLocaleDateString()}</div>
          <div>{ts.toLocaleTimeString()}</div>
          <div className="mt-1">
            Confidence: <span className="text-white font-bold">{confidence}%</span>
          </div>
        </div>
      </div>

      {/* Root Cause */}
      <section className="bg-pilot-surface border border-pilot-border rounded p-4">
        <h3 className="text-sm font-bold text-pilot-accent mb-2">Root Cause</h3>
        <div className="flex items-center gap-2 text-xs text-pilot-muted mb-2">
          <span className="bg-pilot-bg px-2 py-0.5 rounded">{report.root_cause.category}</span>
        </div>
        <p className="text-sm text-white">{report.root_cause.detail}</p>
        {report.root_cause.affected_components && report.root_cause.affected_components.length > 0 && (
          <div className="mt-2 flex gap-1 flex-wrap">
            {report.root_cause.affected_components.map((comp, i) => (
              <span key={i} className="text-xs bg-pilot-bg text-pilot-muted px-2 py-0.5 rounded">
                {comp}
              </span>
            ))}
          </div>
        )}
      </section>

      {/* Evidence Chain */}
      <section className="bg-pilot-surface border border-pilot-border rounded p-4">
        <button
          onClick={() => setExpandedEvidence(!expandedEvidence)}
          className="flex items-center gap-2 text-sm font-bold text-pilot-accent w-full text-left"
        >
          {expandedEvidence ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
          Evidence Chain ({report.evidence_chain?.length || 0} items)
        </button>
        {expandedEvidence && report.evidence_chain && (
          <div className="mt-3 space-y-2">
            {report.evidence_chain.map((evidence, i) => (
              <div key={i} className="bg-pilot-bg rounded p-3 text-xs">
                <div className="flex items-center gap-2 mb-1">
                  <span className="font-bold text-pilot-accent uppercase">{evidence.source}</span>
                  <span className="text-pilot-muted">{evidence.relevance}</span>
                </div>
                <pre className="text-pilot-muted whitespace-pre-wrap break-words max-h-40 overflow-y-auto">
                  {evidence.data}
                </pre>
              </div>
            ))}
          </div>
        )}
      </section>

      {/* Remediation Steps */}
      {report.remediation && report.remediation.length > 0 && (
        <section className="bg-pilot-surface border border-pilot-border rounded p-4">
          <h3 className="text-sm font-bold text-pilot-accent mb-3 flex items-center gap-2">
            <Shield className="w-4 h-4" /> Remediation Steps
          </h3>
          <div className="space-y-2">
            {report.remediation.map((step, i) => {
              const result = remediationResults[i];
              const isExecuting = executing === i;
              return (
                <div
                  key={i}
                  className="flex items-start justify-between bg-pilot-bg rounded p-3 gap-3"
                >
                  <div className="flex-1">
                    <div className="flex items-center gap-2 text-xs mb-1">
                      <span className="text-pilot-accent font-bold">Step {step.order}</span>
                      <span className="uppercase text-pilot-muted">{step.action}</span>
                      <span
                        className={`px-1.5 py-0.5 rounded text-[10px] font-bold uppercase ${
                          step.risk === "safe"
                            ? "bg-green-900/40 text-green-400"
                            : step.risk === "moderate"
                            ? "bg-yellow-900/40 text-yellow-400"
                            : "bg-red-900/40 text-red-400"
                        }`}
                      >
                        {step.risk}
                      </span>
                      {step.requires_cr && (
                        <span className="px-1.5 py-0.5 rounded text-[10px] font-bold uppercase bg-purple-900/40 text-purple-400">
                          CR Required
                        </span>
                      )}
                    </div>
                    <p className="text-sm text-white">{step.description}</p>
                    {step.command && (
                      <code className="text-xs text-pilot-muted mt-1 block bg-black/20 px-2 py-1 rounded">
                        {step.command}
                      </code>
                    )}
                    {result && (
                      <div className={`mt-2 text-xs flex items-center gap-1 ${result.success ? "text-green-400" : "text-red-400"}`}>
                        {result.success ? <CheckCircle className="w-3 h-3" /> : <XCircle className="w-3 h-3" />}
                        <span>{result.output || result.error}</span>
                        {result.dry_run && <span className="text-pilot-muted">(dry run)</span>}
                      </div>
                    )}
                  </div>
                  <button
                    onClick={() => handleExecuteStep(i)}
                    disabled={isExecuting}
                    className="shrink-0 flex items-center gap-1 text-xs px-3 py-1.5 rounded font-semibold bg-pilot-accent hover:bg-blue-500 text-white disabled:opacity-50"
                  >
                    <Play className="w-3 h-3" />
                    {isExecuting ? "Running..." : "Execute"}
                  </button>
                </div>
              );
            })}
          </div>
        </section>
      )}
    </div>
  );
}
