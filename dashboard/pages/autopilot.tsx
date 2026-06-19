/**
 * Autopilot page — live cockpit for KubePilot's closed-loop self-healing:
 * policy, stats, the pause/resume kill switch, and the decision ledger.
 */
import { AutopilotPanel } from "@/components/AutopilotPanel";

export default function AutopilotPage() {
  return (
    <div className="min-h-screen bg-pilot-bg text-white p-6">
      <AutopilotPanel />
    </div>
  );
}
