/**
 * NamespacePicker — a type-to-find namespace combobox.
 *
 * Replaces a native <select>. On a cluster with a hundred namespaces a select
 * means opening a list and scrolling it, with no way to narrow down: you have
 * to already know the name AND find it by eye. Here you type three letters.
 *
 * Built on the listbox/combobox pattern rather than a component library: the
 * behaviour is ~100 lines (filter, arrow keys, Enter, Escape, click-outside)
 * and none of the Radix primitives already in this project do searchable
 * selection.
 */
import { useEffect, useId, useMemo, useRef, useState } from "react";
import { Check, ChevronsUpDown, Search, Layers, Clock, Lock } from "lucide-react";
import { fuzzyFilter, highlightParts } from "@/lib/fuzzy";
import { cn } from "@/lib/utils";

const RECENTS_KEY = "kubepilot-ns-recents";
const MAX_RECENTS = 5;

/** Namespaces the user picked before, most recent first. Survives sessions. */
function readRecents(): string[] {
  try {
    const raw = localStorage.getItem(RECENTS_KEY);
    return raw ? (JSON.parse(raw) as string[]).slice(0, MAX_RECENTS) : [];
  } catch {
    return [];
  }
}

function pushRecent(name: string) {
  if (!name) return;
  try {
    const next = [name, ...readRecents().filter((n) => n !== name)].slice(0, MAX_RECENTS);
    localStorage.setItem(RECENTS_KEY, JSON.stringify(next));
  } catch {
    /* storage unavailable — recents are a convenience, not state we need */
  }
}

export interface NamespaceOption {
  name: string;
  /** Optional per-namespace pod count, shown as a hint on the right. */
  count?: number;
}

interface Props {
  value: string;
  onChange: (namespace: string) => void;
  namespaces: NamespaceOption[];
  /** URL-locked: render as a static chip with no picker at all. */
  locked?: boolean;
  className?: string;
}

const ALL = "\u0000all"; // sentinel so "All namespaces" can be a normal row

export function NamespacePicker({ value, onChange, namespaces, locked, className }: Props) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [cursor, setCursor] = useState(0);
  const [recents, setRecents] = useState<string[]>([]);
  const rootRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLUListElement>(null);
  const listId = useId();

  useEffect(() => setRecents(readRecents()), [open]);

  // Rows = "All namespaces", then recents, then everything else — each fuzzy
  // filtered against the query so typing narrows every group at once.
  const rows = useMemo(() => {
    const all = { key: ALL, name: "All namespaces", group: "" as string, count: undefined as number | undefined };
    const recentSet = new Set(recents);
    const byName = new Map(namespaces.map((n) => [n.name, n]));

    const recentRows = recents
      .filter((n) => byName.has(n))
      .map((n) => ({ key: n, name: n, group: "Recent", count: byName.get(n)?.count }));
    const restRows = namespaces
      .filter((n) => !recentSet.has(n.name))
      .map((n) => ({ key: n.name, name: n.name, group: "All namespaces", count: n.count }));

    const pool = [all, ...recentRows, ...restRows];
    if (!query.trim()) return pool;
    // Keep "All namespaces" out of the fuzzy pool unless it genuinely matches.
    return fuzzyFilter(pool, query, (r) => r.name);
  }, [namespaces, recents, query]);

  // Reset the highlight whenever the result set changes under the cursor.
  useEffect(() => setCursor(0), [query, open]);

  // Close on outside click and on Escape, the two things users expect.
  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [open]);

  useEffect(() => {
    if (open) inputRef.current?.focus();
    else setQuery("");
  }, [open]);

  // Keep the highlighted row in view when arrowing past the fold.
  useEffect(() => {
    listRef.current?.querySelector<HTMLElement>(`[data-idx="${cursor}"]`)?.scrollIntoView({ block: "nearest" });
  }, [cursor]);

  const select = (key: string) => {
    const ns = key === ALL ? "" : key;
    onChange(ns);
    pushRecent(ns);
    setOpen(false);
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setCursor((c) => Math.min(c + 1, rows.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setCursor((c) => Math.max(c - 1, 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      if (rows[cursor]) select(rows[cursor].key);
    } else if (e.key === "Escape") {
      e.preventDefault();
      setOpen(false);
    }
  };

  if (locked) {
    return (
      <span
        className={cn(
          "inline-flex h-11 items-center gap-2 rounded-xl border border-pilot-accent/40 bg-pilot-accent/12 px-3 text-sm font-medium text-pilot-accent-light",
          className
        )}
        title="Locked to this namespace by the ?namespace= URL parameter"
      >
        <Lock className="h-4 w-4 shrink-0" aria-hidden="true" />
        <span className="font-mono truncate">{value}</span>
      </span>
    );
  }

  const label = value || "All namespaces";

  return (
    <div ref={rootRef} className={cn("relative", className)}>
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? listId : undefined}
        className="inline-flex h-11 w-full min-w-[13rem] items-center gap-2 rounded-xl border border-pilot-border bg-pilot-surface px-3 text-sm font-medium text-pilot-text-primary transition-colors hover:border-pilot-border-hover"
      >
        <Layers className="h-4 w-4 shrink-0 text-pilot-muted" aria-hidden="true" />
        <span className={cn("truncate", !value && "text-pilot-text-secondary")}>{label}</span>
        <ChevronsUpDown className="ml-auto h-4 w-4 shrink-0 text-pilot-muted" aria-hidden="true" />
      </button>

      {open && (
        <div className="absolute right-0 z-50 mt-2 w-[min(22rem,calc(100vw-2rem))] overflow-hidden rounded-xl border border-pilot-border bg-pilot-surface shadow-card-hover">
          <div className="flex items-center gap-2 border-b border-pilot-border px-3 py-2.5">
            <Search className="h-4 w-4 shrink-0 text-pilot-muted" aria-hidden="true" />
            <input
              ref={inputRef}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={onKeyDown}
              placeholder="Type to find a namespace…"
              aria-label="Filter namespaces"
              aria-autocomplete="list"
              aria-controls={listId}
              className="w-full bg-transparent text-sm text-pilot-text-primary placeholder:text-pilot-muted focus:outline-none"
            />
            {query && (
              <span className="shrink-0 text-xs tabular-nums text-pilot-muted">{rows.length}</span>
            )}
          </div>

          <ul
            ref={listRef}
            id={listId}
            role="listbox"
            aria-label="Namespaces"
            className="max-h-80 overflow-y-auto py-1"
          >
            {rows.length === 0 && (
              <li className="px-3 py-6 text-center text-sm text-pilot-muted">
                No namespace matches &ldquo;{query}&rdquo;.
              </li>
            )}
            {rows.map((row, i) => {
              const selected = (row.key === ALL && !value) || row.key === value;
              const showGroup = !query.trim() && row.group && rows[i - 1]?.group !== row.group;
              return (
                <li key={row.key}>
                  {showGroup && (
                    <div className="flex items-center gap-1.5 px-3 pb-1 pt-2 text-[0.7rem] font-semibold uppercase tracking-wider text-pilot-muted">
                      {row.group === "Recent" && <Clock className="h-3 w-3" aria-hidden="true" />}
                      {row.group}
                    </div>
                  )}
                  <button
                    type="button"
                    data-idx={i}
                    role="option"
                    aria-selected={selected}
                    onMouseEnter={() => setCursor(i)}
                    onClick={() => select(row.key)}
                    className={cn(
                      "flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors",
                      i === cursor ? "bg-pilot-accent/12 text-pilot-text-primary" : "text-pilot-text-secondary"
                    )}
                  >
                    <Check
                      className={cn("h-4 w-4 shrink-0", selected ? "text-pilot-accent" : "opacity-0")}
                      aria-hidden="true"
                    />
                    <span className={cn("truncate", row.key !== ALL && "font-mono")}>
                      {highlightParts(row.name, query).map((part, pi) =>
                        part.hit ? (
                          <mark key={pi} className="bg-transparent font-bold text-pilot-accent-light">
                            {part.text}
                          </mark>
                        ) : (
                          <span key={pi}>{part.text}</span>
                        )
                      )}
                    </span>
                    {row.count !== undefined && (
                      <span className="ml-auto shrink-0 text-xs tabular-nums text-pilot-muted">
                        {row.count} pod{row.count === 1 ? "" : "s"}
                      </span>
                    )}
                  </button>
                </li>
              );
            })}
          </ul>
        </div>
      )}
    </div>
  );
}
