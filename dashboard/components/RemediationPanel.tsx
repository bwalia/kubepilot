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
      <h3 className="text-sm font-bold text-pilot-accent flex items-center gap-2">
        <Shield className="w-4 h-4" /> Remediation Steps
      </h3>

      {steps.map((step, i) => {
        const result = results[i];
        const isExecuting = executing === i;

        return (
          <div key={i} className="bg-pilot-surface border border-pilot-border rounded p-3">
            <div className="flex items-start justify-between gap-3">
              <div className="flex-1">
                <div className="flex items-center gap-2 text-xs mb-1">
                  <span className="text-pilot-accent font-bold">#{step.order}</span>
                  <span className="uppercase text-white font-semibold">{step.action}</span>
                  <RiskBadge risk={step.risk} />
                  {step.requires_cr && (
                    <span className="flex items-center gap-0.5 px-1.5 py-0.5 rounded text-[10px] font-bold uppercase bg-purple-900/40 text-purple-400">
                      <Lock className="w-2.5 h-2.5" /> CR
                    </span>
                  )}
                </div>
                <p className="text-sm text-white">{step.description}</p>
                {step.command && (
                  <code className="text-xs text-pilot-muted mt-1 block bg-black/20 px-2 py-1 rounded font-mono">
                    {step.command}
                  </code>
                )}
              </div>
              <button
                onClick={() => handleStepClick(i, step)}
                disabled={isExecuting}
                className={`shrink-0 flex items-center gap-1 text-xs px-3 py-1.5 rounded font-semibold disabled:opacity-50 ${
                  step.requires_cr
                    ? "bg-pilot-warning text-black hover:bg-yellow-400"
                    : "bg-pilot-accent text-white hover:bg-blue-500"
                }`}
              >
                <Play className="w-3 h-3" />
                {isExecuting ? "Running..." : step.requires_cr ? "Authorize" : "Execute"}
              </button>
            </div>

            {result && (
              <div className={`mt-2 text-xs flex items-center gap-1 ${result.success ? "text-green-400" : "text-red-400"}`}>
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
          <div className="bg-pilot-surface border border-pilot-border rounded-lg p-6 w-96 space-y-4">
            <h3 className="text-sm font-bold text-white flex items-center gap-2">
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
              className="w-full bg-pilot-bg border border-pilot-border rounded px-3 py-2 text-sm text-white placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent"
            />
            <input
              type="password"
              value={crCode}
              onChange={(e) => setCrCode(e.target.value)}
              placeholder="CR Code"
              className="w-full bg-pilot-bg border border-pilot-border rounded px-3 py-2 text-sm text-white placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent"
            />
            <div className="flex gap-2 justify-end">
              <button
                onClick={() => setCrModal(null)}
                className="px-4 py-2 text-xs rounded bg-pilot-bg text-pilot-muted hover:text-white border border-pilot-border"
              >
                Cancel
              </button>
              <button
                onClick={() => handleExecute(crModal.stepIndex, changeId, crCode)}
                disabled={!changeId || !crCode}
                className="px-4 py-2 text-xs rounded bg-pilot-warning text-black font-semibold hover:bg-yellow-400 disabled:opacity-50"
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
      ? "bg-green-900/40 text-green-400"
      : risk === "moderate"
      ? "bg-yellow-900/40 text-yellow-400"
      : "bg-red-900/40 text-red-400";

  return (
    <span className={`px-1.5 py-0.5 rounded text-[10px] font-bold uppercase ${classes}`}>
      {risk}
    </span>
  );
}
