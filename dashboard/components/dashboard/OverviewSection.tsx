/**
 * OverviewSection — the at-a-glance summary for the Kubernetes browser.
 * Organised into clearly-labelled groups (capacity → pod health → inventory →
 * breakdowns), each with a one-line description so it never reads as a wall of
 * widgets. All chart colours resolve from the active theme (Daylight/Night) so
 * the visualisations stay consistent with the rest of the site.
 */
import { useQuery } from "@tanstack/react-query";
import { qk } from "@/lib/queryKeys";
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
  Gauge,
  HeartPulse,
  PieChart as PieIcon,
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
import { useThemeColors } from "@/lib/useThemeColors";
import { ResourceMeters } from "./ResourceMeters";
import { StatusPill } from "@/components/ui/StatusPill";
import { explainPod } from "@/lib/k8sExplain";
import { requestDashboardTarget } from "@/lib/dashboardNav";
import { ChevronRight, ShieldCheck } from "lucide-react";

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

export function OverviewSection({ namespace }: { namespace: string }) {
  const refetchInterval = 15_000;
  const pods = useQuery({ queryKey: qk.pods(namespace), queryFn: () => listPods(namespace), refetchInterval });
  const deployments = useQuery({ queryKey: qk.deployments(namespace), queryFn: () => listDeployments(namespace), refetchInterval });
  const statefulsets = useQuery({ queryKey: qk.statefulSets(namespace), queryFn: () => listStatefulSets(namespace), refetchInterval });
  const daemonsets = useQuery({ queryKey: qk.daemonSets(namespace), queryFn: () => listDaemonSets(namespace), refetchInterval });
  const jobs = useQuery({ queryKey: qk.jobs(namespace), queryFn: () => listK8sJobs(namespace), refetchInterval });
  const cronjobs = useQuery({ queryKey: qk.cronJobs(namespace), queryFn: () => listCronJobs(namespace), refetchInterval });
  const services = useQuery({ queryKey: qk.services(namespace), queryFn: () => listServiceEndpoints(namespace), refetchInterval });
  const ingresses = useQuery({ queryKey: qk.ingresses(namespace), queryFn: () => listIngresses(namespace), refetchInterval });
  const pvcs = useQuery({ queryKey: qk.pvcs(namespace), queryFn: () => listPVCs(namespace), refetchInterval });
  const configmaps = useQuery({ queryKey: qk.configMaps(namespace), queryFn: () => listConfigMaps(namespace), refetchInterval });
  const secrets = useQuery({ queryKey: qk.secrets(namespace), queryFn: () => listSecrets(namespace), refetchInterval });
  const nodes = useQuery({ queryKey: qk.nodes(), queryFn: listNodes, refetchInterval: 30_000 });

  // Theme-resolved chart palette (recolours instantly on Daylight/Night toggle).
  const tc = useThemeColors(["success", "warning", "danger", "accent", "accent-light", "info", "muted", "surface", "border", "text-primary", "text-secondary"]);
  const C = {
    success: tc.success || "#047857",
    warning: tc.warning || "#B45309",
    danger: tc.danger || "#C82028",
    accent: tc.accent || "#0D6F8A",
    accentLight: tc["accent-light"] || "#0B627A",
    muted: tc.muted || "#5A667A",
    purple: tc.info || "#6B28D6",
  };
  const podBucketColor: Record<PodBucket, string> = {
    Running: C.success,
    Pending: C.warning,
    CrashLoop: C.danger,
    Failed: C.danger,
    Succeeded: C.accentLight,
  };

  const tooltipStyle = {
    contentStyle: {
      background: tc.surface || "#fff",
      border: `1px solid ${tc.border || "#D7DDE7"}`,
      borderRadius: 10,
      fontSize: 12,
      color: tc["text-primary"] || "#0F172A",
      boxShadow: "0 8px 24px -8px rgba(15,23,42,.22)",
    },
    itemStyle: { color: tc["text-secondary"] || "#334155" },
    labelStyle: { color: tc["text-primary"] || "#0F172A", fontWeight: 600 },
  } as const;
  const legendStyle = {
    wrapperStyle: { fontSize: 12, color: tc["text-secondary"] || "#334155" },
    iconType: "circle" as const,
  };
  const axisTick = { fill: C.muted, fontSize: 11 };
  const cursorFill = tc.border ? `${tc.border}66` : "rgba(100,116,139,0.15)";

  const podList = pods.data ?? [];

  const podBuckets: Record<PodBucket, number> = { Running: 0, Pending: 0, CrashLoop: 0, Failed: 0, Succeeded: 0 };
  podList.forEach((p) => { podBuckets[classifyPod(p)] += 1; });
  const podHealthData = (Object.keys(podBuckets) as PodBucket[])
    .map((k) => ({ name: k, value: podBuckets[k], color: podBucketColor[k] }))
    .filter((d) => d.value > 0);
  const healthyPods = podBuckets.Running + podBuckets.Succeeded;
  const unhealthyPods = podBuckets.CrashLoop + podBuckets.Failed;

  const workloadData = [
    { name: "Deployments", count: deployments.data?.length ?? 0 },
    { name: "StatefulSets", count: statefulsets.data?.length ?? 0 },
    { name: "DaemonSets", count: daemonsets.data?.length ?? 0 },
    { name: "Jobs", count: jobs.data?.length ?? 0 },
    { name: "CronJobs", count: cronjobs.data?.length ?? 0 },
  ];

  const pvcBuckets: Record<string, number> = {};
  (pvcs.data ?? []).forEach((p) => {
    const s = p.Status || "Unknown";
    pvcBuckets[s] = (pvcBuckets[s] || 0) + 1;
  });
  const pvcColor = (s: string) =>
    s === "Bound" ? C.success : s === "Pending" ? C.warning : s === "Lost" ? C.danger : C.muted;
  const pvcData = Object.entries(pvcBuckets).map(([name, value]) => ({ name, value, color: pvcColor(name) }));

  const perNs: Record<string, number> = {};
  if (namespace === "") {
    podList.forEach((p) => { perNs[p.Namespace] = (perNs[p.Namespace] || 0) + 1; });
  }
  const perNsData = Object.entries(perNs)
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => b.count - a.count)
    .slice(0, 10);

  const inventory = [
    { label: "Pods", value: podList.length, icon: Boxes, color: C.accent },
    { label: "Deployments", value: deployments.data?.length ?? 0, icon: Layers, color: C.accentLight },
    { label: "StatefulSets", value: statefulsets.data?.length ?? 0, icon: Server, color: C.purple },
    { label: "DaemonSets", value: daemonsets.data?.length ?? 0, icon: Cog, color: C.purple },
    { label: "Jobs", value: jobs.data?.length ?? 0, icon: Briefcase, color: C.muted },
    { label: "CronJobs", value: cronjobs.data?.length ?? 0, icon: CalendarClock, color: C.muted },
    { label: "Services", value: services.data?.length ?? 0, icon: Network, color: C.accent },
    { label: "Ingresses", value: ingresses.data?.length ?? 0, icon: Globe, color: C.accentLight },
    { label: "PVCs", value: pvcs.data?.length ?? 0, icon: HardDrive, color: C.warning },
    { label: "ConfigMaps", value: configmaps.data?.length ?? 0, icon: FileText, color: C.muted },
    { label: "Secrets", value: secrets.data?.length ?? 0, icon: KeyRound, color: C.warning },
    ...(namespace === "" ? [{ label: "Nodes", value: nodes.data?.length ?? 0, icon: Cpu, color: C.success }] : []),
  ];

  const scopeLabel = namespace === "" ? "all namespaces" : `namespace “${namespace}”`;

  // Anything a person would call "broken", worst first. This is the first
  // thing on the page on purpose: the question someone opens a dashboard to
  // answer is "is anything wrong?", and a wall of charts does not answer it.
  //
  // Grouped by owner, not listed per pod. A CronJob that has failed twelve
  // times produces twelve pods; twelve near-identical rows bury every other
  // problem on the cluster while describing a single fault.
  const needsAttention = groupFailures(podList);

  return (
    <div className="space-y-8">
      {/* ── Needs attention ── */}
      <AttentionPanel pods={needsAttention} loading={pods.isLoading} scopeLabel={scopeLabel} />

      {/* ── Cluster capacity ── */}
      <section className="space-y-4">
        <SectionHeader
          icon={<Gauge className="w-4 h-4" />}
          title="Cluster capacity"
          desc="Live CPU, memory and storage pressure across all nodes."
        />
        <ResourceMeters />
      </section>

      {/* ── Pod health ── */}
      <section className="space-y-4">
        <SectionHeader
          icon={<HeartPulse className="w-4 h-4" />}
          title="Pod health"
          desc={`How the ${podList.length} pods in ${scopeLabel} are doing right now.`}
        />
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <HeadlineStat label="Healthy Pods" value={healthyPods} total={podList.length} icon={<CheckCircle2 className="w-5 h-5" />} color={C.success} />
          <HeadlineStat label="Unhealthy Pods" value={unhealthyPods} total={podList.length} icon={<AlertTriangle className="w-5 h-5" />} color={unhealthyPods > 0 ? C.danger : C.muted} />
          <HeadlineStat label="Pending Pods" value={podBuckets.Pending} total={podList.length} icon={<Boxes className="w-5 h-5" />} color={podBuckets.Pending > 0 ? C.warning : C.muted} />
        </div>
      </section>

      {/* ── Object inventory ── */}
      <section className="space-y-4">
        <SectionHeader
          icon={<Boxes className="w-4 h-4" />}
          title="Object inventory"
          desc={`Every Kubernetes resource kind in ${scopeLabel}, counted at a glance.`}
        />
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4">
          {inventory.map((item) => {
            const Icon = item.icon;
            return (
              <div
                key={item.label}
                className="bg-pilot-surface border border-pilot-border rounded-xl p-4 shadow-card hover:shadow-card-hover hover:border-pilot-border-hover transition-all"
              >
                <div className="flex items-center justify-between mb-2">
                  <span className="inline-flex items-center justify-center w-8 h-8 rounded-lg bg-pilot-surface-2" style={{ color: item.color }}>
                    <Icon className="w-4.5 h-4.5" style={{ width: 18, height: 18 }} />
                  </span>
                  <span className="font-display text-2xl font-bold text-pilot-text-primary tabular-nums">{item.value}</span>
                </div>
                <span className="eyebrow">{item.label}</span>
              </div>
            );
          })}
        </div>
      </section>

      {/* ── Breakdowns ── */}
      <section className="space-y-4">
        <SectionHeader
          icon={<PieIcon className="w-4 h-4" />}
          title="Breakdowns"
          desc="Distribution of pods, workloads and storage — hover any slice for details."
        />
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <ChartCard title="Pod Health" subtitle={`${podList.length} pods`}>
            {podHealthData.length === 0 ? (
              <EmptyChart />
            ) : (
              <ResponsiveContainer width="100%" height={260}>
                <PieChart>
                  <Pie data={podHealthData} dataKey="value" nameKey="name" cx="50%" cy="50%" innerRadius={60} outerRadius={95} paddingAngle={2} stroke="none">
                    {podHealthData.map((d) => (<Cell key={d.name} fill={d.color} />))}
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
                <XAxis dataKey="name" tick={axisTick} axisLine={{ stroke: C.muted, strokeOpacity: 0.3 }} tickLine={false} />
                <YAxis allowDecimals={false} tick={axisTick} axisLine={false} tickLine={false} />
                <Tooltip {...tooltipStyle} cursor={{ fill: cursorFill }} />
                <Bar dataKey="count" fill={C.accent} radius={[4, 4, 0, 0]} maxBarSize={48} />
              </BarChart>
            </ResponsiveContainer>
          </ChartCard>

          <ChartCard title="Persistent Volume Claims" subtitle={`${pvcs.data?.length ?? 0} PVCs`}>
            {pvcData.length === 0 ? (
              <EmptyChart message="No PVCs in scope." />
            ) : (
              <ResponsiveContainer width="100%" height={260}>
                <PieChart>
                  <Pie data={pvcData} dataKey="value" nameKey="name" cx="50%" cy="50%" innerRadius={60} outerRadius={95} paddingAngle={2} stroke="none">
                    {pvcData.map((d) => (<Cell key={d.name} fill={d.color} />))}
                  </Pie>
                  <Tooltip {...tooltipStyle} />
                  <Legend {...legendStyle} />
                </PieChart>
              </ResponsiveContainer>
            )}
          </ChartCard>

          <ChartCard title="Pods per Namespace" subtitle={namespace === "" ? "top 10" : "select All Namespaces to view"}>
            {namespace !== "" ? (
              <EmptyChart message="Available when viewing All Namespaces." />
            ) : perNsData.length === 0 ? (
              <EmptyChart />
            ) : (
              <ResponsiveContainer width="100%" height={260}>
                <BarChart layout="vertical" data={perNsData} margin={{ top: 4, right: 16, left: 8, bottom: 0 }}>
                  <XAxis type="number" allowDecimals={false} tick={axisTick} axisLine={false} tickLine={false} />
                  <YAxis type="category" dataKey="name" width={110} tick={axisTick} axisLine={false} tickLine={false} />
                  <Tooltip {...tooltipStyle} cursor={{ fill: cursorFill }} />
                  <Bar dataKey="count" fill={C.accentLight} radius={[0, 4, 4, 0]} maxBarSize={20} />
                </BarChart>
              </ResponsiveContainer>
            )}
          </ChartCard>
        </div>
      </section>
    </div>
  );
}

function SectionHeader({ icon, title, desc }: { icon: React.ReactNode; title: string; desc: string }) {
  return (
    <div>
      <h3 className="font-display text-lg font-bold text-pilot-text-primary flex items-center gap-2">
        <span className="text-pilot-accent">{icon}</span>
        {title}
      </h3>
      <p className="text-sm text-pilot-muted mt-0.5">{desc}</p>
    </div>
  );
}

function ChartCard({ title, subtitle, children }: { title: string; subtitle?: string; children: React.ReactNode }) {
  return (
    <div className="bg-pilot-surface border border-pilot-border rounded-xl p-4 shadow-card">
      <div className="flex items-baseline justify-between mb-3">
        <h4 className="font-display text-[0.95rem] font-bold text-pilot-text-primary">{title}</h4>
        {subtitle && <span className="eyebrow">{subtitle}</span>}
      </div>
      {children}
    </div>
  );
}

function EmptyChart({ message = "No data to display." }: { message?: string }) {
  return <div className="h-[260px] flex items-center justify-center text-sm text-pilot-muted">{message}</div>;
}

function HeadlineStat({ label, value, total, icon, color }: { label: string; value: number; total: number; icon: React.ReactNode; color: string }) {
  const pct = total > 0 ? Math.round((value / total) * 100) : 0;
  return (
    <div className="bg-pilot-surface border border-pilot-border rounded-xl p-4 shadow-card flex items-center gap-4">
      <span className="shrink-0 inline-flex items-center justify-center w-11 h-11 rounded-xl bg-pilot-surface-2" style={{ color }}>
        {icon}
      </span>
      <div className="min-w-0">
        <div className="flex items-baseline gap-2">
          <span className="font-display text-2xl font-bold text-pilot-text-primary tabular-nums">{value}</span>
          <span className="text-sm text-pilot-muted">/ {total} ({pct}%)</span>
        </div>
        <span className="eyebrow">{label}</span>
      </div>
    </div>
  );
}


/**
 * The "anything broken?" panel. Green and quiet when all is well; a short,
 * clickable list when it is not. Capped at eight rows — past that the point is
 * made, and the full list is one click away in Workloads.
 */
function AttentionPanel({
  pods,
  loading,
  scopeLabel,
}: {
  pods: FailureGroup[];
  loading: boolean;
  scopeLabel: string;
}) {
  if (loading) {
    return <div className="h-24 animate-pulse rounded-xl bg-pilot-surface" />;
  }

  if (pods.length === 0) {
    return (
      <div className="flex items-center gap-3 rounded-xl border border-pilot-success/30 bg-pilot-success/10 px-4 py-3.5">
        <ShieldCheck className="h-5 w-5 shrink-0 text-pilot-success" aria-hidden="true" />
        <p className="text-sm text-pilot-text-secondary">
          <span className="font-semibold text-pilot-success">Everything is running.</span> No app in{" "}
          {scopeLabel} is failing or stuck.
        </p>
      </div>
    );
  }

  const shown = pods.slice(0, 8);
  const affected = pods.reduce((n, g) => n + g.count, 0);

  return (
    <section className="overflow-hidden rounded-xl border border-pilot-danger/30 bg-pilot-surface shadow-card">
      <div className="flex items-center gap-3 border-b border-pilot-border bg-pilot-danger/10 px-4 py-3">
        <AlertTriangle className="h-5 w-5 shrink-0 text-pilot-danger" aria-hidden="true" />
        <div>
          <h3 className="font-display text-sm font-bold text-pilot-text-primary">
            {pods.length} {pods.length === 1 ? "app needs" : "apps need"} attention
          </h3>
          <p className="text-xs text-pilot-muted">
            {affected > pods.length ? `${affected} pods affected. ` : ""}
            Select one to see its logs and what went wrong.
          </p>
        </div>
      </div>
      <ul className="divide-y divide-pilot-border">
        {shown.map((group) => (
          <li key={group.key}>
            <button
              type="button"
              onClick={() =>
                requestDashboardTarget({
                  type: "pod",
                  namespace: group.sample.Namespace,
                  name: group.sample.Name,
                })
              }
              className="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-pilot-surface-2"
            >
              <span className="min-w-0 flex-1">
                <span className="block truncate font-mono text-sm font-semibold text-pilot-text-primary">
                  {group.label}
                </span>
                <span className="block truncate text-xs text-pilot-muted">
                  {group.sample.Namespace}
                  {group.count > 1 && ` \u00b7 ${group.count} pods`}
                </span>
              </span>
              <StatusPill
                explanation={explainPod(group.sample)}
                raw={group.sample.Reason || group.sample.Phase}
                hideWhy
              />
              <ChevronRight className="h-4 w-4 shrink-0 text-pilot-muted" aria-hidden="true" />
            </button>
          </li>
        ))}
      </ul>
      {pods.length > shown.length && (
        <div className="border-t border-pilot-border px-4 py-2.5 text-xs text-pilot-muted">
          and {pods.length - shown.length} more &mdash; see the full list under Workloads.
        </div>
      )}
    </section>
  );
}


interface FailureGroup {
  key: string;
  /** The owner's name, with the generated pod suffixes stripped. */
  label: string;
  count: number;
  /** A representative pod — the one the row opens. */
  sample: PodSummary;
}

/**
 * Strip the suffixes Kubernetes generates, so every pod of one Deployment or
 * CronJob collapses to the name a person would use for it:
 *
 *   checkout-7d9f8b-x2k4l        -> checkout      (Deployment: hash + pod id)
 *   reconcile-29827706-4v92q     -> reconcile     (CronJob: unix minute + pod id)
 *   backup-x9k2p                 -> backup        (Job: pod id)
 *   coredns-node-6kknm           -> coredns-node  (DaemonSet: pod id)
 *
 * A heuristic, not a lookup of ownerReferences — the pod list endpoint does not
 * carry them, and a wrong grouping here costs a slightly odd label, not
 * correctness: the row still opens a real pod.
 */
function ownerName(podName: string): string {
  let name = podName;
  // Trailing pod id: five lowercase alphanumerics.
  name = name.replace(/-[a-z0-9]{5}$/, "");
  // Then a ReplicaSet hash or a CronJob schedule stamp, if one is left.
  name = name.replace(/-(?:[0-9]{8,11}|[a-f0-9]{6,10}|[a-z0-9]{9,10})$/, "");
  return name || podName;
}

/** Failing pods, collapsed per owner, worst first then most-affected first. */
function groupFailures(pods: PodSummary[]): FailureGroup[] {
  const groups = new Map<string, FailureGroup>();

  for (const pod of pods) {
    const ex = explainPod(pod);
    if (ex.tone !== "bad" && ex.tone !== "warn") continue;

    // Keyed by label too, so one owner failing two different ways stays two rows.
    const label = ownerName(pod.Name);
    const key = `${pod.Namespace}/${label}/${ex.label}`;
    const existing = groups.get(key);
    if (existing) existing.count += 1;
    else groups.set(key, { key, label, count: 1, sample: pod });
  }

  return [...groups.values()].sort((a, b) => {
    const rank = (g: FailureGroup) => (explainPod(g.sample).tone === "bad" ? 0 : 1);
    return rank(a) - rank(b) || b.count - a.count || a.label.localeCompare(b.label);
  });
}
