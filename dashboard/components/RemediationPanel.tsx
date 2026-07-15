/**
 * RemediationPanel — Execute/approve remediation steps from an RCA report
 * with CR code integration for production-impacting changes.
 */
import { useState } from "react";
import { Play, Shield, CheckCircle, XCircle, Lock } from "lucide-react";
import {
  executeRemediation,
  type RemediationStep,
  type RemediationResult,
} from "@/lib/api";

interface RemediationPanelProps {
  reportId: string;
  steps: RemediationStep[];
}

export function RemediationPanel({ reportId, steps }: RemediationPanelProps) {
  const [results, setResults] = useState<Record<number, RemediationResult>>({});
  const [executing, setExecuting] = useState<number | null>(null);
  const [crModal, setCrModal] = useState<{ stepIndex: number } | null>(null);
  const [changeId, setChangeId] = useState("");
  const [crCode, setCrCode] = useState("");

  const handleExecute = async (stepIndex: number, changeIdVal?: string, crCodeVal?: string) => {
    setExecuting(stepIndex);
    try {
      const result = await executeRemediation(reportId, stepIndex, changeIdVal, crCodeVal);
      setResults((prev) => ({ ...prev, [stepIndex]: result }));
    } catch (err) {
      console.error("Remediation failed:", err);
    } finally {
      setExecuting(null);
      setCrModal(null);
      setChangeId("");
      setCrCode("");
    }
  };

  const handleStepClick = (stepIndex: number, step: RemediationStep) => {
    if (step.requires_cr) {
      setCrModal({ stepIndex });
    } else {
      handleExecute(stepIndex);
    }
  };

  if (!steps || steps.length === 0) {
    return (
      <div className="text-pilot-muted text-sm py-4 text-center">
        No remediation steps available.
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <h3 className="text-sm font-bold font-display text-pilot-text-primary flex items-center gap-2">
        <Shield className="w-4 h-4 text-pilot-accent" /> Remediation Steps
      </h3>

      {steps.map((step, i) => {
        const result = results[i];
        const isExecuting = executing === i;

        return (
          <div key={i} className="bg-pilot-surface border border-pilot-border rounded-lg p-3 shadow-card">
            <div className="flex items-start justify-between gap-3">
              <div className="flex-1">
                <div className="flex items-center gap-2 text-xs mb-1">
                  <span className="text-pilot-accent font-bold">#{step.order}</span>
                  <span className="uppercase text-pilot-text-primary font-semibold">{step.action}</span>
                  <RiskBadge risk={step.risk} />
                  {step.requires_cr && (
                    <span className="flex items-center gap-0.5 px-1.5 py-0.5 rounded text-[12px] font-bold uppercase bg-pilot-info/12 text-pilot-info">
                      <Lock className="w-2.5 h-2.5" /> CR
                    </span>
                  )}
                </div>
                <p className="text-sm text-pilot-text-primary">{step.description}</p>
                {step.command && (
                  <code className="text-xs text-pilot-muted mt-1 block bg-pilot-surface-2 px-2 py-1 rounded font-mono">
                    {step.command}
                  </code>
                )}
              </div>
              <button
                onClick={() => handleStepClick(i, step)}
                disabled={isExecuting}
                className={`shrink-0 flex items-center gap-1 text-xs px-3 py-1.5 rounded-lg font-semibold disabled:opacity-50 ${
                  step.requires_cr
                    ? "bg-pilot-warning text-pilot-bg hover:bg-pilot-warning/90"
                    : "bg-pilot-accent text-pilot-bg hover:bg-pilot-accent-light"
                }`}
              >
                <Play className="w-3 h-3" />
                {isExecuting ? "Running..." : step.requires_cr ? "Authorize" : "Execute"}
              </button>
            </div>

            {result && (
              <div className={`mt-2 text-xs flex items-center gap-1 ${result.success ? "text-pilot-success" : "text-pilot-danger"}`}>
                {result.success ? <CheckCircle className="w-3 h-3" /> : <XCircle className="w-3 h-3" />}
                <span>{result.output || result.error}</span>
                {result.dry_run && <span className="text-pilot-muted ml-1">(dry run)</span>}
              </div>
            )}
          </div>
        );
      })}

      {/* CR Code Modal */}
      {crModal && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
          <div className="bg-pilot-surface border border-pilot-border rounded-xl p-6 w-96 space-y-4 shadow-card">
            <h3 className="text-sm font-bold font-display text-pilot-text-primary flex items-center gap-2">
              <Lock className="w-4 h-4 text-pilot-warning" />
              CR Code Authorization Required
            </h3>
            <p className="text-xs text-pilot-muted">
              This remediation step requires a CR code for production safety.
            </p>
            <input
              type="text"
              value={changeId}
              onChange={(e) => setChangeId(e.target.value)}
              placeholder="Change ID"
              className="w-full bg-pilot-bg border border-pilot-border rounded-lg px-3 py-2 text-sm text-pilot-text-primary placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent/60 focus:ring-2 focus:ring-pilot-accent/25"
            />
            <input
              type="password"
              value={crCode}
              onChange={(e) => setCrCode(e.target.value)}
              placeholder="CR Code"
              className="w-full bg-pilot-bg border border-pilot-border rounded-lg px-3 py-2 text-sm text-pilot-text-primary placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent/60 focus:ring-2 focus:ring-pilot-accent/25"
            />
            <div className="flex gap-2 justify-end">
              <button
                onClick={() => setCrModal(null)}
                className="px-4 py-2 text-xs rounded-lg bg-pilot-bg text-pilot-muted hover:text-pilot-text-primary border border-pilot-border"
              >
                Cancel
              </button>
              <button
                onClick={() => handleExecute(crModal.stepIndex, changeId, crCode)}
                disabled={!changeId || !crCode}
                className="px-4 py-2 text-xs rounded-lg bg-pilot-warning text-pilot-bg font-semibold hover:bg-pilot-warning/90 disabled:opacity-50"
              >
                Authorize & Execute
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function RiskBadge({ risk }: { risk: string }) {
  const classes =
    risk === "safe"
      ? "bg-pilot-success/12 text-pilot-success"
      : risk === "moderate"
      ? "bg-pilot-warning/12 text-pilot-warning"
      : "bg-pilot-danger/12 text-pilot-danger";

  return (
    <span className={`px-1.5 py-0.5 rounded text-[12px] font-bold uppercase ${classes}`}>
      {risk}
    </span>
  );
}
