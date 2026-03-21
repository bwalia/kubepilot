/**
 * AnomalyTimeline — Real-time anomaly feed with severity badges.
 * Polls /api/v1/anomalies and displays them as a live timeline.
 */
import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, AlertOctagon, Info, Clock } from "lucide-react";
import { listAnomalies, type Anomaly, type Severity } from "@/lib/api";

const SEVERITY_STYLES: Record<Severity, { bg: string; text: string; icon: React.ReactNode }> = {
  critical: { bg: "bg-red-900/30", text: "text-red-400", icon: <AlertOctagon className="w-4 h-4" /> },
  high: { bg: "bg-orange-900/30", text: "text-orange-400", icon: <AlertTriangle className="w-4 h-4" /> },
  medium: { bg: "bg-yellow-900/30", text: "text-yellow-400", icon: <AlertTriangle className="w-4 h-4" /> },
  low: { bg: "bg-blue-900/30", text: "text-blue-400", icon: <Info className="w-4 h-4" /> },
  info: { bg: "bg-gray-800/50", text: "text-gray-400", icon: <Info className="w-4 h-4" /> },
};

export function AnomalyTimeline() {
  const { data: anomalies = [], isLoading } = useQuery({
    queryKey: ["anomalies"],
    queryFn: () => listAnomalies({ since: "1h" }),
    refetchInterval: 10_000,
  });

  if (isLoading) {
    return (
      <div className="text-pilot-muted text-sm py-10 text-center">
        Loading anomalies...
      </div>
    );
  }

  if (anomalies.length === 0) {
    return (
      <div className="text-pilot-muted text-sm py-10 text-center">
        No anomalies detected in the last hour.
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <h3 className="text-sm font-semibold text-pilot-muted uppercase tracking-wider">
        Anomaly Timeline
      </h3>
      <div className="space-y-2 max-h-[500px] overflow-y-auto">
        {anomalies.map((anomaly) => (
          <AnomalyRow key={anomaly.id} anomaly={anomaly} />
        ))}
      </div>
    </div>
  );
}

function AnomalyRow({ anomaly }: { anomaly: Anomaly }) {
  const style = SEVERITY_STYLES[anomaly.severity] || SEVERITY_STYLES.info;
  const ts = new Date(anomaly.detected_at);
  const timeStr = ts.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });

  return (
    <div className={`flex items-start gap-3 px-4 py-3 rounded-xl ${style.bg} border border-pilot-border`}>
      <div className={`mt-0.5 ${style.text}`}>{style.icon}</div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 text-sm">
          <span className={`font-bold uppercase text-xs ${style.text}`}>{anomaly.severity}</span>
          <span className="text-pilot-muted text-sm">{anomaly.rule}</span>
        </div>
        <p className="text-sm text-white mt-0.5 leading-relaxed">{anomaly.description}</p>
        <div className="flex items-center gap-2 text-sm text-pilot-muted mt-1">
          <span className="font-mono text-xs">{anomaly.resource.namespace}/{anomaly.resource.name}</span>
        </div>
      </div>
      <div className="flex items-center gap-1.5 text-sm text-pilot-muted shrink-0">
        <Clock className="w-3.5 h-3.5" />
        <span>{timeStr}</span>
      </div>
    </div>
  );
}
