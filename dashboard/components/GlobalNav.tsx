/**
 * GlobalNav — slim top-level navigation shared across all pages.
 * Purely additive: it renders above each page's own content and does not
 * replace the existing AI Troubleshooting page header.
 */
import Link from "next/link";
import { useRouter } from "next/router";
import { Layers, Lock } from "lucide-react";
import { useNamespaceLock } from "@/lib/useNamespaceLock";

const NAV_LINKS = [
  { href: "/", label: "Kubernetes CoPilot" },
  { href: "/dashboard", label: "Kubernetes Dashboard" },
  { href: "/autopilot", label: "Kubernetes AutoPilot" },
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
    <div className="sticky top-0 z-[60] bg-pilot-bg/95 backdrop-blur border-b border-pilot-border">
      <div className="flex items-center gap-3 px-4 sm:px-6 lg:px-8 h-11">
        <div className="flex items-center gap-2 mr-2">
          <div className="flex items-center justify-center w-6 h-6 rounded bg-pilot-accent/10">
            <Layers className="text-pilot-accent w-3.5 h-3.5" />
          </div>
          <span className="text-sm font-bold tracking-tight hidden sm:inline">KubePilot</span>
        </div>
        <nav className="flex items-center gap-1" aria-label="Primary">
          {NAV_LINKS.map((link) => (
            <Link
              key={link.href}
              href={link.href}
              aria-current={isActive(link.href) ? "page" : undefined}
              className={`px-3 py-1.5 text-sm font-medium rounded-md transition-colors ${
                isActive(link.href)
                  ? "bg-pilot-accent/15 text-pilot-accent-light"
                  : "text-pilot-muted hover:text-white"
              }`}
            >
              {link.label}
            </Link>
          ))}
        </nav>
        <div className="ml-auto flex items-center gap-3">
          {locked && (
            <span
              className="inline-flex items-center gap-1.5 bg-pilot-accent/15 text-pilot-accent-light border border-pilot-accent/40 rounded-md px-2.5 py-1 text-xs font-medium"
              title="The dashboard is locked to this namespace via the URL. Remove the ?namespace= parameter to browse all namespaces."
            >
              <Lock className="w-3 h-3" />
              <span className="hidden sm:inline">Namespace:</span>
              <span className="font-mono">{namespace}</span>
            </span>
          )}
          <span
            className="font-mono text-[11px] text-pilot-muted/80 select-text"
            title="Dashboard build version"
          >
            {BUILD_VERSION}
          </span>
        </div>
      </div>
    </div>
  );
}
