import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  getPodDiagnostics,
  getPodLogs,
  getResourceYAML,
  troubleshootPod,
  listClusterEvents,
  type ContainerDiag,
  type SuggestedAction,
} from "@/lib/api";
import { LogViewer } from "@/components/LogViewer";
import { PortForwardButton } from "@/components/PortForwardButton";
import { Badge } from "@/components/ui/badge";
import { Dialog, DrawerContent } from "@/components/ui/dialog";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { X, RefreshCw, Brain } from "lucide-react";

interface Props {
  namespace: string;
  pod: string;
  mutationsEnabled: boolean;
  onClose: () => void;
  onAuthorizeAction: (action: SuggestedAction) => void;
}

export function PodDetailDrawer({ namespace, pod, mutationsEnabled, onClose, onAuthorizeAction }: Props) {
  const { data: diag } = useQuery({
    queryKey: ["pod-diag", namespace, pod],
    queryFn: () => getPodDiagnostics(namespace, pod),
  });
  const containerPorts = (diag?.diagnostics.container_statuses ?? []).flatMap((c) => c.ports ?? []);

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DrawerContent aria-describedby={undefined}>
        <div className="flex items-center justify-between px-6 py-4 border-b border-pilot-border shrink-0">
          <div>
            <h3 className="font-bold text-white text-base">Pod Details</h3>
            <p className="text-sm text-pilot-muted mt-0.5 font-mono">
              {namespace}/{pod}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <PortForwardButton
              kind="pod"
              namespace={namespace}
              name={pod}
              availablePorts={containerPorts}
              mutationsEnabled={mutationsEnabled}
            />
            <button
              onClick={onClose}
              className="text-pilot-muted hover:text-white p-1.5 rounded-lg hover:bg-pilot-surface"
              aria-label="Close"
            >
              <X className="w-5 h-5" />
            </button>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto px-6 py-4">
          <Tabs defaultValue="overview">
            <TabsList className="overflow-x-auto">
              <TabsTrigger value="overview">Overview</TabsTrigger>
              <TabsTrigger value="containers">Containers</TabsTrigger>
              <TabsTrigger value="logs">Logs</TabsTrigger>
              <TabsTrigger value="events">Events</TabsTrigger>
              <TabsTrigger value="yaml">YAML</TabsTrigger>
              <TabsTrigger value="ai">AI Analysis</TabsTrigger>
            </TabsList>

            <TabsContent value="overview">
              <OverviewTab namespace={namespace} pod={pod} />
            </TabsContent>
            <TabsContent value="containers">
              <ContainersTab namespace={namespace} pod={pod} />
            </TabsContent>
            <TabsContent value="logs">
              <LogsTab namespace={namespace} pod={pod} />
            </TabsContent>
            <TabsContent value="events">
              <EventsTab namespace={namespace} pod={pod} />
            </TabsContent>
            <TabsContent value="yaml">
              <YAMLTab namespace={namespace} pod={pod} />
            </TabsContent>
            <TabsContent value="ai">
              <AIAnalysisTab
                namespace={namespace}
                pod={pod}
                mutationsEnabled={mutationsEnabled}
                onAuthorizeAction={onAuthorizeAction}
              />
            </TabsContent>
          </Tabs>
        </div>
      </DrawerContent>
    </Dialog>
  );
}

function OverviewTab({ namespace, pod }: { namespace: string; pod: string }) {
  const { data, isLoading } = useQuery({
    queryKey: ["pod-diag", namespace, pod],
    queryFn: () => getPodDiagnostics(namespace, pod),
  });

  if (isLoading) return <Loading />;
  if (!data) return <Empty message="No diagnostics available." />;
  const d = data.diagnostics;

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-3">
        <Field label="Phase" value={d.phase} />
        <Field label="Node" value={d.node_name || "—"} />
        <Field label="Service Account" value={d.service_account || "—"} />
        <Field label="Created" value={new Date(d.created_at).toLocaleString()} />
      </div>

      {d.conditions && d.conditions.length > 0 && (
        <Panel title="Conditions">
          <div className="space-y-1">
            {d.conditions.map((c, i) => (
              <div key={i} className="flex items-center justify-between text-sm">
                <span className="text-pilot-text-secondary">{c.type}</span>
                <Badge variant={c.status === "True" ? "success" : "warning"}>{c.status}</Badge>
              </div>
            ))}
          </div>
        </Panel>
      )}

      {d.owner_chain && d.owner_chain.length > 0 && (
        <Panel title="Owner Chain">
          <div className="text-sm text-pilot-text-secondary font-mono">
            {d.owner_chain.map((o) => `${o.kind}/${o.name}`).join(" → ")}
          </div>
        </Panel>
      )}

      {d.resource_usage && (
        <Panel title="Resources">
          <div className="grid grid-cols-2 gap-2 text-sm">
            <Field label="CPU Request" value={d.resource_usage.cpu_request} />
            <Field label="CPU Limit" value={d.resource_usage.cpu_limit} />
            <Field label="Mem Request" value={d.resource_usage.mem_request} />
            <Field label="Mem Limit" value={d.resource_usage.mem_limit} />
            <Field label="CPU Usage" value={d.resource_usage.cpu_usage} />
            <Field label="Mem Usage" value={d.resource_usage.mem_usage} />
          </div>
        </Panel>
      )}

      {d.volumes && d.volumes.length > 0 && (
        <Panel title="Volumes">
          <div className="flex flex-wrap gap-1.5">
            {d.volumes.map((v) => (
              <Badge key={v} variant="muted">{v}</Badge>
            ))}
          </div>
        </Panel>
      )}
    </div>
  );
}

function ContainersTab({ namespace, pod }: { namespace: string; pod: string }) {
  const { data, isLoading } = useQuery({
    queryKey: ["pod-diag", namespace, pod],
    queryFn: () => getPodDiagnostics(namespace, pod),
  });
  if (isLoading) return <Loading />;
  const containers = data?.diagnostics.container_statuses ?? [];
  if (containers.length === 0) return <Empty message="No container statuses." />;

  return (
    <div className="space-y-3">
      {containers.map((c) => (
        <ContainerCard key={c.name} container={c} />
      ))}
    </div>
  );
}

function ContainerCard({ container: c }: { container: ContainerDiag }) {
  const stateVariant =
    c.state === "Running" ? "success" : c.state === "Waiting" ? "warning" : "danger";
  return (
    <div className="bg-pilot-surface border border-pilot-border rounded-xl p-4">
      <div className="flex items-center justify-between mb-2">
        <span className="text-sm font-semibold text-white font-mono">{c.name}</span>
        <Badge variant={stateVariant}>{c.state || "Unknown"}</Badge>
      </div>
      <p className="text-xs text-pilot-muted font-mono mb-2">{c.image}</p>
      <div className="grid grid-cols-3 gap-2 text-xs">
        <Field label="Ready" value={c.ready ? "Yes" : "No"} />
        <Field label="Restarts" value={String(c.restart_count)} />
        {c.state === "Terminated" && <Field label="Exit Code" value={String(c.exit_code)} />}
      </div>
      {c.state_reason && (
        <p className="text-xs text-pilot-danger mt-2">
          {c.state_reason}
          {c.state_message ? `: ${c.state_message}` : ""}
        </p>
      )}
      {c.last_terminated_reason && (
        <p className="text-xs text-pilot-warning mt-1">Last terminated: {c.last_terminated_reason}</p>
      )}
    </div>
  );
}

function LogsTab({ namespace, pod }: { namespace: string; pod: string }) {
  const [container, setContainer] = useState("");
  const [tail, setTail] = useState(200);

  // Container choices come from diagnostics (cached from the Overview/Containers tabs).
  const { data: diag } = useQuery({
    queryKey: ["pod-diag", namespace, pod],
    queryFn: () => getPodDiagnostics(namespace, pod),
  });
  const containers = diag?.diagnostics.container_statuses ?? [];

  const { data: logs = "", isLoading, refetch, isFetching } = useQuery({
    queryKey: ["pod-logs", namespace, pod, container, tail],
    queryFn: () => getPodLogs(namespace, pod, container, tail),
    refetchInterval: 15_000,
  });

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2 flex-wrap">
        <select
          value={container}
          onChange={(e) => setContainer(e.target.value)}
          className="bg-pilot-surface border border-pilot-border rounded-lg px-3 py-1.5 text-sm text-white"
        >
          <option value="">Default container</option>
          {containers.map((c) => (
            <option key={c.name} value={c.name}>
              {c.name}
            </option>
          ))}
        </select>
        {[200, 500, 1000].map((n) => (
          <button
            key={n}
            onClick={() => setTail(n)}
            className={`text-xs px-2.5 py-1.5 rounded-lg font-medium ${
              tail === n ? "bg-pilot-accent text-white" : "bg-pilot-surface text-pilot-muted hover:text-white"
            }`}
          >
            Tail {n}
          </button>
        ))}
        <button
          onClick={() => refetch()}
          className="inline-flex items-center gap-1.5 text-xs px-2.5 py-1.5 rounded-lg bg-pilot-surface text-pilot-muted hover:text-white"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${isFetching ? "animate-spin" : ""}`} /> Refresh
        </button>
      </div>
      {isLoading ? <Loading /> : <LogViewer title="Container Logs" content={logs || "(no logs)"} maxHeight="500px" />}
    </div>
  );
}

function EventsTab({ namespace, pod }: { namespace: string; pod: string }) {
  const { data, isLoading } = useQuery({
    queryKey: ["pod-events", namespace, pod],
    queryFn: () => listClusterEvents({ namespace, kind: "Pod", search: pod, sort: "desc" }),
  });
  if (isLoading) return <Loading />;
  const events = data?.items ?? [];
  if (events.length === 0) return <Empty message="No recent events for this pod." />;

  return (
    <div className="space-y-2">
      {events.map((e, i) => (
        <div key={i} className="bg-pilot-surface border border-pilot-border rounded-lg p-3">
          <div className="flex items-center gap-2 mb-1">
            <Badge variant={e.type === "Warning" ? "warning" : "muted"}>{e.type}</Badge>
            <span className="text-sm font-semibold text-white">{e.reason}</span>
            {e.count > 1 && <span className="text-xs text-pilot-muted">×{e.count}</span>}
          </div>
          <p className="text-sm text-pilot-text-secondary">{e.message}</p>
        </div>
      ))}
    </div>
  );
}

function YAMLTab({ namespace, pod }: { namespace: string; pod: string }) {
  const { data = "", isLoading, error } = useQuery({
    queryKey: ["pod-yaml", namespace, pod],
    queryFn: () => getResourceYAML("pod", namespace, pod),
  });
  if (isLoading) return <Loading />;
  if (error) return <Empty message="Could not load YAML." />;
  return <LogViewer title="Pod YAML (read-only · sensitive fields redacted)" content={data} maxHeight="600px" />;
}

function AIAnalysisTab({
  namespace,
  pod,
  mutationsEnabled,
  onAuthorizeAction,
}: {
  namespace: string;
  pod: string;
  mutationsEnabled: boolean;
  onAuthorizeAction: (a: SuggestedAction) => void;
}) {
  const [enabled, setEnabled] = useState(false);
  const { data: report, isLoading, isError } = useQuery({
    queryKey: ["pod-ai", namespace, pod],
    queryFn: () => troubleshootPod(namespace, pod),
    enabled,
  });

  if (!enabled) {
    return (
      <div className="text-center py-10">
        <Brain className="w-10 h-10 text-pilot-accent mx-auto mb-3" />
        <p className="text-sm text-pilot-text-secondary mb-4">
          Run AI root-cause analysis on this pod using KubePilot&apos;s troubleshooting engine.
        </p>
        <button
          onClick={() => setEnabled(true)}
          className="bg-pilot-accent hover:bg-blue-500 text-white px-6 py-2.5 rounded-lg text-sm font-semibold shadow-glow-blue"
        >
          Analyze with AI
        </button>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="flex items-center gap-3 text-pilot-muted text-sm py-8">
        <RefreshCw className="w-5 h-5 animate-spin" /> Analyzing pod with AI...
      </div>
    );
  }
  if (isError || !report) return <Empty message="AI analysis failed. Is the model reachable?" />;

  return (
    <div className="space-y-4">
      <Panel title="Root Cause">
        <p className="text-base font-bold text-pilot-danger leading-relaxed">{report.RootCause || "Unknown"}</p>
      </Panel>
      <Panel title="Analysis">
        <p className="text-sm text-white leading-relaxed whitespace-pre-wrap">{report.Analysis}</p>
      </Panel>
      {report.Actions && report.Actions.length > 0 && (
        <div>
          <p className="text-xs font-medium text-pilot-muted uppercase tracking-wider mb-2">Suggested Actions</p>
          <div className="space-y-2">
            {report.Actions.map((action, i) => (
              <div key={i} className="bg-pilot-surface border border-pilot-border rounded-xl p-4 flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <span className="text-xs font-bold text-pilot-accent uppercase">{action.type}</span>
                  <p className="text-sm text-pilot-text-secondary mt-1.5 leading-relaxed">{action.explanation}</p>
                </div>
                <button
                  onClick={() => onAuthorizeAction(action)}
                  disabled={!mutationsEnabled}
                  title={mutationsEnabled ? undefined : "Mutations are disabled on this server"}
                  className={`shrink-0 text-xs px-3 py-2 rounded-lg font-semibold ${
                    mutationsEnabled
                      ? "bg-pilot-success text-black hover:brightness-110"
                      : "bg-pilot-surface-2 text-pilot-muted cursor-not-allowed"
                  }`}
                >
                  {action.requires_cr_code ? "Authorize & Run" : "Execute"}
                </button>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-xs uppercase tracking-wider text-pilot-muted">{label}</div>
      <div className="text-sm text-white font-mono break-all">{value}</div>
    </div>
  );
}

function Panel({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="bg-pilot-surface border border-pilot-border rounded-xl p-4">
      <p className="text-xs font-medium text-pilot-muted uppercase tracking-wider mb-2">{title}</p>
      {children}
    </div>
  );
}

function Loading() {
  return (
    <div className="space-y-2 py-2">
      {[...Array(3)].map((_, i) => (
        <div key={i} className="h-12 bg-pilot-surface rounded-lg animate-pulse" />
      ))}
    </div>
  );
}

function Empty({ message }: { message: string }) {
  return <div className="text-center py-10 text-sm text-pilot-muted">{message}</div>;
}
