import Foundation

/// Transport-independent input sender for the mock profile. A future TLS adapter
/// must call on ONE serial executor, poll at least every 20 ms, send only the frame
/// returned by poll, and call didWrite after contentProcessed. EOF/error/cancel:
/// abort AND cancel TLS; server-side cleanup remains essential after any failure.
/// Never dispatch one unbounded async block per captured event ahead of this queue.
struct AuthenticatedInputSender {
    struct ReceivedJpegFrame: Equatable {
        let screen: ScreenInfoPayload
        let jpeg: Data
    }
    enum ReceivedMessage: Equatable {
        case jpeg(ReceivedJpegFrame)
        case clipboard(ClipboardTextPayload)
        case clipboardSetResult(ClipboardTextPayload)
    }
    private enum ClipboardOperation: Equatable { case read, write }
    enum State: Equatable { case authenticating, active, draining, finished, failed }
    private(set) var state: State = .authenticating
    private var gate = SessionGate(localRole: .controller)
    private var queue: InputSendQueue
    private let key: Data
    private let nonce: Data
    private let acceptsJpeg: Bool
    private let acceptsClipboard: Bool
    private var agentNonce: Data?
    private var screen: ScreenInfoPayload?
    private var incomingSequence: UInt32 = 1
    private var outgoingSequence: UInt32 = 1
    private var helloStarted = false
    private var responseStarted = false
    private var inFlight: Frame?
    private var writeDeadline: Double?
    private let authDeadline: Double
    private var closeDeadline: Double?
    private var heartbeatDeadline: Double?
    private var heartbeat: Data?
    private var nextHeartbeat: Double
    private var nextWrite: Double
    private var lastTime: Double
    private var clipboardOperationPending: ClipboardOperation?
    private var clipboardDeadline: Double?
    var pendingCount: Int { queue.count }
    var hasWriteInFlight: Bool { inFlight != nil }

    init(deviceKey: Data, localControlAllowed: Bool = false, acceptsJpeg: Bool = false,
         acceptsClipboard: Bool = false, capacity: Int = 64,
         nonce: Data? = nil, now: Double) throws {
        guard localControlAllowed, deviceKey.count == 32, now.isFinite, now >= 0 else {
            throw InputSendError.invalidConfiguration
        }
        self.key = deviceKey
        self.acceptsJpeg = acceptsJpeg
        self.acceptsClipboard = acceptsClipboard
        self.nonce = try nonce ?? SecureRandom.bytes(count: 32)
        guard self.nonce.count == 32 else { throw InputSendError.invalidConfiguration }
        queue = try InputSendQueue(capacity: capacity)
        authDeadline = now + 15
        nextHeartbeat = now + 5
        nextWrite = now
        lastTime = now
        var capabilities: Capabilities = [.input]
        if acceptsJpeg { capabilities.insert(.jpeg) }
        if acceptsClipboard { capabilities.insert(.clipboardText) }
        try queue.append(QueuedInputMessage(type: .hello,
            payload: HelloPayload(role: .controller, capabilities: capabilities, nonce: self.nonce).encode()))
    }

    mutating func enqueue(_ inputs: [CapturedInput], now: Double) throws {
        do {
            try checkTime(now)
            guard state == .active else { throw InputSendError.invalidState }
            let clipboardOperations = inputs.compactMap { input -> ClipboardOperation? in
                switch input.messageType {
                case .clipboardRequest: return .read
                case .clipboardSetText: return .write
                default: return nil
                }
            }
            guard clipboardOperations.count <= 1,
                  clipboardOperations.isEmpty || acceptsClipboard else {
                throw InputSendError.invalidState
            }
            // A fast repeated Command+C/Command+V must not tear down the whole
            // control session. Keep at most one clipboard operation in flight.
            if !clipboardOperations.isEmpty, clipboardOperationPending != nil { return }
            try queue.append(inputs)
            if let operation = clipboardOperations.first { clipboardOperationPending = operation }
        } catch { abort(); throw error }
    }

    /// Releases must come from capture.stop(). Preserve prior accepted commands,
    /// then releases, then DISCONNECT. A full queue fails; the adapter must cancel TLS.
    mutating func finish(releases: [CapturedInput], now: Double) throws {
        do {
            try checkTime(now)
            guard state == .active else { throw InputSendError.invalidState }
            guard releases.allSatisfy({ input in
                switch input {
                case .key(let value): return value.action == .up
                case .button(let value): return value.action == .up
                default: return false
                }
            }) else { throw ProtocolError.invalidPayload }
            try queue.append(releases)
            try queue.append(QueuedInputMessage(type: .disconnect, payload: Data([0, 0])))
            state = .draining
            closeDeadline = now + 5
        } catch { abort(); throw error }
    }

    /// One frame at a time, paced to at most 100 writes/s without catch-up bursts.
    /// Call even with no queued data so all deadlines and idle heartbeats progress.
    mutating func poll(now: Double) throws -> Frame? {
        if state == .finished || state == .failed { return nil }
        do {
            try checkTime(now)
            if state == .active, heartbeat == nil, now >= nextHeartbeat {
                let token = try SecureRandom.bytes(count: 8)
                try queue.append(QueuedInputMessage(type: .ping, payload: token))
                heartbeat = token
            }
            guard inFlight == nil, now >= nextWrite, let message = queue.take() else { return nil }
            let frame = Frame(type: message.type, sequence: outgoingSequence, payload: message.payload)
            outgoingSequence = outgoingSequence == .max ? 1 : outgoingSequence + 1
            inFlight = frame
            writeDeadline = now + 5
            nextWrite = now + 0.01
            if frame.type == .hello { helloStarted = true }
            if frame.type == .authResponse { responseStarted = true }
            if frame.type == .ping { heartbeatDeadline = now + 5 }
            return frame
        } catch { abort(); throw error }
    }

    mutating func didWrite(sequence: UInt32, now: Double) throws {
        // Cancellation can race a late completion. It must not restart the sender.
        if state == .finished || state == .failed { return }
        do {
            try checkTime(now)
            guard let frame = inFlight, frame.sequence == sequence else { throw InputSendError.invalidState }
            inFlight = nil
            writeDeadline = nil
            if frame.type == .disconnect {
                state = .finished
                queue.stop()
            }
            if frame.type == .clipboardRequest || frame.type == .clipboardSetText {
                clipboardDeadline = now + 5
            }
        } catch { abort(); throw error }
    }

    @discardableResult
    mutating func receive(_ frame: Frame, now: Double) throws -> ReceivedMessage? {
        do {
            try checkTime(now)
            let payloadLimit: Int
            if acceptsJpeg && frame.type == .videoFrameJPEG {
                payloadLimit = ProtocolConstants.maximumPayloadLength
            } else if acceptsClipboard && (frame.type == .clipboardText || frame.type == .clipboardSetResult) {
                payloadLimit = ClipboardTextPayload.maximumTextBytes + 1
            } else {
                payloadLimit = 64
            }
            guard state != .finished, state != .failed, helloStarted,
                  frame.sequence == incomingSequence, frame.flags == 0,
                  frame.payload.count <= payloadLimit else {
                throw InputSendError.invalidState
            }
            incomingSequence = incomingSequence == .max ? 1 : incomingSequence + 1
            try gate.receive(frame)
            switch frame.type {
            case .hello:
                let hello = try HelloPayload.decode(frame.payload)
                guard hello.capabilities.contains(.input),
                      !acceptsJpeg || hello.capabilities.contains(.jpeg),
                      !acceptsClipboard || hello.capabilities.contains(.clipboardText) else {
                    throw ProtocolError.invalidPayload
                }
                agentNonce = hello.nonce
            case .authChallenge:
                guard let agentNonce else { throw InputSendError.invalidState }
                let challenge = try AuthChallengePayload.decode(frame.payload)
                let response = try Authentication.response(deviceKey: key, controllerNonce: nonce,
                    agentNonce: agentNonce, challenge: challenge.challenge, agentIdentifier: challenge.agentIdentifier)
                try queue.append(QueuedInputMessage(type: .authResponse, payload: response))
            case .authResult:
                guard responseStarted else { throw InputSendError.invalidState }
                guard gate.phase == .authenticated else { throw InputSendError.authenticationRejected }
                state = .active
                nextHeartbeat = now + 5
            case .screenInfo:
                guard acceptsJpeg else { throw ProtocolError.invalidState }
                screen = try ScreenInfoPayload.decode(frame.payload)
            case .videoFrameJPEG:
                guard acceptsJpeg, let screen else { throw ProtocolError.invalidState }
                return .jpeg(ReceivedJpegFrame(screen: screen, jpeg: frame.payload))
            case .clipboardText:
                guard acceptsClipboard, clipboardOperationPending == .read, clipboardDeadline != nil else {
                    throw ProtocolError.invalidState
                }
                clipboardOperationPending = nil
                clipboardDeadline = nil
                return .clipboard(try ClipboardTextPayload.decode(frame.payload))
            case .clipboardSetResult:
                guard acceptsClipboard, clipboardOperationPending == .write, clipboardDeadline != nil else {
                    throw ProtocolError.invalidState
                }
                let result = try ClipboardTextPayload.decode(frame.payload)
                guard result.text.isEmpty else { throw ProtocolError.invalidPayload }
                clipboardOperationPending = nil
                clipboardDeadline = nil
                return .clipboardSetResult(result)
            case .pong:
                guard let heartbeat, heartbeatDeadline != nil, frame.payload == heartbeat else {
                    throw ProtocolError.invalidPayload
                }
                self.heartbeat = nil
                heartbeatDeadline = nil
                nextHeartbeat = now + 5
            case .disconnect, .error:
                // Remote termination is not confirmation that our pending releases arrived.
                abort()
            default:
                throw ProtocolError.invalidState
            }
            return nil
        } catch { abort(); throw error }
    }

    mutating func abort() {
        state = .failed
        queue.stop()
        inFlight = nil
        writeDeadline = nil
        heartbeat = nil
        heartbeatDeadline = nil
        closeDeadline = nil
        clipboardOperationPending = nil
        clipboardDeadline = nil
    }

    private mutating func checkTime(_ now: Double) throws {
        guard now.isFinite, now >= lastTime else { throw InputSendError.invalidClock }
        lastTime = now
        guard state != .failed, state != .finished else { throw InputSendError.stopped }
        if state == .authenticating && now >= authDeadline { throw InputSendError.authenticationTimeout }
        if let closeDeadline, now >= closeDeadline { throw InputSendError.closingTimeout }
        if let writeDeadline, now >= writeDeadline { throw InputSendError.writeTimeout }
        if let heartbeatDeadline, now >= heartbeatDeadline { throw InputSendError.heartbeatTimeout }
        if let clipboardDeadline, now >= clipboardDeadline { throw InputSendError.writeTimeout }
    }
}
