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

                    Section("Server") {
                        TextField("https://kubepilot.example.com", text: $viewModel.serverURL)
                            .textInputAutocapitalization(.never)
                            .keyboardType(.URL)
                            .autocorrectionDisabled()
                    }
                    .listRowBackground(Theme.surface)

                    Section("Authentication") {
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
                    }
                    .listRowBackground(Theme.surface)

                    Section("Security") {
                        Toggle("Require Face ID on launch", isOn: $viewModel.biometricLock)
                    }
                    .listRowBackground(Theme.surface)

                    if let error = viewModel.errorMessage {
                        Section {
                            Text(error).foregroundStyle(Theme.danger)
                        }
                        .listRowBackground(Theme.surface)
                    }

                    Section {
                        Button {
                            Task { await viewModel.connect(using: appState.authManager) }
                        } label: {
                            HStack {
                                if viewModel.isConnecting {
                                    ProgressView().padding(.trailing, 4)
                                }
                                Text(viewModel.isConnecting ? "Connecting…" : "Connect")
                                    .frame(maxWidth: .infinity)
                            }
                        }
                        .disabled(viewModel.isConnecting || viewModel.serverURL.isEmpty)
                    }
                    .listRowBackground(Theme.surface)
                }
                .scrollContentBackground(.hidden)
            }
            .navigationTitle("Welcome")
            .navigationBarTitleDisplayMode(.inline)
            .toolbarBackground(Theme.brandBg.opacity(0.95), for: .navigationBar)
            .toolbarBackground(.visible, for: .navigationBar)
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

            let cluster = ClusterProfile(
                id: UUID().uuidString,
                name: url.host ?? "Cluster",
                serverURL: url,
                provider: "Custom",
                region: "",
                colorHex: "#3b82f6",
                environment: .production,
                isFavorite: true,
                lastConnectedAt: .now
            )

            let account = ServerAccount(
                id: UUID().uuidString,
                displayName: url.host ?? "KubePilot",
                baseURL: url,
                authMethod: authMethod,
                bearerToken: bearerToken.isEmpty ? nil : bearerToken,
                username: username.isEmpty ? nil : username,
                password: password.isEmpty ? nil : password,
                clusters: [cluster],
                activeClusterID: cluster.id,
                biometricLockEnabled: biometricLock,
                createdAt: .now
            )
            try await auth.addAccount(account)
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}
