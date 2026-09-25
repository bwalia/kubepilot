/**
 * ResourceTable — the one list surface every dashboard section renders through.
 *
 * Upgraded in place rather than added alongside, so Workloads, Network and
 * Config & Storage all gain the same behaviour from one change:
 *   - a filter box that fuzzy-matches (typing "chkout" finds "checkout-7d9f8b")
 *   - click-to-sort columns with a real aria-sort announcement
 *   - a count of what is shown vs. what exists, so a filter is never silently
 *     hiding things
 *   - a sticky header, so column meaning survives a long scroll
 *   - cards instead of a sideways-scrolling table on narrow screens
 *   - loading / empty / error states that say what to do next
 *
 * The Column<T> shape is unchanged; `sortValue` and the table's `searchText`
 * are opt-in, and a column without `sortValue` simply is not sortable.
 */
import { useEffect, useMemo, useState, type ReactNode } from "react";
import { ArrowDown, ArrowUp, ChevronsUpDown, Search, Inbox, AlertTriangle, X } from "lucide-react";
import { fuzzyFilter } from "@/lib/fuzzy";
import { cn } from "@/lib/utils";

export interface Column<T> {
  header: string;
  /** Cell renderer. Receives the row item. */
  cell: (item: T) => ReactNode;
  align?: "left" | "right" | "center";
  /** Return a comparable value to make this column sortable. */
  sortValue?: (item: T) => string | number;
  /** Hide this column on narrow screens (it still appears in the card view). */
  hideBelow?: "sm" | "md" | "lg";
}

interface Props<T> {
  columns: Column<T>[];
  items: T[];
  rowKey: (item: T) => string;
  loading?: boolean;
  error?: unknown;
  emptyMessage?: string;
  onRowClick?: (item: T) => void;
  /** Text the filter box matches against. Omit to hide the filter box. */
  searchText?: (item: T) => string;
  searchPlaceholder?: string;
  /** Extra controls rendered on the right of the toolbar. */
  filterSlot?: ReactNode;
  /** Word for the thing being listed, used in counts and empty states. */
  noun?: string;
}

/**
 * Rows rendered before the "show more" break. A real cluster returns 700+ pods,
 * and rendering all of them costs a visibly janky first paint for a list nobody
 * scrolls to the bottom of — the filter above is how you find a specific one.
 * A plain page-size beats pulling in a virtualisation dependency here.
 */
const PAGE_SIZE = 100;

const HIDE_CLASS = {
  sm: "hidden sm:table-cell",
  md: "hidden md:table-cell",
  lg: "hidden lg:table-cell",
} as const;

export function ResourceTable<T>({
  columns,
  items,
  rowKey,
  loading,
  error,
  emptyMessage,
  onRowClick,
  searchText,
  searchPlaceholder,
  filterSlot,
  noun = "resource",
}: Props<T>) {
  const [query, setQuery] = useState("");
  const [sort, setSort] = useState<{ col: number; dir: "asc" | "desc" } | null>(null);
  const [limit, setLimit] = useState(PAGE_SIZE);

  const shown = useMemo(() => {
    let rows = searchText ? fuzzyFilter(items, query, searchText) : items;
    if (sort) {
      const get = columns[sort.col]?.sortValue;
      if (get) {
        // Copy before sorting: `items` belongs to the react-query cache and
        // sorting it in place would mutate shared state.
        rows = [...rows].sort((a, b) => {
          const av = get(a);
          const bv = get(b);
          const cmp =
            typeof av === "number" && typeof bv === "number"
              ? av - bv
              : String(av).localeCompare(String(bv), undefined, { numeric: true });
          return sort.dir === "asc" ? cmp : -cmp;
        });
      }
    }
    return rows;
  }, [items, query, sort, columns, searchText]);

  // Filtering or re-sorting produces a different list, so start from the top.
  useEffect(() => setLimit(PAGE_SIZE), [query, sort]);

  const visible = shown.slice(0, limit);

  const toggleSort = (i: number) => {
    setSort((s) =>
      s?.col === i ? (s.dir === "asc" ? { col: i, dir: "desc" } : null) : { col: i, dir: "asc" }
    );
  };

  const toolbar = (searchText || filterSlot) && (
    <div className="mb-4 flex flex-wrap items-center gap-3">
      {searchText && (
        <div className="flex h-11 min-w-0 flex-1 items-center gap-2 rounded-xl border border-pilot-border bg-pilot-surface px-3 sm:max-w-sm sm:flex-none focus-within:border-pilot-accent/60 focus-within:ring-2 focus-within:ring-pilot-accent/25">
          <Search className="h-4 w-4 shrink-0 text-pilot-muted" aria-hidden="true" />
          <input
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={searchPlaceholder ?? `Filter ${noun}s…`}
            aria-label={`Filter ${noun}s`}
            className="w-full bg-transparent text-sm text-pilot-text-primary placeholder:text-pilot-muted focus:outline-none [&::-webkit-search-cancel-button]:hidden"
          />
          {query && (
            <button
              type="button"
              onClick={() => setQuery("")}
              aria-label="Clear filter"
              className="shrink-0 rounded-md p-0.5 text-pilot-muted hover:text-pilot-text-primary"
            >
              <X className="h-4 w-4" />
            </button>
          )}
        </div>
      )}
      {/* Never let a filter hide rows silently — always say how many of how many.
          While loading, say nothing rather than "0 pods", which reads as an
          answer ("there are none") when it is really "not known yet". */}
      <span className="text-sm tabular-nums text-pilot-muted" aria-live="polite">
        {loading
          ? "Loading\u2026"
          : query
          ? `${shown.length} of ${items.length}`
          : `${items.length} ${noun}${items.length === 1 ? "" : "s"}`}
      </span>
      {filterSlot && <div className="ml-auto flex items-center gap-2">{filterSlot}</div>}
    </div>
  );

  if (loading) {
    return (
      <div>
        {toolbar}
        <div className="space-y-2" aria-busy="true" aria-label={`Loading ${noun}s`}>
          {[...Array(5)].map((_, i) => (
            <div key={i} className="h-14 animate-pulse rounded-xl bg-pilot-surface" />
          ))}
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div>
        {toolbar}
        <div className="flex items-start gap-3 rounded-xl border border-pilot-danger/30 bg-pilot-danger/10 p-4">
          <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-pilot-danger" aria-hidden="true" />
          <div className="min-w-0">
            <p className="text-sm font-semibold text-pilot-danger">Could not load {noun}s.</p>
            <p className="mt-1 break-words text-xs text-pilot-text-secondary">
              {error instanceof Error ? error.message : String(error)}
            </p>
            <p className="mt-1.5 text-xs text-pilot-muted">
              The cluster may be unreachable, or this kubeconfig may not have permission to list {noun}s.
            </p>
          </div>
        </div>
      </div>
    );
  }

  const empty = shown.length === 0;

  return (
    <div>
      {toolbar}

      {empty ? (
        <div className="rounded-xl border border-dashed border-pilot-border bg-pilot-surface/50 px-6 py-12 text-center">
          <Inbox className="mx-auto h-7 w-7 text-pilot-muted" aria-hidden="true" />
          <p className="mt-3 text-sm font-medium text-pilot-text-secondary">
            {query ? `No ${noun} matches “${query}”.` : emptyMessage ?? `No ${noun}s here.`}
          </p>
          {query ? (
            <button
              type="button"
              onClick={() => setQuery("")}
              className="mt-2 text-sm font-medium text-pilot-accent-light hover:underline"
            >
              Clear the filter
            </button>
          ) : (
            <p className="mt-1 text-xs text-pilot-muted">
              Try a different namespace, or choose “All namespaces”.
            </p>
          )}
        </div>
      ) : (
        <>
          {/* Table — md and up. */}
          <div className="hidden overflow-hidden rounded-xl border border-pilot-border bg-pilot-surface shadow-card md:block">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead className="sticky top-0 z-10 bg-pilot-surface">
                  <tr>
                    {columns.map((col, i) => {
                      const sortable = Boolean(col.sortValue);
                      const active = sort?.col === i;
                      const SortIcon = !active ? ChevronsUpDown : sort.dir === "asc" ? ArrowUp : ArrowDown;
                      return (
                        <th
                          key={i}
                          scope="col"
                          aria-sort={active ? (sort.dir === "asc" ? "ascending" : "descending") : sortable ? "none" : undefined}
                          className={cn(
                            "whitespace-nowrap border-b-2 border-pilot-border px-4 py-2.5",
                            col.align === "right" ? "text-right" : col.align === "center" ? "text-center" : "text-left",
                            col.hideBelow && HIDE_CLASS[col.hideBelow]
                          )}
                        >
                          {sortable ? (
                            <button
                              type="button"
                              onClick={() => toggleSort(i)}
                              className={cn(
                                "eyebrow inline-flex items-center gap-1.5 rounded transition-colors hover:text-pilot-text-primary",
                                active && "text-pilot-accent"
                              )}
                            >
                              {col.header}
                              <SortIcon className="h-3 w-3" aria-hidden="true" />
                            </button>
                          ) : (
                            <span className="eyebrow">{col.header}</span>
                          )}
                        </th>
                      );
                    })}
                  </tr>
                </thead>
                <tbody className="divide-y divide-pilot-border">
                  {visible.map((item) => (
                    <tr
                      key={rowKey(item)}
                      onClick={onRowClick ? () => onRowClick(item) : undefined}
                      // Rows open a detail panel, so they must be reachable and
                      // activatable without a mouse.
                      tabIndex={onRowClick ? 0 : undefined}
                      role={onRowClick ? "button" : undefined}
                      onKeyDown={
                        onRowClick
                          ? (e) => {
                              if (e.key === "Enter" || e.key === " ") {
                                e.preventDefault();
                                onRowClick(item);
                              }
                            }
                          : undefined
                      }
                      className={cn(
                        "transition-colors hover:bg-pilot-surface-2",
                        onRowClick && "cursor-pointer focus-visible:bg-pilot-accent/10 focus-visible:outline-none"
                      )}
                    >
                      {columns.map((col, i) => (
                        <td
                          key={i}
                          className={cn(
                            "px-4 py-3",
                            // The first column is the row's identity — a
                            // resource name. Wrapping it mid-identifier
                            // ("beacon-alert / manager-777b / 8598ff-fbpcj")
                            // makes it unreadable and unscannable, so it stays
                            // on one line and the table scrolls instead.
                            i === 0 && "whitespace-nowrap",
                            col.align === "right" ? "text-right" : col.align === "center" ? "text-center" : "text-left",
                            col.hideBelow && HIDE_CLASS[col.hideBelow]
                          )}
                        >
                          {col.cell(item)}
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          {/* Cards — below md, so a phone never scrolls a table sideways. */}
          <ul className="space-y-2 md:hidden">
            {visible.map((item) => (
              <li key={rowKey(item)}>
                <div
                  onClick={onRowClick ? () => onRowClick(item) : undefined}
                  tabIndex={onRowClick ? 0 : undefined}
                  role={onRowClick ? "button" : undefined}
                  onKeyDown={
                    onRowClick
                      ? (e) => {
                          if (e.key === "Enter" || e.key === " ") {
                            e.preventDefault();
                            onRowClick(item);
                          }
                        }
                      : undefined
                  }
                  className={cn(
                    "rounded-xl border border-pilot-border bg-pilot-surface p-4 shadow-card",
                    onRowClick && "cursor-pointer active:border-pilot-accent/50"
                  )}
                >
                  {/* First column is the identity of the row — give it the lead. */}
                  <div className="mb-2 min-w-0 break-words text-sm font-semibold text-pilot-text-primary">
                    {columns[0]?.cell(item)}
                  </div>
                  <dl className="grid grid-cols-[auto,1fr] gap-x-3 gap-y-1.5">
                    {columns.slice(1).map((col, i) => (
                      <div key={i} className="contents">
                        <dt className="eyebrow self-center">{col.header}</dt>
                        <dd className="min-w-0 break-words text-sm text-pilot-text-secondary">{col.cell(item)}</dd>
                      </div>
                    ))}
                  </dl>
                </div>
              </li>
            ))}
          </ul>

          {shown.length > visible.length && (
            <div className="mt-3 flex items-center justify-center gap-3">
              <span className="text-sm text-pilot-muted tabular-nums">
                Showing {visible.length} of {shown.length}
              </span>
              <button
                type="button"
                onClick={() => setLimit((n) => n + PAGE_SIZE)}
                className="h-10 rounded-xl border border-pilot-border bg-pilot-surface px-4 text-sm font-medium text-pilot-text-primary transition-colors hover:border-pilot-border-hover"
              >
                Show {Math.min(PAGE_SIZE, shown.length - visible.length)} more
              </button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
