import { useQuery } from "@tanstack/react-query";
import { getNodeTargeting } from "@/lib/api";

// Lists workloads whose nodeSelector pins them to this node, with the matching
// selector shown. Loads lazily — only when a node row is expanded.
export function NodeTargeting({ node }: { node: string }) {
  const { data, isLoading, isError } = useQuery({
    queryKey: ["node-targeting", node],
    queryFn: () => getNodeTargeting(node),
  });

  if (isLoading) {
    return <span className="text-sm text-pilot-muted">Loading…</span>;
  }
  if (isError) {
    return <span className="text-sm text-pilot-danger">Failed to load targeting.</span>;
  }

  const workloads = data ?? [];
  if (workloads.length === 0) {
    return (
      <span className="text-sm text-pilot-muted">
        No workloads pin to this node via nodeSelector.
      </span>
    );
  }

  return (
    <div className="flex flex-col gap-2">
      {workloads.map((w) => (
        <div
          key={`${w.namespace}/${w.kind}/${w.name}`}
          className="flex flex-wrap items-center gap-2 text-sm"
        >
          <span className="font-mono font-semibold text-pilot-text-primary">
            {w.kind}/{w.name}
          </span>
          <span className="text-xs text-pilot-muted">{w.namespace}</span>
          <span className="text-xs text-pilot-muted tabular-nums">×{w.pods}</span>
          <span className="flex flex-wrap gap-1">
            {Object.entries(w.selector).map(([k, v]) => (
              <span
                key={k}
                className="text-xs font-mono px-2 py-0.5 rounded-md border bg-pilot-surface text-pilot-text-secondary border-pilot-border"
              >
                {k}={v}
              </span>
            ))}
          </span>
        </div>
      ))}
    </div>
  );
}
