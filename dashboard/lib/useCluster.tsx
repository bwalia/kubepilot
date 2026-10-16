/**
 * The cluster this tab is looking at, as React state.
 *
 * Switching drops every cached query — all of it describes the cluster we are
 * leaving — and bumps `epoch`, which _app uses as a key to remount the page so
 * no component keeps local state (a selected pod, an open drawer) that points
 * into the old cluster.
 */
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { getSelectedCluster, onClusterChange, setSelectedCluster } from "@/lib/clusterSelection";

interface ClusterScope {
  /** Selected cluster ID; "" means the server default. */
  clusterId: string;
  setClusterId: (id: string) => void;
  /** Increments on every switch; key page content on it to reset state. */
  epoch: number;
}

const Ctx = createContext<ClusterScope | null>(null);

export function ClusterProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  // Start empty so the static export and first client render agree.
  const [clusterId, setState] = useState("");
  const [epoch, setEpoch] = useState(0);

  useEffect(() => {
    // Hydrating is not a switch: requests already carried the stored ID.
    setState(getSelectedCluster());
    return onClusterChange((id) => {
      void queryClient.cancelQueries();
      queryClient.clear();
      setState(id);
      setEpoch((e) => e + 1);
    });
  }, [queryClient]);

  const setClusterId = useCallback((id: string) => setSelectedCluster(id), []);

  const value = useMemo(() => ({ clusterId, setClusterId, epoch }), [clusterId, setClusterId, epoch]);
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useCluster(): ClusterScope {
  const ctx = useContext(Ctx);
  if (!ctx) throw new Error("useCluster must be used inside <ClusterProvider>");
  return ctx;
}
