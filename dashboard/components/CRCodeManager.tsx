/**
 * CRCodeManager — standalone CR Code management panel.
 * Allows operators to register new CR codes, test authorization,
 * and revoke existing codes directly from the dashboard.
 */
import { useState } from "react";
import {
  ShieldCheck,
  ShieldOff,
  KeyRound,
  Plus,
  Trash2,
  CheckCircle,
  XCircle,
  Clock,
  AlertTriangle,
} from "lucide-react";
import { registerCRCode, authorizeCRCode, revokeCRCode } from "@/lib/api";

interface CREntry {
  changeId: string;
  registeredAt: string;
  expiresAt?: string;
  status: "active" | "revoked" | "verified";
}

export function CRCodeManager() {
  const [entries, setEntries] = useState<CREntry[]>([]);

  // Register form
  const [regChangeId, setRegChangeId] = useState("");
  const [regCRCode, setRegCRCode] = useState("");
  const [regExpiry, setRegExpiry] = useState("");
  const [regLoading, setRegLoading] = useState(false);
  const [regResult, setRegResult] = useState<{ ok: boolean; msg: string } | null>(null);

  // Verify form
  const [verChangeId, setVerChangeId] = useState("");
  const [verCRCode, setVerCRCode] = useState("");
  const [verLoading, setVerLoading] = useState(false);
  const [verResult, setVerResult] = useState<{ ok: boolean; msg: string } | null>(null);

  // Revoke form
  const [revChangeId, setRevChangeId] = useState("");
  const [revLoading, setRevLoading] = useState(false);
  const [revResult, setRevResult] = useState<{ ok: boolean; msg: string } | null>(null);

  const handleRegister = async () => {
    if (!regChangeId.trim() || !regCRCode.trim()) {
      setRegResult({ ok: false, msg: "Change ID and CR Code are required." });
      return;
    }
    setRegLoading(true);
    setRegResult(null);
    try {
      const expiresAt = regExpiry ? new Date(regExpiry).toISOString() : undefined;
      await registerCRCode(regChangeId.trim(), regCRCode.trim(), expiresAt);
      setEntries((prev) => [
        {
          changeId: regChangeId.trim(),
          registeredAt: new Date().toISOString(),
          expiresAt: expiresAt,
          status: "active",
        },
        ...prev,
      ]);
      setRegResult({ ok: true, msg: `CR code registered for ${regChangeId.trim()}` });
      setRegChangeId("");
      setRegCRCode("");
      setRegExpiry("");
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : "Registration failed.";
      setRegResult({ ok: false, msg: message });
    } finally {
      setRegLoading(false);
    }
  };

  const handleVerify = async () => {
    if (!verChangeId.trim() || !verCRCode.trim()) {
      setVerResult({ ok: false, msg: "Both fields are required." });
      return;
    }
    setVerLoading(true);
    setVerResult(null);
    try {
      await authorizeCRCode(verChangeId.trim(), verCRCode.trim());
      setVerResult({ ok: true, msg: "CR code is valid and authorized." });
      setEntries((prev) =>
        prev.map((e) =>
          e.changeId === verChangeId.trim() ? { ...e, status: "verified" as const } : e
        )
      );
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : "Authorization failed.";
      setVerResult({ ok: false, msg: message });
    } finally {
      setVerLoading(false);
    }
  };

  const handleRevoke = async (changeId?: string) => {
    const target = changeId || revChangeId.trim();
    if (!target) {
      setRevResult({ ok: false, msg: "Change ID is required." });
      return;
    }
    setRevLoading(true);
    setRevResult(null);
    try {
      await revokeCRCode(target);
      setEntries((prev) =>
        prev.map((e) => (e.changeId === target ? { ...e, status: "revoked" as const } : e))
      );
      setRevResult({ ok: true, msg: `CR code revoked for ${target}` });
      if (!changeId) setRevChangeId("");
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : "Revocation failed.";
      setRevResult({ ok: false, msg: message });
    } finally {
      setRevLoading(false);
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center gap-3">
        <KeyRound className="w-5 h-5 text-pilot-warning" />
        <h2 className="text-sm font-bold text-white">CR Code Management</h2>
        <span className="text-xs text-pilot-muted">
          Production change authorization via Kubernetes secrets
        </span>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* ── Register ──────────────────────────────────── */}
        <div className="bg-pilot-surface border border-pilot-border rounded-lg p-5">
          <div className="flex items-center gap-2 mb-4">
            <Plus className="w-4 h-4 text-pilot-success" />
            <h3 className="text-sm font-bold text-white">Register CR Code</h3>
          </div>
          <div className="space-y-3">
            <div>
              <label className="block text-xs text-pilot-muted mb-1">Change Request ID</label>
              <input
                type="text"
                value={regChangeId}
                onChange={(e) => setRegChangeId(e.target.value)}
                placeholder="e.g. INFRA-1234"
                className="w-full bg-pilot-bg border border-pilot-border rounded px-3 py-2 text-sm text-white placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent"
              />
            </div>
            <div>
              <label className="block text-xs text-pilot-muted mb-1">CR Code</label>
              <input
                type="password"
                value={regCRCode}
                onChange={(e) => setRegCRCode(e.target.value)}
                placeholder="Secret code"
                className="w-full bg-pilot-bg border border-pilot-border rounded px-3 py-2 text-sm text-white placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent"
              />
            </div>
            <div>
              <label className="block text-xs text-pilot-muted mb-1">
                Expiry <span className="text-pilot-muted">(optional)</span>
              </label>
              <input
                type="datetime-local"
                value={regExpiry}
                onChange={(e) => setRegExpiry(e.target.value)}
                className="w-full bg-pilot-bg border border-pilot-border rounded px-3 py-2 text-sm text-white focus:outline-none focus:border-pilot-accent [color-scheme:dark]"
              />
            </div>
            <button
              onClick={handleRegister}
              disabled={regLoading}
              className="w-full bg-pilot-success text-black text-sm py-2 rounded font-bold hover:brightness-110 disabled:opacity-50 flex items-center justify-center gap-2"
            >
              {regLoading ? "Registering…" : <><ShieldCheck className="w-4 h-4" /> Register</>}
            </button>
            {regResult && <ResultBanner {...regResult} />}
          </div>
        </div>

        {/* ── Verify ────────────────────────────────────── */}
        <div className="bg-pilot-surface border border-pilot-border rounded-lg p-5">
          <div className="flex items-center gap-2 mb-4">
            <ShieldCheck className="w-4 h-4 text-pilot-accent" />
            <h3 className="text-sm font-bold text-white">Verify CR Code</h3>
          </div>
          <div className="space-y-3">
            <div>
              <label className="block text-xs text-pilot-muted mb-1">Change Request ID</label>
              <input
                type="text"
                value={verChangeId}
                onChange={(e) => setVerChangeId(e.target.value)}
                placeholder="e.g. INFRA-1234"
                className="w-full bg-pilot-bg border border-pilot-border rounded px-3 py-2 text-sm text-white placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent"
              />
            </div>
            <div>
              <label className="block text-xs text-pilot-muted mb-1">CR Code</label>
              <input
                type="password"
                value={verCRCode}
                onChange={(e) => setVerCRCode(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && handleVerify()}
                placeholder="Enter code to verify"
                className="w-full bg-pilot-bg border border-pilot-border rounded px-3 py-2 text-sm text-white placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent"
              />
            </div>
            <button
              onClick={handleVerify}
              disabled={verLoading}
              className="w-full bg-pilot-accent text-white text-sm py-2 rounded font-bold hover:brightness-110 disabled:opacity-50 flex items-center justify-center gap-2"
            >
              {verLoading ? "Verifying…" : <><ShieldCheck className="w-4 h-4" /> Verify</>}
            </button>
            {verResult && <ResultBanner {...verResult} />}
          </div>
        </div>

        {/* ── Revoke ────────────────────────────────────── */}
        <div className="bg-pilot-surface border border-pilot-border rounded-lg p-5">
          <div className="flex items-center gap-2 mb-4">
            <ShieldOff className="w-4 h-4 text-pilot-danger" />
            <h3 className="text-sm font-bold text-white">Revoke CR Code</h3>
          </div>
          <div className="space-y-3">
            <div>
              <label className="block text-xs text-pilot-muted mb-1">Change Request ID</label>
              <input
                type="text"
                value={revChangeId}
                onChange={(e) => setRevChangeId(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && handleRevoke()}
                placeholder="e.g. INFRA-1234"
                className="w-full bg-pilot-bg border border-pilot-border rounded px-3 py-2 text-sm text-white placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent"
              />
            </div>
            <div className="bg-pilot-bg border border-pilot-border rounded p-3">
              <div className="flex items-start gap-2">
                <AlertTriangle className="w-4 h-4 text-pilot-warning shrink-0 mt-0.5" />
                <p className="text-xs text-pilot-muted">
                  Revoking a CR code deletes the Kubernetes secret from the{" "}
                  <code className="text-pilot-accent">kubepilot-security</code> namespace.
                  This action cannot be undone.
                </p>
              </div>
            </div>
            <button
              onClick={() => handleRevoke()}
              disabled={revLoading}
              className="w-full bg-pilot-danger text-white text-sm py-2 rounded font-bold hover:brightness-110 disabled:opacity-50 flex items-center justify-center gap-2"
            >
              {revLoading ? "Revoking…" : <><Trash2 className="w-4 h-4" /> Revoke</>}
            </button>
            {revResult && <ResultBanner {...revResult} />}
          </div>
        </div>
      </div>

      {/* ── Session History ─────────────────────────────── */}
      {entries.length > 0 && (
        <div className="bg-pilot-surface border border-pilot-border rounded-lg overflow-hidden">
          <div className="px-4 py-3 border-b border-pilot-border">
            <h3 className="text-sm font-bold text-white">Session Activity</h3>
            <p className="text-xs text-pilot-muted">CR codes registered or modified in this session</p>
          </div>
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-pilot-border text-left text-xs text-pilot-muted">
                <th className="px-4 py-2">Change ID</th>
                <th className="px-4 py-2">Registered</th>
                <th className="px-4 py-2">Expires</th>
                <th className="px-4 py-2">Status</th>
                <th className="px-4 py-2">Actions</th>
              </tr>
            </thead>
            <tbody>
              {entries.map((entry) => (
                <tr key={entry.changeId} className="border-b border-pilot-border last:border-0">
                  <td className="px-4 py-2 font-mono text-pilot-accent">{entry.changeId}</td>
                  <td className="px-4 py-2 text-pilot-muted text-xs">
                    {new Date(entry.registeredAt).toLocaleTimeString()}
                  </td>
                  <td className="px-4 py-2 text-xs">
                    {entry.expiresAt ? (
                      <span className="flex items-center gap-1 text-pilot-warning">
                        <Clock className="w-3 h-3" />
                        {new Date(entry.expiresAt).toLocaleString()}
                      </span>
                    ) : (
                      <span className="text-pilot-muted">No expiry</span>
                    )}
                  </td>
                  <td className="px-4 py-2">
                    <StatusBadge status={entry.status} />
                  </td>
                  <td className="px-4 py-2">
                    {entry.status !== "revoked" && (
                      <button
                        onClick={() => handleRevoke(entry.changeId)}
                        className="text-xs text-pilot-danger hover:text-red-400 flex items-center gap-1"
                      >
                        <Trash2 className="w-3 h-3" /> Revoke
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function StatusBadge({ status }: { status: CREntry["status"] }) {
  const styles = {
    active: "bg-pilot-success/20 text-pilot-success border-pilot-success/30",
    verified: "bg-pilot-accent/20 text-pilot-accent border-pilot-accent/30",
    revoked: "bg-pilot-danger/20 text-pilot-danger border-pilot-danger/30",
  };
  return (
    <span className={`text-xs font-bold px-2 py-0.5 rounded border ${styles[status]}`}>
      {status}
    </span>
  );
}

function ResultBanner({ ok, msg }: { ok: boolean; msg: string }) {
  return (
    <div
      className={`flex items-start gap-2 rounded p-3 ${
        ok ? "bg-emerald-950 border border-pilot-success/30" : "bg-red-950 border border-pilot-danger/30"
      }`}
    >
      {ok ? (
        <CheckCircle className="w-4 h-4 text-pilot-success shrink-0 mt-0.5" />
      ) : (
        <XCircle className="w-4 h-4 text-pilot-danger shrink-0 mt-0.5" />
      )}
      <p className={`text-xs ${ok ? "text-pilot-success" : "text-pilot-danger"}`}>{msg}</p>
    </div>
  );
}
