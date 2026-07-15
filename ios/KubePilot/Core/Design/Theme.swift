import SwiftUI

/// Brand palette aligned with https://kubepilot.org (docs/landing/index.html).
enum Theme {
    // MARK: - Brand (landing :root)

    static let brandBg = Color(hex: 0x0a0f1c)
    static let brandSurface = Color(hex: 0x111827)
    static let brandSurface2 = Color(hex: 0x1f2937)
    static let brandBorder = Color.white.opacity(0.08)

    static let accent = Color(hex: 0x3b82f6)
    static let accentLight = Color(hex: 0x60a5fa)
    static let purple = Color(hex: 0xa78bfa)
    static let green = Color(hex: 0x22c55e)
    static let amber = Color(hex: 0xfbbf24)

    static let textPrimary = Color(hex: 0xf1f5f9)
    static let textSecondary = Color(hex: 0xcbd5e1)
    static let muted = Color(hex: 0x94a3b8)

    static let brandGradient = LinearGradient(
        colors: [accentLight, purple],
        startPoint: .topLeading,
        endPoint: .bottomTrailing
    )

    // MARK: - Semantic

    static let success = green
    static let warning = amber
    static let danger = Color(hex: 0xf87171)

    static let background = brandBg
    static let surface = brandSurface
    static let surfaceElevated = brandSurface2

    static let cornerRadius: CGFloat = 16
    static let cornerRadiusSmall: CGFloat = 10

    static func healthColor(for score: HealthScore) -> Color {
        switch score {
        case .healthy: success
        case .degraded: warning
        case .critical: danger
        }
    }
}

extension Color {
    init(hex: UInt32, opacity: Double = 1) {
        let r = Double((hex >> 16) & 0xff) / 255
        let g = Double((hex >> 8) & 0xff) / 255
        let b = Double(hex & 0xff) / 255
        self.init(.sRGB, red: r, green: g, blue: b, opacity: opacity)
    }
}

enum HealthScore: String, Codable, Sendable {
    case healthy
    case degraded
    case critical

    var label: String {
        switch self {
        case .healthy: "Healthy"
        case .degraded: "Degraded"
        case .critical: "Critical"
        }
    }
}

struct BrandScreenBackground: View {
    var body: some View {
        ZStack {
            Theme.brandBg.ignoresSafeArea()
            RadialGradient(
                colors: [Theme.accent.opacity(0.18), .clear],
                center: .top,
                startRadius: 20,
                endRadius: 420
            )
            .ignoresSafeArea()
        }
    }
}

struct MetricCard: View {
    let title: String
    let value: String
    let subtitle: String?
    let tint: Color
    let action: (() -> Void)?

    init(
        title: String,
        value: String,
        subtitle: String? = nil,
        tint: Color = Theme.accent,
        action: (() -> Void)? = nil
    ) {
        self.title = title
        self.value = value
        self.subtitle = subtitle
        self.tint = tint
        self.action = action
    }

    var body: some View {
        Button(action: { action?() }) {
            VStack(alignment: .leading, spacing: 8) {
                Text(title)
                    .font(.caption)
                    .foregroundStyle(Theme.muted)
                    .textCase(.uppercase)
                Text(value)
                    .font(.system(.title, design: .rounded, weight: .bold))
                    .foregroundStyle(tint)
                    .contentTransition(.numericText())
                if let subtitle {
                    Text(subtitle)
                        .font(.caption2)
                        .foregroundStyle(Theme.muted)
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding()
            .background(Theme.surface, in: RoundedRectangle(cornerRadius: Theme.cornerRadius, style: .continuous))
            .overlay(
                RoundedRectangle(cornerRadius: Theme.cornerRadius, style: .continuous)
                    .stroke(Theme.brandBorder, lineWidth: 1)
            )
        }
        .buttonStyle(.plain)
        .disabled(action == nil)
    }
}

struct StatusBadge: View {
    let text: String
    let color: Color

    var body: some View {
        Text(text)
            .font(.caption2.weight(.semibold))
            .padding(.horizontal, 8)
            .padding(.vertical, 4)
            .background(color.opacity(0.15), in: Capsule())
            .foregroundStyle(color)
    }
}

struct LoadingOverlay: View {
    let message: String

    var body: some View {
        VStack(spacing: 12) {
            ProgressView()
                .tint(Theme.accent)
            Text(message)
                .font(.subheadline)
                .foregroundStyle(Theme.textSecondary)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

struct ErrorBanner: View {
    let message: String
    let retry: (() -> Void)?

    var body: some View {
        VStack(spacing: 12) {
            Image(systemName: "exclamationmark.triangle.fill")
                .font(.title2)
                .foregroundStyle(Theme.danger)
            Text(message)
                .font(.subheadline)
                .multilineTextAlignment(.center)
                .foregroundStyle(Theme.textSecondary)
            if let retry {
                Button("Retry", action: retry)
                    .buttonStyle(.borderedProminent)
                    .tint(Theme.accent)
            }
        }
        .padding()
    }
}

struct SearchBar: View {
    @Binding var text: String
    let placeholder: String

    var body: some View {
        HStack(spacing: 8) {
            Image(systemName: "magnifyingglass")
                .foregroundStyle(Theme.muted)
            TextField(placeholder, text: $text)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .foregroundStyle(Theme.textPrimary)
            if !text.isEmpty {
                Button { text = "" } label: {
                    Image(systemName: "xmark.circle.fill")
                        .foregroundStyle(Theme.muted)
                }
            }
        }
        .padding(10)
        .background(Theme.surface, in: RoundedRectangle(cornerRadius: Theme.cornerRadiusSmall, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: Theme.cornerRadiusSmall, style: .continuous)
                .stroke(Theme.brandBorder, lineWidth: 1)
        )
    }
}
