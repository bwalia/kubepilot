import WidgetKit
import SwiftUI

struct ClusterHealthEntry: TimelineEntry {
    let date: Date
    let clusterName: String
    let healthLabel: String
    let failingPods: Int
    let crashloopPods: Int
    let alerts: Int
}

struct ClusterHealthProvider: TimelineProvider {
    func placeholder(in context: Context) -> ClusterHealthEntry {
        ClusterHealthEntry(
            date: .now,
            clusterName: "Production",
            healthLabel: "Healthy",
            failingPods: 0,
            crashloopPods: 0,
            alerts: 0
        )
    }

    func getSnapshot(in context: Context, completion: @escaping (ClusterHealthEntry) -> Void) {
        completion(placeholder(in: context))
    }

    func getTimeline(in context: Context, completion: @escaping (Timeline<ClusterHealthEntry>) -> Void) {
        let entry = placeholder(in: context)
        let timeline = Timeline(entries: [entry], policy: .after(.now.addingTimeInterval(300)))
        completion(timeline)
    }
}

struct ClusterHealthWidgetView: View {
    var entry: ClusterHealthEntry
    @Environment(\.widgetFamily) private var family

    var body: some View {
        switch family {
        case .systemSmall: smallView
        case .systemMedium: mediumView
        default: largeView
        }
    }

    private var smallView: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(entry.clusterName).font(.caption).foregroundStyle(.secondary)
            Text(entry.healthLabel).font(.title2.bold())
            Text("\(entry.failingPods) failing").font(.caption2)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private var mediumView: some View {
        HStack {
            VStack(alignment: .leading) {
                Text(entry.clusterName).font(.caption)
                Text(entry.healthLabel).font(.title.bold())
            }
            Spacer()
            VStack(alignment: .trailing) {
                Text("\(entry.crashloopPods) crashloop")
                Text("\(entry.alerts) alerts")
            }
            .font(.caption)
        }
    }

    private var largeView: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("KubePilot").font(.caption).foregroundStyle(.secondary)
            Text(entry.clusterName).font(.headline)
            Text(entry.healthLabel).font(.largeTitle.bold())
            HStack {
                metric("Failing", entry.failingPods)
                metric("CrashLoop", entry.crashloopPods)
                metric("Alerts", entry.alerts)
            }
        }
    }

    private func metric(_ label: String, _ value: Int) -> some View {
        VStack {
            Text("\(value)").font(.title3.bold())
            Text(label).font(.caption2)
        }
        .frame(maxWidth: .infinity)
    }
}

struct ClusterHealthWidget: Widget {
    let kind = "ClusterHealth"

    var body: some WidgetConfiguration {
        StaticConfiguration(kind: kind, provider: ClusterHealthProvider()) { entry in
            ClusterHealthWidgetView(entry: entry)
                .containerBackground(.fill.tertiary, for: .widget)
        }
        .configurationDisplayName("Cluster Health")
        .description("Monitor production cluster health at a glance.")
        .supportedFamilies([.systemSmall, .systemMedium, .systemLarge])
    }
}

@main
struct KubePilotWidgetsBundle: WidgetBundle {
    var body: some Widget {
        ClusterHealthWidget()
    }
}
