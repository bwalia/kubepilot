/**
 * Topology page — Service dependency graph visualization for a namespace.
 */
import { ServiceTopologyCanvas } from "@/components/ServiceTopologyCanvas";

export default function TopologyPage() {
  return (
    <div className="h-[calc(100vh-140px)] min-h-[520px] p-4 sm:p-6 lg:p-8">
      <ServiceTopologyCanvas />
    </div>
  );
}
