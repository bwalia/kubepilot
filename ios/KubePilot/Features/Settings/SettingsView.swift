import SwiftUI

struct SettingsView: View {
    @Environment(AppState.self) private var appState

    var body: some View {
        NavigationStack {
            ZStack {
                BrandScreenBackground()

                List {
                    Section {
                        HStack(spacing: Theme.spacingSM) {
                            KubePilotMark(size: 44)
                            VStack(alignment: .leading, spacing: 2) {
                                KubePilotWordmark(size: .nav)
                                Text("Kubernetes operations on the go")
                                    .font(.caption)
                                    .foregroundStyle(Theme.muted)
                            }
                        }
                        .listRowBackground(Color.clear)
                    }

                    Section("Account") {
                        if let account = appState.authManager.activeAccount {
                            LabeledContent("Server", value: account.baseURL.absoluteString)
                            LabeledContent("Auth", value: account.authMethod.label)
                        }
                    }
                    .listRowBackground(Theme.surface)

                    Section("Cluster") {
                        LabeledContent("Context", value: appState.clusterManager.activeContextName)
                        Button("Refresh cluster metadata") {
                            Task { try? await appState.clusterManager.refresh() }
                        }
                    }
                    .listRowBackground(Theme.surface)

                    Section {
                        Toggle(
                            "Face ID on launch",
                            isOn: Binding(
                                get: { appState.authManager.activeAccount?.biometricLockEnabled ?? false },
                                set: { _ in }
                            )
                        )
                        .disabled(true)
                        Text("Configured during sign-in. Sign out and reconnect to change.")
                            .font(.caption)
                            .foregroundStyle(Theme.muted)
                    } header: {
                        Text("Security")
                    }
                    .listRowBackground(Theme.surface)

                    Section {
                        Button("Sign Out", role: .destructive) {
                            appState.signOut()
                        }
                    }
                    .listRowBackground(Theme.surface)

                    Section("About") {
                        LabeledContent("Version", value: "1.0.0")
                        LabeledContent("Build", value: "1")
                        Link("kubepilot.org", destination: URL(string: "https://kubepilot.org/")!)
                        Link("GitHub", destination: URL(string: "https://github.com/bwalia/kubepilot")!)
                    }
                    .listRowBackground(Theme.surface)
                }
                .themedForm()
            }
            .navigationTitle("Settings")
            .navigationBarTitleDisplayMode(.large)
            .themedScreen()
        }
    }
}
