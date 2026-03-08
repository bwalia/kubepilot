import type { NodeSummary } from "@/lib/api";
import { Server, AlertTriangle, CheckCircle } from "lucide-react";

interface Props {
  nodes: NodeSummary[];
  loading: boolean;
}

export function ClusterList({ nodes, loading }: Props) {
  if (loading) return <Skeleton />;

  return (
    <section>
      <h2 className="text-sm font-bold text-white mb-3 flex items-center gap-2">
        <Server className="w-4 h-4 text-pilot-accent" />
        Cluster Nodes ({nodes.length})
      </h2>
      <div className="overflow-x-auto">
        <table className="w-full text-sm border border-pilot-border rounded-lg overflow-hidden">
          <thead className="bg-pilot-surface text-pilot-muted text-xs uppercase tracking-widest">
            <tr>
              <th className="text-left px-4 py-3">Node</th>
              <th className="text-left px-4 py-3">Status</th>
              <th className="text-left px-4 py-3">CPU</th>
              <th className="text-left px-4 py-3">Memory</th>
              <th className="text-left px-4 py-3">Kubelet</th>
              <th className="text-left px-4 py-3">Pressure</th>
            </tr>
          </thead>
          <tbody>
            {nodes.map((node) => (
              <tr
                key={node.Name}
                className="border-t border-pilot-border hover:bg-pilot-surface/50 transition-colors"
              >
                <td className="px-4 py-3 font-mono text-xs text-white">{node.Name}</td>
                <td className="px-4 py-3">
                  {node.Ready ? (
                    <span className="flex items-center gap-1 text-pilot-success text-xs">
                      <CheckCircle className="w-3.5 h-3.5" /> Ready
                    </span>
                  ) : (
                    <span className="flex items-center gap-1 text-pilot-danger text-xs">
                      <AlertTriangle className="w-3.5 h-3.5" /> NotReady
                    </span>
                  )}
                </td>
                <td className="px-4 py-3 text-xs text-pilot-muted">{node.CPUCapacity}</td>
                <td className="px-4 py-3 text-xs text-pilot-muted">{node.MemoryCapacity}</td>
                <td className="px-4 py-3 text-xs text-pilot-muted">{node.KubeletVersion}</td>
                <td className="px-4 py-3">
                  <PressureBadges node={node} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
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

  if (badges.length === 0) return <span className="text-xs text-pilot-muted">—</span>;

  return (
    <div className="flex gap-1 flex-wrap">
      {badges.map((b) => (
        <span
          key={b.label}
          className="text-xs bg-pilot-warning text-black px-1.5 py-0.5 rounded font-bold"
        >
          {b.label}
        </span>
      ))}
    </div>
  );
}

function Skeleton() {
  return (
    <div className="animate-pulse space-y-2">
      {Array.from({ length: 3 }).map((_, i) => (
        <div key={i} className="h-10 bg-pilot-surface rounded" />
      ))}
    </div>
  );
}
