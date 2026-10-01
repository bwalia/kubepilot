import SwiftUI

struct OnboardingView: View {
    @Environment(AppState.self) private var appState
    @State private var viewModel = OnboardingViewModel()

    var body: some View {
        NavigationStack {
            ZStack {
                BrandScreenBackground()

                Form {
                    Section {
                        KubePilotBrandHeader()
                            .listRowBackground(Color.clear)
                            .listRowInsets(EdgeInsets(top: 12, leading: 0, bottom: 4, trailing: 0))
                    }

                    // The fastest path into the app, kept above the fold so a first-time
                    // user (or an App Review tester) never has to scroll to find it.
                    Section {
                        DemoLaunchCard(isBusy: viewModel.isConnecting) {
                            Task { await viewModel.tryDemo(using: appState.authManager) }
                        }
                        .listRowInsets(EdgeInsets(top: 4, leading: 0, bottom: 4, trailing: 0))
                        .listRowBackground(Color.clear)
                    }

                    Section {
                        LabeledField(title: "Server URL") {
                            TextField(
                                "",
                                text: $viewModel.serverURL,
                                prompt: Text("https://kubepilot.example.com").foregroundStyle(Theme.muted)
                            )
                            .textInputAutocapitalization(.never)
                            .keyboardType(.URL)
                            .textContentType(.URL)
                            .autocorrectionDisabled()
                            .foregroundStyle(Theme.textPrimary)
                        }

                        Picker("Method", selection: $viewModel.authMethod) {
                            ForEach(ServerAccount.AuthMethod.allCases, id: \.self) { method in
                                Text(method.label).tag(method)
                            }
                        }
                        .foregroundStyle(Theme.textPrimary)

                        switch viewModel.authMethod {
                        case .bearer, .oauthGitHub, .oauthGitLab, .oauthGoogle, .oauthMicrosoft, .oidc:
                            LabeledField(title: "Token") {
                                SecureField(
                                    "",
                                    text: $viewModel.bearerToken,
                                    prompt: Text("API or OAuth token").foregroundStyle(Theme.muted)
                                )
                                .foregroundStyle(Theme.textPrimary)
                            }
                        case .basic:
                            LabeledField(title: "Username") {
                                TextField(
                                    "",
                                    text: $viewModel.username,
                                    prompt: Text("Your username").foregroundStyle(Theme.muted)
                                )
                                .textInputAutocapitalization(.never)
                                .textContentType(.username)
                                .autocorrectionDisabled()
                                .foregroundStyle(Theme.textPrimary)
                            }
                            LabeledField(title: "Password") {
                                SecureField(
                                    "",
                                    text: $viewModel.password,
                                    prompt: Text("Your password").foregroundStyle(Theme.muted)
                                )
                                .textContentType(.password)
                                .foregroundStyle(Theme.textPrimary)
                            }
                        }
                    } header: {
                        FormSectionHeader(title: "Connect your own server")
                    }
                    .listRowBackground(Theme.surface)

                    Section {
                        Toggle("Require Face ID on launch", isOn: $viewModel.biometricLock)
                            .foregroundStyle(Theme.textPrimary)
                    } header: {
                        FormSectionHeader(title: "Security")
                    } footer: {
                        Text("Protect cluster credentials with Face ID or device passcode when reopening the app.")
                            .foregroundStyle(Theme.muted)
                    }
                    .listRowBackground(Theme.surface)

                    if let error = viewModel.errorMessage {
                        Section {
                            Label {
                                Text(error).foregroundStyle(Theme.textSecondary)
                            } icon: {
                                Image(systemName: "exclamationmark.triangle.fill")
                                    .foregroundStyle(Theme.danger)
                            }
                            .font(.subheadline)
                        }
                        .listRowBackground(Theme.surface)
                        .accessibilityAddTraits(.isStaticText)
                    }

                    Section {
                        ThemedPrimaryButton(
                            title: viewModel.isConnecting ? "Connecting…" : "Connect",
                            isLoading: viewModel.isConnecting
                        ) {
                            Task { await viewModel.connect(using: appState.authManager) }
                        }
                        .listRowInsets(EdgeInsets())
                        .listRowBackground(Color.clear)
                        .disabled(viewModel.isConnecting || viewModel.serverURL.isEmpty)
                    }
                }
                .themedForm()
            }
            .navigationTitle("Welcome")
            .navigationBarTitleDisplayMode(.inline)
            .themedScreen()
        }
    }
}

/// One-tap entry to the hosted demo. Deliberately the most prominent control on the
/// screen: for anyone without a KubePilot backend it is the only path that works.
private struct DemoLaunchCard: View {
    let isBusy: Bool
    let action: () -> Void

    var body: some View {
        SurfaceCard {
            VStack(alignment: .leading, spacing: Theme.spacingMD) {
                HStack(spacing: Theme.spacingSM) {
                    Image(systemName: "play.circle.fill")
                        .font(.title2)
                        .foregroundStyle(Theme.accentLight)
                        .symbolRenderingMode(.hierarchical)

                    VStack(alignment: .leading, spacing: 2) {
                        Text("Try the live demo")
                            .font(.headline)
                            .foregroundStyle(Theme.textPrimary)
                        Text("No account or cluster needed")
                            .font(.caption)
                            .foregroundStyle(Theme.muted)
                    }

                    Spacer(minLength: 0)
                }

                Text("Opens a sample cluster with running and crash-looping pods, live logs, events and AI root-cause analysis.")
                    .font(.footnote)
                    .foregroundStyle(Theme.textSecondary)
                    .fixedSize(horizontal: false, vertical: true)

                ThemedSecondaryButton(
                    title: isBusy ? "Starting demo…" : "Start demo",
                    systemImage: isBusy ? nil : "arrow.right"
                ) {
                    action()
                }
                .disabled(isBusy)
                .opacity(isBusy ? 0.7 : 1)
            }
        }
        .overlay(alignment: .topTrailing) {
            Image(systemName: "sparkles")
                .font(.caption)
                .foregroundStyle(Theme.purple)
                .padding(Theme.spacingMD)
                .accessibilityHidden(true)
        }
    }
}

/// Field with a persistent visible label. A placeholder alone disappears the moment
/// someone types, which is the single most common accessibility miss in iOS forms.
private struct LabeledField<Content: View>: View {
    let title: String
    @ViewBuilder let content: () -> Content

    var body: some View {
        VStack(alignment: .leading, spacing: 5) {
            Text(title)
                .font(.caption2.weight(.semibold))
                .tracking(0.5)
                .foregroundStyle(Theme.muted)
                .textCase(.uppercase)
            content()
                .font(Theme.Typography.identifier)
        }
        .padding(.vertical, 4)
        .accessibilityElement(children: .combine)
        .accessibilityLabel(title)
    }
}

@MainActor
@Observable
final class OnboardingViewModel {
    // Default to the hosted demo so App Review (and anyone trying the app for
    // the first time) lands on a server that actually answers. localhost:8383
    // only works for a developer running the backend on the same machine.
    var serverURL = DemoFixtures.hostedURL.absoluteString
    var authMethod: ServerAccount.AuthMethod = .basic
    var bearerToken = ""
    var username = ""
    var password = ""
    var biometricLock = true
    var isConnecting = false
    var errorMessage: String?

    func tryDemo(using auth: AuthManager) async {
        serverURL = DemoFixtures.hostedURL.absoluteString
        authMethod = .basic
        username = DemoFixtures.username
        password = DemoFixtures.password
        bearerToken = ""
        isConnecting = true
        errorMessage = nil
        defer { isConnecting = false }

        do {
            let ok = try await auth.testConnection(
                serverURL: DemoFixtures.hostedURL,
                authMethod: .basic,
                bearerToken: nil,
                username: DemoFixtures.username,
                password: DemoFixtures.password
            )
            if ok {
                try await saveAccount(
                    using: auth,
                    url: DemoFixtures.hostedURL,
                    displayName: "KubePilot Demo",
                    authMethod: .basic,
                    username: DemoFixtures.username,
                    password: DemoFixtures.password,
                    provider: "Demo",
                    environment: .development
                )
                return
            }
        } catch {
            // Fall through to offline fixtures.
        }

        do {
            try await saveAccount(
                using: auth,
                url: DemoFixtures.offlineBaseURL,
                displayName: "KubePilot Demo (Offline)",
                authMethod: .basic,
                username: DemoFixtures.username,
                password: DemoFixtures.password,
                provider: "Demo",
                environment: .development
            )
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func connect(using auth: AuthManager) async {
        guard let url = URL(string: serverURL.trimmingCharacters(in: .whitespacesAndNewlines)) else {
            errorMessage = "Enter a valid server URL."
            return
        }
        isConnecting = true
        errorMessage = nil
        defer { isConnecting = false }

        do {
            let ok = try await auth.testConnection(
                serverURL: url,
                authMethod: authMethod,
                bearerToken: bearerToken.isEmpty ? nil : bearerToken,
                username: username.isEmpty ? nil : username,
                password: password.isEmpty ? nil : password
            )
            guard ok else {
                errorMessage = "Server did not respond to health check."
                return
            }

            try await saveAccount(
                using: auth,
                url: url,
                displayName: url.host ?? "KubePilot",
                authMethod: authMethod,
                bearerToken: bearerToken.isEmpty ? nil : bearerToken,
                username: username.isEmpty ? nil : username,
                password: password.isEmpty ? nil : password
            )
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func saveAccount(
        using auth: AuthManager,
        url: URL,
        displayName: String,
        authMethod: ServerAccount.AuthMethod,
        bearerToken: String? = nil,
        username: String? = nil,
        password: String? = nil,
        provider: String = "Custom",
        environment: ClusterProfile.ClusterEnvironment = .production
    ) async throws {
        let cluster = ClusterProfile(
            id: UUID().uuidString,
            name: url.host ?? "Cluster",
            serverURL: url,
            provider: provider,
            region: "",
            colorHex: "#3b82f6",
            environment: environment,
            isFavorite: true,
            lastConnectedAt: .now
        )

        let account = ServerAccount(
            id: UUID().uuidString,
            displayName: displayName,
            baseURL: url,
            authMethod: authMethod,
            bearerToken: bearerToken,
            username: username,
            password: password,
            clusters: [cluster],
            activeClusterID: cluster.id,
            biometricLockEnabled: biometricLock,
            createdAt: .now
        )
        try await auth.addAccount(account)
    }
}
