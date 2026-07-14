/**
 * GlobalNav — the flight-deck top rail shared across all pages.
 * Purely additive: it renders above each page's own content and does not
 * replace the existing AI Troubleshooting page header.
 */
import Link from "next/link";
import { useRouter } from "next/router";
import { Navigation, Lock } from "lucide-react";
import { useNamespaceLock } from "@/lib/useNamespaceLock";

const NAV_LINKS = [
  { href: "/", label: "CoPilot" },
  { href: "/dashboard", label: "Dashboard" },
  { href: "/autopilot", label: "AutoPilot" },
];

// Stamped at build time by `make dashboard` (NEXT_PUBLIC_BUILD_VERSION).
// Falls back to "dev" for local `next dev` runs that don't set it.
const BUILD_VERSION = process.env.NEXT_PUBLIC_BUILD_VERSION || "dev";

export function GlobalNav() {
  const { pathname } = useRouter();
  const { locked, namespace } = useNamespaceLock();

  const isActive = (href: string) =>
    href === "/" ? pathname === "/" : pathname.startsWith(href);

  return (
    <div className="sticky top-0 z-[60] bg-pilot-bg/85 backdrop-blur-md border-b border-pilot-border">
      <div className="flex items-center gap-4 px-4 sm:px-6 lg:px-8 h-12">
        {/* Brand mark */}
        <Link href="/" className="flex items-center gap-2.5 mr-1 group" aria-label="KubePilot home">
          <div className="flex items-center justify-center w-7 h-7 rounded-lg bg-pilot-accent/12 border border-pilot-accent/25 group-hover:border-pilot-accent/50 transition-colors">
            <Navigation className="text-pilot-accent w-4 h-4 -rotate-45" />
          </div>
          <span className="font-display text-[15px] font-bold tracking-tight text-pilot-text-primary hidden sm:inline">
            Kube<span className="text-pilot-accent">Pilot</span>
          </span>
        </Link>

        {/* Primary nav */}
        <nav className="flex items-center gap-0.5" aria-label="Primary">
          {NAV_LINKS.map((link) => {
            const active = isActive(link.href);
            return (
              <Link
                key={link.href}
                href={link.href}
                aria-current={active ? "page" : undefined}
                className={`relative px-3 py-1.5 text-[13px] font-medium rounded-md transition-colors ${
                  active
                    ? "text-pilot-text-primary bg-pilot-accent/10"
                    : "text-pilot-muted hover:text-pilot-text-primary hover:bg-white/[0.03]"
                }`}
              >
                {link.label}
                {active && (
                  <span className="absolute inset-x-3 -bottom-[7px] h-0.5 rounded-full bg-pilot-accent shadow-[0_0_8px_rgba(34,211,238,0.7)]" />
                )}
              </Link>
            );
          })}
        </nav>

        <div className="ml-auto flex items-center gap-3">
          {locked && (
            <span
              className="inline-flex items-center gap-1.5 bg-pilot-accent/12 text-pilot-accent-light border border-pilot-accent/35 rounded-md px-2.5 py-1 text-xs font-medium"
              title="The dashboard is locked to this namespace via the URL. Remove the ?namespace= parameter to browse all namespaces."
            >
              <Lock className="w-3 h-3" />
              <span className="hidden sm:inline text-pilot-muted">ns</span>
              <span className="font-mono">{namespace}</span>
            </span>
          )}
          {/* Live telemetry indicator */}
          <span className="hidden sm:inline-flex items-center gap-1.5 text-2xs font-medium text-pilot-muted">
            <span className="relative flex h-1.5 w-1.5">
              <span className="absolute inline-flex h-full w-full rounded-full bg-pilot-success animate-pulse-dot" />
              <span className="relative inline-flex h-1.5 w-1.5 rounded-full bg-pilot-success" />
            </span>
            <span className="eyebrow !text-[10px]">Live</span>
          </span>
          <span
            className="font-mono text-[11px] text-pilot-muted/70 select-text"
            title="Dashboard build version"
          >
            {BUILD_VERSION}
          </span>
        </div>
      </div>
    </div>
  );
}
