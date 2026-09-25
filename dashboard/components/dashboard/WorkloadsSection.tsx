import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { qk } from "@/lib/queryKeys";
import {
  listPods,
  listDeployments,
  listStatefulSets,
  listDaemonSets,
  listK8sJobs,
  listCronJobs,
  type DeploymentSummary,
  type StatefulSetSummary,
  type DaemonSetSummary,
  type K8sJobSummary,
  type CronJobSummary,
} from "@/lib/api";
import { PodTable } from "@/components/PodTable";
import { ResourceTable, type Column } from "./ResourceTable";
import { SubTabs } from "./SubTabs";
import { Badge } from "@/components/ui/badge";
import { KindHint } from "@/components/ui/StatusPill";
import { KIND_HELP } from "@/lib/k8sExplain";

type WorkloadTab = "pods" | "deployments" | "statefulsets" | "daemonsets" | "jobs" | "cronjobs";

const TABS: { key: WorkloadTab; label: string }[] = [
  { key: "pods", label: "Pods" },
  { key: "deployments", label: "Deployments" },
  { key: "statefulsets", label: "StatefulSets" },
  { key: "daemonsets", label: "DaemonSets" },
  { key: "jobs", label: "Jobs" },
  { key: "cronjobs", label: "CronJobs" },
];

interface Props {
  namespace: string;
  onSelectPod: (namespace: string, name: string) => void;
  mutationsEnabled?: boolean;
}

export function WorkloadsSection({ namespace, onSelectPod, mutationsEnabled = false }: Props) {
  const [tab, setTab] = useState<WorkloadTab>("pods");

  return (
    <div>
      <SubTabs tabs={TABS} active={tab} onChange={setTab} />
      {tab === "pods" && (
        <PodsTab namespace={namespace} onSelectPod={onSelectPod} mutationsEnabled={mutationsEnabled} />
      )}
      {tab === "deployments" && <DeploymentsTab namespace={namespace} />}
      {tab === "statefulsets" && <StatefulSetsTab namespace={namespace} />}
      {tab === "daemonsets" && <DaemonSetsTab namespace={namespace} />}
      {tab === "jobs" && <JobsTab namespace={namespace} />}
      {tab === "cronjobs" && <CronJobsTab namespace={namespace} />}
    </div>
  );
}

/** Parse a Kubernetes-style uptime string ("4d15h", "3h48m", "12m", "45s") to
 *  minutes. Empty/unparseable → Infinity so it's excluded by an age filter. */
function uptimeToMinutes(uptime: string): number {
  if (!uptime) return Infinity;
  let mins = 0;
  const d = uptime.match(/(\d+)\s*d/);
  const h = uptime.match(/(\d+)\s*h/);
  const m = uptime.match(/(\d+)\s*m/);
  const s = uptime.match(/(\d+)\s*s/);
  if (d) mins += parseInt(d[1], 10) * 1440;
  if (h) mins += parseInt(h[1], 10) * 60;
  if (m) mins += parseInt(m[1], 10);
  if (s) mins += parseInt(s[1], 10) / 60;
  return d || h || m || s ? mins : Infinity;
}

const AGE_RANGES: { label: string; minutes: number }[] = [
  { label: "Any time", minutes: 0 },
  { label: "Last 1 hour", minutes: 60 },
  { label: "Last 2 hours", minutes: 120 },
  { label: "Last 6 hours", minutes: 360 },
  { label: "Last 24 hours", minutes: 1440 },
  { label: "Last 3 days", minutes: 4320 },
  { label: "Last 7 days", minutes: 10080 },
];

function PodsTab({
  namespace,
  onSelectPod,
  mutationsEnabled,
}: {
  namespace: string;
  onSelectPod: (ns: string, name: string) => void;
  mutationsEnabled: boolean;
}) {
  const [rangeMin, setRangeMin] = useState(0);
  const { data: pods = [], isLoading } = useQuery({
    queryKey: qk.pods(namespace),
    queryFn: () => listPods(namespace),
  });

  // Filter to pods started within the selected window, newest first.
  const shown =
    rangeMin > 0
      ? pods
          .filter((p) => uptimeToMinutes(p.Uptime) <= rangeMin)
          .sort((a, b) => uptimeToMinutes(a.Uptime) - uptimeToMinutes(b.Uptime))
      : pods;

  return (
    <PodTable
      pods={shown}
      loading={isLoading}
      onRowClick={onSelectPod}
      mutationsEnabled={mutationsEnabled}
      filterSlot={
        <div className="flex items-center gap-2">
          <label htmlFor="pod-age-filter" className="eyebrow">
            Started within
          </label>
          <select
            id="pod-age-filter"
            value={rangeMin}
            onChange={(e) => setRangeMin(Number(e.target.value))}
            className="h-11 rounded-xl border border-pilot-border bg-pilot-surface px-3 text-sm text-pilot-text-primary focus:border-pilot-accent/60 focus:outline-none focus:ring-2 focus:ring-pilot-accent/25"
          >
            {AGE_RANGES.map((r) => (
              <option key={r.minutes} value={r.minutes}>
                {r.label}
              </option>
            ))}
          </select>
        </div>
      }
    />
  );
}

function DeploymentsTab({ namespace }: { namespace: string }) {
  const { data = [], isLoading, error } = useQuery({
    queryKey: qk.deployments(namespace),
    queryFn: () => listDeployments(namespace),
  });
  const columns: Column<DeploymentSummary>[] = [
    {
      header: "Name",
      cell: (d) => <span className="font-mono font-semibold text-pilot-text-primary">{d.Name}</span>,
      sortValue: (d) => d.Name,
    },
    {
      header: "Namespace",
      cell: (d) => <span className="text-pilot-text-secondary">{d.Namespace}</span>,
      sortValue: (d) => d.Namespace,
    },
    {
      header: "Ready",
      align: "center",
      cell: (d) => <ReadyBadge ready={d.ReadyReplicas} total={d.Replicas} />,
      // Sort by how far short of the target it is, so the worst float together.
      sortValue: (d) => d.ReadyReplicas - d.Replicas,
    },
    {
      header: "Image",
      cell: (d) => <span className="font-mono text-xs text-pilot-muted">{d.Image}</span>,
      sortValue: (d) => d.Image,
      hideBelow: "lg",
    },
  ];
  return (
    <>
      <KindHint {...KIND_HELP.deployments} />
      <ResourceTable
        columns={columns}
        items={data}
        rowKey={(d) => `${d.Namespace}/${d.Name}`}
        loading={isLoading}
        error={error}
        noun="deployment"
        searchText={(d) => `${d.Namespace}/${d.Name} ${d.Image}`}
        emptyMessage="No deployments in this namespace."
      />
    </>
  );
}

function StatefulSetsTab({ namespace }: { namespace: string }) {
  const { data = [], isLoading, error } = useQuery({
    queryKey: qk.statefulSets(namespace),
    queryFn: () => listStatefulSets(namespace),
  });
  const columns: Column<StatefulSetSummary>[] = [
    {
      header: "Name",
      cell: (s) => <span className="font-mono font-semibold text-pilot-text-primary">{s.Name}</span>,
      sortValue: (s) => s.Name,
    },
    {
      header: "Namespace",
      cell: (s) => <span className="text-pilot-text-secondary">{s.Namespace}</span>,
      sortValue: (s) => s.Namespace,
    },
    {
      header: "Ready",
      align: "center",
      cell: (s) => <ReadyBadge ready={s.ReadyReplicas} total={s.Replicas} />,
      sortValue: (s) => s.ReadyReplicas - s.Replicas,
    },
    {
      header: "Service",
      cell: (s) => <span className="text-pilot-text-secondary">{s.ServiceName || "\u2014"}</span>,
      sortValue: (s) => s.ServiceName,
      hideBelow: "lg",
    },
    {
      header: "Image",
      cell: (s) => <span className="font-mono text-xs text-pilot-muted">{s.Image}</span>,
      sortValue: (s) => s.Image,
      hideBelow: "lg",
    },
  ];
  return (
    <>
      <KindHint {...KIND_HELP.statefulsets} />
      <ResourceTable
        columns={columns}
        items={data}
        rowKey={(s) => `${s.Namespace}/${s.Name}`}
        loading={isLoading}
        error={error}
        noun="StatefulSet"
        searchText={(s) => `${s.Namespace}/${s.Name} ${s.Image}`}
        emptyMessage="No StatefulSets in this namespace."
      />
    </>
  );
}

function DaemonSetsTab({ namespace }: { namespace: string }) {
  const { data = [], isLoading, error } = useQuery({
    queryKey: qk.daemonSets(namespace),
    queryFn: () => listDaemonSets(namespace),
  });
  const columns: Column<DaemonSetSummary>[] = [
    {
      header: "Name",
      cell: (d) => <span className="font-mono font-semibold text-pilot-text-primary">{d.Name}</span>,
      sortValue: (d) => d.Name,
    },
    {
      header: "Namespace",
      cell: (d) => <span className="text-pilot-text-secondary">{d.Namespace}</span>,
      sortValue: (d) => d.Namespace,
    },
    {
      header: "Ready",
      align: "center",
      cell: (d) => <ReadyBadge ready={d.NumberReady} total={d.DesiredNumberScheduled} />,
      sortValue: (d) => d.NumberReady - d.DesiredNumberScheduled,
    },
    {
      header: "Image",
      cell: (d) => <span className="font-mono text-xs text-pilot-muted">{d.Image}</span>,
      sortValue: (d) => d.Image,
      hideBelow: "lg",
    },
  ];
  return (
    <>
      <KindHint {...KIND_HELP.daemonsets} />
      <ResourceTable
        columns={columns}
        items={data}
        rowKey={(d) => `${d.Namespace}/${d.Name}`}
        loading={isLoading}
        error={error}
        noun="DaemonSet"
        searchText={(d) => `${d.Namespace}/${d.Name} ${d.Image}`}
        emptyMessage="No DaemonSets in this namespace."
      />
    </>
  );
}

function JobsTab({ namespace }: { namespace: string }) {
  const { data = [], isLoading, error } = useQuery({
    queryKey: qk.jobs(namespace),
    queryFn: () => listK8sJobs(namespace),
  });
  const columns: Column<K8sJobSummary>[] = [
    {
      header: "Name",
      cell: (j) => <span className="font-mono font-semibold text-pilot-text-primary">{j.Name}</span>,
      sortValue: (j) => j.Name,
    },
    {
      header: "Namespace",
      cell: (j) => <span className="text-pilot-text-secondary">{j.Namespace}</span>,
      sortValue: (j) => j.Namespace,
    },
    { header: "Status", cell: (j) => <JobStatusBadge status={j.Status} />, sortValue: (j) => j.Status },
    {
      header: "Completions",
      align: "center",
      cell: (j) => (
        <span className="text-pilot-text-secondary">
          {j.Succeeded}/{j.Completions || "—"}
        </span>
      ),
    },
  ];
  return (
    <>
      <KindHint {...KIND_HELP.jobs} />
      <ResourceTable
        columns={columns}
        items={data}
        rowKey={(j) => `${j.Namespace}/${j.Name}`}
        loading={isLoading}
        error={error}
        noun="job"
        searchText={(j) => `${j.Namespace}/${j.Name} ${j.Status}`}
        emptyMessage="No jobs in this namespace."
      />
    </>
  );
}

function CronJobsTab({ namespace }: { namespace: string }) {
  const { data = [], isLoading, error } = useQuery({
    queryKey: qk.cronJobs(namespace),
    queryFn: () => listCronJobs(namespace),
  });
  const columns: Column<CronJobSummary>[] = [
    {
      header: "Name",
      cell: (c) => <span className="font-mono font-semibold text-pilot-text-primary">{c.Name}</span>,
      sortValue: (c) => c.Name,
    },
    {
      header: "Namespace",
      cell: (c) => <span className="text-pilot-text-secondary">{c.Namespace}</span>,
      sortValue: (c) => c.Namespace,
    },
    {
      header: "Schedule",
      cell: (c) => <span className="font-mono text-xs text-pilot-text-secondary">{c.Schedule}</span>,
      sortValue: (c) => c.Schedule,
    },
    {
      header: "Suspended",
      align: "center",
      cell: (c) =>
        c.Suspend ? <Badge variant="warning">Suspended</Badge> : <Badge variant="success">Active</Badge>,
    },
    { header: "Running", align: "center", cell: (c) => <span className="text-pilot-text-secondary">{c.Active}</span> },
  ];
  return (
    <>
      <KindHint {...KIND_HELP.cronjobs} />
      <ResourceTable
        columns={columns}
        items={data}
        rowKey={(c) => `${c.Namespace}/${c.Name}`}
        loading={isLoading}
        error={error}
        noun="CronJob"
        searchText={(c) => `${c.Namespace}/${c.Name} ${c.Schedule}`}
        emptyMessage="No CronJobs in this namespace."
      />
    </>
  );
}

function ReadyBadge({ ready, total }: { ready: number; total: number }) {
  const ok = total > 0 && ready >= total;
  return (
    <Badge variant={ok ? "success" : "warning"}>
      {ready}/{total}
    </Badge>
  );
}

function JobStatusBadge({ status }: { status: string }) {
  const variant =
    status === "Complete" ? "success" : status === "Failed" ? "danger" : status === "Active" ? "default" : "muted";
  return <Badge variant={variant}>{status}</Badge>;
}
