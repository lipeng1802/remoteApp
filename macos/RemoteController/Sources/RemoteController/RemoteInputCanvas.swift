import AppKit
import SwiftUI
import RemoteProtocol

@MainActor
final class RemoteInputCanvas: NSView {
    var onInputs: (@MainActor ([CapturedInput]) -> Bool)?
    var onPaused: (@MainActor () -> Void)?
    var onFocusSuspended: (@MainActor () -> Void)?
    var onFocusResumed: (@MainActor () -> Void)?
    var onLocalClipboardIssue: (@MainActor (ClipboardTextStatus) -> Void)?
    private var capture = ControllerInputCapture()
    private var modifiers = MacModifierEventState()
    private var tracking: NSTrackingArea?
    private var screen: ScreenInfoPayload?
    private var enabled = false
    private var resumeWhenFocused = false
    private var clipboardShortcutKeyDown = false
    private var clipboardPasteKeyDown = false

    override var isFlipped: Bool { true }
    override var acceptsFirstResponder: Bool { true }
    override func acceptsFirstMouse(for event: NSEvent?) -> Bool { false }
    override func hitTest(_ point: NSPoint) -> NSView? { capture.isActive ? super.hitTest(point) : nil }

    override init(frame frameRect: NSRect) {
        super.init(frame: frameRect)
        NotificationCenter.default.addObserver(self, selector: #selector(focusLost(_:)),
            name: NSApplication.didResignActiveNotification, object: nil)
        NotificationCenter.default.addObserver(self, selector: #selector(focusRegained(_:)),
            name: NSApplication.didBecomeActiveNotification, object: nil)
    }

    required init?(coder: NSCoder) { nil }

    deinit {
        NotificationCenter.default.removeObserver(self)
    }

    override func viewDidMoveToWindow() {
        super.viewDidMoveToWindow()
        NotificationCenter.default.removeObserver(self, name: NSWindow.didResignKeyNotification, object: nil)
        NotificationCenter.default.removeObserver(self, name: NSWindow.didBecomeKeyNotification, object: nil)
        window?.acceptsMouseMovedEvents = true
        if let window {
            NotificationCenter.default.addObserver(self, selector: #selector(focusLost(_:)),
                name: NSWindow.didResignKeyNotification, object: window)
            NotificationCenter.default.addObserver(self, selector: #selector(focusRegained(_:)),
                name: NSWindow.didBecomeKeyNotification, object: window)
        }
    }

    @objc private func focusLost(_ notification: Notification) { suspendForFocusLoss() }
    @objc private func focusRegained(_ notification: Notification) { resumeAfterFocusLoss() }

    func update(screen: ScreenInfoPayload?, enabled: Bool) {
        self.screen = screen
        self.enabled = enabled
        if !enabled, capture.isActive { pause() }
    }

    @discardableResult
    func start() -> Bool {
        guard screen != nil, window?.isKeyWindow == true,
              window?.makeFirstResponder(self) == true else { return false }
        resumeWhenFocused = false
        modifiers.reset()
        clipboardShortcutKeyDown = false
        clipboardPasteKeyDown = false
        capture.start()
        return true
    }

    @discardableResult
    func pause() -> [CapturedInput] {
        resumeWhenFocused = false
        let releases = capture.stop()
        modifiers.reset()
        clipboardShortcutKeyDown = false
        clipboardPasteKeyDown = false
        if !releases.isEmpty { _ = onInputs?(releases) }
        onPaused?()
        return releases
    }

    func stopForDisconnect() -> [CapturedInput] {
        resumeWhenFocused = false
        let releases = capture.stop()
        modifiers.reset()
        clipboardShortcutKeyDown = false
        clipboardPasteKeyDown = false
        onPaused?()
        return releases
    }

    func abandon() {
        resumeWhenFocused = false
        _ = capture.stop()
        modifiers.reset()
        clipboardShortcutKeyDown = false
        clipboardPasteKeyDown = false
        onPaused?()
    }

    override func resignFirstResponder() -> Bool {
        suspendForFocusLoss()
        return true
    }

    private func suspendForFocusLoss() {
        guard capture.isActive else { return }
        resumeWhenFocused = enabled
        let releases = capture.stop()
        modifiers.reset()
        clipboardShortcutKeyDown = false
        clipboardPasteKeyDown = false
        if !releases.isEmpty { _ = onInputs?(releases) }
        if resumeWhenFocused { onFocusSuspended?() }
    }

    private func resumeAfterFocusLoss() {
        guard enabled, resumeWhenFocused, screen != nil,
              window?.isKeyWindow == true,
              window?.makeFirstResponder(self) == true else { return }
        resumeWhenFocused = false
        modifiers.reset()
        clipboardShortcutKeyDown = false
        clipboardPasteKeyDown = false
        capture.start()
        onFocusResumed?()
    }

    override func updateTrackingAreas() {
        super.updateTrackingAreas()
        if let tracking { removeTrackingArea(tracking) }
        let area = NSTrackingArea(rect: .zero,
            options: [.mouseMoved, .activeInKeyWindow, .inVisibleRect], owner: self, userInfo: nil)
        addTrackingArea(area)
        tracking = area
    }

    private func send(_ inputs: [CapturedInput]) {
        guard !inputs.isEmpty else { return }
        if onInputs?(inputs) != true {
            resumeWhenFocused = false
            _ = capture.stop()
            modifiers.reset()
            clipboardShortcutKeyDown = false
            clipboardPasteKeyDown = false
            onPaused?()
        }
    }

    private func point(_ event: NSEvent) -> MouseMovePayload? {
        guard let screen else { return nil }
        let point = convert(event.locationInWindow, from: nil)
        return MouseCoordinates.map(x: Double(point.x), y: Double(point.y),
            viewWidth: Double(bounds.width), viewHeight: Double(bounds.height),
            screenWidth: Int(screen.width), screenHeight: Int(screen.height))
    }

    private func aggregateFlag(for keyCode: UInt16) -> NSEvent.ModifierFlags? {
        switch keyCode {
        case 54, 55: return .command
        case 56, 60: return .shift
        case 58, 61: return .option
        case 59, 62: return .control
        default: return nil
        }
    }

    override func flagsChanged(with event: NSEvent) {
        guard capture.isActive, let flag = aggregateFlag(for: event.keyCode) else { return }
        let held = modifiers.update(keyCode: event.keyCode,
            aggregatePressed: event.modifierFlags.contains(flag))
        send(capture.modifierSnapshot(held))
    }

    override func keyDown(with event: NSEvent) {
        guard capture.isActive else { return }
        if event.keyCode == 53 { pause(); return }
        if event.keyCode == 8, event.modifierFlags.contains(.command) {
            guard !event.isARepeat, !clipboardShortcutKeyDown else { return }
            clipboardShortcutKeyDown = true
            send(capture.clipboardCopyShortcut())
            return
        }
        if event.keyCode == 9, event.modifierFlags.contains(.command) {
            guard !event.isARepeat, !clipboardPasteKeyDown else { return }
            guard let text = NSPasteboard.general.string(forType: .string) else {
                onLocalClipboardIssue?(.unavailable)
                return
            }
            guard Data(text.utf8).count <= ClipboardTextPayload.maximumTextBytes else {
                onLocalClipboardIssue?(.tooLarge)
                return
            }
            clipboardPasteKeyDown = true
            send(capture.clipboardPasteShortcut(text: text))
            return
        }
        send(capture.key(keyCode: event.keyCode, action: .down, isRepeat: event.isARepeat))
    }

    override func keyUp(with event: NSEvent) {
        guard capture.isActive else { return }
        if event.keyCode == 8, clipboardShortcutKeyDown {
            clipboardShortcutKeyDown = false
            return
        }
        if event.keyCode == 9, clipboardPasteKeyDown {
            clipboardPasteKeyDown = false
            return
        }
        send(capture.key(keyCode: event.keyCode, action: .up))
    }

    override func performKeyEquivalent(with event: NSEvent) -> Bool {
        guard capture.isActive, window?.firstResponder === self, event.type == .keyDown,
              MacKeyboardMapper.event(keyCode: event.keyCode, action: .down) != nil else {
            return super.performKeyEquivalent(with: event)
        }
        keyDown(with: event)
        return true
    }

    override func mouseMoved(with event: NSEvent) { send(capture.move(to: point(event))) }
    override func mouseDragged(with event: NSEvent) { mouseMoved(with: event) }
    override func rightMouseDragged(with event: NSEvent) { mouseMoved(with: event) }
    override func otherMouseDragged(with event: NSEvent) { mouseMoved(with: event) }

    private func button(_ event: NSEvent, _ button: MouseButton, _ action: ButtonAction) {
        send(capture.button(button, action: action, at: point(event)))
    }
    override func mouseDown(with event: NSEvent) { button(event, .left, .down) }
    override func mouseUp(with event: NSEvent) { button(event, .left, .up) }
    override func rightMouseDown(with event: NSEvent) { button(event, .right, .down) }
    override func rightMouseUp(with event: NSEvent) { button(event, .right, .up) }
    override func otherMouseDown(with event: NSEvent) {
        if event.buttonNumber == 2 { button(event, .middle, .down) }
    }
    override func otherMouseUp(with event: NSEvent) {
        if event.buttonNumber == 2 { button(event, .middle, .up) }
    }
    override func scrollWheel(with event: NSEvent) {
        let scale = event.hasPreciseScrollingDeltas ? 1.0 : 120.0
        send(capture.wheel(horizontal: -Double(event.scrollingDeltaX) * scale,
                           vertical: Double(event.scrollingDeltaY) * scale,
                           at: point(event)))
    }
}

@MainActor
struct RemoteInputOverlay: NSViewRepresentable {
    let screen: ScreenInfoPayload?
    let enabled: Bool
    let register: @MainActor (RemoteInputCanvas) -> Void
    let submit: @MainActor ([CapturedInput]) -> Bool
    let paused: @MainActor () -> Void
    let focusSuspended: @MainActor () -> Void
    let focusResumed: @MainActor () -> Void
    let localClipboardIssue: @MainActor (ClipboardTextStatus) -> Void

    func makeNSView(context: Context) -> RemoteInputCanvas {
        let view = RemoteInputCanvas()
        view.onInputs = submit
        view.onPaused = paused
        view.onFocusSuspended = focusSuspended
        view.onFocusResumed = focusResumed
        view.onLocalClipboardIssue = localClipboardIssue
        register(view)
        return view
    }

    func updateNSView(_ view: RemoteInputCanvas, context: Context) {
        view.onInputs = submit
        view.onPaused = paused
        view.onFocusSuspended = focusSuspended
        view.onFocusResumed = focusResumed
        view.onLocalClipboardIssue = localClipboardIssue
        view.update(screen: screen, enabled: enabled)
    }
}
