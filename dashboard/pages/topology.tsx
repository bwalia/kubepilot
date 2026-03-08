/**
 * Topology page — Service dependency graph visualization for a namespace.
 */
import { TopologyGraph } from "@/components/TopologyGraph";

export default function TopologyPage() {
  return (
    <div className="min-h-screen bg-pilot-bg text-white font-mono p-6">
      <TopologyGraph namespace="default" />
    </div>
  );
}
