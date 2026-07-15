/**
 * DeckSidebar — the section navigation rail for full-page browsers (e.g. the
 * Kubernetes Dashboard). On desktop it's a sticky left rail that collapses to
 * an icon-only strip (choice remembered); below `lg` it becomes a slide-in
 * drawer opened from a menu button the parent renders. Framer Motion drives the
 * drawer slide and the collapse chevron; all of it respects reduced-motion.
 */
import * as React from "react";
import type { LucideIcon } from "lucide-react";
import { ChevronLeft, X } from "lucide-react";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";

export interface DeckSection<K extends string = string> {
  key: K;
  label: string;
  icon: LucideIcon;
  badge?: number;
}

interface Props<K extends string> {
  heading?: string;
  items: DeckSection<K>[];
  active: K;
  onSelect: (key: K) => void;
  /** Mobile drawer visibility (controlled by the parent's menu button). */
  mobileOpen: boolean;
  onMobileOpenChange: (open: boolean) => void;
  /** localStorage key for the desktop collapse preference. */
  storageKey?: string;
}

const COLLAPSE_KEY_DEFAULT = "kubepilot-sidebar-collapsed";

function NavList<K extends string>({
  items,
  active,
  onSelect,
  collapsed,
}: {
  items: DeckSection<K>[];
  active: K;
  onSelect: (key: K) => void;
  collapsed: boolean;
}) {
  return (
    <nav className="flex flex-col gap-1 px-3 py-2" aria-label="Sections">
      {items.map((item) => {
        const Icon = item.icon;
        const isActive = item.key === active;
        return (
          <button
            key={item.key}
            type="button"
            onClick={() => onSelect(item.key)}
            aria-current={isActive ? "page" : undefined}
            title={collapsed ? item.label : undefined}
            className={`group relative flex items-center rounded-xl min-h-[3rem] text-[0.95rem] font-semibold transition-colors ${
              collapsed ? "justify-center px-0" : "gap-3.5 px-3.5"
            } ${
              isActive
                ? "bg-pilot-accent/[0.14] text-pilot-accent"
                : "text-pilot-text-secondary hover:text-pilot-text-primary hover:bg-pilot-hover/[0.05]"
            }`}
          >
            {isActive && (
              <span className="absolute left-0 top-2 bottom-2 w-1 rounded-r-full bg-pilot-accent" />
            )}
            <span className={`grid place-items-center shrink-0 ${isActive ? "text-pilot-accent" : "text-pilot-muted group-hover:text-pilot-text-secondary"}`}>
              <Icon className="w-[1.35rem] h-[1.35rem]" />
            </span>
            {!collapsed && <span className="truncate">{item.label}</span>}
            {typeof item.badge === "number" && item.badge > 0 && (
              collapsed ? (
                <span className="absolute top-2 right-2.5 w-2 h-2 rounded-full bg-pilot-danger" />
              ) : (
                <span className="ml-auto min-w-[1.4rem] h-6 px-1.5 grid place-items-center rounded-full bg-pilot-danger text-white text-xs font-bold tabular-nums">
                  {item.badge}
                </span>
              )
            )}
          </button>
        );
      })}
    </nav>
  );
}

export function DeckSidebar<K extends string>({
  heading = "Sections",
  items,
  active,
  onSelect,
  mobileOpen,
  onMobileOpenChange,
  storageKey = COLLAPSE_KEY_DEFAULT,
}: Props<K>) {
  const reduce = useReducedMotion();
  const [collapsed, setCollapsed] = React.useState(false);

  // Restore collapse preference after mount (SSR-safe).
  React.useEffect(() => {
    try {
      setCollapsed(localStorage.getItem(storageKey) === "1");
    } catch {
      /* storage unavailable — default expanded */
    }
  }, [storageKey]);

  const toggleCollapsed = () => {
    setCollapsed((c) => {
      const next = !c;
      try {
        localStorage.setItem(storageKey, next ? "1" : "0");
      } catch {
        /* ignore */
      }
      return next;
    });
  };

  const handleSelect = (key: K) => {
    onSelect(key);
    onMobileOpenChange(false); // close the drawer after picking on mobile
  };

  return (
    <>
      {/* ---------- Desktop rail ---------- */}
      <aside
        className={`hidden lg:flex flex-col shrink-0 sticky top-14 self-start h-[calc(100vh-3.5rem)] border-r border-pilot-border bg-pilot-surface transition-[width] duration-200 ease-out ${
          collapsed ? "w-[4.75rem]" : "w-60"
        }`}
      >
        <div className="px-4 pt-4 pb-1 h-9">
          {!collapsed && <span className="eyebrow">{heading}</span>}
        </div>
        <div className="flex-1 overflow-y-auto">
          <NavList items={items} active={active} onSelect={handleSelect} collapsed={collapsed} />
        </div>
        <div className="border-t border-pilot-border p-3">
          <button
            type="button"
            onClick={toggleCollapsed}
            aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
            className={`flex items-center rounded-xl w-full min-h-[2.75rem] text-sm font-semibold text-pilot-muted hover:text-pilot-text-primary hover:bg-pilot-hover/[0.05] transition-colors ${
              collapsed ? "justify-center px-0" : "gap-3.5 px-3.5"
            }`}
          >
            <motion.span
              className="grid place-items-center shrink-0"
              animate={reduce ? undefined : { rotate: collapsed ? 180 : 0 }}
              transition={{ duration: 0.2 }}
            >
              <ChevronLeft className="w-[1.35rem] h-[1.35rem]" />
            </motion.span>
            {!collapsed && <span>Collapse</span>}
          </button>
        </div>
      </aside>

      {/* ---------- Mobile drawer ---------- */}
      <AnimatePresence>
        {mobileOpen && (
          <>
            <motion.div
              className="lg:hidden fixed inset-0 z-[65] bg-black/50"
              initial={reduce ? { opacity: 1 } : { opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={reduce ? { opacity: 0 } : { opacity: 0 }}
              transition={{ duration: 0.2 }}
              onClick={() => onMobileOpenChange(false)}
            />
            <motion.aside
              className="lg:hidden fixed top-0 left-0 z-[66] h-full w-[17rem] bg-pilot-surface border-r border-pilot-border flex flex-col shadow-2xl"
              initial={reduce ? false : { x: "-100%" }}
              animate={{ x: 0 }}
              exit={reduce ? undefined : { x: "-100%" }}
              transition={{ type: "tween", duration: 0.26, ease: [0.4, 0, 0.2, 1] }}
              role="dialog"
              aria-label="Section navigation"
            >
              <div className="flex items-center justify-between px-4 h-14 border-b border-pilot-border">
                <span className="eyebrow">{heading}</span>
                <button
                  type="button"
                  onClick={() => onMobileOpenChange(false)}
                  aria-label="Close navigation"
                  className="p-2 rounded-lg text-pilot-muted hover:text-pilot-text-primary hover:bg-pilot-hover/[0.06]"
                >
                  <X className="w-5 h-5" />
                </button>
              </div>
              <div className="flex-1 overflow-y-auto">
                <NavList items={items} active={active} onSelect={handleSelect} collapsed={false} />
              </div>
            </motion.aside>
          </>
        )}
      </AnimatePresence>
    </>
  );
}
