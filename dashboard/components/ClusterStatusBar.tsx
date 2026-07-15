/**
 * ClusterStatusBar — the shared cluster/AI status cluster shown in page headers:
 * the kubeconfig switcher (active context, e.g. "default"), the active kubeconfig
 * basename (e.g. "k3s0-config.yaml"), the AI model badge (e.g. "AI: llama3"), and
 * a live indicator. Rendered on both the home page and the Autopilot page so the
 * cockpit shows the same context everywhere.
 */
import { useQuery } from "@tanstack/react-query";
import { Brain } from "lucide-react";
import { listKubeconfigs, getAIHealth } from "@/lib/api";
import { KubeconfigSwitcher } from "@/components/KubeconfigSwitcher";

interface Props {
  onSwitched?: () => void;
}

export function ClusterStatusBar({ onSwitched }: Props) {
  const { data: kubeconfigs } = useQuery({
    queryKey: ["kubeconfigs"],
    queryFn: listKubeconfigs,
    refetchInterval: 15_000,
  });

  const { data: aiHealth } = useQuery({
    queryKey: ["ai-health"],
    queryFn: getAIHealth,
    refetchInterval: 30_000,
  });

  const activeKubeconfig = kubeconfigs?.active_path || "";
  const activeKubeconfigBasename = activeKubeconfig
    ? activeKubeconfig.split(/[\\/]/).pop() || activeKubeconfig
    : "in-cluster";

  return (
    <div className="flex flex-wrap items-center justify-end gap-2 min-w-0">
      <KubeconfigSwitcher onSwitched={onSwitched} />
      <span
        className="hidden lg:inline text-xs bg-pilot-surface border border-pilot-border text-pilot-muted px-2.5 py-1.5 rounded-lg font-mono max-w-[13rem] truncate"
        title={activeKubeconfig || "in-cluster"}
      >
        {activeKubeconfigBasename}
      </span>
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
