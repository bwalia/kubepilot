import SwiftUI

struct LogViewerView: View {
    let logs: String
    let isLive: Bool
    let onRefresh: () async -> Void
    let onToggleLive: () -> Void

    @State private var searchText = ""
    @State private var isPaused = false

    private var filteredLogs: String {
        guard !searchText.isEmpty else { return logs }
        return logs
            .components(separatedBy: .newlines)
            .filter { $0.localizedCaseInsensitiveContains(searchText) }
            .joined(separator: "\n")
    }

    var body: some View {
        VStack(spacing: 0) {
            HStack(spacing: 8) {
                SearchBar(text: $searchText, placeholder: "Search logs…")
                Button {
                    onToggleLive()
                } label: {
                    Image(systemName: isLive ? "dot.radiowaves.left.and.right" : "play.fill")
                }
                .buttonStyle(.bordered)
                .tint(isLive ? Theme.success : Theme.accent)

                ShareLink(item: logs) {
                    Image(systemName: "square.and.arrow.up")
                }
                .buttonStyle(.bordered)
            }
            .padding()

            ScrollView {
                ScrollViewReader { proxy in
                    Text(filteredLogs.isEmpty ? "No logs available." : filteredLogs)
                        .font(.system(.caption, design: .monospaced))
                        .textSelection(.enabled)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .padding()
                        .id("log-bottom")
                        .onChange(of: logs) { _, _ in
                            guard isLive, !isPaused else { return }
                            withAnimation { proxy.scrollTo("log-bottom", anchor: .bottom) }
                        }
                }
            }
            .background(Color(uiColor: .systemBackground))
        }
        .task {
            if isLive { await onRefresh() }
        }
    }
}
