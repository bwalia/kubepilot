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


// ─────────────────────────────────────────────────────────────────────────────
// Guided debugging
// ─────────────────────────────────────────────────────────────────────────────

/** Where in the pod drawer a step is carried out. */
export type DebugTab = "logs" | "events" | "containers" | "yaml";

export interface DebugStep {
  /** The action, as an instruction: "Read what it printed before it died". */
  title: string;
  /** What to look for, and how to read what you find. */
  detail: string;
  /** Tab that answers this step — the UI turns it into a jump button. */
  tab?: DebugTab;
  /**
   * The kubectl equivalent. Shown so an engineer who prefers the terminal can
   * copy it, and so someone learning can see what the UI is doing for them.
   * `{ns}` and `{pod}` are substituted by the renderer.
   */
  command?: string;
}

const LOGS_STEP: DebugStep = {
  title: "Read what it printed before it stopped",
  detail:
    "Applications almost always log the reason on their way down — a missing environment variable, a database it could not reach, a port already in use. This is the single most useful thing to look at.",
  tab: "logs",
  command: "kubectl logs {pod} -n {ns} --previous --tail=100",
};

const EVENTS_STEP: DebugStep = {
  title: "Check what Kubernetes itself reported",
  detail:
    "Events are Kubernetes' own account of what it tried and what happened — failed probes, missing images, no room to schedule.",
  tab: "events",
  command: "kubectl describe pod {pod} -n {ns}",
};

/**
 * An ordered checklist for a failing pod, most-likely-to-explain-it first.
 *
 * These are the steps an engineer actually performs, written so that someone
 * who has never run kubectl can follow them. They are keyed off the same
 * reason/phase the status pill uses, so the guide always matches the badge.
 *
 * Deliberately deterministic: this works with no AI configured, on a
 * disconnected cluster, and it never invents a cause. The AI analysis is a
 * separate, optional layer on top.
 */
export function debugSteps(pod: {
  Phase?: string;
  Reason?: string;
  Ready?: boolean;
  Restarts?: number;
}): DebugStep[] {
  const reason = pod.Reason?.trim() ?? "";
  const phase = pod.Phase?.trim() ?? "";

  if (reason === "CrashLoopBackOff" || reason === "Error" || phase === "Failed") {
    return [
      LOGS_STEP,
      {
        title: "Look at the exit code",
        detail:
          "The code the container exited with narrows it down fast. 137 means it was killed for using too much memory; 1 or 2 is usually the application failing on its own; 0 with restarts usually means a health check is killing a process that thinks it is fine.",
        tab: "containers",
      },
      EVENTS_STEP,
      {
        title: "Check the settings it starts with",
        detail:
          "A container that dies instantly often cannot find a ConfigMap, Secret or environment variable it expects. Check the names it references actually exist in this namespace.",
        tab: "yaml",
      },
    ];
  }

  if (reason === "ImagePullBackOff" || reason === "ErrImagePull" || reason === "InvalidImageName") {
    return [
      {
        title: "Check the image name and tag, character by character",
        detail:
          "Most pull failures are a typo, or a tag that was never pushed. Compare what is requested against what exists in the registry.",
        tab: "yaml",
        command: "kubectl get pod {pod} -n {ns} -o jsonpath='{.spec.containers[*].image}'",
      },
      {
        title: "Read the registry's actual refusal",
        detail:
          "Events carry the registry's own words. \"not found\" means the image or tag does not exist; \"unauthorized\" or \"denied\" means this cluster has no credentials for it.",
        tab: "events",
        command: "kubectl describe pod {pod} -n {ns}",
      },
      {
        title: "If the registry is private, check the pull secret",
        detail:
          "A private image needs an imagePullSecret on the pod or its service account, and that secret has to live in this same namespace.",
        tab: "yaml",
        command: "kubectl get secrets -n {ns}",
      },
    ];
  }

  if (reason === "OOMKilled") {
    return [
      {
        title: "Find the current memory limit",
        detail:
          "The container was stopped for exceeding its allowed memory. Start by seeing what that allowance actually is.",
        tab: "containers",
      },
      LOGS_STEP,
      {
        title: "Decide: raise the limit, or fix the usage",
        detail:
          "If the limit was simply set too low for the workload, raise it. If memory climbs steadily until it dies, the application is leaking and a higher limit only delays the crash.",
        tab: "yaml",
      },
      {
        title: "Check the machine has the memory to give",
        detail:
          "Raising a limit only helps if the node has headroom. Cluster Health shows memory per machine.",
      },
    ];
  }

  if (reason === "CreateContainerConfigError" || reason === "CreateContainerError") {
    return [
      {
        title: "Find which ConfigMap or Secret is missing",
        detail:
          "Events name the exact object that could not be found. This is nearly always a typo or something that lives in a different namespace.",
        tab: "events",
        command: "kubectl describe pod {pod} -n {ns}",
      },
      {
        title: "Confirm it exists here",
        detail:
          "Secrets and ConfigMaps are per-namespace — one in 'default' is invisible to a pod in 'prod'. Check under Config & Storage.",
        command: "kubectl get configmap,secret -n {ns}",
      },
      { title: "Check the keys inside it", detail: "The object can exist while the specific key the container asks for does not.", tab: "yaml" },
    ];
  }

  if (reason === "Evicted") {
    return [
      {
        title: "Find what the machine ran out of",
        detail: "Eviction is the node protecting itself — usually memory or disk. The event says which.",
        tab: "events",
      },
      {
        title: "Check that machine under Cluster Health",
        detail: "If one node keeps evicting pods it is under-provisioned or has something else consuming it.",
      },
      {
        title: "Give this pod a resource request",
        detail:
          "Pods with no memory request are evicted first. Setting a request makes it a harder target and helps the scheduler place it somewhere with room.",
        tab: "yaml",
      },
    ];
  }

  if (phase === "Pending" || reason === "Unschedulable" || reason === "NodeAffinity") {
    return [
      {
        title: "Read why it could not be placed",
        detail:
          "The scheduler explains its refusal per node: not enough CPU or memory, a taint it does not tolerate, or no node matching its selector.",
        tab: "events",
        command: "kubectl describe pod {pod} -n {ns}",
      },
      {
        title: "Check there is room anywhere",
        detail: "Cluster Health shows free CPU and memory per machine. If everything is full, this pod has nowhere to go.",
      },
      {
        title: "Check its placement rules are satisfiable",
        detail:
          "A nodeSelector or affinity rule referencing a label no node carries will keep a pod Pending forever, on an otherwise empty cluster.",
        tab: "yaml",
      },
      {
        title: "If it needs storage, check the disk was created",
        detail: "A pod waiting on a storage claim that is not yet Bound stays Pending. Config & Storage shows claim status.",
        command: "kubectl get pvc -n {ns}",
      },
    ];
  }

  if (phase === "Running" && pod.Ready === false) {
    return [
      {
        title: "See which health check is failing",
        detail:
          "The container is up but its readiness probe says no, so it receives no traffic. Events give the probe's actual response — a connection refused, a 404, a timeout.",
        tab: "events",
      },
      {
        title: "Check whether it is simply still starting",
        detail:
          "Slow-booting applications are not-ready for a while by design. If the logs show normal startup progress, it may just need a longer initialDelaySeconds.",
        tab: "logs",
      },
      {
        title: "Verify the probe points at the right place",
        detail:
          "A readiness probe on the wrong port or path fails against a perfectly healthy application. Compare the probe to what the app actually serves.",
        tab: "yaml",
      },
    ];
  }

  if ((pod.Restarts ?? 0) > 5) {
    return [
      {
        title: "Look at what it logged before the last restart",
        detail: "It is running now, but something has repeatedly killed it. The previous container's logs hold the reason.",
        tab: "logs",
        command: "kubectl logs {pod} -n {ns} --previous",
      },
      {
        title: "Check whether a liveness probe is doing the killing",
        detail:
          "A liveness probe that is too aggressive restarts a healthy-but-busy application in a loop. Events show each restart and its trigger.",
        tab: "events",
      },
      { title: "Check for memory pressure", detail: "Repeated restarts with exit code 137 mean it is being killed for memory, not crashing.", tab: "containers" },
    ];
  }

  // Healthy, or a state with no specific playbook — still give the two
  // universally useful moves rather than an empty panel.
  return [LOGS_STEP, EVENTS_STEP];
}
