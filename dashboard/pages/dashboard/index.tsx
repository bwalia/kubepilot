/**
 * Kubernetes Dashboard — a Lens/Rancher-style read-only cluster browser.
 * Complements the AI Troubleshooting homepage. All navigation is client-side
 * state (no dynamic file-based routes) so the page is compatible with Next.js
 * static export (output: "export").
 */
import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  listPods,
  listNamespaces,
  getServerConfig,
  getResourceYAML,
  executeSuggestedAction,
  type SuggestedAction,
} from "@/lib/api";
import type { PodSummary } from "@/lib/api";
import { useNamespaceLock } from "@/lib/useNamespaceLock";
import { consumeDashboardTarget, onDashboardTarget, type DashboardTarget } from "@/lib/dashboardNav";
import { NamespacePicker } from "@/components/ui/NamespacePicker";
import { openCommandPalette } from "@/components/CommandPalette";
import { useSessionState } from "@/lib/useSessionState";
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
import { DeckSidebar } from "@/components/DeckSidebar";
import { Breadcrumb } from "@/components/Breadcrumb";
import { Dialog, DrawerContent } from "@/components/ui/dialog";
import { LayoutDashboard, Boxes, Network, Database, HeartPulse, FileWarning, Share2, X, Menu, Search } from "lucide-react";

type Section = "overview" | "workloads" | "network" | "config" | "topology" | "health" | "events";

// Every section carries a one-line description. The nav label has to stay
// short, but a label alone ("Workloads", "Config & Storage") assumes you
// already know the Kubernetes vocabulary — which is exactly the assumption
// this dashboard should not make.
const SECTIONS: { key: Section; label: string; icon: typeof Boxes; blurb: string }[] = [
  {
    key: "overview",
    label: "Overview",
    icon: LayoutDashboard,
    blurb: "How the cluster is doing right now, and anything that needs attention.",
  },
  {
    key: "workloads",
    label: "Workloads",
    icon: Boxes,
    blurb: "Your running apps — and the instructions that keep them running.",
  },
  {
    key: "network",
    label: "Network",
    icon: Network,
    blurb: "How traffic reaches your apps, from the public internet and from inside the cluster.",
  },
  {
    key: "config",
    label: "Config & Storage",
    icon: Database,
    blurb: "Settings, credentials and disks that your apps depend on.",
  },
  {
    key: "topology",
    label: "Topology",
    icon: Share2,
    blurb: "A map of which services talk to which.",
  },
  {
    key: "health",
    label: "Cluster Health",
    icon: HeartPulse,
    blurb: "The machines behind the cluster, and whether they have room to spare.",
  },
  {
    key: "events",
    label: "Events",
    icon: FileWarning,
    blurb: "Kubernetes' own running commentary — the first place to look when something changed.",
  },
];

interface YAMLTarget {
  kind: string;
  namespace: string;
  name: string;
}

export default function KubernetesDashboard() {
  const { locked, namespace: lockedNamespace } = useNamespaceLock();
  // Persisted for the session so a page refresh keeps the chosen namespace.
  const [selectedNamespace, setSelectedNamespace] = useSessionState("kubepilot-dashboard-ns", "");
  // When the URL locks a namespace, it overrides the dropdown selection.
  const namespace = locked ? lockedNamespace! : selectedNamespace;
  // Section persists across refresh (session-scoped), like the namespace.
  const [sectionRaw, setSectionRaw] = useSessionState("kubepilot-dashboard-section", "overview");
  const section = sectionRaw as Section;
  const setSection = (s: Section) => setSectionRaw(s);
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const [selectedPod, setSelectedPod] = useState<{ namespace: string; name: string } | null>(null);
  const [yamlTarget, setYamlTarget] = useState<YAMLTarget | null>(null);
  const [crAction, setCrAction] = useState<SuggestedAction | null>(null);

  const { data: namespaces = [] } = useQuery({
    queryKey: ["namespaces"],
    queryFn: listNamespaces,
  });

  // Pod counts next to each namespace are a hint, not data worth a request of
  // their own: read the all-namespaces pod list only if something else (the
  // command palette, or browsing "All namespaces") has already loaded it.
  // `enabled: false` never fetches, but react-query still requires a queryFn on
  // the observer — without one it logs an error on every render.
  const { data: allPods } = useQuery<PodSummary[]>({
    queryKey: ["dash-pods", ""],
    queryFn: () => listPods(""),
    enabled: false,
  });
  const namespaceOptions = useMemo(() => {
    const counts = new Map<string, number>();
    for (const pod of allPods ?? []) counts.set(pod.Namespace, (counts.get(pod.Namespace) ?? 0) + 1);
    return namespaces.map((ns) => ({ name: ns.Name, count: counts.get(ns.Name) }));
  }, [namespaces, allPods]);

  // Requests from the command palette ("open this pod", "go to this section").
  // Handled here because the palette lives in _app, outside this page's state.
  useEffect(() => {
    const apply = (t: DashboardTarget) => {
      if (t.type === "pod") {
        setSelectedPod({ namespace: t.namespace, name: t.name });
      } else if (t.type === "section") {
        setSectionRaw(t.section);
      } else if (t.type === "namespace" && !locked) {
        setSelectedNamespace(t.namespace);
        setSectionRaw("workloads");
      }
    };
    const pending = consumeDashboardTarget();
    if (pending) apply(pending);
    return onDashboardTarget(apply);
  }, [locked, setSectionRaw, setSelectedNamespace]);

  // Deep link: /dashboard?pod=<namespace>/<name> opens straight to a pod, so a
  // link to a broken thing can be pasted into a chat and just work.
  useEffect(() => {
    const raw = new URLSearchParams(window.location.search).get("pod");
    if (!raw) return;
    const slash = raw.indexOf("/");
    if (slash > 0) setSelectedPod({ namespace: raw.slice(0, slash), name: raw.slice(slash + 1) });
  }, []);

  const { data: serverConfig } = useQuery({
    queryKey: ["server-config"],
    queryFn: getServerConfig,
    staleTime: 60_000,
    refetchInterval: false,
  });
  const mutationsEnabled = serverConfig?.mutations_enabled ?? false;
  const current = SECTIONS.find((s) => s.key === section);

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
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-pilot-border bg-pilot-surface px-4 py-4 sm:px-6 lg:px-8">
        <div className="flex min-w-0 flex-1 items-center gap-3">
          <button
            type="button"
            onClick={() => setMobileNavOpen(true)}
            aria-label="Open sections"
            className="lg:hidden shrink-0 inline-flex items-center justify-center w-11 h-11 rounded-xl border border-pilot-border bg-pilot-surface-2 text-pilot-text-secondary hover:text-pilot-text-primary transition-colors"
          >
            <Menu className="w-5 h-5" />
          </button>
          <div className="min-w-0">
            <h1 className="font-display text-2xl font-bold tracking-tight text-pilot-text-primary">
              Cluster
            </h1>
            {/* Hidden on phones: at 390px it reflows into a narrow column
                beside the buttons and pushes the real content off-screen. */}
            <p className="mt-0.5 hidden text-sm text-pilot-muted sm:block">
              Browse and troubleshoot everything running here. Nothing on this page changes the cluster.
            </p>
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {/* The palette is the primary way to find anything, so it gets a
              visible control — a keyboard shortcut nobody is told about is not
              a feature. */}
          <button
            type="button"
            onClick={openCommandPalette}
            className="inline-flex h-11 items-center gap-2 rounded-xl border border-pilot-border bg-pilot-surface-2 px-3 text-sm text-pilot-text-secondary transition-colors hover:border-pilot-border-hover hover:text-pilot-text-primary"
          >
            <Search className="h-4 w-4 shrink-0" aria-hidden="true" />
            <span className="hidden sm:inline">Search cluster</span>
            <kbd className="hidden rounded border border-pilot-border bg-pilot-surface px-1.5 py-0.5 font-mono text-[0.7rem] text-pilot-muted lg:inline">
              &#8984;K
            </kbd>
          </button>
          <KubeconfigSwitcher onSwitched={() => setSelectedPod(null)} />
        </div>
      </header>

      {/* Sidebar + section content */}
      <div className="flex items-start">
        <DeckSidebar
          heading="Browse"
          items={SECTIONS}
          active={section}
          onSelect={setSection}
          mobileOpen={mobileNavOpen}
          onMobileOpenChange={setMobileNavOpen}
          storageKey="kubepilot-dashboard-sidebar"
        />

        <main className="flex-1 min-w-0 px-4 sm:px-6 lg:px-8 py-6">
          {/* Section identity + the namespace scope it is showing. */}
          <div className="mb-6">
            <Breadcrumb items={["Cluster", current?.label ?? ""]} className="mb-3" />
            <div className="flex flex-wrap items-start justify-between gap-x-6 gap-y-3">
              <div className="min-w-0 max-w-2xl">
                <h2 className="font-display text-xl font-bold tracking-tight text-pilot-text-primary">
                  {current?.label}
                </h2>
                <p className="mt-1 text-sm leading-relaxed text-pilot-muted">{current?.blurb}</p>
              </div>
              <div className="flex flex-col gap-1">
                <span className="eyebrow">Showing</span>
                <NamespacePicker
                  value={namespace}
                  onChange={setSelectedNamespace}
                  namespaces={namespaceOptions}
                  locked={locked}
                />
              </div>
            </div>
          </div>

          <div className="animate-fade-in">
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
          </div>
        </main>
      </div>

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
