import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  listServiceEndpoints,
  listIngresses,
  type ServiceEndpointSummary,
  type IngressSummary,
} from "@/lib/api";
import { ResourceTable, type Column } from "./ResourceTable";
import { SubTabs } from "./SubTabs";
import { Badge } from "@/components/ui/badge";
import { PortForwardButton } from "@/components/PortForwardButton";
import { ExternalLink } from "lucide-react";

type NetTab = "services" | "ingresses";

const TABS: { key: NetTab; label: string }[] = [
  { key: "services", label: "Services" },
  { key: "ingresses", label: "Ingresses" },
];

export function NetworkSection({
  namespace,
  mutationsEnabled = false,
}: {
  namespace: string;
  mutationsEnabled?: boolean;
}) {
  const [tab, setTab] = useState<NetTab>("services");
  return (
    <div>
      <SubTabs tabs={TABS} active={tab} onChange={setTab} />
      {tab === "services" && <ServicesTab namespace={namespace} mutationsEnabled={mutationsEnabled} />}
      {tab === "ingresses" && <IngressesTab namespace={namespace} />}
    </div>
  );
}

function ServicesTab({ namespace, mutationsEnabled }: { namespace: string; mutationsEnabled: boolean }) {
  const { data = [], isLoading, error } = useQuery({
    queryKey: ["dash-services", namespace],
    queryFn: () => listServiceEndpoints(namespace),
  });
  const columns: Column<ServiceEndpointSummary>[] = [
    { header: "Namespace", cell: (s) => <span className="text-pilot-text-secondary">{s.namespace}</span> },
    { header: "Name", cell: (s) => <span className="text-white font-mono">{s.name}</span> },
    { header: "Type", cell: (s) => <Badge variant="muted">{s.type}</Badge> },
    { header: "Cluster IP", cell: (s) => <span className="text-pilot-text-secondary font-mono text-xs">{s.cluster_ip || "—"}</span> },
    {
      header: "Ports",
      cell: (s) => (
        <span className="text-pilot-muted font-mono text-xs">
          {(s.ports || []).map((p) => `${p.port}/${p.protocol}`).join(", ") || "—"}
        </span>
      ),
    },
    {
      header: "Actions",
      align: "right",
      cell: (s) => (
        <PortForwardButton
          kind="service"
          namespace={s.namespace}
          name={s.name}
          availablePorts={(s.ports || []).map((p) => p.port)}
          mutationsEnabled={mutationsEnabled}
        />
      ),
    },
  ];
  return (
    <ResourceTable
      columns={columns}
      items={data}
      rowKey={(s) => `${s.namespace}/${s.name}`}
      loading={isLoading}
      error={error}
      emptyMessage="No services in this namespace."
    />
  );
}

function IngressesTab({ namespace }: { namespace: string }) {
  const { data = [], isLoading, error } = useQuery({
    queryKey: ["dash-ingresses", namespace],
    queryFn: () => listIngresses(namespace),
  });
  const columns: Column<IngressSummary>[] = [
    { header: "Namespace", cell: (i) => <span className="text-pilot-text-secondary">{i.Namespace}</span> },
    { header: "Name", cell: (i) => <span className="text-white font-mono">{i.Name}</span> },
    { header: "Host", cell: (i) => <span className="text-pilot-text-secondary">{i.Host || "—"}</span> },
    {
      header: "URL",
      cell: (i) =>
        i.IngressURL ? (
          <a
            href={i.IngressURL}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1 text-pilot-accent-light hover:underline"
            onClick={(e) => e.stopPropagation()}
          >
            {i.IngressURL}
            <ExternalLink className="w-3 h-3" />
          </a>
        ) : (
          <span className="text-pilot-muted">—</span>
        ),
    },
    { header: "TLS", align: "center", cell: (i) => (i.TLS ? <Badge variant="success">TLS</Badge> : <span className="text-pilot-muted">—</span>) },
  ];
  return (
    <ResourceTable
      columns={columns}
      items={data}
      rowKey={(i) => `${i.Namespace}/${i.Name}`}
      loading={isLoading}
      error={error}
      emptyMessage="No ingresses in this namespace."
    />
  );
}
