import { ClusterEventsTroubleshooting } from "@/components/ClusterEventsTroubleshooting";

// EventsSection reuses the existing cluster events + troubleshooting component,
// which manages its own queries and cluster-wide event stream with filtering.
export function EventsSection() {
  return <ClusterEventsTroubleshooting />;
}
