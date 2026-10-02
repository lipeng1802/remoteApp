import Foundation
import Carbon

// Physical ANSI positions, not character/IME translation. No event tap or GUI hook.
public enum MacKeyboardMapper {
    public static func event(keyCode: UInt16, action: KeyAction) -> KeyEventPayload? {
        let code: UInt16
        let extended: Bool
        switch keyCode {
        case UInt16(kVK_ANSI_A): code = 0x1e; extended = false
        case UInt16(kVK_ANSI_S): code = 0x1f; extended = false
        case UInt16(kVK_ANSI_D): code = 0x20; extended = false
        case UInt16(kVK_ANSI_F): code = 0x21; extended = false
        case UInt16(kVK_ANSI_H): code = 0x23; extended = false
        case UInt16(kVK_ANSI_G): code = 0x22; extended = false
        case UInt16(kVK_ANSI_Z): code = 0x2c; extended = false
        case UInt16(kVK_ANSI_X): code = 0x2d; extended = false
        case UInt16(kVK_ANSI_C): code = 0x2e; extended = false
        case UInt16(kVK_ANSI_V): code = 0x2f; extended = false
        case UInt16(kVK_ANSI_B): code = 0x30; extended = false
        case UInt16(kVK_ANSI_Q): code = 0x10; extended = false
        case UInt16(kVK_ANSI_W): code = 0x11; extended = false
        case UInt16(kVK_ANSI_E): code = 0x12; extended = false
        case UInt16(kVK_ANSI_R): code = 0x13; extended = false
        case UInt16(kVK_ANSI_Y): code = 0x15; extended = false
        case UInt16(kVK_ANSI_T): code = 0x14; extended = false
        case UInt16(kVK_ANSI_1): code = 0x2; extended = false
        case UInt16(kVK_ANSI_2): code = 0x3; extended = false
        case UInt16(kVK_ANSI_3): code = 0x4; extended = false
        case UInt16(kVK_ANSI_4): code = 0x5; extended = false
        case UInt16(kVK_ANSI_6): code = 0x7; extended = false
        case UInt16(kVK_ANSI_5): code = 0x6; extended = false
        case UInt16(kVK_ANSI_Equal): code = 0xd; extended = false
        case UInt16(kVK_ANSI_9): code = 0xa; extended = false
        case UInt16(kVK_ANSI_7): code = 0x8; extended = false
        case UInt16(kVK_ANSI_Minus): code = 0xc; extended = false
        case UInt16(kVK_ANSI_8): code = 0x9; extended = false
        case UInt16(kVK_ANSI_0): code = 0xb; extended = false
        case UInt16(kVK_ANSI_RightBracket): code = 0x1b; extended = false
        case UInt16(kVK_ANSI_O): code = 0x18; extended = false
        case UInt16(kVK_ANSI_U): code = 0x16; extended = false
        case UInt16(kVK_ANSI_LeftBracket): code = 0x1a; extended = false
        case UInt16(kVK_ANSI_I): code = 0x17; extended = false
        case UInt16(kVK_ANSI_P): code = 0x19; extended = false
        case UInt16(kVK_ANSI_L): code = 0x26; extended = false
        case UInt16(kVK_ANSI_J): code = 0x24; extended = false
        case UInt16(kVK_ANSI_Quote): code = 0x28; extended = false
        case UInt16(kVK_ANSI_K): code = 0x25; extended = false
        case UInt16(kVK_ANSI_Semicolon): code = 0x27; extended = false
        case UInt16(kVK_ANSI_Backslash): code = 0x2b; extended = false
        case UInt16(kVK_ANSI_Comma): code = 0x33; extended = false
        case UInt16(kVK_ANSI_Slash): code = 0x35; extended = false
        case UInt16(kVK_ANSI_N): code = 0x31; extended = false
        case UInt16(kVK_ANSI_M): code = 0x32; extended = false
        case UInt16(kVK_ANSI_Period): code = 0x34; extended = false
        case UInt16(kVK_ANSI_Grave): code = 0x29; extended = false
        case UInt16(kVK_Return): code = 0x1c; extended = false
        case UInt16(kVK_Tab): code = 0xf; extended = false
        case UInt16(kVK_Space): code = 0x39; extended = false
        case UInt16(kVK_Delete): code = 0xe; extended = false
        case UInt16(kVK_Escape): code = 0x1; extended = false
        case UInt16(kVK_Shift): code = 0x2a; extended = false
        case UInt16(kVK_RightShift): code = 0x36; extended = false
        case UInt16(kVK_Control): code = 0x1d; extended = false
        case UInt16(kVK_Option): code = 0x38; extended = false
        case UInt16(kVK_F1): code = 0x3b; extended = false
        case UInt16(kVK_F2): code = 0x3c; extended = false
        case UInt16(kVK_F3): code = 0x3d; extended = false
        case UInt16(kVK_F4): code = 0x3e; extended = false
        case UInt16(kVK_F5): code = 0x3f; extended = false
        case UInt16(kVK_F6): code = 0x40; extended = false
        case UInt16(kVK_F7): code = 0x41; extended = false
        case UInt16(kVK_F8): code = 0x42; extended = false
        case UInt16(kVK_F9): code = 0x43; extended = false
        case UInt16(kVK_F10): code = 0x44; extended = false
        case UInt16(kVK_F11): code = 0x57; extended = false
        case UInt16(kVK_F12): code = 0x58; extended = false
        case UInt16(kVK_Command): code = 0x5b; extended = true
        case UInt16(kVK_RightCommand): code = 0x5c; extended = true
        case UInt16(kVK_RightControl): code = 0x1d; extended = true
        case UInt16(kVK_RightOption): code = 0x38; extended = true
        case UInt16(kVK_LeftArrow): code = 0x4b; extended = true
        case UInt16(kVK_RightArrow): code = 0x4d; extended = true
        case UInt16(kVK_UpArrow): code = 0x48; extended = true
        case UInt16(kVK_DownArrow): code = 0x50; extended = true
        case UInt16(kVK_Home): code = 0x47; extended = true
        case UInt16(kVK_End): code = 0x4f; extended = true
        case UInt16(kVK_PageUp): code = 0x49; extended = true
        case UInt16(kVK_PageDown): code = 0x51; extended = true
        case UInt16(kVK_ForwardDelete): code = 0x53; extended = true
        case UInt16(kVK_ANSI_KeypadEnter): code = 0x1c; extended = true
        default: return nil // Caps Lock, Fn, media, ISO/JIS extras and unsupported keys.
        }
        return try? KeyEventPayload(scanCode: code, extended: extended, action: action)
    }

    static let modifierKeyCodes: Set<UInt16> = [
        UInt16(kVK_Command), UInt16(kVK_RightCommand), UInt16(kVK_Control), UInt16(kVK_RightControl),
        UInt16(kVK_Option), UInt16(kVK_RightOption), UInt16(kVK_Shift), UInt16(kVK_RightShift)
    ]
}

// The UI adapter must supply a left/right-aware snapshot; aggregate NSEvent flags
// alone cannot distinguish release of one side while its partner remains held.
public struct MacModifierTracker {
    private var held: Set<UInt16> = []
    var heldKeyCodes: Set<UInt16> { held }
    public init() {}
    public mutating func update(pressedKeyCodes: Set<UInt16>) -> [KeyEventPayload] {
        let next = pressedKeyCodes.intersection(MacKeyboardMapper.modifierKeyCodes)
        let ups = held.subtracting(next).sorted().compactMap { MacKeyboardMapper.event(keyCode: $0, action: .up) }
        let downs = next.subtracting(held).sorted().compactMap { MacKeyboardMapper.event(keyCode: $0, action: .down) }
        held = next
        return ups + downs
    }
    public mutating func releaseAll() -> [KeyEventPayload] { update(pressedKeyCodes: []) }
}

/// Builds a side-aware modifier snapshot from AppKit `flagsChanged` events.
/// Aggregate modifier flags cannot distinguish two keys of the same kind, so
/// the physical keyCode that generated each transition owns the state change.
public struct MacModifierEventState {
    public private(set) var pressedKeyCodes: Set<UInt16> = []
    public init() {}

    public mutating func update(keyCode: UInt16, aggregatePressed: Bool) -> Set<UInt16> {
        guard MacKeyboardMapper.modifierKeyCodes.contains(keyCode) else {
            return pressedKeyCodes
        }
        if pressedKeyCodes.remove(keyCode) == nil, aggregatePressed {
            pressedKeyCodes.insert(keyCode)
        }
        return pressedKeyCodes
    }

    public mutating func reset() { pressedKeyCodes.removeAll() }
}
