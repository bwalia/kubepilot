/**
 * KubePilot – Kubernetes Cockpit
 * Dashboard home: cluster health overview, AI command bar, crashing pods, nodes.
 */
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Activity, Cpu, AlertTriangle, Terminal, Layers, FileSearch, Network, Shield, FileWarning, CalendarClock, KeyRound, Brain } from "lucide-react";
import {
  listCrashingPods,
  listNodes,
  listDeployments,
  listAnomalies,
  listKubeconfigs,
  interpretCommand,
  getAIHealth,
  type SuggestedAction,
  type AIHealthStatus,
} from "@/lib/api";
import { ClusterList } from "@/components/ClusterList";
import { PodTable } from "@/components/PodTable";
import { MetricsPanel } from "@/components/MetricsPanel";
import { CRCodeApproval } from "@/components/CRCodeApproval";
import { CRCodeManager } from "@/components/CRCodeManager";
import { JobScheduler } from "@/components/JobScheduler";
import { AnomalyTimeline } from "@/components/AnomalyTimeline";
import { KubeconfigSwitcher } from "@/components/KubeconfigSwitcher";
import { ClusterEventsTroubleshooting } from "@/components/ClusterEventsTroubleshooting";
import RCAPage from "@/pages/rca";
import TopologyPage from "@/pages/topology";

const TABS = [
  { key: "overview" as const, label: "Overview", icon: Activity },
  { key: "rca" as const, label: "RCA", icon: FileSearch },
  { key: "topology" as const, label: "Topology", icon: Network },
  { key: "jobs" as const, label: "Jobs", icon: CalendarClock },
  { key: "events" as const, label: "Cluster Events", icon: FileWarning },
  { key: "cr-codes" as const, label: "CR Codes", icon: KeyRound },
];

type TabKey = typeof TABS[number]["key"];

export default function DashboardHome() {
  const [command, setCommand] = useState("");
  const [aiActions, setAiActions] = useState<SuggestedAction[] | null>(null);
  const [aiLoading, setAiLoading] = useState(false);
  const [selectedAction, setSelectedAction] = useState<SuggestedAction | null>(null);
  const [crModalOpen, setCrModalOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<TabKey>("overview");

  const { data: crashingPods = [], isLoading: podsLoading } = useQuery({
    queryKey: ["crashing-pods"],
    queryFn: () => listCrashingPods(),
  });

  const { data: nodes = [], isLoading: nodesLoading } = useQuery({
    queryKey: ["nodes"],
    queryFn: listNodes,
  });

  const { data: deployments = [] } = useQuery({
    queryKey: ["deployments"],
    queryFn: () => listDeployments(),
  });

  const { data: anomalies = [] } = useQuery({
    queryKey: ["anomalies-count"],
    queryFn: () => listAnomalies({ since: "1h" }),
    refetchInterval: 15_000,
  });

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

  const handleAICommand = async () => {
    if (!command.trim()) return;
    setAiLoading(true);
    setAiActions(null);
    try {
      const result = await interpretCommand(command);
      setAiActions(result.actions);
    } catch (err) {
      console.error("AI command failed:", err);
    } finally {
      setAiLoading(false);
    }
  };

  const handleActionClick = (action: SuggestedAction) => {
    if (action.requires_cr_code) {
      setSelectedAction(action);
      setCrModalOpen(true);
    }
  };

  const totalPods = deployments.reduce((acc, d) => acc + d.Replicas, 0);
  const readyNodes = nodes.filter((n) => n.Ready).length;
  const pressureNodes = nodes.filter(
    (n) => n.MemoryPressure || n.DiskPressure || n.PIDPressure
  ).length;
  const activeKubeconfig = kubeconfigs?.active_path || "";
  const activeKubeconfigBasename = activeKubeconfig
    ? activeKubeconfig.split(/[\\/]/).pop() || activeKubeconfig
    : "in-cluster";

  return (
    <div className="min-h-screen bg-pilot-bg text-white">
      {/* ── Header ───────────────────────────────────────────────── */}
      <header className="sticky top-0 z-40 bg-pilot-bg/90 backdrop-blur-md border-b border-pilot-border px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16">
          <div className="flex items-center gap-3">
            <div className="flex items-center justify-center w-9 h-9 rounded-lg bg-pilot-accent/10">
              <Layers className="text-pilot-accent w-5 h-5" />
            </div>
            <div>
              <span className="text-lg font-bold tracking-tight">KubePilot</span>
              <span className="hidden sm:inline text-xs text-pilot-muted ml-2 bg-pilot-surface px-2 py-0.5 rounded-md font-medium">
                Kubernetes Cockpit
              </span>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <KubeconfigSwitcher
              onSwitched={() => {
                setAiActions(null);
                setActiveTab("overview");
              }}
            />
            <span
              className="hidden sm:inline text-xs bg-pilot-surface border border-pilot-border text-pilot-muted px-2.5 py-1 rounded-md font-mono max-w-56 truncate"
              title={activeKubeconfig || "in-cluster"}
            >
              {activeKubeconfigBasename}
            </span>
            <div
              className={`flex items-center gap-1.5 text-xs font-medium px-2.5 py-1 rounded-full ${
                aiHealth?.healthy
                  ? "text-pilot-success bg-emerald-500/10"
                  : "text-pilot-danger bg-red-500/10"
              }`}
              title={
                aiHealth
                  ? aiHealth.healthy
                    ? `AI Model: ${aiHealth.model} (${aiHealth.latency_ms}ms)`
                    : `AI Error: ${aiHealth.error || "unreachable"}`
                  : "Checking AI..."
              }
            >
              <Brain className="w-3.5 h-3.5" />
              {aiHealth ? (aiHealth.healthy ? `AI: ${aiHealth.model}` : "AI: Offline") : "AI: ..."}
            </div>
            <div className="flex items-center gap-1.5 text-xs font-medium text-pilot-success bg-emerald-500/10 px-2.5 py-1 rounded-full">
              <span className="relative flex h-2 w-2">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-pilot-success opacity-75" />
                <span className="relative inline-flex rounded-full h-2 w-2 bg-pilot-success" />
              </span>
              Live
            </div>
          </div>
        </div>
      </header>

      {/* ── KPI Bar ──────────────────────────────────────────────── */}
      <div className="px-4 sm:px-6 lg:px-8 py-5 border-b border-pilot-border bg-pilot-surface/30">
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3 sm:gap-4">
          <KPICard label="Total Pods" value={totalPods} icon={<Cpu className="w-5 h-5" />} />
          <KPICard
            label="Crashing Pods"
            value={crashingPods.length}
            icon={<AlertTriangle className="w-5 h-5" />}
            alert={crashingPods.length > 0}
          />
          <KPICard label="Nodes Ready" value={`${readyNodes}/${nodes.length}`} icon={<Activity className="w-5 h-5" />} />
          <KPICard
            label="Node Pressure"
            value={pressureNodes}
            icon={<AlertTriangle className="w-5 h-5" />}
            alert={pressureNodes > 0}
          />
          <KPICard
            label="Anomalies (1h)"
            value={anomalies.length}
            icon={<Shield className="w-5 h-5" />}
            alert={anomalies.length > 0}
          />
        </div>
      </div>

      {/* ── AI Command Bar ───────────────────────────────────────── */}
      <div className="px-4 sm:px-6 lg:px-8 py-4 border-b border-pilot-border">
        <div className="flex gap-3 items-center">
          <div className="flex items-center justify-center w-9 h-9 rounded-lg bg-pilot-accent/10 shrink-0">
            <Terminal className="text-pilot-accent w-4 h-4" />
          </div>
          <input
            type="text"
            value={command}
            onChange={(e) => setCommand(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && handleAICommand()}
            placeholder='Try: "Fix CrashLoopBackOff pods in production" or "Scale api-server to 5 replicas"'
            className="flex-1 bg-pilot-surface border border-pilot-border rounded-lg px-4 py-2.5 text-sm placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent focus:ring-1 focus:ring-pilot-accent/30"
          />
          <button
            onClick={handleAICommand}
            disabled={aiLoading}
            className="bg-pilot-accent hover:bg-blue-500 text-white px-6 py-2.5 rounded-lg text-sm font-semibold disabled:opacity-50 shadow-glow-blue whitespace-nowrap"
          >
            {aiLoading ? "Thinking..." : "Ask AI"}
          </button>
        </div>

        {/* AI suggested actions */}
        {aiActions && aiActions.length > 0 && (
          <div className="mt-4 space-y-2 animate-fade-in">
            <p className="text-xs font-medium text-pilot-muted uppercase tracking-wider">AI Suggested Actions</p>
            {aiActions.map((action, i) => (
              <div
                key={i}
                className="flex items-start justify-between bg-pilot-surface border border-pilot-border rounded-lg p-4 gap-4 hover:border-pilot-border-hover"
              >
                <div className="min-w-0">
                  <span className="text-xs font-bold text-pilot-accent uppercase mr-2">
                    {action.type}
                  </span>
                  {action.namespace && (
                    <span className="text-xs text-pilot-muted mr-1">{action.namespace}/</span>
                  )}
                  {action.resource && (
                    <span className="text-sm text-white font-mono">{action.resource}</span>
                  )}
                  <p className="text-sm text-pilot-text-secondary mt-1.5 leading-relaxed">{action.explanation}</p>
                </div>
                <button
                  onClick={() => handleActionClick(action)}
                  className={`shrink-0 text-xs px-4 py-2 rounded-lg font-semibold ${
                    action.requires_cr_code
                      ? "bg-pilot-warning text-black hover:brightness-110"
                      : "bg-pilot-success text-black hover:brightness-110"
                  }`}
                >
                  {action.requires_cr_code ? "Authorize & Run" : "Execute"}
                </button>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* ── Tab Bar ──────────────────────────────────────────────── */}
      <div className="px-4 sm:px-6 lg:px-8 border-b border-pilot-border overflow-x-auto">
        <nav className="flex gap-1 -mb-px" role="tablist">
          {TABS.map((tab) => {
            const Icon = tab.icon;
            const isActive = activeTab === tab.key;
            return (
              <button
                key={tab.key}
                onClick={() => setActiveTab(tab.key)}
                role="tab"
                aria-selected={isActive}
                className={`flex items-center gap-2 px-4 py-3 text-sm font-medium border-b-2 whitespace-nowrap transition-colors ${
                  isActive
                    ? "border-pilot-accent text-white"
                    : "border-transparent text-pilot-muted hover:text-pilot-text-secondary hover:border-pilot-border"
                }`}
              >
                <Icon className="w-4 h-4" />
                {tab.label}
                {tab.key === "rca" && anomalies.length > 0 && (
                  <span className="bg-red-600 text-white text-2xs px-1.5 py-0.5 rounded-full leading-none font-bold min-w-[1.25rem] text-center">
                    {anomalies.length}
                  </span>
                )}
              </button>
            );
          })}
        </nav>
      </div>

      {/* ── Main Content ─────────────────────────────────────────── */}
      <main className="px-4 sm:px-6 lg:px-8 py-6 space-y-6 animate-fade-in">
        {activeTab === "overview" && (
          <>
            <ClusterList nodes={nodes} loading={nodesLoading} />
            {crashingPods.length > 0 && (
              <section>
                <h2 className="text-base font-bold text-pilot-danger mb-4 flex items-center gap-2">
                  <AlertTriangle className="w-5 h-5" /> Crashing Pods
                </h2>
                <PodTable pods={crashingPods} loading={podsLoading} />
              </section>
            )}
            <MetricsPanel deployments={deployments} />
          </>
        )}
        {activeTab === "rca" && <RCAPage />}
        {activeTab === "topology" && <TopologyPage />}
        {activeTab === "jobs" && <JobScheduler />}
        {activeTab === "events" && <ClusterEventsTroubleshooting />}
        {activeTab === "cr-codes" && <CRCodeManager />}
      </main>

      {/* ── CR Code Modal ────────────────────────────────────────── */}
      {crModalOpen && selectedAction && (
        <CRCodeApproval
          action={selectedAction}
          onClose={() => setCrModalOpen(false)}
          onAuthorized={() => {
            setCrModalOpen(false);
            setSelectedAction(null);
          }}
        />
      )}
    </div>
  );
}

function KPICard({
  label,
  value,
  icon,
  alert,
}: {
  label: string;
  value: string | number;
  icon: React.ReactNode;
  alert?: boolean;
}) {
  return (
    <div className={`bg-pilot-surface border rounded-xl p-4 shadow-card transition-all hover:shadow-card-hover ${
      alert ? "border-pilot-danger/40" : "border-pilot-border"
    }`}>
      <div className={`flex items-center gap-2 mb-2 ${alert ? "text-pilot-danger" : "text-pilot-accent"}`}>
        {icon}
        <span className="text-xs uppercase tracking-wider text-pilot-muted font-medium">{label}</span>
      </div>
      <div className={`text-3xl font-bold tracking-tight ${alert ? "text-pilot-danger" : "text-white"}`}>
        {value}
      </div>
    </div>
  );
}
