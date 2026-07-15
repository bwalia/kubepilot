import SwiftUI

struct RootView: View {
    @Environment(AppState.self) private var appState
    @Environment(\.scenePhase) private var scenePhase

    var body: some View {
        Group {
            if appState.authManager.requiresBiometricUnlock {
                BiometricUnlockView()
            } else if appState.isAuthenticated {
                MainTabView()
            } else {
                OnboardingView()
            }
        }
        .animation(.easeInOut(duration: 0.25), value: appState.isAuthenticated)
        .onChange(of: scenePhase) { _, phase in
            if phase == .background {
                appState.authManager.lockIfNeeded()
            }
        }
    }
}

struct MainTabView: View {
    @Environment(AppState.self) private var appState

    var body: some View {
        @Bindable var state = appState
        TabView(selection: $state.selectedTab) {
            ForEach(AppTab.allCases) { tab in
                tabContent(for: tab)
                    .tabItem { Label(tab.title, systemImage: tab.systemImage) }
                    .tag(tab)
            }
        }
        .tint(Theme.accent)
    }

    @ViewBuilder
    private func tabContent(for tab: AppTab) -> some View {
        switch tab {
        case .dashboard:
            DashboardView()
        case .pods:
            PodsListView()
        case .ai:
            AIAssistantView()
        case .alerts:
            AlertsView()
        case .settings:
            SettingsView()
        }
    }
}

#Preview {
    RootView()
        .environment(AppState())
}
