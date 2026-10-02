import Foundation

/// Unsequenced commands: a future authenticated transport owns frame sequencing.
/// This state machine grants no network or remote-control permission.
public enum CapturedInput: Equatable {
    case move(MouseMovePayload)
    case button(MouseButtonPayload)
    case wheel(MouseWheelPayload)
    case key(KeyEventPayload)
    case clipboardRequest
    case clipboardSet(ClipboardTextPayload)

    public var messageType: MessageType {
        switch self {
        case .move: return .mouseMove
        case .button: return .mouseButton
        case .wheel: return .mouseWheel
        case .key: return .keyEvent
        case .clipboardRequest: return .clipboardRequest
        case .clipboardSet: return .clipboardSetText
        }
    }
    public var payload: Data {
        switch self {
        case .move(let value): return value.encode()
        case .button(let value): return value.encode()
        case .wheel(let value): return value.encode()
        case .key(let value): return value.encode()
        case .clipboardRequest: return Data()
        case .clipboardSet(let value): return (try? value.encode()) ?? Data()
        }
    }
}

/// Serial/main-thread use. No event history, network queue, or OS input injection.
public struct ControllerInputCapture {
    public private(set) var isActive = false
    private var keys: Set<UInt16> = []
    private var buttons: Set<UInt8> = []
    private var modifierKeys: Set<UInt16> = []
    private var modifiers = MacModifierTracker()
    private var wheelX = 0.0
    private var wheelY = 0.0

    public init() {}
    public var heldCount: Int { keys.count + buttons.count + modifierKeys.count }

    /// Activation is explicit; calling it twice must not forget held inputs.
    public mutating func start() { isActive = true }

    /// Call before losing focus, changing sessions, stopping or closing.
    /// Deliver releases to the sink before discarding a live transport.
    public mutating func stop() -> [CapturedInput] {
        isActive = false
        var result = keys.sorted().compactMap {
            MacKeyboardMapper.event(keyCode: $0, action: .up).map(CapturedInput.key)
        }
        result += modifiers.releaseAll().map(CapturedInput.key)
        result += buttons.sorted().compactMap { raw in
            MouseButton(rawValue: raw).map { .button(MouseButtonPayload(button: $0, action: .up)) }
        }
        keys.removeAll()
        buttons.removeAll()
        modifierKeys.removeAll()
        wheelX = 0
        wheelY = 0
        return result
    }

    public mutating func key(keyCode: UInt16, action: KeyAction, isRepeat: Bool = false) -> [CapturedInput] {
        guard isActive, !MacKeyboardMapper.modifierKeyCodes.contains(keyCode),
              let event = MacKeyboardMapper.event(keyCode: keyCode, action: action) else { return [] }
        if action == .down {
            // A repeat arriving after refocus must not start a new held key.
            if isRepeat {
                guard keys.contains(keyCode) else { return [] }
            } else {
                guard keys.insert(keyCode).inserted else { return [] }
            }
        } else {
            guard keys.remove(keyCode) != nil else { return [] }
        }
        return [.key(event)]
    }

    public mutating func modifierSnapshot(_ pressed: Set<UInt16>) -> [CapturedInput] {
        guard isActive else { return [] }
        modifierKeys = pressed.intersection(MacKeyboardMapper.modifierKeyCodes)
        return modifiers.update(pressedKeyCodes: modifierKeys).map(CapturedInput.key)
    }

    /// User-initiated Command+C convenience for a Windows target. Temporarily
    /// release every tracked key already mirrored to Windows, perform an
    /// isolated Ctrl+C, request clipboard text, then restore held keys.
    public func clipboardCopyShortcut() -> [CapturedInput] {
        let commandCodes: Set<UInt16> = [54, 55]
        guard isActive, !modifierKeys.isDisjoint(with: commandCodes) else { return [] }
        var result = heldKeyEvents(action: .up)
        guard let controlDown = try? KeyEventPayload(scanCode: 0x1d, extended: false, action: .down),
              let cDown = try? KeyEventPayload(scanCode: 0x2e, extended: false, action: .down),
              let cUp = try? KeyEventPayload(scanCode: 0x2e, extended: false, action: .up),
              let controlUp = try? KeyEventPayload(scanCode: 0x1d, extended: false, action: .up) else {
            return []
        }
        result += [.key(controlDown), .key(cDown), .key(cUp), .key(controlUp), .clipboardRequest]
        result += heldKeyEvents(action: .down)
        return result
    }

    public func clipboardPasteShortcut(text: String) -> [CapturedInput] {
        let commandCodes: Set<UInt16> = [54, 55]
        guard isActive, !modifierKeys.isDisjoint(with: commandCodes),
              Data(text.utf8).count <= ClipboardTextPayload.maximumTextBytes else { return [] }
        return heldKeyEvents(action: .up)
            + [.clipboardSet(ClipboardTextPayload(status: .success, text: text))]
            + heldKeyEvents(action: .down)
    }

    private func heldKeyEvents(action: KeyAction) -> [CapturedInput] {
        let ordinary = keys.sorted().compactMap {
            MacKeyboardMapper.event(keyCode: $0, action: action).map(CapturedInput.key)
        }
        let modifierEvents = modifierKeys.sorted().compactMap {
            MacKeyboardMapper.event(keyCode: $0, action: action).map(CapturedInput.key)
        }
        return ordinary + modifierEvents
    }

    public func move(to point: MouseMovePayload?) -> [CapturedInput] {
        guard isActive, let point else { return [] }
        return [.move(point)]
    }

    public mutating func button(_ button: MouseButton, action: ButtonAction,
                                at point: MouseMovePayload?) -> [CapturedInput] {
        guard isActive else { return [] }
        if action == .down {
            guard let point, buttons.insert(button.rawValue).inserted else { return [] }
            // Position always precedes down, even without a prior mouseMoved.
            return [.move(point), .button(MouseButtonPayload(button: button, action: .down))]
        }
        // Release a tracked button even if the pointer has left the image.
        guard buttons.remove(button.rawValue) != nil else { return [] }
        return move(to: point) + [.button(MouseButtonPayload(button: button, action: .up))]
    }

    /// Deltas already have Windows wheel-unit signs: vertical positive = up,
    /// horizontal positive = right. 120 units = one discrete notch.
    /// The UI adapter chooses pixel scaling; fractional motion is accumulated.
    public mutating func wheel(horizontal: Double, vertical: Double,
                               at point: MouseMovePayload?) -> [CapturedInput] {
        guard isActive, let point, horizontal.isFinite, vertical.isFinite else { return [] }
        // Bound a single event before converting to Int32; preserve sub-unit motion.
        wheelX += min(1200, max(-1200, horizontal))
        wheelY += min(1200, max(-1200, vertical))
        let x = Int32(wheelX.rounded(.towardZero))
        let y = Int32(wheelY.rounded(.towardZero))
        wheelX -= Double(x)
        wheelY -= Double(y)
        guard x != 0 || y != 0 else { return [] }
        return [.move(point), .wheel(MouseWheelPayload(horizontal: x, vertical: y))]
    }
}
