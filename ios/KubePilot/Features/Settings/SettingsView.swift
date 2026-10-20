import SwiftUI

struct SettingsView: View {
    @Environment(AppState.self) private var appState
    @State private var showClusters = false
    @State private var showAIConsent = false
    /// Mirrors the stored consent so the toggle redraws when it changes.
    @State private var aiAllowed = false

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
                        LabeledContent("Cluster", value: appState.clusterManager.activeContextName)
                        if let server = appState.clusterManager.activeCluster?.serverHost {
                            LabeledContent("API server", value: server)
                        }
                        Button("Switch or add cluster…") { showClusters = true }
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
                        Toggle("Share data with AI", isOn: Binding(
                            get: { aiAllowed },
                            set: { allow in
                                if allow {
                                    // Turning it on always goes through the full disclosure.
                                    showAIConsent = true
                                } else {
                                    AIConsent.set(false, account: appState.authManager.activeAccount?.id)
                                    aiAllowed = false
                                }
                            }
                        ))
                        Button("What's shared with AI") { showAIConsent = true }
                        Link("Privacy Policy", destination: AIConsentSheet.privacyPolicyURL)
                    } header: {
                        Text("AI & Privacy")
                    } footer: {
                        Text("AI Assistant and Analyse with AI send your questions, pod details and recent logs to the AI model configured on your KubePilot server. Off until you allow it.")
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
            .sheet(isPresented: $showClusters) { ClusterPickerView() }
            .sheet(isPresented: $showAIConsent) {
                AIConsentSheet { allowed in
                    aiAllowed = allowed
                    showAIConsent = false
                }
            }
            .onAppear { aiAllowed = AIConsent.isGranted(account: appState.authManager.activeAccount?.id) }
        }
    }
}
