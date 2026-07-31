import SwiftUI

struct AIAssistantView: View {
    @Environment(AppState.self) private var appState

    var body: some View {
        let vm = appState.aiAssistant
        NavigationStack {
            ZStack {
                BrandScreenBackground()

                VStack(spacing: 0) {
                    ScrollViewReader { proxy in
                        ScrollView {
                            LazyVStack(spacing: Theme.spacingSM) {
                                ForEach(vm.messages) { message in
                                    ChatBubble(message: message)
                                        .id(message.id)
                                }
                                if vm.isSending {
                                    HStack {
                                        ProgressView()
                                            .tint(Theme.accent)
                                        Text("Thinking…")
                                            .font(.caption)
                                            .foregroundStyle(Theme.muted)
                                        Spacer()
                                    }
                                    .padding(.horizontal, Theme.spacingMD)
                                }
                            }
                            .padding(Theme.spacingMD)
                        }
                        .onChange(of: vm.messages.count) { _, _ in
                            if let last = vm.messages.last {
                                withAnimation(.snappy) { proxy.scrollTo(last.id, anchor: .bottom) }
                            }
                        }
                    }

                    suggestionChips(vm: vm)

                    ChatComposerBar(
                        text: Bindable(vm).inputText,
                        placeholder: "Ask KubePilot AI…",
                        isSending: vm.isSending
                    ) {
                        Task { await vm.send() }
                    }
                }
            }
            .navigationTitle("AI Assistant")
            .navigationBarTitleDisplayMode(.large)
            .themedScreen()
            .onAppear {
                if vm.messages.isEmpty {
                    vm.seedWelcome()
                }
            }
        }
    }

    @ViewBuilder
    private func suggestionChips(vm: AIAssistantViewModel) -> some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: Theme.spacingSM) {
                ForEach(vm.suggestions, id: \.self) { suggestion in
                    Button(suggestion) {
                        vm.inputText = suggestion
                        Task { await vm.send() }
                    }
                    .buttonStyle(.bordered)
                    .tint(Theme.accent)
                    .font(.caption.weight(.medium))
                }
            }
            .padding(.horizontal, Theme.spacingMD)
            .padding(.bottom, Theme.spacingSM)
        }
    }
}

struct ChatBubble: View {
    let message: AIChatMessage

    var body: some View {
        HStack {
            if message.role == .user { Spacer(minLength: 48) }
            VStack(alignment: message.role == .user ? .trailing : .leading, spacing: Theme.spacingSM) {
                Text(message.content)
                    .font(.subheadline)
                    .foregroundStyle(message.role == .user ? Theme.textPrimary : Theme.textSecondary)
                    .padding(Theme.spacingMD)
                    .background(bubbleBackground, in: RoundedRectangle(cornerRadius: Theme.cornerRadius, style: .continuous))
                    .overlay(
                        RoundedRectangle(cornerRadius: Theme.cornerRadius, style: .continuous)
                            .stroke(message.role == .user ? Theme.accent.opacity(0.35) : Theme.brandBorder, lineWidth: 1)
                    )
                    .textSelection(.enabled)

                if !message.actions.isEmpty {
                    ForEach(message.actions) { action in
                        Label(action.explanation, systemImage: "arrow.right.circle")
                            .font(.caption)
                            .foregroundStyle(Theme.muted)
                    }
                }

                if message.role == .assistant && !message.content.isEmpty {
                    AIReportCopyBar(outputText: message.outputText, fixPrompt: message.fixPrompt)
                }
            }
            if message.role != .user { Spacer(minLength: 48) }
        }
    }

    private var bubbleBackground: Color {
        message.role == .user ? Theme.accent.opacity(0.18) : Theme.surface
    }
}

@MainActor
@Observable
final class AIAssistantViewModel {
    var messages: [AIChatMessage] = []
    var inputText = ""
    var isSending = false

    let suggestions = [
        "Summarise cluster health",
        "Why are pods restarting?",
        "Show failed pods",
        "Explain OOMKilled",
        "Find networking issues"
    ]

    func seedWelcome() {
        messages = [
            AIChatMessage(
                role: .assistant,
                content: "I'm your KubePilot AI assistant. Ask me to investigate namespaces, explain failures, or summarise cluster health."
            )
        ]
    }

    func send() async {
        let prompt = inputText.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !prompt.isEmpty else { return }
        inputText = ""
        messages.append(AIChatMessage(role: .user, content: prompt))
        isSending = true
        defer { isSending = false }

        do {
            let response = try await KubePilotService.shared.interpretCommand(prompt)
            let text = response.actions.isEmpty
                ? "I couldn't map that to a specific action yet. Try asking about cluster health, failing pods, or a specific namespace."
                : response.actions.map(\.explanation).joined(separator: "\n\n")
            messages.append(AIChatMessage(role: .assistant, content: text, actions: response.actions))
        } catch {
            if let summary = try? await KubePilotService.shared.fetchTroubleshootingSummary() {
                let text = buildSummaryReply(summary, prompt: prompt)
                messages.append(AIChatMessage(role: .assistant, content: text))
            } else {
                messages.append(AIChatMessage(role: .assistant, content: "Error: \(error.localizedDescription)"))
            }
        }
    }

    private func buildSummaryReply(_ summary: ClusterTroubleshootingSummary, prompt: String) -> String {
        let h = summary.healthSummary
        var parts = [
            "Cluster snapshot",
            "• CrashLoop pods: \(h.crashloopPods)",
            "• Pending pods: \(h.pendingPods)",
            "• Not-ready nodes: \(h.notReadyNodes)",
            "• Warning events: \(h.warningEvents)"
        ]
        if !summary.insights.isEmpty {
            parts.append("\nTop insight: \(summary.insights[0].title) — \(summary.insights[0].summary)")
        }
        if !h.recommendedActions.isEmpty {
            parts.append("\nRecommended: \(h.recommendedActions.joined(separator: "; "))")
        }
        return parts.joined(separator: "\n")
    }
}
