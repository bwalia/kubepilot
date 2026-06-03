import { ServiceTopologyCanvas } from "@/components/ServiceTopologyCanvas";

// TopologySection embeds the service-dependency graph (also used on the AI page)
// into the Kubernetes Dashboard browser. ServiceTopologyCanvas is self-contained
// (own namespace selector + queries), so no props are required.
export function TopologySection() {
  return (
    <div className="h-[calc(100vh-220px)] min-h-[480px] font-mono">
      <ServiceTopologyCanvas />
    </div>
  );
}
