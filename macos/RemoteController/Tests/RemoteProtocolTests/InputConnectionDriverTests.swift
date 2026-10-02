import Foundation
import XCTest
@testable import RemoteProtocol

final class InputConnectionDriverTests: XCTestCase {
    private let key = Data(repeating: 1, count: 32)
    private let down = CapturedInput.button(MouseButtonPayload(button: .left, action: .down))
    private let up = CapturedInput.button(MouseButtonPayload(button: .left, action: .up))

    private final class Clock {
        private let lock = NSLock()
        private var value = 0.0
        var now: Double {
            get { lock.lock(); defer { lock.unlock() }; return value }
            set { lock.lock(); defer { lock.unlock() }; value = newValue }
        }
    }

    private final class FakeTransport: InputTransport {
        var owner: DispatchQueue!
        var stateCallback: ((InputTransportState) -> Void)?
        var receiveCallback: ((Data?, Bool, Bool) -> Void)?
        var writes: [Data] = []
        var writeCallbacks: [(Bool) -> Void] = []
        var cancels = 0
        func start(queue: DispatchQueue, state: @escaping (InputTransportState) -> Void) {
            owner = queue
            stateCallback = state
        }
        func send(_ bytes: Data, completion: @escaping (Bool) -> Void) {
            writes.append(bytes)
            writeCallbacks.append(completion)
        }
        func receive(completion: @escaping (Data?, Bool, Bool) -> Void) {
            XCTAssertNil(receiveCallback, "Only one outstanding receive")
            receiveCallback = completion
        }
        func cancel() { cancels += 1 }
        func ready() { owner.sync { stateCallback?(.ready) } }
        func ack(_ index: Int, success: Bool = true) { owner.sync { writeCallbacks[index](success) } }
        func deliver(_ bytes: Data?, eof: Bool = false) {
            owner.sync {
                let callback = receiveCallback
                receiveCallback = nil
                XCTAssertNotNil(callback)
                callback?(bytes, eof, true)
            }
            owner.sync {} // Flush the single deferred receive registration.
        }
        func frames() throws -> [Frame] {
            try owner.sync { try writes.flatMap { data -> [Frame] in
                var decoder = FrameDecoder()
                return try decoder.append(data)
            } }
        }
    }

    private func make(clock: Clock, transport: FakeTransport, authenticated: @escaping () -> Void = {},
                      jpeg: ((AuthenticatedInputSender.ReceivedJpegFrame) -> Void)? = nil,
                      clipboard: ((ClipboardTextPayload) -> Void)? = nil,
                      clipboardSet: ((ClipboardTextPayload) -> Void)? = nil,
                      done: @escaping (Result<Void, InputSimulationError>) -> Void) throws -> InputConnectionDriver {
        try InputConnectionDriver(transport: transport, deviceKey: key, localControlAllowed: true,
            automaticTimer: false, clock: { clock.now }, onJpegFrame: jpeg,
            onClipboardText: clipboard,
            onClipboardSetResult: clipboardSet,
            onAuthenticated: authenticated, completion: done)
    }

    private func authenticate(_ driver: InputConnectionDriver, _ transport: FakeTransport, _ clock: Clock) throws {
        driver.start()
        transport.ready()
        let controllerHello = try HelloPayload.decode(XCTUnwrap(transport.frames().first).payload)
        let nonce = Data(repeating: 3, count: 32)
        let challenge = AuthChallengePayload(challenge: Data(repeating: 5, count: 32),
                                             agentIdentifier: Data(repeating: 6, count: 16))
        let hello = Frame(type: .hello, sequence: 1,
            payload: try HelloPayload(role: .agent, capabilities: controllerHello.capabilities,
                nonce: nonce).encode())
        let request = Frame(type: .authChallenge, sequence: 2, payload: try challenge.encode())
        transport.deliver(try FrameCodec.encode(hello) + FrameCodec.encode(request))
        XCTAssertEqual(try transport.frames().count, 1)
        transport.ack(0)
        clock.now = 0.02
        driver.tick()
        let response = try XCTUnwrap(transport.frames().last)
        XCTAssertEqual(response.type, .authResponse)
        XCTAssertEqual(response.payload, try Authentication.response(deviceKey: key,
            controllerNonce: controllerHello.nonce, agentNonce: nonce,
            challenge: challenge.challenge, agentIdentifier: challenge.agentIdentifier))
        transport.ack(1)
        let result = Frame(type: .authResult, sequence: 3,
            payload: try AuthResultPayload(status: .success, retryDelayMilliseconds: 0).encode())
        transport.deliver(try FrameCodec.encode(result))
    }

    func testDuplexReceivesSplitJpegWhileInputWriterStaysContinuous() throws {
        let clock = Clock(), transport = FakeTransport()
        let ended = expectation(description: "cancel")
        var videos: [AuthenticatedInputSender.ReceivedJpegFrame] = []
        let driver = try make(clock: clock, transport: transport, jpeg: { videos.append($0) }) { result in
            if case .success = result { XCTFail("Expected cancellation") }
            ended.fulfill()
        }
        try authenticate(driver, transport, clock)
        let screen = ScreenInfoPayload(width: 1280, height: 720, dpiX100: 9600, dpiY100: 9600)
        let jpeg = Data([0xff, 0xd8]) + Data(repeating: 0x55, count: 5_000) + Data([0xff, 0xd9])
        let wire = try FrameCodec.encode(Frame(type: .screenInfo, sequence: 4, payload: screen.encode()))
            + FrameCodec.encode(Frame(type: .videoFrameJPEG, sequence: 5, payload: jpeg))
        for start in stride(from: 0, to: wire.count, by: 1_337) {
            transport.deliver(wire.subdata(in: start..<min(start + 1_337, wire.count)))
        }
        XCTAssertEqual(videos, [.init(screen: screen, jpeg: jpeg)])
        clock.now = 1
        XCTAssertTrue(driver.submit([down]))
        driver.tick()
        XCTAssertEqual(try transport.frames().last?.sequence, 3)
        XCTAssertEqual(try transport.frames().last?.type, .mouseButton)
        driver.cancel()
        wait(for: [ended], timeout: 2)
    }

    func testClipboardResponseOnlyAfterExplicitRequestReachesCallback() throws {
        let clock = Clock(), transport = FakeTransport()
        let received = expectation(description: "clipboard")
        let ended = expectation(description: "cancel")
        let expected = ClipboardTextPayload(status: .success, text: "跨设备 clipboard")
        let driver = try make(clock: clock, transport: transport, clipboard: { payload in
            XCTAssertEqual(payload, expected)
            received.fulfill()
        }) { _ in ended.fulfill() }
        try authenticate(driver, transport, clock)
        clock.now = 1
        XCTAssertTrue(driver.submit([.clipboardRequest]))
        driver.tick()
        XCTAssertEqual(try transport.frames().last?.type, .clipboardRequest)
        transport.ack(2)
        transport.deliver(try FrameCodec.encode(Frame(type: .clipboardText, sequence: 4,
            payload: expected.encode())))
        wait(for: [received], timeout: 2)
        driver.cancel()
        wait(for: [ended], timeout: 2)
    }

    func testClipboardSetResultReachesCallback() throws {
        let clock = Clock(), transport = FakeTransport()
        let received = expectation(description: "clipboard set")
        let ended = expectation(description: "cancel")
        let outgoing = ClipboardTextPayload(status: .success, text: "Mac → Windows")
        let result = ClipboardTextPayload(status: .success, text: "")
        let driver = try make(clock: clock, transport: transport, clipboardSet: { payload in
            XCTAssertEqual(payload, result)
            received.fulfill()
        }) { _ in ended.fulfill() }
        try authenticate(driver, transport, clock)
        clock.now = 1
        XCTAssertTrue(driver.submit([.clipboardSet(outgoing)]))
        driver.tick()
        XCTAssertEqual(try transport.frames().last?.type, .clipboardSetText)
        transport.ack(2)
        transport.deliver(try FrameCodec.encode(Frame(type: .clipboardSetResult, sequence: 4,
            payload: result.encode())))
        wait(for: [received], timeout: 2)
        driver.cancel()
        wait(for: [ended], timeout: 2)
    }

    func testDuplexRejectsAdapterCallbackAboveBound() throws {
        let clock = Clock(), transport = FakeTransport()
        let ended = expectation(description: "oversized callback")
        let driver = try make(clock: clock, transport: transport, jpeg: { _ in }) { result in
            if case .failure(let error) = result { XCTAssertEqual(error, .protocolFailure) }
            else { XCTFail("Expected protocol failure") }
            ended.fulfill()
        }
        driver.start()
        transport.ready()
        transport.deliver(Data(count: 65_537))
        XCTAssertTrue(driver.isEnded)
        wait(for: [ended], timeout: 2)
    }

    func testSlowWriteKeepsOneFlightAndCoalescesIngress() throws {
        let clock = Clock(), transport = FakeTransport()
        let ended = expectation(description: "cancel")
        let driver = try make(clock: clock, transport: transport) { result in
            if case .success = result { XCTFail("Expected cancellation") }
            ended.fulfill()
        }
        try authenticate(driver, transport, clock)
        clock.now = 1
        XCTAssertTrue(driver.submit([down]))
        driver.tick()
        for value in 0..<10000 {
            XCTAssertTrue(driver.submit([.move(MouseMovePayload(x: UInt16(value), y: 0))]))
        }
        XCTAssertEqual(driver.pendingCount, 1)
        driver.tick()
        XCTAssertEqual(try transport.frames().count, 3)
        transport.ack(2)
        clock.now = 1.02
        driver.tick()
        XCTAssertEqual(try transport.frames().last?.payload, MouseMovePayload(x: 9999, y: 0).encode())
        driver.cancel()
        wait(for: [ended], timeout: 2)
    }

    func testGracefulFinishSendsReleaseThenDisconnect() throws {
        let clock = Clock(), transport = FakeTransport()
        let ended = expectation(description: "finished")
        let driver = try make(clock: clock, transport: transport) { result in
            if case .failure = result { XCTFail("Expected graceful local completion") }
            ended.fulfill()
        }
        try authenticate(driver, transport, clock)
        XCTAssertTrue(driver.submit([down]))
        driver.finish(releases: [up])
        for index in 2...4 {
            clock.now = Double(index)
            driver.tick()
            transport.ack(index)
        }
        XCTAssertEqual(try transport.frames().suffix(3).map { $0.type }, [.mouseButton, .mouseButton, .disconnect])
        XCTAssertTrue(driver.isEnded)
        wait(for: [ended], timeout: 2)
        XCTAssertEqual(transport.cancels, 1)
    }

    func testCongestionCancelsTransportOnce() throws {
        let clock = Clock(), transport = FakeTransport()
        let ended = expectation(description: "overflow")
        ended.assertForOverFulfill = true
        let driver = try make(clock: clock, transport: transport) { result in
            if case .failure(let error) = result { XCTAssertEqual(error, .congested) }
            else { XCTFail("Expected congestion") }
            ended.fulfill()
        }
        try authenticate(driver, transport, clock)
        XCTAssertFalse(driver.submit(Array(repeating: down, count: 65)))
        driver.cancel()
        XCTAssertEqual(driver.pendingCount, 0)
        XCTAssertEqual(transport.cancels, 1)
        wait(for: [ended], timeout: 2)
    }

    func testWriteFailureAndLateCallbacksCannotRestart() throws {
        let clock = Clock(), transport = FakeTransport()
        let ended = expectation(description: "write failure")
        ended.assertForOverFulfill = true
        let driver = try make(clock: clock, transport: transport) { _ in ended.fulfill() }
        try authenticate(driver, transport, clock)
        clock.now = 1
        XCTAssertTrue(driver.submit([down]))
        driver.tick()
        transport.ack(2, success: false)
        transport.ack(2)
        transport.ready()
        driver.tick()
        XCTAssertTrue(driver.isEnded)
        XCTAssertEqual(transport.cancels, 1)
        wait(for: [ended], timeout: 2)
    }

    func testConnectAndWriteDeadlinesCancelActualAdapter() throws {
        for ready in [false, true] {
            let clock = Clock(), transport = FakeTransport()
            let ended = expectation(description: "timeout")
            let driver = try make(clock: clock, transport: transport) { result in
                if case .failure(let error) = result { XCTAssertEqual(error, .timedOut) }
                else { XCTFail("Expected timeout") }
                ended.fulfill()
            }
            driver.start()
            if ready { transport.ready() }
            clock.now = ready ? 5 : 15
            driver.tick()
            XCTAssertTrue(driver.isEnded)
            XCTAssertEqual(transport.cancels, 1)
            wait(for: [ended], timeout: 2)
        }
    }

    func testPartialHeaderEOFEndsSession() throws {
        let clock = Clock(), transport = FakeTransport()
        let ended = expectation(description: "EOF")
        let driver = try make(clock: clock, transport: transport) { _ in ended.fulfill() }
        driver.start()
        transport.ready()
        transport.deliver(Data([0x50]), eof: true)
        XCTAssertTrue(driver.isEnded)
        XCTAssertEqual(transport.cancels, 1)
        wait(for: [ended], timeout: 2)
    }

    func testQueuedAuthenticationNotificationSuppressedAfterCancel() throws {
        let clock = Clock(), transport = FakeTransport()
        let ended = expectation(description: "cancel")
        let driver = try make(clock: clock, transport: transport,
            authenticated: { XCTFail("Cancelled authentication notification must not activate UI") }) { _ in ended.fulfill() }
        try authenticate(driver, transport, clock)
        driver.cancel() // Main queue notification has not yet run.
        wait(for: [ended], timeout: 2)
    }

    func testSubmitBeforeAuthenticationFailsClosed() throws {
        let clock = Clock(), transport = FakeTransport()
        let ended = expectation(description: "preauth")
        let driver = try make(clock: clock, transport: transport) { _ in ended.fulfill() }
        driver.start()
        XCTAssertFalse(driver.submit([down]))
        XCTAssertTrue(driver.isEnded)
        XCTAssertEqual(transport.cancels, 1)
        wait(for: [ended], timeout: 2)
    }

    func testBatchAdmissionBoundAndRepeatedCancel() throws {
        let clock = Clock(), transport = FakeTransport()
        let ended = expectation(description: "oversize")
        let driver = try make(clock: clock, transport: transport) { _ in ended.fulfill() }
        try authenticate(driver, transport, clock)
        XCTAssertFalse(driver.submit(Array(repeating: down, count: 257)))
        driver.cancel()
        driver.cancel()
        XCTAssertEqual(transport.cancels, 1)
        wait(for: [ended], timeout: 2)
    }

    func testLoopbackClientRequiresExplicitConsentAndValidPort() throws {
        let fingerprint = try CertificateFingerprint(bytes: Data(count: 32))
        XCTAssertThrowsError(try TLSInputSimulationClient(port: 0, expectedFingerprint: fingerprint,
            deviceKey: key, localControlAllowed: true, onAuthenticated: {}, completion: { _ in }))
        XCTAssertThrowsError(try TLSInputSimulationClient(port: 9, expectedFingerprint: fingerprint,
            deviceKey: key, onAuthenticated: {}, completion: { _ in }))
        XCTAssertThrowsError(try TLSInputSimulationClient(port: 9, expectedFingerprint: fingerprint,
            deviceKey: key, localControlAllowed: true, connectionTimeout: 0,
            onAuthenticated: {}, completion: { _ in }))
    }

    func testCrossDeviceClientOnlyAcceptsLiteralTailscaleIPv4() throws {
        XCTAssertTrue(InputSimulationEndpointPolicy.isTailscaleIPv4("100.64.0.1"))
        XCTAssertTrue(InputSimulationEndpointPolicy.isTailscaleIPv4("100.127.255.255"))
        for host in ["100.63.255.255", "100.128.0.1", "127.0.0.1", "localhost",
                     "100.064.0.1", "100.64.0.1.example", "fd7a:115c:a1e0::1"] {
            XCTAssertFalse(InputSimulationEndpointPolicy.isTailscaleIPv4(host), host)
            XCTAssertThrowsError(try TLSInputSimulationClient(tailscaleHost: host, port: 47475,
                expectedFingerprint: CertificateFingerprint(bytes: Data(count: 32)), deviceKey: key,
                localControlAllowed: true, onAuthenticated: {}, completion: { _ in }))
        }
    }

    func testAutomaticTimerCancelsStalledConnection() throws {
        let clock = Clock(), transport = FakeTransport()
        let ended = expectation(description: "automatic timeout")
        let driver = try InputConnectionDriver(transport: transport, deviceKey: key,
            localControlAllowed: true, clock: { clock.now }, onAuthenticated: {}) { result in
                if case .failure(let error) = result { XCTAssertEqual(error, .timedOut) }
                else { XCTFail("Expected timed out connection") }
                ended.fulfill()
            }
        clock.now = 15
        driver.start()
        wait(for: [ended], timeout: 2)
        XCTAssertTrue(driver.isEnded)
        XCTAssertEqual(transport.cancels, 1)
    }

    func testReleasingClientCancelsWithoutStartingNetwork() throws {
        let ended = expectation(description: "client released")
        var client: TLSInputSimulationClient? = try TLSInputSimulationClient(port: 9,
            expectedFingerprint: CertificateFingerprint(bytes: Data(count: 32)), deviceKey: key,
            localControlAllowed: true, onAuthenticated: {}) { result in
                if case .failure(let error) = result { XCTAssertEqual(error, .cancelled) }
                else { XCTFail("Expected cancellation") }
                ended.fulfill()
            }
        XCTAssertNotNil(client)
        client = nil
        wait(for: [ended], timeout: 2)
    }
}
