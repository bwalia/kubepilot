/**
 * ClusterSwitcher — the top-bar cluster picker.
 *
 * Picking a cluster changes only this tab: the choice travels with each API
 * request (X-KubePilot-Cluster), so other tabs and other people keep their own.
 * "Manage clusters" adds kubeconfigs to the server, sets the server default
 * (which background Autopilot and RCA watch) and forgets stale files.
 */
import { useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Check, ChevronsUpDown, Plus, Server, Settings2, Star, Trash2, Upload, RefreshCw } from "lucide-react";
import {
  addClusterKubeconfig,
  listClusters,
  removeKubeconfig,
  setDefaultCluster,
  uploadClusterKubeconfig,
  uploadClusterKubeconfigBase64,
  type ClusterInfo,
  type ClusterList,
} from "@/lib/api";
import { useCluster } from "@/lib/useCluster";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { cn } from "@/lib/utils";

const CLUSTERS_KEY = ["clusters"] as const;

function errorText(err: any, fallback: string): string {
  return err?.response?.data?.error ?? err?.message ?? fallback;
}

function host(server?: string): string {
  if (!server) return "";
  try {
    return new URL(server).host;
  } catch {
    return server;
  }
}

function StatusDot({ c }: { c?: ClusterInfo }) {
  const tone =
    c?.reachable === undefined ? "bg-pilot-muted/60" : c.reachable ? "bg-pilot-success" : "bg-pilot-danger";
  return <span className={cn("inline-block h-2 w-2 shrink-0 rounded-full", tone)} aria-hidden="true" />;
}

/** The selected cluster, resolving "" to the server default. */
export function useActiveCluster(): { list?: ClusterList; active?: ClusterInfo } {
  const { clusterId } = useCluster();
  // The plain list answers instantly; the probe waits on every API server
  // (up to a few seconds for a dead one), so it only decorates the list.
  const { data: plain } = useQuery({
    queryKey: [...CLUSTERS_KEY, "plain"],
    queryFn: () => listClusters(false),
    staleTime: 30_000,
  });
  const { data: probed } = useQuery({
    queryKey: [...CLUSTERS_KEY, "probe"],
    queryFn: () => listClusters(true),
    refetchInterval: 60_000,
    staleTime: 30_000,
  });
  const data = probed ?? plain;
  const active =
    data?.clusters.find((c) => c.id === clusterId) ?? data?.clusters.find((c) => c.id === data.default_id);
  return { list: data, active };
}

export function ClusterSwitcher() {
  const { clusterId, setClusterId } = useCluster();
  const { list, active } = useActiveCluster();
  const [open, setOpen] = useState(false);
  const [manage, setManage] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false);
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const clusters = list?.clusters ?? [];
  const pick = (c: ClusterInfo) => {
    // Selecting the default stores its ID, not "": the tab should stay on
    // this cluster even if someone later moves the server default.
    if (c.id !== active?.id || clusterId === "") setClusterId(c.id);
    setOpen(false);
  };

  return (
    <div ref={rootRef} className="relative">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-haspopup="listbox"
        aria-expanded={open}
        className="inline-flex h-10 max-w-[10rem] items-center gap-2 rounded-xl border border-pilot-border bg-pilot-surface px-3 text-sm font-medium text-pilot-text-primary transition-colors hover:border-pilot-border-hover sm:max-w-[15rem]"
        title={active ? `Cluster ${active.name}${active.server ? ` (${host(active.server)})` : ""}` : "Cluster"}
      >
        <Server className="h-4 w-4 shrink-0 text-pilot-accent" aria-hidden="true" />
        <StatusDot c={active} />
        <span className="truncate font-mono">{active?.name ?? "Cluster"}</span>
        <ChevronsUpDown className="ml-auto h-4 w-4 shrink-0 text-pilot-muted" aria-hidden="true" />
      </button>

      {open && (
        <div className="absolute right-0 z-50 mt-2 w-[min(24rem,calc(100vw-2rem))] overflow-hidden rounded-xl border border-pilot-border bg-pilot-surface shadow-card-hover">
          <div className="border-b border-pilot-border px-3 py-2 text-xs text-pilot-muted">
            Switches this tab only. Other tabs and users keep their own cluster.
          </div>
          <ul role="listbox" aria-label="Clusters" className="max-h-80 overflow-y-auto py-1">
            {clusters.length === 0 && <li className="px-3 py-2 text-sm text-pilot-muted">Loading clusters…</li>}
            {clusters.map((c) => {
              const selected = c.id === active?.id;
              return (
                <li key={c.id} role="option" aria-selected={selected}>
                  <button
                    type="button"
                    onClick={() => pick(c)}
                    className={cn(
                      "flex w-full items-center gap-2.5 px-3 py-2 text-left text-sm hover:bg-pilot-hover/[0.06]",
                      selected && "bg-pilot-accent/10"
                    )}
                  >
                    <StatusDot c={c} />
                    <span className="min-w-0 flex-1">
                      <span className="flex items-center gap-1.5">
                        <span className="truncate font-mono text-pilot-text-primary">{c.name}</span>
                        {c.default && (
                          <span className="rounded border border-pilot-border px-1 text-[0.65rem] uppercase tracking-wide text-pilot-muted">
                            default
                          </span>
                        )}
                      </span>
                      <span className="block truncate text-xs text-pilot-muted">
                        {[host(c.server), c.version, c.reachable === false ? "unreachable" : ""]
                          .filter(Boolean)
                          .join(" · ") || c.context}
                      </span>
                    </span>
                    {selected && <Check className="h-4 w-4 shrink-0 text-pilot-accent" aria-hidden="true" />}
                  </button>
                </li>
              );
            })}
          </ul>
          <button
            type="button"
            onClick={() => {
              setOpen(false);
              setManage(true);
            }}
            className="flex w-full items-center gap-2 border-t border-pilot-border px-3 py-2.5 text-sm font-medium text-pilot-text-secondary hover:bg-pilot-hover/[0.06] hover:text-pilot-text-primary"
          >
            <Settings2 className="h-4 w-4" aria-hidden="true" />
            Manage clusters…
          </button>
        </div>
      )}

      <ManageClustersDialog open={manage} onOpenChange={setManage} activeId={active?.id} />
    </div>
  );
}

/** Accepts either a raw kubeconfig or its base64 form. */
function toBase64(text: string): string {
  const t = text.trim();
  if (/^(apiVersion|kind|clusters|contexts|current-context|users|preferences):/m.test(t)) {
    return btoa(unescape(encodeURIComponent(t)));
  }
  return t;
}

function ManageClustersDialog({
  open,
  onOpenChange,
  activeId,
}: {
  open: boolean;
  onOpenChange: (o: boolean) => void;
  activeId?: string;
}) {
  const queryClient = useQueryClient();
  const { setClusterId } = useCluster();
  const [path, setPath] = useState("");
  const [paste, setPaste] = useState("");
  const [pasteName, setPasteName] = useState("");
  const [error, setError] = useState<string | null>(null);

  const { data, isFetching, refetch } = useQuery({
    queryKey: [...CLUSTERS_KEY, "probe"],
    queryFn: () => listClusters(true),
    enabled: open,
  });

  const refresh = () => queryClient.invalidateQueries({ queryKey: CLUSTERS_KEY });

  // A newly added kubeconfig is selected for this tab straight away.
  const onAdded = async (res: ClusterList) => {
    setError(null);
    setPath("");
    setPaste("");
    setPasteName("");
    await refresh();
    const first = res.added_ids?.[0];
    if (first) {
      onOpenChange(false);
      setClusterId(first);
    }
  };
  const onFail = (fallback: string) => (err: any) => setError(errorText(err, fallback));

  const addPath = useMutation({ mutationFn: addClusterKubeconfig, onSuccess: onAdded, onError: onFail("Could not add that kubeconfig") });
  const upload = useMutation({ mutationFn: uploadClusterKubeconfig, onSuccess: onAdded, onError: onFail("Upload failed") });
  const pasteMut = useMutation({
    mutationFn: () => uploadClusterKubeconfigBase64(toBase64(paste), pasteName.trim() || undefined),
    onSuccess: onAdded,
    onError: onFail("Could not read that kubeconfig"),
  });
  const makeDefault = useMutation({
    mutationFn: setDefaultCluster,
    onSuccess: () => {
      setError(null);
      void refresh();
    },
    onError: onFail("Could not change the default cluster"),
  });
  const forget = useMutation({
    mutationFn: removeKubeconfig,
    onSuccess: () => {
      setError(null);
      void refresh();
    },
    onError: onFail("Could not remove that kubeconfig"),
  });

  const busy = addPath.isPending || upload.isPending || pasteMut.isPending || makeDefault.isPending || forget.isPending;
  const canMutate = data?.mutations !== false;

  // Group contexts under the kubeconfig file they came from.
  const groups = new Map<string, ClusterInfo[]>();
  for (const c of data?.clusters ?? []) {
    groups.set(c.kubeconfig, [...(groups.get(c.kubeconfig) ?? []), c]);
  }

  const input =
    "w-full rounded-lg border border-pilot-border bg-pilot-surface-2 px-3 py-2 text-sm text-pilot-text-primary placeholder:text-pilot-muted focus:border-pilot-accent/60 focus:outline-none focus:ring-2 focus:ring-pilot-accent/25";
  const primary =
    "inline-flex items-center gap-1.5 rounded-lg bg-pilot-accent px-3 py-2 text-sm font-semibold text-pilot-bg hover:bg-pilot-accent-light disabled:opacity-50";

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] max-w-2xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Clusters</DialogTitle>
          <DialogDescription>
            Each tab picks its own cluster from this list. The default cluster is what Autopilot, RCA and new tabs use.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h4 className="text-sm font-semibold text-pilot-text-primary">Saved clusters</h4>
            <button
              type="button"
              onClick={() => refetch()}
              className="inline-flex items-center gap-1 text-xs text-pilot-muted hover:text-pilot-text-primary"
            >
              <RefreshCw className={cn("h-3.5 w-3.5", isFetching && "animate-spin")} aria-hidden="true" /> Re-check
            </button>
          </div>

          {[...groups.entries()].map(([file, items]) => {
            const isDefaultFile = items.some((c) => c.default);
            return (
              <div key={file} className="rounded-xl border border-pilot-border">
                <div className="flex items-center gap-2 border-b border-pilot-border px-3 py-2">
                  <span className="min-w-0 flex-1 truncate font-mono text-xs text-pilot-muted" title={file}>
                    {file}
                  </span>
                  {canMutate && !isDefaultFile && (
                    <button
                      type="button"
                      disabled={busy}
                      onClick={() => forget.mutate(file)}
                      className="inline-flex items-center gap-1 text-xs text-pilot-muted hover:text-pilot-danger disabled:opacity-50"
                      title="Forget this kubeconfig (the file is not deleted)"
                    >
                      <Trash2 className="h-3.5 w-3.5" aria-hidden="true" /> Forget
                    </button>
                  )}
                </div>
                <ul>
                  {items.map((c) => (
                    <li key={c.id} className="flex items-center gap-2.5 px-3 py-2 text-sm">
                      <StatusDot c={c} />
                      <span className="min-w-0 flex-1">
                        <span className="block truncate font-mono text-pilot-text-primary">
                          {c.name}
                          {c.id === activeId && <span className="ml-2 text-xs text-pilot-accent">this tab</span>}
                        </span>
                        <span className="block truncate text-xs text-pilot-muted" title={c.error}>
                          {c.reachable === false ? c.error || "unreachable" : [host(c.server), c.version].filter(Boolean).join(" · ")}
                        </span>
                      </span>
                      {c.default ? (
                        <span className="inline-flex items-center gap-1 text-xs text-pilot-muted">
                          <Star className="h-3.5 w-3.5 fill-current text-pilot-warning" aria-hidden="true" /> Default
                        </span>
                      ) : (
                        canMutate && (
                          <button
                            type="button"
                            disabled={busy}
                            onClick={() => makeDefault.mutate(c.id)}
                            className="text-xs text-pilot-text-secondary hover:text-pilot-text-primary disabled:opacity-50"
                          >
                            Make default
                          </button>
                        )
                      )}
                      {c.id !== activeId && (
                        <button
                          type="button"
                          onClick={() => {
                            onOpenChange(false);
                            setClusterId(c.id);
                          }}
                          className="rounded-lg border border-pilot-border px-2 py-1 text-xs font-medium text-pilot-text-primary hover:border-pilot-accent/50"
                        >
                          Open
                        </button>
                      )}
                    </li>
                  ))}
                </ul>
              </div>
            );
          })}

          {(data?.unavailable ?? []).map((u) => (
            <div key={u.path} className="flex items-center gap-2 rounded-xl border border-pilot-danger/40 bg-pilot-danger/5 px-3 py-2 text-xs">
              <span className="min-w-0 flex-1">
                <span className="block truncate font-mono text-pilot-text-primary">{u.path}</span>
                <span className="block truncate text-pilot-muted">{u.error}</span>
              </span>
              {canMutate && (
                <button type="button" disabled={busy} onClick={() => forget.mutate(u.path)} className="text-pilot-muted hover:text-pilot-danger">
                  Forget
                </button>
              )}
            </div>
          ))}

          {canMutate ? (
            <div className="space-y-4 border-t border-pilot-border pt-4">
              <h4 className="text-sm font-semibold text-pilot-text-primary">Add a cluster</h4>

              <div className="space-y-2">
                <label className="text-xs font-medium text-pilot-muted" htmlFor="kp-add-path">
                  Kubeconfig already on the server
                </label>
                <div className="flex gap-2">
                  <input id="kp-add-path" value={path} onChange={(e) => setPath(e.target.value)} placeholder="~/.kube/prod.yaml" className={input} />
                  <button type="button" disabled={!path.trim() || busy} onClick={() => addPath.mutate(path.trim())} className={primary}>
                    <Plus className="h-4 w-4" aria-hidden="true" /> Add
                  </button>
                </div>
              </div>

              <div className="space-y-2">
                <span className="text-xs font-medium text-pilot-muted">Upload a kubeconfig file</span>
                <label className="flex cursor-pointer items-center gap-2 rounded-lg border border-dashed border-pilot-border px-3 py-2.5 text-sm text-pilot-text-secondary hover:border-pilot-accent/50">
                  <Upload className="h-4 w-4" aria-hidden="true" />
                  Choose file…
                  <input
                    type="file"
                    accept=".yaml,.yml,.conf,.kubeconfig,*/*"
                    className="hidden"
                    disabled={busy}
                    onChange={(e) => {
                      const file = e.target.files?.[0];
                      if (file) upload.mutate(file);
                      e.currentTarget.value = "";
                    }}
                  />
                </label>
              </div>

              <div className="space-y-2">
                <label className="text-xs font-medium text-pilot-muted" htmlFor="kp-paste">
                  Or paste a kubeconfig (YAML or base64)
                </label>
                <input value={pasteName} onChange={(e) => setPasteName(e.target.value)} placeholder="Name, e.g. prod-eu.yaml (optional)" className={input} />
                <textarea id="kp-paste" value={paste} onChange={(e) => setPaste(e.target.value)} className={cn(input, "min-h-28 resize-y font-mono text-xs")} />
                <button type="button" disabled={!paste.trim() || busy} onClick={() => pasteMut.mutate()} className={primary}>
                  <Plus className="h-4 w-4" aria-hidden="true" /> Add pasted kubeconfig
                </button>
              </div>
            </div>
          ) : (
            <p className="border-t border-pilot-border pt-4 text-xs text-pilot-muted">
              Adding clusters is disabled on this server (start it with --enable-kubeconfig-mutations).
            </p>
          )}

          {error && <div className="rounded-xl border border-pilot-danger/40 bg-pilot-danger/10 p-3 text-sm text-pilot-danger">{error}</div>}
        </div>
      </DialogContent>
    </Dialog>
  );
}
