/**
 * ServiceMap — topology derived from OpenTelemetry traces.
 *
 * Layered left → right: callers (and the "Clients" entry point) on the left,
 * services in the middle, databases / queues / external hosts on the right.
 * Node ring = health; edge width = request rate; edge colour = error rate.
 * Hovering a node dims everything outside its 1-hop neighbourhood; clicking
 * opens the RED / pods / operations panel.
 */
import { useMemo, useState } from "react";
import { Database, Globe, MessageSquare, Server, Users, AlertTriangle, Sparkles } from "lucide-react";
import type { OTELGraphEdge, OTELGraphNode, OTELNodeKind, OTELServiceGraph } from "@/lib/api";
import { HEALTH_STROKE, fmtMs, fmtPct, fmtRate } from "./format";

const NW = 200;
const NH = 92;
const COL_GAP = 96;
const ROW_GAP = 22;
const PAD = 32;

const KIND_ICON: Record<OTELNodeKind, typeof Server> = {
  service: Server,
  database: Database,
  messaging: MessageSquare,
  external: Globe,
  clients: Users,
};

interface Placed {
  node: OTELGraphNode;
  x: number;
  y: number;
}

/** Longest-path layering from sources; cycles are broken by visit order. */
function layout(nodes: OTELGraphNode[], edges: OTELGraphEdge[]) {
  const out = new Map<string, string[]>();
  const indeg = new Map<string, number>();
  nodes.forEach((n) => {
    out.set(n.id, []);
    indeg.set(n.id, 0);
  });
  edges.forEach((e) => {
    if (!out.has(e.source) || !out.has(e.target) || e.source === e.target) return;
    out.get(e.source)!.push(e.target);
    indeg.set(e.target, (indeg.get(e.target) || 0) + 1);
  });

  const layer = new Map<string, number>();
  // Kahn's algorithm; anything left in a cycle is placed after its earliest caller.
  const queue = nodes.filter((n) => (indeg.get(n.id) || 0) === 0).map((n) => n.id);
  queue.forEach((id) => layer.set(id, 0));
  const remaining = new Map(indeg);
  while (queue.length) {
    const id = queue.shift()!;
    for (const t of out.get(id) || []) {
      layer.set(t, Math.max(layer.get(t) ?? 0, (layer.get(id) ?? 0) + 1));
      remaining.set(t, (remaining.get(t) || 0) - 1);
      if (remaining.get(t) === 0) queue.push(t);
    }
  }
  nodes.forEach((n) => {
    if (!layer.has(n.id)) layer.set(n.id, 1);
  });
  const cols = new Map<number, OTELGraphNode[]>();
  nodes.forEach((n) => {
    const l = layer.get(n.id)!;
    if (!cols.has(l)) cols.set(l, []);
    cols.get(l)!.push(n);
  });
  const placed = new Map<string, Placed>();
  let height = 0;
  const colKeys = Array.from(cols.keys()).sort((a, b) => a - b);
  const columns: OTELGraphNode[][] = [];
  colKeys.forEach((l) => {
    const col = cols.get(l)!;
    // Group by namespace, unhealthy first within a namespace.
    const rank = { critical: 0, degraded: 1, healthy: 2, unknown: 3 } as const;
    col.sort(
      (a, b) =>
        (a.namespace || "~").localeCompare(b.namespace || "~") ||
        rank[a.health] - rank[b.health] ||
        a.name.localeCompare(b.name),
    );
    columns.push(col);
  });
  // Barycenter pass: order each column by the mean row of its callers so
  // edges run mostly straight instead of criss-crossing.
  const rowOf = new Map<string, number>();
  const callers = new Map<string, string[]>();
  edges.forEach((e) => {
    if (!callers.has(e.target)) callers.set(e.target, []);
    callers.get(e.target)!.push(e.source);
  });
  columns.forEach((col, ci) => {
    if (ci > 0) {
      const bary = (n: OTELGraphNode, fallback: number) => {
        const rows = (callers.get(n.id) || []).map((c) => rowOf.get(c)).filter((r): r is number => r !== undefined);
        return rows.length ? rows.reduce((a, b) => a + b, 0) / rows.length : fallback;
      };
      const keyed = col.map((n, i) => ({ n, k: bary(n, i) }));
      keyed.sort((a, b) => a.k - b.k);
      col.splice(0, col.length, ...keyed.map((x) => x.n));
    }
    // Rows are compared in "centred" units so short and tall columns align.
    col.forEach((n, ri) => rowOf.set(n.id, ri - (col.length - 1) / 2));
  });
  const tallest = Math.max(...columns.map((c) => c.length));
  height = PAD * 2 + tallest * (NH + ROW_GAP) - ROW_GAP;
  columns.forEach((col, ci) => {
    const offset = ((tallest - col.length) * (NH + ROW_GAP)) / 2;
    col.forEach((n, ri) => {
      placed.set(n.id, { node: n, x: PAD + ci * (NW + COL_GAP), y: PAD + offset + ri * (NH + ROW_GAP) });
    });
  });
  const width = PAD * 2 + colKeys.length * (NW + COL_GAP) - COL_GAP;
  return { placed, width: Math.max(width, 600), height: Math.max(height, 160) };
}

export function ServiceMap({
  graph,
  selected,
  onSelectNode,
  onSelectEdge,
  onlyUnhealthy,
}: {
  graph: OTELServiceGraph;
  selected: string | null;
  onSelectNode: (n: OTELGraphNode) => void;
  onSelectEdge: (e: OTELGraphEdge) => void;
  onlyUnhealthy: boolean;
}) {
  const [hover, setHover] = useState<string | null>(null);

  const { nodes, edges } = useMemo(() => {
    if (!onlyUnhealthy) return { nodes: graph.nodes, edges: graph.edges };
    // Keep unhealthy nodes plus their direct neighbours for context.
    const bad = new Set(graph.nodes.filter((n) => n.health === "critical" || n.health === "degraded").map((n) => n.id));
    graph.edges.forEach((e) => {
      if (e.health === "critical" || e.health === "degraded") {
        bad.add(e.source);
        bad.add(e.target);
      }
    });
    const keep = new Set(bad);
    graph.edges.forEach((e) => {
      if (bad.has(e.source)) keep.add(e.target);
      if (bad.has(e.target)) keep.add(e.source);
    });
    return {
      nodes: graph.nodes.filter((n) => keep.has(n.id)),
      edges: graph.edges.filter((e) => keep.has(e.source) && keep.has(e.target)),
    };
  }, [graph, onlyUnhealthy]);

  const { placed, width, height } = useMemo(() => layout(nodes, edges), [nodes, edges]);
  const maxRate = Math.max(1, ...edges.map((e) => e.stats.rate_per_min));

  const focus = hover || selected;
  const neighbours = useMemo(() => {
    if (!focus) return null;
    const s = new Set([focus]);
    edges.forEach((e) => {
      if (e.source === focus) s.add(e.target);
      if (e.target === focus) s.add(e.source);
    });
    return s;
  }, [focus, edges]);

  if (nodes.length === 0) {
    return (
      <div className="h-64 flex flex-col items-center justify-center text-pilot-muted text-sm gap-2">
        <Sparkles className="w-5 h-5 text-pilot-success" />
        {onlyUnhealthy ? "Every service on the map is healthy." : "No services in this window."}
      </div>
    );
  }

  return (
    <div className="overflow-auto rounded-xl" style={{ maxHeight: "75vh" }}>
      {/* Scales down to fit the panel; scrolls only below a legible minimum. */}
      <svg
        viewBox={`0 0 ${width} ${height}`}
        width="100%"
        style={{ minWidth: Math.min(width, 960), maxWidth: width }}
        className="block mx-auto"
        role="img"
        aria-label="Service map"
      >
        <defs>
          {(["healthy", "degraded", "critical", "unknown"] as const).map((h) => (
            <marker key={h} id={`arrow-${h}`} viewBox="0 0 10 10" refX="9" refY="5" markerUnits="userSpaceOnUse" markerWidth="10" markerHeight="10" orient="auto-start-reverse">
              <path d="M 0 0 L 10 5 L 0 10 z" fill={HEALTH_STROKE[h]} />
            </marker>
          ))}
        </defs>

        {edges.map((e) => {
          const a = placed.get(e.source);
          const b = placed.get(e.target);
          if (!a || !b) return null;
          const back = b.x <= a.x;
          const x1 = a.x + NW;
          const y1 = a.y + NH / 2;
          const x2 = back ? b.x + NW / 2 : b.x - 2;
          const y2 = back ? b.y + NH + 2 : b.y + NH / 2;
          const dx = Math.max(40, (x2 - x1) / 2);
          const d = back
            ? `M ${x1} ${y1} C ${x1 + 60} ${y1 + 80}, ${x2} ${y2 + 80}, ${x2} ${y2}`
            : `M ${x1} ${y1} C ${x1 + dx} ${y1}, ${x2 - dx} ${y2}, ${x2} ${y2}`;
          const w = 1.5 + 5 * Math.sqrt(e.stats.rate_per_min / maxRate);
          const dim = neighbours && !(neighbours.has(e.source) && neighbours.has(e.target));
          const stroke = HEALTH_STROKE[e.health === "unknown" ? "unknown" : e.health];
          const midX = (x1 + x2) / 2;
          const midY = back ? Math.max(y1, y2) + 60 : (y1 + y2) / 2;
          const showLabel = !dim && (focus !== null || e.health !== "healthy");
          return (
            <g key={e.id} opacity={dim ? 0.12 : 1} className="cursor-pointer" onClick={() => onSelectEdge(e)}>
              <path d={d} fill="none" stroke="transparent" strokeWidth={14} />
              <path
                d={d}
                fill="none"
                stroke={stroke}
                strokeWidth={w}
                strokeOpacity={e.health === "healthy" ? 0.55 : 0.9}
                markerEnd={`url(#arrow-${e.health})`}
                strokeDasharray={e.health === "critical" ? "6 4" : undefined}
                className={e.health === "critical" ? "otel-flow" : undefined}
              >
                <title>
                  {`${e.source} → ${e.target}\n${fmtRate(e.stats.rate_per_min)} · ${fmtPct(e.stats.error_rate)} errors · p95 ${fmtMs(e.stats.p95_ms)}`}
                </title>
              </path>
              {showLabel && (
                <g transform={`translate(${midX}, ${midY})`}>
                  <rect x={-52} y={-11} width={104} height={22} rx={11} className="fill-pilot-surface stroke-pilot-border" />
                  <text textAnchor="middle" y={4} className="fill-pilot-text-secondary font-mono" fontSize={11}>
                    {e.stats.errors > 0 ? `${fmtPct(e.stats.error_rate)} err` : fmtRate(e.stats.rate_per_min)} · {fmtMs(e.stats.p95_ms)}
                  </text>
                </g>
              )}
            </g>
          );
        })}

        {Array.from(placed.values()).map(({ node, x, y }) => {
          const Icon = KIND_ICON[node.kind] || Server;
          const dim = neighbours && !neighbours.has(node.id);
          const isSel = selected === node.id;
          const virtual = node.kind !== "service";
          const ring = HEALTH_STROKE[node.health];
          const pods = node.pods || [];
          const badPods = pods.filter((p) => p.known && (!p.ready || p.restarts > 0)).length;
          return (
            <g
              key={node.id}
              transform={`translate(${x}, ${y})`}
              opacity={dim ? 0.25 : 1}
              className="cursor-pointer"
              onMouseEnter={() => setHover(node.id)}
              onMouseLeave={() => setHover(null)}
              onClick={() => onSelectNode(node)}
            >
              {node.health === "critical" && (
                <rect x={-5} y={-5} width={NW + 10} height={NH + 10} rx={16} fill="none" stroke={ring} strokeOpacity={0.35} strokeWidth={6} className="otel-pulse" />
              )}
              <rect
                width={NW}
                height={NH}
                rx={12}
                className="fill-pilot-surface"
                stroke={isSel ? "rgb(var(--p-accent))" : ring}
                strokeWidth={isSel ? 2.5 : node.health === "healthy" || node.health === "unknown" ? 1.25 : 2}
                strokeDasharray={virtual ? "5 4" : undefined}
              />
              <rect x={0} y={0} width={5} height={NH} rx={2.5} fill={ring} />
              <foreignObject x={12} y={8} width={NW - 20} height={NH - 14}>
                <div className="h-full flex flex-col justify-between text-left select-none">
                  <div className="flex items-center gap-1.5 min-w-0">
                    <Icon className="w-3.5 h-3.5 shrink-0 text-pilot-muted" />
                    <span className="font-semibold text-sm text-pilot-text-primary truncate">{node.name}</span>
                    {node.insight_id && <AlertTriangle className="w-3.5 h-3.5 shrink-0 text-pilot-danger" aria-label="Open insight" />}
                  </div>
                  <div className="text-2xs text-pilot-muted truncate">
                    {node.kind === "service"
                      ? [node.namespace, node.language].filter(Boolean).join(" · ") || "service"
                      : node.kind === "clients"
                        ? "external callers"
                        : `${node.system || node.kind}`}
                  </div>
                  {node.kind !== "clients" && node.stats.requests > 0 ? (
                    <div className="flex items-center gap-2 font-mono text-2xs text-pilot-text-secondary">
                      <span>{fmtRate(node.stats.rate_per_min)}</span>
                      <span className={node.stats.errors > 0 ? "text-pilot-danger font-semibold" : ""}>{fmtPct(node.stats.error_rate)}</span>
                      <span>p95 {fmtMs(node.stats.p95_ms)}</span>
                    </div>
                  ) : (
                    <div className="text-2xs text-pilot-muted">{node.kind === "clients" ? "entry point" : "no inbound traffic"}</div>
                  )}
                </div>
              </foreignObject>
              {pods.length > 0 && (
                <g transform={`translate(${NW - 12}, ${NH - 12})`}>
                  {pods.slice(0, 6).map((p, i) => (
                    <circle
                      key={p.name}
                      cx={-i * 9}
                      cy={0}
                      r={3.2}
                      fill={!p.known ? "rgb(var(--p-muted))" : p.ready && p.restarts === 0 ? HEALTH_STROKE.healthy : p.ready ? HEALTH_STROKE.degraded : HEALTH_STROKE.critical}
                    >
                      <title>{`${p.name}${p.known ? ` · ${p.phase}${p.ready ? " ready" : " NOT ready"} · ${p.restarts} restarts` : " · not in cluster"}`}</title>
                    </circle>
                  ))}
                  {badPods > 0 && (
                    <text x={-Math.min(pods.length, 6) * 9 - 2} y={4} textAnchor="end" fontSize={10} className="fill-pilot-danger font-mono">
                      {badPods}!
                    </text>
                  )}
                </g>
              )}
            </g>
          );
        })}
      </svg>
    </div>
  );
}
