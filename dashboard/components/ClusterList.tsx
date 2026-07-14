import type { NodeSummary } from "@/lib/api";
import { Server, AlertTriangle, CheckCircle } from "lucide-react";

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
  if (loading) return <Skeleton />;

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
              <tr className="bg-pilot-surface-2">
                <th className="text-left px-5 py-3.5 eyebrow">Node</th>
                <th className="text-left px-5 py-3.5 eyebrow">IP Address</th>
                <th className="text-left px-5 py-3.5 eyebrow">Status</th>
                <th className="text-left px-5 py-3.5 eyebrow">CPU</th>
                <th className="text-left px-5 py-3.5 eyebrow">Memory</th>
                <th className="text-left px-5 py-3.5 eyebrow">Kubelet</th>
                <th className="text-left px-5 py-3.5 eyebrow">Pressure</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-pilot-border">
              {nodes.map((node) => (
                <tr
                  key={node.Name}
                  className="hover:bg-pilot-accent/[0.03]"
                >
                  <td className="px-5 py-3.5 font-mono text-sm text-pilot-text-primary">{node.Name}</td>
                  <td className="px-5 py-3.5 font-mono text-sm text-pilot-text-secondary">{node.InternalIP || "—"}</td>
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
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </section>
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
