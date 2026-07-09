import Foundation
import LocalAuthentication
import Observation

@MainActor
@Observable
final class AuthManager {
    private(set) var accounts: [ServerAccount] = []
    private(set) var activeAccount: ServerAccount?
    private(set) var requiresBiometricUnlock = false

    private let keychainAccount = "kubepilot.accounts"
    private var unlockedSession = false

    init() {
        Task { await loadAccounts() }
    }

    func loadAccounts() async {
        do {
            if let stored = try await KeychainStore.shared.load([ServerAccount].self, account: keychainAccount) {
                accounts = stored
                activeAccount = stored.first
                if activeAccount?.biometricLockEnabled == true {
                    requiresBiometricUnlock = true
                } else {
                    unlockedSession = true
                    await configureService()
                }
            }
        } catch {
            accounts = []
            activeAccount = nil
        }
    }

    func addAccount(_ account: ServerAccount) async throws {
        var updated = accounts.filter { $0.id != account.id }
        updated.insert(account, at: 0)
        accounts = updated
        activeAccount = account
        unlockedSession = true
        requiresBiometricUnlock = false
        try await persist()
        await configureService()
    }

    func switchAccount(_ account: ServerAccount) async {
        activeAccount = account
        try? await persist()
        await configureService()
    }

    func signOut() {
        activeAccount = nil
        unlockedSession = false
        requiresBiometricUnlock = false
    }

    func lockIfNeeded() {
        guard activeAccount?.biometricLockEnabled == true else { return }
        unlockedSession = false
        requiresBiometricUnlock = true
    }

    func unlockWithBiometrics() async throws {
        let context = LAContext()
        var error: NSError?
        guard context.canEvaluatePolicy(.deviceOwnerAuthenticationWithBiometrics, error: &error) else {
            throw BiometricError.unavailable
        }
        let success = try await context.evaluatePolicy(
            .deviceOwnerAuthenticationWithBiometrics,
            localizedReason: "Unlock KubePilot"
        )
        guard success else { throw BiometricError.failed }
        requiresBiometricUnlock = false
        unlockedSession = true
        await configureService()
    }

    func unlockWithPasscode() async throws {
        let context = LAContext()
        let success = try await context.evaluatePolicy(
            .deviceOwnerAuthentication,
            localizedReason: "Unlock KubePilot"
        )
        guard success else { throw BiometricError.failed }
        requiresBiometricUnlock = false
        unlockedSession = true
        await configureService()
    }

    func testConnection(serverURL: URL, authMethod: ServerAccount.AuthMethod, bearerToken: String?, username: String?, password: String?) async throws -> Bool {
        var probe = ServerAccount(
            id: UUID().uuidString,
            displayName: "Probe",
            baseURL: serverURL,
            authMethod: authMethod,
            bearerToken: bearerToken,
            username: username,
            password: password,
            clusters: [],
            activeClusterID: nil,
            biometricLockEnabled: false,
            createdAt: .now
        )
        await KubePilotService.shared.configure(account: probe)
        return try await KubePilotService.shared.ping()
    }

    private func persist() async throws {
        try await KeychainStore.shared.save(accounts, account: keychainAccount)
    }

    private func configureService() async {
        guard unlockedSession, let activeAccount else { return }
        await KubePilotService.shared.configure(account: activeAccount)
    }
}

enum BiometricError: LocalizedError {
    case unavailable
    case failed

    var errorDescription: String? {
        switch self {
        case .unavailable: "Biometric authentication is not available on this device."
        case .failed: "Authentication failed."
        }
    }
}
