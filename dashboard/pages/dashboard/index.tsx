/**
 * Kubernetes Dashboard — a Lens/Rancher-style read-only cluster browser.
 * Complements the AI Troubleshooting homepage. All navigation is client-side
 * state (no dynamic file-based routes) so the page is compatible with Next.js
 * static export (output: "export").
 */
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  listNamespaces,
  getServerConfig,
  getResourceYAML,
  executeSuggestedAction,
  type SuggestedAction,
} from "@/lib/api";
import { useNamespaceLock } from "@/lib/useNamespaceLock";
import { KubeconfigSwitcher } from "@/components/KubeconfigSwitcher";
import { LogViewer } from "@/components/LogViewer";
import { CRCodeApproval } from "@/components/CRCodeApproval";
import { OverviewSection } from "@/components/dashboard/OverviewSection";
import { WorkloadsSection } from "@/components/dashboard/WorkloadsSection";
import { NetworkSection } from "@/components/dashboard/NetworkSection";
import { ConfigSection } from "@/components/dashboard/ConfigSection";
import { ClusterHealthSection } from "@/components/dashboard/ClusterHealthSection";
import { EventsSection } from "@/components/dashboard/EventsSection";
import { TopologySection } from "@/components/dashboard/TopologySection";
import { PodDetailDrawer } from "@/components/dashboard/PodDetailDrawer";
import { PortForwardSessionsPanel } from "@/components/PortForwardSessionsPanel";
import { Dialog, DrawerContent } from "@/components/ui/dialog";
import { LayoutDashboard, Boxes, Network, Database, HeartPulse, FileWarning, Share2, X, Lock } from "lucide-react";

type Section = "overview" | "workloads" | "network" | "config" | "topology" | "health" | "events";

const SECTIONS: { key: Section; label: string; icon: typeof Boxes }[] = [
  { key: "overview", label: "Overview", icon: LayoutDashboard },
  { key: "workloads", label: "Workloads", icon: Boxes },
  { key: "network", label: "Network", icon: Network },
  { key: "config", label: "Config & Storage", icon: Database },
  { key: "topology", label: "Topology", icon: Share2 },
  { key: "health", label: "Cluster Health", icon: HeartPulse },
  { key: "events", label: "Events", icon: FileWarning },
];

interface YAMLTarget {
  kind: string;
  namespace: string;
  name: string;
}

export default function KubernetesDashboard() {
  const { locked, namespace: lockedNamespace } = useNamespaceLock();
  const [selectedNamespace, setSelectedNamespace] = useState("");
  // When the URL locks a namespace, it overrides the dropdown selection.
  const namespace = locked ? lockedNamespace! : selectedNamespace;
  const [section, setSection] = useState<Section>("overview");
  const [selectedPod, setSelectedPod] = useState<{ namespace: string; name: string } | null>(null);
  const [yamlTarget, setYamlTarget] = useState<YAMLTarget | null>(null);
  const [crAction, setCrAction] = useState<SuggestedAction | null>(null);

  const { data: namespaces = [] } = useQuery({
    queryKey: ["namespaces"],
    queryFn: listNamespaces,
  });

  const { data: serverConfig } = useQuery({
    queryKey: ["server-config"],
    queryFn: getServerConfig,
    staleTime: 60_000,
    refetchInterval: false,
  });
  const mutationsEnabled = serverConfig?.mutations_enabled ?? false;

  const handleAuthorizeAction = async (action: SuggestedAction) => {
    if (action.requires_cr_code) {
      setCrAction(action);
      return;
    }
    try {
      await executeSuggestedAction(action);
    } catch (err) {
      console.error("action execution failed:", err);
    }
  };

  return (
    <div className="min-h-screen bg-pilot-bg text-pilot-text-primary">
      {/* Header */}
      <header className="border-b border-pilot-border px-4 sm:px-6 lg:px-8 py-4 flex items-center justify-between">
        <div>
          <h1 className="font-display text-xl font-bold tracking-tight text-pilot-text-primary">Kubernetes Dashboard</h1>
          <p className="text-[13px] text-pilot-muted mt-0.5">Read-only cluster resource browser</p>
        </div>
        <KubeconfigSwitcher onSwitched={() => setSelectedPod(null)} />
      </header>

      {/* Namespace selector + section tabs */}
      <div className="px-4 sm:px-6 lg:px-8 py-3 border-b border-pilot-border flex flex-wrap items-center gap-3">
        <div className="flex items-center gap-2">
          <span className="eyebrow">Namespace</span>
          {locked ? (
            <span
              className="inline-flex items-center gap-1.5 bg-pilot-accent/15 text-pilot-accent-light border border-pilot-accent/40 rounded-lg px-3 py-1.5 text-sm font-medium min-w-44"
              title="Locked to this namespace via URL parameter"
            >
              <Lock className="w-3.5 h-3.5" />
              {namespace}
            </span>
          ) : (
            <select
              value={selectedNamespace}
              onChange={(e) => setSelectedNamespace(e.target.value)}
              className="bg-pilot-surface border border-pilot-border rounded-lg px-3 py-1.5 text-sm text-pilot-text-primary min-w-44"
            >
              <option value="">All Namespaces</option>
              {namespaces.map((ns) => (
                <option key={ns.Name} value={ns.Name}>
                  {ns.Name}
                </option>
              ))}
            </select>
          )}
        </div>

        <nav className="flex gap-1 overflow-x-auto" role="tablist">
          {SECTIONS.map((s) => {
            const Icon = s.icon;
            const active = section === s.key;
            return (
              <button
                key={s.key}
                onClick={() => setSection(s.key)}
                role="tab"
                aria-selected={active}
                className={`inline-flex items-center gap-2 px-3 py-1.5 text-[13px] font-medium rounded-lg whitespace-nowrap transition-colors ${
                  active
                    ? "bg-pilot-accent/12 text-pilot-accent-light border border-pilot-accent/30 shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]"
                    : "text-pilot-muted hover:text-pilot-text-primary hover:bg-white/[0.03] border border-transparent"
                }`}
              >
                <Icon className="w-4 h-4" />
                {s.label}
              </button>
            );
          })}
        </nav>
      </div>

      {/* Section content */}
      <main className="px-4 sm:px-6 lg:px-8 py-6 animate-fade-in">
        {section === "overview" && <OverviewSection namespace={namespace} />}
        {section === "workloads" && (
          <WorkloadsSection
            namespace={namespace}
            onSelectPod={(ns, name) => setSelectedPod({ namespace: ns, name })}
            mutationsEnabled={mutationsEnabled}
          />
        )}
        {section === "network" && (
          <NetworkSection namespace={namespace} mutationsEnabled={mutationsEnabled} />
        )}
        {section === "config" && (
          <ConfigSection
            namespace={namespace}
            onViewYAML={(kind, ns, name) => setYamlTarget({ kind, namespace: ns, name })}
          />
        )}
        {section === "topology" && <TopologySection />}
        {section === "health" && <ClusterHealthSection namespace={namespace} />}
        {section === "events" && <EventsSection />}

        {section !== "topology" && (
          <div className="mt-6">
            <PortForwardSessionsPanel mutationsEnabled={mutationsEnabled} />
          </div>
        )}
      </main>

      {/* Pod detail drawer */}
      {selectedPod && (
        <PodDetailDrawer
          namespace={selectedPod.namespace}
          pod={selectedPod.name}
          mutationsEnabled={mutationsEnabled}
          onClose={() => setSelectedPod(null)}
          onAuthorizeAction={handleAuthorizeAction}
        />
      )}

      {/* Generic YAML drawer (config/storage resources) */}
      {yamlTarget && <YAMLDrawer target={yamlTarget} onClose={() => setYamlTarget(null)} />}

      {/* CR code authorization modal */}
      {crAction && (
        <CRCodeApproval
          action={crAction}
          onClose={() => setCrAction(null)}
          onAuthorized={() => setCrAction(null)}
        />
      )}
    </div>
  );
}

function YAMLDrawer({ target, onClose }: { target: YAMLTarget; onClose: () => void }) {
  const { data = "", isLoading, error } = useQuery({
    queryKey: ["yaml", target.kind, target.namespace, target.name],
    queryFn: () => getResourceYAML(target.kind, target.namespace, target.name),
  });

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DrawerContent aria-describedby={undefined}>
        <div className="flex items-center justify-between px-6 py-4 border-b border-pilot-border shrink-0">
          <div>
            <h3 className="font-bold text-pilot-text-primary text-base capitalize">{target.kind} YAML</h3>
            <p className="text-sm text-pilot-muted mt-0.5 font-mono">
              {target.namespace ? `${target.namespace}/` : ""}
              {target.name}
            </p>
          </div>
          <button onClick={onClose} className="text-pilot-muted hover:text-pilot-text-primary p-1.5 rounded-lg hover:bg-pilot-surface" aria-label="Close">
            <X className="w-5 h-5" />
          </button>
        </div>
        <div className="flex-1 overflow-y-auto px-6 py-4">
          {isLoading ? (
            <div className="h-40 bg-pilot-surface rounded-lg animate-pulse" />
          ) : error ? (
            <div className="text-sm text-pilot-danger">Failed to load YAML.</div>
          ) : (
            <LogViewer title="Read-only · sensitive fields redacted" content={data} maxHeight="calc(100vh - 160px)" />
          )}
        </div>
      </DrawerContent>
    </Dialog>
  );
}
