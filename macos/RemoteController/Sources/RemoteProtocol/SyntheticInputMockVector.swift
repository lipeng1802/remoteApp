import Foundation

/// Development-only deterministic sequence shared with controller-input-v1.json.
/// It contains no captured user data and performs no input injection.
public enum SyntheticInputMockVector {
    public static func make() throws -> (inputs: [CapturedInput], releases: [CapturedInput]) {
        let center = CapturedInput.move(MouseMovePayload(x: 32_768, y: 32_768))
        return (inputs: [
            .key(try KeyEventPayload(scanCode: 0x1d, extended: false, action: .down)),
            .key(try KeyEventPayload(scanCode: 0x1d, extended: true, action: .down)),
            .key(try KeyEventPayload(scanCode: 0x1e, extended: false, action: .down)),
            .key(try KeyEventPayload(scanCode: 0x1e, extended: false, action: .down)),
            center,
            .button(MouseButtonPayload(button: .left, action: .down)),
            center,
            .wheel(MouseWheelPayload(horizontal: 0, vertical: -120)),
        ], releases: [
            .key(try KeyEventPayload(scanCode: 0x1e, extended: false, action: .up)),
            .key(try KeyEventPayload(scanCode: 0x1d, extended: false, action: .up)),
            .key(try KeyEventPayload(scanCode: 0x1d, extended: true, action: .up)),
            .button(MouseButtonPayload(button: .left, action: .up)),
        ])
    }
}
