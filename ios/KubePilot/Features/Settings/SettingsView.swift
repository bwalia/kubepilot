import SwiftUI

struct SettingsView: View {
    @Environment(AppState.self) private var appState

    var body: some View {
        NavigationStack {
            List {
                Section("Account") {
                    if let account = appState.authManager.activeAccount {
                        LabeledContent("Server", value: account.baseURL.absoluteString)
                        LabeledContent("Auth", value: account.authMethod.label)
                    }
                }

                Section("Cluster") {
                    LabeledContent("Context", value: appState.clusterManager.activeContextName)
                    Button("Refresh cluster metadata") {
                        Task { try? await appState.clusterManager.refresh() }
                    }
                }

                Section("Security") {
                    Toggle(
                        "Face ID on launch",
                        isOn: Binding(
                            get: { appState.authManager.activeAccount?.biometricLockEnabled ?? false },
                            set: { _ in }
                        )
                    )
                    .disabled(true) // configured at login; editable in future settings flow
                }

                Section {
                    Button("Sign Out", role: .destructive) {
                        appState.signOut()
                    }
                }

                Section("About") {
                    LabeledContent("Version", value: "1.0.0")
                    LabeledContent("Build", value: "1")
                    Link("kubepilot.org", destination: URL(string: "https://kubepilot.org/")!)
                    Link("GitHub", destination: URL(string: "https://github.com/bwalia/kubepilot")!)
                }
            }
            .navigationTitle("Settings")
        }
    }
}
