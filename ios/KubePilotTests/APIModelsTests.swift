import XCTest
@testable import KubePilot

final class APIModelsTests: XCTestCase {
    func testPodSummaryDecodesPascalCase() throws {
        let json = """
        {
          "Name": "api-1",
          "Namespace": "payments",
          "Phase": "Running",
          "Reason": "",
          "NodeName": "worker-1",
          "Restarts": 2,
          "Ready": true,
          "Uptime": "3h"
        }
        """.data(using: .utf8)!

        let pod = try JSONDecoder.kubePilot.decode(PodSummary.self, from: json)
        XCTAssertEqual(pod.name, "api-1")
        XCTAssertEqual(pod.namespace, "payments")
        XCTAssertEqual(pod.restarts, 2)
        XCTAssertTrue(pod.ready)
    }

    func testNodeSummaryLANWANTunnel() {
        let node = NodeSummary(
            name: "n1",
            ready: true,
            memoryPressure: false,
            diskPressure: false,
            pidPressure: false,
            cpuCapacity: "4",
            memoryCapacity: "8Gi",
            kubeletVersion: "v1.29.0",
            internalIP: "LAN: 192.168.1.10 | WAN: 203.0.113.10 | Tunnel: 10.8.0.2",
            ips: ["10.8.0.2", "192.168.1.10", "203.0.113.10"],
            lanIPs: ["192.168.1.10"],
            wanIPs: ["203.0.113.10"],
            tunnelIPs: ["10.8.0.2"],
            unschedulable: false
        )
        XCTAssertEqual(node.lanIPs, ["192.168.1.10"])
        XCTAssertEqual(node.wanIPs, ["203.0.113.10"])
        XCTAssertEqual(node.tunnelIPs, ["10.8.0.2"])
        XCTAssertEqual(node.allIPs.count, 3)
    }

    func testPodSeverityCrashLoop() {
        let pod = PodSummary(
            name: "p",
            namespace: "ns",
            phase: "Running",
            reason: "CrashLoopBackOff",
            nodeName: "n1",
            restarts: 10,
            ready: false,
            uptime: "5m"
        )
        XCTAssertEqual(pod.severity, .critical)
    }
}

final class PodFilterTests: XCTestCase {
    @MainActor
    func testCrashLoopFilter() {
        let pods = [
            PodSummary(name: "a", namespace: "ns", phase: "Running", reason: "CrashLoopBackOff", nodeName: "", restarts: 1, ready: false, uptime: ""),
            PodSummary(name: "b", namespace: "ns", phase: "Running", reason: "", nodeName: "", restarts: 0, ready: true, uptime: "")
        ]
        let vm = PodsListViewModel()
        vm.pods = pods
        vm.filter = .crashloop
        XCTAssertEqual(vm.filteredPods.count, 1)
        XCTAssertEqual(vm.filteredPods.first?.name, "a")
    }
}
