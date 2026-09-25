/**
 * The cluster-wide namespace scope — one choice, honoured by every page.
 *
 * Before this, five surfaces each kept their own namespace state: the home
 * page, the dashboard, cluster events, the topology canvas and RCA. Picking
 * "prod" on one did nothing to the others, and the home page additionally
 * refused to show anything until you picked a namespace *again* on that page.
 *
 * Now the picker lives once in the top bar, the choice persists for the
 * session, and "" means all namespaces — an explicit, allowed option rather
 * than "nothing chosen yet".
 *
 * A ?namespace= URL parameter still overrides and locks it (see
 * useNamespaceLock); that is a scoping mechanism for sharing links, not an
 * authorization boundary.
 */
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { useNamespaceLock } from "@/lib/useNamespaceLock";

const STORAGE_KEY = "kubepilot-namespace";

interface NamespaceScope {
  /** The active namespace. "" means every namespace. */
  namespace: string;
  setNamespace: (ns: string) => void;
  /** True when a URL parameter pins the scope and the picker is read-only. */
  locked: boolean;
}

const Ctx = createContext<NamespaceScope | null>(null);

export function NamespaceProvider({ children }: { children: ReactNode }) {
  const { locked, namespace: lockedNamespace } = useNamespaceLock();
  // Start empty so server and first client render agree; hydrate after mount.
  const [stored, setStored] = useState("");

  useEffect(() => {
    try {
      const saved = sessionStorage.getItem(STORAGE_KEY);
      if (saved !== null) setStored(saved);
    } catch {
      /* storage unavailable — an in-memory scope still works for this tab */
    }
  }, []);

  const setNamespace = useCallback((ns: string) => {
    setStored(ns);
    try {
      sessionStorage.setItem(STORAGE_KEY, ns);
    } catch {
      /* ignore */
    }
  }, []);

  const value = useMemo<NamespaceScope>(
    () => ({
      namespace: locked ? lockedNamespace! : stored,
      setNamespace,
      locked,
    }),
    [locked, lockedNamespace, stored, setNamespace]
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useNamespace(): NamespaceScope {
  const ctx = useContext(Ctx);
  if (!ctx) throw new Error("useNamespace must be used inside <NamespaceProvider>");
  return ctx;
}
