/**
 * A one-shot channel for "open this thing on the dashboard".
 *
 * The command palette lives in _app so it works on every page, but the thing it
 * opens — a pod drawer, a section, a namespace — is state inside the dashboard
 * page. The two are not in the same React tree branch, and the palette may fire
 * while the user is on a different route entirely.
 *
 * So a request is parked in sessionStorage and announced on an event:
 *   - already on /dashboard  → the event delivers it immediately
 *   - on another route       → the router navigates, and the dashboard picks
 *                              the parked request up on mount
 * Either way it is consumed exactly once, so a refresh does not silently
 * reopen a drawer the user already closed.
 *
 * Note this is deliberately NOT the ?namespace= URL parameter: that one *locks*
 * the whole UI to a namespace (see useNamespaceLock) and is a different feature.
 */

export type DashboardTarget =
  | { type: "pod"; namespace: string; name: string }
  | { type: "section"; section: string }
  | { type: "namespace"; namespace: string };

const KEY = "kubepilot-dashboard-target";
const EVENT = "kubepilot:dashboard-target";

/** Ask the dashboard to open something. Safe to call from any page. */
export function requestDashboardTarget(target: DashboardTarget): void {
  try {
    sessionStorage.setItem(KEY, JSON.stringify(target));
  } catch {
    /* storage unavailable — the event below still covers the same-page case */
  }
  window.dispatchEvent(new CustomEvent<DashboardTarget>(EVENT, { detail: target }));
}

/** Take the parked request, if any, and clear it. */
export function consumeDashboardTarget(): DashboardTarget | null {
  try {
    const raw = sessionStorage.getItem(KEY);
    if (!raw) return null;
    sessionStorage.removeItem(KEY);
    return JSON.parse(raw) as DashboardTarget;
  } catch {
    return null;
  }
}

/** Subscribe to requests. Returns an unsubscribe function. */
export function onDashboardTarget(handler: (target: DashboardTarget) => void): () => void {
  const listener = (e: Event) => {
    // Clear the parked copy: this listener is handling it now, so the next
    // mount must not replay it.
    try {
      sessionStorage.removeItem(KEY);
    } catch {
      /* ignore */
    }
    handler((e as CustomEvent<DashboardTarget>).detail);
  };
  window.addEventListener(EVENT, listener);
  return () => window.removeEventListener(EVENT, listener);
}
