import SwiftUI

struct PodDetailView: View {
    let namespace: String
    let podName: String

    @State private var viewModel: PodDetailViewModel
    @State private var selectedTab: PodDetailTab = .overview

    init(namespace: String, podName: String) {
        self.namespace = namespace
        self.podName = podName
        _viewModel = State(initialValue: PodDetailViewModel(namespace: namespace, podName: podName))
    }

    var body: some View {
        VStack(spacing: 0) {
            Picker("Tab", selection: $selectedTab) {
                ForEach(PodDetailTab.allCases) { tab in
                    Text(tab.title).tag(tab)
                }
            }
            .pickerStyle(.segmented)
            .padding()

            TabView(selection: $selectedTab) {
                overviewTab.tag(PodDetailTab.overview)
                logsTab.tag(PodDetailTab.logs)
                eventsTab.tag(PodDetailTab.events)
                yamlTab.tag(PodDetailTab.yaml)
                aiTab.tag(PodDetailTab.ai)
            }
            .tabViewStyle(.page(indexDisplayMode: .never))
        }
        .navigationTitle(podName)
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                Button("Analyse with AI") {
                    selectedTab = .ai
                    Task { await viewModel.runAIAnalysis() }
                }
                .font(.subheadline.weight(.semibold))
            }
        }
        .task { await viewModel.load() }
        .refreshable { await viewModel.load() }
    }

    @ViewBuilder
    private var overviewTab: some View {
        ScrollView {
            if viewModel.isLoading && viewModel.diagnostics == nil {
                LoadingOverlay(message: "Loading pod details…")
            } else if let diag = viewModel.diagnostics {
                VStack(alignment: .leading, spacing: 16) {
                    infoRow("Phase", diag.phase)
                    infoRow("Node", diag.nodeName)
                    infoRow("Created", diag.createdAt)

                    if !diag.containerStatuses.isEmpty {
                        Text("Containers").font(.headline)
                        ForEach(diag.containerStatuses) { c in
                            VStack(alignment: .leading, spacing: 4) {
                                Text(c.name).font(.subheadline.weight(.semibold))
                                Text(c.image).font(.caption).foregroundStyle(.secondary)
                                HStack {
                                    StatusBadge(text: c.state, color: c.ready ? Theme.success : Theme.danger)
                                    Text("\(c.restartCount) restarts").font(.caption)
                                }
                            }
                            .padding()
                            .background(Theme.surface, in: RoundedRectangle(cornerRadius: 12))
                        }
                    }
                }
                .padding()
            } else if let error = viewModel.errorMessage {
                ErrorBanner(message: error) { Task { await viewModel.load() } }
            }
        }
    }

    private var logsTab: some View {
        LogViewerView(
            logs: viewModel.logs,
            isLive: viewModel.isTailingLogs,
            onRefresh: { await viewModel.refreshLogs() },
            onToggleLive: { viewModel.toggleLogTail() }
        )
    }

    private var eventsTab: some View {
        List(viewModel.diagnostics?.events ?? []) { event in
            VStack(alignment: .leading, spacing: 4) {
                HStack {
                    StatusBadge(text: event.type, color: event.type == "Warning" ? Theme.warning : Theme.accent)
                    Text(event.reason).font(.subheadline.weight(.semibold))
                }
                Text(event.message).font(.caption).foregroundStyle(.secondary)
            }
        }
    }

    private var yamlTab: some View {
        ScrollView(.horizontal) {
            Text(viewModel.yaml)
                .font(.system(.caption, design: .monospaced))
                .padding()
                .frame(maxWidth: .infinity, alignment: .leading)
        }
        .task { await viewModel.loadYAML() }
    }

    private var aiTab: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                if viewModel.isAnalyzing {
                    LoadingOverlay(message: "AI is analysing pod context…")
                } else if let report = viewModel.aiReport {
                    AIAnalysisCard(report: report)
                } else {
                    Text("Tap **Analyse with AI** to investigate this pod with full context including logs, events, and workload metadata.")
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                }
            }
            .padding()
        }
    }

    private func infoRow(_ label: String, _ value: String) -> some View {
        HStack {
            Text(label).foregroundStyle(.secondary)
            Spacer()
            Text(value).fontWeight(.medium)
        }
    }
}

enum PodDetailTab: String, CaseIterable, Identifiable {
    case overview, logs, events, yaml, ai
    var id: String { rawValue }
    var title: String {
        switch self {
        case .overview: "Overview"
        case .logs: "Logs"
        case .events: "Events"
        case .yaml: "YAML"
        case .ai: "AI"
        }
    }
}

@MainActor
@Observable
final class PodDetailViewModel {
    let namespace: String
    let podName: String

    var diagnostics: PodDiagnostics?
    var logs = ""
    var yaml = "Loading…"
    var aiReport: TroubleshootReport?
    var isLoading = false
    var isAnalyzing = false
    var isTailingLogs = false
    var errorMessage: String?
    private var tailTask: Task<Void, Never>?

    init(namespace: String, podName: String) {
        self.namespace = namespace
        self.podName = podName
    }

    func load() async {
        isLoading = true
        errorMessage = nil
        defer { isLoading = false }
        do {
            let response = try await KubePilotService.shared.fetchPodDiagnostics(namespace: namespace, pod: podName)
            diagnostics = response.diagnostics
            logs = response.logs
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func refreshLogs() async {
        do {
            logs = try await KubePilotService.shared.fetchPodLogs(namespace: namespace, pod: podName)
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func loadYAML() async {
        do {
            yaml = try await KubePilotService.shared.fetchResourceYAML(kind: "pod", namespace: namespace, name: podName)
        } catch {
            yaml = "Failed to load YAML: \(error.localizedDescription)"
        }
    }

    func runAIAnalysis() async {
        isAnalyzing = true
        defer { isAnalyzing = false }
        do {
            aiReport = try await KubePilotService.shared.troubleshootPod(namespace: namespace, pod: podName)
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func toggleLogTail() {
        isTailingLogs.toggle()
        tailTask?.cancel()
        guard isTailingLogs else { return }
        tailTask = Task {
            while !Task.isCancelled && isTailingLogs {
                await refreshLogs()
                try? await Task.sleep(for: .seconds(3))
            }
        }
    }
}

struct AIAnalysisCard: View {
    let report: TroubleshootReport

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            Label("Executive Summary", systemImage: "doc.text.magnifyingglass")
                .font(.headline)
            Text(report.rootCause)
                .font(.subheadline)

            Label("Analysis", systemImage: "brain.head.profile")
                .font(.headline)
            Text(report.analysis)
                .font(.subheadline)
                .foregroundStyle(.secondary)

            if !report.actions.isEmpty {
                Label("Recommended Actions", systemImage: "wrench.and.screwdriver")
                    .font(.headline)
                ForEach(report.actions) { action in
                    VStack(alignment: .leading, spacing: 4) {
                        Text(action.type.uppercased())
                            .font(.caption.weight(.bold))
                            .foregroundStyle(Theme.accent)
                        Text(action.explanation)
                            .font(.caption)
                    }
                    .padding()
                    .background(Theme.surface, in: RoundedRectangle(cornerRadius: 10))
                }
            }
        }
    }
}
