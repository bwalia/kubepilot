import Foundation
import Observation
import SwiftUI

/// Global application state shared across features.
@MainActor
@Observable
final class AppState {
    var authManager = AuthManager()
    var clusterManager = ClusterManager()
    var selectedTab: AppTab = .dashboard
    var aiAssistant = AIAssistantViewModel()

    var isAuthenticated: Bool {
        authManager.activeAccount != nil
    }

    func signOut() {
        authManager.signOut()
        clusterManager.reset()
        selectedTab = .dashboard
    }

    /// Seeds a temporary account + tab selection when launched with
    /// `-UITestingScreenshots` (used only for App Store screenshot capture).
    func applyScreenshotLaunchConfigurationIfNeeded() async {
        let args = ProcessInfo.processInfo.arguments
        guard args.contains("-UITestingScreenshots") else { return }

        func value(after flag: String) -> String? {
            guard let idx = args.firstIndex(of: flag), args.index(after: idx) < args.endIndex else {
                return nil
            }
            return args[args.index(after: idx)]
        }

        let server = value(after: "-UIScreenshotServer") ?? "http://127.0.0.1:8383"
        guard let baseURL = URL(string: server) else { return }

        let user = value(after: "-UIScreenshotUser") ?? "admin"
        let password = value(after: "-UIScreenshotPassword") ?? ""
        let account = ServerAccount(
            id: "screenshot-demo",
            displayName: "Screenshot Demo",
            baseURL: baseURL,
            authMethod: .basic,
            bearerToken: nil,
            username: user,
            password: password,
            clusters: [],
            activeClusterID: nil,
            biometricLockEnabled: false,
            createdAt: .now
        )
        try? await authManager.addAccount(account)

        if let tabRaw = value(after: "-UITab"), let tab = AppTab(rawValue: tabRaw) {
            selectedTab = tab
        }
    }
}

enum AppTab: String, CaseIterable, Identifiable {
    case dashboard
    case pods
    case ai
    case autopilot
    case alerts
    case settings

    var id: String { rawValue }

    var title: String {
        switch self {
        case .dashboard: "Dashboard"
        case .pods: "Pods"
        case .ai: "AI"
        case .autopilot: "Autopilot"
        case .alerts: "Alerts"
        case .settings: "Settings"
        }
    }

    var systemImage: String {
        switch self {
        case .dashboard: "gauge.with.dots.needle.67percent"
        case .pods: "cube.box"
        case .ai: "sparkles"
        case .autopilot: "autostartstop"
        case .alerts: "bell.badge"
        case .settings: "gearshape"
        }
    }
}
