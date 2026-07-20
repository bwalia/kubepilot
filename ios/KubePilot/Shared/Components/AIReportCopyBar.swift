import SwiftUI
import UIKit

/// Two developer utilities for an AI analysis result:
///  - Copy output: the raw analysis text.
///  - Copy as prompt: a ready-to-run prompt to paste into a terminal LLM (Claude)
///    to analyse further or produce a concrete fix.
struct AIReportCopyBar: View {
    let report: TroubleshootReport

    @State private var copied: Copied?

    private enum Copied { case output, prompt }

    var body: some View {
        HStack(spacing: 8) {
            button(
                title: copied == .output ? "Copied" : "Copy output",
                systemImage: copied == .output ? "checkmark" : "doc.on.doc",
                tint: Theme.textSecondary
            ) { copy(report.outputText, as: .output) }

            button(
                title: copied == .prompt ? "Copied prompt" : "Copy as prompt",
                systemImage: copied == .prompt ? "checkmark" : "terminal",
                tint: Theme.accent
            ) { copy(report.fixPrompt, as: .prompt) }

            Spacer(minLength: 0)
        }
    }

    private func button(title: String, systemImage: String, tint: Color, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Label(title, systemImage: systemImage)
                .font(.caption.weight(.semibold))
                .foregroundStyle(tint)
                .padding(.horizontal, 12)
                .padding(.vertical, 7)
                .background(tint.opacity(0.12), in: Capsule())
                .overlay(Capsule().stroke(tint.opacity(0.25), lineWidth: 1))
        }
        .buttonStyle(.plain)
    }

    private func copy(_ text: String, as kind: Copied) {
        UIPasteboard.general.string = text
        withAnimation { copied = kind }
        DispatchQueue.main.asyncAfter(deadline: .now() + 1.8) {
            withAnimation { if copied == kind { copied = nil } }
        }
    }
}
