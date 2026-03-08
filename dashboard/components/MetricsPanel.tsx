import type { DeploymentSummary } from "@/lib/api";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  Cell,
} from "recharts";

interface Props {
  deployments: DeploymentSummary[];
}

export function MetricsPanel({ deployments }: Props) {
  // Chart data: show ready vs desired replicas per deployment (top 10 by replica count).
  const chartData = [...deployments]
    .sort((a, b) => b.Replicas - a.Replicas)
    .slice(0, 10)
    .map((d) => ({
      name: d.Name.length > 16 ? d.Name.slice(0, 16) + "…" : d.Name,
      desired: d.Replicas,
      ready: d.ReadyReplicas,
      degraded: d.Replicas - d.ReadyReplicas,
    }));

  // KPI totals
  const totalDesired = deployments.reduce((acc, d) => acc + d.Replicas, 0);
  const totalReady = deployments.reduce((acc, d) => acc + d.ReadyReplicas, 0);
  const healthPct = totalDesired > 0 ? Math.round((totalReady / totalDesired) * 100) : 100;

  return (
    <section>
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-sm font-bold text-white">Deployment Health</h2>
        <span
          className={`text-xs font-bold px-2 py-0.5 rounded ${
            healthPct === 100
              ? "bg-pilot-success text-black"
              : healthPct >= 80
              ? "bg-pilot-warning text-black"
              : "bg-pilot-danger text-white"
          }`}
        >
          {healthPct}% healthy
        </span>
      </div>

      {chartData.length > 0 ? (
        <div className="bg-pilot-surface border border-pilot-border rounded-lg p-4">
          <ResponsiveContainer width="100%" height={220}>
            <BarChart data={chartData} margin={{ top: 4, right: 8, left: -20, bottom: 0 }}>
              <XAxis
                dataKey="name"
                tick={{ fontSize: 10, fill: "#6b7280" }}
                axisLine={false}
                tickLine={false}
              />
              <YAxis
                tick={{ fontSize: 10, fill: "#6b7280" }}
                axisLine={false}
                tickLine={false}
              />
              <Tooltip
                contentStyle={{
                  background: "#111827",
                  border: "1px solid #1f2937",
                  borderRadius: 6,
                  fontSize: 12,
                }}
                labelStyle={{ color: "#fff" }}
              />
              <Bar dataKey="ready" name="Ready" stackId="a" radius={[0, 0, 4, 4]}>
                {chartData.map((entry, i) => (
                  <Cell
                    key={i}
                    fill={entry.degraded > 0 ? "#f59e0b" : "#10b981"}
                  />
                ))}
              </Bar>
              <Bar dataKey="degraded" name="Degraded" stackId="a" fill="#ef4444" radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
          <p className="text-xs text-pilot-muted mt-2 text-center">
            Replicas: {totalReady} ready / {totalDesired} desired across {deployments.length} deployments
          </p>
        </div>
      ) : (
        <div className="text-center text-pilot-muted text-xs py-8">No deployments found.</div>
      )}
    </section>
  );
}
