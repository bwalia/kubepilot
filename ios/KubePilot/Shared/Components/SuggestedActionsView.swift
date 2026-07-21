import SwiftUI

/// Renders AI-suggested remediation actions with an Execute button each.
/// Actions that require a change-request code prompt for one before running.
/// Execution calls POST /ai/execute-action; a 403 (mutations disabled on the
/// server) is surfaced inline rather than as a hard failure.
struct SuggestedActionsView: View {
    let actions: [SuggestedAction]

    @State private var runningID: String?
    @State private var results: [String: ActionOutcome] = [:]
    @State private var crAction: SuggestedAction?

    struct ActionOutcome {
        let ok: Bool
        let text: String
    }

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.spacingSM) {
            ForEach(actions) { action in
                row(action)
            }
        }
        .sheet(item: $crAction) { action in
            CRCodeSheet(action: action) { changeID, crCode in
                await run(action, changeID: changeID, crCode: crCode)
            }
        }
    }

    private func row(_ action: SuggestedAction) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(alignment: .top) {
                VStack(alignment: .leading, spacing: 4) {
                    Text(action.type.uppercased())
                        .font(.caption.weight(.bold))
                        .foregroundStyle(Theme.accent)
                    Text(action.explanation)
                        .font(.caption)
                        .foregroundStyle(Theme.textSecondary)
                    if let resource = action.resource, !resource.isEmpty {
                        Text(resource)
                            .font(.caption2.monospaced())
                            .foregroundStyle(Theme.muted)
                    }
                }
                Spacer(minLength: Theme.spacingSM)
                executeButton(action)
            }

            if let outcome = results[action.id] {
                Text(outcome.text)
                    .font(.caption2)
                    .foregroundStyle(outcome.ok ? Theme.success : Theme.danger)
            }
        }
        .padding()
        .background(Theme.surface, in: RoundedRectangle(cornerRadius: 10))
    }

    @ViewBuilder
    private func executeButton(_ action: SuggestedAction) -> some View {
        let running = runningID == action.id
        Button {
            if action.requiresCrCode {
                crAction = action
            } else {
                Task { await run(action) }
            }
        } label: {
            HStack(spacing: 6) {
                if running { ProgressView().controlSize(.mini) }
                Text(action.requiresCrCode ? "Authorize & Run" : "Execute")
                    .font(.caption.weight(.semibold))
            }
            .padding(.horizontal, 12)
            .padding(.vertical, 7)
            .background(Theme.success.opacity(0.15), in: Capsule())
            .foregroundStyle(Theme.success)
        }
        .buttonStyle(.plain)
        .disabled(running)
    }

    private func run(_ action: SuggestedAction, changeID: String? = nil, crCode: String? = nil) async {
        runningID = action.id
        defer { runningID = nil }
        do {
            let result = try await KubePilotService.shared.executeAction(action, changeID: changeID, crCode: crCode)
            results[action.id] = ActionOutcome(ok: result.didExecute, text: result.message)
        } catch let APIError.forbidden(message) {
            results[action.id] = ActionOutcome(ok: false, text: message.isEmpty ? "Action execution is disabled on this server." : message)
        } catch {
            results[action.id] = ActionOutcome(ok: false, text: error.localizedDescription)
        }
    }
}

/// Collects a change-id + CR code for a production-impacting action.
private struct CRCodeSheet: View {
    let action: SuggestedAction
    let onRun: (String, String) async -> Void

    @Environment(\.dismiss) private var dismiss
    @State private var changeID = ""
    @State private var crCode = ""
    @State private var submitting = false

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    Text(action.explanation)
                        .font(.subheadline)
                        .foregroundStyle(Theme.textSecondary)
                } header: {
                    Text("\(action.type.uppercased()) — requires change-request approval")
                }
                Section("Change request") {
                    TextField("Change ID", text: $changeID)
                        .autocorrectionDisabled()
                        .textInputAutocapitalization(.never)
                    SecureField("CR code", text: $crCode)
                }
                Section {
                    Button {
                        Task {
                            submitting = true
                            await onRun(changeID, crCode)
                            submitting = false
                            dismiss()
                        }
                    } label: {
                        HStack {
                            if submitting { ProgressView().controlSize(.small) }
                            Text("Authorize & Run")
                        }
                    }
                    .disabled(changeID.isEmpty || crCode.isEmpty || submitting)
                }
            }
            .navigationTitle("Authorize action")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
            }
        }
    }
}
