import SwiftUI

@MainActor
@Observable
final class AutopilotViewModel {
    var status: AutopilotStatus?
    var isLoading = false
    var isMutating = false
    var errorMessage: String?
    var notConfigured = false
    var actionError: String?

    func load() async {
        isLoading = status == nil
        errorMessage = nil
        do {
            status = try await KubePilotService.shared.fetchAutopilotStatus()
            notConfigured = false
        } catch {
            handleLoadError(error)
        }
        isLoading = false
    }

    func setMode(_ mode: AutopilotMode) async {
        await mutate { try await KubePilotService.shared.setAutopilotMode(mode) }
    }

    func pause() async {
        await mutate { try await KubePilotService.shared.pauseAutopilot() }
    }

    func resume() async {
        await mutate { try await KubePilotService.shared.resumeAutopilot() }
    }

    private func mutate(_ op: @escaping () async throws -> AutopilotStatus) async {
        isMutating = true
        actionError = nil
        do {
            status = try await op()
        } catch {
            actionError = Self.message(for: error)
        }
        isMutating = false
    }

    private func handleLoadError(_ error: Error) {
        // 503 from the server means autopilot isn't configured — a normal state,
        // not a failure. Show guidance instead of an error banner.
        if case APIError.serverError(let code, _) = error, code == 503 {
            notConfigured = true
            errorMessage = nil
            return
        }
        errorMessage = Self.message(for: error)
    }

    static func message(for error: Error) -> String {
        error.localizedDescription
    }
}

struct AutopilotView: View {
    @State private var model = AutopilotViewModel()
    @State private var confirmActivate = false

    var body: some View {
        NavigationStack {
            Group {
                if model.isLoading && model.status == nil {
                    LoadingOverlay(message: "Loading Autopilot…")
                } else if model.notConfigured {
                    notConfiguredState
                } else if let message = model.errorMessage, model.status == nil {
                    ErrorBanner(message: message) { Task { await model.load() } }
                } else if let status = model.status {
                    content(status)
                }
            }
            .navigationTitle("Autopilot")
            .themedScreen()
        }
        .task { await model.load() }
        .refreshable { await model.load() }
        .alert("Enable active remediation?", isPresented: $confirmActivate) {
            Button("Cancel", role: .cancel) {}
            Button("Activate", role: .destructive) { Task { await model.setMode(.active) } }
        } message: {
            Text("Autopilot will apply safe remediations automatically, within the policy limits. You can pause it at any time.")
        }
    }

    // MARK: Content

    private func content(_ status: AutopilotStatus) -> some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Theme.spacingMD) {
                ClusterContextBanner()

                if let actionError = model.actionError {
                    Text(actionError)
                        .font(.caption)
                        .foregroundStyle(Theme.danger)
                        .padding(Theme.spacingSM)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .background(Theme.danger.opacity(0.1), in: RoundedRectangle(cornerRadius: Theme.cornerRadiusSmall))
                }

                modeCard(status)
                if status.policy != nil { killSwitch(status) }
                statsCard(status)
                if let policy = status.policy { policyCard(policy) }
                decisionsSection(status)
            }
            .padding(Theme.spacingMD)
        }
    }

    private func modeCard(_ status: AutopilotStatus) -> some View {
        SurfaceCard {
            VStack(alignment: .leading, spacing: Theme.spacingMD) {
                HStack {
                    SectionHeader(title: "Mode")
                    Spacer()
                    StatusBadge(text: status.mode.label, color: modeColor(status.mode))
                }
                Text(status.mode.subtitle)
                    .font(.caption)
                    .foregroundStyle(Theme.muted)

                HStack(spacing: Theme.spacingSM) {
                    ForEach(AutopilotMode.allCases, id: \.self) { mode in
                        modeButton(mode, current: status.mode)
                    }
                }
                .disabled(model.isMutating)
                .opacity(model.isMutating ? 0.5 : 1)
            }
        }
    }

    private func modeButton(_ mode: AutopilotMode, current: AutopilotMode) -> some View {
        let selected = mode == current
        return Button {
            if mode == .active {
                confirmActivate = true
            } else {
                Task { await model.setMode(mode) }
            }
        } label: {
            Text(mode.label)
                .font(.subheadline.weight(.semibold))
                .frame(maxWidth: .infinity)
                .padding(.vertical, 10)
                .background(
                    selected ? modeColor(mode).opacity(0.2) : Theme.surfaceElevated,
                    in: RoundedRectangle(cornerRadius: Theme.cornerRadiusSmall)
                )
                .overlay(
                    RoundedRectangle(cornerRadius: Theme.cornerRadiusSmall)
                        .stroke(selected ? modeColor(mode) : Theme.brandBorder, lineWidth: selected ? 1.5 : 1)
                )
                .foregroundStyle(selected ? modeColor(mode) : Theme.textSecondary)
        }
        .buttonStyle(.plain)
    }

    private func killSwitch(_ status: AutopilotStatus) -> some View {
        let paused = !status.enabled
        return Button {
            Task { paused ? await model.resume() : await model.pause() }
        } label: {
            Label(paused ? "Resume Autopilot" : "Pause Autopilot",
                  systemImage: paused ? "play.fill" : "pause.fill")
                .font(.subheadline.weight(.semibold))
                .frame(maxWidth: .infinity)
                .padding(.vertical, 12)
                .background((paused ? Theme.success : Theme.warning).opacity(0.15),
                           in: RoundedRectangle(cornerRadius: Theme.cornerRadiusSmall))
                .foregroundStyle(paused ? Theme.success : Theme.warning)
        }
        .buttonStyle(.plain)
        .disabled(model.isMutating)
    }

    private func statsCard(_ status: AutopilotStatus) -> some View {
        SurfaceCard {
            VStack(alignment: .leading, spacing: Theme.spacingSM) {
                SectionHeader(title: "Activity")
                let cols = [GridItem(.flexible()), GridItem(.flexible()), GridItem(.flexible())]
                LazyVGrid(columns: cols, spacing: Theme.spacingSM) {
                    stat("Executed", status.stat("executed"), Theme.success)
                    stat("Dry-run", status.stat("dry-run"), Theme.accent)
                    stat("Skipped", status.stat("skipped"), Theme.muted)
                    stat("Escalated", status.stat("escalated"), Theme.warning)
                    stat("Failed", status.stat("failed"), Theme.danger)
                    stat("Last hour", status.stat("actions_last_hour"), Theme.textSecondary)
                }
            }
        }
    }

    private func stat(_ label: String, _ value: Int, _ color: Color) -> some View {
        VStack(spacing: 2) {
            Text("\(value)")
                .font(.title3.weight(.bold).monospacedDigit())
                .foregroundStyle(color)
            Text(label)
                .font(.caption2)
                .foregroundStyle(Theme.muted)
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, 8)
        .background(Theme.surfaceElevated, in: RoundedRectangle(cornerRadius: Theme.cornerRadiusSmall))
    }

    private func policyCard(_ policy: AutopilotPolicy) -> some View {
        SurfaceCard {
            VStack(alignment: .leading, spacing: Theme.spacingSM) {
                SectionHeader(title: "Policy")
                policyRow("Min confidence", "\(Int(policy.minConfidence * 100))%")
                policyRow("Max risk", policy.maxRisk.capitalized)
                policyRow("Cooldown", policy.cooldownDescription)
                policyRow("Max actions / hour", "\(policy.maxActionsPerHour)")
                if !policy.allowedActions.isEmpty {
                    policyRow("Allowed actions", policy.allowedActions.joined(separator: ", "))
                }
                if let blocked = policy.blockedNamespaces, !blocked.isEmpty {
                    policyRow("Blocked namespaces", blocked.joined(separator: ", "))
                }
            }
        }
    }

    private func policyRow(_ label: String, _ value: String) -> some View {
        HStack(alignment: .top) {
            Text(label)
                .font(.caption)
                .foregroundStyle(Theme.muted)
            Spacer()
            Text(value)
                .font(.caption.weight(.medium))
                .foregroundStyle(Theme.textSecondary)
                .multilineTextAlignment(.trailing)
        }
    }

    private func decisionsSection(_ status: AutopilotStatus) -> some View {
        VStack(alignment: .leading, spacing: Theme.spacingSM) {
            SectionHeader(title: "Decisions")
            if status.decisions.isEmpty {
                Text("No decisions recorded yet. When Autopilot evaluates an RCA report, its verdict appears here.")
                    .font(.caption)
                    .foregroundStyle(Theme.muted)
                    .padding(.vertical, Theme.spacingSM)
            } else {
                ForEach(status.decisions.prefix(50)) { decision in
                    decisionRow(decision)
                }
            }
        }
    }

    private func decisionRow(_ d: AutopilotDecision) -> some View {
        SurfaceCard(cornerRadius: Theme.cornerRadiusSmall) {
            VStack(alignment: .leading, spacing: 6) {
                HStack {
                    Text(d.resource.displayString)
                        .font(.subheadline.weight(.semibold))
                        .foregroundStyle(Theme.textPrimary)
                    Spacer()
                    StatusBadge(text: d.verdict, color: verdictColor(d.verdict))
                }
                Text(d.rootCause)
                    .font(.caption)
                    .foregroundStyle(Theme.textSecondary)
                    .lineLimit(3)
                HStack(spacing: Theme.spacingSM) {
                    if let action = d.action, !action.isEmpty {
                        Label(action, systemImage: "wrench.and.screwdriver")
                    }
                    Label("\(Int(d.confidence * 100))%", systemImage: "gauge.medium")
                    Text(d.time, style: .relative)
                }
                .font(.caption2)
                .foregroundStyle(Theme.muted)
            }
        }
    }

    private var notConfiguredState: some View {
        EmptyStateView(
            title: "Autopilot not configured",
            systemImage: "autostartstop",
            description: "Enable Autopilot on the server (set autopilot_mode to dry-run or active in the KubePilot config) to see decisions and control self-healing here."
        )
    }

    // MARK: Colours

    private func modeColor(_ mode: AutopilotMode) -> Color {
        switch mode {
        case .off: return Theme.muted
        case .dryRun: return Theme.accent
        case .active: return Theme.success
        }
    }

    private func verdictColor(_ verdict: String) -> Color {
        switch verdict.lowercased() {
        case "executed": return Theme.success
        case "dry-run": return Theme.accent
        case "escalated": return Theme.warning
        case "failed": return Theme.danger
        default: return Theme.muted
        }
    }
}
