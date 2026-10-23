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

        XCTAssertEqual(thirdParty.kind, .provider("OpenAI"))

        let unknown = AIRecipient(serverURL: server, health: nil)
        XCTAssertNil(unknown.endpointHost)
        XCTAssertEqual(unknown.kind, .unknown)
    }

    func testRecipientKindNamesPrivateAndThirdPartyEndpoints() {
        func kind(_ baseURL: String) -> AIRecipient.Kind {
            AIRecipient(
                serverURL: URL(string: "https://k3s1.example.com"),
                health: AIHealthResponse(healthy: true, model: "m", baseURL: baseURL, latencyMs: 1, error: nil)
            ).kind
        }
        XCTAssertEqual(kind("http://ollama.ai.svc:11434/v1"), .privateNetwork)
        XCTAssertEqual(kind("http://ollama:11434/v1"), .privateNetwork)
        XCTAssertEqual(kind("http://192.168.1.20:11434/v1"), .privateNetwork)
        XCTAssertEqual(kind("http://172.20.0.5/v1"), .privateNetwork)
        XCTAssertEqual(kind("https://api.anthropic.com/v1"), .provider("Anthropic"))
        XCTAssertEqual(kind("https://myco.openai.azure.com/openai"), .provider("Microsoft Azure OpenAI"))
        XCTAssertEqual(kind("https://llm.example.net/v1"), .thirdParty)
        XCTAssertEqual(kind("http://8.8.8.8/v1"), .thirdParty)
    }

    func testAnsweringEitherWayCountsAsAnswered() {
        let account = "test-\(UUID().uuidString)"
        XCTAssertFalse(AIConsent.hasAnswered(account: account))
        AIConsent.set(false, account: account)
        XCTAssertTrue(AIConsent.hasAnswered(account: account))
        XCTAssertFalse(AIConsent.isGranted(account: account))
    }
}
