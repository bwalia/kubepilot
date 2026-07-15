export interface NodeIPFields {
  IPs?: string[];
  LANIPs?: string[];
  WANIPs?: string[];
  TunnelIPs?: string[];
  InternalIP?: string;
  ips?: string[];
  lan_ips?: string[];
  wan_ips?: string[];
  tunnel_ips?: string[];
}

interface NodeIPDisplayProps {
  node: NodeIPFields;
}

export function NodeIPDisplay({ node }: NodeIPDisplayProps) {
  const parsed = parseInternalIPSummary(node.InternalIP);
  const lan = node.LANIPs ?? node.lan_ips ?? parsed?.lan ?? [];
  const wan = node.WANIPs ?? node.wan_ips ?? parsed?.wan ?? [];
  const tunnel = node.TunnelIPs ?? node.tunnel_ips ?? parsed?.tunnel ?? [];
  const hasStructured = lan.length > 0 || wan.length > 0 || tunnel.length > 0;

  if (hasStructured) {
    return (
      <div className="flex flex-col gap-1 text-sm font-mono text-pilot-text-secondary">
        {lan.length > 0 && (
          <div>
            <span className="text-pilot-muted text-xs uppercase tracking-wide mr-2">LAN</span>
            {lan.join(", ")}
          </div>
        )}
        {wan.length > 0 && (
          <div>
            <span className="text-pilot-muted text-xs uppercase tracking-wide mr-2">WAN</span>
            {wan.join(", ")}
          </div>
        )}
        {tunnel.length > 0 && (
          <div>
            <span className="text-pilot-muted text-xs uppercase tracking-wide mr-2">Tunnel</span>
            {tunnel.join(", ")}
          </div>
        )}
      </div>
    );
  }

  const fallback = node.IPs?.length
    ? node.IPs
    : node.ips?.length
    ? node.ips
    : node.InternalIP
    ? node.InternalIP.split(",").map((s) => s.trim())
    : [];
  if (fallback.length === 0) return <>—</>;

  return (
    <div className="flex flex-col gap-0.5 text-sm font-mono text-pilot-text-secondary">
      {fallback.map((ip) => (
        <span key={ip}>{ip}</span>
      ))}
    </div>
  );
}

/** Parse backend InternalIP summary: "LAN: 10.0.0.1 | WAN: 1.2.3.4 | Tunnel: 10.8.0.2" */
function parseInternalIPSummary(
  internalIP?: string
): { lan: string[]; wan: string[]; tunnel: string[] } | null {
  if (!internalIP || !/(LAN|WAN|Tunnel):/i.test(internalIP)) return null;

  const buckets = { lan: [] as string[], wan: [] as string[], tunnel: [] as string[] };
  for (const segment of internalIP.split("|")) {
    const trimmed = segment.trim();
    const match = trimmed.match(/^(LAN|WAN|Tunnel):\s*(.+)$/i);
    if (!match) continue;
    const ips = match[2]
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);
    const key = match[1].toLowerCase() as "lan" | "wan" | "tunnel";
    buckets[key].push(...ips);
  }

  if (buckets.lan.length === 0 && buckets.wan.length === 0 && buckets.tunnel.length === 0) {
    return null;
  }
  return buckets;
}
