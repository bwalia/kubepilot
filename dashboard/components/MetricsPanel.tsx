import type { DeploymentSummary } from "@/lib/api";
import { BarChart3 } from "lucide-react";
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
  const chartData = [...deployments]
    .sort((a, b) => b.Replicas - a.Replicas)
    .slice(0, 10)
    .map((d) => ({
      name: (d.Name || "unknown").length > 16 ? (d.Name || "unknown").slice(0, 16) + "\u2026" : (d.Name || "unknown"),
      desired: d.Replicas,
      ready: d.ReadyReplicas,
      degraded: d.Replicas - d.ReadyReplicas,
    }));

  const totalDesired = deployments.reduce((acc, d) => acc + d.Replicas, 0);
  const totalReady = deployments.reduce((acc, d) => acc + d.ReadyReplicas, 0);
  const healthPct = totalDesired > 0 ? Math.round((totalReady / totalDesired) * 100) : 100;

  return (
    <section>
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-base font-bold text-white flex items-center gap-2">
          <BarChart3 className="w-5 h-5 text-pilot-accent" />
          Deployment Health
        </h2>
        <span
          className={`text-sm font-bold px-3 py-1 rounded-lg ${
            healthPct === 100
              ? "bg-pilot-success/15 text-pilot-success border border-pilot-success/30"
              : healthPct >= 80
              ? "bg-pilot-warning/15 text-pilot-warning border border-pilot-warning/30"
              : "bg-pilot-danger/15 text-pilot-danger border border-pilot-danger/30"
          }`}
        >
          {healthPct}% healthy
        </span>
      </div>

      {chartData.length > 0 ? (
        <div className="bg-pilot-surface border border-pilot-border rounded-xl p-5 shadow-card">
          <ResponsiveContainer width="100%" height={240}>
            <BarChart data={chartData} margin={{ top: 4, right: 8, left: -16, bottom: 0 }}>
              <XAxis
                dataKey="name"
                tick={{ fontSize: 11, fill: "#7c8ba3", fontFamily: "JetBrains Mono, monospace" }}
                axisLine={false}
                tickLine={false}
              />
              <YAxis
                tick={{ fontSize: 11, fill: "#7c8ba3" }}
                axisLine={false}
                tickLine={false}
              />
              <Tooltip
                contentStyle={{
                  background: "#1a2540",
                  border: "1px solid #1e2d4a",
                  borderRadius: 10,
                  fontSize: 13,
                  fontFamily: "Inter, sans-serif",
                  padding: "10px 14px",
                }}
                labelStyle={{ color: "#fff", fontWeight: 600 }}
                cursor={{ fill: "rgba(59, 130, 246, 0.06)" }}
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
          <p className="text-sm text-pilot-muted mt-3 text-center">
            <span className="font-semibold text-white">{totalReady}</span> ready / <span className="font-semibold text-white">{totalDesired}</span> desired across {deployments.length} deployments
          </p>
        </div>
      ) : (
        <div className="text-center text-pilot-muted text-sm py-12 bg-pilot-surface border border-pilot-border rounded-xl">
          No deployments found.
        </div>
      )}
    </section>
  );
}
