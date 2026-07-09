import SwiftUI

struct AlertsView: View {
    @State private var viewModel = AlertsViewModel()
    @State private var filter: EventFilter = .all

    var body: some View {
        NavigationStack {
            VStack(spacing: 0) {
                Picker("Filter", selection: $filter) {
                    ForEach(EventFilter.allCases, id: \.self) { f in
                        Text(f.title).tag(f)
                    }
                }
                .pickerStyle(.segmented)
                .padding()

                if viewModel.isLoading && viewModel.events.isEmpty && viewModel.anomalies.isEmpty {
                    LoadingOverlay(message: "Loading alerts…")
                } else {
                    List {
                        if !viewModel.anomalies.isEmpty {
                            Section("Detected Anomalies") {
                                ForEach(viewModel.anomalies) { anomaly in
                                    AnomalyRow(anomaly: anomaly)
                                }
                            }
                        }
                        Section("Cluster Events") {
                            ForEach(viewModel.filteredEvents(filter)) { event in
                                EventRow(event: event)
                            }
                        }
                        if !viewModel.rcaReports.isEmpty {
                            Section("RCA Reports") {
                                ForEach(viewModel.rcaReports) { report in
                                    NavigationLink {
                                        RCADetailView(report: report)
                                    } label: {
                                        RCARow(report: report)
                                    }
                                }
                            }
                        }
                    }
                }
            }
            .navigationTitle("Alerts")
            .refreshable { await viewModel.load() }
            .task { await viewModel.load() }
        }
    }
}

enum EventFilter: CaseIterable {
    case all, warnings, errors, normal

    var title: String {
        switch self {
        case .all: "All"
        case .warnings: "Warnings"
        case .errors: "Errors"
        case .normal: "Normal"
        }
    }
}

struct EventRow: View {
    let event: KubeEvent

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack {
                StatusBadge(
                    text: event.type,
                    color: event.type == "Warning" ? Theme.warning : Theme.accent
                )
                Text(event.reason).font(.subheadline.weight(.semibold))
                Spacer()
                Text("×\(event.count)").font(.caption).foregroundStyle(.secondary)
            }
            Text(event.message)
                .font(.caption)
                .foregroundStyle(.secondary)
                .lineLimit(3)
            Text("\(event.involvedObject.namespace)/\(event.involvedObject.kind)/\(event.involvedObject.name)")
                .font(.caption2)
                .foregroundStyle(.tertiary)
        }
        .padding(.vertical, 4)
    }
}

struct AnomalyRow: View {
    let anomaly: Anomaly

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                StatusBadge(text: anomaly.severity.uppercased(), color: Theme.danger)
                Text(anomaly.rule).font(.subheadline.weight(.semibold))
            }
            Text(anomaly.description).font(.caption).foregroundStyle(.secondary)
            Text(anomaly.resource).font(.caption2).foregroundStyle(.tertiary)
        }
    }
}

struct RCARow: View {
    let report: RCAReport

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(report.targetResource).font(.subheadline.weight(.semibold))
            Text(report.rootCause).font(.caption).foregroundStyle(.secondary).lineLimit(2)
            HStack {
                StatusBadge(text: report.severity, color: Theme.warning)
                Text("\(Int(report.confidence * 100))% confidence")
                    .font(.caption2)
                    .foregroundStyle(.tertiary)
            }
        }
    }
}

struct RCADetailView: View {
    let report: RCAReport

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                Group {
                    Text("Root Cause").font(.headline)
                    Text(report.rootCause)
                }
                if !report.evidenceChain.isEmpty {
                    Text("Evidence").font(.headline)
                    ForEach(report.evidenceChain, id: \.self) { item in
                        Text("• \(item)").font(.caption)
                    }
                }
                if !report.remediation.isEmpty {
                    Text("Remediation").font(.headline)
                    ForEach(report.remediation, id: \.self) { step in
                        Text("→ \(step)").font(.subheadline)
                    }
                }
            }
            .padding()
        }
        .navigationTitle("RCA")
    }
}

@MainActor
@Observable
final class AlertsViewModel {
    var events: [KubeEvent] = []
    var anomalies: [Anomaly] = []
    var rcaReports: [RCAReport] = []
    var isLoading = false

    func filteredEvents(_ filter: EventFilter) -> [KubeEvent] {
        switch filter {
        case .all: return events
        case .warnings: return events.filter { $0.type == "Warning" }
        case .errors: return events.filter { $0.reason.localizedCaseInsensitiveContains("error") || $0.reason.localizedCaseInsensitiveContains("failed") }
        case .normal: return events.filter { $0.type == "Normal" }
        }
    }

    func load() async {
        isLoading = true
        defer { isLoading = false }
        async let eventsTask = KubePilotService.shared.fetchEvents(limit: 200)
        async let anomaliesTask = KubePilotService.shared.fetchAnomalies()
        async let rcaTask = KubePilotService.shared.fetchRCAReports()
        events = (try? await eventsTask)?.items ?? []
        anomalies = (try? await anomaliesTask) ?? []
        rcaReports = (try? await rcaTask) ?? []
    }
}
