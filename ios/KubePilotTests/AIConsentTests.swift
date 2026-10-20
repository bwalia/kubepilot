import XCTest
@testable import KubePilot

final class AIConsentTests: XCTestCase {
    func testConsentIsOffUntilGrantedAndScopedPerAccount() {
        let account = "test-\(UUID().uuidString)"
        XCTAssertFalse(AIConsent.isGranted(account: account))
        XCTAssertFalse(AIConsent.isGranted(account: nil))

        AIConsent.set(true, account: account)
        XCTAssertTrue(AIConsent.isGranted(account: account))
        XCTAssertFalse(AIConsent.isGranted(account: account + "-other"))

        AIConsent.set(false, account: account)
        XCTAssertFalse(AIConsent.isGranted(account: account))
    }

    func testRecipientDistinguishesOnServerModelFromRemoteService() {
        let server = URL(string: "https://k3s1.example.com")!
        let local = AIHealthResponse(healthy: true, model: "llama3.2:3b", baseURL: "http://127.0.0.1:11438/v1", latencyMs: 5, error: nil)
        let remote = AIHealthResponse(healthy: true, model: "gpt-4o", baseURL: "https://api.openai.com/v1", latencyMs: 90, error: nil)

        let onServer = AIRecipient(serverURL: server, health: local)
        XCTAssertTrue(onServer.isOnServer)
        XCTAssertEqual(onServer.serverHost, "k3s1.example.com")
        XCTAssertEqual(onServer.model, "llama3.2:3b")

        let thirdParty = AIRecipient(serverURL: server, health: remote)
        XCTAssertFalse(thirdParty.isOnServer)
        XCTAssertEqual(thirdParty.endpointHost, "api.openai.com")

        XCTAssertNil(AIRecipient(serverURL: server, health: nil).endpointHost)
    }
}
