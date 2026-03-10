/**
 * Topology page — Service dependency graph visualization for a namespace.
 */
import { ServiceTopologyCanvas } from "@/components/ServiceTopologyCanvas";

export default function TopologyPage() {
  return (
    <div className="h-[calc(100vh-180px)] min-h-[500px] font-mono">
      <ServiceTopologyCanvas />
    </div>
  );
}
