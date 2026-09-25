/**
 * Shared react-query keys.
 *
 * These exist because the same endpoint was being fetched under several
 * different keys — the pod list lived at ["ov-pods", ns] on the Overview,
 * ["dash-pods", ns] in Workloads and ["dash-pods", ""] in the command palette.
 * react-query caches per key, so identical data was fetched, stored and polled
 * two or three times over. On a 771-pod cluster that is the difference between
 * 123KB a poll and 370KB a poll.
 *
 * One key per (resource, namespace) means every surface shares one cache entry
 * and one request.
 */
export const qk = {
  namespaces: () => ["namespaces"] as const,
  nodes: () => ["nodes"] as const,
  serverConfig: () => ["server-config"] as const,

  pods: (namespace: string) => ["pods", namespace] as const,
  crashingPods: (namespace: string) => ["crashing-pods", namespace] as const,
  deployments: (namespace: string) => ["deployments", namespace] as const,
  statefulSets: (namespace: string) => ["statefulsets", namespace] as const,
  daemonSets: (namespace: string) => ["daemonsets", namespace] as const,
  jobs: (namespace: string) => ["k8s-jobs", namespace] as const,
  cronJobs: (namespace: string) => ["cronjobs", namespace] as const,
  services: (namespace: string) => ["services", namespace] as const,
  ingresses: (namespace: string) => ["ingresses", namespace] as const,
  configMaps: (namespace: string) => ["configmaps", namespace] as const,
  secrets: (namespace: string) => ["secrets", namespace] as const,
  pvcs: (namespace: string) => ["pvcs", namespace] as const,
  storageClasses: () => ["storageclasses"] as const,
  health: (namespace: string) => ["troubleshooting-summary", namespace] as const,
};
