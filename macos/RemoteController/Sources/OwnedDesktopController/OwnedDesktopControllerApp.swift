import AppKit
import Combine
import SwiftUI
import RemoteProtocol
import RemoteCanvas

// Dedicated owned controller. No argv mode, host, Keychain, TOFU or clipboard.
// stdin/stdout are exclusively the pinned owner's inherited private pipes.
@main @MainActor
struct OwnedDesktopControllerApp: App {
    @NSApplicationDelegateAdaptor(OwnedApplicationDelegate.self) private var delegate
    var body: some Scene {
        Window("RemoteApp", id: "owned-desktop") { OwnedDesktopView(model: OwnedViewerModel.shared) }
            .defaultSize(width: 1060, height: 720)
    }
}

@MainActor
private final class OwnedApplicationDelegate: NSObject, NSApplicationDelegate {
    func applicationDidFinishLaunching(_ notification: Notification) {
        guard CommandLine.arguments.count == 1 else { exit(1) }
        NSApp.setActivationPolicy(.regular)
        OwnedViewerModel.shared.start()
    }
    func applicationShouldTerminate(_ sender: NSApplication) -> NSApplication.TerminateReply {
        if OwnedViewerModel.shared.finished { return .terminateNow }
        OwnedViewerModel.shared.requestStop()
        return .terminateCancel
    }
}

// SwiftUI can create its window after didFinishLaunching. Attach to the actual
// content window instead of relying on NSApp.windows.first at launch time.
@MainActor
private final class OwnedWindowDelegate: NSObject, NSWindowDelegate {
    static let shared = OwnedWindowDelegate()
    func windowShouldClose(_ sender: NSWindow) -> Bool {
        OwnedViewerModel.shared.requestStop()
        return false
    }
}

private struct OwnedWindowAnchor: NSViewRepresentable {
    final class Anchor: NSView {
        override func viewDidMoveToWindow() {
            super.viewDidMoveToWindow()
            window?.delegate = OwnedWindowDelegate.shared
        }
    }
    func makeNSView(context: Context) -> Anchor { Anchor() }
    func updateNSView(_ view: Anchor, context: Context) {}
}

private struct OwnedFrame { let screen: ScreenInfoPayload; let image: CGImage }
private final class OwnedFrameSlot {
    private let lock = NSLock()
    private var latest: OwnedFrame?
    private var total: Int64 = 0
    private var sealed = false
    func put(_ screen: ScreenInfoPayload, _ image: CGImage) {
        lock.lock(); defer { lock.unlock() }
        guard !sealed else { return }
        latest = OwnedFrame(screen: screen, image: image)
        total = min(total + 1, 9_000_000_000_000_000)
    }
    func snapshot() -> (OwnedFrame?, Int64) {
        lock.lock(); defer { lock.unlock() }
        let frame = latest; latest = nil
        return (frame, total)
    }
    func seal() { lock.lock(); sealed = true; latest = nil; lock.unlock() }
}

private final class OwnedPipe {
    private let writeLock = NSLock()
    func write(_ data: Data) {
        writeLock.lock(); defer { writeLock.unlock() }
        FileHandle.standardOutput.write(data + Data([10]))
    }
    func write(_ value: [String: Any]) {
        guard let data = try? JSONSerialization.data(withJSONObject: value, options: [.sortedKeys, .withoutEscapingSlashes]) else { return }
        write(data)
    }
    func start(frame: @escaping (Data) -> Void, ended: @escaping () -> Void) {
        DispatchQueue.global(qos: .utility).async {
            defer { ended() }
            do {
                var line = Data()
                while let byte = try FileHandle.standardInput.read(upToCount: 1), !byte.isEmpty {
                    if byte[0] == 10 {
                        guard !line.isEmpty else { return }
                        frame(line); line.removeAll(keepingCapacity: true)
                    } else {
                        guard line.count < 8191 else { return }
                        line.append(byte)
                    }
                }
            } catch { /* Parent sees only the normal fixed lifecycle response. */ }
        }
    }
}

@MainActor
private final class OwnedViewerModel: ObservableObject {
    static let shared = OwnedViewerModel()
    @Published var status = "等待连接"
    @Published var permission = ""
    @Published var targetCode = ""
    @Published var verificationCode = ""
    @Published var enteringConnection = false
    @Published var preview: OwnedControllerPreparation.RequestPreview?
    @Published var image: CGImage?
    @Published var screen: ScreenInfoPayload?
    @Published var frames: Int64 = 0
    @Published var controlReady = false
    @Published var capturing = false
    @Published private(set) var stopRequested = false
    private(set) var finished = false
    private let pipe = OwnedPipe()
    private let slot = OwnedFrameSlot()
    private var preparation: OwnedControllerPreparation?
    private var client: TLSInputSimulationClient?
    private var stopReadOnly: (() -> Void)?
    private weak var canvas: RemoteInputCanvas?
    private var started = false, activated = false, stopping = false
    private var readerEnded = false, clientEnded = true, failed = false, closedWritten = false
    private var submitted: Int64 = 0

    func start() {
        guard !started else { return }; started = true
        pipe.start(frame: { [weak self] data in DispatchQueue.main.async { self?.receive(data) } },
                   ended: { [weak self] in DispatchQueue.main.async { self?.parentEnded() } })
    }
    private func receive(_ data: Data) {
        guard !finished else { return }
        if data == Data("{\"Op\":\"stop\"}".utf8) { beginStop(); return }
        if data == Data("{\"Op\":\"desktop_state\"}".utf8) {
            refresh()
            pipe.write(["status": "desktop_state", "frames": frames, "inputs": submitted,
                        "stop_requested": stopRequested, "ended": activated && clientEnded, "failed": failed])
            return
        }
        guard !stopping, !stopRequested else { protocolFailure(); return }
        do {
            if preparation == nil {
                let p = try OwnedControllerPreparation(privatePreparation: data)
                preparation = p; permission = p.permission
                status = "等待发起连接"
                pipe.write(p.preparedReply())
            } else if data == Data("{\"Op\":\"connect_intent\"}".utf8) {
                try preparation!.beginConnectIntent(privateCommand: data)
                enteringConnection = true; status = "请输入 Windows 设备码和验证码"
                NSApp.activate(ignoringOtherApps: true)
            } else if let object = try JSONSerialization.jsonObject(with: data) as? [String: Any], object["Op"] as? String == "consent" {
                preview = try preparation!.beginRequest(privateRequest: data)
                status = permission == "desktop-control" ? "确认连接并控制 Windows 键鼠" : "确认查看 Windows 画面"
                NSApp.activate(ignoringOtherApps: true)
            } else {
                let handoff = try preparation!.acceptBootstrap(privateBootstrap: data)
                try activate(handoff)
            }
        } catch { protocolFailure() }
    }
    func connect() {
        guard enteringConnection, !stopRequested, let p = preparation else { return }
        do {
            let reply = try p.connectIntent(targetCode: targetCode, verificationCode: verificationCode)
            targetCode = ""; verificationCode = ""; enteringConnection = false
            status = "等待 Windows 确认"
            pipe.write(reply)
        } catch { status = "设备码须为12位小写十六进制，验证码须为8位数字" }
    }
    func decide(_ allowed: Bool) {
        guard preview != nil, let p = preparation, !stopping else { return }
        do {
            let reply = try p.decide(localUserAllowed: allowed)
            preview = nil
            status = allowed ? "等待已授权的连接" : "已拒绝连接"
            pipe.write(reply)
        } catch { preview = nil; protocolFailure() }
    }
    private func activate(_ handoff: OwnedDesktopHandoff) throws {
        guard !activated else { throw OwnedControllerPreparationError.invalidState }
        activated = true; clientEnded = false
        var launched = false
        defer { if !launched { clientEnded = true } }
        let slot = self.slot
        if permission == "desktop-control" {
            let c = try handoff.makeControlClient(localControlAllowed: true,
                onJpegFrame: { screen, bytes in
                    if let image = JpegImageDecoder.decode(bytes) { slot.put(screen, image) }
                }, onAuthenticated: { [weak self] in DispatchQueue.main.async {
                    guard let self, !self.stopRequested, !self.stopping else { return }
                    self.controlReady = true; self.status = "已连接 · 点击开始控制"
                } }, completion: { [weak self] _ in DispatchQueue.main.async { self?.networkEnded() } })
            client = c; launched = true; c.start()
        } else {
            stopReadOnly = try handoff.startReadOnly(onJpegFrame: { screen, image in slot.put(screen, image) },
                onStatus: { _ in }, completion: { [weak self] _ in DispatchQueue.main.async { self?.networkEnded() } })
            launched = true
        }
        status = "正在连接 Windows"
        pipe.write(["status": "ready", "Permission": permission])
    }
    func refresh() {
        let (frame, total) = slot.snapshot(); frames = total
        if !stopRequested, let frame { image = frame.image; screen = frame.screen }
    }
    func register(_ view: RemoteInputCanvas) { canvas = view }
    func startControl() {
        guard controlReady, !stopRequested, !stopping, canvas?.start() == true else { return }
        capturing = true; status = "正在控制 · Esc 可停止控制"
    }
    func pauseControl() { _ = canvas?.pause(); capturing = false }
    func submit(_ inputs: [CapturedInput]) -> Bool {
        guard controlReady, !stopRequested, !stopping, let client else { return false }
        guard inputs.allSatisfy({ value in
            switch value { case .clipboardRequest, .clipboardSet: return false; default: return true }
        }) else { return false }
        guard client.submit(inputs) else { return false }
        submitted = min(submitted + Int64(inputs.count), 9_000_000_000_000_000)
        return true
    }
    func requestStop() {
        guard !stopping, !stopRequested else { return }
        pauseControl() // release held input before suppressing new events
        if preview != nil { decide(false) }
        if enteringConnection, let reply = try? preparation?.cancelConnectIntent() { pipe.write(reply) }
        stopRequested = true; enteringConnection = false
        targetCode = ""; verificationCode = ""
        slot.seal(); image = nil; screen = nil
        status = "正在安全结束连接"
        // Keep the process/connection until the live owner observes the intent
        // and commands its joined quiesce. Never emit an unsolicited closed ACK.
    }
    private func protocolFailure() {
        guard !failed else { return }
        failed = true; preview = nil; enteringConnection = false
        preparation?.cancel(); requestStop()
        pipe.write(["status": "failed"])
    }
    private func beginStop() {
        guard !stopping else { return }
        // EOF/owner-stop can interrupt a private question. Do not manufacture
        // a late decision/cancellation reply into that already-faulted exchange.
        preview = nil; enteringConnection = false
        requestStop(); stopping = true
        preparation?.cancel(); preview = nil
        controlReady = false; capturing = false
        if let client { client.finish(releases: []) }
        else if let stopReadOnly { stopReadOnly(); self.stopReadOnly = nil }
        else { clientEnded = true }
        finishIfJoined()
    }
    private func parentEnded() {
        readerEnded = true
        beginStop(); finishIfJoined()
    }
    private func networkEnded() {
        clientEnded = true; controlReady = false; capturing = false
        canvas?.abandon()
        if !stopping { failed = true; stopRequested = true; slot.seal(); image = nil; screen = nil; status = "连接已结束" }
        finishIfJoined()
    }
    private func finishIfJoined() {
        guard stopping, readerEnded, clientEnded, !closedWritten else { return }
        closedWritten = true; finished = true
        client = nil; stopReadOnly = nil
        pipe.write(["status": "closed"])
        NSApp.terminate(nil)
    }
}

private struct OwnedDesktopView: View {
    @ObservedObject var model: OwnedViewerModel
    private let refresh = Timer.publish(every: 1.0 / 30, on: .main, in: .common).autoconnect()
    var body: some View {
        VStack(spacing: 12) {
            HStack {
                Text(model.permission == "desktop-control" ? "远程控制" : "只读连接").font(.headline)
                Spacer()
                Button("停止连接") { model.requestStop() }.disabled(model.stopRequested)
            }
            if model.enteringConnection {
                HStack {
                    TextField("Windows 设备码", text: $model.targetCode).textFieldStyle(.roundedBorder)
                    SecureField("本次验证码", text: $model.verificationCode).textFieldStyle(.roundedBorder)
                    Button("连接") { model.connect() }
                }
            }
            if let preview = model.preview {
                HStack {
                    Text("请求 \(String(preview.requestID.suffix(8))) · \(preview.permission == "desktop-control" ? "画面与键鼠" : "仅画面")")
                    Spacer()
                    Button("拒绝") { model.decide(false) }
                    Button("确认连接") { model.decide(true) }
                }
            }
            HStack {
                Text(model.status); Spacer(); Text("画面 \(model.frames)")
                Button(model.capturing ? "暂停控制" : "开始控制") {
                    if model.capturing { model.pauseControl() } else { model.startControl() }
                }.disabled(!model.controlReady || model.stopRequested || model.screen == nil)
            }.font(.callout)
            ZStack {
                Color.black
                if let image = model.image { Image(decorative: image, scale: 1).resizable().aspectRatio(contentMode: .fit) }
                else { Text("等待已认证的 Windows 画面").foregroundStyle(.gray) }
                RemoteInputOverlay(screen: model.screen, enabled: model.capturing, allowsClipboard: false,
                    register: model.register, submit: model.submit,
                    paused: { model.capturing = false }, focusSuspended: {}, focusResumed: {}, localClipboardIssue: { _ in })
            }.frame(maxWidth: .infinity, maxHeight: .infinity).clipped()
        }.padding(18).frame(minWidth: 760, minHeight: 520)
            .background(OwnedWindowAnchor().frame(width: 0, height: 0))
            .onReceive(refresh) { _ in model.refresh() }
    }
}
