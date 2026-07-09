import SwiftUI
import LocalAuthentication

struct BiometricUnlockView: View {
    @Environment(AppState.self) private var appState
    @State private var errorMessage: String?

    var body: some View {
        ZStack {
            BrandScreenBackground()

            VStack(spacing: 28) {
                KubePilotMark(size: 96)

                VStack(spacing: 8) {
                    KubePilotWordmark(size: .large)
                    Text("Authenticate to access your clusters.")
                        .font(.subheadline)
                        .foregroundStyle(Theme.textSecondary)
                        .multilineTextAlignment(.center)
                }

                if let errorMessage {
                    Text(errorMessage)
                        .font(.caption)
                        .foregroundStyle(Theme.danger)
                        .multilineTextAlignment(.center)
                }

                VStack(spacing: 12) {
                    Button("Unlock with Face ID") {
                        Task { await unlockBiometric() }
                    }
                    .buttonStyle(.borderedProminent)
                    .tint(Theme.accent)

                    Button("Use Passcode") {
                        Task { await unlockPasscode() }
                    }
                    .buttonStyle(.bordered)
                    .tint(Theme.accentLight)
                }
                .padding(.top, 8)
            }
            .padding()
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    private func unlockBiometric() async {
        do {
            try await appState.authManager.unlockWithBiometrics()
            errorMessage = nil
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func unlockPasscode() async {
        do {
            try await appState.authManager.unlockWithPasscode()
            errorMessage = nil
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}
