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

/// App icon mark — bespoke helm wheel on the brand dark surface, matching the
/// 1024pt App Icon and the landing hero glow. Uses `HelmMark` as the source of
/// truth for the wheel geometry so every surface renders the identical logo.
struct KubePilotMark: View {
    var size: CGFloat = 88
    /// When embedded on an already-dark surface, drop the tile chrome.
    var showsTile: Bool = true

    var body: some View {
        ZStack {
            if showsTile {
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
                    endRadius: size * 0.5
                )
                .clipShape(RoundedRectangle(cornerRadius: size * 0.22, style: .continuous))
            }

            HelmMark()
                .frame(width: size * 0.82, height: size * 0.82)
                .shadow(color: Theme.accent.opacity(showsTile ? 0.4 : 0), radius: size * 0.06)
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
