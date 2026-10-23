import SwiftUI

/// The heaviest pods by CPU, memory and storage — the iOS counterpart of the
/// dashboard's Top consumers strip. Each row opens the pod.
struct TopConsumersSection: View {
    let report: PodUsageReport
    let namespace: String
    var limit = 5

    @State private var metric: UsageMetric = .cpu

    private var scope: String { namespace.isEmpty ? "all namespaces" : namespace }

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.spacingSM) {
            HStack(alignment: .firstTextBaseline, spacing: Theme.spacingXS) {
                Image(systemName: "flame.fill")
                    .foregroundStyle(Theme.warning)
                Text("Top consumers")
                    .font(Theme.Typography.sectionTitle)
                    .foregroundStyle(Theme.textPrimary)
                Text("· \(scope)")
                    .font(.caption.monospaced())
                    .foregroundStyle(Theme.muted)
                    .lineLimit(1)
            }

            Picker("Metric", selection: $metric) {
                ForEach(UsageMetric.allCases) { m in
                    Label(m.label, systemImage: m.systemImage).tag(m)
                }
            }
            .pickerStyle(.segmented)

            if let note = availabilityNote {
                Label(note, systemImage: "info.circle")
                    .font(.caption2)
                    .foregroundStyle(Theme.muted)
            }

            SurfaceCard(cornerRadius: Theme.cornerRadiusSmall) {
                let top = metric.top(report.pods, limit)
                if top.isEmpty {
                    Text("No \(metric.label.lowercased()) usage reported.")
                        .font(.subheadline)
                        .foregroundStyle(Theme.muted)
                        .padding(.vertical, Theme.spacingSM)
                } else {
                    let maxValue = metric.value(of: top[0])
                    VStack(spacing: Theme.spacingSM) {
                        ForEach(Array(top.enumerated()), id: \.element.id) { index, pod in
                            NavigationLink(value: DashboardDestination.pod(pod.namespace, pod.name)) {
                                TopConsumerRow(rank: index + 1, pod: pod, metric: metric, maxValue: maxValue)
                            }
                            .buttonStyle(.plain)
                        }
                    }
                }
            }
        }
    }

    private var availabilityNote: String? {
        switch metric {
        case .cpu, .memory:
            report.metricsAvailable ? nil : "metrics-server is not answering, so CPU and memory show zero."
        case .storage:
            report.storageStatsAvailable ? nil : "Kubelet storage stats are unavailable, so storage shows claimed PVC size."
        }
    }
}

private struct TopConsumerRow: View {
    let rank: Int
    let pod: PodUsage
    let metric: UsageMetric
    let maxValue: Int64

    private var color: Color {
        switch metric {
        case .cpu: Theme.accent
        case .memory: Theme.purple
        case .storage: Theme.warning
        }
    }

    var body: some View {
        let value = metric.value(of: pod)
        let pressure = metric.pressure(of: pod)
        VStack(alignment: .leading, spacing: 4) {
            HStack(alignment: .firstTextBaseline) {
                Text("\(rank).")
                    .font(.caption.monospacedDigit())
                    .foregroundStyle(Theme.muted)
                Text(pod.name)
                    .font(.subheadline.monospaced())
                    .foregroundStyle(Theme.textPrimary)
                    .lineLimit(1)
                    .truncationMode(.middle)
                Spacer(minLength: Theme.spacingSM)
                Text(metric.format(value))
                    .font(.subheadline.monospacedDigit().weight(.semibold))
                    .foregroundStyle(color)
            }
            HStack(spacing: Theme.spacingSM) {
                GeometryReader { geo in
                    let fraction = maxValue > 0 ? max(0.03, Double(value) / Double(maxValue)) : 0
                    ZStack(alignment: .leading) {
                        Capsule().fill(Theme.surfaceElevated)
                        Capsule().fill(color).frame(width: geo.size.width * fraction)
                    }
                }
                .frame(height: 6)
                Text(pod.namespace)
                    .font(.caption2.monospaced())
                    .foregroundStyle(Theme.muted)
                    .lineLimit(1)
                    .frame(maxWidth: 110, alignment: .trailing)
            }
            if let pressure {
                let hot = pressure.of == "limit" && pressure.pct >= 85
                Text("\(Int(pressure.pct.rounded()))% of \(pressure.of)")
                    .font(.caption2.weight(hot ? .semibold : .regular))
                    .foregroundStyle(hot ? Theme.danger : Theme.muted)
            }
            if metric == .storage && pod.storageEstimated == true {
                Text("includes claimed PVC size (live usage not measurable)")
                    .font(.caption2)
                    .foregroundStyle(Theme.muted)
            }
        }
        .contentShape(Rectangle())
        .accessibilityElement(children: .combine)
    }
}
