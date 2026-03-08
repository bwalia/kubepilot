/**
 * AnomalyTimeline — Real-time anomaly feed with severity badges.
 * Polls /api/v1/anomalies and displays them as a live timeline.
 */
import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, AlertOctagon, Info, Clock } from "lucide-react";
import { listAnomalies, type Anomaly, type Severity } from "@/lib/api";

const SEVERITY_STYLES: Record<Severity, { bg: string; text: string; icon: React.ReactNode }> = {
  critical: { bg: "bg-red-900/40", text: "text-red-400", icon: <AlertOctagon className="w-3.5 h-3.5" /> },
  high: { bg: "bg-orange-900/40", text: "text-orange-400", icon: <AlertTriangle className="w-3.5 h-3.5" /> },
  medium: { bg: "bg-yellow-900/40", text: "text-yellow-400", icon: <AlertTriangle className="w-3.5 h-3.5" /> },
  low: { bg: "bg-blue-900/40", text: "text-blue-400", icon: <Info className="w-3.5 h-3.5" /> },
  info: { bg: "bg-gray-800", text: "text-gray-400", icon: <Info className="w-3.5 h-3.5" /> },
};

export function AnomalyTimeline() {
  const { data: anomalies = [], isLoading } = useQuery({
    queryKey: ["anomalies"],
    queryFn: () => listAnomalies({ since: "1h" }),
    refetchInterval: 10_000,
  });

  if (isLoading) {
    return (
      <div className="text-pilot-muted text-sm py-8 text-center">
        Loading anomalies...
      </div>
    );
  }

  if (anomalies.length === 0) {
    return (
      <div className="text-pilot-muted text-sm py-8 text-center">
        No anomalies detected in the last hour.
      </div>
    );
  }

  return (
    <div className="space-y-2">
      <h3 className="text-sm font-bold text-pilot-muted uppercase tracking-wider mb-3">
        Anomaly Timeline
      </h3>
      <div className="space-y-1 max-h-[500px] overflow-y-auto">
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
    <div className={`flex items-start gap-3 px-3 py-2 rounded ${style.bg} border border-pilot-border`}>
      <div className={`mt-0.5 ${style.text}`}>{style.icon}</div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 text-xs">
          <span className={`font-bold uppercase ${style.text}`}>{anomaly.severity}</span>
          <span className="text-pilot-muted">{anomaly.rule}</span>
        </div>
        <p className="text-sm text-white truncate">{anomaly.description}</p>
        <div className="flex items-center gap-2 text-xs text-pilot-muted mt-0.5">
          <span>{anomaly.resource.namespace}/{anomaly.resource.name}</span>
        </div>
      </div>
      <div className="flex items-center gap-1 text-xs text-pilot-muted shrink-0">
        <Clock className="w-3 h-3" />
        <span>{timeStr}</span>
      </div>
    </div>
  );
}
