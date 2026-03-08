/**
 * CRCodeApproval — modal dialog that collects a Change Request code before
 * allowing a production-impacting AI action to execute.
 *
 * The CR code is validated against the Kubernetes-stored secret via the
 * /api/v1/crcode/authorize endpoint. On success the caller receives the
 * onAuthorized callback so it can proceed with job submission.
 */
import { useState } from "react";
import { ShieldCheck, XCircle, Lock } from "lucide-react";
import { authorizeCRCode, type SuggestedAction } from "@/lib/api";

interface Props {
  action: SuggestedAction;
  onClose: () => void;
  onAuthorized: () => void;
}

export function CRCodeApproval({ action, onClose, onAuthorized }: Props) {
  const [changeId, setChangeId] = useState("");
  const [crCode, setCrCode] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async () => {
    if (!changeId.trim() || !crCode.trim()) {
      setError("Both Change ID and CR Code are required.");
      return;
    }

    setLoading(true);
    setError(null);

    try {
      await authorizeCRCode(changeId.trim(), crCode.trim());
      onAuthorized();
    } catch (err: unknown) {
      const message =
        err instanceof Error
          ? err.message
          : "Authorization failed. Check your CR code and try again.";
      setError(message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/70 z-50 flex items-center justify-center p-4">
      <div className="bg-pilot-bg border border-pilot-border rounded-xl w-full max-w-md p-6 shadow-2xl">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-2">
            <Lock className="w-5 h-5 text-pilot-warning" />
            <h2 className="font-bold text-white text-sm">Production Authorization Required</h2>
          </div>
          <button onClick={onClose} className="text-pilot-muted hover:text-white">
            <XCircle className="w-5 h-5" />
          </button>
        </div>

        {/* Action summary */}
        <div className="bg-pilot-surface border border-pilot-border rounded-lg p-3 mb-5">
          <p className="text-xs text-pilot-muted mb-1">Requested action</p>
          <div className="flex items-center gap-2">
            <span className="text-xs font-bold text-pilot-accent uppercase">{action.type}</span>
            {action.namespace && (
              <span className="text-xs text-pilot-muted">{action.namespace}</span>
            )}
            {action.resource && (
              <span className="text-xs text-white">/ {action.resource}</span>
            )}
          </div>
          <p className="text-xs text-pilot-muted mt-1">{action.explanation}</p>
        </div>

        {/* Fields */}
        <div className="space-y-4">
          <div>
            <label className="block text-xs text-pilot-muted mb-1">
              Change Request ID (e.g. INFRA-1234)
            </label>
            <input
              type="text"
              value={changeId}
              onChange={(e) => setChangeId(e.target.value)}
              placeholder="INFRA-1234"
              className="w-full bg-pilot-surface border border-pilot-border rounded px-3 py-2 text-sm text-white placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent"
            />
          </div>
          <div>
            <label className="block text-xs text-pilot-muted mb-1">CR Code</label>
            <input
              type="password"
              value={crCode}
              onChange={(e) => setCrCode(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleSubmit()}
              placeholder="Enter CR code"
              className="w-full bg-pilot-surface border border-pilot-border rounded px-3 py-2 text-sm text-white placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent"
            />
            <p className="text-xs text-pilot-muted mt-1">
              CR codes are validated against Kubernetes secrets in the{" "}
              <code className="text-pilot-accent">kubepilot-security</code> namespace.
            </p>
          </div>

          {error && (
            <div className="flex items-start gap-2 bg-red-950 border border-pilot-danger rounded p-3">
              <XCircle className="w-4 h-4 text-pilot-danger shrink-0 mt-0.5" />
              <p className="text-xs text-pilot-danger">{error}</p>
            </div>
          )}
        </div>

        {/* Actions */}
        <div className="flex gap-3 mt-6">
          <button
            onClick={onClose}
            className="flex-1 bg-pilot-surface border border-pilot-border text-white text-sm py-2 rounded hover:bg-pilot-border"
          >
            Cancel
          </button>
          <button
            onClick={handleSubmit}
            disabled={loading}
            className="flex-1 bg-pilot-warning text-black text-sm py-2 rounded font-bold hover:brightness-110 disabled:opacity-50 flex items-center justify-center gap-2"
          >
            {loading ? (
              "Authorizing…"
            ) : (
              <>
                <ShieldCheck className="w-4 h-4" />
                Authorize & Execute
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  );
}
