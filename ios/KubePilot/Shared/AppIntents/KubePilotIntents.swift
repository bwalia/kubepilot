import AppIntents
import Foundation

struct ShowProductionClusterIntent: AppIntent {
    nonisolated(unsafe) static var title: LocalizedStringResource = "Show Production Cluster"
    nonisolated(unsafe) static var description = IntentDescription("Open the KubePilot production cluster dashboard.")
    nonisolated(unsafe) static var openAppWhenRun = true

    func perform() async throws -> some IntentResult {
        .result()
    }
}

struct ShowFailedPodsIntent: AppIntent {
    nonisolated(unsafe) static var title: LocalizedStringResource = "Show Failed Pods"
    nonisolated(unsafe) static var description = IntentDescription("List pods that are failing or not ready.")
    nonisolated(unsafe) static var openAppWhenRun = true

    func perform() async throws -> some IntentResult {
        .result()
    }
}

struct ClusterHealthSummaryIntent: AppIntent {
    nonisolated(unsafe) static var title: LocalizedStringResource = "Cluster Health Summary"
    nonisolated(unsafe) static var description = IntentDescription("Get an AI summary of current cluster health.")

    func perform() async throws -> some IntentResult & ReturnsValue<String> {
        do {
            let summary = try await KubePilotService.shared.fetchTroubleshootingSummary()
            let h = summary.healthSummary
            let text = "CrashLoop: \(h.crashloopPods), Pending: \(h.pendingPods), Not-ready nodes: \(h.notReadyNodes)"
            return .result(value: text)
        } catch {
            return .result(value: "Unable to fetch cluster health: \(error.localizedDescription)")
        }
    }
}

struct KubePilotShortcuts: AppShortcutsProvider {
    @AppShortcutsBuilder
    static var appShortcuts: [AppShortcut] {
        AppShortcut(
            intent: ShowProductionClusterIntent(),
            phrases: ["Show production cluster in \(.applicationName)", "Open \(.applicationName) production"],
            shortTitle: "Production Dashboard",
            systemImageName: "gauge.with.dots.needle.67percent"
        )
        AppShortcut(
            intent: ShowFailedPodsIntent(),
            phrases: ["Show failed pods in \(.applicationName)", "Why are pods failing in \(.applicationName)"],
            shortTitle: "Failed Pods",
            systemImageName: "exclamationmark.triangle"
        )
        AppShortcut(
            intent: ClusterHealthSummaryIntent(),
            phrases: ["Summarise cluster health with \(.applicationName)"],
            shortTitle: "Health Summary",
            systemImageName: "sparkles"
        )
    }
}
