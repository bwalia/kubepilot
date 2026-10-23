import XCTest
@testable import KubePilot

final class PodUsageTests: XCTestCase {
    private func report() throws -> PodUsageReport {
        try JSONDecoder.kubePilot.decode(PodUsageReport.self, from: Data(DemoFixtures.podUsageJSON.utf8))
    }

    func testDecodesPodUsageReport() throws {
        let r = try report()
        XCTAssertTrue(r.metricsAvailable)
        XCTAssertEqual(r.pods.count, 5)
        XCTAssertEqual(r.pods[1].ownerKind, "StatefulSet")
        XCTAssertNil(r.pods[0].storageEstimated)
        XCTAssertEqual(r.pods[4].storageEstimated, true)
    }

    func testTopRanksByMetricAndSkipsZero() throws {
        let pods = try report().pods
        XCTAssertEqual(UsageMetric.cpu.top(pods, 2).map(\.name), ["fraud-scorer-5b8d6", "checkout-api-7d9f8c"])
        XCTAssertEqual(UsageMetric.storage.top(pods, 1).map(\.name), ["prometheus-server-0"])
        XCTAssertEqual(UsageMetric.memory.top(pods, 10).count, 5)
    }

    func testPressureUsesLimitThenRequest() throws {
        let pods = try report().pods
        let checkout = pods[0]
        XCTAssertEqual(UsageMetric.cpu.pressure(of: checkout)?.pct ?? 0, 82, accuracy: 0.01)
        XCTAssertEqual(UsageMetric.cpu.pressure(of: checkout)?.of, "limit")
        XCTAssertEqual(UsageMetric.cpu.pressure(of: pods[3])?.of, "request")
        XCTAssertNil(UsageMetric.storage.pressure(of: pods[4]), "estimated storage has no pressure")
    }

    func testFormatting() {
        XCTAssertEqual(UsageMetric.formatCPU(0), "0")
        XCTAssertEqual(UsageMetric.formatCPU(820), "820m")
        XCTAssertEqual(UsageMetric.formatCPU(1340), "1.34 cores")
        XCTAssertEqual(UsageMetric.formatBytes(503316480), "480 MiB")
        XCTAssertEqual(UsageMetric.formatBytes(6442450944), "6.0 GiB")
    }
}
