import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  listConfigMaps,
  listSecrets,
  listPVCs,
  listStorageClasses,
  type ConfigMapSummary,
  type SecretSummary,
  type PVCSummary,
  type StorageClassInfo,
} from "@/lib/api";
import { ResourceTable, type Column } from "./ResourceTable";
import { SubTabs } from "./SubTabs";
import { Badge } from "@/components/ui/badge";
import { KindHint } from "@/components/ui/StatusPill";
import { KIND_HELP } from "@/lib/k8sExplain";
import { ShieldAlert } from "lucide-react";

type ConfigTab = "configmaps" | "secrets" | "pvcs" | "storageclasses";

const TABS: { key: ConfigTab; label: string }[] = [
  { key: "configmaps", label: "ConfigMaps" },
  { key: "secrets", label: "Secrets" },
  { key: "pvcs", label: "PVCs" },
  { key: "storageclasses", label: "StorageClasses" },
];

interface Props {
  namespace: string;
  onViewYAML: (kind: string, namespace: string, name: string) => void;
}

export function ConfigSection({ namespace, onViewYAML }: Props) {
  const [tab, setTab] = useState<ConfigTab>("configmaps");
  return (
    <div>
      <SubTabs tabs={TABS} active={tab} onChange={setTab} />
      {tab === "configmaps" && <ConfigMapsTab namespace={namespace} onViewYAML={onViewYAML} />}
      {tab === "secrets" && <SecretsTab namespace={namespace} onViewYAML={onViewYAML} />}
      {tab === "pvcs" && <PVCsTab namespace={namespace} onViewYAML={onViewYAML} />}
      {tab === "storageclasses" && <StorageClassesTab onViewYAML={onViewYAML} />}
    </div>
  );
}

function ConfigMapsTab({ namespace, onViewYAML }: Props) {
  const { data = [], isLoading, error } = useQuery({
    queryKey: ["dash-configmaps", namespace],
    queryFn: () => listConfigMaps(namespace),
  });
  const columns: Column<ConfigMapSummary>[] = [
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
      header: "Settings",
      align: "center",
      cell: (c) => <span className="tabular-nums text-pilot-text-secondary">{c.KeyCount}</span>,
      sortValue: (c) => c.KeyCount,
    },
  ];
  return (
    <>
      <KindHint {...KIND_HELP.configmaps} />
      <ResourceTable
        columns={columns}
        items={data}
        rowKey={(c) => `${c.Namespace}/${c.Name}`}
        loading={isLoading}
        error={error}
        noun="ConfigMap"
        searchText={(c) => `${c.Namespace}/${c.Name}`}
        emptyMessage="No ConfigMaps in this namespace."
        onRowClick={(c) => onViewYAML("configmap", c.Namespace, c.Name)}
      />
    </>
  );
}

function SecretsTab({ namespace, onViewYAML }: Props) {
  const { data = [], isLoading, error } = useQuery({
    queryKey: ["dash-secrets", namespace],
    queryFn: () => listSecrets(namespace),
  });
  const columns: Column<SecretSummary>[] = [
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
    { header: "Type", cell: (s) => <Badge variant="muted">{s.Type}</Badge>, sortValue: (s) => s.Type },
    {
      header: "Entries",
      align: "center",
      cell: (s) => <span className="tabular-nums text-pilot-text-secondary">{s.KeyCount}</span>,
      sortValue: (s) => s.KeyCount,
    },
  ];
  return (
    <div>
      <KindHint {...KIND_HELP.secrets} />
      <div className="mb-3 flex items-center gap-2 rounded-lg border border-pilot-warning/25 bg-pilot-warning/10 px-3 py-2 text-xs text-pilot-warning">
        <ShieldAlert className="h-4 w-4 shrink-0" aria-hidden="true" />
        Secret values are never displayed here — only names, types and how many entries each holds.
      </div>
      <ResourceTable
        columns={columns}
        items={data}
        rowKey={(s) => `${s.Namespace}/${s.Name}`}
        loading={isLoading}
        error={error}
        noun="secret"
        searchText={(s) => `${s.Namespace}/${s.Name} ${s.Type}`}
        emptyMessage="No secrets in this namespace."
        onRowClick={(s) => onViewYAML("secret", s.Namespace, s.Name)}
      />
    </div>
  );
}

function PVCsTab({ namespace, onViewYAML }: Props) {
  const { data = [], isLoading, error } = useQuery({
    queryKey: ["dash-pvcs", namespace],
    queryFn: () => listPVCs(namespace),
  });
  const columns: Column<PVCSummary>[] = [
    {
      header: "Name",
      cell: (p) => <span className="font-mono font-semibold text-pilot-text-primary">{p.Name}</span>,
      sortValue: (p) => p.Name,
    },
    {
      header: "Namespace",
      cell: (p) => <span className="text-pilot-text-secondary">{p.Namespace}</span>,
      sortValue: (p) => p.Namespace,
    },
    {
      header: "Status",
      cell: (p) => (
        <Badge variant={p.Status === "Bound" ? "success" : p.Status === "Pending" ? "warning" : "danger"}>
          {p.Status === "Bound" ? "Attached" : p.Status}
        </Badge>
      ),
      sortValue: (p) => p.Status,
    },
    {
      header: "Storage Class",
      cell: (p) => <span className="text-pilot-text-secondary">{p.StorageClass || "\u2014"}</span>,
      sortValue: (p) => p.StorageClass,
      hideBelow: "lg",
    },
    {
      header: "Size",
      align: "right",
      cell: (p) => <span className="tabular-nums text-pilot-text-secondary">{p.Capacity || "\u2014"}</span>,
      sortValue: (p) => p.Capacity,
    },
  ];
  return (
    <>
      <KindHint {...KIND_HELP.pvcs} />
      <ResourceTable
        columns={columns}
        items={data}
        rowKey={(p) => `${p.Namespace}/${p.Name}`}
        loading={isLoading}
        error={error}
        noun="storage claim"
        searchText={(p) => `${p.Namespace}/${p.Name} ${p.StorageClass}`}
        emptyMessage="No storage claims in this namespace."
        onRowClick={(p) => onViewYAML("pvc", p.Namespace, p.Name)}
      />
    </>
  );
}

function StorageClassesTab({ onViewYAML }: { onViewYAML: Props["onViewYAML"] }) {
  const { data = [], isLoading, error } = useQuery({
    queryKey: ["dash-storageclasses"],
    queryFn: () => listStorageClasses(),
  });
  const columns: Column<StorageClassInfo>[] = [
    {
      header: "Name",
      cell: (s) => <span className="font-mono font-semibold text-pilot-text-primary">{s.Name}</span>,
      sortValue: (s) => s.Name,
    },
    {
      header: "Provisioner",
      cell: (s) => <span className="font-mono text-xs text-pilot-text-secondary">{s.Provisioner}</span>,
      sortValue: (s) => s.Provisioner,
    },
    {
      header: "Reclaim",
      cell: (s) => <span className="text-pilot-text-secondary">{s.ReclaimPolicy || "\u2014"}</span>,
      sortValue: (s) => s.ReclaimPolicy,
      hideBelow: "lg",
    },
    {
      header: "Binding",
      cell: (s) => <span className="text-pilot-text-secondary">{s.VolumeBindingMode || "\u2014"}</span>,
      sortValue: (s) => s.VolumeBindingMode,
      hideBelow: "lg",
    },
    {
      header: "Expansion",
      align: "center",
      cell: (s) => (s.AllowVolumeExpansion ? <Badge variant="success">Yes</Badge> : <span className="text-pilot-muted">No</span>),
    },
  ];
  return (
    <>
      <KindHint {...KIND_HELP.storageclasses} />
      <ResourceTable
        columns={columns}
        items={data}
        rowKey={(s) => s.Name}
        loading={isLoading}
        error={error}
        noun="storage class"
        searchText={(s) => `${s.Name} ${s.Provisioner}`}
        emptyMessage="No storage classes found."
        onRowClick={(s) => onViewYAML("storageclass", "", s.Name)}
      />
    </>
  );
}
