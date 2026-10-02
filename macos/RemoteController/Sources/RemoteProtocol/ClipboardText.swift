import Foundation

public enum ClipboardTextStatus: UInt8, Equatable {
    case success = 0
    case unavailable = 1
    case tooLarge = 2
}

public struct ClipboardTextPayload: Equatable {
    public static let maximumTextBytes = 32 * 1024

    public let status: ClipboardTextStatus
    public let text: String

    public init(status: ClipboardTextStatus, text: String = "") {
        self.status = status
        self.text = text
    }

    public func encode() throws -> Data {
        guard status == .success || text.isEmpty else { throw ProtocolError.invalidPayload }
        let bytes = Data(text.utf8)
        guard bytes.count <= Self.maximumTextBytes else { throw ProtocolError.invalidPayload }
        return Data([status.rawValue]) + bytes
    }

    public static func decode(_ payload: Data) throws -> ClipboardTextPayload {
        guard (1...(maximumTextBytes + 1)).contains(payload.count),
              let status = ClipboardTextStatus(rawValue: payload[0]),
              status == .success || payload.count == 1 else {
            throw ProtocolError.invalidPayload
        }
        let bytes = payload.dropFirst()
        guard let text = String(data: bytes, encoding: .utf8), Data(text.utf8) == bytes else {
            throw ProtocolError.invalidPayload
        }
        return ClipboardTextPayload(status: status, text: text)
    }
}
