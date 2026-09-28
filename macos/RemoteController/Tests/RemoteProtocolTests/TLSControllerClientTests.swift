import Dispatch
import XCTest
@testable import RemoteProtocol

final class TLSControllerClientTests: XCTestCase {
    func testProbeCompletesAfterRunProbeReturns() {
        // Prevent callbacks from running until the local state has left scope.
        let queue = DispatchQueue(label: "tls-probe-lifetime-test")
        queue.suspend()
        let client = TLSControllerClient(trustStore: UnusedFingerprintStore(), queue: queue)
        let completed = expectation(description: "probe retains state until completion")
        completed.assertForOverFulfill = true
        client.runProbe(
            host: "127.0.0.1",
            port: 9,
            deviceIdentifier: "loopback-test",
            deviceKey: Data(repeating: 1, count: 32),
            timeout: 0.05,
            approveFirstUse: { _ in false },
            completion: { result in
                if case .success = result {
                    XCTFail("No successful TLS endpoint is expected")
                }
                completed.fulfill()
            }
        )
        queue.resume()
        wait(for: [completed], timeout: 2)
        withExtendedLifetime(client) {}
    }
}

private struct UnusedFingerprintStore: TrustedFingerprintStore {
    func loadFingerprint(for deviceIdentifier: String) throws -> CertificateFingerprint? { nil }
    func saveFingerprint(_ fingerprint: CertificateFingerprint, for deviceIdentifier: String) throws {
        XCTFail("This test must not persist trust")
    }
    func removeFingerprint(for deviceIdentifier: String) throws {}
}
