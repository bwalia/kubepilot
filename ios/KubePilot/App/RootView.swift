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
        .task {
            await appState.applyScreenshotLaunchConfigurationIfNeeded()
        }
        // Lives here, outside the tab view that is rebuilt on every cluster
        // switch, so loading the cluster list cannot restart itself.
        .task(id: appState.authManager.configuredAccountID) {
            guard appState.authManager.configuredAccountID != nil else { return }
            await appState.clusterManager.accountChanged()
        }
        .onChange(of: scenePhase) { _, phase in
            if phase == .background {
                appState.authManager.lockIfNeeded()
            }
        }
    }
}

struct MainTabView: View {
    @Environment(AppState.self) private var appState
    @State private var askAIConsent = false

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
            Tab(AppTab.autopilot.title, systemImage: AppTab.autopilot.systemImage, value: AppTab.autopilot) {
                AutopilotView()
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
        // Rebuild every tab when the cluster changes, so no screen keeps
        // pods, logs or navigation state from the cluster we just left.
        .id(appState.clusterManager.epoch)
        // Ask about AI data sharing once per server account, right after
        // sign-in, so the choice is made before any AI feature is in view.
        .task(id: appState.authManager.activeAccount?.id) {
            guard !ProcessInfo.processInfo.arguments.contains("-UITestingScreenshots") else { return }
            askAIConsent = !AIConsent.hasAnswered(account: appState.authManager.activeAccount?.id)
        }
        .sheet(isPresented: $askAIConsent) {
            AIConsentSheet { _ in askAIConsent = false }
        }
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
