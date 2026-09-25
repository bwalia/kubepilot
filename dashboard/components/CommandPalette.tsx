/**
 * CommandPalette — one search box for the whole cluster, on Cmd/Ctrl-K.
 *
 * The problem it solves: finding anything used to mean knowing which page it
 * was on, picking its namespace from a long dropdown, then filtering a table.
 * Three steps, each needing you to already know where the thing lives. Here you
 * type part of a name and press Enter, from any page.
 *
 * It searches pods, nodes, namespaces and deployments across ALL namespaces at
 * once — deliberately ignoring the current namespace filter, because the whole
 * point is finding something when you do not know where it is.
 *
 * Results are ranked by fuzzy score, and broken things float to the top: if a
 * pod is crash looping it is almost certainly what you opened this to find.
 */
import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/router";
import { useQuery } from "@tanstack/react-query";
import { qk } from "@/lib/queryKeys";
import {
  Search,
  Box,
  Server,
  Layers,
  Layout,
  CornerDownLeft,
  ArrowUp,
  ArrowDown,
  Loader2,
} from "lucide-react";
import { listPods, listNodes, listNamespaces, listDeployments } from "@/lib/api";
import { fuzzyMatch, highlightParts } from "@/lib/fuzzy";
import { explainPod, explainNode, TONE_CLASS, type Tone } from "@/lib/k8sExplain";
import { requestDashboardTarget } from "@/lib/dashboardNav";
import { Dialog } from "@/components/ui/dialog";
import * as DialogPrimitive from "@radix-ui/react-dialog";
import { cn } from "@/lib/utils";

type ResultKind = "pod" | "node" | "namespace" | "deployment" | "page";

interface Result {
  kind: ResultKind;
  id: string;
  /** What the user typed against. */
  title: string;
  /** Namespace, node name, or a short description. */
  subtitle?: string;
  tone?: Tone;
  /** Plain-English status, shown on the right. */
  status?: string;
  run: () => void;
}

const KIND_META: Record<ResultKind, { icon: typeof Box; label: string }> = {
  pod: { icon: Box, label: "Pods" },
  deployment: { icon: Layers, label: "Deployments" },
  node: { icon: Server, label: "Machines" },
  namespace: { icon: Layers, label: "Namespaces" },
  page: { icon: Layout, label: "Go to" },
};

/** Tie-break order when two groups score equally. */
const GROUP_ORDER: ResultKind[] = ["page", "namespace", "pod", "deployment", "node"];

/**
 * Most a single group may contribute. Without this, a cluster with 768 pods
 * fills every slot and the namespace or node you were actually looking for is
 * never shown — the failure mode of a flat "top N" over a lopsided dataset.
 */
const PER_GROUP_LIMIT = 6;

/**
 * A hit on the subtitle (a pod's namespace) is worth much less than a hit on
 * the name itself. Searching "kube-system" should offer the NAMESPACE first,
 * not the 200 pods that happen to live in it.
 */
const SUBTITLE_PENALTY = 0.35;

/** How much a broken thing is boosted up the list. Tuned so a crash-looping
 *  pod outranks a healthy pod with a slightly better name match, but never
 *  outranks an exact name hit on a healthy one. */
const TONE_BOOST: Record<Tone, number> = { bad: 220, warn: 90, ok: 0, info: 0, idle: 0 };

export function CommandPalette() {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [cursor, setCursor] = useState(0);
  const listRef = useRef<HTMLDivElement>(null);

  // Cmd/Ctrl-K from anywhere, plus "/" when not already typing in a field.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const typingInField =
        e.target instanceof HTMLElement &&
        (e.target.tagName === "INPUT" ||
          e.target.tagName === "TEXTAREA" ||
          e.target.isContentEditable);

      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen((o) => !o);
      } else if (e.key === "/" && !typingInField && !e.metaKey && !e.ctrlKey) {
        e.preventDefault();
        setOpen(true);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  // Also open on a click of the nav search button (it dispatches this event),
  // so the feature is discoverable without knowing the shortcut.
  useEffect(() => {
    const onOpen = () => setOpen(true);
    window.addEventListener("kubepilot:open-palette", onOpen);
    return () => window.removeEventListener("kubepilot:open-palette", onOpen);
  }, []);

  // Cluster data is only fetched while the palette is open, and is shared with
  // the rest of the app through the react-query cache (no duplicate requests).
  const enabled = open;
  const pods = useQuery({ queryKey: qk.pods(""), queryFn: () => listPods(""), enabled });
  const nodes = useQuery({ queryKey: qk.nodes(), queryFn: listNodes, enabled });
  const namespaces = useQuery({ queryKey: qk.namespaces(), queryFn: listNamespaces, enabled });
  const deployments = useQuery({
    queryKey: qk.deployments(""),
    queryFn: () => listDeployments(""),
    enabled,
  });
  const loading =
    pods.isLoading || nodes.isLoading || namespaces.isLoading || deployments.isLoading;

  const go = (target: Parameters<typeof requestDashboardTarget>[0]) => {
    requestDashboardTarget(target);
    if (router.pathname !== "/dashboard") void router.push("/dashboard");
    setOpen(false);
  };

  const results = useMemo<Result[]>(() => {
    const out: Result[] = [];

    const PAGES: { title: string; subtitle: string; run: () => void }[] = [
      { title: "Overview", subtitle: "Cluster at a glance", run: () => go({ type: "section", section: "overview" }) },
      { title: "Workloads", subtitle: "Pods, Deployments, Jobs", run: () => go({ type: "section", section: "workloads" }) },
      { title: "Network", subtitle: "Services and Ingress", run: () => go({ type: "section", section: "network" }) },
      { title: "Config & Storage", subtitle: "ConfigMaps, Secrets, Disks", run: () => go({ type: "section", section: "config" }) },
      { title: "Cluster Health", subtitle: "Machines and capacity", run: () => go({ type: "section", section: "health" }) },
      { title: "Events", subtitle: "What Kubernetes just did", run: () => go({ type: "section", section: "events" }) },
      { title: "Topology", subtitle: "How services talk to each other", run: () => go({ type: "section", section: "topology" }) },
      { title: "Ask AI", subtitle: "Describe a problem in plain English", run: () => { setOpen(false); void router.push("/"); } },
      { title: "AutoPilot", subtitle: "Automated remediation decisions", run: () => { setOpen(false); void router.push("/autopilot"); } },
      { title: "Telemetry", subtitle: "OpenTelemetry metrics, logs, traces", run: () => { setOpen(false); void router.push("/otel"); } },
    ];
    for (const p of PAGES) {
      out.push({ kind: "page", id: `page:${p.title}`, title: p.title, subtitle: p.subtitle, run: p.run });
    }

    for (const pod of pods.data ?? []) {
      const ex = explainPod(pod);
      out.push({
        kind: "pod",
        id: `pod:${pod.Namespace}/${pod.Name}`,
        title: pod.Name,
        subtitle: pod.Namespace,
        tone: ex.tone,
        status: ex.label,
        run: () => go({ type: "pod", namespace: pod.Namespace, name: pod.Name }),
      });
    }

    for (const d of deployments.data ?? []) {
      const healthy = d.ReadyReplicas >= d.Replicas && d.Replicas > 0;
      out.push({
        kind: "deployment",
        id: `deploy:${d.Namespace}/${d.Name}`,
        title: d.Name,
        subtitle: d.Namespace,
        tone: healthy ? "ok" : "warn",
        status: `${d.ReadyReplicas}/${d.Replicas} ready`,
        run: () => go({ type: "section", section: "workloads" }),
      });
    }

    for (const n of nodes.data ?? []) {
      const ex = explainNode(n);
      out.push({
        kind: "node",
        id: `node:${n.Name}`,
        title: n.Name,
        subtitle: n.Hardware || n.InternalIP,
        tone: ex.tone,
        status: ex.label,
        run: () => go({ type: "section", section: "health" }),
      });
    }

    for (const ns of namespaces.data ?? []) {
      out.push({
        kind: "namespace",
        id: `ns:${ns.Name}`,
        title: ns.Name,
        subtitle: "Namespace",
        run: () => go({ type: "namespace", namespace: ns.Name }),
      });
    }

    const q = query.trim();
    if (!q) {
      // Empty query: show the navigation targets plus anything currently broken,
      // which is the most useful thing to put in front of someone who just
      // opened a search box on a cluster.
      const broken = out
        .filter((r) => r.tone === "bad" || r.tone === "warn")
        .slice(0, 8);
      return [...out.filter((r) => r.kind === "page"), ...broken];
    }

    const scored = out
      .map((r) => {
        const onTitle = fuzzyMatch(q, r.title);
        const onSubtitle = !onTitle && r.subtitle ? fuzzyMatch(q, r.subtitle) : null;
        const m = onTitle ?? onSubtitle;
        if (!m) return null;
        const base = onTitle ? m.score : m.score * SUBTITLE_PENALTY;
        return { r, score: base + (r.tone ? TONE_BOOST[r.tone] ?? 0 : 0) };
      })
      .filter((x): x is { r: Result; score: number } => x !== null)
      .sort((a, b) => b.score - a.score);

    // Take the best few per kind, so every kind that matched is represented.
    const perKind = new Map<ResultKind, number>();
    return scored
      .filter(({ r }) => {
        const n = perKind.get(r.kind) ?? 0;
        if (n >= PER_GROUP_LIMIT) return false;
        perKind.set(r.kind, n + 1);
        return true;
      })
      .map((x) => x.r);
    // `go` is stable enough for this memo: it only closes over router, and a
    // route change re-renders anyway.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query, pods.data, nodes.data, namespaces.data, deployments.data, router.pathname]);

  // Group while preserving rank: a group appears where its best hit ranked.
  const grouped = useMemo(() => {
    const seen = new Map<ResultKind, Result[]>();
    const bestRank = new Map<ResultKind, number>();
    results.forEach((r, i) => {
      if (!seen.has(r.kind)) {
        seen.set(r.kind, []);
        bestRank.set(r.kind, i);
      }
      seen.get(r.kind)!.push(r);
    });
    // A group sits where its strongest hit ranked, so typing an exact namespace
    // name puts Namespaces on top even though Pods usually dominate.
    return [...seen.keys()]
      .sort(
        (a, b) =>
          (bestRank.get(a)! - bestRank.get(b)!) ||
          GROUP_ORDER.indexOf(a) - GROUP_ORDER.indexOf(b)
      )
      .map((k) => ({ kind: k, items: seen.get(k)! }));
  }, [results]);

  // Flat order for keyboard navigation must match the rendered order.
  const flat = useMemo(() => grouped.flatMap((g) => g.items), [grouped]);

  useEffect(() => setCursor(0), [query, open]);
  useEffect(() => {
    listRef.current?.querySelector<HTMLElement>(`[data-idx="${cursor}"]`)?.scrollIntoView({ block: "nearest" });
  }, [cursor]);

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setCursor((c) => (flat.length ? (c + 1) % flat.length : 0));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setCursor((c) => (flat.length ? (c - 1 + flat.length) % flat.length : 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      flat[cursor]?.run();
    }
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Overlay className="fixed inset-0 z-[80] bg-black/50 backdrop-blur-sm data-[state=open]:animate-in data-[state=open]:fade-in-0" />
        <DialogPrimitive.Content
          onKeyDown={onKeyDown}
          className="fixed left-1/2 top-[12vh] z-[80] w-[min(42rem,calc(100vw-2rem))] -translate-x-1/2 overflow-hidden rounded-2xl border border-pilot-border bg-pilot-surface shadow-card-hover focus-visible:outline-none data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95"
        >
          <DialogPrimitive.Title className="sr-only">Search the cluster</DialogPrimitive.Title>
          <DialogPrimitive.Description className="sr-only">
            Type the name of a pod, machine, namespace or page. Use the arrow keys to choose and Enter to open.
          </DialogPrimitive.Description>

          <div className="flex items-center gap-3 border-b border-pilot-border px-4 py-3.5">
            {loading ? (
              <Loader2 className="h-5 w-5 shrink-0 animate-spin text-pilot-accent" aria-hidden="true" />
            ) : (
              <Search className="h-5 w-5 shrink-0 text-pilot-muted" aria-hidden="true" />
            )}
            {/* eslint-disable-next-line jsx-a11y/no-autofocus -- a search dialog exists to be typed into */}
            <input
              autoFocus
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search pods, machines, namespaces…"
              aria-label="Search the cluster"
              className="w-full bg-transparent text-base text-pilot-text-primary placeholder:text-pilot-muted focus:outline-none"
            />
            <kbd className="hidden shrink-0 rounded border border-pilot-border bg-pilot-surface-2 px-1.5 py-0.5 font-mono text-[0.7rem] text-pilot-muted sm:inline">
              esc
            </kbd>
          </div>

          <div ref={listRef} className="max-h-[min(26rem,60vh)] overflow-y-auto py-2" role="listbox" aria-label="Results">
            {flat.length === 0 && (
              <div className="px-4 py-10 text-center">
                <p className="text-sm text-pilot-text-secondary">
                  Nothing matches &ldquo;{query}&rdquo;.
                </p>
                <p className="mt-1 text-xs text-pilot-muted">
                  Names are matched loosely — &ldquo;kbsys&rdquo; finds &ldquo;kube-system&rdquo;.
                </p>
              </div>
            )}

            {grouped.map((group) => {
              const Icon = KIND_META[group.kind].icon;
              return (
                <div key={group.kind}>
                  <div className="px-4 pb-1 pt-3 text-[0.7rem] font-semibold uppercase tracking-wider text-pilot-muted">
                    {KIND_META[group.kind].label}
                  </div>
                  {group.items.map((r) => {
                    const idx = flat.indexOf(r);
                    const active = idx === cursor;
                    return (
                      <button
                        key={r.id}
                        data-idx={idx}
                        role="option"
                        aria-selected={active}
                        onMouseEnter={() => setCursor(idx)}
                        onClick={r.run}
                        className={cn(
                          "flex w-full items-center gap-3 px-4 py-2.5 text-left transition-colors",
                          active && "bg-pilot-accent/12"
                        )}
                      >
                        <Icon
                          className={cn("h-4 w-4 shrink-0", active ? "text-pilot-accent" : "text-pilot-muted")}
                          aria-hidden="true"
                        />
                        <span className="min-w-0 flex-1">
                          <span
                            className={cn(
                              "block truncate text-sm text-pilot-text-primary",
                              r.kind !== "page" && "font-mono"
                            )}
                          >
                            {highlightParts(r.title, query).map((part, pi) =>
                              part.hit ? (
                                <mark key={pi} className="bg-transparent font-bold text-pilot-accent-light">
                                  {part.text}
                                </mark>
                              ) : (
                                <span key={pi}>{part.text}</span>
                              )
                            )}
                          </span>
                          {r.subtitle && (
                            <span className="block truncate text-xs text-pilot-muted">{r.subtitle}</span>
                          )}
                        </span>
                        {r.status && (
                          <span
                            className={cn(
                              "shrink-0 rounded-md border px-2 py-0.5 text-[0.7rem] font-semibold",
                              TONE_CLASS[r.tone ?? "idle"].pill
                            )}
                          >
                            {r.status}
                          </span>
                        )}
                      </button>
                    );
                  })}
                </div>
              );
            })}
          </div>

          <div className="flex items-center gap-4 border-t border-pilot-border bg-pilot-surface-2/60 px-4 py-2 text-[0.7rem] text-pilot-muted">
            <span className="inline-flex items-center gap-1">
              <ArrowUp className="h-3 w-3" aria-hidden="true" />
              <ArrowDown className="h-3 w-3" aria-hidden="true" />
              move
            </span>
            <span className="inline-flex items-center gap-1">
              <CornerDownLeft className="h-3 w-3" aria-hidden="true" />
              open
            </span>
            <span className="ml-auto hidden sm:inline">Searching every namespace</span>
          </div>
        </DialogPrimitive.Content>
      </DialogPrimitive.Portal>
    </Dialog>
  );
}

/** Opens the palette from anywhere (the nav button uses this). */
export function openCommandPalette() {
  window.dispatchEvent(new CustomEvent("kubepilot:open-palette"));
}
