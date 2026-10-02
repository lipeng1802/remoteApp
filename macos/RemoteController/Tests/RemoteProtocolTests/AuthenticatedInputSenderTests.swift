import Foundation
import XCTest
@testable import RemoteProtocol

final class AuthenticatedInputSenderTests: XCTestCase {
    private let key = Data(repeating: 1, count: 32)
    private let nonce = Data(repeating: 2, count: 32)
    private let agentNonce = Data(repeating: 3, count: 32)
    private let down = CapturedInput.button(MouseButtonPayload(button: .left, action: .down))
    private let up = CapturedInput.button(MouseButtonPayload(button: .left, action: .up))

    private func challenged(capacity: Int = 64, acceptsJpeg: Bool = false,
                            acceptsClipboard: Bool = false) throws -> AuthenticatedInputSender {
        var sender = try AuthenticatedInputSender(deviceKey: key, localControlAllowed: true,
            acceptsJpeg: acceptsJpeg, acceptsClipboard: acceptsClipboard,
            capacity: capacity, nonce: nonce, now: 0)
        let hello = try XCTUnwrap(sender.poll(now: 0))
        XCTAssertEqual(hello.sequence, 1)
        var capabilities: Capabilities = [.input]
        if acceptsJpeg { capabilities.insert(.jpeg) }
        if acceptsClipboard { capabilities.insert(.clipboardText) }
        XCTAssertEqual(try HelloPayload.decode(hello.payload).capabilities, capabilities)
        try sender.receive(Frame(type: .hello, sequence: 1,
            payload: HelloPayload(role: .agent, capabilities: capabilities, nonce: agentNonce).encode()), now: 0.001)
        let challenge = AuthChallengePayload(challenge: Data(repeating: 5, count: 32),
                                             agentIdentifier: Data(repeating: 6, count: 16))
        try sender.receive(Frame(type: .authChallenge, sequence: 2, payload: challenge.encode()), now: 0.002)
        // Peer responses may precede the local write-completion callback.
        XCTAssertNil(try sender.poll(now: 0.002))
        try sender.didWrite(sequence: 1, now: 0.003)
        let response = try XCTUnwrap(sender.poll(now: 0.02))
        XCTAssertEqual(response.sequence, 2)
        XCTAssertEqual(response.payload, try Authentication.response(deviceKey: key, controllerNonce: nonce,
            agentNonce: agentNonce, challenge: challenge.challenge, agentIdentifier: challenge.agentIdentifier))
        try sender.didWrite(sequence: 2, now: 0.021)
        return sender
    }

    private func authenticated(capacity: Int = 64, acceptsJpeg: Bool = false,
                               acceptsClipboard: Bool = false) throws -> AuthenticatedInputSender {
        var sender = try challenged(capacity: capacity, acceptsJpeg: acceptsJpeg,
                                    acceptsClipboard: acceptsClipboard)
        try sender.receive(Frame(type: .authResult, sequence: 3,
            payload: AuthResultPayload(status: .success, retryDelayMilliseconds: 0).encode()), now: 0.03)
        XCTAssertEqual(sender.state, .active)
        return sender
    }

    func testDuplexNegotiatesJpegAndPreservesIndependentSequences() throws {
        var sender = try authenticated(acceptsJpeg: true)
        let screen = ScreenInfoPayload(width: 1280, height: 720, dpiX100: 9600, dpiY100: 9600)
        XCTAssertNil(try sender.receive(Frame(type: .screenInfo, sequence: 4,
            payload: screen.encode()), now: 0.04))
        let jpeg = Data([0xff, 0xd8, 0xff, 0xd9])
        XCTAssertEqual(try sender.receive(Frame(type: .videoFrameJPEG, sequence: 5,
            payload: jpeg), now: 0.05), .jpeg(.init(screen: screen, jpeg: jpeg)))
        try sender.enqueue([down], now: 1)
        let input = try XCTUnwrap(sender.poll(now: 1))
        XCTAssertEqual(input.type, .mouseButton)
        XCTAssertEqual(input.sequence, 3)
    }

    func testClipboardRequiresNegotiationRequestAndBoundedUtf8Response() throws {
        var sender = try authenticated(acceptsClipboard: true)
        let text = ClipboardTextPayload(status: .success, text: "跨设备 clipboard")
        XCTAssertThrowsError(try sender.receive(Frame(type: .clipboardText, sequence: 4,
            payload: text.encode()), now: 0.04))

        sender = try authenticated(acceptsClipboard: true)
        try sender.enqueue([.clipboardRequest], now: 1)
        let pending = sender.pendingCount
        try sender.enqueue([.clipboardRequest], now: 1.001)
        XCTAssertEqual(sender.pendingCount, pending)
        XCTAssertEqual(sender.state, .active)

        sender = try authenticated(acceptsClipboard: true)
        try sender.enqueue([.clipboardRequest], now: 1)
        let request = try XCTUnwrap(sender.poll(now: 1))
        XCTAssertEqual(request.type, .clipboardRequest)
        try sender.didWrite(sequence: request.sequence, now: 1.001)
        XCTAssertEqual(try sender.receive(Frame(type: .clipboardText, sequence: 4,
            payload: text.encode()), now: 1.01), .clipboard(text))
    }

    func testClipboardSetRequiresMatchingResponseAndDropsFastRepeat() throws {
        let outgoing = ClipboardTextPayload(status: .success, text: "Mac → Windows")
        let result = ClipboardTextPayload(status: .success, text: "")
        var sender = try authenticated(acceptsClipboard: true)
        try sender.enqueue([.clipboardSet(outgoing)], now: 1)
        let pending = sender.pendingCount
        try sender.enqueue([.clipboardSet(outgoing)], now: 1.001)
        XCTAssertEqual(sender.pendingCount, pending)
        XCTAssertEqual(sender.state, .active)

        let request = try XCTUnwrap(sender.poll(now: 1.01))
        XCTAssertEqual(request.type, .clipboardSetText)
        XCTAssertEqual(try ClipboardTextPayload.decode(request.payload), outgoing)
        try sender.didWrite(sequence: request.sequence, now: 1.011)
        XCTAssertEqual(try sender.receive(Frame(type: .clipboardSetResult, sequence: 4,
            payload: result.encode()), now: 1.02), .clipboardSetResult(result))

        sender = try authenticated(acceptsClipboard: true)
        XCTAssertThrowsError(try sender.receive(Frame(type: .clipboardSetResult, sequence: 4,
            payload: result.encode()), now: 0.04))
        sender = try authenticated(acceptsClipboard: true)
        try sender.enqueue([.clipboardSet(outgoing)], now: 1)
        let written = try XCTUnwrap(sender.poll(now: 1))
        try sender.didWrite(sequence: written.sequence, now: 1.001)
        let invalid = ClipboardTextPayload(status: .success, text: "must be empty")
        XCTAssertThrowsError(try sender.receive(Frame(type: .clipboardSetResult, sequence: 4,
            payload: invalid.encode()), now: 1.01))
    }

    func testDuplexRejectsVideoBeforeMetadataAndMissingJpegCapability() throws {
        var sender = try authenticated(acceptsJpeg: true)
        XCTAssertThrowsError(try sender.receive(Frame(type: .videoFrameJPEG, sequence: 4,
            payload: Data([0xff, 0xd8, 0xff, 0xd9])), now: 0.04))
        XCTAssertEqual(sender.state, .failed)

        sender = try AuthenticatedInputSender(deviceKey: key, localControlAllowed: true,
            acceptsJpeg: true, nonce: nonce, now: 0)
        _ = try sender.poll(now: 0)
        XCTAssertThrowsError(try sender.receive(Frame(type: .hello, sequence: 1,
            payload: HelloPayload(role: .agent, capabilities: [.input], nonce: agentNonce).encode()), now: 0.01))
        XCTAssertEqual(sender.state, .failed)
    }

    func testDefaultPermissionAndPreauthInputRejected() throws {
        XCTAssertThrowsError(try AuthenticatedInputSender(deviceKey: key, now: 0))
        var sender = try challenged()
        XCTAssertThrowsError(try sender.enqueue([down], now: 0.1))
        XCTAssertEqual(sender.state, .failed)
        XCTAssertEqual(sender.pendingCount, 0)
    }

    func testMissingCapabilityAndRejectedAuthenticationClose() throws {
        var sender = try AuthenticatedInputSender(deviceKey: key, localControlAllowed: true, now: 0)
        _ = try sender.poll(now: 0)
        XCTAssertThrowsError(try sender.receive(Frame(type: .hello, sequence: 1,
            payload: HelloPayload(role: .agent, capabilities: [.jpeg], nonce: agentNonce).encode()), now: 0.01))
        XCTAssertEqual(sender.state, .failed)
        sender = try challenged()
        XCTAssertThrowsError(try sender.receive(Frame(type: .authResult, sequence: 3,
            payload: AuthResultPayload(status: .rejected, retryDelayMilliseconds: 1000).encode()), now: 0.03)) {
            XCTAssertEqual($0 as? InputSendError, .authenticationRejected)
        }
    }

    func testSingleWriterPacingAndContinuousSequence() throws {
        var sender = try authenticated()
        try sender.enqueue([down, up], now: 1)
        let first = try XCTUnwrap(sender.poll(now: 1))
        XCTAssertEqual(first.sequence, 3)
        XCTAssertNil(try sender.poll(now: 1.02))
        try sender.didWrite(sequence: 3, now: 1.02)
        let second = try XCTUnwrap(sender.poll(now: 1.02))
        XCTAssertEqual(second.sequence, 4)
        try sender.didWrite(sequence: 4, now: 1.021)
        try sender.enqueue([down], now: 1.022)
        XCTAssertNil(try sender.poll(now: 1.022))
        XCTAssertEqual(try sender.poll(now: 1.04)?.sequence, 5)
    }

    func testFinishDrainsReleaseBeforeDisconnect() throws {
        var sender = try authenticated()
        try sender.enqueue([down], now: 1)
        try sender.finish(releases: [up], now: 1)
        var types: [MessageType] = []
        var payloads: [Data] = []
        for index in 0..<3 {
            let time = 1 + Double(index) * 0.02
            let frame = try XCTUnwrap(sender.poll(now: time))
            types.append(frame.type)
            payloads.append(frame.payload)
            try sender.didWrite(sequence: frame.sequence, now: time)
        }
        XCTAssertEqual(types, [.mouseButton, .mouseButton, .disconnect])
        XCTAssertEqual(payloads, [down.payload, up.payload, Data([0, 0])])
        XCTAssertEqual(sender.state, .finished)
        XCTAssertNil(try sender.poll(now: 2))
    }

    func testFinishRejectsNewPressOrSubsequentInput() throws {
        var sender = try authenticated()
        XCTAssertThrowsError(try sender.finish(releases: [down], now: 1))
        XCTAssertEqual(sender.state, .failed)
        sender = try authenticated()
        try sender.finish(releases: [], now: 1)
        XCTAssertThrowsError(try sender.enqueue([down], now: 1))
        XCTAssertEqual(sender.state, .failed)
    }

    func testOverflowOnReleaseOrDisconnectAborts() throws {
        var sender = try authenticated(capacity: 2)
        try sender.enqueue([down], now: 1)
        XCTAssertThrowsError(try sender.finish(releases: [up], now: 1)) {
            XCTAssertEqual($0 as? InputSendError, .queueOverflow)
        }
        XCTAssertEqual(sender.state, .failed)
        XCTAssertEqual(sender.pendingCount, 0)
        XCTAssertNil(try sender.poll(now: 1))
    }

    func testHeartbeatContinuesIdleConnectionAndChecksToken() throws {
        var sender = try authenticated()
        let ping = try XCTUnwrap(sender.poll(now: 6))
        XCTAssertEqual(ping.type, .ping)
        XCTAssertEqual(ping.payload.count, 8)
        // PONG may arrive before contentProcessed too.
        try sender.receive(Frame(type: .pong, sequence: 4, payload: ping.payload), now: 6.01)
        try sender.didWrite(sequence: ping.sequence, now: 6.02)
        XCTAssertNil(try sender.poll(now: 10))
        let next = try XCTUnwrap(sender.poll(now: 12))
        try sender.didWrite(sequence: next.sequence, now: 12.01)
        XCTAssertThrowsError(try sender.receive(Frame(type: .pong, sequence: 5,
            payload: Data(count: 7)), now: 12.02))
        XCTAssertEqual(sender.state, .failed)
    }

    func testAuthenticationWriteHeartbeatAndDrainDeadlines() throws {
        var sender = try AuthenticatedInputSender(deviceKey: key, localControlAllowed: true, now: 0)
        XCTAssertThrowsError(try sender.poll(now: 15)) {
            XCTAssertEqual($0 as? InputSendError, .authenticationTimeout)
        }
        sender = try authenticated()
        try sender.enqueue([down], now: 1)
        _ = try sender.poll(now: 1)
        XCTAssertThrowsError(try sender.poll(now: 6)) {
            XCTAssertEqual($0 as? InputSendError, .writeTimeout)
        }
        sender = try authenticated()
        let ping = try XCTUnwrap(sender.poll(now: 6))
        try sender.didWrite(sequence: ping.sequence, now: 6.01)
        XCTAssertThrowsError(try sender.poll(now: 11)) {
            XCTAssertEqual($0 as? InputSendError, .heartbeatTimeout)
        }
        sender = try authenticated()
        try sender.finish(releases: [], now: 1)
        XCTAssertThrowsError(try sender.poll(now: 6)) {
            XCTAssertEqual($0 as? InputSendError, .closingTimeout)
        }
    }

    func testBadSequenceWrongCompletionAndClockAbort() throws {
        var sender = try authenticated()
        XCTAssertThrowsError(try sender.receive(Frame(type: .pong, sequence: 99, payload: Data(count: 8)), now: 1))
        XCTAssertEqual(sender.state, .failed)
        sender = try authenticated()
        try sender.enqueue([down], now: 1)
        _ = try sender.poll(now: 1)
        XCTAssertThrowsError(try sender.didWrite(sequence: 99, now: 1))
        sender = try authenticated()
        XCTAssertThrowsError(try sender.poll(now: .nan))
        sender = try authenticated()
        XCTAssertThrowsError(try sender.poll(now: 0))
    }

    func testAbortDropsPendingAndIgnoresLateWriteCompletion() throws {
        var sender = try authenticated()
        try sender.enqueue([down, up], now: 1)
        let frame = try XCTUnwrap(sender.poll(now: 1))
        sender.abort()
        sender.abort()
        try sender.didWrite(sequence: frame.sequence, now: 2)
        XCTAssertEqual(sender.state, .failed)
        XCTAssertEqual(sender.pendingCount, 0)
        XCTAssertFalse(sender.hasWriteInFlight)
        XCTAssertNil(try sender.poll(now: 2))
    }

    func testRemoteErrorDiscardsPendingInput() throws {
        var sender = try authenticated()
        try sender.enqueue([down], now: 1)
        try sender.receive(Frame(type: .error, sequence: 4, payload: Data([0, 1])), now: 1)
        XCTAssertEqual(sender.state, .failed)
        XCTAssertEqual(sender.pendingCount, 0)
    }

    func testSharedQueueProducesContinuousWireFramesAfterAuthentication() throws {
        let fixture = try InputQueueFixture.load()
        var sender = try authenticated(capacity: fixture.capacity)
        try sender.enqueue(fixture.inputs.map { try $0.captured() }, now: 1)
        var decoder = ProbeFrameDecoder()
        for (index, expected) in fixture.expected.enumerated() {
            let time = 1 + Double(index) * 0.02
            let frame = try XCTUnwrap(sender.poll(now: time))
            XCTAssertEqual(frame.sequence, UInt32(index + 3))
            XCTAssertEqual(String(describing: frame.type), expected.type)
            XCTAssertEqual(frame.payload, expected.payload)
            let bytes = try FrameCodec.encode(frame)
            var decoded: [Frame] = []
            for byte in bytes { decoded += try decoder.append(Data([byte])) }
            XCTAssertEqual(decoded, [frame])
            try sender.didWrite(sequence: frame.sequence, now: time)
        }
        XCTAssertEqual(sender.pendingCount, 0)
    }

    func testAuthResultBeforeResponseWasSubmittedIsRejected() throws {
        var sender = try AuthenticatedInputSender(deviceKey: key, localControlAllowed: true, now: 0)
        _ = try sender.poll(now: 0)
        try sender.receive(Frame(type: .hello, sequence: 1,
            payload: HelloPayload(role: .agent, capabilities: [.input], nonce: agentNonce).encode()), now: 0)
        try sender.receive(Frame(type: .authChallenge, sequence: 2, payload: Data(count: 48)), now: 0)
        XCTAssertThrowsError(try sender.receive(Frame(type: .authResult, sequence: 3,
            payload: AuthResultPayload(status: .success, retryDelayMilliseconds: 0).encode()), now: 0))
        XCTAssertEqual(sender.state, .failed)
    }
}
