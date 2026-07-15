/**
 * ResourceTable — a small, generic table for the Kubernetes Dashboard sections.
 * Renders columns + rows with the shared pilot-* styling and handles the
 * loading / empty / error states consistently across resource kinds.
 */
import type { ReactNode } from "react";

export interface Column<T> {
  header: string;
  /** Cell renderer. Receives the row item. */
  cell: (item: T) => ReactNode;
  align?: "left" | "right" | "center";
}

interface Props<T> {
  columns: Column<T>[];
  items: T[];
  rowKey: (item: T) => string;
  loading?: boolean;
  error?: unknown;
  emptyMessage?: string;
  onRowClick?: (item: T) => void;
}

export function ResourceTable<T>({
  columns,
  items,
  rowKey,
  loading,
  error,
  emptyMessage = "No resources found.",
  onRowClick,
}: Props<T>) {
  if (loading) {
    return (
      <div className="space-y-2">
        {[...Array(4)].map((_, i) => (
          <div key={i} className="h-12 bg-pilot-surface rounded-lg animate-pulse" />
        ))}
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-pilot-danger/10 border border-pilot-danger/30 rounded-lg p-4 text-sm text-pilot-danger">
        Failed to load: {error instanceof Error ? error.message : String(error)}
      </div>
    );
  }

  return (
    <div className="bg-pilot-surface border border-pilot-border rounded-xl overflow-hidden shadow-card">
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr>
              {columns.map((col, i) => (
                <th
                  key={i}
                  className={`px-5 py-3 border-b-2 border-pilot-border ${
                    col.align === "right"
                      ? "text-right"
                      : col.align === "center"
                      ? "text-center"
                      : "text-left"
                  }`}
                >
                  <span className="eyebrow">{col.header}</span>
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-pilot-border">
            {items.map((item) => (
              <tr
                key={rowKey(item)}
                onClick={onRowClick ? () => onRowClick(item) : undefined}
                className={`hover:bg-pilot-surface-2 transition-colors ${onRowClick ? "cursor-pointer" : ""}`}
              >
                {columns.map((col, i) => (
                  <td
                    key={i}
                    className={`px-5 py-3.5 ${
                      col.align === "right"
                        ? "text-right"
                        : col.align === "center"
                        ? "text-center"
                        : "text-left"
                    }`}
                  >
                    {col.cell(item)}
                  </td>
                ))}
              </tr>
            ))}
            {items.length === 0 && (
              <tr>
                <td
                  colSpan={columns.length}
                  className="px-5 py-10 text-center text-pilot-muted text-sm"
                >
                  {emptyMessage}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
