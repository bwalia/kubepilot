import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  addKubeconfigPath,
  listKubeconfigs,
  switchCluster,
  switchContext,
  uploadKubeconfigBase64,
  uploadKubeconfig,
} from "@/lib/api";
import { Link2, Upload, RefreshCw, X } from "lucide-react";

interface Props {
  onSwitched?: () => void;
}

export function KubeconfigSwitcher({ onSwitched }: Props) {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [newPath, setNewPath] = useState("");
  const [base64Content, setBase64Content] = useState("");
  const [base64Name, setBase64Name] = useState("");
  const [selectedPath, setSelectedPath] = useState("");
  const [error, setError] = useState<string | null>(null);

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["kubeconfigs"],
    queryFn: listKubeconfigs,
  });

  const paths = data?.paths ?? [];
  const activePath = data?.active_path ?? "";
  const contexts = data?.contexts ?? [];
  const activeContext = data?.active_context ?? "";

  const normalizedSelectedPath = useMemo(() => {
    if (selectedPath) {
      return selectedPath;
    }
    return activePath;
  }, [selectedPath, activePath]);

  const invalidateClusterData = async () => {
    await queryClient.invalidateQueries({ queryKey: ["kubeconfigs"] });
    await queryClient.invalidateQueries({ queryKey: ["crashing-pods"] });
    await queryClient.invalidateQueries({ queryKey: ["nodes"] });
    await queryClient.invalidateQueries({ queryKey: ["deployments"] });
    await queryClient.invalidateQueries({ queryKey: ["anomalies-count"] });
    onSwitched?.();
  };

  const addPathMutation = useMutation({
    mutationFn: (path: string) => addKubeconfigPath(path, true),
    onSuccess: async () => {
      setError(null);
      setNewPath("");
      await invalidateClusterData();
    },
    onError: (err: any) => {
      setError(err?.response?.data?.error ?? err?.message ?? "Failed to add kubeconfig path");
    },
  });

  const switchMutation = useMutation({
    mutationFn: (path: string) => switchCluster(path),
    onSuccess: async () => {
      setError(null);
      await invalidateClusterData();
    },
    onError: (err: any) => {
      setError(err?.response?.data?.error ?? err?.message ?? "Failed to switch cluster");
    },
  });

  const uploadMutation = useMutation({
    mutationFn: (file: File) => uploadKubeconfig(file),
    onSuccess: async () => {
      setError(null);
      await invalidateClusterData();
    },
    onError: (err: any) => {
      setError(err?.response?.data?.error ?? err?.message ?? "Failed to upload kubeconfig");
    },
  });

  const uploadBase64Mutation = useMutation({
    mutationFn: (payload: { content: string; name?: string }) =>
      uploadKubeconfigBase64(payload.content, payload.name),
    onSuccess: async () => {
      setError(null);
      setBase64Content("");
      setBase64Name("");
      await invalidateClusterData();
    },
    onError: (err: any) => {
      setError(err?.response?.data?.error ?? err?.message ?? "Failed to upload base64 kubeconfig");
    },
  });

  const switchContextMutation = useMutation({
    mutationFn: (ctx: string) => switchContext(ctx),
    onSuccess: async () => {
      setError(null);
      await invalidateClusterData();
    },
    onError: (err: any) => {
      setError(err?.response?.data?.error ?? err?.message ?? "Failed to switch context");
    },
  });

  const busy =
    addPathMutation.isPending ||
    switchMutation.isPending ||
    uploadMutation.isPending ||
    uploadBase64Mutation.isPending ||
    switchContextMutation.isPending;

  return (
    <>
      <button
        onClick={() => setOpen(true)}
        className="text-sm border border-pilot-border bg-pilot-surface hover:border-pilot-border-hover px-3.5 py-2 rounded-lg flex items-center gap-2 font-medium"
      >
        <Link2 className="w-4 h-4 text-pilot-accent" />
        {activeContext || "Cluster"}
      </button>

      {open && (
        <div className="fixed inset-0 z-[70] bg-black/60 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="w-full max-w-2xl max-h-[90vh] overflow-y-auto bg-pilot-bg border border-pilot-border rounded-2xl p-6 space-y-5 animate-fade-in shadow-card-hover">
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-display font-bold text-pilot-text-primary">Switch Cluster / Kubeconfig</h3>
              <button onClick={() => setOpen(false)} className="text-pilot-muted hover:text-pilot-text-primary p-1.5 rounded-lg hover:bg-pilot-surface">
                <X className="w-5 h-5" />
              </button>
            </div>

            <div className="text-sm text-pilot-muted">
              Active kubeconfig: <span className="text-pilot-text-primary font-mono">{activePath || "(in-cluster)"}</span>
              {activeContext && (
                <> &middot; Context: <span className="text-pilot-text-primary font-mono">{activeContext}</span></>
              )}
            </div>

            {contexts.length > 0 && (
              <div className="space-y-2">
                <label className="text-sm text-pilot-muted font-medium">Kubernetes context</label>
                <select
                  value={activeContext}
                  onChange={(e) => {
                    if (e.target.value && e.target.value !== activeContext) {
                      switchContextMutation.mutate(e.target.value);
                    }
                  }}
                  disabled={busy}
                  className="w-full bg-pilot-surface border border-pilot-border rounded-lg px-4 py-2.5 text-sm text-pilot-text-primary focus:outline-none focus:border-pilot-accent/60 focus:ring-2 focus:ring-pilot-accent/25"
                >
                  {contexts.map((ctx) => (
                    <option key={ctx.name} value={ctx.name}>
                      {ctx.name} ({ctx.cluster})
                    </option>
                  ))}
                </select>
              </div>
            )}

            <div className="space-y-3 border-t border-pilot-border pt-5">
              <label className="text-sm text-pilot-muted font-medium">Saved kubeconfig paths</label>
              <div className="flex gap-2">
                <select
                  value={normalizedSelectedPath}
                  onChange={(e) => setSelectedPath(e.target.value)}
                  className="flex-1 min-w-0 truncate bg-pilot-surface border border-pilot-border rounded-lg px-4 py-2.5 text-sm text-pilot-text-primary focus:outline-none focus:border-pilot-accent/60 focus:ring-2 focus:ring-pilot-accent/25"
                >
                  <option value="">Select a kubeconfig path</option>
                  {paths.map((p) => (
                    <option key={p} value={p}>
                      {p}
                    </option>
                  ))}
                </select>
                <button
                  onClick={() => refetch()}
                  className="border border-pilot-border bg-pilot-surface text-pilot-text-secondary px-3 py-2.5 rounded-lg hover:border-pilot-accent/50 hover:text-pilot-text-primary"
                  title="Refresh list"
                >
                  <RefreshCw className="w-4 h-4" />
                </button>
              </div>
              <button
                disabled={!normalizedSelectedPath || busy}
                onClick={() => switchMutation.mutate(normalizedSelectedPath)}
                className="bg-pilot-accent hover:bg-pilot-accent-light disabled:opacity-50 text-pilot-bg text-sm px-4 py-2.5 rounded-lg font-semibold"
              >
                Connect selected cluster
              </button>
            </div>

            <div className="space-y-3 border-t border-pilot-border pt-5">
              <label className="text-sm text-pilot-muted font-medium">Add existing kubeconfig path on server</label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={newPath}
                  onChange={(e) => setNewPath(e.target.value)}
                  placeholder="/path/to/kubeconfig"
                  className="flex-1 bg-pilot-surface border border-pilot-border rounded-lg px-4 py-2.5 text-sm text-pilot-text-primary placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent/60 focus:ring-2 focus:ring-pilot-accent/25"
                />
                <button
                  disabled={!newPath.trim() || busy}
                  onClick={() => addPathMutation.mutate(newPath.trim())}
                  className="bg-pilot-success text-pilot-bg disabled:opacity-50 text-sm px-4 py-2.5 rounded-lg font-semibold"
                >
                  Save + Connect
                </button>
              </div>
            </div>

            <div className="space-y-3 border-t border-pilot-border pt-5">
              <label className="text-sm text-pilot-muted font-medium">Upload kubeconfig file and connect</label>
              <label className="inline-flex items-center gap-2 text-sm border border-pilot-border bg-pilot-surface hover:border-pilot-border-hover px-4 py-2.5 rounded-lg cursor-pointer font-medium">
                <Upload className="w-4 h-4" />
                <span>Choose kubeconfig file</span>
                <input
                  type="file"
                  accept=".yaml,.yml,.conf,.kubeconfig,*/*"
                  className="hidden"
                  onChange={(e) => {
                    const file = e.target.files?.[0];
                    if (file) {
                      uploadMutation.mutate(file);
                    }
                    e.currentTarget.value = "";
                  }}
                />
              </label>
            </div>

            <div className="space-y-3 border-t border-pilot-border pt-5">
              <label className="text-sm text-pilot-muted font-medium">Paste kubeconfig as base64 and connect</label>
              <input
                type="text"
                value={base64Name}
                onChange={(e) => setBase64Name(e.target.value)}
                placeholder="Optional filename (e.g. prod-us.kubeconfig)"
                className="w-full bg-pilot-surface border border-pilot-border rounded-lg px-4 py-2.5 text-sm text-pilot-text-primary placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent/60 focus:ring-2 focus:ring-pilot-accent/25"
              />
              <textarea
                value={base64Content}
                onChange={(e) => setBase64Content(e.target.value)}
                placeholder="Paste base64 kubeconfig content"
                className="w-full min-h-28 bg-pilot-surface border border-pilot-border rounded-lg px-4 py-2.5 text-sm text-pilot-text-primary placeholder:text-pilot-muted font-mono focus:outline-none focus:border-pilot-accent/60 focus:ring-2 focus:ring-pilot-accent/25 resize-none"
              />
              <button
                disabled={!base64Content.trim() || busy}
                onClick={() =>
                  uploadBase64Mutation.mutate({
                    content: base64Content.trim(),
                    name: base64Name.trim() || undefined,
                  })
                }
                className="bg-pilot-accent hover:bg-pilot-accent-light disabled:opacity-50 text-pilot-bg text-sm px-4 py-2.5 rounded-lg font-semibold"
              >
                Save base64 + Connect
              </button>
            </div>

            {isLoading && <div className="text-sm text-pilot-muted">Loading kubeconfig profiles...</div>}
            {error && <div className="text-sm text-pilot-danger bg-pilot-danger/10 border border-pilot-danger/40 rounded-xl p-3">{error}</div>}
          </div>
        </div>
      )}
    </>
  );
}
