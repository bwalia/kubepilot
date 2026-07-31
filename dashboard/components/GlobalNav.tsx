/**
 * GlobalNav — the single primary top bar shared across every page: brand,
 * product navigation (CoPilot / Pilot / AutoPilot), and the theme toggle.
 * It is the one place the product names itself, so page headers below it carry
 * only their own title + context (no duplicated brand).
 */
import Link from "next/link";
import { useRouter } from "next/router";
import { Navigation, Lock, Sparkles, LayoutDashboard, Bot } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useNamespaceLock } from "@/lib/useNamespaceLock";
import { ThemeToggle } from "@/components/ThemeToggle";

const NAV_LINKS: { href: string; label: string; icon: LucideIcon }[] = [
  { href: "/", label: "CoPilot", icon: Sparkles },
  { href: "/dashboard", label: "Pilot", icon: LayoutDashboard },
  { href: "/autopilot", label: "AutoPilot", icon: Bot },
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
    <div className="sticky top-0 z-[60] bg-pilot-surface/90 backdrop-blur-md shadow-bar">
      <div className="flex items-center gap-2 sm:gap-4 px-3 sm:px-6 lg:px-8 h-14">
        {/* Brand mark — the one and only product brand on screen */}
        <Link href="/" className="flex items-center gap-2.5 shrink-0 group" aria-label="KubePilot home">
          <div className="flex items-center justify-center w-9 h-9 rounded-xl bg-pilot-accent/12 border border-pilot-accent/25 group-hover:border-pilot-accent/50 transition-colors">
            <Navigation className="text-pilot-accent w-[1.15rem] h-[1.15rem] -rotate-45" />
          </div>
          <span className="font-display text-lg font-bold tracking-tight text-pilot-text-primary hidden md:inline">
            Kube<span className="text-pilot-accent">Pilot</span>
          </span>
        </Link>

        {/* Primary nav — readable icon+label pills with a clear active state */}
        <nav className="flex items-center gap-1 overflow-x-auto no-scrollbar" aria-label="Primary">
          {NAV_LINKS.map((link) => {
            const Icon = link.icon;
            const active = isActive(link.href);
            return (
              <Link
                key={link.href}
                href={link.href}
                aria-current={active ? "page" : undefined}
                className={`inline-flex items-center gap-2 px-3 sm:px-3.5 h-10 rounded-xl text-[0.95rem] font-semibold whitespace-nowrap transition-colors ${
                  active
                    ? "bg-pilot-accent/12 text-pilot-accent-light border border-pilot-accent/30"
                    : "text-pilot-text-secondary hover:text-pilot-text-primary hover:bg-pilot-hover/[0.05] border border-transparent"
                }`}
              >
                <Icon className={`w-[1.15rem] h-[1.15rem] ${active ? "text-pilot-accent" : "text-pilot-muted"}`} />
                {link.label}
              </Link>
            );
          })}
        </nav>

        <div className="ml-auto flex items-center gap-2 sm:gap-3 shrink-0">
          {locked && (
            <span
              className="hidden sm:inline-flex items-center gap-1.5 bg-pilot-accent/12 text-pilot-accent-light border border-pilot-accent/35 rounded-lg px-2.5 py-1.5 text-xs font-medium max-w-[12rem]"
              title="The dashboard is locked to this namespace via the URL. Remove the ?namespace= parameter to browse all namespaces."
            >
              <Lock className="w-3.5 h-3.5 shrink-0" />
              <span className="text-pilot-muted">ns</span>
              <span className="font-mono truncate">{namespace}</span>
            </span>
          )}
          <ThemeToggle />
          <span
            className="hidden lg:inline font-mono text-xs text-pilot-muted/70 select-text"
            title="Dashboard build version"
          >
            {BUILD_VERSION}
          </span>
        </div>
      </div>
    </div>
  );
}
