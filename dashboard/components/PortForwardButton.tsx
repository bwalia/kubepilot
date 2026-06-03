/**
 * PortForwardButton — shared control for starting a kubectl-style port-forward
 * against a pod or service. Used on both the AI Troubleshooting page and the
 * Kubernetes Dashboard browser. Renders disabled with a tooltip when the server
 * has action mutations turned off.
 */
import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { ArrowRightLeft, Copy, Check, RefreshCw } from "lucide-react";
import { startPortForward, type PortForwardKind, type PortForwardSession } from "@/lib/api";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Tooltip, TooltipTrigger, TooltipContent, TooltipProvider } from "@/components/ui/tooltip";
import { Button } from "@/components/ui/button";

interface Props {
  kind: PortForwardKind;
  namespace: string;
  name: string;
  availablePorts?: number[];
  mutationsEnabled: boolean;
  onStarted?: () => void;
}

/** Build an absolute URL for a relative proxy path against the current origin. */
export function proxyURL(proxyPath: string): string {
  if (typeof window === "undefined") return proxyPath;
  return `${window.location.origin}${proxyPath}`;
}

type Phase = "pick" | "starting" | "result" | "error";

export function PortForwardButton({
  kind,
  namespace,
  name,
  availablePorts = [],
  mutationsEnabled,
  onStarted,
}: Props) {
  const [open, setOpen] = useState(false);
  const [phase, setPhase] = useState<Phase>("pick");
  const [remotePort, setRemotePort] = useState<string>(
    availablePorts.length > 0 ? String(availablePorts[0]) : ""
  );
  const [session, setSession] = useState<PortForwardSession | null>(null);
  const [errorMsg, setErrorMsg] = useState("");
  const [copied, setCopied] = useState(false);
  const queryClient = useQueryClient();

  const reset = () => {
    setPhase("pick");
    setSession(null);
    setErrorMsg("");
    setCopied(false);
  };

  const handleStart = async () => {
    setPhase("starting");
    try {
      const parsed = remotePort.trim() === "" ? undefined : Number(remotePort);
      const result = await startPortForward({
        kind,
        namespace,
        name,
        remote_port: parsed,
      });
      setSession(result);
      setPhase("result");
      queryClient.invalidateQueries({ queryKey: ["portforward-sessions"] });
      onStarted?.();
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        (err instanceof Error ? err.message : "Failed to start port forward");
      setErrorMsg(msg);
      setPhase("error");
    }
  };

  const copyProxyURL = async () => {
    if (!session) return;
    try {
      await navigator.clipboard.writeText(proxyURL(session.proxy_path));
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      /* clipboard may be unavailable over plain http; ignore */
    }
  };

  if (!mutationsEnabled) {
    return (
      <TooltipProvider>
        <Tooltip>
          <TooltipTrigger asChild>
            <span onClick={(e) => e.stopPropagation()}>
              <Button variant="outline" size="sm" disabled>
                <ArrowRightLeft className="w-3.5 h-3.5" /> Forward
              </Button>
            </span>
          </TooltipTrigger>
          <TooltipContent>
            Port forwarding is disabled on this server (enable action mutations to use it).
          </TooltipContent>
        </Tooltip>
      </TooltipProvider>
    );
  }

  return (
    <>
      <Button
        variant="outline"
        size="sm"
        onClick={(e) => {
          e.stopPropagation();
          reset();
          setOpen(true);
        }}
      >
        <ArrowRightLeft className="w-3.5 h-3.5" /> Forward
      </Button>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent onClick={(e) => e.stopPropagation()}>
          <DialogHeader>
            <DialogTitle>Port Forward</DialogTitle>
            <p className="text-sm text-pilot-muted font-mono">
              {kind}/{namespace}/{name}
            </p>
          </DialogHeader>

          {phase === "pick" && (
            <div className="space-y-4">
              <div>
                <label className="text-xs uppercase tracking-wider text-pilot-muted">
                  Target port
                </label>
                {availablePorts.length > 1 ? (
                  <select
                    value={remotePort}
                    onChange={(e) => setRemotePort(e.target.value)}
                    className="mt-1 w-full bg-pilot-surface border border-pilot-border rounded-lg px-3 py-2 text-sm text-white"
                  >
                    {availablePorts.map((p) => (
                      <option key={p} value={p}>
                        {p}
                      </option>
                    ))}
                  </select>
                ) : (
                  <input
                    type="number"
                    value={remotePort}
                    onChange={(e) => setRemotePort(e.target.value)}
                    placeholder={availablePorts.length === 1 ? String(availablePorts[0]) : "e.g. 8080"}
                    className="mt-1 w-full bg-pilot-surface border border-pilot-border rounded-lg px-3 py-2 text-sm text-white font-mono"
                  />
                )}
                <p className="text-xs text-pilot-muted mt-1">
                  Leave blank to auto-select the first exposed port.
                </p>
              </div>
              <div className="flex justify-end gap-2">
                <Button variant="ghost" onClick={() => setOpen(false)}>
                  Cancel
                </Button>
                <Button onClick={handleStart}>Start</Button>
              </div>
            </div>
          )}

          {phase === "starting" && (
            <div className="flex items-center gap-3 text-pilot-muted text-sm py-6">
              <RefreshCw className="w-5 h-5 animate-spin" /> Starting port forward...
            </div>
          )}

          {phase === "result" && session && (
            <div className="space-y-4">
              <div>
                <label className="text-xs uppercase tracking-wider text-pilot-muted">
                  Access URL (HTTP, remote port {session.remote_port})
                </label>
                <div className="mt-1 flex items-center gap-2">
                  <a
                    href={proxyURL(session.proxy_path)}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex-1 bg-pilot-surface-2 border border-pilot-border rounded-lg px-3 py-2 text-sm text-pilot-accent-light font-mono truncate hover:underline"
                  >
                    {proxyURL(session.proxy_path)}
                  </a>
                  <Button variant="outline" size="sm" onClick={copyProxyURL}>
                    {copied ? <Check className="w-3.5 h-3.5" /> : <Copy className="w-3.5 h-3.5" />}
                  </Button>
                </div>
                <p className="text-xs text-pilot-muted mt-1 font-mono">
                  Direct (server host only): {session.address}
                </p>
              </div>
              <div className="text-xs text-pilot-text-secondary bg-pilot-accent/10 border border-pilot-accent/20 rounded-lg p-3 leading-relaxed">
                The Access URL proxies through KubePilot, so it works even when KubePilot runs in a
                container. It supports <span className="font-semibold">HTTP services only</span> —
                raw TCP protocols (databases, DNS) can&apos;t be tunnelled through a browser; for
                those, use the direct address from a host that can reach the server.
              </div>
              <div className="flex justify-end">
                <Button onClick={() => setOpen(false)}>Done</Button>
              </div>
            </div>
          )}

          {phase === "error" && (
            <div className="space-y-4">
              <div className="text-sm text-pilot-danger bg-red-500/10 border border-pilot-danger/30 rounded-lg p-3">
                {errorMsg}
              </div>
              <div className="flex justify-end gap-2">
                <Button variant="ghost" onClick={() => setOpen(false)}>
                  Close
                </Button>
                <Button onClick={() => setPhase("pick")}>Try Again</Button>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}
