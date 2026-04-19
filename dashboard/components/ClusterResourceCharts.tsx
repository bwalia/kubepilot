/**
 * ClusterResourceCharts — three radial gauge charts showing cluster-wide
 * CPU, memory, and storage usage. Data comes from /api/v1/troubleshooting/summary.
 */
import { useQuery } from "@tanstack/react-query";
import { RadialBarChart, RadialBar, PolarAngleAxis, ResponsiveContainer } from "recharts";
import { Cpu, MemoryStick, HardDrive } from "lucide-react";
import { getClusterTroubleshootingSummary, type ResourcePressureSummary } from "@/lib/api";

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

interface GaugeProps {
  label: string;
  icon: React.ReactNode;
  percent: number;
  primary: string;
  secondary: string;
  color: string;
}

function Gauge({ label, icon, percent, primary, secondary, color }: GaugeProps) {
  const data = [{ name: label, value: percent, fill: color }];
  const clamped = Math.max(0, Math.min(100, percent));
  return (
    <div className="bg-pilot-surface border border-pilot-border rounded-xl p-4 shadow-card hover:shadow-card-hover transition-all">
      <div className="flex items-center gap-2 mb-2">
        <span style={{ color }}>{icon}</span>
        <span className="text-xs uppercase tracking-wider text-pilot-muted font-medium">{label}</span>
      </div>
      <div className="relative h-32 sm:h-36">
        <ResponsiveContainer width="100%" height="100%">
          <RadialBarChart
            cx="50%"
            cy="50%"
            innerRadius="70%"
            outerRadius="95%"
            barSize={12}
            data={data}
            startAngle={90}
            endAngle={-270}
          >
            <PolarAngleAxis type="number" domain={[0, 100]} tick={false} />
            <RadialBar background={{ fill: "rgba(255,255,255,0.05)" }} dataKey="value" cornerRadius={6} />
          </RadialBarChart>
        </ResponsiveContainer>
        <div className="absolute inset-0 flex flex-col items-center justify-center pointer-events-none">
          <div className="text-3xl font-bold tracking-tight" style={{ color }}>
            {clamped}%
          </div>
          <div className="text-2xs text-pilot-muted uppercase tracking-wider mt-0.5">used</div>
        </div>
      </div>
      <div className="flex items-center justify-between mt-2 text-xs">
        <span className="text-pilot-text-secondary font-mono">{primary}</span>
        <span className="text-pilot-muted font-mono">{secondary}</span>
      </div>
    </div>
  );
}

export function ClusterResourceCharts() {
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
  const storagePrimary = formatBytes(rp?.storage_bound_bytes);
  const storageSecondary = `of ${formatBytes(rp?.storage_capacity_bytes)}`;

  // Color based on utilization: green < 60%, amber < 85%, red >=85%
  const colorFor = (p: number) =>
    p >= 85 ? "#ef4444" : p >= 60 ? "#f59e0b" : "#10b981";

  return (
    <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 sm:gap-4">
      <Gauge
        label="Cluster CPU"
        icon={<Cpu className="w-5 h-5" />}
        percent={cpuPercent}
        primary={cpuPrimary}
        secondary={cpuSecondary}
        color={colorFor(cpuPercent)}
      />
      <Gauge
        label="Cluster Memory"
        icon={<MemoryStick className="w-5 h-5" />}
        percent={memPercent}
        primary={memPrimary}
        secondary={memSecondary}
        color={colorFor(memPercent)}
      />
      <Gauge
        label="Cluster Storage"
        icon={<HardDrive className="w-5 h-5" />}
        percent={storagePercent}
        primary={storagePrimary}
        secondary={storageSecondary}
        color={colorFor(storagePercent)}
      />
    </div>
  );
}
