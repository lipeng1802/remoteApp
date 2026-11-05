import XCTest
@testable import RemoteProtocol

final class OwnedDesktopHandoffTests: XCTestCase {
    private let key = Data(repeating: 255, count: 32).base64EncodedString()
    private let fingerprint = String(repeating: "ab", count: 32)
    private let target = String(repeating: "cd", count: 32)

    private func bootstrap(port: String = "47479", key: String? = nil,
                           fingerprint: String? = nil, target: String? = nil,
                           permission: String = "desktop-control") -> String {
        "{\"Fingerprint\":\"\(fingerprint ?? self.fingerprint)\",\"Key\":\"\(key ?? self.key)\",\"Permission\":\"\(permission)\",\"Port\":\(port),\"Target\":\"\(target ?? self.target)\"}"
    }

    func testCanonicalOwnerHandoffSingleUseAndRedacted() throws {
        let handoff = try OwnedDesktopHandoff(privateBootstrap: Data(bootstrap().utf8))
        XCTAssertFalse(String(reflecting: handoff).contains(key))
        let client = try handoff.makeControlClient(localControlAllowed: true,
            onJpegFrame: { _, _ in }, onAuthenticated: {}, completion: { _ in })
        // Factory creates no connection until start and never accepts host/TOFU.
        client.cancel()
        XCTAssertThrowsError(try handoff.makeControlClient(localControlAllowed: true,
            onJpegFrame: { _, _ in }, onAuthenticated: {}, completion: { _ in })) {
            XCTAssertEqual($0 as? OwnedDesktopHandoffError, .consumed)
        }
    }

    func testConsentRefusalAndOwnerCancellationConsumeHandoff() throws {
        for cancel in [false, true] {
            let handoff = try OwnedDesktopHandoff(privateBootstrap: Data(bootstrap().utf8))
            if cancel { handoff.discard() }
            XCTAssertThrowsError(try handoff.makeControlClient(localControlAllowed: false,
                onJpegFrame: { _, _ in }, onAuthenticated: {}, completion: { _ in })) {
                XCTAssertEqual($0 as? OwnedDesktopHandoffError,
                    cancel ? .consumed : .localControlRequired)
            }
            XCTAssertThrowsError(try handoff.makeControlClient(localControlAllowed: true,
                onJpegFrame: { _, _ in }, onAuthenticated: {}, completion: { _ in }))
        }
    }

    func testViewGrantCannotEnableInputEvenWithLocalConsent() throws {
        let handoff = try OwnedDesktopHandoff(privateBootstrap: Data(bootstrap(permission: "desktop-view").utf8))
        XCTAssertThrowsError(try handoff.makeControlClient(localControlAllowed: true,
            onJpegFrame: { _, _ in }, onAuthenticated: {}, completion: { _ in })) {
            XCTAssertEqual($0 as? OwnedDesktopHandoffError, .controlNotGranted)
        }
        XCTAssertThrowsError(try handoff.makeControlClient(localControlAllowed: true,
            onJpegFrame: { _, _ in }, onAuthenticated: {}, completion: { _ in })) {
            XCTAssertEqual($0 as? OwnedDesktopHandoffError, .consumed)
        }
    }

    func testRefusesSelectorsDuplicateFieldsAndNoncanonicalValues() {
        let good = bootstrap()
        let invalid = [
            "", "null", "[]", good + "\n", good + good,
            good.replacingOccurrences(of: "\"Key\":", with: "\"Host\":\"127.0.0.1\",\"Key\":"),
            good.replacingOccurrences(of: "\"Key\":", with: "\"Key\":\"bad\",\"Key\":"),
            good.replacingOccurrences(of: "\"Port\":", with: "\"Control\":true,\"Port\":"),
            good.replacingOccurrences(of: "\"Port\":", with: "\"Preauth\":true,\"Port\":"),
            good.replacingOccurrences(of: "\"Port\":", with: "\"port\":1,\"Port\":"),
            good.replacingOccurrences(of: "\"Target\":", with: "\"Root\":\"/tmp\",\"Target\":"),
            bootstrap(port: "0"), bootstrap(port: "65536"), bootstrap(port: "-1"),
            bootstrap(port: "true"), bootstrap(port: "47479.0"), bootstrap(port: "4.7479e4"),
            bootstrap(port: "\"47479\""), bootstrap(key: key + "="),
            bootstrap(key: Data(count: 31).base64EncodedString()),
            bootstrap(fingerprint: fingerprint.uppercased()), bootstrap(fingerprint: "ab"),
            bootstrap(target: "hostname"), bootstrap(target: target.uppercased()),
            bootstrap(permission: "fixed-payload"), bootstrap(permission: ""),
            bootstrap(permission: "desktop-control "), bootstrap(permission: "clipboard"),
            good.replacingOccurrences(of: "\"Permission\":\"desktop-control\",", with: ""),
            String(repeating: " ", count: 8192) + good
        ]
        for raw in invalid {
            XCTAssertThrowsError(try OwnedDesktopHandoff(privateBootstrap: Data(raw.utf8))) {
                XCTAssertEqual($0 as? OwnedDesktopHandoffError, .invalidBootstrap)
            }
        }
    }
}
