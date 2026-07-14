import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
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

function PodsTab({
  namespace,
  onSelectPod,
  mutationsEnabled,
}: {
  namespace: string;
  onSelectPod: (ns: string, name: string) => void;
  mutationsEnabled: boolean;
}) {
  const { data: pods = [], isLoading } = useQuery({
    queryKey: ["dash-pods", namespace],
    queryFn: () => listPods(namespace),
  });
  return (
    <PodTable pods={pods} loading={isLoading} onRowClick={onSelectPod} mutationsEnabled={mutationsEnabled} />
  );
}

function DeploymentsTab({ namespace }: { namespace: string }) {
  const { data = [], isLoading, error } = useQuery({
    queryKey: ["dash-deployments", namespace],
    queryFn: () => listDeployments(namespace),
  });
  const columns: Column<DeploymentSummary>[] = [
    { header: "Namespace", cell: (d) => <span className="text-pilot-text-secondary">{d.Namespace}</span> },
    { header: "Name", cell: (d) => <span className="text-pilot-text-primary font-mono">{d.Name}</span> },
    {
      header: "Ready",
      align: "center",
      cell: (d) => (
        <ReadyBadge ready={d.ReadyReplicas} total={d.Replicas} />
      ),
    },
    { header: "Image", cell: (d) => <span className="text-pilot-muted font-mono text-xs">{d.Image}</span> },
  ];
  return (
    <ResourceTable
      columns={columns}
      items={data}
      rowKey={(d) => `${d.Namespace}/${d.Name}`}
      loading={isLoading}
      error={error}
      emptyMessage="No deployments in this namespace."
    />
  );
}

function StatefulSetsTab({ namespace }: { namespace: string }) {
  const { data = [], isLoading, error } = useQuery({
    queryKey: ["dash-statefulsets", namespace],
    queryFn: () => listStatefulSets(namespace),
  });
  const columns: Column<StatefulSetSummary>[] = [
    { header: "Namespace", cell: (s) => <span className="text-pilot-text-secondary">{s.Namespace}</span> },
    { header: "Name", cell: (s) => <span className="text-pilot-text-primary font-mono">{s.Name}</span> },
    { header: "Ready", align: "center", cell: (s) => <ReadyBadge ready={s.ReadyReplicas} total={s.Replicas} /> },
    { header: "Service", cell: (s) => <span className="text-pilot-text-secondary">{s.ServiceName || "—"}</span> },
    { header: "Image", cell: (s) => <span className="text-pilot-muted font-mono text-xs">{s.Image}</span> },
  ];
  return (
    <ResourceTable
      columns={columns}
      items={data}
      rowKey={(s) => `${s.Namespace}/${s.Name}`}
      loading={isLoading}
      error={error}
      emptyMessage="No statefulsets in this namespace."
    />
  );
}

function DaemonSetsTab({ namespace }: { namespace: string }) {
  const { data = [], isLoading, error } = useQuery({
    queryKey: ["dash-daemonsets", namespace],
    queryFn: () => listDaemonSets(namespace),
  });
  const columns: Column<DaemonSetSummary>[] = [
    { header: "Namespace", cell: (d) => <span className="text-pilot-text-secondary">{d.Namespace}</span> },
    { header: "Name", cell: (d) => <span className="text-pilot-text-primary font-mono">{d.Name}</span> },
    { header: "Ready", align: "center", cell: (d) => <ReadyBadge ready={d.NumberReady} total={d.DesiredNumberScheduled} /> },
    { header: "Image", cell: (d) => <span className="text-pilot-muted font-mono text-xs">{d.Image}</span> },
  ];
  return (
    <ResourceTable
      columns={columns}
      items={data}
      rowKey={(d) => `${d.Namespace}/${d.Name}`}
      loading={isLoading}
      error={error}
      emptyMessage="No daemonsets in this namespace."
    />
  );
}

function JobsTab({ namespace }: { namespace: string }) {
  const { data = [], isLoading, error } = useQuery({
    queryKey: ["dash-jobs", namespace],
    queryFn: () => listK8sJobs(namespace),
  });
  const columns: Column<K8sJobSummary>[] = [
    { header: "Namespace", cell: (j) => <span className="text-pilot-text-secondary">{j.Namespace}</span> },
    { header: "Name", cell: (j) => <span className="text-pilot-text-primary font-mono">{j.Name}</span> },
    { header: "Status", cell: (j) => <JobStatusBadge status={j.Status} /> },
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
    <ResourceTable
      columns={columns}
      items={data}
      rowKey={(j) => `${j.Namespace}/${j.Name}`}
      loading={isLoading}
      error={error}
      emptyMessage="No jobs in this namespace."
    />
  );
}

function CronJobsTab({ namespace }: { namespace: string }) {
  const { data = [], isLoading, error } = useQuery({
    queryKey: ["dash-cronjobs", namespace],
    queryFn: () => listCronJobs(namespace),
  });
  const columns: Column<CronJobSummary>[] = [
    { header: "Namespace", cell: (c) => <span className="text-pilot-text-secondary">{c.Namespace}</span> },
    { header: "Name", cell: (c) => <span className="text-pilot-text-primary font-mono">{c.Name}</span> },
    { header: "Schedule", cell: (c) => <span className="text-pilot-text-secondary font-mono text-xs">{c.Schedule}</span> },
    {
      header: "Suspended",
      align: "center",
      cell: (c) =>
        c.Suspend ? <Badge variant="warning">Suspended</Badge> : <Badge variant="success">Active</Badge>,
    },
    { header: "Running", align: "center", cell: (c) => <span className="text-pilot-text-secondary">{c.Active}</span> },
  ];
  return (
    <ResourceTable
      columns={columns}
      items={data}
      rowKey={(c) => `${c.Namespace}/${c.Name}`}
      loading={isLoading}
      error={error}
      emptyMessage="No cronjobs in this namespace."
    />
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
