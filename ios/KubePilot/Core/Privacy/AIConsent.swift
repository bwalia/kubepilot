import Foundation
import SwiftUI

/// The user's permission to send cluster data to the AI model behind their
/// KubePilot server (App Store guidelines 5.1.1(i) and 5.1.2(i)).
///
/// Consent is stored per server account and per disclosure version: bump
/// `version` whenever what is sent, or to whom, changes, so everyone is asked
/// again rather than being held to an answer they gave about something else.
enum AIConsent {
    static let version = 1

    private static func key(_ account: String) -> String {
        "kubepilot.aiConsent.v\(version).\(account)"
    }

    static func isGranted(account: String?) -> Bool {
        guard let account else { return false }
        return UserDefaults.standard.bool(forKey: key(account))
    }

    static func set(_ granted: Bool, account: String?) {
        guard let account else { return }
        UserDefaults.standard.set(granted, forKey: key(account))
    }
}

enum AIConsentError: LocalizedError {
    case notGranted

    var errorDescription: String? {
        "AI features are off. Allow data sharing with AI in Settings → AI & Privacy to use them."
    }
}

/// Where AI requests end up, derived from the server's /ai/health answer.
struct AIRecipient: Equatable {
    let serverHost: String
    let model: String?
    let endpointHost: String?

    /// The model runs on the KubePilot server itself (e.g. Ollama on localhost).
    var isOnServer: Bool {
        guard let host = endpointHost?.lowercased() else { return false }
        return host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0"
    }

    init(serverURL: URL?, health: AIHealthResponse?) {
        serverHost = serverURL?.host() ?? "your KubePilot server"
        model = health.map(\.model).flatMap { $0.isEmpty ? nil : $0 }
        endpointHost = health.flatMap { URL(string: $0.baseURL)?.host() }
    }
}

/// Runs AI actions only once the user has agreed to share data, presenting the
/// disclosure sheet first when they have not.
@MainActor
@Observable
final class AIConsentGate {
    var isPresenting = false
    private var pending: (@MainActor () async -> Void)?

    func run(_ action: @escaping @MainActor () async -> Void) {
        Task {
            let account = await KubePilotService.shared.currentAccountKey()
            if AIConsent.isGranted(account: account) {
                await action()
            } else {
                pending = action
                isPresenting = true
            }
        }
    }

    func decide(_ allowed: Bool) {
        isPresenting = false
        let action = pending
        pending = nil
        guard allowed, let action else { return }
        Task { await action() }
    }
}

extension View {
    /// Attaches the AI data-sharing disclosure that `gate.run` presents.
    func aiConsentSheet(_ gate: AIConsentGate) -> some View {
        sheet(isPresented: Binding(get: { gate.isPresenting }, set: { if !$0 { gate.decide(false) } })) {
            AIConsentSheet { gate.decide($0) }
        }
    }
}

/// Explains what is sent, who receives it, and asks for permission.
struct AIConsentSheet: View {
    @Environment(AppState.self) private var appState
    /// Called with the user's answer. Granting is recorded before it is called.
    let onDecision: (Bool) -> Void

    @State private var health: AIHealthResponse?
    @State private var loaded = false

    static let privacyPolicyURL = URL(string: "https://pages.kubepilot.org/privacy.html")!

    private var account: ServerAccount? { appState.authManager.activeAccount }
    private var recipient: AIRecipient { AIRecipient(serverURL: account?.baseURL, health: health) }

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: Theme.spacingMD) {
                    Image(systemName: "sparkles")
                        .font(.largeTitle)
                        .foregroundStyle(Theme.accent)
                    Text("Share cluster data with AI?")
                        .font(.title2.weight(.bold))
                        .foregroundStyle(Theme.textPrimary)
                    Text("AI analysis works by sending information about your cluster to an AI model. Nothing is sent unless you allow it.")
                        .font(.subheadline)
                        .foregroundStyle(Theme.textSecondary)

                    section("What is sent", systemImage: "doc.text") {
                        bullet("**AI Assistant:** the question you type, plus a summary of cluster state — names and status of failing pods, deployments and nodes under pressure.")
                        bullet("**Analyse with AI:** the pod's name, namespace, status, Kubernetes events, resource usage and up to its last 200 lines of container logs.")
                        bullet("Logs can include anything your applications print, which may contain personal data. Avoid AI analysis on workloads that log personal data if you do not want it shared.")
                    }

                    section("Who receives it", systemImage: "arrow.triangle.branch") {
                        bullet("**Your KubePilot server** at \(recipient.serverHost), which you or your organisation operate.")
                        if !loaded {
                            HStack(spacing: Theme.spacingSM) {
                                ProgressView()
                                Text("Checking which AI model the server uses…").font(.footnote).foregroundStyle(Theme.muted)
                            }
                        } else if let endpoint = recipient.endpointHost {
                            if recipient.isOnServer {
                                bullet("**The AI model \(recipient.model ?? "configured on the server")**, which runs on that same server. Your data is not sent to an outside AI company.")
                            } else {
                                bullet("**The AI service at \(endpoint)**\(recipient.model.map { " (model \($0))" } ?? ""), chosen by your server operator. This may be a third-party AI provider, which receives the data above and handles it under its own terms.")
                            }
                        } else {
                            bullet("**The AI model configured on that server** by your server operator. The server did not report which one, so it may be a third-party AI provider.")
                        }
                        bullet("KubePilot's developers do not receive this data, and it is never used for advertising or tracking.")
                    }

                    Text("You can change this at any time in Settings → AI & Privacy.")
                        .font(.footnote)
                        .foregroundStyle(Theme.muted)
                    Link("Read the Privacy Policy", destination: Self.privacyPolicyURL)
                        .font(.footnote.weight(.semibold))
                }
                .padding(Theme.spacingMD)
            }
            .safeAreaInset(edge: .bottom) {
                VStack(spacing: Theme.spacingSM) {
                    Button {
                        AIConsent.set(true, account: account?.id)
                        onDecision(true)
                    } label: {
                        Text("Allow").fontWeight(.semibold).frame(maxWidth: .infinity).padding(.vertical, 6)
                    }
                    .buttonStyle(.borderedProminent)
                    .tint(Theme.accent)

                    Button("Don't Allow") {
                        AIConsent.set(false, account: account?.id)
                        onDecision(false)
                    }
                    .foregroundStyle(Theme.textSecondary)
                }
                .padding(Theme.spacingMD)
                .background(.bar)
            }
            .navigationBarTitleDisplayMode(.inline)
        }
        .interactiveDismissDisabled()
        .task {
            // The health probe carries no cluster or personal data.
            health = try? await KubePilotService.shared.fetchAIHealth()
            loaded = true
        }
    }

    private func section<Content: View>(_ title: String, systemImage: String, @ViewBuilder content: () -> Content) -> some View {
        VStack(alignment: .leading, spacing: Theme.spacingSM) {
            Label(title, systemImage: systemImage)
                .font(.headline)
                .foregroundStyle(Theme.textPrimary)
            content()
        }
        .padding(Theme.spacingMD)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface, in: RoundedRectangle(cornerRadius: Theme.cornerRadius, style: .continuous))
    }

    private func bullet(_ text: LocalizedStringKey) -> some View {
        HStack(alignment: .firstTextBaseline, spacing: Theme.spacingSM) {
            Text("•").foregroundStyle(Theme.muted)
            Text(text).font(.subheadline).foregroundStyle(Theme.textSecondary)
        }
    }
}
