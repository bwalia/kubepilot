/**
 * SubTabs — the secondary tab strip inside a dashboard section.
 *
 * A segmented control rather than an underline row: at this level the tabs are
 * switching between kinds of thing (Pods / Deployments / Jobs), which reads as
 * a set of choices, not as page navigation. The active choice is carried by a
 * filled surface AND a weight change, so it survives greyscale.
 */
import { cn } from "@/lib/utils";

interface Props<K extends string> {
  tabs: { key: K; label: string; count?: number }[];
  active: K;
  onChange: (key: K) => void;
  /** Accessible name for the group, e.g. "Workload kinds". */
  label?: string;
}

export function SubTabs<K extends string>({ tabs, active, onChange, label }: Props<K>) {
  return (
    <div
      role="tablist"
      aria-label={label ?? "Sections"}
      className="no-scrollbar mb-5 flex gap-1 overflow-x-auto rounded-xl border border-pilot-border bg-pilot-surface-2/60 p-1"
    >
      {tabs.map((tab) => {
        const selected = active === tab.key;
        return (
          <button
            key={tab.key}
            role="tab"
            aria-selected={selected}
            onClick={() => onChange(tab.key)}
            className={cn(
              "inline-flex h-9 shrink-0 items-center gap-2 whitespace-nowrap rounded-lg px-3.5 text-sm transition-colors",
              selected
                ? "bg-pilot-surface font-semibold text-pilot-text-primary shadow-card"
                : "font-medium text-pilot-muted hover:text-pilot-text-secondary"
            )}
          >
            {tab.label}
            {tab.count !== undefined && (
              <span
                className={cn(
                  "rounded-md px-1.5 py-0.5 text-[0.7rem] tabular-nums",
                  selected ? "bg-pilot-accent/15 text-pilot-accent-light" : "bg-pilot-surface text-pilot-muted"
                )}
              >
                {tab.count}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}
