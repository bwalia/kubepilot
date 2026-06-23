/**
 * useNamespaceLock — reads a namespace from the URL query string and, when
 * present, "locks" the entire dashboard to that single namespace.
 *
 * This is a lightweight demo-grade scoping mechanism: pass ?namespace=foo
 * (or the ?ns=foo shorthand) and every namespace-aware surface pins to that
 * namespace and hides its namespace switcher. It is NOT an authorization
 * boundary — the backend still serves any namespace it can see. Real
 * per-namespace access control (AD groups, OAuth, SSO) will replace this later.
 *
 * We parse window.location.search directly rather than next/router's query,
 * because the dashboard is built with `output: "export"` (static export) where
 * router.query is not populated until after hydration.
 */
import { useEffect, useState } from "react";

const NAMESPACE_PARAMS = ["namespace", "ns"] as const;

export interface NamespaceLock {
  /** True when the URL pins the UI to a single namespace. */
  locked: boolean;
  /** The locked namespace, or null when not locked. */
  namespace: string | null;
}

function readLockedNamespace(): string | null {
  if (typeof window === "undefined") {
    return null;
  }
  const params = new URLSearchParams(window.location.search);
  for (const key of NAMESPACE_PARAMS) {
    const value = params.get(key);
    if (value !== null) {
      const trimmed = value.trim();
      if (trimmed !== "") {
        return trimmed;
      }
    }
  }
  return null;
}

export function useNamespaceLock(): NamespaceLock {
  // Start unlocked so server and first client render agree (no hydration
  // mismatch); resolve the real value once mounted on the client.
  const [namespace, setNamespace] = useState<string | null>(null);

  useEffect(() => {
    setNamespace(readLockedNamespace());
  }, []);

  return { locked: namespace !== null, namespace };
}
