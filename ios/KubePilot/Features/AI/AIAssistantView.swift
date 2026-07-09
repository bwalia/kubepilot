import SwiftUI

struct AIAssistantView: View {
    @Environment(AppState.self) private var appState

    var body: some View {
        let vm = appState.aiAssistant
        NavigationStack {
            VStack(spacing: 0) {
                ScrollViewReader { proxy in
                    ScrollView {
                        LazyVStack(spacing: 12) {
                            ForEach(vm.messages) { message in
                                ChatBubble(message: message)
                                    .id(message.id)
                            }
                        }
                        .padding()
                    }
                    .onChange(of: vm.messages.count) { _, _ in
                        if let last = vm.messages.last {
                            withAnimation { proxy.scrollTo(last.id, anchor: .bottom) }
                        }
                    }
                }

                suggestionChips(vm: vm)

                HStack(spacing: 8) {
                    TextField("Ask KubePilot AI…", text: Bindable(vm).inputText, axis: .vertical)
                        .textFieldStyle(.roundedBorder)
                        .lineLimit(1...4)
                    Button {
                        Task { await vm.send() }
                    } label: {
                        Image(systemName: "arrow.up.circle.fill")
                            .font(.title2)
                    }
                    .disabled(vm.inputText.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty || vm.isSending)
                }
                .padding()
                .background(.bar)
            }
            .navigationTitle("AI Assistant")
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
            HStack {
                ForEach(vm.suggestions, id: \.self) { suggestion in
                    Button(suggestion) {
                        vm.inputText = suggestion
                        Task { await vm.send() }
                    }
                    .buttonStyle(.bordered)
                    .font(.caption)
                }
            }
            .padding(.horizontal)
        }
    }
}

struct ChatBubble: View {
    let message: AIChatMessage

    var body: some View {
        HStack {
            if message.role == .user { Spacer(minLength: 40) }
            VStack(alignment: message.role == .user ? .trailing : .leading, spacing: 8) {
                Text(message.content)
                    .font(.subheadline)
                    .padding(12)
                    .background(
                        message.role == .user ? Theme.accent.opacity(0.2) : Theme.surface,
                        in: RoundedRectangle(cornerRadius: 16, style: .continuous)
                    )

                if !message.actions.isEmpty {
                    ForEach(message.actions) { action in
                        Text("→ \(action.explanation)")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
            }
            if message.role != .user { Spacer(minLength: 40) }
        }
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
            // Fallback: synthesise from troubleshooting summary
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
            "**Cluster snapshot**",
            "• CrashLoop pods: \(h.crashloopPods)",
            "• Pending pods: \(h.pendingPods)",
            "• Not-ready nodes: \(h.notReadyNodes)",
            "• Warning events: \(h.warningEvents)"
        ]
        if !summary.insights.isEmpty {
            parts.append("\n**Top insight:** \(summary.insights[0].title) — \(summary.insights[0].summary)")
        }
        if !h.recommendedActions.isEmpty {
            parts.append("\n**Recommended:** \(h.recommendedActions.joined(separator: "; "))")
        }
        return parts.joined(separator: "\n")
    }
}
