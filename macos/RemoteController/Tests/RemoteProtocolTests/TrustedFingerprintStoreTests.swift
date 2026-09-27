import Foundation
import XCTest
@testable import RemoteProtocol

final class TrustedFingerprintStoreTests: XCTestCase {
    func testApprovedFirstUseIsStoredAndThenTrusted() throws {
        let store = MemoryFingerprintStore()
        let coordinator = StoredCertificateTrustCoordinator(store: store)
        let certificate = Data(repeating: 10, count: 64)
        var approvalCalls = 0

        let first = try coordinator.evaluate(
            deviceIdentifier: "windows-agent",
            certificateDER: certificate
        ) { _ in
            approvalCalls += 1
            return true
        }
        let second = try coordinator.evaluate(
            deviceIdentifier: "windows-agent",
            certificateDER: certificate
        ) { _ in
            approvalCalls += 1
            return false
        }

        guard case .approvedAndStored = first else { return XCTFail("Expected first-use approval") }
        XCTAssertEqual(second, .trusted)
        XCTAssertEqual(approvalCalls, 1)
    }

    func testRejectedFirstUseIsNotStored() throws {
        let store = MemoryFingerprintStore()
        let coordinator = StoredCertificateTrustCoordinator(store: store)

        let result = try coordinator.evaluate(
            deviceIdentifier: "windows-agent",
            certificateDER: Data(repeating: 11, count: 64),
            approveFirstUse: { _ in false }
        )

        guard case .rejectedFirstUse = result else { return XCTFail("Expected first-use rejection") }
        XCTAssertNil(try store.loadFingerprint(for: "windows-agent"))
    }

    func testChangedCertificateIsRejectedWithoutApprovalPrompt() throws {
        let store = MemoryFingerprintStore()
        let coordinator = StoredCertificateTrustCoordinator(store: store)
        _ = try coordinator.evaluate(
            deviceIdentifier: "windows-agent",
            certificateDER: Data(repeating: 12, count: 64),
            approveFirstUse: { _ in true }
        )
        var approvalCalled = false

        let result = try coordinator.evaluate(
            deviceIdentifier: "windows-agent",
            certificateDER: Data(repeating: 13, count: 64)
        ) { _ in
            approvalCalled = true
            return true
        }

        XCTAssertEqual(result, .rejectedFingerprintMismatch)
        XCTAssertFalse(approvalCalled)
    }
}

private final class MemoryFingerprintStore: TrustedFingerprintStore {
    private var fingerprints: [String: CertificateFingerprint] = [:]

    func loadFingerprint(for deviceIdentifier: String) throws -> CertificateFingerprint? {
        fingerprints[deviceIdentifier]
    }

    func saveFingerprint(
        _ fingerprint: CertificateFingerprint,
        for deviceIdentifier: String
    ) throws {
        fingerprints[deviceIdentifier] = fingerprint
    }

    func removeFingerprint(for deviceIdentifier: String) throws {
        fingerprints.removeValue(forKey: deviceIdentifier)
    }
}
