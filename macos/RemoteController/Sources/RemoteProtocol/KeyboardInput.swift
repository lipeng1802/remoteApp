import Foundation

public enum KeyAction: UInt8 { case down = 1, up = 2 }

public struct KeyEventPayload: Equatable {
    public let scanCode: UInt16
    public let extended: Bool
    public let action: KeyAction

    public init(scanCode: UInt16, extended: Bool, action: KeyAction) throws {
        guard (1...0x7f).contains(scanCode) else { throw ProtocolError.invalidPayload }
        self.scanCode = scanCode
        self.extended = extended
        self.action = action
    }

    public func encode() -> Data {
        Data([UInt8(scanCode >> 8), UInt8(scanCode & 255), extended ? 1 : 0, action.rawValue])
    }

    public static func decode(_ data: Data) throws -> Self {
        let bytes = Array(data)
        guard bytes.count == 4, bytes[2] <= 1, let action = KeyAction(rawValue: bytes[3])
        else { throw ProtocolError.invalidPayload }
        return try Self(scanCode: UInt16(bytes[0]) << 8 | UInt16(bytes[1]),
                        extended: bytes[2] == 1, action: action)
    }
}
