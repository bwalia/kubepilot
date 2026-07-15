# KubePilot iOS

Native SwiftUI app for AI-powered Kubernetes troubleshooting.

## Quick Start

```bash
brew install xcodegen   # once
./generate.sh
open KubePilot.xcodeproj
```

Build succeeded target: **iPhone Simulator (iOS 17+)**

## Brand assets

Visual identity matches [kubepilot.org](https://kubepilot.org/) (`docs/landing/index.html`):

- **Theme** — `KubePilot/Core/Design/Theme.swift` (navy `#0a0f1c`, brand blue `#3b82f6`, gradient accents)
- **Logo** — `KubePilot/Core/Design/KubePilotLogo.swift` (`KubePilotWordmark`, `KubePilotMark`)
- **App icon** — regenerate after brand tweaks:

```bash
swift scripts/generate_app_icon.swift
./generate.sh
```

## Release (TestFlight & App Store)

See [docs/IOS_RELEASE.md](../docs/IOS_RELEASE.md) for the GitHub Actions + Vault + fastlane pipeline (`.github/workflows/ios_release.yml`).

## Documentation

See [docs/IOS_APP.md](../docs/IOS_APP.md) for architecture, API integration, testing, and release process.
