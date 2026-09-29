import AppKit
import CoreGraphics
import RemoteProtocol

// Development-only, in-memory sink. This executable has no transport or credentials.
final class InputPreviewCanvas: NSView {
    private var capture = ControllerInputCapture()
    private var eventCount: UInt64 = 0
    private var releaseCount: UInt64 = 0
    var onStatus: ((String) -> Void)?
    private var tracking: NSTrackingArea?
    private let modifierCodes: [UInt16] = [54, 55, 56, 58, 59, 60, 61, 62]

    override var isFlipped: Bool { true }
    override var acceptsFirstResponder: Bool { true }
    override func acceptsFirstMouse(for event: NSEvent?) -> Bool { false }

    func start() {
        guard window?.isKeyWindow == true, window?.makeFirstResponder(self) == true else { return }
        capture.start()
        // Do not import keys held in another application on entry.
        publish([])
        needsDisplay = true
    }

    func stop() {
        let releases = capture.stop()
        publish(releases)
        needsDisplay = true
    }

    override func resignFirstResponder() -> Bool {
        stop()
        return true
    }

    private func publish(_ commands: [CapturedInput]) {
        // Consume synchronously: no stored event history and no unbounded queue.
        for command in commands {
            _ = command.payload // Exercise the same encoder a future sender will use.
            eventCount &+= 1
            switch command {
            case .key(let key) where key.action == .up: releaseCount &+= 1
            case .button(let button) where button.action == .up: releaseCount &+= 1
            default: break
            }
        }
        let mode = capture.isActive ? "本地模拟中" : "已停止"
        onStatus?("\(mode) · 事件 \(eventCount) · 释放 \(releaseCount) · 持有 \(capture.heldCount)")
    }

    override func updateTrackingAreas() {
        super.updateTrackingAreas()
        if let tracking { removeTrackingArea(tracking) }
        let area = NSTrackingArea(rect: .zero,
            options: [.mouseMoved, .activeInKeyWindow, .inVisibleRect], owner: self, userInfo: nil)
        addTrackingArea(area)
        tracking = area
    }

    private func point(_ event: NSEvent) -> MouseMovePayload? {
        let p = convert(event.locationInWindow, from: nil)
        return MouseCoordinates.map(x: Double(p.x), y: Double(p.y),
            viewWidth: Double(bounds.width), viewHeight: Double(bounds.height),
            screenWidth: 1280, screenHeight: 720)
    }

    private func updateModifiers() {
        // Read only eight modifier states while this canvas is the active responder.
        // No global event tap/monitor. Side-aware state avoids aggregate flag ambiguity.
        let held = Set(modifierCodes.filter {
            CGEventSource.keyState(.combinedSessionState, key: CGKeyCode($0))
        })
        publish(capture.modifierSnapshot(held))
    }

    override func flagsChanged(with event: NSEvent) {
        guard capture.isActive else { return }
        updateModifiers()
    }
    override func keyDown(with event: NSEvent) {
        guard capture.isActive else { return }
        if event.keyCode == 53 { stop(); return } // Escape is always the local stop key.
        updateModifiers()
        publish(capture.key(keyCode: event.keyCode, action: .down, isRepeat: event.isARepeat))
    }
    override func keyUp(with event: NSEvent) {
        guard capture.isActive else { return }
        updateModifiers()
        publish(capture.key(keyCode: event.keyCode, action: .up))
    }
    override func performKeyEquivalent(with event: NSEvent) -> Bool {
        // Keep supported Command combinations inside the explicit preview canvas.
        guard capture.isActive, window?.firstResponder === self,
              event.type == .keyDown,
              MacKeyboardMapper.event(keyCode: event.keyCode, action: .down) != nil else {
            return super.performKeyEquivalent(with: event)
        }
        keyDown(with: event)
        return true
    }

    override func mouseMoved(with event: NSEvent) { publish(capture.move(to: point(event))) }
    override func mouseDragged(with event: NSEvent) { mouseMoved(with: event) }
    override func rightMouseDragged(with event: NSEvent) { mouseMoved(with: event) }
    override func otherMouseDragged(with event: NSEvent) { mouseMoved(with: event) }

    private func button(_ event: NSEvent, _ button: MouseButton, _ action: ButtonAction) {
        publish(capture.button(button, action: action, at: point(event)))
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
        // NSEvent deltas already reflect the user's natural-scroll preference.
        // Precise device: 1 point -> 1 wheel unit; discrete device: 1 notch -> 120.
        // Windows horizontal-positive means right, unlike AppKit's positive-left.
        let scale = event.hasPreciseScrollingDeltas ? 1.0 : 120.0
        publish(capture.wheel(horizontal: -Double(event.scrollingDeltaX) * scale,
                              vertical: Double(event.scrollingDeltaY) * scale,
                              at: point(event)))
    }

    override func draw(_ dirtyRect: NSRect) {
        NSColor.black.setFill()
        bounds.fill()
        let scale = min(bounds.width / 1280, bounds.height / 720)
        let image = NSRect(x: (bounds.width - 1280 * scale) / 2,
                           y: (bounds.height - 720 * scale) / 2,
                           width: 1280 * scale, height: 720 * scale)
        NSColor(calibratedWhite: 0.18, alpha: 1).setFill()
        image.fill()
        let label = capture.isActive
            ? "模拟画布 1280 × 720\n可测试键鼠；Esc / 切换窗口即停止"
            : "点击上方「开始本地模拟」\n此处不显示远程画面"
        (label as NSString).draw(at: NSPoint(x: image.minX + 20, y: image.minY + 20),
            withAttributes: [.font: NSFont.systemFont(ofSize: 18), .foregroundColor: NSColor.white])
    }
}

final class PreviewDelegate: NSObject, NSApplicationDelegate, NSWindowDelegate {
    private var window: NSWindow!
    private let canvas = InputPreviewCanvas()
    private let status = NSTextField(labelWithString: "已停止 · 事件 0 · 释放 0 · 持有 0")

    func applicationDidFinishLaunching(_ notification: Notification) {
        NSApp.setActivationPolicy(.regular)
        window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 960, height: 650),
            styleMask: [.titled, .closable, .miniaturizable, .resizable],
            backing: .buffered, defer: false)
        window.title = "输入预览 · 仅本地模拟"
        window.minSize = NSSize(width: 680, height: 480)
        window.delegate = self
        window.acceptsMouseMovedEvents = true
        window.isReleasedWhenClosed = false
        let root = NSView()
        window.contentView = root
        let start = NSButton(title: "开始本地模拟", target: self, action: #selector(startPreview))
        let stop = NSButton(title: "停止并释放", target: self, action: #selector(stopPreview))
        let note = NSTextField(labelWithString: "只统计事件，不记录按键内容；没有网络连接，也不会控制 Windows。")
        let views: [NSView] = [start, stop, note, status, canvas]
        for view in views {
            view.translatesAutoresizingMaskIntoConstraints = false
            root.addSubview(view)
        }
        NSLayoutConstraint.activate([
            start.leadingAnchor.constraint(equalTo: root.leadingAnchor, constant: 16),
            start.topAnchor.constraint(equalTo: root.topAnchor, constant: 16),
            stop.leadingAnchor.constraint(equalTo: start.trailingAnchor, constant: 12),
            stop.centerYAnchor.constraint(equalTo: start.centerYAnchor),
            note.leadingAnchor.constraint(equalTo: start.leadingAnchor),
            note.topAnchor.constraint(equalTo: start.bottomAnchor, constant: 12),
            status.leadingAnchor.constraint(equalTo: start.leadingAnchor),
            status.topAnchor.constraint(equalTo: note.bottomAnchor, constant: 10),
            canvas.leadingAnchor.constraint(equalTo: root.leadingAnchor, constant: 16),
            canvas.trailingAnchor.constraint(equalTo: root.trailingAnchor, constant: -16),
            canvas.topAnchor.constraint(equalTo: status.bottomAnchor, constant: 12),
            canvas.bottomAnchor.constraint(equalTo: root.bottomAnchor, constant: -16)
        ])
        canvas.onStatus = { [weak self] text in self?.status.stringValue = text }
        window.center()
        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }
    @objc private func startPreview() { canvas.start() }
    @objc private func stopPreview() { canvas.stop() }
    func windowDidResignKey(_ notification: Notification) { canvas.stop() }
    func windowWillClose(_ notification: Notification) { canvas.stop() }
    func applicationWillResignActive(_ notification: Notification) { canvas.stop() }
    func applicationWillTerminate(_ notification: Notification) { canvas.stop() }
    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { true }
}

let application = NSApplication.shared
let delegate = PreviewDelegate()
application.delegate = delegate
application.run()
