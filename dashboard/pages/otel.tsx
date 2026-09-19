/**
 * OTLP Autopilot page — automatic observability engine coverage +
 * built-in Metrics / Logs / Traces overview (cluster + application planes).
 */
import { OTELAutopilotPanel } from "@/components/OTELAutopilotPanel";
import { ClusterStatusBar } from "@/components/ClusterStatusBar";

export default function OTELAutopilotPage() {
  return (
    <div className="min-h-screen bg-pilot-bg text-pilot-text-primary">
      <header className="bg-pilot-surface border-b border-pilot-border px-4 sm:px-6 lg:px-8 py-4">
        <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-3">
          <div className="min-w-0">
            <h1 className="font-display text-2xl font-bold tracking-tight text-pilot-text-primary">
              OTLP AutoPilot
            </h1>
            <p className="text-sm text-pilot-muted mt-1">
              Deploy anything. Get observability automatically — Logs, Metrics, and Traces without code changes.
            </p>
          </div>
          <ClusterStatusBar />
        </div>
      </header>

      <main className="px-4 sm:px-6 lg:px-8 py-6">
        <OTELAutopilotPanel />
      </main>
    </div>
  );
}
