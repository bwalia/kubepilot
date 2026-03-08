/**
 * KubePilot – Kubernetes Cockpit
 * Dashboard home: cluster health overview, AI command bar, crashing pods, nodes.
 */
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Activity, Cpu, AlertTriangle, Terminal, Layers, FileSearch, Network, Shield } from "lucide-react";
import {
  listCrashingPods,
  listNodes,
  listDeployments,
  listAnomalies,
  interpretCommand,
  type SuggestedAction,
} from "@/lib/api";
import { ClusterList } from "@/components/ClusterList";
import { PodTable } from "@/components/PodTable";
import { MetricsPanel } from "@/components/MetricsPanel";
import { CRCodeApproval } from "@/components/CRCodeApproval";
import { CRCodeManager } from "@/components/CRCodeManager";
import { JobScheduler } from "@/components/JobScheduler";
import { AnomalyTimeline } from "@/components/AnomalyTimeline";
import RCAPage from "@/pages/rca";
import TopologyPage from "@/pages/topology";

export default function DashboardHome() {
  const [command, setCommand] = useState("");
  const [aiActions, setAiActions] = useState<SuggestedAction[] | null>(null);
  const [aiLoading, setAiLoading] = useState(false);
  const [selectedAction, setSelectedAction] = useState<SuggestedAction | null>(null);
  const [crModalOpen, setCrModalOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<"overview" | "rca" | "topology" | "jobs" | "cr-codes">("overview");

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
    // Non-production actions: submit as job directly (future: optimistic execute).
  };

  const totalPods = deployments.reduce((acc, d) => acc + d.Replicas, 0);
  const readyNodes = nodes.filter((n) => n.Ready).length;
  const pressureNodes = nodes.filter(
    (n) => n.MemoryPressure || n.DiskPressure || n.PIDPressure
  ).length;

  return (
    <div className="min-h-screen bg-pilot-bg text-white font-mono">
      {/* ── Header ───────────────────────────────────────────────── */}
      <header className="border-b border-pilot-border px-6 py-4 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Layers className="text-pilot-accent w-6 h-6" />
          <span className="text-xl font-bold tracking-wide">KubePilot</span>
          <span className="text-xs text-pilot-muted bg-pilot-surface px-2 py-0.5 rounded">
            Kubernetes Cockpit
          </span>
        </div>
        <div className="flex items-center gap-2 text-sm text-pilot-muted">
          <Activity className="w-4 h-4 text-pilot-success" />
          <span>Live</span>
        </div>
      </header>

      {/* ── KPI Bar ──────────────────────────────────────────────── */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-4 px-6 py-4 border-b border-pilot-border">
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

      {/* ── AI Command Bar ───────────────────────────────────────── */}
      <div className="px-6 py-4 border-b border-pilot-border">
        <div className="flex gap-2 items-center">
          <Terminal className="text-pilot-accent w-5 h-5 shrink-0" />
          <input
            type="text"
            value={command}
            onChange={(e) => setCommand(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && handleAICommand()}
            placeholder='Try: "Fix CrashLoopBackOff pods in production" or "Scale api-server to 5 replicas"'
            className="flex-1 bg-pilot-surface border border-pilot-border rounded px-4 py-2 text-sm placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent"
          />
          <button
            onClick={handleAICommand}
            disabled={aiLoading}
            className="bg-pilot-accent hover:bg-blue-500 text-white px-5 py-2 rounded text-sm font-semibold disabled:opacity-50"
          >
            {aiLoading ? "Thinking…" : "Ask AI"}
          </button>
        </div>

        {/* AI suggested actions */}
        {aiActions && aiActions.length > 0 && (
          <div className="mt-3 space-y-2">
            <p className="text-xs text-pilot-muted">AI suggested actions:</p>
            {aiActions.map((action, i) => (
              <div
                key={i}
                className="flex items-start justify-between bg-pilot-surface border border-pilot-border rounded p-3 gap-4"
              >
                <div>
                  <span className="text-xs font-bold text-pilot-accent uppercase mr-2">
                    {action.type}
                  </span>
                  {action.namespace && (
                    <span className="text-xs text-pilot-muted mr-1">{action.namespace}/</span>
                  )}
                  {action.resource && (
                    <span className="text-xs text-white">{action.resource}</span>
                  )}
                  <p className="text-xs text-pilot-muted mt-1">{action.explanation}</p>
                </div>
                <button
                  onClick={() => handleActionClick(action)}
                  className={`shrink-0 text-xs px-3 py-1 rounded font-semibold ${
                    action.requires_cr_code
                      ? "bg-pilot-warning text-black"
                      : "bg-pilot-success text-black"
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
      <div className="flex gap-1 px-6 pt-4 border-b border-pilot-border">
        {(["overview", "rca", "topology", "jobs", "cr-codes"] as const).map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`px-4 py-2 text-sm capitalize rounded-t flex items-center gap-1.5 ${
              activeTab === tab
                ? "bg-pilot-surface border border-b-0 border-pilot-border text-white"
                : "text-pilot-muted hover:text-white"
            }`}
          >
            {tab === "rca" && <FileSearch className="w-3.5 h-3.5" />}
            {tab === "topology" && <Network className="w-3.5 h-3.5" />}
            {tab === "rca" ? "RCA" : tab}
            {tab === "rca" && anomalies.length > 0 && (
              <span className="bg-red-600 text-white text-[10px] px-1.5 py-0.5 rounded-full leading-none">
                {anomalies.length}
              </span>
            )}
          </button>
        ))}
      </div>

      {/* ── Main Content ─────────────────────────────────────────── */}
      <main className="px-6 py-6 space-y-6">
        {activeTab === "overview" && (
          <>
            <ClusterList nodes={nodes} loading={nodesLoading} />
            {crashingPods.length > 0 && (
              <section>
                <h2 className="text-sm font-bold text-pilot-danger mb-3 flex items-center gap-2">
                  <AlertTriangle className="w-4 h-4" /> Crashing Pods
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
    <div className="bg-pilot-surface border border-pilot-border rounded-lg p-4">
      <div
        className={`flex items-center gap-2 mb-1 ${alert ? "text-pilot-danger" : "text-pilot-accent"}`}
      >
        {icon}
        <span className="text-xs uppercase tracking-widest text-pilot-muted">{label}</span>
      </div>
      <div className={`text-2xl font-bold ${alert ? "text-pilot-danger" : "text-white"}`}>
        {value}
      </div>
    </div>
  );
}
