/**
 * ClusterStatusBar — the shared status cluster shown in page headers: the AI
 * model badge (e.g. "AI: llama3") and a live indicator. The cluster itself is
 * picked in the top bar (ClusterSwitcher), per tab.
 */
import { useQuery } from "@tanstack/react-query";
import { Brain } from "lucide-react";
import { getAIHealth } from "@/lib/api";

export function ClusterStatusBar() {
  const { data: aiHealth } = useQuery({
    queryKey: ["ai-health"],
    queryFn: getAIHealth,
    refetchInterval: 30_000,
  });

  return (
    <div className="flex flex-wrap items-center justify-end gap-2 min-w-0">
      <div
        className={`flex items-center gap-1.5 text-xs font-semibold px-2.5 py-1.5 rounded-full max-w-[10rem] ${
          aiHealth?.healthy
            ? "text-pilot-success bg-pilot-success/10"
            : "text-pilot-danger bg-pilot-danger/10"
        }`}
        title={
          aiHealth
            ? aiHealth.healthy
              ? `AI Model: ${aiHealth.model} (${aiHealth.latency_ms}ms)`
              : `AI Error: ${aiHealth.error || "unreachable"}`
            : "Checking AI..."
        }
      >
        <Brain className="w-3.5 h-3.5 shrink-0" />
        <span className="truncate">
          {aiHealth ? (aiHealth.healthy ? `AI: ${aiHealth.model}` : "AI: Offline") : "AI: …"}
        </span>
      </div>
      <div className="hidden sm:flex items-center gap-1.5 text-xs font-semibold text-pilot-success bg-pilot-success/10 px-2.5 py-1.5 rounded-full">
        <span className="relative flex h-2 w-2">
          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-pilot-success opacity-75" />
          <span className="relative inline-flex rounded-full h-2 w-2 bg-pilot-success" />
        </span>
        Live
      </div>
    </div>
  );
}
