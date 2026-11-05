import XCTest
@testable import RemoteProtocol

final class OwnedControllerPreparationTests: XCTestCase {
    private let ownID = String(repeating: "ab", count: 32)
    private let requestID = String(repeating: "cd", count: 16)
    private func prepare(_ permission: String = "desktop-control") throws -> OwnedControllerPreparation {
        try OwnedControllerPreparation(privatePreparation: Data("{\"Op\":\"prepare\",\"Permission\":\"\(permission)\",\"Role\":\"controller\",\"Self\":\"\(ownID)\"}".utf8))
    }
    private func request(_ controller: String? = nil) -> Data {
        Data("{\"Controller\":\"\(controller ?? ownID)\",\"Op\":\"consent\",\"RequestID\":\"\(requestID)\"}".utf8)
    }
    private func bootstrap(_ permission: String = "desktop-control") -> Data {
        let key = Data(repeating: 255, count: 32).base64EncodedString()
        return Data("{\"Fingerprint\":\"\(String(repeating: "ef", count: 32))\",\"Key\":\"\(key)\",\"Permission\":\"\(permission)\",\"Port\":32123,\"Target\":\"\(String(repeating: "cd", count: 32))\"}".utf8)
    }
    func testMatchingDecisionThenSingleHandoffWithoutStartingNetwork() throws {
        for permission in ["desktop-view", "desktop-control"] {
            let p = try prepare(permission)
            XCTAssertEqual(String(data: p.preparedReply(), encoding: .utf8), "{\"Permission\":\"\(permission)\",\"Role\":\"controller\",\"status\":\"desktop_prepared\"}")
            XCTAssertThrowsError(try p.acceptBootstrap(privateBootstrap: bootstrap(permission)))
            let preview = try p.beginRequest(privateRequest: request())
            XCTAssertEqual(preview.requestID, requestID)
            XCTAssertEqual(preview.permission, permission)
            XCTAssertEqual(preview.controller, ownID)
            XCTAssertThrowsError(try p.acceptBootstrap(privateBootstrap: bootstrap(permission)))
            let reply = try p.decide(localUserAllowed: true)
            XCTAssertEqual(String(data: reply, encoding: .utf8), "{\"Allow\":true,\"Permission\":\"\(permission)\",\"RequestID\":\"\(requestID)\",\"status\":\"desktop_decision\"}")
            XCTAssertThrowsError(try p.decide(localUserAllowed: true))
            let handoff = try p.acceptBootstrap(privateBootstrap: bootstrap(permission))
            handoff.discard() // no network, Keychain, capture or input side effects
            XCTAssertThrowsError(try p.acceptBootstrap(privateBootstrap: bootstrap(permission)))
            XCTAssertFalse(String(reflecting: p).contains(ownID))
        }
    }
    func testRefusalCancellationAndWrongScopeNeverProduceHandoff() throws {
        for mode in ["deny", "cancel", "scope", "oversized"] {
            let p = try prepare("desktop-view")
            _ = try p.beginRequest(privateRequest: request())
            if mode == "cancel" { p.cancel() }
            else { _ = try p.decide(localUserAllowed: mode != "deny") }
            if mode == "oversized" {
                XCTAssertThrowsError(try p.acceptBootstrap(privateBootstrap: Data(repeating: 32, count: 8192)))
            } else {
                XCTAssertThrowsError(try p.acceptBootstrap(privateBootstrap: bootstrap("desktop-control")))
            }
            XCTAssertThrowsError(try p.acceptBootstrap(privateBootstrap: bootstrap("desktop-view")))
            XCTAssertThrowsError(try p.beginRequest(privateRequest: request()))
        }
    }
    func testNoncanonicalOrForeignRequestIsTerminal() throws {
        let valid = String(data: request(), encoding: .utf8)!
        for raw in [valid + "\n", valid.replacingOccurrences(of: "\"consent\"", with: "\"consent\",\"host\":\"remote\""),
                    valid.replacingOccurrences(of: "\"Op\":\"consent\"", with: "\"Op\":\"consent\",\"Op\":\"consent\""),
                    String(data: request(String(repeating: "cd", count: 32)), encoding: .utf8)!] {
            let p = try prepare()
            XCTAssertThrowsError(try p.beginRequest(privateRequest: Data(raw.utf8)))
            XCTAssertThrowsError(try p.beginRequest(privateRequest: request()))
            XCTAssertThrowsError(try p.decide(localUserAllowed: true))
        }
    }
    func testPreparationRejectsRoleScopeAndExtraSelectors() throws {
        let valid = "{\"Op\":\"prepare\",\"Permission\":\"desktop-view\",\"Role\":\"controller\",\"Self\":\"\(ownID)\"}"
        for raw in [valid.replacingOccurrences(of: "desktop-view", with: "fixed-payload"),
                    valid.replacingOccurrences(of: "controller", with: "target"),
                    valid.replacingOccurrences(of: "\"Op\":\"prepare\"", with: "\"Op\":\"prepare\",\"host\":\"remote\""),
                    valid + "\n"] {
            XCTAssertThrowsError(try OwnedControllerPreparation(privatePreparation: Data(raw.utf8)))
        }
    }
}
