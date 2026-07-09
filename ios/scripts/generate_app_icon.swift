#!/usr/bin/env swift
// Generates ios/KubePilot/Assets.xcassets/AppIcon.appiconset/AppIcon-1024.png
// Brand: kubepilot.org — dark navy #0a0f1c, blue #3b82f6, gradient accent.

import AppKit
import Foundation

let size = 1024
let outPath = CommandLine.arguments.count > 1
    ? CommandLine.arguments[1]
    : "KubePilot/Assets.xcassets/AppIcon.appiconset/AppIcon-1024.png"

guard let rep = NSBitmapImageRep(
    bitmapDataPlanes: nil,
    pixelsWide: size,
    pixelsHigh: size,
    bitsPerSample: 8,
    samplesPerPixel: 4,
    hasAlpha: true,
    isPlanar: false,
    colorSpaceName: .deviceRGB,
    bytesPerRow: 0,
    bitsPerPixel: 0
) else {
    fputs("Failed to create bitmap\n", stderr)
    exit(1)
}

NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)

let cgSize = CGFloat(size)
let bg = NSColor(red: 10 / 255, green: 15 / 255, blue: 28 / 255, alpha: 1)
bg.setFill()
let bgPath = NSBezierPath(roundedRect: NSRect(x: 0, y: 0, width: cgSize, height: cgSize),
                          xRadius: cgSize * 0.22, yRadius: cgSize * 0.22)
bgPath.fill()

let glow = NSGradient(colors: [
    NSColor(red: 59 / 255, green: 130 / 255, blue: 246 / 255, alpha: 0.35),
    NSColor.clear,
])!
glow.draw(fromCenter: NSPoint(x: cgSize / 2, y: cgSize / 2),
          radius: 0,
          toCenter: NSPoint(x: cgSize / 2, y: cgSize / 2),
          radius: cgSize * 0.48,
          options: [])

let paragraph = NSMutableParagraphStyle()
paragraph.alignment = .center
let attrs: [NSAttributedString.Key: Any] = [
    .font: NSFont.systemFont(ofSize: cgSize * 0.34, weight: .black),
    .foregroundColor: NSColor(red: 96 / 255, green: 165 / 255, blue: 250 / 255, alpha: 1),
    .paragraphStyle: paragraph,
]
let text = "KP" as NSString
let textSize = text.size(withAttributes: attrs)
let textRect = NSRect(
    x: (cgSize - textSize.width) / 2,
    y: (cgSize - textSize.height) / 2 - cgSize * 0.02,
    width: textSize.width,
    height: textSize.height
)
text.draw(in: textRect, withAttributes: attrs)

NSGraphicsContext.restoreGraphicsState()

guard let png = rep.representation(using: .png, properties: [:]) else {
    fputs("Failed to encode PNG\n", stderr)
    exit(1)
}

let url = URL(fileURLWithPath: outPath)
try FileManager.default.createDirectory(at: url.deletingLastPathComponent(),
                                        withIntermediateDirectories: true)
try png.write(to: url)
print("Wrote \(outPath) (\(size)x\(size))")
