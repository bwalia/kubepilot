/**
 * Breadcrumb — a compact location trail (e.g. "Dashboard › Overview"). The last
 * item is the current location. Used at the top of pages so operators always
 * know where they are, and so section names don't need a second large heading.
 */
import { ChevronRight } from "lucide-react";

export function Breadcrumb({ items, className = "" }: { items: string[]; className?: string }) {
  return (
    <nav aria-label="Breadcrumb" className={`flex items-center gap-1.5 text-sm min-w-0 ${className}`}>
      {items.map((item, i) => {
        const last = i === items.length - 1;
        return (
          <span key={`${item}-${i}`} className="flex items-center gap-1.5 min-w-0">
            {i > 0 && <ChevronRight className="w-4 h-4 text-pilot-muted/60 shrink-0" aria-hidden="true" />}
            <span
              className={last ? "font-bold text-pilot-text-primary truncate" : "font-medium text-pilot-muted"}
              aria-current={last ? "page" : undefined}
            >
              {item}
            </span>
          </span>
        );
      })}
    </nav>
  );
}
