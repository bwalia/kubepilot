import SwiftUI

/// Design tokens for KubePilot. Brand palette aligned with https://kubepilot.org.
///
/// The app runs dark-only (`preferredColorScheme(.dark)` in KubePilotApp), so every
/// pair below is tuned for a dark canvas and verified against WCAG AA (4.5:1 for body
/// text, 3:1 for large text and UI glyphs) on `surface`:
///
///   textPrimary   16.3:1    accentLight  7.3:1    success  8.9:1
///   textSecondary 10.8:1    warning      9.6:1    danger   6.4:1
///   muted          6.5:1
///
/// Depth comes from the border + elevation ramp, not from background contrast:
/// adjacent dark surfaces are only ~1.1:1 apart, which is why `border` has to stay
/// visible. Do not drop it back to a low white opacity.
enum Theme {
    // MARK: - Foundation

    /// Canvas, behind everything.
    static let brandBg = Color(hex: 0x0A0E1A)
    /// Cards, rows, grouped content.
    static let brandSurface = Color(hex: 0x141B2D)
    /// Inputs, chips, anything sitting on top of a card.
    static let brandSurface2 = Color(hex: 0x1D2639)
    /// Hairlines. Carries the separation that background contrast cannot.
    static let brandBorder = Color(hex: 0x2B3952)
    /// Focus rings and emphasised edges.
    static let brandBorderStrong = Color(hex: 0x3D4F6E)

    // MARK: - Brand

    static let accent = Color(hex: 0x3B82F6)
    /// Brighter tint for accent-coloured *text* on dark surfaces (AA at 7.3:1).
    /// `accent` itself is only 4.1:1 on `surfaceElevated`, so it must not be used
    /// for small text — use this instead, or `onTint(for:)`.
    static let accentLight = Color(hex: 0x7DA9FF)
    /// Fill for solid buttons carrying white text. `accent` gives white only
    /// 3.7:1; this is dark enough for 5.2:1 while staying the same hue.
    static let accentStrong = Color(hex: 0x2563EB)
    static let purple = Color(hex: 0xA78BFA)
    static let green = Color(hex: 0x34D399)
    static let amber = Color(hex: 0xFBBF24)

    // MARK: - Text

    static let textPrimary = Color(hex: 0xF7F9FC)
    static let textSecondary = Color(hex: 0xC2CEDF)
    /// Captions and labels. Still AA for body text — do not darken further.
    static let muted = Color(hex: 0x8FA0BC)

    static let brandGradient = LinearGradient(
        colors: [accentLight, purple],
        startPoint: .topLeading,
        endPoint: .bottomTrailing
    )

    // MARK: - Semantic

    static let success = green
    static let warning = amber
    static let danger = Color(hex: 0xFB7185)

    static let background = brandBg
    static let surface = brandSurface
    static let surfaceElevated = brandSurface2

    // MARK: - Shape

    static let cornerRadius: CGFloat = 16
    static let cornerRadiusSmall: CGFloat = 10

    // 4pt rhythm.
    static let spacingXS: CGFloat = 4
    static let spacingSM: CGFloat = 8
    static let spacingMD: CGFloat = 16
    static let spacingLG: CGFloat = 24
    static let spacingXL: CGFloat = 32

    /// Minimum tappable edge (Apple HIG).
    static let minTouchTarget: CGFloat = 44

    // MARK: - Typography

    enum Typography {
        static let sectionTitle = Font.headline.weight(.semibold)
        static let navTitle = Font.headline.weight(.semibold)

        /// Small all-caps label above a value. Pair with `.tracking(0.6)`.
        static let cardLabel = Font.caption.weight(.semibold)

        /// Metrics. Tabular so digits do not reflow as values tick.
        static let metricValue = Font.system(.title2, design: .rounded, weight: .bold)
            .monospacedDigit()

        /// Kubernetes identifiers — pod names, namespaces, image tags. Monospace stops
        /// `kube-system` and `kube-sysstem` looking identical at a glance.
        static let identifier = Font.system(.subheadline, design: .monospaced)
        static let identifierSmall = Font.system(.caption, design: .monospaced)
    }

    // MARK: - Elevation

    enum Elevation {
        /// Resting card.
        static let card = ShadowStyle(color: .black.opacity(0.30), radius: 10, y: 4)
        /// Lifted surfaces — sheets, popovers, the active row.
        static let raised = ShadowStyle(color: .black.opacity(0.42), radius: 20, y: 10)

        struct ShadowStyle {
            let color: Color
            let radius: CGFloat
            let y: CGFloat
        }
    }

    // MARK: - Motion

    /// One rhythm for the whole app, so nothing feels borrowed from another product.
    enum Motion {
        /// Press / selection feedback.
        static let quick = Animation.spring(response: 0.26, dampingFraction: 0.78)
        /// Content appearing or changing.
        static let standard = Animation.spring(response: 0.38, dampingFraction: 0.82)
        /// Scale applied while a card or button is held.
        static let pressedScale: CGFloat = 0.97
    }

    // MARK: - Mapping

    static func healthColor(for score: HealthScore) -> Color {
        switch score {
        case .healthy: success
        case .degraded: warning
        case .critical: danger
        }
    }

    static func severityColor(_ severity: String) -> Color {
        switch severity.lowercased() {
        case "critical", "high", "error": danger
        case "medium", "warning": warning
        default: accent
        }
    }

    /// AA-safe *text* colour for a semantic tint. Every semantic colour here clears
    /// 4.5:1 on its own tinted pill except `accent`, which lands at 3.7:1 — so it is
    /// swapped for `accentLight`. Use this anywhere a tint is painted as text.
    static func onTint(for color: Color) -> Color {
        color == accent ? accentLight : color
    }

    /// Icon paired with a semantic colour so status never relies on colour alone
    /// (WCAG 1.4.1) — matters for the ~8% of men with colour-vision deficiency.
    static func severityIcon(_ color: Color) -> String {
        switch color {
        case danger: "exclamationmark.octagon.fill"
        case warning: "exclamationmark.triangle.fill"
        case success: "checkmark.circle.fill"
        default: "info.circle.fill"
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

extension View {
    func elevation(_ style: Theme.Elevation.ShadowStyle) -> some View {
        shadow(color: style.color, radius: style.radius, x: 0, y: style.y)
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

    var systemImage: String {
        switch self {
        case .healthy: "checkmark.circle.fill"
        case .degraded: "exclamationmark.triangle.fill"
        case .critical: "exclamationmark.octagon.fill"
        }
    }
}

/// App canvas: a deep base lit from the top-left, so screens have a focal point
/// instead of reading as a flat black rectangle.
struct BrandScreenBackground: View {
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        ZStack {
            Theme.brandBg.ignoresSafeArea()

            // Two offset glows rather than a mesh: cheaper, and it keeps the same
            // shape on every iOS version instead of silently falling back.
            RadialGradient(
                colors: [Theme.accent.opacity(0.22), .clear],
                center: UnitPoint(x: 0.08, y: -0.04),
                startRadius: 8,
                endRadius: 480
            )
            .ignoresSafeArea()

            RadialGradient(
                colors: [Theme.purple.opacity(0.14), .clear],
                center: UnitPoint(x: 1.02, y: 0.22),
                startRadius: 8,
                endRadius: 420
            )
            .ignoresSafeArea()
            .blendMode(reduceMotion ? .normal : .plusLighter)
        }
    }
}
