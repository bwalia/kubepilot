#!/usr/bin/env python3
"""WCAG 2.1 contrast check for the KubePilot iOS palette.

Parses the hex literals straight out of Core/Design/Theme.swift, so changing a
colour there re-runs the real numbers instead of checking a stale copy.

    python3 ios/scripts/check_contrast.py

Exits non-zero if any pairing the app actually renders drops below its bar:
4.5:1 for body text, 3:1 for large text (>=18pt, or >=14pt bold).
"""
import re
import sys
from pathlib import Path

THEME = Path(__file__).resolve().parent.parent / "KubePilot/Core/Design/Theme.swift"


def parse_palette(src: str) -> dict[str, tuple[int, int, int]]:
    """Pull the palette out of Theme.swift.

    Handles both literals (`static let accent = Color(hex: 0x3B82F6)`) and the
    semantic aliases layered on top (`static let success = green`).
    """
    literal = re.compile(r"static let (\w+)\s*=\s*Color\(hex:\s*0x([0-9A-Fa-f]{6})\)")
    found = {name: int(hx, 16) for name, hx in literal.findall(src)}

    alias = re.compile(r"static let (\w+)\s*=\s*(\w+)\s*$", re.MULTILINE)
    pairs = alias.findall(src)
    # Resolve chains (success -> green -> 0x34D399); a few passes is ample.
    for _ in range(len(pairs) + 1):
        for name, target in pairs:
            if name not in found and target in found:
                found[name] = found[target]

    return {n: ((v >> 16) & 255, (v >> 8) & 255, v & 255) for n, v in found.items()}


def _linear(channel: int) -> float:
    c = channel / 255
    return c / 12.92 if c <= 0.04045 else ((c + 0.055) / 1.055) ** 2.4


def luminance(rgb: tuple[int, int, int]) -> float:
    r, g, b = (_linear(c) for c in rgb)
    return 0.2126 * r + 0.7152 * g + 0.0722 * b


def ratio(fg: tuple[int, int, int], bg: tuple[int, int, int]) -> float:
    a, b = luminance(fg), luminance(bg)
    return (max(a, b) + 0.05) / (min(a, b) + 0.05)


def over(fg, alpha: float, bg):
    """Composite `fg` at `alpha` over `bg` — what SwiftUI's .opacity() renders."""
    return tuple(round(f * alpha + b * (1 - alpha)) for f, b in zip(fg, bg))


def main() -> int:
    if not THEME.exists():
        print(f"FAIL: cannot find {THEME}")
        return 1

    P = parse_palette(THEME.read_text())
    missing = [k for k in ("brandBg", "brandSurface", "brandSurface2", "accent",
                           "accentStrong", "accentLight", "textPrimary",
                           "textSecondary", "muted") if k not in P]
    if missing:
        print(f"FAIL: Theme.swift no longer defines: {', '.join(missing)}")
        return 1

    WHITE = (255, 255, 255)
    surfaces = [("brandBg", P["brandBg"]),
                ("surface", P["brandSurface"]),
                ("surfaceElevated", P["brandSurface2"])]
    semantic = ["success", "warning", "danger", "purple"]

    checks: list[tuple[str, float, float]] = []

    def check(label, fg, bg, need=4.5):
        checks.append((label, ratio(fg, bg), need))

    # Body text on every surface it can land on.
    for token in ("textPrimary", "textSecondary", "muted", "accentLight"):
        for name, bg in surfaces:
            check(f"{token} on {name}", P[token], bg)

    # Semantic colours as text, and as a StatusBadge pill (tint @18% over surface).
    for token in semantic:
        for name, bg in surfaces:
            check(f"{token} on {name}", P[token], bg)
        check(f"{token} badge", P[token], over(P[token], 0.18, P["brandSurface"]))

    # The accent pill renders accentLight, because raw accent is only ~3.7:1 there.
    check("accent badge (accentLight)",
          P["accentLight"], over(P["accent"], 0.18, P["brandSurface"]))

    # Solid primary button: white on the accentStrong gradient, both stops.
    check("white on primary button (top)", WHITE, P["accentStrong"])
    check("white on primary button (bottom)",
          WHITE, over(P["accentStrong"], 0.86, P["brandSurface"]))

    # Brand wordmark is large display text, so it answers to the 3:1 bar.
    for name, bg in surfaces:
        check(f"wordmark accent on {name} (large)", P["accent"], bg, need=3.0)

    failures = [(l, r, n) for l, r, n in checks if r < n]
    for label, r, need in checks:
        print(f"  {'FAIL' if r < need else 'ok':<5} {r:>6.2f}:1 (>= {need})  {label}")

    print(f"\n{len(checks)} pairings checked, {len(failures)} failing")
    if failures:
        print("\nContrast regressions:")
        for label, r, need in failures:
            print(f"  {label}: {r:.2f}:1, needs {need}:1")
        return 1
    print("contrast: all checks passed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
