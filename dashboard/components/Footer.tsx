/**
 * Footer — a slim flight-deck baseboard shown on every page. Its job is to
 * surface the deployed build's git tag version (from the backend's
 * /api/v1/version, i.e. `git describe --tags`) so operators can confirm exactly
 * what is running. Purely additive; renders below each page's content.
 */
import { useQuery } from "@tanstack/react-query";
import { getVersion } from "@/lib/api";

export function Footer() {
  const { data, isError } = useQuery({
    queryKey: ["version"],
    queryFn: getVersion,
    staleTime: 5 * 60_000,
    refetchInterval: false,
    retry: 1,
  });

  const version = data?.version ?? (isError ? "unavailable" : "…");

  return (
    <footer className="mt-auto border-t border-pilot-border bg-pilot-bg/85 backdrop-blur-md">
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 px-4 sm:px-6 lg:px-8 py-2.5">
        <span className="text-2xs text-pilot-muted">
          KubePilot · flight deck
        </span>
        <div className="flex items-center gap-2 font-mono text-2xs text-pilot-text-secondary">
          <span className="eyebrow text-pilot-muted">version</span>
          <span
            className="text-pilot-text-primary"
            title={
              data
                ? `commit ${data.commit} · built ${data.build} · ${data.goVersion} · ${data.platform}`
                : "Deployed git tag version"
            }
          >
            {version}
          </span>
        </div>
      </div>
    </footer>
  );
}
