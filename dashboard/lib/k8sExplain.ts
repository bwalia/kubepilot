/**
 * Plain-English translations of Kubernetes states.
 *
 * The dashboard is read by people who do not run `kubectl` — a client checking
 * whether their app is up, or a developer who does not carry cluster jargon in
 * their head. "CrashLoopBackOff" tells them nothing; "Keeps crashing and
 * restarting" plus "check the Logs tab for the error" tells them what is wrong
 * and what to do next.
 *
 * Every entry is a fact about Kubernetes behaviour, not a guess about this
 * cluster, so it is safe to state plainly. Where the cause is genuinely
 * ambiguous the wording says so rather than inventing a diagnosis.
 */

/** Semantic severity, mapped to the pilot-* status colours by the UI. */
export type Tone = "ok" | "warn" | "bad" | "info" | "idle";

export interface Explanation {
  tone: Tone;
  /** Short human label shown in the pill, e.g. "Crash looping". */
  label: string;
  /** One sentence a non-engineer can act on. */
  plain: string;
  /** What to do next, when there is a clear next step. */
  advice?: string;
}

/**
 * Reasons Kubernetes reports on a container/pod, keyed exactly as the API
 * spells them. Anything not listed falls back to the phase.
 */
const REASONS: Record<string, Explanation> = {
  CrashLoopBackOff: {
    tone: "bad",
    label: "Crash looping",
    plain: "The app starts, crashes, and Kubernetes keeps restarting it.",
    advice: "Open Logs to see the error it prints just before it dies.",
  },
  ImagePullBackOff: {
    tone: "bad",
    label: "Can't get image",
    plain: "Kubernetes cannot download the container image for this app.",
    advice: "Check the image name and tag, and whether the registry needs credentials.",
  },
  ErrImagePull: {
    tone: "bad",
    label: "Image pull failed",
    plain: "The container image could not be downloaded.",
    advice: "Check the image name and tag, and whether the registry needs credentials.",
  },
  InvalidImageName: {
    tone: "bad",
    label: "Bad image name",
    plain: "The container image name is not valid, so nothing can be downloaded.",
    advice: "Fix the image reference in the deployment.",
  },
  OOMKilled: {
    tone: "bad",
    label: "Out of memory",
    plain: "The app used more memory than it was allowed and was stopped.",
    advice: "Raise the memory limit, or find what is using the memory.",
  },
  Evicted: {
    tone: "bad",
    label: "Evicted",
    plain: "The node ran out of resources and removed this pod to protect itself.",
    advice: "Check the node's memory and disk under Cluster Health.",
  },
  CreateContainerConfigError: {
    tone: "bad",
    label: "Bad configuration",
    plain: "A ConfigMap or Secret the app needs is missing or misspelled.",
    advice: "Check the names referenced under Config & Storage.",
  },
  CreateContainerError: {
    tone: "bad",
    label: "Won't start",
    plain: "The container could not be created on the node.",
    advice: "Open Events for the exact reason the node gave.",
  },
  RunContainerError: {
    tone: "bad",
    label: "Won't run",
    plain: "The container was created but failed immediately on start.",
    advice: "Open Logs and Events for the startup error.",
  },
  ContainerCreating: {
    tone: "info",
    label: "Starting",
    plain: "Kubernetes is setting the app up right now.",
    advice: "Normal for a few seconds. If it lasts minutes, check Events.",
  },
  PodInitializing: {
    tone: "info",
    label: "Initialising",
    plain: "Setup steps are running before the app itself starts.",
  },
  Unschedulable: {
    tone: "warn",
    label: "Nowhere to run",
    plain: "No node has enough free capacity, or matches this pod's placement rules.",
    advice: "Check node capacity under Cluster Health.",
  },
  NodeAffinity: {
    tone: "warn",
    label: "No matching node",
    plain: "This pod is restricted to nodes that do not currently match.",
    advice: "Check the pod's node selector against the node labels.",
  },
  Completed: {
    tone: "idle",
    label: "Finished",
    plain: "This ran to completion and exited normally. Nothing is wrong.",
  },
  Error: {
    tone: "bad",
    label: "Exited with error",
    plain: "The app stopped and reported a failure.",
    advice: "Open Logs for the error it printed.",
  },
  ContainerStatusUnknown: {
    tone: "warn",
    label: "Status unknown",
    plain: "Kubernetes lost contact with the node running this pod.",
    advice: "Check whether that node is still Ready under Cluster Health.",
  },
  Terminating: {
    tone: "info",
    label: "Shutting down",
    plain: "This pod is being stopped.",
  },
};

/** Pod phases — the coarse state, used when there is no more specific reason. */
const PHASES: Record<string, Explanation> = {
  Running: { tone: "ok", label: "Running", plain: "The app is running normally." },
  Pending: {
    tone: "warn",
    label: "Waiting to start",
    plain: "Kubernetes has accepted this pod but it is not running yet.",
    advice: "Usually capacity or image download. Open Events for the reason.",
  },
  Succeeded: { tone: "idle", label: "Finished", plain: "This ran to completion and exited normally." },
  Failed: {
    tone: "bad",
    label: "Failed",
    plain: "The app stopped and did not succeed.",
    advice: "Open Logs for the error it printed before stopping.",
  },
  Unknown: {
    tone: "warn",
    label: "Status unknown",
    plain: "Kubernetes cannot reach the node running this pod.",
    advice: "Check whether that node is still Ready under Cluster Health.",
  },
};

/**
 * Best available explanation for a pod, preferring the specific container
 * reason over the coarse phase.
 *
 * `ready` matters: a pod can be phase Running with no failure reason while its
 * readiness probe is still failing, which means it is up but not yet taking
 * traffic. That distinction is invisible in the phase alone and is a common
 * source of "it says Running but the site is down".
 */
export function explainPod(pod: {
  Phase?: string;
  Reason?: string;
  Ready?: boolean;
  Restarts?: number;
}): Explanation {
  const reason = pod.Reason?.trim();
  if (reason && REASONS[reason]) return REASONS[reason];
  if (reason) {
    // An unmapped reason is still better shown than hidden — say plainly that
    // it is Kubernetes' own wording rather than inventing a meaning for it.
    return {
      tone: "warn",
      label: reason,
      plain: `Kubernetes reported "${reason}" for this pod.`,
      advice: "Open Events and Logs for the detail behind it.",
    };
  }

  const phase = pod.Phase?.trim() || "Unknown";
  const base = PHASES[phase] ?? {
    tone: "info" as Tone,
    label: phase,
    plain: `Kubernetes reports this pod as ${phase}.`,
  };

  if (phase === "Running" && pod.Ready === false) {
    return {
      tone: "warn",
      label: "Not ready",
      plain: "The app is up but is not passing its health check, so it receives no traffic.",
      advice: "Open Logs — it is usually still starting, or a dependency is unreachable.",
    };
  }
  if (phase === "Running" && (pod.Restarts ?? 0) > 5) {
    return {
      tone: "warn",
      label: "Running · restarted often",
      plain: `Running now, but it has restarted ${pod.Restarts} times — something is making it fail repeatedly.`,
      advice: "Open Logs and look at what it printed before the last restart.",
    };
  }
  return base;
}

/** Node readiness and pressure, in the same plain terms. */
export function explainNode(node: {
  Ready?: boolean;
  MemoryPressure?: boolean;
  DiskPressure?: boolean;
  PIDPressure?: boolean;
  Unschedulable?: boolean;
}): Explanation {
  if (!node.Ready) {
    return {
      tone: "bad",
      label: "Offline",
      plain: "This machine is not responding to the cluster.",
      advice: "Anything that was running on it has been moved or is stuck.",
    };
  }
  if (node.MemoryPressure) {
    return {
      tone: "bad",
      label: "Low memory",
      plain: "This machine is running out of memory and may start stopping pods.",
    };
  }
  if (node.DiskPressure) {
    return {
      tone: "bad",
      label: "Low disk",
      plain: "This machine is running out of disk space.",
    };
  }
  if (node.PIDPressure) {
    return { tone: "warn", label: "Too many processes", plain: "This machine is near its process limit." };
  }
  if (node.Unschedulable) {
    return {
      tone: "warn",
      label: "Cordoned",
      plain: "This machine is healthy but has been marked to accept no new pods.",
    };
  }
  return { tone: "ok", label: "Healthy", plain: "This machine is online and accepting work." };
}

/**
 * What each Kubernetes resource kind actually is, for the one-line description
 * shown above every list. Keyed by the lowercase plural used in the UI tabs.
 */
export const KIND_HELP: Record<string, { title: string; plain: string }> = {
  pods: {
    title: "Pods",
    plain: "One running copy of an app. If something is broken, this is where you see it first.",
  },
  deployments: {
    title: "Deployments",
    plain: "The instruction for how many copies of an app should run. Kubernetes keeps that many alive.",
  },
  statefulsets: {
    title: "StatefulSets",
    plain: "Like a Deployment, but for apps that keep data — databases and queues. Each copy keeps its own storage and a stable name.",
  },
  daemonsets: {
    title: "DaemonSets",
    plain: "An app that runs one copy on every machine — usually logging, monitoring or networking.",
  },
  jobs: {
    title: "Jobs",
    plain: "A one-off task that runs until it finishes, then stops. Finishing is success, not a fault.",
  },
  cronjobs: {
    title: "CronJobs",
    plain: "A task on a schedule, like a nightly backup. It creates a Job each time it runs.",
  },
  services: {
    title: "Services",
    plain: "A stable internal address for an app, so other apps can reach it even as copies come and go.",
  },
  ingresses: {
    title: "Ingress",
    plain: "The rules that map public web addresses to apps inside the cluster.",
  },
  configmaps: {
    title: "ConfigMaps",
    plain: "Settings handed to apps at startup — URLs, feature flags, tuning values. Not for passwords.",
  },
  secrets: {
    title: "Secrets",
    plain: "Passwords, API keys and certificates. Values are never shown here.",
  },
  pvcs: {
    title: "Storage Claims",
    plain: "A request for a disk that survives restarts. Apps that keep data use these.",
  },
  storageclasses: {
    title: "Storage Classes",
    plain: "The kinds of disk this cluster can create on demand.",
  },
  nodes: {
    title: "Nodes",
    plain: "The physical or virtual machines your apps actually run on.",
  },
  namespaces: {
    title: "Namespaces",
    plain: "Folders that separate one team, app or environment from another.",
  },
  events: {
    title: "Events",
    plain: "Kubernetes' own running commentary on what it just did and why. The first place to look when something changed.",
  },
};

/** Tailwind classes per tone, so every status surface agrees on colour. */
export const TONE_CLASS: Record<Tone, { pill: string; dot: string; text: string }> = {
  ok: {
    pill: "border-pilot-success/30 bg-pilot-success/12 text-pilot-success",
    dot: "bg-pilot-success",
    text: "text-pilot-success",
  },
  warn: {
    pill: "border-pilot-warning/30 bg-pilot-warning/12 text-pilot-warning",
    dot: "bg-pilot-warning",
    text: "text-pilot-warning",
  },
  bad: {
    pill: "border-pilot-danger/30 bg-pilot-danger/12 text-pilot-danger",
    dot: "bg-pilot-danger",
    text: "text-pilot-danger",
  },
  info: {
    pill: "border-pilot-accent/30 bg-pilot-accent/12 text-pilot-accent-light",
    dot: "bg-pilot-accent",
    text: "text-pilot-accent-light",
  },
  idle: {
    pill: "border-pilot-border bg-pilot-surface-2 text-pilot-muted",
    dot: "bg-pilot-muted",
    text: "text-pilot-muted",
  },
};
