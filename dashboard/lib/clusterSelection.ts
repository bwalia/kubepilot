/**
 * Which cluster this browser tab is looking at.
 *
 * The choice is per tab (sessionStorage), so two tabs — or two people sharing
 * a KubePilot server — can watch different clusters at once. A new tab starts
 * on the cluster last picked anywhere (localStorage), and "" means the
 * server's default cluster.
 *
 * Kept outside React so the axios client can read it synchronously on every
 * request, including the very first ones fired before any provider mounts.
 */

export const CLUSTER_HEADER = "X-KubePilot-Cluster";

const TAB_KEY = "kubepilot-cluster";
const LAST_KEY = "kubepilot-cluster-last";

function readStored(): string {
  if (typeof window === "undefined") return "";
  try {
    const tab = sessionStorage.getItem(TAB_KEY);
    if (tab !== null) return tab;
    return localStorage.getItem(LAST_KEY) ?? "";
  } catch {
    return "";
  }
}

let active = readStored();
const listeners = new Set<(id: string) => void>();

export function getSelectedCluster(): string {
  return active;
}

export function setSelectedCluster(id: string) {
  if (id === active) return;
  active = id;
  try {
    sessionStorage.setItem(TAB_KEY, id);
    localStorage.setItem(LAST_KEY, id);
  } catch {
    /* storage unavailable — the in-memory choice still holds for this tab */
  }
  listeners.forEach((fn) => fn(id));
}

export function onClusterChange(fn: (id: string) => void): () => void {
  listeners.add(fn);
  return () => {
    listeners.delete(fn);
  };
}
