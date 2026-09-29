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
                            .listRowInsets(EdgeInsets(top: 12, leading: 0, bottom: 12, trailing: 0))
                    }

                    Section {
                        TextField("https://kubepilot.example.com", text: $viewModel.serverURL)
                            .textInputAutocapitalization(.never)
                            .keyboardType(.URL)
                            .autocorrectionDisabled()
                            .foregroundStyle(Theme.textPrimary)
                    } header: {
                        Text("Server")
                    }
                    .listRowBackground(Theme.surface)

                    Section {
                        Picker("Method", selection: $viewModel.authMethod) {
                            ForEach(ServerAccount.AuthMethod.allCases, id: \.self) { method in
                                Text(method.label).tag(method)
                            }
                        }

                        switch viewModel.authMethod {
                        case .bearer, .oauthGitHub, .oauthGitLab, .oauthGoogle, .oauthMicrosoft, .oidc:
                            SecureField("API Token / OAuth Token", text: $viewModel.bearerToken)
                        case .basic:
                            TextField("Username", text: $viewModel.username)
                                .textInputAutocapitalization(.never)
                            SecureField("Password", text: $viewModel.password)
                        }
                    } header: {
                        Text("Authentication")
                    }
                    .listRowBackground(Theme.surface)

                    Section {
                        Toggle("Require Face ID on launch", isOn: $viewModel.biometricLock)
                    } header: {
                        Text("Security")
                    } footer: {
                        Text("Protect cluster credentials with Face ID or device passcode when reopening the app.")
                            .foregroundStyle(Theme.muted)
                    }
                    .listRowBackground(Theme.surface)

                    if let error = viewModel.errorMessage {
                        Section {
                            Label(error, systemImage: "exclamationmark.triangle.fill")
                                .foregroundStyle(Theme.danger)
                                .font(.subheadline)
                        }
                        .listRowBackground(Theme.surface)
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

                        Button {
                            Task { await viewModel.tryDemo(using: appState.authManager) }
                        } label: {
                            Text(viewModel.isConnecting ? "Starting demo…" : "Try demo")
                                .frame(maxWidth: .infinity)
                                .font(.body.weight(.semibold))
                        }
                        .disabled(viewModel.isConnecting)
                        .listRowBackground(Color.clear)
                    } footer: {
                        Text("Try demo connects to the App Review fixture at demo.kubepilot.org (apple / review), or loads the same data offline if the host is unreachable.")
                            .foregroundStyle(Theme.muted)
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

@MainActor
@Observable
final class OnboardingViewModel {
    var serverURL = "http://localhost:8383"
    var authMethod: ServerAccount.AuthMethod = .bearer
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
        environment: ClusterEnvironment = .production
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
