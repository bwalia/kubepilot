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
    { header: "Namespace", cell: (c) => <span className="text-pilot-text-secondary">{c.Namespace}</span> },
    { header: "Name", cell: (c) => <span className="text-white font-mono">{c.Name}</span> },
    { header: "Keys", align: "center", cell: (c) => <span className="text-pilot-text-secondary">{c.KeyCount}</span> },
  ];
  return (
    <ResourceTable
      columns={columns}
      items={data}
      rowKey={(c) => `${c.Namespace}/${c.Name}`}
      loading={isLoading}
      error={error}
      emptyMessage="No configmaps in this namespace."
      onRowClick={(c) => onViewYAML("configmap", c.Namespace, c.Name)}
    />
  );
}

function SecretsTab({ namespace, onViewYAML }: Props) {
  const { data = [], isLoading, error } = useQuery({
    queryKey: ["dash-secrets", namespace],
    queryFn: () => listSecrets(namespace),
  });
  const columns: Column<SecretSummary>[] = [
    { header: "Namespace", cell: (s) => <span className="text-pilot-text-secondary">{s.Namespace}</span> },
    { header: "Name", cell: (s) => <span className="text-white font-mono">{s.Name}</span> },
    { header: "Type", cell: (s) => <Badge variant="muted">{s.Type}</Badge> },
    { header: "Keys", align: "center", cell: (s) => <span className="text-pilot-text-secondary">{s.KeyCount}</span> },
  ];
  return (
    <div>
      <div className="mb-3 flex items-center gap-2 text-xs text-pilot-warning bg-amber-500/10 border border-amber-500/20 rounded-lg px-3 py-2">
        <ShieldAlert className="w-4 h-4 shrink-0" />
        Secret values are never displayed. Only metadata and key counts are shown.
      </div>
      <ResourceTable
        columns={columns}
        items={data}
        rowKey={(s) => `${s.Namespace}/${s.Name}`}
        loading={isLoading}
        error={error}
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
    { header: "Namespace", cell: (p) => <span className="text-pilot-text-secondary">{p.Namespace}</span> },
    { header: "Name", cell: (p) => <span className="text-white font-mono">{p.Name}</span> },
    {
      header: "Status",
      cell: (p) => (
        <Badge variant={p.Status === "Bound" ? "success" : p.Status === "Pending" ? "warning" : "danger"}>
          {p.Status}
        </Badge>
      ),
    },
    { header: "Storage Class", cell: (p) => <span className="text-pilot-text-secondary">{p.StorageClass || "—"}</span> },
    { header: "Capacity", align: "right", cell: (p) => <span className="text-pilot-text-secondary">{p.Capacity || "—"}</span> },
  ];
  return (
    <ResourceTable
      columns={columns}
      items={data}
      rowKey={(p) => `${p.Namespace}/${p.Name}`}
      loading={isLoading}
      error={error}
      emptyMessage="No persistent volume claims in this namespace."
      onRowClick={(p) => onViewYAML("pvc", p.Namespace, p.Name)}
    />
  );
}

function StorageClassesTab({ onViewYAML }: { onViewYAML: Props["onViewYAML"] }) {
  const { data = [], isLoading, error } = useQuery({
    queryKey: ["dash-storageclasses"],
    queryFn: () => listStorageClasses(),
  });
  const columns: Column<StorageClassInfo>[] = [
    { header: "Name", cell: (s) => <span className="text-white font-mono">{s.Name}</span> },
    { header: "Provisioner", cell: (s) => <span className="text-pilot-text-secondary font-mono text-xs">{s.Provisioner}</span> },
    { header: "Reclaim", cell: (s) => <span className="text-pilot-text-secondary">{s.ReclaimPolicy || "—"}</span> },
    { header: "Binding", cell: (s) => <span className="text-pilot-text-secondary">{s.VolumeBindingMode || "—"}</span> },
    {
      header: "Expansion",
      align: "center",
      cell: (s) => (s.AllowVolumeExpansion ? <Badge variant="success">Yes</Badge> : <span className="text-pilot-muted">No</span>),
    },
  ];
  return (
    <ResourceTable
      columns={columns}
      items={data}
      rowKey={(s) => s.Name}
      loading={isLoading}
      error={error}
      emptyMessage="No storage classes found."
      onRowClick={(s) => onViewYAML("storageclass", "", s.Name)}
    />
  );
}
