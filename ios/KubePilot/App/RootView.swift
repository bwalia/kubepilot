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
            Tab(AppTab.dashboard.title, systemImage: AppTab.dashboard.systemImage, value: AppTab.dashboard) {
                DashboardView()
            }
            Tab(AppTab.pods.title, systemImage: AppTab.pods.systemImage, value: AppTab.pods) {
                PodsListView()
            }
            Tab(AppTab.ai.title, systemImage: AppTab.ai.systemImage, value: AppTab.ai) {
                AIAssistantView()
            }
            Tab(AppTab.alerts.title, systemImage: AppTab.alerts.systemImage, value: AppTab.alerts) {
                AlertsView()
            }
            Tab(AppTab.settings.title, systemImage: AppTab.settings.systemImage, value: AppTab.settings) {
                SettingsView()
            }
        }
        .tint(Theme.accent)
        .modifier(TabBarMinimizeModifier())
    }
}

private struct TabBarMinimizeModifier: ViewModifier {
    func body(content: Content) -> some View {
        if #available(iOS 26.0, *) {
            content.tabBarMinimizeBehavior(.onScrollDown)
        } else {
            content
        }
    }
}

#Preview {
    RootView()
        .environment(AppState())
}
