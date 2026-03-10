/**
 * ServiceTopologyCanvas — ArgoCD-style visual canvas.
 *
 * Layout (left → right):
 *   Ingress  ──▶  Service  ──▶  Workload (Deployment / StatefulSet / DaemonSet)  ──▶  Pod
 *
 * SVG cubic-bezier edges connect nodes across columns. Clicking any node opens
 * a detail panel on the right.  Health is colour-coded: green / amber / red.
 */
import { useState, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  RefreshCw,
  ExternalLink,
  X,
  Shield,
  Layers,
  GitBranch,
  Box,
  Globe,
  Server,
  AlertTriangle,
  CheckCircle2,
  Clock,
  HelpCircle,
} from "lucide-react";
import {
  getServiceGraph,
  type SGNode,
  type SGEdge,
  type ServiceGraph,
} from "@/lib/api";

// ── Layout constants ──────────────────────────────────────────────────────────
const NW = 192;       // node card width  (px)
const NH = 78;        // node card height (px)
const NY_GAP = 10;    // vertical gap between nodes in the same column
const COL_GAP = 118;  // horizontal gap between columns
const PAD_X = 28;
const PAD_Y = 36;

type ColKind = "Ingress" | "Service" | "Workload" | "Pod";
const COLUMNS: ColKind[] = ["Ingress", "Service", "Workload", "Pod"];

function colOf(kind: string): ColKind {
  if (kind === "Ingress") return "Ingress";
  if (kind === "Service") return "Service";
  if (kind === "Pod") return "Pod";
  return "Workload";
}

interface LayoutNode {
  node: SGNode;
  x: number;
  y: number;
}

/** Pure-function layout: assigns pixel x,y to each node by column → row. */
function buildLayout(nodes: SGNode[]): {
  layoutMap: Map<string, LayoutNode>;
  canvasW: number;
  canvasH: number;
} {
  const byCol: Record<ColKind, SGNode[]> = {
    Ingress: [],
    Service: [],
    Workload: [],
    Pod: [],
  };
  for (const n of nodes) byCol[colOf(n.kind)].push(n);

  const layoutMap = new Map<string, LayoutNode>();

  COLUMNS.forEach((col, ci) => {
    const x = PAD_X + ci * (NW + COL_GAP);
    byCol[col].forEach((node, ri) => {
      const y = PAD_Y + ri * (NH + NY_GAP);
      layoutMap.set(node.id, { node, x, y });
    });
  });

  const maxRows = Math.max(...COLUMNS.map((c) => byCol[c].length), 1);
  const canvasH = PAD_Y * 2 + maxRows * (NH + NY_GAP);
  const canvasW = PAD_X * 2 + COLUMNS.length * NW + (COLUMNS.length - 1) * COL_GAP;

  return { layoutMap, canvasW, canvasH };
}

/** Generate an SVG cubic-bezier path between the right-center of `from` and
 *  the left-center of `to`. */
function edgePath(fx: number, fy: number, tx: number, ty: number): string {
  const mx = (fx + tx) / 2;
  return `M ${fx} ${fy} C ${mx} ${fy}, ${mx} ${ty}, ${tx} ${ty}`;
}

// ── Colour helpers ────────────────────────────────────────────────────────────
function statusRing(status: string): string {
  switch (status) {
    case "healthy":  return "#22c55e"; // green-500
    case "degraded": return "#ef4444"; // red-500
    case "pending":  return "#f59e0b"; // amber-500
    default:         return "#6b7280"; // gray-500
  }
}

const KIND_COLORS: Record<string, string> = {
  Ingress:     "#6366f1", // indigo
  Service:     "#0ea5e9", // sky
  Deployment:  "#8b5cf6", // violet
  StatefulSet: "#d97706", // amber
  DaemonSet:   "#06b6d4", // cyan
  Pod:         "#10b981", // emerald
};

function kindColor(kind: string): string {
  return KIND_COLORS[kind] ?? "#6b7280";
}

function KindIcon({ kind, size = 14 }: { kind: string; size?: number }) {
  const cls = `shrink-0`;
  const s = { width: size, height: size };
  switch (kind) {
    case "Ingress":     return <Globe     className={cls} style={s} />;
    case "Service":     return <Server    className={cls} style={s} />;
    case "Deployment":  return <Layers    className={cls} style={s} />;
    case "StatefulSet": return <GitBranch className={cls} style={s} />;
    case "DaemonSet":   return <Shield    className={cls} style={s} />;
    case "Pod":         return <Box       className={cls} style={s} />;
    default:            return <HelpCircle className={cls} style={s} />;
  }
}

function StatusIcon({ status, size = 12 }: { status: string; size?: number }) {
  const s = { width: size, height: size };
  switch (status) {
    case "healthy":  return <CheckCircle2 style={s} className="shrink-0 text-green-400" />;
    case "degraded": return <AlertTriangle style={s} className="shrink-0 text-red-400" />;
    case "pending":  return <Clock        style={s} className="shrink-0 text-amber-400" />;
    default:         return <HelpCircle   style={s} className="shrink-0 text-gray-400" />;
  }
}

// ── Node Card ─────────────────────────────────────────────────────────────────
function NodeCard({
  node,
  selected,
  onClick,
}: {
  node: SGNode;
  selected: boolean;
  onClick: () => void;
}) {
  const accent = kindColor(node.kind);
  const border = selected ? accent : "rgba(255,255,255,0.1)";

  return (
    <div
      onClick={onClick}
      title={node.name}
      style={{
        width: NW,
        height: NH,
        border: `1.5px solid ${border}`,
        boxShadow: selected ? `0 0 0 2px ${accent}55` : undefined,
      }}
      className="absolute bg-[#0f1117] rounded-lg cursor-pointer hover:brightness-125 transition-all select-none overflow-hidden"
    >
      {/* top accent bar */}
      <div style={{ height: 3, background: accent }} />

      <div className="px-3 pt-2 pb-1 flex flex-col gap-1">
        {/* kind badge + name */}
        <div className="flex items-center gap-1.5">
          <span
            className="text-[10px] font-bold uppercase tracking-wider px-1.5 py-0.5 rounded"
            style={{ color: accent, background: accent + "22" }}
          >
            {node.kind}
          </span>
          <StatusIcon status={node.status} size={11} />
          {node.kind === "Ingress" && node.tls && (
            <span className="text-[9px] font-bold text-green-400 bg-green-900/30 px-1 rounded">TLS</span>
          )}
        </div>

        {/* name */}
        <p className="text-white text-xs font-semibold truncate leading-tight">
          {node.name}
        </p>

        {/* sub-line */}
        <div className="text-[10px] text-gray-400 truncate leading-tight">
          {node.kind === "Pod" && (
            <>
              {node.phase}
              {node.restarts && node.restarts > 0
                ? <span className="text-red-400 ml-1">↻{node.restarts}</span>
                : null}
              {node.node_name && <span className="ml-1">· {node.node_name}</span>}
            </>
          )}
          {(node.kind === "Deployment" || node.kind === "StatefulSet" || node.kind === "DaemonSet") && (
            <>{node.ready_replicas ?? 0}/{node.replicas ?? 0} ready</>
          )}
          {node.kind === "Service" && node.ports && node.ports.length > 0 && (
            <>{node.ports.map((p) => p.port).join(", ")} · {node.service_type}</>
          )}
          {node.kind === "Ingress" && (
            <>{node.host || "(no host)"}</>
          )}
        </div>
      </div>
    </div>
  );
}

// ── Detail Panel ──────────────────────────────────────────────────────────────
function DetailPanel({
  node,
  onClose,
}: {
  node: SGNode;
  onClose: () => void;
}) {
  const accent = kindColor(node.kind);
  const imgShort =
    node.image && node.image.length > 48
      ? "…" + node.image.slice(-48)
      : node.image ?? "";

  return (
    <div className="w-80 shrink-0 bg-[#0f1117] border-l border-white/10 flex flex-col">
      {/* header */}
      <div
        className="flex items-center justify-between px-4 py-3 border-b"
        style={{ borderColor: accent + "55" }}
      >
        <div className="flex items-center gap-2">
          <KindIcon kind={node.kind} size={16} />
          <span className="font-bold text-white text-sm truncate max-w-[180px]">{node.name}</span>
        </div>
        <button
          onClick={onClose}
          className="text-gray-400 hover:text-white p-1 rounded hover:bg-white/10"
        >
          <X className="w-4 h-4" />
        </button>
      </div>

      <div className="flex-1 overflow-y-auto px-4 py-3 space-y-4 text-xs">
        {/* status row */}
        <Row label="Status">
          <span className="flex items-center gap-1.5">
            <StatusIcon status={node.status} size={11} />
            <span className="capitalize">{node.status}</span>
          </span>
        </Row>
        <Row label="Namespace">{node.namespace}</Row>
        <Row label="Kind">
          <span style={{ color: accent }}>{node.kind}</span>
        </Row>

        {/* Ingress-specific */}
        {node.kind === "Ingress" && (
          <>
            {node.host && <Row label="Host">{node.host}</Row>}
            <Row label="TLS">{node.tls ? "Enabled ✓" : "Disabled"}</Row>
            {node.ingress_url && (
              <Row label="URL">
                <a
                  href={node.ingress_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center gap-1 text-sky-400 hover:underline"
                >
                  {node.ingress_url}
                  <ExternalLink className="w-3 h-3" />
                </a>
              </Row>
            )}
          </>
        )}

        {/* Service-specific */}
        {node.kind === "Service" && (
          <>
            <Row label="Type">{node.service_type}</Row>
            <Row label="Cluster IP">{node.cluster_ip || "—"}</Row>
            {node.external_ips && node.external_ips.length > 0 && (
              <Row label="External IPs">
                <div className="space-y-0.5">
                  {node.external_ips.map((ip) => (
                    <div key={ip} className="text-green-300">{ip}</div>
                  ))}
                </div>
              </Row>
            )}
            {node.ports && node.ports.length > 0 && (
              <div>
                <p className="text-gray-500 uppercase tracking-wider text-[10px] mb-1.5">Ports</p>
                <div className="space-y-1">
                  {node.ports.map((p, i) => (
                    <div
                      key={i}
                      className="bg-white/5 rounded px-2 py-1.5 flex items-center justify-between"
                    >
                      <span className="text-gray-300 font-mono">
                        {p.name || `port-${i}`}
                      </span>
                      <span className="text-sky-300 font-mono">
                        {p.port}
                        {p.target_port !== String(p.port) && (
                          <span className="text-gray-500">→{p.target_port}</span>
                        )}
                        {p.node_port ? (
                          <span className="text-amber-400"> NP:{p.node_port}</span>
                        ) : null}
                        <span className="text-gray-500 ml-1">{p.protocol}</span>
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </>
        )}

        {/* Workload-specific */}
        {(node.kind === "Deployment" ||
          node.kind === "StatefulSet" ||
          node.kind === "DaemonSet") && (
          <>
            <Row label="Replicas">
              <span>
                <span className="text-green-400">{node.ready_replicas ?? 0}</span>
                <span className="text-gray-500">/{node.replicas ?? 0}</span>
                <span className="text-gray-500 ml-1">ready</span>
              </span>
            </Row>
            {imgShort && (
              <Row label="Image">
                <span className="font-mono break-all text-sky-300">{imgShort}</span>
              </Row>
            )}
          </>
        )}

        {/* Pod-specific */}
        {node.kind === "Pod" && (
          <>
            <Row label="Phase">{node.phase}</Row>
            <Row label="Ready">{node.ready ? "Yes ✓" : "No ✗"}</Row>
            <Row label="Restarts">
              <span className={node.restarts && node.restarts > 0 ? "text-red-400" : ""}>
                {node.restarts ?? 0}
              </span>
            </Row>
            {node.node_name && <Row label="Node">{node.node_name}</Row>}
          </>
        )}

        {/* Labels */}
        {node.labels && Object.keys(node.labels).length > 0 && (
          <div>
            <p className="text-gray-500 uppercase tracking-wider text-[10px] mb-1.5">Labels</p>
            <div className="flex flex-wrap gap-1">
              {Object.entries(node.labels).map(([k, v]) => (
                <span
                  key={k}
                  className="bg-white/5 rounded px-1.5 py-0.5 font-mono text-[10px] text-gray-300"
                >
                  {k}={v}
                </span>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function Row({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex items-start gap-2">
      <span className="text-gray-500 shrink-0 w-24">{label}</span>
      <span className="text-gray-200 break-all flex-1">{children}</span>
    </div>
  );
}

// ── Legend ─────────────────────────────────────────────────────────────────────
function Legend() {
  return (
    <div className="flex items-center gap-4 px-4 py-2 border-t border-white/10 text-[10px] text-gray-500 flex-wrap">
      {Object.entries(KIND_COLORS).map(([kind, color]) => (
        <span key={kind} className="flex items-center gap-1">
          <span
            className="w-2.5 h-2.5 rounded-sm inline-block"
            style={{ background: color }}
          />
          {kind}
        </span>
      ))}
      <span className="ml-auto flex items-center gap-3">
        <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-green-500 inline-block" /> Healthy</span>
        <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-amber-500 inline-block" /> Pending</span>
        <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-red-500 inline-block" /> Degraded</span>
      </span>
    </div>
  );
}

// ── Column header ─────────────────────────────────────────────────────────────
function ColHeader({ label, count, color }: { label: string; count: number; color: string }) {
  return (
    <div
      className="absolute flex items-center gap-1.5 text-[10px] font-bold uppercase tracking-widest"
      style={{ color }}
    >
      {label}
      {count > 0 && (
        <span
          className="text-[9px] rounded-full px-1.5 py-0.5 font-mono"
          style={{ background: color + "28", color }}
        >
          {count}
        </span>
      )}
    </div>
  );
}

// ── Main Component ────────────────────────────────────────────────────────────
export function ServiceTopologyCanvas() {
  const [namespace, setNamespace] = useState("default");
  const [nsInput, setNsInput] = useState("default");
  const [selected, setSelected] = useState<SGNode | null>(null);

  const normalizeNamespace = (value: string): string => {
    const ns = value.trim();
    if (ns === "" || ns === "all" || ns === "*") {
      return "";
    }
    return ns;
  };

  const { data, isLoading, isError, refetch } = useQuery<ServiceGraph>({
    queryKey: ["service-graph", namespace],
    queryFn: () => getServiceGraph(namespace),
    refetchInterval: 30_000,
  });

  const nodes = Array.isArray(data?.nodes) ? data.nodes : [];
  const edges = Array.isArray(data?.edges) ? data.edges : [];

  const { layoutMap, canvasW, canvasH } = useMemo(() => {
    if (nodes.length === 0) return { layoutMap: new Map(), canvasW: 900, canvasH: 200 };
    return buildLayout(nodes);
  }, [nodes]);

  const svgEdges = useMemo<{ path: string; key: string }[]>(() => {
    if (edges.length === 0) return [];
    return edges.flatMap((e: SGEdge) => {
      const src = layoutMap.get(e.from);
      const tgt = layoutMap.get(e.to);
      if (!src || !tgt) return [];
      const fx = src.x + NW;
      const fy = src.y + NH / 2;
      const tx = tgt.x;
      const ty = tgt.y + NH / 2;
      return [{ path: edgePath(fx, fy, tx, ty), key: e.from + "->" + e.to }];
    });
  }, [edges, layoutMap]);

  // Column node counts for headers
  const counts = useMemo(() => {
    const c: Record<ColKind, number> = { Ingress: 0, Service: 0, Workload: 0, Pod: 0 };
    nodes.forEach((n) => { c[colOf(n.kind)]++; });
    return c;
  }, [nodes]);

  const handleNsSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setNamespace(normalizeNamespace(nsInput));
    setSelected(null);
  };

  const activeNamespaceLabel = namespace === "" ? "all namespaces" : namespace;

  return (
    <div className="flex flex-col h-full bg-[#080a0f] rounded-lg border border-white/10 overflow-hidden">
      {/* ── toolbar ── */}
      <div className="flex items-center gap-3 px-4 py-2.5 border-b border-white/10 shrink-0">
        <Layers className="w-4 h-4 text-sky-400" />
        <span className="text-sm font-bold text-white tracking-wide">Service Topology</span>
        <button
          type="button"
          onClick={() => {
            setNamespace("");
            setNsInput("all");
            setSelected(null);
          }}
          className={`text-xs px-2.5 py-1 rounded border ${
            namespace === ""
              ? "bg-sky-600 text-white border-sky-500"
              : "bg-white/5 text-gray-300 border-white/10 hover:bg-white/10"
          }`}
        >
          All Namespaces
        </button>
        <form onSubmit={handleNsSubmit} className="flex items-center gap-2 ml-2">
          <input
            value={nsInput}
            onChange={(e) => setNsInput(e.target.value)}
            placeholder="Namespace (or all)"
            className="bg-white/5 border border-white/10 rounded px-3 py-1 text-xs text-white placeholder:text-gray-500 focus:outline-none focus:border-sky-500 w-36"
          />
          <button
            type="submit"
            className="text-xs bg-sky-600 hover:bg-sky-500 text-white px-3 py-1 rounded font-semibold"
          >
            Go
          </button>
        </form>
        <button
          onClick={() => refetch()}
          className="ml-auto p-1.5 rounded hover:bg-white/10 text-gray-400 hover:text-white"
          title="Refresh"
        >
          <RefreshCw className={`w-4 h-4 ${isLoading ? "animate-spin" : ""}`} />
        </button>
      </div>

      {/* ── canvas + detail panel ── */}
      <div className="flex flex-1 min-h-0 overflow-hidden">
        {/* scrollable canvas area */}
        <div className="flex-1 overflow-auto p-4">
          {isLoading && (
            <div className="flex items-center justify-center h-40 text-gray-500 text-sm">
              Loading topology…
            </div>
          )}
          {isError && (
            <div className="flex items-center justify-center h-40 text-red-400 text-sm">
              Failed to load service graph.
            </div>
          )}
          {!isLoading && !isError && data && (
            nodes.length === 0 ? (
              <div className="flex items-center justify-center h-40 text-gray-500 text-sm">
                No resources found in {activeNamespaceLabel}.
              </div>
            ) : (
              /* Canvas: absolute-positioned nodes + SVG edge overlay */
              <div
                style={{ position: "relative", width: canvasW, height: canvasH }}
                className="select-none"
              >
                {/* Column headers */}
                {COLUMNS.map((col, ci) => {
                  const x = PAD_X + ci * (NW + COL_GAP);
                  return (
                    <div
                      key={col}
                      style={{ position: "absolute", left: x, top: 8, width: NW }}
                    >
                      <ColHeader
                        label={col}
                        count={counts[col]}
                        color={KIND_COLORS[
                          col === "Workload"
                            ? "Deployment"
                            : col === "Pod"
                            ? "Pod"
                            : col
                        ]}
                      />
                    </div>
                  );
                })}

                {/* SVG edge layer (behind nodes) */}
                <svg
                  style={{ position: "absolute", inset: 0, pointerEvents: "none", overflow: "visible" }}
                  width={canvasW}
                  height={canvasH}
                >
                  {svgEdges.map(({ path, key }) => (
                    <path
                      key={key}
                      d={path}
                      fill="none"
                      stroke="rgba(255,255,255,0.12)"
                      strokeWidth={1.5}
                    />
                  ))}
                  {/* Highlighted edges for selected node */}
                  {selected &&
                    edges
                      .filter((e) => e.from === selected.id || e.to === selected.id)
                      .flatMap((e) => {
                        const src = layoutMap.get(e.from);
                        const tgt = layoutMap.get(e.to);
                        if (!src || !tgt) return [];
                        const fx = src.x + NW;
                        const fy = src.y + NH / 2;
                        const tx = tgt.x;
                        const ty = tgt.y + NH / 2;
                        const col = kindColor(selected.kind);
                        return [
                          <path
                            key={e.from + "->" + e.to + "-hl"}
                            d={edgePath(fx, fy, tx, ty)}
                            fill="none"
                            stroke={col}
                            strokeWidth={2}
                            opacity={0.7}
                          />,
                        ];
                      })}
                </svg>

                {/* Node cards */}
                {nodes.map((node) => {
                  const ln = layoutMap.get(node.id);
                  if (!ln) return null;
                  return (
                    <div
                      key={node.id}
                      style={{ position: "absolute", left: ln.x, top: ln.y + 20 }}
                    >
                      <NodeCard
                        node={node}
                        selected={selected?.id === node.id}
                        onClick={() =>
                          setSelected((prev) => (prev?.id === node.id ? null : node))
                        }
                      />
                    </div>
                  );
                })}
              </div>
            )
          )}
        </div>

        {/* Detail panel */}
        {selected && (
          <DetailPanel node={selected} onClose={() => setSelected(null)} />
        )}
      </div>

      <Legend />
    </div>
  );
}
