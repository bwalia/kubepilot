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
}

enum AppTab: String, CaseIterable, Identifiable {
    case dashboard
    case pods
    case ai
    case alerts
    case settings

    var id: String { rawValue }

    var title: String {
        switch self {
        case .dashboard: "Dashboard"
        case .pods: "Pods"
        case .ai: "AI"
        case .alerts: "Alerts"
        case .settings: "Settings"
        }
    }

    var systemImage: String {
        switch self {
        case .dashboard: "gauge.with.dots.needle.67percent"
        case .pods: "cube.box"
        case .ai: "sparkles"
        case .alerts: "bell.badge"
        case .settings: "gearshape"
        }
    }
}
