/**
 * GlobalNav — slim top-level navigation shared across all pages.
 * Purely additive: it renders above each page's own content and does not
 * replace the existing AI Troubleshooting page header.
 */
import Link from "next/link";
import { useRouter } from "next/router";
import { Layers } from "lucide-react";

const NAV_LINKS = [
  { href: "/", label: "Kubernetes CoPilot" },
  { href: "/dashboard", label: "Kubernetes Dashboard" },
  { href: "/autopilot", label: "Autopilot" },
];

export function GlobalNav() {
  const { pathname } = useRouter();

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
      </div>
    </div>
  );
}
