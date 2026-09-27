import Foundation
import XCTest
@testable import RemoteProtocol

final class HandshakeTests: XCTestCase {
    func testAuthenticationGoldenVector() throws {
        let vector = try loadAuthVector()
        let response = try Authentication.response(
            deviceKey: Data(hexadecimal: vector.deviceKeyHex),
            controllerNonce: Data(hexadecimal: vector.controllerNonceHex),
            agentNonce: Data(hexadecimal: vector.agentNonceHex),
            challenge: Data(hexadecimal: vector.challengeHex),
            agentIdentifier: Data(hexadecimal: vector.agentIdentifierHex)
        )

        XCTAssertEqual(response, Data(hexadecimal: vector.responseHex))
        XCTAssertTrue(Authentication.constantTimeEquals(response, Data(hexadecimal: vector.responseHex)))
        XCTAssertFalse(Authentication.constantTimeEquals(response, Data(repeating: 0, count: 32)))
    }

    func testHelloRoundTrip() throws {
        let hello = HelloPayload(
            role: .controller,
            capabilities: [.jpeg, .reconnect],
            nonce: Data(0..<32)
        )

        XCTAssertEqual(try HelloPayload.decode(hello.encode()), hello)
    }

    func testRejectsMalformedHandshakePayloads() {
        XCTAssertThrowsError(try HelloPayload.decode(Data(repeating: 0, count: 40)))
        XCTAssertThrowsError(try AuthChallengePayload.decode(Data(repeating: 0, count: 47)))
        XCTAssertThrowsError(try AuthResultPayload.decode(Data([0, 0, 0, 0, 1])))
    }

    func testVideoIsRejectedBeforeAuthentication() throws {
        var gate = SessionGate(localRole: .controller)
        let video = Frame(type: .videoFrameJPEG, sequence: 1, payload: Data([0xff, 0xd8]))

        XCTAssertThrowsError(try gate.receive(video)) { error in
            XCTAssertEqual(error as? ProtocolError, .authRequired)
        }
    }

    func testControllerTransitionsToAuthenticated() throws {
        var gate = SessionGate(localRole: .controller)
        try gate.receive(Frame(
            type: .hello,
            sequence: 1,
            payload: HelloPayload(
                role: .agent,
                capabilities: [.jpeg],
                nonce: Data(repeating: 1, count: 32)
            ).encode()
        ))
        XCTAssertEqual(gate.phase, .authenticating)

        try gate.receive(Frame(
            type: .authChallenge,
            sequence: 2,
            payload: AuthChallengePayload(
                challenge: Data(repeating: 2, count: 32),
                agentIdentifier: Data(repeating: 3, count: 16)
            ).encode()
        ))
        try gate.receive(Frame(
            type: .authResult,
            sequence: 3,
            payload: AuthResultPayload(status: .success, retryDelayMilliseconds: 0).encode()
        ))

        XCTAssertEqual(gate.phase, .authenticated)
        XCTAssertNoThrow(try gate.receive(Frame(type: .videoFrameJPEG, sequence: 4)))
    }

    func testAgentRequiresVerifiedResponseBeforeAuthentication() throws {
        var gate = SessionGate(localRole: .agent)
        try gate.receive(Frame(
            type: .hello,
            sequence: 1,
            payload: HelloPayload(
                role: .controller,
                capabilities: [.jpeg],
                nonce: Data(repeating: 4, count: 32)
            ).encode()
        ))
        XCTAssertThrowsError(try gate.completeAgentAuthentication(expectedResponse: Data(repeating: 5, count: 32)))

        try gate.receive(Frame(
            type: .authResponse,
            sequence: 2,
            payload: Data(repeating: 5, count: 32)
        ))
        try gate.completeAgentAuthentication(expectedResponse: Data(repeating: 5, count: 32))

        XCTAssertEqual(gate.phase, .authenticated)

        var rejectedGate = gateForAgentAuthentication(responseByte: 6)
        try rejectedGate.completeAgentAuthentication(expectedResponse: Data(repeating: 7, count: 32))
        XCTAssertEqual(rejectedGate.phase, .closing)
    }

    private func gateForAgentAuthentication(responseByte: UInt8) -> SessionGate {
        var gate = SessionGate(localRole: .agent)
        try! gate.receive(Frame(
            type: .hello,
            sequence: 1,
            payload: try! HelloPayload(
                role: .controller,
                capabilities: [.jpeg],
                nonce: Data(repeating: 4, count: 32)
            ).encode()
        ))
        try! gate.receive(Frame(
            type: .authResponse,
            sequence: 2,
            payload: Data(repeating: responseByte, count: 32)
        ))
        return gate
    }
}

private struct AuthVector: Decodable {
    let deviceKeyHex: String
    let controllerNonceHex: String
    let agentNonceHex: String
    let challengeHex: String
    let agentIdentifierHex: String
    let responseHex: String
}

private func loadAuthVector() throws -> AuthVector {
    var root = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
    for _ in 0..<4 { root.deleteLastPathComponent() }
    let data = try Data(contentsOf: root.appendingPathComponent("protocol/testdata/auth-v1.json"))
    return try JSONDecoder().decode(AuthVector.self, from: data)
}

private extension Data {
    init(hexadecimal: String) {
        self.init(capacity: hexadecimal.count / 2)
        var index = hexadecimal.startIndex
        while index < hexadecimal.endIndex {
            let next = hexadecimal.index(index, offsetBy: 2)
            append(UInt8(hexadecimal[index..<next], radix: 16)!)
            index = next
        }
    }
}
