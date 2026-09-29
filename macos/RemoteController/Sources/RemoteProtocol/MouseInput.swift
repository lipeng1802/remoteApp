import Foundation

public struct MouseMovePayload: Equatable {
    public let x: UInt16
    public let y: UInt16
    public init(x: UInt16, y: UInt16) { self.x = x; self.y = y }
    public func encode() -> Data {
        Data([UInt8(x >> 8), UInt8(x & 255), UInt8(y >> 8), UInt8(y & 255)])
    }
    public static func decode(_ data: Data) throws -> Self {
        let b = Array(data)
        guard b.count == 4 else { throw ProtocolError.invalidPayload }
        return Self(x: UInt16(b[0]) << 8 | UInt16(b[1]), y: UInt16(b[2]) << 8 | UInt16(b[3]))
    }
}

public enum MouseButton: UInt8 { case left = 1, right = 2, middle = 3 }
public enum ButtonAction: UInt8 { case down = 1, up = 2 }
public struct MouseButtonPayload: Equatable {
    public let button: MouseButton
    public let action: ButtonAction
    public init(button: MouseButton, action: ButtonAction) { self.button = button; self.action = action }
    public func encode() -> Data { Data([button.rawValue, action.rawValue]) }
    public static func decode(_ data: Data) throws -> Self {
        let b = Array(data)
        guard b.count == 2, let button = MouseButton(rawValue: b[0]), let action = ButtonAction(rawValue: b[1])
        else { throw ProtocolError.invalidPayload }
        return Self(button: button, action: action)
    }
}

public struct MouseWheelPayload: Equatable {
    public let horizontal: Int32
    public let vertical: Int32
    public init(horizontal: Int32, vertical: Int32) { self.horizontal = horizontal; self.vertical = vertical }
    public func encode() -> Data {
        var data = Data()
        for value in [horizontal, vertical] {
            let bits = UInt32(bitPattern: value)
            data.append(contentsOf: [UInt8(bits >> 24), UInt8((bits >> 16) & 255),
                                     UInt8((bits >> 8) & 255), UInt8(bits & 255)])
        }
        return data
    }
    public static func decode(_ data: Data) throws -> Self {
        let b = Array(data)
        guard b.count == 8 else { throw ProtocolError.invalidPayload }
        func number(_ offset: Int) -> Int32 {
            let value = b[offset..<offset + 4].reduce(UInt32(0)) { ($0 << 8) | UInt32($1) }
            return Int32(bitPattern: value)
        }
        return Self(horizontal: number(0), vertical: number(4))
    }
}

public enum MouseCoordinates {
    // Top-left origin; callers must flip AppKit bottom-left coordinates if needed.
    public static func map(x: Double, y: Double, viewWidth: Double, viewHeight: Double,
                           screenWidth: Int, screenHeight: Int) -> MouseMovePayload? {
        guard x.isFinite, y.isFinite, viewWidth.isFinite, viewHeight.isFinite,
              viewWidth > 0, viewHeight > 0,
              (1...16384).contains(screenWidth), (1...16384).contains(screenHeight) else { return nil }
        let scale = min(viewWidth / Double(screenWidth), viewHeight / Double(screenHeight))
        let width = Double(screenWidth) * scale
        let height = Double(screenHeight) * scale
        guard width > 0, height > 0 else { return nil }
        let left = (viewWidth - width) / 2
        let top = (viewHeight - height) / 2
        guard x >= left, x <= left + width, y >= top, y <= top + height else { return nil }
        return MouseMovePayload(x: UInt16((min(1, max(0, (x - left) / width)) * 65535).rounded()),
                                y: UInt16((min(1, max(0, (y - top) / height)) * 65535).rounded()))
    }
}
