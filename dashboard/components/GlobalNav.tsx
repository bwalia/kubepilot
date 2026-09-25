/**
 * GlobalNav — the single primary top bar shared across every page: brand,
 * product navigation (CoPilot / Pilot / AutoPilot), and the theme toggle.
 * It is the one place the product names itself, so page headers below it carry
 * only their own title + context (no duplicated brand).
 */
import Link from "next/link";
import { useRouter } from "next/router";
import { useQuery } from "@tanstack/react-query";
import { Navigation, Sparkles, LayoutDashboard, Bot, Radio, Search } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { listNamespaces } from "@/lib/api";
import { qk } from "@/lib/queryKeys";
import { useNamespace } from "@/lib/useNamespace";
import { ThemeToggle } from "@/components/ThemeToggle";
import { NamespacePicker } from "@/components/ui/NamespacePicker";
import { openCommandPalette } from "@/components/CommandPalette";

const NAV_LINKS: { href: string; label: string; icon: LucideIcon }[] = [
  { href: "/", label: "CoPilot", icon: Sparkles },
  { href: "/dashboard", label: "Pilot", icon: LayoutDashboard },
  { href: "/autopilot", label: "AutoPilot", icon: Bot },
  { href: "/otel", label: "OTLP", icon: Radio },
];

// Stamped at build time by `make dashboard` (NEXT_PUBLIC_BUILD_VERSION).
// Falls back to "dev" for local `next dev` runs that don't set it.
const BUILD_VERSION = process.env.NEXT_PUBLIC_BUILD_VERSION || "dev";

export function GlobalNav() {
  const { pathname } = useRouter();
  const { namespace, setNamespace, locked } = useNamespace();

  // The namespace list is small and shared with every other surface through
  // this key, so putting the picker in the top bar costs no extra request.
  const { data: namespaces = [] } = useQuery({
    queryKey: qk.namespaces(),
    queryFn: listNamespaces,
    staleTime: 5 * 60_000,
  });
  const options = namespaces.map((ns) => ({ name: ns.Name }));

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

        <div className="ml-auto flex shrink-0 items-center gap-2 sm:gap-3">
          {/* The namespace scope is global: set it once here and every page —
              home, dashboard, events, topology — follows it. */}
          <NamespacePicker
            value={namespace}
            onChange={setNamespace}
            namespaces={options}
            locked={locked}
            compact
          />

          {/* Search is the fastest route to anything in the cluster, so it sits
              in the top bar on every page. Icon-only below md, but never
              without an accessible name. */}
          <button
            type="button"
            onClick={openCommandPalette}
            aria-label="Search the cluster"
            className="inline-flex h-10 items-center gap-2 rounded-xl border border-pilot-border bg-pilot-surface-2 px-2.5 text-sm text-pilot-text-secondary transition-colors hover:border-pilot-border-hover hover:text-pilot-text-primary"
          >
            <Search className="h-[1.15rem] w-[1.15rem] shrink-0" aria-hidden="true" />
            <span className="hidden lg:inline">Search</span>
            <kbd className="hidden rounded border border-pilot-border bg-pilot-surface px-1.5 py-0.5 font-mono text-[0.7rem] text-pilot-muted xl:inline">
              &#8984;K
            </kbd>
          </button>

          <ThemeToggle />
          <span
            className="hidden font-mono text-xs text-pilot-muted/70 select-text 2xl:inline"
            title="Dashboard build version"
          >
            {BUILD_VERSION}
          </span>
        </div>
      </div>
    </div>
  );
}
