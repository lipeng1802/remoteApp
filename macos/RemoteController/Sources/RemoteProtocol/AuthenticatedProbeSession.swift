import Foundation
import Security

public enum SecureRandom {
    public static func bytes(count: Int) throws -> Data {
        var data = Data(count: count)
        let status = data.withUnsafeMutableBytes { buffer in
            SecRandomCopyBytes(kSecRandomDefault, count, buffer.baseAddress!)
        }
        guard status == errSecSuccess else { throw ProtocolError.invalidPayload }
        return data
    }
}

// Probe-only framing limit. Inspect the complete header before buffering a body.
struct ProbeFrameDecoder {
    private var buffer = Data()
    private var targetLength = ProtocolConstants.headerLength

    mutating func append(_ data: Data) throws -> [Frame] {
        var frames: [Frame] = []
        for byte in data {
            buffer.append(byte)
            if buffer.count == ProtocolConstants.headerLength {
                var validation = FrameDecoder()
                _ = try validation.append(buffer)
                let length = buffer[12..<16].reduce(0) { ($0 << 8) | Int($1) }
                guard length <= 64 else { throw ProtocolError.messageTooLarge(length) }
                targetLength = ProtocolConstants.headerLength + length
            }
            if buffer.count == targetLength {
                var decoder = FrameDecoder()
                frames += try decoder.append(buffer)
                try decoder.finish()
                buffer.removeAll(keepingCapacity: true)
                targetLength = ProtocolConstants.headerLength
            }
        }
        return frames
    }
}

struct AuthenticatedProbeSession {
    private var gate = SessionGate(localRole: .controller)
    private var incomingSequence: UInt32 = 1
    private var outgoingSequence: UInt32 = 1
    private let deviceKey: Data
    private let nonce: Data
    private let ping: Data
    private var agentNonce: Data?
    private var started = false
    private(set) var isComplete = false

    init(deviceKey: Data, nonce: Data? = nil, ping: Data? = nil) throws {
        guard deviceKey.count == 32 else { throw ProtocolError.invalidPayload }
        self.deviceKey = deviceKey
        self.nonce = try nonce ?? SecureRandom.bytes(count: 32)
        self.ping = try ping ?? SecureRandom.bytes(count: 8)
        guard self.nonce.count == 32, self.ping.count == 8 else { throw ProtocolError.invalidPayload }
    }

    mutating func start() throws -> Frame {
        guard !started else { throw ProtocolError.invalidState }
        started = true
        return outgoing(.hello, try HelloPayload(role: .controller, capabilities: [], nonce: nonce).encode())
    }

    mutating func receive(_ frame: Frame) throws -> [Frame] {
        guard started, !isComplete, frame.sequence == incomingSequence else { throw ProtocolError.invalidState }
        incomingSequence = incomingSequence == .max ? 1 : incomingSequence + 1
        try gate.receive(frame)
        switch frame.type {
        case .hello:
            agentNonce = try HelloPayload.decode(frame.payload).nonce
            return []
        case .authChallenge:
            guard let agentNonce else { throw ProtocolError.invalidState }
            let challenge = try AuthChallengePayload.decode(frame.payload)
            let response = try Authentication.response(deviceKey: deviceKey, controllerNonce: nonce,
                agentNonce: agentNonce, challenge: challenge.challenge, agentIdentifier: challenge.agentIdentifier)
            return [outgoing(.authResponse, response)]
        case .authResult:
            guard gate.phase == .authenticated else { throw TLSProbeError.authenticationRejected }
            return [outgoing(.ping, ping)]
        case .pong:
            guard frame.payload == ping else { throw ProtocolError.invalidPayload }
            isComplete = true
            return []
        default:
            throw ProtocolError.invalidState
        }
    }

    private mutating func outgoing(_ type: MessageType, _ payload: Data) -> Frame {
        let frame = Frame(type: type, sequence: outgoingSequence, payload: payload)
        outgoingSequence = outgoingSequence == .max ? 1 : outgoingSequence + 1
        return frame
    }
}