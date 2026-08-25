import { Fragment, useState } from "react";
import type { NodeSummary } from "@/lib/api";
import { NodeIPDisplay } from "@/components/NodeIPDisplay";
import { NodeRoleBadge } from "@/components/NodeRoleBadge";
import { NodeLabels } from "@/components/NodeLabels";
import { NodeTargeting } from "@/components/NodeTargeting";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Server, AlertTriangle, CheckCircle, ChevronRight, ChevronDown, Cpu, Boxes, Crosshair } from "lucide-react";

interface Props {
  nodes: NodeSummary[];
  loading: boolean;
}

// Convert a Kubernetes memory quantity (e.g. "16012345Ki", "16Gi", "2048Mi",
// or a plain byte count) to a human-readable GB string. Kubernetes reports
// memory in binary units, so we use GiB (1024^3 bytes) and label it "GB" to
// match common dashboard convention.
function formatMemoryGB(raw: string): string {
  if (!raw) return "—";
  const match = raw.trim().match(/^(\d+(?:\.\d+)?)\s*([A-Za-z]*)$/);
  if (!match) return raw;
  const value = parseFloat(match[1]);
  const binaryUnit: Record<string, number> = {
    "": 1,
    Ki: 1024,
    Mi: 1024 ** 2,
    Gi: 1024 ** 3,
    Ti: 1024 ** 4,
    // decimal SI units, just in case
    k: 1e3,
    M: 1e6,
    G: 1e9,
    T: 1e12,
  };
  const factor = binaryUnit[match[2]] ?? 1;
  const gb = (value * factor) / 1024 ** 3;
  return `${gb.toFixed(1)} GB`;
}

export function ClusterList({ nodes, loading }: Props) {
  const [expanded, setExpanded] = useState<string | null>(null);

  if (loading) return <Skeleton />;

  const toggle = (name: string) =>
    setExpanded((cur) => (cur === name ? null : name));

  return (
    <section>
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-base font-bold font-display text-pilot-text-primary flex items-center gap-2">
          <Server className="w-5 h-5 text-pilot-accent" />
          Cluster Nodes
        </h2>
        <span className="text-sm text-pilot-muted font-medium tabular-nums">{nodes.length} total</span>
      </div>
      <div className="bg-pilot-surface border border-pilot-border rounded-xl overflow-hidden shadow-card">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr>
                <th className="w-8 px-2 py-3.5" aria-label="Expand" />
                <th className="text-left px-5 py-3.5 eyebrow">Node</th>
                <th className="text-left px-5 py-3.5 eyebrow">Role</th>
                <th className="text-left px-5 py-3.5 eyebrow">IP Address</th>
                <th className="text-left px-5 py-3.5 eyebrow">Status</th>
                <th className="text-left px-5 py-3.5 eyebrow">CPU</th>
                <th className="text-left px-5 py-3.5 eyebrow">Memory</th>
                <th className="text-left px-5 py-3.5 eyebrow">Kubelet</th>
                <th className="text-left px-5 py-3.5 eyebrow">Pressure</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-pilot-border">
              {nodes.map((node) => {
                const isOpen = expanded === node.Name;
                return (
                  <Fragment key={node.Name}>
                    <tr
                      className="hover:bg-pilot-accent/[0.03] cursor-pointer"
                      onClick={() => toggle(node.Name)}
                    >
                      <td className="px-2 py-3.5 text-pilot-muted">
                        {isOpen ? (
                          <ChevronDown className="w-4 h-4" />
                        ) : (
                          <ChevronRight className="w-4 h-4" />
                        )}
                      </td>
                      <td className="px-5 py-3.5">
                        <div className="font-mono text-sm font-semibold text-pilot-text-primary">{node.Name}</div>
                        {node.Hardware && (
                          <div className="text-xs text-pilot-muted mt-0.5">{node.Hardware}</div>
                        )}
                      </td>
                      <td className="px-5 py-3.5">
                        <NodeRoleBadge node={node} />
                      </td>
                      <td className="px-5 py-3.5">
                        <NodeIPDisplay node={node} />
                      </td>
                      <td className="px-5 py-3.5">
                        {node.Ready ? (
                          <span className="inline-flex items-center gap-1.5 text-pilot-success text-sm font-medium">
                            <CheckCircle className="w-4 h-4" /> Ready
                          </span>
                        ) : (
                          <span className="inline-flex items-center gap-1.5 text-pilot-danger text-sm font-medium">
                            <AlertTriangle className="w-4 h-4" /> NotReady
                          </span>
                        )}
                      </td>
                      <td className="px-5 py-3.5 text-sm text-pilot-text-secondary">{node.CPUCapacity}</td>
                      <td className="px-5 py-3.5 text-sm text-pilot-text-secondary">{formatMemoryGB(node.MemoryCapacity)}</td>
                      <td className="px-5 py-3.5 text-sm text-pilot-text-secondary font-mono">{node.KubeletVersion}</td>
                      <td className="px-5 py-3.5">
                        <PressureBadges node={node} />
                      </td>
                    </tr>
                    {isOpen && (
                      <tr className="bg-pilot-accent/[0.02]">
                        <td colSpan={9} className="px-5 py-4">
                          <NodeDetailTabs node={node} />
                        </td>
                      </tr>
                    )}
                  </Fragment>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}

// The node detail splits into what the machine is (physical facts the node
// agent reads off the hardware) and what Kubernetes made of it (nodeInfo,
// labels, scheduling). Keeping them in separate tabs is the difference between
// "which of my boxes is this?" and "why did this pod land here?".
function NodeDetailTabs({ node }: { node: NodeSummary }) {
  return (
    <Tabs defaultValue="hardware">
      <TabsList>
        <TabsTrigger value="hardware">
          <Cpu className="w-3.5 h-3.5" /> Hardware
        </TabsTrigger>
        <TabsTrigger value="kubernetes">
          <Boxes className="w-3.5 h-3.5" /> Kubernetes
        </TabsTrigger>
        <TabsTrigger value="workloads">
          <Crosshair className="w-3.5 h-3.5" /> Workloads
        </TabsTrigger>
      </TabsList>

      <TabsContent value="hardware">
        <InfoGrid rows={hardwareRows(node)} />
        {!node.Hardware && (
          <p className="mt-3 text-xs text-pilot-muted max-w-3xl">
            No hardware facts for this node. Kubernetes never reads them — the
            KubePilot node agent does. Install it with the chart
            (<code className="font-mono">nodeAgent.enabled=true</code>), or set them by hand:{" "}
            <code className="font-mono break-all">
              kubectl annotate node {node.Name} kubepilot.io/hardware=&quot;HP ProLiant DL380 Gen9&quot;
            </code>
          </p>
        )}
      </TabsContent>

      <TabsContent value="kubernetes">
        <InfoGrid rows={kubernetesRows(node)} />
        <p className="eyebrow mt-4 mb-2">Labels</p>
        <NodeLabels node={node} />
      </TabsContent>

      <TabsContent value="workloads">
        <p className="eyebrow mb-2">Targeted by (nodeSelector)</p>
        <NodeTargeting node={node.Name} />
      </TabsContent>
    </Tabs>
  );
}

type Row = [label: string, value: string];

// Display order and naming for the facts the node agent publishes as
// kubepilot.io/hw.* annotations.
const HARDWARE_FIELDS: Row[] = [
  ["cpu", "CPU"],
  ["cores", "Cores"],
  ["memory", "Memory"],
  ["disks", "Disks"],
  ["nics", "Network"],
  ["chassis", "Chassis"],
  ["board", "Motherboard"],
  ["bios", "BIOS"],
  ["virtualization", "Virtualisation"],
];

function hardwareRows(node: NodeSummary): Row[] {
  const info = node.HardwareInfo ?? {};
  const named = new Set(HARDWARE_FIELDS.map(([key]) => key));
  return [
    ...(node.Hardware ? ([["Model", node.Hardware]] as Row[]) : []),
    ...(node.Serial ? ([["Serial", node.Serial]] as Row[]) : []),
    ...HARDWARE_FIELDS.filter(([key]) => info[key]).map(([key, label]) => [label, info[key]] as Row),
    // Anything the agent starts publishing later appears without a UI change.
    ...Object.keys(info)
      .filter((key) => !named.has(key))
      .sort()
      .map((key) => [key, info[key]] as Row),
  ];
}

function kubernetesRows(node: NodeSummary): Row[] {
  return (
    [
      ["OS", node.OSImage],
      ["Kernel", node.KernelVersion],
      ["Architecture", node.Architecture],
      ["Container runtime", node.ContainerRuntime],
      ["Kubelet", node.KubeletVersion],
      ["CPU capacity", node.CPUCapacity],
      ["Memory capacity", formatMemoryGB(node.MemoryCapacity)],
      ["Roles", node.Roles?.join(", ")],
      ["Scheduling", node.Unschedulable ? "Cordoned (unschedulable)" : "Schedulable"],
    ] as Array<[string, string | undefined]>
  ).filter((row): row is Row => Boolean(row[1]));
}

function InfoGrid({ rows }: { rows: Row[] }) {
  if (rows.length === 0) return <p className="text-sm text-pilot-muted">No data.</p>;

  return (
    <dl className="grid grid-cols-[8rem_1fr] lg:grid-cols-[8rem_1fr_8rem_1fr] gap-x-6 gap-y-1.5 text-sm">
      {rows.map(([label, value]) => (
        <Fragment key={label}>
          <dt className="text-pilot-muted">{label}</dt>
          <dd className="font-mono text-pilot-text-secondary break-all">{value}</dd>
        </Fragment>
      ))}
    </dl>
  );
}

function PressureBadges({ node }: { node: NodeSummary }) {
  const badges = [
    { label: "Memory", active: node.MemoryPressure },
    { label: "Disk", active: node.DiskPressure },
    { label: "PID", active: node.PIDPressure },
  ].filter((b) => b.active);

  if (badges.length === 0) return <span className="text-sm text-pilot-muted">None</span>;

  return (
    <div className="flex gap-1.5 flex-wrap">
      {badges.map((b) => (
        <span
          key={b.label}
          className="text-xs bg-pilot-warning/20 text-pilot-warning px-2 py-0.5 rounded-md font-semibold border border-pilot-warning/30"
        >
          {b.label}
        </span>
      ))}
    </div>
  );
}

function Skeleton() {
  return (
    <div className="space-y-3">
      <div className="h-5 w-40 bg-pilot-surface rounded animate-pulse" />
      {Array.from({ length: 3 }).map((_, i) => (
        <div key={i} className="h-14 bg-pilot-surface rounded-xl animate-pulse" />
      ))}
    </div>
  );
}
