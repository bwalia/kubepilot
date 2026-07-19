interface NodeLabelFields {
  Labels?: Record<string, string>;
  labels?: Record<string, string>;
}

// The authoritative LAN-IP label stamped by the node-labeler DaemonSet — worth
// highlighting since it drives the LAN address shown for the node.
const LAN_IP_LABEL = "kubepilot.io/lan-ip";

export function NodeLabels({ node }: { node: NodeLabelFields }) {
  const labels = node.Labels ?? node.labels ?? {};
  const entries = Object.entries(labels).sort(([a], [b]) => a.localeCompare(b));

  if (entries.length === 0) {
    return <span className="text-sm text-pilot-muted">No labels</span>;
  }

  return (
    <div className="flex flex-wrap gap-1.5">
      {entries.map(([key, value]) => {
        const highlight = key === LAN_IP_LABEL;
        return (
          <span
            key={key}
            className={
              highlight
                ? "inline-flex items-center text-xs font-mono px-2 py-0.5 rounded-md border bg-pilot-accent/15 text-pilot-accent border-pilot-accent/30"
                : "inline-flex items-center text-xs font-mono px-2 py-0.5 rounded-md border bg-pilot-surface text-pilot-text-secondary border-pilot-border"
            }
          >
            <span className={highlight ? "text-pilot-accent/80" : "text-pilot-muted"}>{key}</span>
            {value ? (
              <>
                <span className="mx-0.5 text-pilot-muted">=</span>
                <span className="font-semibold">{value}</span>
              </>
            ) : null}
          </span>
        );
      })}
    </div>
  );
}
