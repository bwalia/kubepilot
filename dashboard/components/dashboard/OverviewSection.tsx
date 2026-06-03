/**
 * OverviewSection — the at-a-glance charts dashboard for the Kubernetes browser.
 * Aggregates existing list endpoints (no new backend) into:
 *   - object-inventory KPI cards (icon per kind)
 *   - pod-health donut (Running/Ready vs CrashLoop vs Pending vs Failed vs Succeeded)
 *   - workload-inventory bar chart
 *   - PVC-status donut
 *   - pods-per-namespace bar chart (cluster-wide only)
 * plus the physically-themed CPU/RAM/Disk meters.
 */
import { useQuery } from "@tanstack/react-query";
import {
  PieChart,
  Pie,
  Cell,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from "recharts";
import {
  Boxes,
  Layers,
  Server,
  Cog,
  Briefcase,
  CalendarClock,
  Network,
  Globe,
  HardDrive,
  FileText,
  KeyRound,
  Cpu,
  AlertTriangle,
  CheckCircle2,
} from "lucide-react";
import {
  listPods,
  listDeployments,
  listStatefulSets,
  listDaemonSets,
  listK8sJobs,
  listCronJobs,
  listServiceEndpoints,
  listIngresses,
  listPVCs,
  listConfigMaps,
  listSecrets,
  listNodes,
  type PodSummary,
} from "@/lib/api";
import { ResourceMeters } from "./ResourceMeters";

const COLORS = {
  success: "#10b981",
  warning: "#f59e0b",
  danger: "#ef4444",
  accent: "#3b82f6",
  accentLight: "#60a5fa",
  muted: "#7c8ba3",
  purple: "#a78bfa",
};

// ── Pod classification ────────────────────────────────────────────────────────
type PodBucket = "Running" | "Pending" | "CrashLoop" | "Failed" | "Succeeded";

function classifyPod(p: PodSummary): PodBucket {
  const phase = (p.Phase || "").toLowerCase();
  const reason = (p.Reason || "").toLowerCase();
  if (reason.includes("crashloop") || reason.includes("error") || reason.includes("backoff")) return "CrashLoop";
  if (phase === "failed") return "Failed";
  if (phase === "succeeded") return "Succeeded";
  if (phase === "pending") return "Pending";
  if (phase === "running" && !p.Ready) return "CrashLoop";
  return "Running";
}

const POD_BUCKET_COLOR: Record<PodBucket, string> = {
  Running: COLORS.success,
  Pending: COLORS.warning,
  CrashLoop: COLORS.danger,
  Failed: COLORS.danger,
  Succeeded: COLORS.accentLight,
};

export function OverviewSection({ namespace }: { namespace: string }) {
  const refetchInterval = 15_000;
  const pods = useQuery({ queryKey: ["ov-pods", namespace], queryFn: () => listPods(namespace), refetchInterval });
  const deployments = useQuery({ queryKey: ["ov-deploy", namespace], queryFn: () => listDeployments(namespace), refetchInterval });
  const statefulsets = useQuery({ queryKey: ["ov-sts", namespace], queryFn: () => listStatefulSets(namespace), refetchInterval });
  const daemonsets = useQuery({ queryKey: ["ov-ds", namespace], queryFn: () => listDaemonSets(namespace), refetchInterval });
  const jobs = useQuery({ queryKey: ["ov-jobs", namespace], queryFn: () => listK8sJobs(namespace), refetchInterval });
  const cronjobs = useQuery({ queryKey: ["ov-cron", namespace], queryFn: () => listCronJobs(namespace), refetchInterval });
  const services = useQuery({ queryKey: ["ov-svc", namespace], queryFn: () => listServiceEndpoints(namespace), refetchInterval });
  const ingresses = useQuery({ queryKey: ["ov-ing", namespace], queryFn: () => listIngresses(namespace), refetchInterval });
  const pvcs = useQuery({ queryKey: ["ov-pvc", namespace], queryFn: () => listPVCs(namespace), refetchInterval });
  const configmaps = useQuery({ queryKey: ["ov-cm", namespace], queryFn: () => listConfigMaps(namespace), refetchInterval });
  const secrets = useQuery({ queryKey: ["ov-secrets", namespace], queryFn: () => listSecrets(namespace), refetchInterval });
  const nodes = useQuery({ queryKey: ["ov-nodes"], queryFn: listNodes, refetchInterval: 30_000 });

  const podList = pods.data ?? [];

  // Pod health buckets
  const podBuckets: Record<PodBucket, number> = {
    Running: 0,
    Pending: 0,
    CrashLoop: 0,
    Failed: 0,
    Succeeded: 0,
  };
  podList.forEach((p) => {
    podBuckets[classifyPod(p)] += 1;
  });
  const podHealthData = (Object.keys(podBuckets) as PodBucket[])
    .map((k) => ({ name: k, value: podBuckets[k], color: POD_BUCKET_COLOR[k] }))
    .filter((d) => d.value > 0);
  const healthyPods = podBuckets.Running + podBuckets.Succeeded;
  const unhealthyPods = podBuckets.CrashLoop + podBuckets.Failed;

  // Workload inventory bar
  const workloadData = [
    { name: "Deployments", count: deployments.data?.length ?? 0 },
    { name: "StatefulSets", count: statefulsets.data?.length ?? 0 },
    { name: "DaemonSets", count: daemonsets.data?.length ?? 0 },
    { name: "Jobs", count: jobs.data?.length ?? 0 },
    { name: "CronJobs", count: cronjobs.data?.length ?? 0 },
  ];

  // PVC status donut
  const pvcBuckets: Record<string, number> = {};
  (pvcs.data ?? []).forEach((p) => {
    const s = p.Status || "Unknown";
    pvcBuckets[s] = (pvcBuckets[s] || 0) + 1;
  });
  const pvcColor = (s: string) =>
    s === "Bound" ? COLORS.success : s === "Pending" ? COLORS.warning : s === "Lost" ? COLORS.danger : COLORS.muted;
  const pvcData = Object.entries(pvcBuckets).map(([name, value]) => ({ name, value, color: pvcColor(name) }));

  // Pods per namespace (only meaningful cluster-wide)
  const perNs: Record<string, number> = {};
  if (namespace === "") {
    podList.forEach((p) => {
      perNs[p.Namespace] = (perNs[p.Namespace] || 0) + 1;
    });
  }
  const perNsData = Object.entries(perNs)
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => b.count - a.count)
    .slice(0, 10);

  const inventory = [
    { label: "Pods", value: podList.length, icon: Boxes, color: COLORS.accent },
    { label: "Deployments", value: deployments.data?.length ?? 0, icon: Layers, color: COLORS.accentLight },
    { label: "StatefulSets", value: statefulsets.data?.length ?? 0, icon: Server, color: COLORS.purple },
    { label: "DaemonSets", value: daemonsets.data?.length ?? 0, icon: Cog, color: COLORS.purple },
    { label: "Jobs", value: jobs.data?.length ?? 0, icon: Briefcase, color: COLORS.muted },
    { label: "CronJobs", value: cronjobs.data?.length ?? 0, icon: CalendarClock, color: COLORS.muted },
    { label: "Services", value: services.data?.length ?? 0, icon: Network, color: COLORS.accent },
    { label: "Ingresses", value: ingresses.data?.length ?? 0, icon: Globe, color: COLORS.accentLight },
    { label: "PVCs", value: pvcs.data?.length ?? 0, icon: HardDrive, color: COLORS.warning },
    { label: "ConfigMaps", value: configmaps.data?.length ?? 0, icon: FileText, color: COLORS.muted },
    { label: "Secrets", value: secrets.data?.length ?? 0, icon: KeyRound, color: COLORS.warning },
    ...(namespace === "" ? [{ label: "Nodes", value: nodes.data?.length ?? 0, icon: Cpu, color: COLORS.success }] : []),
  ];

  return (
    <div className="space-y-6">
      {/* Resource meters */}
      <ResourceMeters />

      {/* Healthy vs unhealthy headline */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <HeadlineStat
          label="Healthy Pods"
          value={healthyPods}
          total={podList.length}
          icon={<CheckCircle2 className="w-5 h-5" />}
          color={COLORS.success}
        />
        <HeadlineStat
          label="Unhealthy Pods"
          value={unhealthyPods}
          total={podList.length}
          icon={<AlertTriangle className="w-5 h-5" />}
          color={unhealthyPods > 0 ? COLORS.danger : COLORS.muted}
        />
        <HeadlineStat
          label="Pending Pods"
          value={podBuckets.Pending}
          total={podList.length}
          icon={<Boxes className="w-5 h-5" />}
          color={podBuckets.Pending > 0 ? COLORS.warning : COLORS.muted}
        />
      </div>

      {/* Object inventory cards */}
      <section>
        <SectionTitle icon={<Boxes className="w-4 h-4 text-pilot-accent" />} title="Object Inventory" />
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-3">
          {inventory.map((item) => {
            const Icon = item.icon;
            return (
              <div
                key={item.label}
                className="bg-pilot-surface border border-pilot-border rounded-xl p-4 shadow-card hover:shadow-card-hover hover:border-pilot-border-hover transition-all"
              >
                <div className="flex items-center justify-between mb-2">
                  <Icon className="w-5 h-5" style={{ color: item.color }} />
                  <span className="text-2xl font-bold text-white tabular-nums">{item.value}</span>
                </div>
                <span className="text-xs uppercase tracking-wider text-pilot-muted">{item.label}</span>
              </div>
            );
          })}
        </div>
      </section>

      {/* Charts grid */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <ChartCard title="Pod Health" subtitle={`${podList.length} pods`}>
          {podHealthData.length === 0 ? (
            <EmptyChart />
          ) : (
            <ResponsiveContainer width="100%" height={260}>
              <PieChart>
                <Pie
                  data={podHealthData}
                  dataKey="value"
                  nameKey="name"
                  cx="50%"
                  cy="50%"
                  innerRadius={60}
                  outerRadius={95}
                  paddingAngle={2}
                  stroke="none"
                >
                  {podHealthData.map((d) => (
                    <Cell key={d.name} fill={d.color} />
                  ))}
                </Pie>
                <Tooltip {...tooltipStyle} />
                <Legend {...legendStyle} />
              </PieChart>
            </ResponsiveContainer>
          )}
        </ChartCard>

        <ChartCard title="Workload Inventory" subtitle="controllers by kind">
          <ResponsiveContainer width="100%" height={260}>
            <BarChart data={workloadData} margin={{ top: 8, right: 12, left: -16, bottom: 0 }}>
              <XAxis dataKey="name" tick={{ fill: COLORS.muted, fontSize: 11 }} axisLine={{ stroke: "#1e2d4a" }} tickLine={false} />
              <YAxis allowDecimals={false} tick={{ fill: COLORS.muted, fontSize: 11 }} axisLine={false} tickLine={false} />
              <Tooltip {...tooltipStyle} cursor={{ fill: "rgba(255,255,255,0.04)" }} />
              <Bar dataKey="count" fill={COLORS.accent} radius={[4, 4, 0, 0]} maxBarSize={48} />
            </BarChart>
          </ResponsiveContainer>
        </ChartCard>

        <ChartCard title="Persistent Volume Claims" subtitle={`${pvcs.data?.length ?? 0} PVCs`}>
          {pvcData.length === 0 ? (
            <EmptyChart message="No PVCs in scope." />
          ) : (
            <ResponsiveContainer width="100%" height={260}>
              <PieChart>
                <Pie
                  data={pvcData}
                  dataKey="value"
                  nameKey="name"
                  cx="50%"
                  cy="50%"
                  innerRadius={60}
                  outerRadius={95}
                  paddingAngle={2}
                  stroke="none"
                >
                  {pvcData.map((d) => (
                    <Cell key={d.name} fill={d.color} />
                  ))}
                </Pie>
                <Tooltip {...tooltipStyle} />
                <Legend {...legendStyle} />
              </PieChart>
            </ResponsiveContainer>
          )}
        </ChartCard>

        <ChartCard
          title={namespace === "" ? "Pods per Namespace" : "Pods per Namespace"}
          subtitle={namespace === "" ? "top 10" : "select All Namespaces to view"}
        >
          {namespace !== "" ? (
            <EmptyChart message="Available when viewing All Namespaces." />
          ) : perNsData.length === 0 ? (
            <EmptyChart />
          ) : (
            <ResponsiveContainer width="100%" height={260}>
              <BarChart layout="vertical" data={perNsData} margin={{ top: 4, right: 16, left: 8, bottom: 0 }}>
                <XAxis type="number" allowDecimals={false} tick={{ fill: COLORS.muted, fontSize: 11 }} axisLine={false} tickLine={false} />
                <YAxis type="category" dataKey="name" width={110} tick={{ fill: COLORS.muted, fontSize: 11 }} axisLine={false} tickLine={false} />
                <Tooltip {...tooltipStyle} cursor={{ fill: "rgba(255,255,255,0.04)" }} />
                <Bar dataKey="count" fill={COLORS.accentLight} radius={[0, 4, 4, 0]} maxBarSize={20} />
              </BarChart>
            </ResponsiveContainer>
          )}
        </ChartCard>
      </div>
    </div>
  );
}

const tooltipStyle = {
  contentStyle: {
    background: "#131c2e",
    border: "1px solid #1e2d4a",
    borderRadius: 8,
    fontSize: 12,
    color: "#fff",
  },
  itemStyle: { color: "#b0bdd0" },
  labelStyle: { color: "#fff" },
} as const;

const legendStyle = {
  wrapperStyle: { fontSize: 12, color: "#b0bdd0" },
  iconType: "circle" as const,
};

function SectionTitle({ icon, title }: { icon: React.ReactNode; title: string }) {
  return (
    <h3 className="text-sm font-bold text-white mb-3 flex items-center gap-2">
      {icon} {title}
    </h3>
  );
}

function ChartCard({ title, subtitle, children }: { title: string; subtitle?: string; children: React.ReactNode }) {
  return (
    <div className="bg-pilot-surface border border-pilot-border rounded-xl p-4 shadow-card">
      <div className="flex items-baseline justify-between mb-2">
        <h4 className="text-sm font-bold text-white">{title}</h4>
        {subtitle && <span className="text-2xs text-pilot-muted uppercase tracking-wider">{subtitle}</span>}
      </div>
      {children}
    </div>
  );
}

function EmptyChart({ message = "No data to display." }: { message?: string }) {
  return <div className="h-[260px] flex items-center justify-center text-sm text-pilot-muted">{message}</div>;
}

function HeadlineStat({
  label,
  value,
  total,
  icon,
  color,
}: {
  label: string;
  value: number;
  total: number;
  icon: React.ReactNode;
  color: string;
}) {
  const pct = total > 0 ? Math.round((value / total) * 100) : 0;
  return (
    <div className="bg-pilot-surface border border-pilot-border rounded-xl p-4 shadow-card flex items-center gap-4">
      <span className="shrink-0" style={{ color }}>
        {icon}
      </span>
      <div className="min-w-0">
        <div className="flex items-baseline gap-2">
          <span className="text-2xl font-bold text-white tabular-nums">{value}</span>
          <span className="text-xs text-pilot-muted">/ {total} ({pct}%)</span>
        </div>
        <span className="text-xs uppercase tracking-wider text-pilot-muted">{label}</span>
      </div>
    </div>
  );
}
