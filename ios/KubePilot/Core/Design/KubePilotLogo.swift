import SwiftUI

/// Wordmark matching kubepilot.org: **Kube** + blue **Pilot**.
struct KubePilotWordmark: View {
    var size: WordmarkSize = .large

    enum WordmarkSize {
        case nav, large, hero

        var kubeFont: Font {
            switch self {
            case .nav: .system(size: 22, weight: .black, design: .default)
            case .large: .system(size: 34, weight: .black, design: .default)
            case .hero: .system(size: 42, weight: .black, design: .default)
            }
        }
    }

    var body: some View {
        HStack(spacing: 0) {
            Text("Kube")
                .font(size.kubeFont)
                .foregroundStyle(Theme.textPrimary)
                .tracking(-0.5)
            Text("Pilot")
                .font(size.kubeFont)
                .foregroundStyle(Theme.accent)
                .tracking(-0.5)
        }
    }
}

/// App icon mark — helm wheel on brand dark surface (matches landing hero glow).
struct KubePilotMark: View {
    var size: CGFloat = 88

    var body: some View {
        ZStack {
            RoundedRectangle(cornerRadius: size * 0.22, style: .continuous)
                .fill(Theme.brandBg)
                .overlay(
                    RoundedRectangle(cornerRadius: size * 0.22, style: .continuous)
                        .stroke(Theme.brandBorder, lineWidth: max(1, size * 0.012))
                )

            RadialGradient(
                colors: [Theme.accent.opacity(0.35), .clear],
                center: .center,
                startRadius: 0,
                endRadius: size * 0.45
            )
            .clipShape(RoundedRectangle(cornerRadius: size * 0.22, style: .continuous))

            Image(systemName: "helm")
                .font(.system(size: size * 0.44, weight: .semibold))
                .foregroundStyle(Theme.brandGradient)
                .symbolRenderingMode(.hierarchical)
        }
        .frame(width: size, height: size)
    }
}

struct KubePilotBrandHeader: View {
    var subtitle: String = "AI-powered Kubernetes troubleshooting for engineers on the move."
    var markSize: CGFloat = 72

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            HStack(spacing: 14) {
                KubePilotMark(size: markSize)
                KubePilotWordmark(size: .large)
            }
            Text(subtitle)
                .font(.subheadline)
                .foregroundStyle(Theme.textSecondary)
                .fixedSize(horizontal: false, vertical: true)
        }
    }
}

#Preview {
    ZStack {
        BrandScreenBackground()
        VStack(spacing: 32) {
            KubePilotBrandHeader()
            KubePilotMark(size: 120)
        }
        .padding()
    }
    .preferredColorScheme(.dark)
}
