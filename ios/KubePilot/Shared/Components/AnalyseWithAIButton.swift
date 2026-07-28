import SwiftUI

/// Primary CTA used wherever AI analysis should be one tap away.
struct AnalyseWithAIButton: View {
    var title: String = "Analyse with AI"
    var isLoading = false
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: Theme.spacingSM) {
                if isLoading {
                    ProgressView()
                        .tint(.white)
                } else {
                    Image(systemName: "sparkles")
                }
                Text(isLoading ? "Analysing…" : title)
                    .fontWeight(.semibold)
            }
            .frame(maxWidth: .infinity)
            .padding(.vertical, 12)
        }
        .buttonStyle(.borderedProminent)
        .tint(Theme.accent)
        .controlSize(.large)
        .disabled(isLoading)
        .accessibilityLabel(title)
    }
}
