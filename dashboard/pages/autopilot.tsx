/**
 * Autopilot page — live cockpit for KubePilot's closed-loop self-healing:
 * policy, stats, the pause/resume kill switch, and the decision ledger.
 */
import { AutopilotPanel } from "@/components/AutopilotPanel";
import { ClusterStatusBar } from "@/components/ClusterStatusBar";

export default function AutopilotPage() {
  return (
    <div className="min-h-screen bg-pilot-bg text-pilot-text-primary">
      {/* ── Page header ──────────────────────────────────────────── */}
      <header className="bg-pilot-surface border-b border-pilot-border px-4 sm:px-6 lg:px-8 py-4">
        <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-3">
          <div className="min-w-0">
            <h1 className="font-display text-2xl font-bold tracking-tight text-pilot-text-primary">Kubernetes AutoPilot (AI Fix Mode)</h1>
            <p className="text-sm text-pilot-muted mt-1">Closed-loop self-healing — policy, kill switch, and the decision ledger.</p>
          </div>
          <ClusterStatusBar />
        </div>
      </header>

      <main className="px-4 sm:px-6 lg:px-8 py-6">
        <AutopilotPanel />
      </main>
    </div>
  );
}
