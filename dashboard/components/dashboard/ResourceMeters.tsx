/**
 * ResourceMeters — physically-themed cluster resource visualizations for CPU,
 * memory and storage. Replaces flat gauges with object metaphors:
 *   - CPU   → a processor die with a grid of cores that light up by load
 *   - RAM   → a stack of DIMM modules that fill up with usage
 *   - Disk  → a 3D storage cylinder with a liquid fill level
 * Data comes from /api/v1/troubleshooting/summary (resource_pressure).
 */
import { useQuery } from "@tanstack/react-query";
import { Cpu, MemoryStick, HardDrive, Database } from "lucide-react";
import {
  getClusterTroubleshootingSummary,
  type ResourcePressureSummary,
  type StorageClassSummary,
} from "@/lib/api";

function formatBytes(bytes: number | undefined): string {
  if (!bytes || bytes <= 0) return "0";
  const units = ["B", "KiB", "MiB", "GiB", "TiB", "PiB"];
  let value = bytes;
  let i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i++;
  }
  return `${value.toFixed(value >= 100 ? 0 : 1)} ${units[i]}`;
}

function formatCPU(milli: number | undefined): string {
  if (!milli || milli <= 0) return "0";
  return milli >= 1000 ? `${(milli / 1000).toFixed(1)} cores` : `${milli}m`;
}

// green < 60%, amber < 85%, red >= 85%
function colorFor(p: number): string {
  return p >= 85 ? "#ef4444" : p >= 60 ? "#f59e0b" : "#10b981";
}

interface MeterShellProps {
  label: string;
  icon: React.ReactNode;
  color: string;
  percent: number;
  primary: string;
  secondary: string;
  children: React.ReactNode;
}

function MeterShell({ label, icon, color, percent, primary, secondary, children }: MeterShellProps) {
  const clamped = Math.max(0, Math.min(100, Math.round(percent)));
  return (
    <div className="bg-pilot-surface border border-pilot-border rounded-xl p-4 shadow-card hover:shadow-card-hover transition-all">
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <span style={{ color }}>{icon}</span>
          <span className="text-xs uppercase tracking-wider text-pilot-muted font-medium">{label}</span>
        </div>
        <span className="text-2xl font-bold tracking-tight tabular-nums" style={{ color }}>
          {clamped}%
        </span>
      </div>
      <div className="flex items-center justify-center h-36">{children}</div>
      <div className="flex items-center justify-between mt-3 text-xs">
        <span className="text-pilot-text-secondary font-mono">{primary}</span>
        <span className="text-pilot-muted font-mono">{secondary}</span>
      </div>
    </div>
  );
}

/** CPU processor die — a grid of cores that light up proportionally to load. */
function CpuDie({ percent, color }: { percent: number; color: string }) {
  const cols = 6;
  const rows = 6;
  const total = cols * rows;
  const lit = Math.round((Math.max(0, Math.min(100, percent)) / 100) * total);
  const gap = 3;
  const size = 14;
  const dieW = cols * size + (cols + 1) * gap;
  const dieH = rows * size + (rows + 1) * gap;
  const pinLen = 6;

  const pinOffsets = Array.from({ length: cols }).map((_, i) => gap + size / 2 + i * (size + gap));

  return (
    <svg viewBox={`${-pinLen - 2} ${-pinLen - 2} ${dieW + 2 * pinLen + 4} ${dieH + 2 * pinLen + 4}`} className="h-full">
      {/* connector pins along top and bottom edges */}
      {pinOffsets.map((off, i) => (
        <g key={`pin-${i}`}>
          <rect x={off - 1.5} y={-pinLen} width={3} height={pinLen} fill="#2a3f66" rx={1} />
          <rect x={off - 1.5} y={dieH} width={3} height={pinLen} fill="#2a3f66" rx={1} />
        </g>
      ))}
      {/* die body */}
      <rect x={0} y={0} width={dieW} height={dieH} rx={6} fill="#0e1626" stroke="#2a3f66" strokeWidth={1.5} />
      {/* cores */}
      {Array.from({ length: total }).map((_, i) => {
        const c = i % cols;
        const r = Math.floor(i / cols);
        const on = i < lit;
        return (
          <rect
            key={i}
            x={gap + c * (size + gap)}
            y={gap + r * (size + gap)}
            width={size}
            height={size}
            rx={2}
            fill={on ? color : "rgba(255,255,255,0.05)"}
            opacity={on ? 0.9 : 1}
          >
            {on && <animate attributeName="opacity" values="0.55;0.95;0.55" dur={`${1.6 + (i % 5) * 0.25}s`} repeatCount="indefinite" />}
          </rect>
        );
      })}
    </svg>
  );
}

/** RAM — vertical DIMM modules that fill from the bottom with usage. */
function RamSticks({ percent, color }: { percent: number; color: string }) {
  const sticks = 4;
  const stickW = 18;
  const stickH = 96;
  const gap = 10;
  const totalW = sticks * stickW + (sticks - 1) * gap;
  const p = Math.max(0, Math.min(100, percent)) / 100;

  return (
    <svg viewBox={`-6 -8 ${totalW + 12} ${stickH + 24}`} className="h-full">
      {Array.from({ length: sticks }).map((_, i) => {
        const x = i * (stickW + gap);
        const fillH = stickH * p;
        return (
          <g key={i}>
            {/* module board */}
            <rect x={x} y={0} width={stickW} height={stickH} rx={3} fill="#0e1626" stroke="#2a3f66" strokeWidth={1.2} />
            {/* fill level */}
            <rect x={x + 1.5} y={stickH - fillH} width={stickW - 3} height={fillH} rx={2} fill={color} opacity={0.85}>
              <animate attributeName="opacity" values="0.65;0.9;0.65" dur={`${2 + i * 0.3}s`} repeatCount="indefinite" />
            </rect>
            {/* chip notches */}
            {[0.18, 0.4, 0.62].map((cy, k) => (
              <rect key={k} x={x + 3} y={stickH * cy} width={stickW - 6} height={6} rx={1} fill="rgba(255,255,255,0.06)" />
            ))}
            {/* gold connector pins at bottom */}
            <rect x={x + 2} y={stickH + 1} width={stickW - 4} height={4} rx={1} fill="#caa24a" opacity={0.7} />
          </g>
        );
      })}
    </svg>
  );
}

/** Disk — a 3D cylinder (database/drum) with a liquid fill level. */
function DiskCylinder({ percent, color }: { percent: number; color: string }) {
  const w = 90;
  const h = 110;
  const rx = w / 2;
  const ry = 14;
  const bodyTop = ry;
  const bodyBottom = h - ry;
  const bodyH = bodyBottom - bodyTop;
  const p = Math.max(0, Math.min(100, percent)) / 100;
  const fillTopY = bodyBottom - bodyH * p;

  const cylPath = `M0,${bodyTop} A${rx},${ry} 0 0 1 ${w},${bodyTop} L${w},${bodyBottom} A${rx},${ry} 0 0 1 0,${bodyBottom} Z`;

  return (
    <svg viewBox={`-4 -4 ${w + 8} ${h + 8}`} className="h-full">
      <defs>
        <clipPath id="disk-clip">
          <path d={cylPath} />
        </clipPath>
      </defs>
      {/* shell */}
      <path d={cylPath} fill="#0e1626" stroke="#2a3f66" strokeWidth={1.5} />
      {/* liquid fill */}
      <g clipPath="url(#disk-clip)">
        <rect x={0} y={fillTopY} width={w} height={bodyBottom - fillTopY + ry} fill={color} opacity={0.75} />
        <ellipse cx={rx} cy={fillTopY} rx={rx} ry={ry} fill={color} opacity={0.95}>
          <animate attributeName="ry" values={`${ry};${ry - 3};${ry}`} dur="3s" repeatCount="indefinite" />
        </ellipse>
      </g>
      {/* top rim */}
      <ellipse cx={rx} cy={bodyTop} rx={rx} ry={ry} fill="#0e1626" stroke="#2a3f66" strokeWidth={1.5} />
      {/* platter rings on the rim */}
      <ellipse cx={rx} cy={bodyTop} rx={rx * 0.6} ry={ry * 0.55} fill="none" stroke="rgba(255,255,255,0.08)" strokeWidth={1} />
      <ellipse cx={rx} cy={bodyTop} rx={rx * 0.28} ry={ry * 0.28} fill="none" stroke="rgba(255,255,255,0.1)" strokeWidth={1} />
    </svg>
  );
}

export function ResourceMeters() {
  const { data } = useQuery({
    queryKey: ["cluster-resource-pressure"],
    queryFn: () => getClusterTroubleshootingSummary(),
    refetchInterval: 15_000,
  });

  const rp: ResourcePressureSummary | undefined = data?.resource_pressure;

  const cpuPercent = rp?.cpu_usage_percent ?? 0;
  const memPercent = rp?.memory_usage_percent ?? 0;
  const storagePercent = rp?.storage_usage_percent ?? 0;

  const cpuPrimary = formatCPU(rp?.cpu_usage_milli);
  const cpuSecondary = `of ${formatCPU(rp?.cpu_capacity_milli)}`;
  const memPrimary = formatBytes(rp?.memory_usage_bytes);
  const memSecondary = `of ${formatBytes(rp?.memory_capacity_bytes)}`;

  const storageSource = rp?.storage_source ?? "";
  const hasStorageUsage = storageSource === "longhorn" && (rp?.storage_capacity_bytes ?? 0) > 0;
  const storagePrimary = hasStorageUsage
    ? formatBytes(rp?.storage_used_bytes)
    : formatBytes(rp?.storage_capacity_bytes);
  const storageSecondary = hasStorageUsage ? `of ${formatBytes(rp?.storage_capacity_bytes)}` : "capacity";
  const storageLabel = storageSource === "longhorn" ? "Storage (Longhorn)" : "Storage";

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 sm:gap-4">
        <MeterShell
          label="CPU Cores"
          icon={<Cpu className="w-5 h-5" />}
          color={colorFor(cpuPercent)}
          percent={cpuPercent}
          primary={cpuPrimary}
          secondary={cpuSecondary}
        >
          <CpuDie percent={cpuPercent} color={colorFor(cpuPercent)} />
        </MeterShell>

        <MeterShell
          label="Memory (RAM)"
          icon={<MemoryStick className="w-5 h-5" />}
          color={colorFor(memPercent)}
          percent={memPercent}
          primary={memPrimary}
          secondary={memSecondary}
        >
          <RamSticks percent={memPercent} color={colorFor(memPercent)} />
        </MeterShell>

        <MeterShell
          label={storageLabel}
          icon={<HardDrive className="w-5 h-5" />}
          color={colorFor(storagePercent)}
          percent={storagePercent}
          primary={storagePrimary}
          secondary={storageSecondary}
        >
          <DiskCylinder percent={storagePercent} color={colorFor(storagePercent)} />
        </MeterShell>
      </div>

      {rp?.storage_classes && rp.storage_classes.length > 0 && (
        <StorageClassBreakdown classes={rp.storage_classes} />
      )}
    </div>
  );
}

function StorageClassBreakdown({ classes }: { classes: StorageClassSummary[] }) {
  return (
    <div className="bg-pilot-surface border border-pilot-border rounded-xl p-4">
      <div className="flex items-center gap-2 mb-3">
        <Database className="w-4 h-4 text-pilot-accent" />
        <span className="text-xs uppercase tracking-wider text-pilot-muted font-medium">
          Provisioned by StorageClass
        </span>
      </div>
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
        {classes.map((sc) => (
          <div key={sc.name} className="bg-pilot-bg border border-pilot-border rounded-lg p-3">
            <div className="flex items-center justify-between mb-1">
              <span className="text-sm font-semibold text-white truncate" title={sc.name}>
                {sc.name}
              </span>
              <span className="text-2xs text-pilot-muted font-mono">
                {sc.pv_bound_count}/{sc.pv_count} PV
              </span>
            </div>
            <div className="text-lg font-bold text-pilot-accent">{formatBytes(sc.bound_bytes)}</div>
            <div className="text-2xs text-pilot-muted font-mono mt-0.5 truncate" title={sc.provisioner}>
              {sc.provisioner || "—"}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
