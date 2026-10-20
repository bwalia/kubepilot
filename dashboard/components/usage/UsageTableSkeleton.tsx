/** Placeholder shown while the usage table's code chunk loads. */
export function UsageTableSkeleton() {
  return (
    <div className="space-y-4" aria-busy="true" aria-label="Loading resource usage">
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        {[0, 1, 2].map((i) => (
          <div key={i} className="h-[4.5rem] rounded-xl bg-pilot-surface border border-pilot-border animate-pulse" />
        ))}
      </div>
      <div className="h-9 w-80 max-w-full rounded-lg bg-pilot-surface animate-pulse" />
      <div className="space-y-2">
        {[...Array(8)].map((_, i) => (
          <div key={i} className="h-12 rounded-lg bg-pilot-surface animate-pulse" />
        ))}
      </div>
    </div>
  );
}
