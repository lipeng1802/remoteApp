import AppKit
import Combine
import SwiftUI
import RemoteProtocol

@main
struct RemoteControllerApp: App {
    @NSApplicationDelegateAdaptor(ViewerApplicationDelegate.self) private var applicationDelegate
    var body: some Scene {
        WindowGroup { ContentView() }
            .defaultSize(width: 1060, height: 720)
    }
}

@MainActor
private final class ViewerApplicationDelegate: NSObject, NSApplicationDelegate {
    func applicationDidFinishLaunching(_ notification: Notification) {
        // A SwiftPM executable is launched without an .app bundle. Explicitly
        // activate it so the visible window can receive keyboard input.
        guard Bundle.main.bundleURL.pathExtension != "app" else { return }
        NSApp.setActivationPolicy(.regular)
        NSApp.activate(ignoringOtherApps: true)
        NSApp.windows.first(where: { $0.canBecomeKey })?.makeKeyAndOrderFront(nil)
    }
}

private struct ReceivedImage {
    let image: CGImage
    let screen: ScreenInfoPayload
    let count: Int
}

@MainActor
private final class ViewerModel: ObservableObject {
    @Published var host = ""
    @Published var status = "输入已配对的 Windows Tailscale 地址"
    @Published var connected = false
    @Published var image: CGImage?
    @Published var detail = "只读模式"
    private let client = TLSControllerClient(trustStore: KeychainTrustedFingerprintStore())
    private var stopConnection: (() -> Void)?
    private var generation = UUID()
    private var latest = LatestValue<ReceivedImage>()
    private var frameRate = FrameRateMeter()


    func connect() {
        guard !connected else { return }
        let address = host.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !address.isEmpty else { status = "请输入 Windows Tailscale 地址"; return }
        let key: Data
        do {
            guard let stored = try KeychainDeviceKeyStore().loadKey(for: address) else {
                status = "尚未配对：请先用 TLSProbeClient --pair 为同一地址保存密钥"
                return
            }
            key = stored
        } catch { status = "无法读取 Keychain 密钥"; return }
        connected = true
        image = nil
        detail = "只读模式 · 等待首帧"
        frameRate = FrameRateMeter()
        generation = UUID()
        let current = generation
        let slot = LatestValue<ReceivedImage>()
        latest = slot
        var count = 0 // Only touched by the client's serial network queue.
        stopConnection = client.runProbe(host: address, port: 47475, deviceIdentifier: address,
            deviceKey: key, timeout: 180,
            onJpegFrame: { screen, image in
                count += 1
                slot.replace(ReceivedImage(image: image, screen: screen, count: count))
            },
            onStatus: { [weak self] message in
                DispatchQueue.main.async {
                    guard let self, self.generation == current else { return }
                    self.status = message
                }
            },
            approveFirstUse: { [weak self] fingerprint in
                DispatchQueue.main.sync {
                    guard let self, self.generation == current, self.connected else { return false }
                    let alert = NSAlert()
                    alert.messageText = "核对 Windows 证书指纹"
                    alert.informativeText = "请与 Windows 共享窗口的 SHA-256 完全核对后批准：\n\n" + fingerprint.hexadecimal
                    alert.addButton(withTitle: "匹配并信任")
                    alert.addButton(withTitle: "取消")
                    return alert.runModal() == .alertFirstButtonReturn
                }
            },
            completion: { [weak self] result in
                DispatchQueue.main.async {
                    guard let self, self.generation == current else { return }
                    self.connected = false
                    self.image = nil
                    self.latest.clear()
                    self.stopConnection = nil
                    switch result {
                    case .success: self.status = "会话已结束"
                    case .failure(.cancelled): self.status = "已断开"
                    case let .failure(error): self.status = "连接已结束（\(error)），可手动重试"
                    }
                }
            })
    }
    func disconnect() {
        generation = UUID()
        stopConnection?(); stopConnection = nil
        latest.clear(); image = nil; connected = false
        status = "已断开"; detail = "只读模式"
    }
    func presentLatest() {
        guard connected, let next = latest.take() else { return }
        image = next.image
        status = "正在查看 Windows 主屏 · 只读"
        if let fps = frameRate.update(totalFrames: next.count, now: ProcessInfo.processInfo.systemUptime) {
            detail = "\(next.image.width) × \(next.image.height) · \(String(format: "%.1f", fps)) FPS · 主屏 \(next.screen.width) × \(next.screen.height)"

        }
    }
}

@MainActor
private struct ContentView: View {
    @StateObject private var model = ViewerModel()
    @FocusState private var addressFocused: Bool
    private let refresh = Timer.publish(every: 1.0 / 30, on: .main, in: .common).autoconnect()
    var body: some View {
        VStack(spacing: 14) {
            HStack {
                TextField("Windows Tailscale 地址", text: $model.host)
                    .textFieldStyle(.roundedBorder).disabled(model.connected)
                    .focused($addressFocused)
                    .onSubmit { model.connect() }
                Button("连接") { model.connect() }.disabled(model.connected)
                Button("断开") { model.disconnect() }.disabled(!model.connected)
            }
            HStack { Text(model.status); Spacer(); Text(model.detail).foregroundStyle(.secondary) }
                .font(.callout)
            ZStack {
                Color.black
                if let image = model.image {
                    Image(decorative: image, scale: 1).resizable().aspectRatio(contentMode: .fit)
                } else { Text("等待已认证的 Windows 画面").foregroundStyle(.gray) }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .clipped()
        }
        .padding(18)
        .frame(minWidth: 760, minHeight: 520)
        .task {
            await Task.yield()
            addressFocused = !model.connected
        }
        .onChange(of: model.connected) { connected in
            addressFocused = !connected
        }
        .onReceive(refresh) { _ in model.presentLatest() }
        .onDisappear { model.disconnect() }
    }
}