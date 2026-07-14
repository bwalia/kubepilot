/**
 * Autopilot page — live cockpit for KubePilot's closed-loop self-healing:
 * policy, stats, the pause/resume kill switch, and the decision ledger.
 */
import { Layers } from "lucide-react";
import { AutopilotPanel } from "@/components/AutopilotPanel";
import { ClusterStatusBar } from "@/components/ClusterStatusBar";

export default function AutopilotPage() {
  return (
    <div className="min-h-screen bg-pilot-bg text-pilot-text-primary">
      {/* ── Header ───────────────────────────────────────────────── */}
      <header className="sticky top-0 z-40 bg-pilot-bg/90 backdrop-blur-md border-b border-pilot-border px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16">
          <div className="flex items-center gap-3">
            <div className="flex items-center justify-center w-9 h-9 rounded-lg bg-pilot-accent/10">
              <Layers className="text-pilot-accent w-5 h-5" />
            </div>
            <div>
              <span className="text-lg font-display font-bold tracking-tight text-pilot-text-primary">KubePilot</span>
              <span className="hidden sm:inline text-xs text-pilot-muted ml-2 bg-pilot-surface px-2 py-0.5 rounded-md font-medium">
                Autopilot
              </span>
            </div>
          </div>
          <ClusterStatusBar />
        </div>
      </header>

      <main className="p-6">
        <AutopilotPanel />
      </main>
    </div>
  );
}
