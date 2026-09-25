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
import { KindHint } from "@/components/ui/StatusPill";
import { KIND_HELP } from "@/lib/k8sExplain";
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
    {
      header: "Name",
      cell: (s) => <span className="font-mono font-semibold text-pilot-text-primary">{s.name}</span>,
      sortValue: (s) => s.name,
    },
    {
      header: "Namespace",
      cell: (s) => <span className="text-pilot-text-secondary">{s.namespace}</span>,
      sortValue: (s) => s.namespace,
    },
    { header: "Type", cell: (s) => <Badge variant="muted">{s.type}</Badge>, sortValue: (s) => s.type },
    {
      header: "Cluster IP",
      cell: (s) => <span className="font-mono text-xs text-pilot-text-secondary">{s.cluster_ip || "\u2014"}</span>,
      sortValue: (s) => s.cluster_ip,
      hideBelow: "lg",
    },
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
    <>
      <KindHint {...KIND_HELP.services} />
      <ResourceTable
        columns={columns}
        items={data}
        rowKey={(s) => `${s.namespace}/${s.name}`}
        loading={isLoading}
        error={error}
        noun="service"
        searchText={(s) => `${s.namespace}/${s.name} ${s.type} ${s.cluster_ip}`}
        emptyMessage="No services in this namespace."
      />
    </>
  );
}

function IngressesTab({ namespace }: { namespace: string }) {
  const { data = [], isLoading, error } = useQuery({
    queryKey: ["dash-ingresses", namespace],
    queryFn: () => listIngresses(namespace),
  });
  const columns: Column<IngressSummary>[] = [
    {
      header: "Name",
      cell: (i) => <span className="font-mono font-semibold text-pilot-text-primary">{i.Name}</span>,
      sortValue: (i) => i.Name,
    },
    {
      header: "Namespace",
      cell: (i) => <span className="text-pilot-text-secondary">{i.Namespace}</span>,
      sortValue: (i) => i.Namespace,
    },
    {
      header: "Host",
      cell: (i) => <span className="text-pilot-text-secondary">{i.Host || "\u2014"}</span>,
      sortValue: (i) => i.Host,
    },
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
    <>
      <KindHint {...KIND_HELP.ingresses} />
      <ResourceTable
        columns={columns}
        items={data}
        rowKey={(i) => `${i.Namespace}/${i.Name}`}
        loading={isLoading}
        error={error}
        noun="ingress rule"
        searchText={(i) => `${i.Namespace}/${i.Name} ${i.Host} ${i.IngressURL ?? ""}`}
        emptyMessage="No ingress rules in this namespace."
      />
    </>
  );
}
