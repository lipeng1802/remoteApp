import XCTest
@testable import RemoteProtocol

final class AuthenticatedProbeSessionTests: XCTestCase {
    private let key = Data(repeating: 1, count: 32)
    private let nonce = Data(repeating: 2, count: 32)
    private let agentNonce = Data(repeating: 3, count: 32)
    private let ping = Data(repeating: 4, count: 8)

    private func challenged() throws -> AuthenticatedProbeSession {
        var session = try AuthenticatedProbeSession(deviceKey: key, nonce: nonce, ping: ping)
        let hello = try session.start()
        XCTAssertEqual(hello.type, .hello)
        XCTAssertEqual(hello.sequence, 1)
        XCTAssertEqual(try HelloPayload.decode(hello.payload).nonce, nonce)
        XCTAssertTrue(try session.receive(Frame(type: .hello, sequence: 1,
            payload: HelloPayload(role: .agent, capabilities: [], nonce: agentNonce).encode())).isEmpty)
        let challenge = AuthChallengePayload(challenge: Data(repeating: 5, count: 32),
                                             agentIdentifier: Data(repeating: 6, count: 16))
        let replies = try session.receive(Frame(type: .authChallenge, sequence: 2, payload: challenge.encode()))
        XCTAssertEqual(replies.count, 1)
        XCTAssertEqual(replies[0].type, .authResponse)
        XCTAssertEqual(replies[0].sequence, 2)
        XCTAssertEqual(replies[0].payload, try Authentication.response(deviceKey: key, controllerNonce: nonce,
            agentNonce: agentNonce, challenge: challenge.challenge, agentIdentifier: challenge.agentIdentifier))
        return session
    }
    func testAuthenticatedPingPong() throws {
        var session = try challenged()
        let replies = try session.receive(Frame(type: .authResult, sequence: 3,
            payload: AuthResultPayload(status: .success, retryDelayMilliseconds: 0).encode()))
        XCTAssertEqual(replies, [Frame(type: .ping, sequence: 3, payload: ping)])
        XCTAssertFalse(session.isComplete)
        XCTAssertTrue(try session.receive(Frame(type: .pong, sequence: 4, payload: ping)).isEmpty)
        XCTAssertTrue(session.isComplete)
        XCTAssertThrowsError(try session.receive(Frame(type: .pong, sequence: 5, payload: ping)))
    }
    func testRejectedAuthDoesNotProducePing() throws {
        var session = try challenged()
        XCTAssertThrowsError(try session.receive(Frame(type: .authResult, sequence: 3,
            payload: AuthResultPayload(status: .rejected, retryDelayMilliseconds: 1000).encode()))) {
            XCTAssertEqual($0 as? TLSProbeError, .authenticationRejected)
        }
        XCTAssertFalse(session.isComplete)
    }
    func testPreauthPongRejected() throws {
        var session = try challenged()
        XCTAssertThrowsError(try session.receive(Frame(type: .pong, sequence: 3, payload: ping))) {
            XCTAssertEqual($0 as? ProtocolError, .authRequired)
        }
    }
    func testDuplicateChallengeRejected() throws {
        var session = try challenged()
        XCTAssertThrowsError(try session.receive(Frame(type: .authChallenge, sequence: 3,
            payload: Data(repeating: 0, count: 48))))
    }
    func testBadSequenceRejected() throws {
        var session = try challenged()
        XCTAssertThrowsError(try session.receive(Frame(type: .authResult, sequence: 2,
            payload: AuthResultPayload(status: .success, retryDelayMilliseconds: 0).encode())))
    }
    func testBoundedDecoderSplitsAndCoalescing() throws {
        let first = Frame(type: .hello, sequence: 1,
            payload: try HelloPayload(role: .agent, capabilities: [], nonce: agentNonce).encode())
        let second = Frame(type: .authChallenge, sequence: 2, payload: Data(repeating: 0, count: 48))
        let bytes = try FrameCodec.encode(first) + FrameCodec.encode(second)
        var decoder = ProbeFrameDecoder()
        XCTAssertEqual(try decoder.append(bytes), [first, second])
        var split = ProbeFrameDecoder()
        var frames: [Frame] = []
        for byte in bytes { frames += try split.append(Data([byte])) }
        XCTAssertEqual(frames, [first, second])
    }
    func testBoundedDecoderRejectsHeaderBeforeBody() throws {
        let bytes = try FrameCodec.encode(Frame(type: .hello, sequence: 1, payload: Data(count: 65)))
        var decoder = ProbeFrameDecoder()
        XCTAssertThrowsError(try decoder.append(Data(bytes.prefix(28)))) {
            XCTAssertEqual($0 as? ProtocolError, .messageTooLarge(65))
        }
    }
    func testMismatchedPongRejected() throws {
        var session = try challenged()
        _ = try session.receive(Frame(type: .authResult, sequence: 3,
            payload: AuthResultPayload(status: .success, retryDelayMilliseconds: 0).encode()))
        XCTAssertThrowsError(try session.receive(Frame(type: .pong, sequence: 4, payload: Data(count: 8))))
    }
    func testKeychainKeyRoundTripAndIsolation() throws {
        let identifier = UUID().uuidString
        let store = KeychainDeviceKeyStore(service: "com.personalremotedesktop.tests.device-key.\(identifier)")
        defer { try? store.removeKey(for: identifier) }
        XCTAssertNil(try store.loadKey(for: identifier))
        try store.saveKey(key, for: identifier)
        XCTAssertEqual(try store.loadKey(for: identifier), key)
        try store.saveKey(nonce, for: identifier)
        XCTAssertEqual(try store.loadKey(for: identifier), nonce)
        XCTAssertThrowsError(try store.saveKey(Data(count: 31), for: identifier))
        XCTAssertEqual(try store.loadKey(for: identifier), nonce)
        XCTAssertNil(try KeychainTrustedFingerprintStore().loadFingerprint(for: identifier))
        try store.removeKey(for: identifier)
        XCTAssertNil(try store.loadKey(for: identifier))
    }
}