import SwiftUI

/// Bespoke KubePilot logo: a ship's **helm wheel** — Kubernetes is Greek for
/// *helmsman*, Helm is the K8s package manager, and "Pilot" steers the cluster.
///
/// The geometry here is the single source of truth for the brand mark. The
/// 1024pt App Icon (`ios/scripts/generate_app_icon.swift`) mirrors these exact
/// fractions with Core Graphics so the icon and the in-app mark are identical.
enum HelmGeometry {
    /// Radius of the ring centreline, as a fraction of the mark's side.
    static let ringMid: CGFloat = 0.315
    static let ringThickness: CGFloat = 0.074
    /// Inner end of a spoke (meets the hub) and outer end (meets a handle knob).
    static let spokeInner: CGFloat = 0.105
    static let spokeOuter: CGFloat = 0.352
    static let spokeWidth: CGFloat = 0.052
    static let knobRadius: CGFloat = 0.052
    static let hubRadius: CGFloat = 0.122
    static let hubHoleRadius: CGFloat = 0.058
    static let spokeCount = 6

    /// Unit-circle direction of spoke `i` (spoke 0 points straight up).
    static func spokeAngle(_ i: Int) -> Angle {
        .degrees(-90 + Double(i) * 360 / Double(spokeCount))
    }
}

/// The helm wheel, filled with the brand gradient. Transparent background so it
/// can sit on the branded tile (`KubePilotMark`) or any surface.
struct HelmMark: View {
    /// Gradient endpoints stay in [0,1] canvas fraction → resolution-independent.
    var body: some View {
        Canvas { ctx, size in
            let s = min(size.width, size.height)
            let c = CGPoint(x: size.width / 2, y: size.height / 2)
            let shading = GraphicsContext.Shading.linearGradient(
                Gradient(colors: [Theme.accentLight, Theme.purple]),
                startPoint: .zero,
                endPoint: CGPoint(x: size.width, y: size.height)
            )
            let hole = GraphicsContext.Shading.color(Theme.brandBg)

            func point(_ angle: Angle, _ r: CGFloat) -> CGPoint {
                CGPoint(x: c.x + cos(angle.radians) * r * s,
                        y: c.y + sin(angle.radians) * r * s)
            }

            // Ring.
            var ring = Path()
            ring.addArc(center: c, radius: s * HelmGeometry.ringMid,
                        startAngle: .zero, endAngle: .degrees(360), clockwise: false)
            ctx.stroke(ring, with: shading,
                       style: StrokeStyle(lineWidth: s * HelmGeometry.ringThickness))

            // Spokes.
            var spokes = Path()
            for i in 0..<HelmGeometry.spokeCount {
                let a = HelmGeometry.spokeAngle(i)
                spokes.move(to: point(a, HelmGeometry.spokeInner))
                spokes.addLine(to: point(a, HelmGeometry.spokeOuter))
            }
            ctx.stroke(spokes, with: shading,
                       style: StrokeStyle(lineWidth: s * HelmGeometry.spokeWidth,
                                          lineCap: .round))

            // Handle knobs at the spoke tips.
            for i in 0..<HelmGeometry.spokeCount {
                let tip = point(HelmGeometry.spokeAngle(i), HelmGeometry.spokeOuter)
                let r = s * HelmGeometry.knobRadius
                ctx.fill(Path(ellipseIn: CGRect(x: tip.x - r, y: tip.y - r,
                                                width: r * 2, height: r * 2)),
                         with: shading)
            }

            // Hub with a punched-out centre for depth.
            let hubR = s * HelmGeometry.hubRadius
            ctx.fill(Path(ellipseIn: CGRect(x: c.x - hubR, y: c.y - hubR,
                                            width: hubR * 2, height: hubR * 2)),
                     with: shading)
            let holeR = s * HelmGeometry.hubHoleRadius
            ctx.fill(Path(ellipseIn: CGRect(x: c.x - holeR, y: c.y - holeR,
                                            width: holeR * 2, height: holeR * 2)),
                     with: hole)
        }
    }
}

#Preview {
    ZStack {
        Theme.brandBg
        HelmMark()
            .frame(width: 240, height: 240)
    }
    .ignoresSafeArea()
    .preferredColorScheme(.dark)
}
