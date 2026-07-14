/**
 * TopologyGraph — Service dependency graph visualization.
 * Renders services as nodes with health status and connection edges.
 * Uses a simple CSS-grid-based layout (no external graph library dependency).
 */
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Network, CheckCircle, XCircle, RefreshCw } from "lucide-react";
import { getTopology, type ServiceTopology, type ServiceNode } from "@/lib/api";

export function TopologyGraph({ namespace }: { namespace: string }) {
  const [selectedNs, setSelectedNs] = useState(namespace || "default");
  const { data: topology, isLoading, refetch } = useQuery({
    queryKey: ["topology", selectedNs],
    queryFn: () => getTopology(selectedNs),
    enabled: !!selectedNs,
  });

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="eyebrow flex items-center gap-2">
          <Network className="w-4 h-4 text-pilot-accent" />
          Service Topology
        </h3>
        <div className="flex items-center gap-2">
          <input
            type="text"
            value={selectedNs}
            onChange={(e) => setSelectedNs(e.target.value)}
            placeholder="Namespace"
            className="bg-pilot-surface border border-pilot-border rounded-lg px-3 py-1 text-sm text-pilot-text-primary placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent/60 focus:ring-2 focus:ring-pilot-accent/25 w-40"
          />
          <button
            onClick={() => refetch()}
            className="p-1.5 rounded-lg bg-pilot-surface border border-pilot-border hover:border-pilot-border-hover"
          >
            <RefreshCw className="w-4 h-4 text-pilot-muted" />
          </button>
        </div>
      </div>

      {isLoading && (
        <div className="text-pilot-muted text-sm py-8 text-center">
          Loading topology...
        </div>
      )}

      {topology && topology.services && topology.services.length > 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
          {topology.services.map((svc) => (
            <ServiceCard key={`${svc.namespace}/${svc.name}`} service={svc} />
          ))}
        </div>
      ) : (
        !isLoading && (
          <div className="text-pilot-muted text-sm py-8 text-center">
            No services found in namespace &quot;{selectedNs}&quot;.
          </div>
        )
      )}

      {topology && topology.edges && topology.edges.length > 0 && (
        <div className="mt-4">
          <h4 className="eyebrow mb-2">Connections</h4>
          <div className="space-y-1">
            {topology.edges.map((edge, i) => (
              <div key={i} className="flex items-center gap-2 text-xs text-pilot-muted bg-pilot-surface border border-pilot-border rounded-lg px-3 py-1.5">
                <span className="text-pilot-text-primary">{edge.from || "?"}</span>
                <span className="text-pilot-accent">&#8594;</span>
                <span className="text-pilot-text-primary">{edge.to || "(exposed)"}</span>
                <span className="ml-auto">{edge.type}:{edge.port}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function ServiceCard({ service }: { service: ServiceNode }) {
  return (
    <div className={`bg-pilot-surface border rounded-xl shadow-card p-3 ${service.healthy ? "border-pilot-border" : "border-pilot-danger/50"}`}>
      <div className="flex items-center gap-2 mb-2">
        {service.healthy ? (
          <CheckCircle className="w-4 h-4 text-pilot-success" />
        ) : (
          <XCircle className="w-4 h-4 text-pilot-danger" />
        )}
        <span className="text-sm font-display font-bold text-pilot-text-primary">{service.name}</span>
        <span className="text-xs text-pilot-muted ml-auto">{service.pods?.length || 0} pods</span>
      </div>
      {service.anomalies && service.anomalies.length > 0 && (
        <div className="space-y-1 mt-1">
          {service.anomalies.map((a, i) => (
            <p key={i} className="text-xs text-pilot-danger truncate">{a}</p>
          ))}
        </div>
      )}
    </div>
  );
}
