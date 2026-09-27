import CryptoKit
import Foundation

public enum PeerRole: UInt8, Codable {
    case controller = 1
    case agent = 2
}

public struct Capabilities: OptionSet, Equatable, Sendable {
    public let rawValue: UInt32

    public init(rawValue: UInt32) {
        self.rawValue = rawValue
    }

    public static let jpeg = Capabilities(rawValue: 1 << 0)
    public static let h264 = Capabilities(rawValue: 1 << 1)
    public static let input = Capabilities(rawValue: 1 << 2)
    public static let reconnect = Capabilities(rawValue: 1 << 3)
}

public struct HelloPayload: Equatable {
    public static let encodedLength = 41

    public let role: PeerRole
    public let minimumVersion: UInt16
    public let maximumVersion: UInt16
    public let capabilities: Capabilities
    public let nonce: Data

    public init(
        role: PeerRole,
        minimumVersion: UInt16 = ProtocolConstants.version,
        maximumVersion: UInt16 = ProtocolConstants.version,
        capabilities: Capabilities,
        nonce: Data
    ) {
        self.role = role
        self.minimumVersion = minimumVersion
        self.maximumVersion = maximumVersion
        self.capabilities = capabilities
        self.nonce = nonce
    }

    public func encode() throws -> Data {
        guard nonce.count == 32, minimumVersion <= maximumVersion else {
            throw ProtocolError.invalidPayload
        }
        var output = Data(capacity: Self.encodedLength)
        output.append(role.rawValue)
        output.appendUInt16(minimumVersion)
        output.appendUInt16(maximumVersion)
        output.appendUInt32(capabilities.rawValue)
        output.append(nonce)
        return output
    }

    public static func decode(_ data: Data) throws -> HelloPayload {
        guard data.count == encodedLength,
              let role = PeerRole(rawValue: data[0]) else {
            throw ProtocolError.invalidPayload
        }
        let minimumVersion = data.uint16(at: 1)
        let maximumVersion = data.uint16(at: 3)
        guard minimumVersion <= maximumVersion else {
            throw ProtocolError.invalidPayload
        }
        return HelloPayload(
            role: role,
            minimumVersion: minimumVersion,
            maximumVersion: maximumVersion,
            capabilities: Capabilities(rawValue: data.uint32(at: 5)),
            nonce: Data(data[9..<41])
        )
    }
}

public struct AuthChallengePayload: Equatable {
    public static let encodedLength = 48

    public let challenge: Data
    public let agentIdentifier: Data

    public init(challenge: Data, agentIdentifier: Data) {
        self.challenge = challenge
        self.agentIdentifier = agentIdentifier
    }

    public func encode() throws -> Data {
        guard challenge.count == 32, agentIdentifier.count == 16 else {
            throw ProtocolError.invalidPayload
        }
        return challenge + agentIdentifier
    }

    public static func decode(_ data: Data) throws -> AuthChallengePayload {
        guard data.count == encodedLength else { throw ProtocolError.invalidPayload }
        return AuthChallengePayload(
            challenge: Data(data[0..<32]),
            agentIdentifier: Data(data[32..<48])
        )
    }
}

public enum AuthResultStatus: UInt8 {
    case success = 0
    case rejected = 1
    case temporarilyLocked = 2
}

public struct AuthResultPayload: Equatable {
    public static let encodedLength = 5

    public let status: AuthResultStatus
    public let retryDelayMilliseconds: UInt32

    public init(status: AuthResultStatus, retryDelayMilliseconds: UInt32) {
        self.status = status
        self.retryDelayMilliseconds = retryDelayMilliseconds
    }

    public func encode() throws -> Data {
        guard (status == .success && retryDelayMilliseconds == 0) || status != .success else {
            throw ProtocolError.invalidPayload
        }
        var output = Data([status.rawValue])
        output.appendUInt32(retryDelayMilliseconds)
        return output
    }

    public static func decode(_ data: Data) throws -> AuthResultPayload {
        guard data.count == encodedLength,
              let status = AuthResultStatus(rawValue: data[0]) else {
            throw ProtocolError.invalidPayload
        }
        let delay = data.uint32(at: 1)
        guard status != .success || delay == 0 else { throw ProtocolError.invalidPayload }
        return AuthResultPayload(status: status, retryDelayMilliseconds: delay)
    }
}

public enum Authentication {
    private static let context = Data("PRD-AUTH-V1".utf8)

    public static func response(
        deviceKey: Data,
        controllerNonce: Data,
        agentNonce: Data,
        challenge: Data,
        agentIdentifier: Data
    ) throws -> Data {
        guard deviceKey.count == 32,
              controllerNonce.count == 32,
              agentNonce.count == 32,
              challenge.count == 32,
              agentIdentifier.count == 16 else {
            throw ProtocolError.invalidPayload
        }
        let message = context + controllerNonce + agentNonce + challenge + agentIdentifier
        let code = HMAC<SHA256>.authenticationCode(
            for: message,
            using: SymmetricKey(data: deviceKey)
        )
        return Data(code)
    }

    public static func constantTimeEquals(_ left: Data, _ right: Data) -> Bool {
        guard left.count == right.count else { return false }
        var difference: UInt8 = 0
        for (lhs, rhs) in zip(left, right) {
            difference |= lhs ^ rhs
        }
        return difference == 0
    }
}

public enum SessionPhase: Equatable {
    case awaitingHello
    case authenticating
    case authenticated
    case closing
}

public struct SessionGate {
    public let localRole: PeerRole
    public private(set) var phase: SessionPhase = .awaitingHello
    private var receivedChallenge = false
    private var receivedResponse: Data?

    public init(localRole: PeerRole) {
        self.localRole = localRole
    }

    public mutating func receive(_ frame: Frame) throws {
        if frame.type.requiresAuthentication && phase != .authenticated {
            throw ProtocolError.authRequired
        }
        if frame.type == .disconnect || frame.type == .error {
            phase = .closing
            return
        }

        switch phase {
        case .awaitingHello:
            guard frame.type == .hello else { throw ProtocolError.invalidState }
            let hello = try HelloPayload.decode(frame.payload)
            guard hello.role != localRole,
                  hello.minimumVersion <= ProtocolConstants.version,
                  hello.maximumVersion >= ProtocolConstants.version else {
                throw ProtocolError.invalidPayload
            }
            phase = .authenticating

        case .authenticating:
            switch localRole {
            case .controller:
                if frame.type == .authChallenge && !receivedChallenge {
                    _ = try AuthChallengePayload.decode(frame.payload)
                    receivedChallenge = true
                } else if frame.type == .authResult && receivedChallenge {
                    let result = try AuthResultPayload.decode(frame.payload)
                    phase = result.status == .success ? .authenticated : .closing
                } else {
                    throw ProtocolError.invalidState
                }
            case .agent:
                guard frame.type == .authResponse,
                      receivedResponse == nil,
                      frame.payload.count == 32 else {
                    throw ProtocolError.invalidState
                }
                receivedResponse = frame.payload
            }

        case .authenticated:
            guard !frame.type.isHandshake else { throw ProtocolError.invalidState }

        case .closing:
            throw ProtocolError.invalidState
        }
    }

    public mutating func completeAgentAuthentication(expectedResponse: Data) throws {
        guard localRole == .agent,
              phase == .authenticating,
              let receivedResponse else {
            throw ProtocolError.invalidState
        }
        let success = Authentication.constantTimeEquals(receivedResponse, expectedResponse)
        phase = success ? .authenticated : .closing
    }
}

public extension MessageType {
    var requiresAuthentication: Bool {
        switch self {
        case .screenInfo, .videoFrameJPEG, .videoFrameH264,
             .mouseMove, .mouseButton, .mouseWheel, .keyEvent, .ping, .pong:
            return true
        default:
            return false
        }
    }

    var isHandshake: Bool {
        self == .hello || self == .authChallenge || self == .authResponse || self == .authResult
    }
}

private extension Data {
    mutating func appendUInt16(_ value: UInt16) {
        append(UInt8((value >> 8) & 0xff))
        append(UInt8(value & 0xff))
    }

    mutating func appendUInt32(_ value: UInt32) {
        append(UInt8((value >> 24) & 0xff))
        append(UInt8((value >> 16) & 0xff))
        append(UInt8((value >> 8) & 0xff))
        append(UInt8(value & 0xff))
    }

    func uint16(at offset: Int) -> UInt16 {
        (UInt16(self[offset]) << 8) | UInt16(self[offset + 1])
    }

    func uint32(at offset: Int) -> UInt32 {
        (UInt32(self[offset]) << 24)
            | (UInt32(self[offset + 1]) << 16)
            | (UInt32(self[offset + 2]) << 8)
            | UInt32(self[offset + 3])
    }
}
