using System.ComponentModel;
using System.Runtime.InteropServices;
using System.Text;
using System.Windows;
using System.Windows.Interop;
using RemoteProtocol;

namespace RemoteAgent;

public partial class MainWindow : Window
{
    private const int WmHotkey = 0x0312;
    private const int WmKeyDown = 0x0100;
    private const int WmSystemKeyDown = 0x0104;
    private const int WhKeyboardLowLevel = 13;
    private const int EmergencyHotkeyId = 0x5052;
    private const uint ModAlt = 0x0001;
    private const uint ModControl = 0x0002;
    private const uint ModNoRepeat = 0x4000;
    private const uint VirtualKeyEscape = 0x1b;
    private const int WmKeyUp = 0x0101;
    private const int WmSystemKeyUp = 0x0105;
    private readonly PhysicalEmergencyChord physicalChord = new();
    private CancellationTokenSource? sharing;
    private HwndSource? windowSource;
    private LowLevelKeyboardProcedure? keyboardProcedure;
    private nint keyboardHook;
    private bool registeredWindowHotkey;
    private bool emergencyHotkeyAvailable;
    private bool emergencyStopRequested;
    private bool observedKeyboardCallback;

    public MainWindow()
    {
        EmergencyDiagnostics.Write("constructor before InitializeComponent");
        InitializeComponent();
        EmergencyDiagnostics.Write($"constructor after InitializeComponent source={PresentationSource.FromVisual(this) is not null}");
        SourceInitialized += Window_SourceInitialized;
        var remoteSession = GetSystemMetrics(0x1000) != 0; // SM_REMOTESESSION
        EmergencyDiagnostics.Write($"Session remote={remoteSession}");
        if (remoteSession)
            SessionWarning.Text = "当前通过 Windows 远程桌面运行：Ctrl + Alt + Esc 可能被远程桌面截获。请使用“停止共享”；本机紧急停止需在 Windows 实体键盘验证。";
    }

    private void Window_SourceInitialized(object? sender, EventArgs e)
    {
        EmergencyDiagnostics.Write("SourceInitialized");
        var handle = new WindowInteropHelper(this).Handle;
        windowSource = HwndSource.FromHwnd(handle);
        windowSource?.AddHook(WindowMessageHook);
        registeredWindowHotkey = windowSource is not null && RegisterHotKey(handle, EmergencyHotkeyId,
            ModControl | ModAlt | ModNoRepeat, VirtualKeyEscape);
        EmergencyDiagnostics.Write($"RegisterHotKey success={registeredWindowHotkey} error={(registeredWindowHotkey ? 0 : Marshal.GetLastWin32Error())}");
        keyboardProcedure = KeyboardHookCallback;
        keyboardHook = SetWindowsHookEx(WhKeyboardLowLevel, keyboardProcedure, GetModuleHandle(null), 0);
        EmergencyDiagnostics.Write($"SetWindowsHookEx success={keyboardHook != 0} error={(keyboardHook != 0 ? 0 : Marshal.GetLastWin32Error())}");
        // WM_HOTKEY has no injection provenance; only the physical hook authorizes stopping.
        emergencyHotkeyAvailable = keyboardHook != 0;
        if (!emergencyHotkeyAvailable)
            StatusText.Text = "未共享 · Ctrl + Alt + Esc 紧急停止快捷键不可用，真实控制已禁用";
    }

    private nint WindowMessageHook(nint hwnd, int message, nint wParam, nint lParam, ref bool handled)
    {
        if (message == WmHotkey && wParam == (nint)EmergencyHotkeyId)
        {
            EmergencyDiagnostics.Write("WM_HOTKEY received");
            handled = true;
            // Diagnostic only: SendInput can also generate WM_HOTKEY.
        }
        return 0;
    }

    private nint KeyboardHookCallback(int code, nint message, nint dataPointer)
    {
        if (!observedKeyboardCallback) { observedKeyboardCallback = true; EmergencyDiagnostics.Write("First keyboard callback received"); }
        if (code >= 0 && (message == (nint)WmKeyDown || message == (nint)WmSystemKeyDown ||
            message == (nint)WmKeyUp || message == (nint)WmSystemKeyUp))
        {
            var data = Marshal.PtrToStructure<LowLevelKeyboardData>(dataPointer);
            var down = message == (nint)WmKeyDown || message == (nint)WmSystemKeyDown;
            var matched = physicalChord.Process(data.VirtualKey, data.Flags, down);
            if (data.VirtualKey == VirtualKeyEscape)
                EmergencyDiagnostics.Write($"Escape down={down} injected={(data.Flags & 0x12) != 0} ctrl={physicalChord.ControlDown} alt={physicalChord.AltDown} matched={matched}");
            if (matched)
                _ = Dispatcher.BeginInvoke(new Action(RequestEmergencyStop));
        }
        return CallNextHookEx(keyboardHook, code, message, dataPointer);
    }

    private void RequestEmergencyStop()
    {
        EmergencyDiagnostics.Write($"RequestEmergencyStop sharing={sharing is not null} alreadyRequested={emergencyStopRequested}");
        if (sharing is null || emergencyStopRequested) return;
        emergencyStopRequested = true;
        StatusText.Text = "已触发本机紧急停止，正在释放输入并结束共享";
        sharing.Cancel();
        EmergencyDiagnostics.Write("Emergency cancellation requested");
    }

    private async void Start_Click(object sender, RoutedEventArgs e)
    {
        if (sharing is not null) return;
        var allowControl = ControlConsent.IsChecked == true;
        if (allowControl && !emergencyHotkeyAvailable)
        {
            MessageBox.Show(this, "无法注册 Ctrl + Alt + Esc 本机紧急停止快捷键，因此不会启用真实控制。\n\n仍可取消控制许可后开始只读共享。",
                "真实控制已阻止", MessageBoxButton.OK, MessageBoxImage.Error);
            ControlConsent.IsChecked = false;
            ControlText.Text = "未允许：本次共享只发送画面";
            return;
        }
        if (allowControl && MessageBox.Show(this,
            "启用后，已认证的 Mac 可以真实移动鼠标、点击和输入按键。\n\n" +
            "许可仅限这一次共享；可随时点击“停止共享”立即结束。是否继续？",
            "确认允许本次远程控制", MessageBoxButton.YesNo, MessageBoxImage.Warning,
            MessageBoxResult.No) != MessageBoxResult.Yes)
        {
            ControlConsent.IsChecked = false;
            ControlText.Text = "未允许：本次共享只发送画面";
            return;
        }
        using var lifetime = new CancellationTokenSource();
        sharing = lifetime;
        emergencyStopRequested = false;
        var quality = QualitySelector.SelectedIndex switch { 0 => 40, 2 => 85, _ => 70 };
        QualitySelector.IsEnabled = false;
        ControlConsent.IsEnabled = false;
        HidePairingKey();
        PairingKeyButton.IsEnabled = false;
        StartButton.IsEnabled = false;
        StopButton.IsEnabled = true;
        MetricsText.Text = "等待首帧统计";
        try
        {
            StatusText.Text = "正在检查 Tailscale 和配对凭据";
            var endpoints = await TailscaleEndpoints.DiscoverAsync(lifetime.Token);
            using var certificate = AgentCertificateStore.LoadOrCreate();
            using var credentials = AgentCredentialStore.LoadOrCreate();
            FingerprintText.Text = CertificateFingerprint.FromCertificateDer(certificate.RawData).Hexadecimal;
            ControlText.Text = allowControl
                ? "本次已允许真实控制 · 等待已认证的 Mac"
                : "本次只读 · 不接受输入";
            StatusText.Text = "等待已配对的 Mac 连接 · 端口 47475";
            InputSimulationOptions? inputSession = allowControl
                ? new InputSimulationOptions(
                    () => new SessionNativeInputSink(snapshot => Dispatcher.BeginInvoke(() =>
                        ControlText.Text = $"远程控制中 · 事件 {snapshot.Events} · 释放 {snapshot.Releases} · 持有 {snapshot.Held}")),
                    LocalControlAllowed: true,
                    ReadClipboardText: ReadClipboardTextAsync,
                    WriteClipboardText: WriteClipboardTextAsync)
                : null;
            await Task.Run(() => TlsProbeServer.RunContinuousAsync(endpoints.Local, endpoints.Peer, 47475,
                certificate, credentials.DeviceKey, credentials.AgentIdentifier, lifetime.Token,
                createJpegSource: () => new PrimaryScreenCapture(quality),
                reportStatus: text => Dispatcher.Invoke(() => StatusText.Text = text),
                reportMetrics: metrics => Dispatcher.Invoke(() => MetricsText.Text =
                    $"帧 {metrics.FrameNumber} · 采集+编码 {metrics.CaptureMilliseconds:F0} ms · 网络写入 {metrics.SendMilliseconds:F0} ms · 本帧总耗时 {metrics.FrameMilliseconds:F0} ms · 每帧 {metrics.JpegBytes / 1024.0:F1} KiB"),
                inputSession: inputSession), lifetime.Token);
        }
        catch (JpegTransferTimeoutException)
        {
            StatusText.Text = "共享结束：发送画面超过 10 秒（网络发送超时）";
        }
        catch (OperationCanceledException) when (lifetime.IsCancellationRequested)
        {
            StatusText.Text = emergencyStopRequested
                ? "共享已由本机紧急停止（Ctrl + Alt + Esc）"
                : "共享已由本机停止";
        }
        catch (OperationCanceledException) { StatusText.Text = "共享结束：等待连接或应用认证超时"; }
        catch (System.IO.EndOfStreamException) { StatusText.Text = "共享结束：Mac 已关闭连接"; }
        catch (Exception ex)
        {
            // Only fixed type labels: never echo raw remote data or credentials.
            StatusText.Text = "共享已结束（" + ex.GetType().Name + "）。请确认两端 Tailscale 在线、端口空闲且已配对。";
        }
        finally
        {
            if (MetricsText.Text.StartsWith("帧 ", StringComparison.Ordinal))
                MetricsText.Text = "最后成功发送：" + MetricsText.Text;
            EmergencyDiagnostics.Write($"Sharing cleanup complete emergency={emergencyStopRequested}");
            sharing = null;
            emergencyStopRequested = false;
            QualitySelector.IsEnabled = true;
            ControlConsent.IsChecked = false;
            ControlConsent.IsEnabled = true;
            PairingKeyButton.IsEnabled = true;
            ControlText.Text = "未允许：下一次共享默认为只读";
            StartButton.IsEnabled = true;
            StopButton.IsEnabled = false;
        }
    }
    private void ControlConsent_Click(object sender, RoutedEventArgs e)
    {
        if (sharing is null)
            ControlText.Text = ControlConsent.IsChecked == true
                ? "已选择：开始共享时还需确认，之后会真实操作 Windows"
                : "未允许：本次共享只发送画面";
    }

    private async Task<ClipboardTextPayload> ReadClipboardTextAsync(CancellationToken cancellationToken)
    {
        // SendInput is asynchronous. Give the foreground application a brief chance
        // to process Ctrl+C before reading its clipboard result on the WPF STA thread.
        await Task.Delay(120, cancellationToken).ConfigureAwait(false);
        for (var attempt = 0; attempt < 4; attempt++)
        {
            cancellationToken.ThrowIfCancellationRequested();
            var result = await Dispatcher.InvokeAsync(() =>
            {
                try
                {
                    if (!Clipboard.ContainsText(TextDataFormat.UnicodeText))
                        return (Busy: false, Payload: new ClipboardTextPayload(
                            ClipboardTextStatus.Unavailable, string.Empty));
                    var text = Clipboard.GetText(TextDataFormat.UnicodeText);
                    var payload = Encoding.UTF8.GetByteCount(text) <= ClipboardTextPayload.MaximumTextBytes
                        ? new ClipboardTextPayload(ClipboardTextStatus.Success, text)
                        : new ClipboardTextPayload(ClipboardTextStatus.TooLarge, string.Empty);
                    return (Busy: false, Payload: payload);
                }
                catch (ExternalException)
                {
                    return (Busy: true, Payload: new ClipboardTextPayload(
                        ClipboardTextStatus.Unavailable, string.Empty));
                }
            });
            if (!result.Busy) return result.Payload;
            await Task.Delay(40, cancellationToken).ConfigureAwait(false);
        }
        return new ClipboardTextPayload(ClipboardTextStatus.Unavailable, string.Empty);
    }

    private async Task<ClipboardTextStatus> WriteClipboardTextAsync(
        string text, CancellationToken cancellationToken)
    {
        if (Encoding.UTF8.GetByteCount(text) > ClipboardTextPayload.MaximumTextBytes)
            return ClipboardTextStatus.TooLarge;
        for (var attempt = 0; attempt < 4; attempt++)
        {
            cancellationToken.ThrowIfCancellationRequested();
            var written = await Dispatcher.InvokeAsync(() =>
            {
                try { Clipboard.SetText(text, TextDataFormat.UnicodeText); return true; }
                catch (ExternalException) { return false; }
            });
            if (written) return ClipboardTextStatus.Success;
            await Task.Delay(40, cancellationToken).ConfigureAwait(false);
        }
        return ClipboardTextStatus.Unavailable;
    }

    private void PairingKey_Click(object sender, RoutedEventArgs e)
    {
        if (PairingKeyText.Visibility == Visibility.Visible)
        {
            HidePairingKey();
            return;
        }
        using var credentials = AgentCredentialStore.LoadOrCreate();
        PairingKeyText.Text = Convert.ToBase64String(credentials.DeviceKey);
        PairingKeyText.Visibility = Visibility.Visible;
        PairingKeyWarning.Visibility = Visibility.Visible;
        PairingKeyButton.Content = "隐藏配对密钥";
    }

    private void HidePairingKey()
    {
        PairingKeyText.Text = string.Empty;
        PairingKeyText.Visibility = Visibility.Collapsed;
        PairingKeyWarning.Visibility = Visibility.Collapsed;
        PairingKeyButton.Content = "显示配对密钥";
    }

    private void Stop_Click(object sender, RoutedEventArgs e) => sharing?.Cancel();
    private void Window_Closing(object? sender, CancelEventArgs e)
    {
        HidePairingKey();
        sharing?.Cancel();
        if (windowSource is not null)
        {
            windowSource.RemoveHook(WindowMessageHook);
            if (registeredWindowHotkey)
                _ = UnregisterHotKey(windowSource.Handle, EmergencyHotkeyId);
        }
        registeredWindowHotkey = false;
        if (keyboardHook != 0)
        {
            _ = UnhookWindowsHookEx(keyboardHook);
            keyboardHook = 0;
        }
        keyboardProcedure = null;
        emergencyHotkeyAvailable = false;
    }

    [StructLayout(LayoutKind.Sequential)]
    private readonly struct LowLevelKeyboardData
    {
        public readonly uint VirtualKey;
        public readonly uint ScanCode;
        public readonly uint Flags;
        public readonly uint Time;
        public readonly nuint ExtraInformation;
    }

    private delegate nint LowLevelKeyboardProcedure(int code, nint message, nint dataPointer);

    [DllImport("user32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool RegisterHotKey(nint window, int id, uint modifiers, uint virtualKey);

    [DllImport("user32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool UnregisterHotKey(nint window, int id);

    [DllImport("user32.dll", SetLastError = true)]
    private static extern nint SetWindowsHookEx(int hookId, LowLevelKeyboardProcedure procedure,
        nint module, uint threadId);

    [DllImport("user32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool UnhookWindowsHookEx(nint hook);

    [DllImport("user32.dll")]
    private static extern nint CallNextHookEx(nint hook, int code, nint message, nint dataPointer);

    [DllImport("user32.dll")]
    private static extern int GetSystemMetrics(int index);

    [DllImport("kernel32.dll", CharSet = CharSet.Unicode)]
    private static extern nint GetModuleHandle(string? moduleName);
}
