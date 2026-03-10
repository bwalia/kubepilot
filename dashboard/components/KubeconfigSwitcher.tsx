import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  addKubeconfigPath,
  listKubeconfigs,
  switchCluster,
  uploadKubeconfigBase64,
  uploadKubeconfig,
} from "@/lib/api";
import { Link2, Upload, RefreshCw } from "lucide-react";

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

  const busy =
    addPathMutation.isPending ||
    switchMutation.isPending ||
    uploadMutation.isPending ||
    uploadBase64Mutation.isPending;

  return (
    <>
      <button
        onClick={() => setOpen(true)}
        className="text-xs border border-pilot-border bg-pilot-surface hover:border-pilot-accent px-3 py-1.5 rounded flex items-center gap-1.5"
      >
        <Link2 className="w-3.5 h-3.5 text-pilot-accent" />
        Cluster
      </button>

      {open && (
        <div className="fixed inset-0 z-50 bg-black/70 flex items-center justify-center p-4">
          <div className="w-full max-w-2xl bg-pilot-bg border border-pilot-border rounded-lg p-5 space-y-4">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-bold text-white">Switch Cluster / Kubeconfig</h3>
              <button
                onClick={() => setOpen(false)}
                className="text-xs text-pilot-muted hover:text-white"
              >
                Close
              </button>
            </div>

            <div className="text-xs text-pilot-muted">
              Active kubeconfig: <span className="text-white font-mono">{activePath || "(in-cluster)"}</span>
            </div>

            <div className="space-y-2">
              <label className="text-xs text-pilot-muted">Saved kubeconfig paths</label>
              <div className="flex gap-2">
                <select
                  value={normalizedSelectedPath}
                  onChange={(e) => setSelectedPath(e.target.value)}
                  className="flex-1 bg-pilot-surface border border-pilot-border rounded px-3 py-2 text-xs"
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
                  className="border border-pilot-border bg-pilot-surface px-3 py-2 rounded"
                  title="Refresh list"
                >
                  <RefreshCw className="w-4 h-4" />
                </button>
              </div>
              <button
                disabled={!normalizedSelectedPath || busy}
                onClick={() => switchMutation.mutate(normalizedSelectedPath)}
                className="bg-pilot-accent hover:bg-blue-500 disabled:opacity-50 text-white text-xs px-3 py-2 rounded"
              >
                Connect selected cluster
              </button>
            </div>

            <div className="space-y-2 border-t border-pilot-border pt-4">
              <label className="text-xs text-pilot-muted">Add existing kubeconfig path on server</label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={newPath}
                  onChange={(e) => setNewPath(e.target.value)}
                  placeholder="/path/to/kubeconfig"
                  className="flex-1 bg-pilot-surface border border-pilot-border rounded px-3 py-2 text-xs"
                />
                <button
                  disabled={!newPath.trim() || busy}
                  onClick={() => addPathMutation.mutate(newPath.trim())}
                  className="bg-pilot-success text-black disabled:opacity-50 text-xs px-3 py-2 rounded font-semibold"
                >
                  Save + Connect
                </button>
              </div>
            </div>

            <div className="space-y-2 border-t border-pilot-border pt-4">
              <label className="text-xs text-pilot-muted">Upload kubeconfig file and connect</label>
              <label className="inline-flex items-center gap-2 text-xs border border-pilot-border bg-pilot-surface hover:border-pilot-accent px-3 py-2 rounded cursor-pointer">
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

            <div className="space-y-2 border-t border-pilot-border pt-4">
              <label className="text-xs text-pilot-muted">Paste kubeconfig as base64 and connect</label>
              <input
                type="text"
                value={base64Name}
                onChange={(e) => setBase64Name(e.target.value)}
                placeholder="Optional filename (e.g. prod-us.kubeconfig)"
                className="w-full bg-pilot-surface border border-pilot-border rounded px-3 py-2 text-xs"
              />
              <textarea
                value={base64Content}
                onChange={(e) => setBase64Content(e.target.value)}
                placeholder="Paste base64 kubeconfig content"
                className="w-full min-h-28 bg-pilot-surface border border-pilot-border rounded px-3 py-2 text-xs font-mono"
              />
              <button
                disabled={!base64Content.trim() || busy}
                onClick={() =>
                  uploadBase64Mutation.mutate({
                    content: base64Content.trim(),
                    name: base64Name.trim() || undefined,
                  })
                }
                className="bg-pilot-accent hover:bg-blue-500 disabled:opacity-50 text-white text-xs px-3 py-2 rounded"
              >
                Save base64 + Connect
              </button>
            </div>

            {isLoading && <div className="text-xs text-pilot-muted">Loading kubeconfig profiles...</div>}
            {error && <div className="text-xs text-pilot-danger">{error}</div>}
          </div>
        </div>
      )}
    </>
  );
}
