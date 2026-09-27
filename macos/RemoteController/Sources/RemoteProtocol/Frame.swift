import Foundation

public enum ProtocolConstants {
    public static let magic = Data([0x50, 0x52, 0x44, 0x31])
    public static let version: UInt16 = 1
    public static let headerLength = 28
    public static let maximumPayloadLength = 8 * 1024 * 1024
    public static let maximumControlPayloadLength = 64 * 1024
}

public enum MessageType: UInt16, Codable, CaseIterable {
    case hello = 0x0001
    case authChallenge = 0x0002
    case authResponse = 0x0003
    case authResult = 0x0004
    case screenInfo = 0x0005
    case videoFrameJPEG = 0x0006
    case videoFrameH264 = 0x0007
    case mouseMove = 0x0008
    case mouseButton = 0x0009
    case mouseWheel = 0x000a
    case keyEvent = 0x000b
    case ping = 0x000c
    case pong = 0x000d
    case disconnect = 0x000e
    case error = 0x000f

    var isVideo: Bool {
        self == .videoFrameJPEG || self == .videoFrameH264
    }
}

public struct Frame: Equatable {
    public let type: MessageType
    public let flags: UInt16
    public let sequence: UInt32
    public let timestampMicros: UInt64
    public let payload: Data

    public init(
        type: MessageType,
        flags: UInt16 = 0,
        sequence: UInt32,
        timestampMicros: UInt64 = 0,
        payload: Data = Data()
    ) {
        self.type = type
        self.flags = flags
        self.sequence = sequence
        self.timestampMicros = timestampMicros
        self.payload = payload
    }
}

public enum ProtocolError: Error, Equatable {
    case invalidMagic
    case unsupportedVersion(UInt16)
    case invalidHeaderLength(UInt16)
    case unsupportedMessageType(UInt16)
    case nonzeroFlags(UInt16)
    case messageTooLarge(Int)
    case incompleteFrame
}

public enum FrameCodec {
    public static func encode(_ frame: Frame) throws -> Data {
        let limit = frame.type.isVideo
            ? ProtocolConstants.maximumPayloadLength
            : ProtocolConstants.maximumControlPayloadLength
        guard frame.payload.count <= limit else {
            throw ProtocolError.messageTooLarge(frame.payload.count)
        }
        guard frame.flags == 0 else {
            throw ProtocolError.nonzeroFlags(frame.flags)
        }

        var result = Data(capacity: ProtocolConstants.headerLength + frame.payload.count)
        result.append(ProtocolConstants.magic)
        result.appendBigEndian(ProtocolConstants.version)
        result.appendBigEndian(UInt16(ProtocolConstants.headerLength))
        result.appendBigEndian(frame.type.rawValue)
        result.appendBigEndian(frame.flags)
        result.appendBigEndian(UInt32(frame.payload.count))
        result.appendBigEndian(frame.sequence)
        result.appendBigEndian(frame.timestampMicros)
        result.append(frame.payload)
        return result
    }
}

public struct FrameDecoder {
    private var buffer = Data()

    public init() {}

    public mutating func append(_ data: Data) throws -> [Frame] {
        buffer.append(data)
        var frames: [Frame] = []

        while buffer.count >= ProtocolConstants.headerLength {
            guard buffer.prefix(4) == ProtocolConstants.magic else {
                throw ProtocolError.invalidMagic
            }

            let version = buffer.readUInt16(at: 4)
            guard version == ProtocolConstants.version else {
                throw ProtocolError.unsupportedVersion(version)
            }

            let headerLength = buffer.readUInt16(at: 6)
            guard headerLength == UInt16(ProtocolConstants.headerLength) else {
                throw ProtocolError.invalidHeaderLength(headerLength)
            }

            let rawType = buffer.readUInt16(at: 8)
            guard let type = MessageType(rawValue: rawType) else {
                throw ProtocolError.unsupportedMessageType(rawType)
            }

            let flags = buffer.readUInt16(at: 10)
            guard flags == 0 else {
                throw ProtocolError.nonzeroFlags(flags)
            }

            let payloadLength = Int(buffer.readUInt32(at: 12))
            let limit = type.isVideo
                ? ProtocolConstants.maximumPayloadLength
                : ProtocolConstants.maximumControlPayloadLength
            guard payloadLength <= limit else {
                throw ProtocolError.messageTooLarge(payloadLength)
            }

            let frameLength = ProtocolConstants.headerLength + payloadLength
            guard buffer.count >= frameLength else { break }

            let sequence = buffer.readUInt32(at: 16)
            let timestamp = buffer.readUInt64(at: 20)
            let payload = Data(buffer[ProtocolConstants.headerLength..<frameLength])
            frames.append(Frame(
                type: type,
                flags: flags,
                sequence: sequence,
                timestampMicros: timestamp,
                payload: payload
            ))
            buffer = Data(buffer.dropFirst(frameLength))
        }

        return frames
    }

    public func finish() throws {
        guard buffer.isEmpty else { throw ProtocolError.incompleteFrame }
    }
}

private extension Data {
    mutating func appendBigEndian(_ value: UInt16) {
        append(UInt8((value >> 8) & 0xff))
        append(UInt8(value & 0xff))
    }

    mutating func appendBigEndian(_ value: UInt32) {
        append(UInt8((value >> 24) & 0xff))
        append(UInt8((value >> 16) & 0xff))
        append(UInt8((value >> 8) & 0xff))
        append(UInt8(value & 0xff))
    }

    mutating func appendBigEndian(_ value: UInt64) {
        append(UInt8((value >> 56) & 0xff))
        append(UInt8((value >> 48) & 0xff))
        append(UInt8((value >> 40) & 0xff))
        append(UInt8((value >> 32) & 0xff))
        append(UInt8((value >> 24) & 0xff))
        append(UInt8((value >> 16) & 0xff))
        append(UInt8((value >> 8) & 0xff))
        append(UInt8(value & 0xff))
    }

    func readUInt16(at offset: Int) -> UInt16 {
        (UInt16(self[offset]) << 8) | UInt16(self[offset + 1])
    }

    func readUInt32(at offset: Int) -> UInt32 {
        (UInt32(self[offset]) << 24)
            | (UInt32(self[offset + 1]) << 16)
            | (UInt32(self[offset + 2]) << 8)
            | UInt32(self[offset + 3])
    }

    func readUInt64(at offset: Int) -> UInt64 {
        (UInt64(readUInt32(at: offset)) << 32) | UInt64(readUInt32(at: offset + 4))
    }
}
