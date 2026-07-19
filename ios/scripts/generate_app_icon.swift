#!/usr/bin/env swift
// Generates ios/KubePilot/Assets.xcassets/AppIcon.appiconset/AppIcon-1024.png
//
// Draws the bespoke KubePilot helm-wheel mark — the SAME geometry as the in-app
// `HelmMark` (ios/KubePilot/Core/Design/HelmMark.swift). Keep the fractions in
// `Helm` below in sync with `HelmGeometry` there.
//
// HIG-compliant: a full-bleed 1024×1024 square with NO rounded corners — iOS
// applies the corner mask itself.

import AppKit
import Foundation

let size = 1024
let outPath = CommandLine.arguments.count > 1
    ? CommandLine.arguments[1]
    : "KubePilot/Assets.xcassets/AppIcon.appiconset/AppIcon-1024.png"

// Brand palette (kubepilot.org).
func rgb(_ r: Int, _ g: Int, _ b: Int, _ a: CGFloat = 1) -> CGColor {
    CGColor(red: CGFloat(r) / 255, green: CGFloat(g) / 255, blue: CGFloat(b) / 255, alpha: a)
}
let brandBg = rgb(10, 15, 28)          // #0a0f1c
let accent = rgb(59, 130, 246)         // #3b82f6
let accentLight = rgb(96, 165, 250)    // #60a5fa
let purple = rgb(167, 139, 250)        // #a78bfa

// Helm geometry as a fraction of the icon side — mirrors HelmGeometry.swift.
enum Helm {
    static let ringMid: CGFloat = 0.315
    static let ringThickness: CGFloat = 0.074
    static let spokeInner: CGFloat = 0.105
    static let spokeOuter: CGFloat = 0.352
    static let spokeWidth: CGFloat = 0.052
    static let knobRadius: CGFloat = 0.052
    static let hubRadius: CGFloat = 0.122
    static let hubHoleRadius: CGFloat = 0.058
    static let spokeCount = 6
}

// App Store validation (error 90717) rejects a 1024pt icon with an alpha
// channel, so the bitmap is opaque RGB — no alpha sample to encode. 32bpp with
// only 3 samples is deliberate: CoreGraphics has no 24bpp backing store, so the
// 4th byte must exist but be skipped rather than dropped.
guard let rep = NSBitmapImageRep(
    bitmapDataPlanes: nil,
    pixelsWide: size,
    pixelsHigh: size,
    bitsPerSample: 8,
    samplesPerPixel: 3,
    hasAlpha: false,
    isPlanar: false,
    colorSpaceName: .deviceRGB,
    bitmapFormat: [],
    bytesPerRow: size * 4,
    bitsPerPixel: 32
) else { fputs("Failed to create bitmap\n", stderr); exit(1) }

NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
guard let ctx = NSGraphicsContext.current?.cgContext else {
    fputs("No CGContext\n", stderr); exit(1)
}

let S = CGFloat(size)
let center = CGPoint(x: S / 2, y: S / 2)
let space = CGColorSpaceCreateDeviceRGB()
let markGradient = CGGradient(colorsSpace: space,
                              colors: [accentLight, purple] as CFArray,
                              locations: [0, 1])!

// Flip to a top-left origin so this matches SwiftUI's coordinate space exactly
// (spoke 0 points up, gradient runs top-left → bottom-right).
ctx.translateBy(x: 0, y: S)
ctx.scaleBy(x: 1, y: -1)

func point(_ i: Int, _ r: CGFloat) -> CGPoint {
    let a = (-90 + Double(i) * 360 / Double(Helm.spokeCount)) * .pi / 180
    return CGPoint(x: center.x + cos(a) * r * S, y: center.y + sin(a) * r * S)
}

// 1. Opaque background — full square, no rounded corners.
ctx.setFillColor(brandBg)
ctx.fill(CGRect(x: 0, y: 0, width: S, height: S))

// 2. Radial brand glow: a cool wash from the top, and a lift behind the mark.
func radialGlow(center gc: CGPoint, radius: CGFloat, color: CGColor) {
    let g = CGGradient(colorsSpace: space,
                       colors: [color, color.copy(alpha: 0)!] as CFArray,
                       locations: [0, 1])!
    ctx.drawRadialGradient(g, startCenter: gc, startRadius: 0,
                           endCenter: gc, endRadius: radius, options: [])
}
radialGlow(center: CGPoint(x: S / 2, y: S * 0.16), radius: S * 0.62, color: accent.copy(alpha: 0.20)!)
radialGlow(center: center, radius: S * 0.48, color: accent.copy(alpha: 0.32)!)

// Fill the current path with the brand gradient (clipped to that path).
func fillGradient(_ path: CGPath) {
    ctx.saveGState()
    ctx.addPath(path)
    ctx.clip()
    ctx.drawLinearGradient(markGradient, start: .zero,
                           end: CGPoint(x: S, y: S), options: [])
    ctx.restoreGState()
}
// Stroke a path with the brand gradient (round caps), clipped to the stroke.
func strokeGradient(_ path: CGPath, width: CGFloat, cap: CGLineCap = .butt) {
    ctx.saveGState()
    ctx.addPath(path)
    ctx.setLineWidth(width)
    ctx.setLineCap(cap)
    ctx.replacePathWithStrokedPath()
    ctx.clip()
    ctx.drawLinearGradient(markGradient, start: .zero,
                           end: CGPoint(x: S, y: S), options: [])
    ctx.restoreGState()
}

// 3. Ring.
let ring = CGMutablePath()
ring.addArc(center: center, radius: S * Helm.ringMid, startAngle: 0, endAngle: .pi * 2, clockwise: false)
strokeGradient(ring, width: S * Helm.ringThickness)

// 4. Spokes (round caps).
let spokes = CGMutablePath()
for i in 0..<Helm.spokeCount {
    spokes.move(to: point(i, Helm.spokeInner))
    spokes.addLine(to: point(i, Helm.spokeOuter))
}
strokeGradient(spokes, width: S * Helm.spokeWidth, cap: .round)

// 5. Handle knobs.
let knobs = CGMutablePath()
for i in 0..<Helm.spokeCount {
    let tip = point(i, Helm.spokeOuter)
    let r = S * Helm.knobRadius
    knobs.addEllipse(in: CGRect(x: tip.x - r, y: tip.y - r, width: r * 2, height: r * 2))
}
fillGradient(knobs)

// 6. Hub + punched-out centre for depth.
let hubR = S * Helm.hubRadius
let hub = CGMutablePath()
hub.addEllipse(in: CGRect(x: center.x - hubR, y: center.y - hubR, width: hubR * 2, height: hubR * 2))
fillGradient(hub)

let holeR = S * Helm.hubHoleRadius
ctx.setFillColor(brandBg)
ctx.fillEllipse(in: CGRect(x: center.x - holeR, y: center.y - holeR, width: holeR * 2, height: holeR * 2))

NSGraphicsContext.restoreGraphicsState()

guard let png = rep.representation(using: .png, properties: [:]) else {
    fputs("Failed to encode PNG\n", stderr); exit(1)
}
let url = URL(fileURLWithPath: outPath)
try FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
try png.write(to: url)
print("Wrote \(outPath) (\(size)x\(size))")
