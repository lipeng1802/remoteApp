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
    @Published var host = "100.73.4.118"
    @Published var status = "输入已配对的 Windows Tailscale 地址"
    @Published var connected = false
    @Published var image: CGImage?
    @Published var detail = "默认只读"
    @Published var requestControl = false
    @Published var controlReady = false
    @Published var controlCapturing = false
    @Published var pairingKey = ""
    @Published var pairingStatus = "首次使用请保存 Windows 显示的配对密钥"
    @Published var screenInfo: ScreenInfoPayload?
    // Development DMGs are ad-hoc signed and have no provisioned keychain
    // access group. Keep app-owned items separate from legacy CLI-created
    // items; switch these v2 services to the data-protection keychain when
    // Developer ID signing and its entitlements are available.
    private let fingerprintStore = KeychainTrustedFingerprintStore(
        service: FingerprintKeychainService.application
    )
    private let deviceKeyStore = KeychainDeviceKeyStore(
        service: DeviceKeychainService.application
    )
    private lazy var readOnlyClient = TLSControllerClient(trustStore: fingerprintStore)
    private var duplexClient: TLSInputSimulationClient?
    private var stopConnection: (() -> Void)?
    private var generation = UUID()
    private var latest = LatestValue<ReceivedImage>()
    private var frameRate = FrameRateMeter()
    private weak var inputCanvas: RemoteInputCanvas?

    func connect() {
        guard !connected else { return }
        let address = host.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !address.isEmpty else { status = "请输入 Windows Tailscale 地址"; return }
        let key: Data
        do {
            guard let stored = try deviceKeyStore.loadKey(for: address) else {
                status = "尚未配对：请在上方输入 Windows 显示的配对密钥并保存"
                return
            }
            key = stored
        } catch { status = "无法读取 Keychain 密钥"; return }
        let fingerprint: CertificateFingerprint?
        do { fingerprint = try fingerprintStore.loadFingerprint(for: address) }
        catch { status = "无法读取已信任的证书指纹"; return }
        if requestControl && fingerprint == nil {
            status = "远程控制要求已有证书指纹；请先取消控制选项并完成一次只读连接"
            return
        }
        connected = true
        image = nil
        screenInfo = nil
        controlReady = false
        controlCapturing = false
        detail = requestControl ? "远程控制 · 等待认证" : "只读模式 · 等待首帧"
        frameRate = FrameRateMeter()
        generation = UUID()
        let current = generation
        let slot = LatestValue<ReceivedImage>()
        latest = slot
        var count = 0 // Only touched by the client's serial network queue.
        if requestControl, let fingerprint {
            do {
                let connection = try TLSInputSimulationClient(tailscaleHost: address, port: 47475,
                    expectedFingerprint: fingerprint, deviceKey: key, localControlAllowed: true,
                    connectionTimeout: 180,
                    onJpegFrame: { screen, jpeg in
                        guard let decoded = JpegImageDecoder.decode(jpeg) else { return }
                        count += 1
                        slot.replace(ReceivedImage(image: decoded, screen: screen, count: count))
                    },
                    onAuthenticated: { [weak self] in
                        DispatchQueue.main.async {
                            guard let self, self.generation == current, self.connected else { return }
                            self.controlReady = true
                            self.status = "已认证 · Windows 已允许远程控制；点击开始控制"
                        }
                    },
                    completion: { [weak self] result in
                        DispatchQueue.main.async {
                            guard let self, self.generation == current else { return }
                            switch result {
                            case .success: self.complete(status: "远程控制会话已安全结束")
                            case .failure(.cancelled): self.complete(status: "已断开")
                            case let .failure(error): self.complete(status: "远程控制连接已结束（\(error)），可手动重试")
                            }
                        }
                    })
                duplexClient = connection
                stopConnection = { connection.cancel() }
                status = "正在连接 TLS 远程控制会话"
                connection.start()
            } catch {
                complete(status: "无法启动远程控制连接（\(error)）")
            }
            return
        }
        stopConnection = readOnlyClient.runProbe(host: address, port: 47475, deviceIdentifier: address,
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
                    switch result {
                    case .success: self.complete(status: "会话已结束")
                    case .failure(.cancelled): self.complete(status: "已断开")
                    case let .failure(error): self.complete(status: "连接已结束（\(error)），可手动重试")
                    }
                }
            })
    }

    func savePairingKey() {
        guard !connected else { return }
        let address = host.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !address.isEmpty else {
            pairingStatus = "请先输入 Windows Tailscale 地址"
            return
        }
        do {
            let key = try PairingKeyParser.parseBase64(pairingKey)
            try deviceKeyStore.saveKey(key, for: address)
            pairingKey = ""
            pairingStatus = "配对密钥已安全保存；首次连接还需核对一次证书指纹"
        } catch PairingKeyParserError.invalidEncoding {
            pairingStatus = "配对密钥不是有效的 Base64"
        } catch PairingKeyParserError.invalidLength {
            pairingStatus = "配对密钥长度不正确"
        } catch {
            pairingStatus = "无法保存配对密钥"
        }
    }

    private func complete(status: String) {
        inputCanvas?.abandon()
        connected = false
        image = nil
        screenInfo = nil
        latest.clear()
        stopConnection = nil
        duplexClient = nil
        controlReady = false
        controlCapturing = false
        detail = requestControl ? "远程控制" : "只读模式"
        self.status = status
    }

    func disconnect() {
        if let duplexClient, controlReady {
            let releases = inputCanvas?.stopForDisconnect() ?? []
            controlCapturing = false
            controlReady = false
            status = "正在释放输入并断开"
            duplexClient.finish(releases: releases)
            return
        }
        generation = UUID()
        inputCanvas?.abandon()
        stopConnection?(); stopConnection = nil
        duplexClient = nil
        latest.clear(); image = nil; screenInfo = nil; connected = false
        controlReady = false; controlCapturing = false
        status = "已断开"; detail = requestControl ? "远程控制" : "只读模式"
    }

    func registerInputCanvas(_ canvas: RemoteInputCanvas) { inputCanvas = canvas }

    func startControl() {
        guard connected, controlReady, screenInfo != nil else { return }
        guard inputCanvas?.start() == true else {
            status = "无法取得画面键盘焦点；请先激活此窗口后重试"
            return
        }
        controlCapturing = true
        status = "正在远程控制 Windows · Esc 可立即停止"
    }

    func stopControl() { inputCanvas?.pause() }

    func submitInputs(_ inputs: [CapturedInput]) -> Bool {
        guard connected, controlReady, let duplexClient else { return false }
        return duplexClient.submit(inputs)
    }

    func inputPaused() {
        guard controlCapturing else { return }
        controlCapturing = false
        if connected { status = "控制已暂停并释放；再次开始需手动点击" }
    }

    func inputFocusSuspended() {
        guard connected, controlReady, controlCapturing else { return }
        status = "窗口已失焦 · 输入已安全释放，返回窗口后自动恢复控制"
    }

    func inputFocusResumed() {
        guard connected, controlReady, controlCapturing else { return }
        status = "正在远程控制 Windows · Esc 可立即停止"
    }

    func presentLatest() {
        guard connected, let next = latest.take() else { return }
        image = next.image
        screenInfo = next.screen
        if !controlCapturing {
            status = controlReady ? "正在查看 Windows 主屏 · 远程控制已就绪" : "正在查看 Windows 主屏 · 只读"
        }
        if let fps = frameRate.update(totalFrames: next.count, now: ProcessInfo.processInfo.systemUptime) {
            let mode = controlReady ? "远程控制" : "只读"
            detail = "\(mode) · \(next.image.width) × \(next.image.height) · \(String(format: "%.1f", fps)) FPS · 主屏 \(next.screen.width) × \(next.screen.height)"

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
            HStack {
                SecureField("Windows 配对密钥（Base64）", text: $model.pairingKey)
                    .textFieldStyle(.roundedBorder)
                    .disabled(model.connected)
                Button("保存配对") { model.savePairingKey() }
                    .disabled(model.connected || model.pairingKey.isEmpty)
            }
            HStack {
                Text(model.pairingStatus).foregroundStyle(.secondary)
                Spacer()
            }
            .font(.caption)
            HStack {
                Toggle("请求远程控制（会真实操作 Windows）", isOn: $model.requestControl)
                    .disabled(model.connected)
                Spacer()
                Button(model.controlCapturing ? "停止控制" : "开始控制") {
                    if model.controlCapturing { model.stopControl() } else { model.startControl() }
                }
                .disabled(!model.controlReady || model.screenInfo == nil)
            }
            HStack { Text(model.status); Spacer(); Text(model.detail).foregroundStyle(.secondary) }
                .font(.callout)
            ZStack {
                Color.black
                if let image = model.image {
                    Image(decorative: image, scale: 1).resizable().aspectRatio(contentMode: .fit)
                } else { Text("等待已认证的 Windows 画面").foregroundStyle(.gray) }
                RemoteInputOverlay(screen: model.screenInfo, enabled: model.controlCapturing,
                    register: model.registerInputCanvas,
                    submit: model.submitInputs,
                    paused: model.inputPaused,
                    focusSuspended: model.inputFocusSuspended,
                    focusResumed: model.inputFocusResumed)
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
