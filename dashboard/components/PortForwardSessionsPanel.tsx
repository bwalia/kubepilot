/**
 * PortForwardSessionsPanel — lists active/terminated port-forward sessions and
 * lets the user stop them. Polls every 5s. Renders nothing when there are no
 * sessions so it stays out of the way until used.
 */
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRightLeft, X } from "lucide-react";
import { listPortForwards, stopPortForward, type PortForwardSession } from "@/lib/api";
import { proxyURL } from "@/components/PortForwardButton";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

export function PortForwardSessionsPanel({ mutationsEnabled }: { mutationsEnabled: boolean }) {
  const queryClient = useQueryClient();
  const { data: sessions = [] } = useQuery({
    queryKey: ["portforward-sessions"],
    queryFn: listPortForwards,
    refetchInterval: 5_000,
  });

  if (sessions.length === 0) return null;

  const handleStop = async (id: string) => {
    try {
      await stopPortForward(id);
    } catch {
      /* session may already be gone */
    }
    queryClient.invalidateQueries({ queryKey: ["portforward-sessions"] });
  };

  return (
    <section>
      <h2 className="text-base font-display font-bold text-pilot-text-primary mb-3 flex items-center gap-2">
        <ArrowRightLeft className="w-4 h-4 text-pilot-accent" /> Active Port Forwards
      </h2>
      <div className="bg-pilot-surface border border-pilot-border rounded-xl overflow-hidden shadow-card">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr>
                <th className="text-left px-5 py-3 eyebrow">Kind</th>
                <th className="text-left px-5 py-3 eyebrow">Target</th>
                <th className="text-center px-5 py-3 eyebrow">Remote</th>
                <th className="text-left px-5 py-3 eyebrow">Access URL</th>
                <th className="text-left px-5 py-3 eyebrow">Status</th>
                <th className="text-right px-5 py-3 eyebrow">Action</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-pilot-border">
              {sessions.map((s) => (
                <SessionRow key={s.id} session={s} mutationsEnabled={mutationsEnabled} onStop={handleStop} />
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}

function SessionRow({
  session,
  mutationsEnabled,
  onStop,
}: {
  session: PortForwardSession;
  mutationsEnabled: boolean;
  onStop: (id: string) => void;
}) {
  const statusVariant =
    session.status === "active" ? "success" : session.status === "error" ? "danger" : "muted";
  return (
    <tr className="hover:bg-pilot-accent/[0.03]">
      <td className="px-5 py-3 text-pilot-text-secondary capitalize">{session.kind}</td>
      <td className="px-5 py-3 text-pilot-text-primary font-mono">
        {session.namespace}/{session.name}
      </td>
      <td className="px-5 py-3 text-center text-pilot-text-secondary">{session.remote_port}</td>
      <td className="px-5 py-3 font-mono">
        {session.status === "active" ? (
          <a
            href={proxyURL(session.proxy_path)}
            target="_blank"
            rel="noopener noreferrer"
            className="text-pilot-accent-light hover:underline"
            onClick={(e) => e.stopPropagation()}
          >
            {session.proxy_path}
          </a>
        ) : (
          <span className="text-pilot-muted">{session.address}</span>
        )}
      </td>
      <td className="px-5 py-3">
        <Badge variant={statusVariant}>{session.status}</Badge>
        {session.error && <span className="text-xs text-pilot-danger ml-2">{session.error}</span>}
      </td>
      <td className="px-5 py-3 text-right">
        {session.status === "active" && (
          <Button
            variant="outline"
            size="sm"
            disabled={!mutationsEnabled}
            title={mutationsEnabled ? undefined : "Mutations are disabled on this server"}
            onClick={() => onStop(session.id)}
          >
            <X className="w-3.5 h-3.5" /> Stop
          </Button>
        )}
      </td>
    </tr>
  );
}
