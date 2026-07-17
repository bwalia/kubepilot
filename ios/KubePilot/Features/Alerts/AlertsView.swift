import SwiftUI

struct AlertsView: View {
    @State private var viewModel = AlertsViewModel()
    @State private var filter: EventFilter = .all

    private var hasContent: Bool {
        !viewModel.anomalies.isEmpty
            || !viewModel.filteredEvents(filter).isEmpty
            || !viewModel.rcaReports.isEmpty
    }

    var body: some View {
        NavigationStack {
            ZStack {
                BrandScreenBackground()

                VStack(spacing: Theme.spacingSM) {
                    FilterChipBar(
                        items: Array(EventFilter.allCases),
                        selection: $filter,
                        title: { $0.title }
                    )
                    .padding(.top, Theme.spacingSM)

                    content
                }
            }
            .navigationTitle("Alerts")
            .navigationBarTitleDisplayMode(.large)
            .themedScreen()
            .refreshable { await viewModel.load() }
            .task { await viewModel.load() }
        }
    }

    @ViewBuilder
    private var content: some View {
        if viewModel.isLoading && !hasContent {
            LoadingOverlay(message: "Loading alerts…")
        } else if let error = viewModel.errorMessage, !hasContent {
            ErrorBanner(message: error) {
                Task { await viewModel.load() }
            }
        } else if !hasContent {
            EmptyStateView(
                title: "All Clear",
                systemImage: "bell.slash",
                description: "No anomalies, events, or RCA reports match this filter."
            )
        } else {
            List {
                if !viewModel.anomalies.isEmpty {
                    Section {
                        ForEach(viewModel.anomalies) { anomaly in
                            AnomalyRow(anomaly: anomaly)
                                .listRowBackground(Theme.surface)
                        }
                    } header: {
                        Text("Detected Anomalies")
                    }
                }

                let events = viewModel.filteredEvents(filter)
                if !events.isEmpty {
                    Section {
                        ForEach(events) { event in
                            EventRow(event: event)
                                .listRowBackground(Theme.surface)
                        }
                    } header: {
                        Text("Cluster Events")
                    }
                }

                if !viewModel.rcaReports.isEmpty {
                    Section {
                        ForEach(viewModel.rcaReports) { report in
                            NavigationLink {
                                RCADetailView(report: report)
                            } label: {
                                RCARow(report: report)
                            }
                            .listRowBackground(Theme.surface)
                        }
                    } header: {
                        Text("RCA Reports")
                    }
                }
            }
            .themedList()
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
        VStack(alignment: .leading, spacing: Theme.spacingXS) {
            HStack {
                StatusBadge(
                    text: event.type,
                    color: event.type == "Warning" ? Theme.warning : Theme.accent
                )
                Text(event.reason)
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(Theme.textPrimary)
                Spacer()
                Text("×\(event.count)")
                    .font(.caption)
                    .foregroundStyle(Theme.muted)
            }
            Text(event.message)
                .font(.caption)
                .foregroundStyle(Theme.textSecondary)
                .lineLimit(3)
            Text("\(event.involvedObject.namespace)/\(event.involvedObject.kind)/\(event.involvedObject.name)")
                .font(.caption2)
                .foregroundStyle(Theme.muted)
        }
        .padding(.vertical, Theme.spacingXS)
    }
}

struct AnomalyRow: View {
    let anomaly: Anomaly

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.spacingXS) {
            HStack {
                StatusBadge(text: anomaly.severity.uppercased(), color: Theme.danger)
                Text(anomaly.rule)
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(Theme.textPrimary)
            }
            Text(anomaly.description)
                .font(.caption)
                .foregroundStyle(Theme.textSecondary)
            Text(anomaly.resource.displayString)
                .font(.caption2)
                .foregroundStyle(Theme.muted)
        }
    }
}

struct RCARow: View {
    let report: RCAReport

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.spacingXS) {
            Text(report.targetResource)
                .font(.subheadline.weight(.semibold))
                .foregroundStyle(Theme.textPrimary)
            Text(report.rootCause)
                .font(.caption)
                .foregroundStyle(Theme.textSecondary)
                .lineLimit(2)
            HStack {
                StatusBadge(text: report.severity, color: Theme.warning)
                Text("\(Int(report.confidence * 100))% confidence")
                    .font(.caption2)
                    .foregroundStyle(Theme.muted)
            }
        }
    }
}

struct RCADetailView: View {
    let report: RCAReport

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Theme.spacingMD) {
                SurfaceCard {
                    VStack(alignment: .leading, spacing: Theme.spacingSM) {
                        SectionHeader(title: "Root Cause")
                        Text(report.rootCause)
                            .foregroundStyle(Theme.textSecondary)
                    }
                }

                if !report.evidenceChain.isEmpty {
                    SurfaceCard {
                        VStack(alignment: .leading, spacing: Theme.spacingSM) {
                            SectionHeader(title: "Evidence")
                            ForEach(report.evidenceChain, id: \.self) { item in
                                Label(item, systemImage: "checkmark.circle")
                                    .font(.caption)
                                    .foregroundStyle(Theme.textSecondary)
                            }
                        }
                    }
                }

                if !report.remediation.isEmpty {
                    SurfaceCard {
                        VStack(alignment: .leading, spacing: Theme.spacingSM) {
                            SectionHeader(title: "Remediation")
                            ForEach(report.remediation, id: \.self) { step in
                                Label(step, systemImage: "arrow.turn.down.right")
                                    .font(.subheadline)
                                    .foregroundStyle(Theme.textPrimary)
                            }
                        }
                    }
                }
            }
            .padding(Theme.spacingMD)
        }
        .background(Theme.background)
        .navigationTitle("RCA")
        .themedScreen()
    }
}

@MainActor
@Observable
final class AlertsViewModel {
    var events: [KubeEvent] = []
    var anomalies: [Anomaly] = []
    var rcaReports: [RCAReport] = []
    var isLoading = false
    var errorMessage: String?

    func filteredEvents(_ filter: EventFilter) -> [KubeEvent] {
        switch filter {
        case .all: return events
        case .warnings: return events.filter { $0.type == "Warning" }
        case .errors: return events.filter {
            $0.reason.localizedCaseInsensitiveContains("error")
                || $0.reason.localizedCaseInsensitiveContains("failed")
        }
        case .normal: return events.filter { $0.type == "Normal" }
        }
    }

    func load() async {
        isLoading = true
        errorMessage = nil
        defer { isLoading = false }
        do {
            async let eventsTask = KubePilotService.shared.fetchEvents(limit: 200)
            async let anomaliesTask = KubePilotService.shared.fetchAnomalies()
            async let rcaTask = KubePilotService.shared.fetchRCAReports()
            events = try await eventsTask.items
            anomalies = try await anomaliesTask
            rcaReports = try await rcaTask
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}
