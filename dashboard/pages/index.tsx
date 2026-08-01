/**
 * KubePilot — Kubernetes Troubleshooting powered by AI
 * Dashboard home: cluster health overview, AI command bar, crashing pods, nodes.
 */
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Activity, Cpu, AlertTriangle, Terminal, FileSearch, Network, Shield, FileWarning, CalendarClock, KeyRound, BookOpen, Layers, Lock, CheckCircle } from "lucide-react";
import {
  listCrashingPods,
  listNodes,
  listDeployments,
  listAnomalies,
  listNamespaces,
  interpretCommand,
  getServerConfig,
  type SuggestedAction,
} from "@/lib/api";
import { useNamespaceLock } from "@/lib/useNamespaceLock";
import { StaggerGroup, StaggerCard } from "@/components/motion";
import { ClusterList } from "@/components/ClusterList";
import { PodTable } from "@/components/PodTable";
import { MetricsPanel } from "@/components/MetricsPanel";
import { CRCodeApproval } from "@/components/CRCodeApproval";
import { CRCodeManager } from "@/components/CRCodeManager";
import { JobScheduler } from "@/components/JobScheduler";
import { AnomalyTimeline } from "@/components/AnomalyTimeline";
import { RunbooksPanel } from "@/components/RunbooksPanel";
import { useSessionState } from "@/lib/useSessionState";
import { ResourceMeters } from "@/components/dashboard/ResourceMeters";
import { ClusterStatusBar } from "@/components/ClusterStatusBar";
import { ClusterEventsTroubleshooting } from "@/components/ClusterEventsTroubleshooting";
import { PortForwardSessionsPanel } from "@/components/PortForwardSessionsPanel";
import RCAPage from "@/pages/rca";
import TopologyPage from "@/pages/topology";

const TABS = [
  { key: "overview" as const, label: "Overview", icon: Activity },
  { key: "rca" as const, label: "RCA", icon: FileSearch },
  { key: "runbooks" as const, label: "Runbooks", icon: BookOpen },
  { key: "topology" as const, label: "Topology", icon: Network },
  { key: "jobs" as const, label: "Jobs", icon: CalendarClock },
  { key: "events" as const, label: "Cluster Events", icon: FileWarning },
  { key: "cr-codes" as const, label: "CR Codes", icon: KeyRound },
];

type TabKey = typeof TABS[number]["key"];

// Sentinel for the explicit "look across every namespace" choice, kept distinct
// from "" which here means "nothing selected yet" (prompt the user first).
const ALL_NAMESPACES = "__all__";

export default function DashboardHome() {
  const [command, setCommand] = useState("");
  const [aiActions, setAiActions] = useState<SuggestedAction[] | null>(null);
  const [aiLoading, setAiLoading] = useState(false);
  const [selectedAction, setSelectedAction] = useState<SuggestedAction | null>(null);
  const [crModalOpen, setCrModalOpen] = useState(false);
  const [activeTabRaw, setActiveTabRaw] = useSessionState("kubepilot-home-tab", "overview");
  const activeTab = activeTabRaw as TabKey;
  const setActiveTab = (t: TabKey) => setActiveTabRaw(t);

  // Crashing pods are scoped to a namespace the user must pick first — we do not
  // fan out across the whole cluster by default. A URL ?namespace= lock wins.
  const { locked, namespace: lockedNamespace } = useNamespaceLock();
  const [selectedNamespace, setSelectedNamespace] = useSessionState("kubepilot-home-ns", "");
  const nsChosen = locked || selectedNamespace !== "";
  const effectiveNamespace = locked ? lockedNamespace! : selectedNamespace;
  // Backend treats "" as "all namespaces"; map our explicit ALL sentinel to it.
  const nsForQuery = effectiveNamespace === ALL_NAMESPACES ? "" : effectiveNamespace;

  const { data: namespaces = [] } = useQuery({
    queryKey: ["namespaces"],
    queryFn: listNamespaces,
  });

  const { data: crashingPods = [], isLoading: podsLoading } = useQuery({
    queryKey: ["crashing-pods", nsForQuery],
    queryFn: () => listCrashingPods(nsForQuery),
    enabled: nsChosen,
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

  const { data: serverConfig } = useQuery({
    queryKey: ["server-config"],
    queryFn: getServerConfig,
    staleTime: 60_000,
    refetchInterval: false,
  });
  const mutationsEnabled = serverConfig?.mutations_enabled ?? false;

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

  return (
    <div className="min-h-screen bg-pilot-bg text-pilot-text-primary">
      {/* ── Page header ──────────────────────────────────────────── */}
      <header className="bg-pilot-surface border-b border-pilot-border px-4 sm:px-6 lg:px-8 py-4">
        <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-3">
          <div className="min-w-0">
            <h1 className="font-display text-2xl font-bold tracking-tight text-pilot-text-primary">Kubernetes CoPilot (AI Troubleshooting Mode)</h1>
            <p className="text-sm text-pilot-muted mt-1">Ask in plain English — KubePilot diagnoses issues and proposes fixes.</p>
          </div>
          <div className="flex flex-wrap items-center gap-x-4 gap-y-3">
            <NamespacePicker
              locked={locked}
              lockedNamespace={lockedNamespace}
              value={selectedNamespace}
              namespaces={namespaces}
              onChange={setSelectedNamespace}
            />
            <ClusterStatusBar
              onSwitched={() => {
                setAiActions(null);
                setActiveTab("overview");
              }}
            />
          </div>
        </div>
      </header>

      {/* ── Cluster Resource Charts ─────────────────────────────── */}
      <div className="px-4 sm:px-6 lg:px-8 py-5 border-b border-pilot-border bg-pilot-surface/30">
        <ResourceMeters />
      </div>

      {/* ── KPI Bar ──────────────────────────────────────────────── */}
      <div className="px-4 sm:px-6 lg:px-8 py-5 border-b border-pilot-border bg-pilot-surface/30">
        <StaggerGroup className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3 sm:gap-4">
          <StaggerCard>
            <KPICard label="Total Pods" value={totalPods} icon={<Cpu className="w-5 h-5" />} />
          </StaggerCard>
          <StaggerCard>
            <KPICard
              label="Crashing Pods"
              value={nsChosen ? crashingPods.length : "—"}
              icon={<AlertTriangle className="w-5 h-5" />}
              alert={nsChosen && crashingPods.length > 0}
            />
          </StaggerCard>
          <StaggerCard>
            <KPICard label="Nodes Ready" value={`${readyNodes}/${nodes.length}`} icon={<Activity className="w-5 h-5" />} />
          </StaggerCard>
          <StaggerCard>
            <KPICard
              label="Node Pressure"
              value={pressureNodes}
              icon={<AlertTriangle className="w-5 h-5" />}
              alert={pressureNodes > 0}
            />
          </StaggerCard>
          <StaggerCard>
            <KPICard
              label="Anomalies (1h)"
              value={anomalies.length}
              icon={<Shield className="w-5 h-5" />}
              alert={anomalies.length > 0}
            />
          </StaggerCard>
        </StaggerGroup>
      </div>

      {/* ── AI Command Bar ───────────────────────────────────────── */}
      <div className="px-4 sm:px-6 lg:px-8 py-4 border-b border-pilot-border">
        <div className="flex gap-3 items-center">
          <div className="flex items-center justify-center w-9 h-9 rounded-lg bg-pilot-accent/12 border border-pilot-accent/25 shrink-0">
            <Terminal className="text-pilot-accent w-4 h-4" />
          </div>
          <input
            type="text"
            value={command}
            onChange={(e) => setCommand(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && handleAICommand()}
            placeholder='Try: "Fix CrashLoopBackOff pods in production" or "Scale api-server to 5 replicas"'
            className="flex-1 bg-pilot-surface border border-pilot-border rounded-lg px-4 py-3 text-base text-pilot-text-primary placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent/60 focus:ring-2 focus:ring-pilot-accent/25"
          />
          <button
            onClick={handleAICommand}
            disabled={aiLoading}
            className="bg-pilot-accent hover:bg-pilot-accent-light text-pilot-bg px-6 py-3 rounded-lg text-base font-semibold disabled:opacity-50 hover:shadow-glow-blue whitespace-nowrap transition-all"
          >
            {aiLoading ? "Thinking…" : "Ask AI"}
          </button>
        </div>

        {/* AI suggested actions */}
        {aiActions && aiActions.length > 0 && (
          <div className="mt-4 space-y-2 animate-fade-in">
            <p className="eyebrow">AI Suggested Actions</p>
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
                    <span className="text-sm text-pilot-text-primary font-mono">{action.resource}</span>
                  )}
                  <p className="text-sm text-pilot-text-secondary mt-1.5 leading-relaxed">{action.explanation}</p>
                </div>
                <button
                  onClick={() => handleActionClick(action)}
                  className={`shrink-0 text-xs px-4 py-2 rounded-lg font-semibold ${
                    action.requires_cr_code
                      ? "bg-pilot-warning text-pilot-bg hover:brightness-110"
                      : "bg-pilot-success text-pilot-bg hover:brightness-110"
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
                className={`flex items-center gap-2.5 px-4 py-3.5 text-[0.95rem] font-semibold border-b-2 whitespace-nowrap transition-colors ${
                  isActive
                    ? "border-pilot-accent text-pilot-text-primary [text-shadow:0_0_12px_rgba(34,211,238,0.35)]"
                    : "border-transparent text-pilot-muted hover:text-pilot-text-secondary hover:border-pilot-border-hover"
                }`}
              >
                <Icon className="w-[1.15rem] h-[1.15rem]" />
                {tab.label}
                {tab.key === "rca" && anomalies.length > 0 && (
                  <span className="bg-pilot-danger text-pilot-text-primary text-2xs px-1.5 py-0.5 rounded-full leading-none font-bold min-w-[1.25rem] text-center">
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
            <section>
              <h2 className="text-base font-bold text-pilot-text-primary mb-4 flex items-center gap-2">
                <AlertTriangle className={`w-5 h-5 ${nsChosen && crashingPods.length > 0 ? "text-pilot-danger" : "text-pilot-muted"}`} />
                Crashing Pods
                {nsChosen && (
                  <span className="text-sm font-medium text-pilot-muted font-mono">
                    · {nsForQuery === "" ? "all namespaces" : nsForQuery}
                  </span>
                )}
              </h2>
              {!nsChosen ? (
                <NamespacePrompt
                  namespaces={namespaces}
                  onChange={setSelectedNamespace}
                />
              ) : podsLoading ? (
                <div className="space-y-2">
                  {[...Array(3)].map((_, i) => (
                    <div key={i} className="h-14 bg-pilot-surface rounded-xl animate-pulse" />
                  ))}
                </div>
              ) : crashingPods.length > 0 ? (
                <PodTable pods={crashingPods} loading={false} mutationsEnabled={mutationsEnabled} />
              ) : (
                <div className="flex items-center gap-3 bg-pilot-surface border border-pilot-border rounded-xl p-5 text-sm">
                  <CheckCircle className="w-5 h-5 text-pilot-success shrink-0" />
                  <span className="text-pilot-text-secondary">
                    No crashing pods in{" "}
                    <span className="font-mono text-pilot-text-primary">
                      {nsForQuery === "" ? "any namespace" : nsForQuery}
                    </span>
                    .
                  </span>
                </div>
              )}
            </section>
            <PortForwardSessionsPanel mutationsEnabled={mutationsEnabled} />
            <MetricsPanel deployments={deployments} />
          </>
        )}
        {activeTab === "rca" && <RCAPage />}
        {activeTab === "runbooks" && <RunbooksPanel />}
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

/** Header namespace control. Shows a locked badge when the URL pins a namespace,
 *  otherwise a dropdown whose blank default forces an explicit choice. */
function NamespacePicker({
  locked,
  lockedNamespace,
  value,
  namespaces,
  onChange,
}: {
  locked: boolean;
  lockedNamespace: string | null;
  value: string;
  namespaces: { Name: string }[];
  onChange: (ns: string) => void;
}) {
  return (
    <div className="flex items-center gap-2">
      <span className="eyebrow flex items-center gap-1.5">
        <Layers className="w-3.5 h-3.5" /> Namespace
      </span>
      {locked ? (
        <span
          className="inline-flex items-center gap-1.5 bg-pilot-accent/15 text-pilot-accent-light border border-pilot-accent/40 rounded-lg px-3 py-2 text-sm font-medium"
          title="Locked to this namespace via URL parameter"
        >
          <Lock className="w-4 h-4" />
          {lockedNamespace}
        </span>
      ) : (
        <select
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="bg-pilot-surface border border-pilot-border rounded-lg px-3 py-2 text-sm text-pilot-text-primary min-w-44 focus:outline-none focus:border-pilot-accent/60 focus:ring-2 focus:ring-pilot-accent/25"
        >
          <option value="">Select namespace…</option>
          <option value={ALL_NAMESPACES}>All Namespaces</option>
          {namespaces.map((ns) => (
            <option key={ns.Name} value={ns.Name}>
              {ns.Name}
            </option>
          ))}
        </select>
      )}
    </div>
  );
}

/** Empty-state shown in place of the crashing-pods table until a namespace is
 *  chosen, so we never fan out across the whole cluster unprompted. */
function NamespacePrompt({
  namespaces,
  onChange,
}: {
  namespaces: { Name: string }[];
  onChange: (ns: string) => void;
}) {
  return (
    <div className="bg-pilot-surface border border-dashed border-pilot-border rounded-xl p-8 text-center">
      <div className="inline-flex items-center justify-center w-12 h-12 rounded-xl bg-pilot-accent/12 text-pilot-accent mb-4">
        <Layers className="w-6 h-6" />
      </div>
      <p className="text-base font-semibold text-pilot-text-primary">Select a namespace</p>
      <p className="text-sm text-pilot-muted mt-1.5 max-w-md mx-auto">
        Choose a namespace to scan for crashing pods. KubePilot won&rsquo;t scan every
        namespace at once unless you ask it to.
      </p>
      <select
        value=""
        onChange={(e) => onChange(e.target.value)}
        className="mt-5 bg-pilot-surface border border-pilot-border rounded-lg px-3 py-2 text-sm text-pilot-text-primary min-w-52 focus:outline-none focus:border-pilot-accent/60 focus:ring-2 focus:ring-pilot-accent/25"
      >
        <option value="" disabled>Select namespace…</option>
        <option value={ALL_NAMESPACES}>All Namespaces</option>
        {namespaces.map((ns) => (
          <option key={ns.Name} value={ns.Name}>
            {ns.Name}
          </option>
        ))}
      </select>
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
    <div className={`h-full bg-pilot-surface border rounded-2xl p-5 shadow-card transition-shadow hover:shadow-card-hover ${
      alert ? "border-pilot-danger/40" : "border-pilot-border"
    }`}>
      <div className="flex items-center gap-3 mb-3.5">
        <span className={`inline-flex items-center justify-center w-10 h-10 rounded-xl ${
          alert ? "bg-pilot-danger/12 text-pilot-danger" : "bg-pilot-accent/12 text-pilot-accent"
        }`}>
          {icon}
        </span>
        <span className="eyebrow">{label}</span>
      </div>
      <div className={`font-display text-[2.6rem] leading-none font-bold tracking-tight tabular-nums ${alert ? "text-pilot-danger" : "text-pilot-text-primary"}`}>
        {value}
      </div>
    </div>
  );
}
