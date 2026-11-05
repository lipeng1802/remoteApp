using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using System.Text;
using System.Text.Json;
using System.Windows;
using System.Windows.Controls;
using RemoteProtocol;

namespace RemoteAgent;

internal static class Program
{
    [STAThread]
    private static int Main(string[] args)
    {
        if (args.Length != 0) return 1;
        var app = new Application { ShutdownMode = ShutdownMode.OnExplicitShutdown };
        return app.Run(new OwnedWindow());
    }
}

// No legacy paired key, TOFU, clipboard, host picker or continuous reconnect.
// Owner pipes are private inherited handles, never a public PowerShell service.
internal sealed class OwnedWindow : Window
{
    private readonly TextBlock status = new() { Text = "等待设备准备", TextWrapping = TextWrapping.Wrap };
    private readonly TextBlock invite = new() { TextWrapping = TextWrapping.Wrap };
    private readonly TextBlock request = new() { TextWrapping = TextWrapping.Wrap };
    private readonly TextBlock metrics = new();
    private readonly CheckBox control = new() { Content = "明确允许本次 Mac 操作此 Windows 的键鼠", IsChecked = false };
    private readonly Button allow = new() { Content = "允许本次连接", IsEnabled = false };
    private readonly Button deny = new() { Content = "拒绝", IsEnabled = false };
    private readonly Button stopButton = new() { Content = "停止共享" };
    private readonly Stream input = Console.OpenStandardInput();
    private readonly Stream output = Console.OpenStandardOutput();
    private OwnedAgentPreparation? preparation;
    private PhysicalEmergencyListener? emergency;
    private Task<bool>? reader;
    private Task? server;
    private readonly CancellationTokenSource stop = new();
    private X509Certificate2? certificate;
    private byte[]? key;
    private bool pending, localControl, activated, proofUsed, stopRequested, ownerStopping, readerEnded, serverEnded = true, failed, closed;
    private long frames, events, releases;
    private long plannedStop;
    private int held;

    internal OwnedWindow()
    {
        Title = "RemoteApp · 本次共享"; Width = 630; Height = 490;
        var panel = new StackPanel { Margin = new Thickness(24) };
        panel.Children.Add(new TextBlock { Text = "共享 Windows 主屏", FontSize = 24 });
        panel.Children.Add(new TextBlock { Text = "认证及本机确认通过后才采集画面。Ctrl + Alt + Esc 可在物理键盘紧急停止。", TextWrapping = TextWrapping.Wrap });
        foreach (var element in new FrameworkElement[] { invite, request, control, allow, deny, stopButton, status, metrics })
        { element.Margin = new Thickness(0, 12, 0, 0); panel.Children.Add(element); }
        Content = panel;
        allow.Click += (_, _) => Decide(true);
        deny.Click += (_, _) => Decide(false);
        control.Checked += (_, _) => UpdateAllow(); control.Unchecked += (_, _) => UpdateAllow();
        stopButton.Click += (_, _) => RequestStop();
        Closing += OnClosing;
        Loaded += (_, _) => Start();
    }

    private void Start()
    {
        if (reader is not null) return;
        emergency = new PhysicalEmergencyListener(() => Dispatcher.BeginInvoke(new Action(RequestStop)));
        reader = Task.Run(ReadPipe);
        _ = reader.ContinueWith(result => Dispatcher.BeginInvoke(new Action(() =>
        { failed |= !result.IsCompletedSuccessfully || !result.Result; readerEnded = true; BeginOwnerStop(); })), TaskScheduler.Default);
    }

    private bool ReadPipe()
    {
        // Read in the background without blocking the visible desktop dispatcher.
        // A partial/invalid/oversized line becomes terminal EOF, never a command.
        var line = new List<byte>();
        var utf8 = new UTF8Encoding(false, true);
        try
        {
            int b;
            while ((b = input.ReadByte()) >= 0)
            {
                if (b == 10)
                {
                    if (line.Count == 0) return false;
                    var frame = utf8.GetString(line.ToArray()); line.Clear();
                    Dispatcher.Invoke(() => Receive(frame));
                }
                else { if (line.Count >= 8191) return false; line.Add((byte)b); }
            }
        }
        catch { return false; } // reader completion drives joined shutdown, never logs input
        return line.Count == 0;
    }

    private void Reply(object value)
    {
        var bytes = JsonSerializer.SerializeToUtf8Bytes(value);
        output.Write(bytes); output.WriteByte(10); output.Flush();
    }

    private void Receive(string json)
    {
        if (closed) return;
        if (json == "{\"Op\":\"stop\"}") { BeginOwnerStop(); return; }
        if (json == "{\"Op\":\"arm_stop\"}")
        {
            // This expectation belongs to a single active transfer. The local
            // stop button and emergency chord can still cancel it immediately.
            if (plannedStop != 0 || !activated || server is null || server.IsCompleted ||
                ownerStopping || stopRequested || failed || Interlocked.Read(ref frames) == 0)
            {
                Reply(new { status = "desktop_stop_unavailable" });
                return;
            }
            plannedStop = Stopwatch.GetTimestamp();
            Reply(new { status = "desktop_stop_armed" });
            return;
        }
        if (json == "{\"Op\":\"desktop_state\"}")
        {
            Reply(new { status = "desktop_state", frames = Interlocked.Read(ref frames),
                inputs = Interlocked.Read(ref events), releases = Interlocked.Read(ref releases), held = Volatile.Read(ref held),
                stop_requested = stopRequested, ended = activated && serverEnded, failed }); return;
        }
        if (ownerStopping || stopRequested) { Fail(); return; }
        try
        {
            if (preparation is null)
            {
                preparation = new OwnedAgentPreparation(json);
                status.Text = preparation.Scope == DesktopSessionPermission.Control ? "等待本次画面与键鼠请求" : "等待本次只读请求";
                control.Visibility = preparation.Scope == DesktopSessionPermission.Control ? Visibility.Visible : Visibility.Collapsed;
                Reply(new { status = "desktop_prepared", preparation.Permission, Role = "target" });
            }
            else if (json.Contains("\"show_invite\"", StringComparison.Ordinal))
            {
                var value = preparation.ShowInvite(json);
                invite.Text = $"本机设备码：{value.DeviceCode}\n本次验证码：{value.Code}";
                Reply(new { status = "invite_displayed", preparation.Permission });
            }
            else if (json.Contains("\"consent\"", StringComparison.Ordinal))
            {
                var value = preparation.BeginRequest(json);
                request.Text = $"请求 {value.RequestID[^8..]} · Mac {value.Controller[^12..]}\n" +
                    (preparation.Scope == DesktopSessionPermission.Control ? "请求查看画面并操作键鼠" : "请求仅查看画面");
                invite.Text = ""; control.IsChecked = false; pending = true; deny.IsEnabled = true;
                UpdateAllow(); Activate();
            }
            else if (json == "{\"Op\":\"activate\"}") ActivateDesktop(json);
            else CertificateProof(json);
        }
        catch { Fail(); }
    }

    private void UpdateAllow() => allow.IsEnabled = pending && !stopRequested &&
        (preparation?.Scope == DesktopSessionPermission.View || control.IsChecked == true && emergency?.Available == true);

    private void Decide(bool approved)
    {
        if (!pending || preparation is null || ownerStopping) return;
        try
        {
            localControl = preparation.Scope == DesktopSessionPermission.Control && control.IsChecked == true && emergency?.Available == true;
            var reply = preparation.Decide(approved, localControl);
            pending = false; allow.IsEnabled = deny.IsEnabled = control.IsEnabled = false;
            status.Text = approved ? "等待已授权的连接" : "已拒绝本次连接";
            Reply(reply);
        }
        catch { pending = false; Fail(); }
    }

    private void ActivateDesktop(string json)
    {
        if (activated || preparation is null) throw new InvalidOperationException();
        var scope = preparation.Activate(json);
        if (scope == DesktopSessionPermission.Control && (!localControl || emergency?.Available != true)) throw new InvalidOperationException();
        activated = true;
        certificate = AgentCertificateFactory.CreateSelfSigned(); // no persistent store reuse
        key = RandomNumberGenerator.GetBytes(32);
        var identity = RandomNumberGenerator.GetBytes(16);
        server = TlsProbeServer.RunOwnedDesktopOnceAsync(certificate, key, identity, scope, true,
            () => new PrimaryScreenCapture(), stop.Token,
            scope == DesktopSessionPermission.Control
                ? new InputSimulationOptions(() => new SessionNativeInputSink(s =>
                    { Interlocked.Exchange(ref events, s.Events); Interlocked.Exchange(ref releases, s.Releases); Volatile.Write(ref held, s.Held); }), LocalControlAllowed: true)
                : null,
            reportMetrics: m => Interlocked.Exchange(ref frames, m.FrameNumber));
        if (server.IsCompleted) { server.GetAwaiter().GetResult(); throw new InvalidOperationException(); }
        serverEnded = false;
        _ = ObserveServer(server);
        status.Text = "已允许 · 等待认证的 Mac";
        Reply(new { status = "ready", preparation.Permission, Key = Convert.ToBase64String(key),
            Fingerprint = CertificateFingerprint.FromCertificateDer(certificate.RawData).Hexadecimal });
    }

    private void CertificateProof(string json)
    {
        if (!activated || serverEnded || proofUsed || certificate is null || preparation is null) throw new InvalidOperationException();
        proofUsed = true;
        var p = OwnedAgentPreparation.Parse(json, "Op", "GrantID", "Target", "Challenge");
        if (p["Op"] != "certificate_proof" || !OwnedAgentPreparation.Hex(p["GrantID"], 32) ||
            p["Target"] != preparation.Self || !OwnedAgentPreparation.Hex(p["Challenge"], 64)) throw new InvalidDataException();
        var fingerprint = CertificateFingerprint.FromCertificateDer(certificate.RawData).Hexadecimal;
        var message = Encoding.UTF8.GetBytes("remoteapp-tls-certificate-proof-v1\n" + p["GrantID"] + "\n" +
            p["Target"] + "\n" + fingerprint + "\n" + p["Challenge"]);
        using var rsa = certificate.GetRSAPrivateKey() ?? throw new InvalidDataException();
        var proof = rsa.SignData(message, HashAlgorithmName.SHA256, RSASignaturePadding.Pss);
        Reply(new { status = "certificate_proof", Certificate = Convert.ToBase64String(certificate.RawData),
            CertificateProof = Convert.ToHexString(proof).ToLowerInvariant() });
    }

    private async Task ObserveServer(Task active)
    {
        try { await active; }
        catch (OperationCanceledException) when (stop.IsCancellationRequested) { }
        catch (IOException) when (ExpectedPeerClose()) { }
        catch { failed = true; }
        serverEnded = true;
        if (!ownerStopping && !stopRequested)
        {
            if (!ExpectedPeerClose()) failed = true;
            stopRequested = true;
            status.Text = "连接已结束";
        }
        metrics.Text = $"画面 {Interlocked.Read(ref frames)} · 已应用输入 {Interlocked.Read(ref events)} · 按住 {Volatile.Read(ref held)}";
        FinishIfJoined();
    }

    private bool ExpectedPeerClose() =>
        plannedStop != 0 && Stopwatch.GetElapsedTime(plannedStop) <= TimeSpan.FromSeconds(20);

    private void RequestStop()
    {
        if (stopRequested) return;
        if (pending) Decide(false);
        stopRequested = true; invite.Text = ""; allow.IsEnabled = deny.IsEnabled = control.IsEnabled = false;
        stop.Cancel(); // immediately stop screen/input; retain process for owner join
        status.Text = "正在释放输入并结束共享";
    }
    private void Fail()
    {
        if (failed) return;
        failed = true; pending = false; preparation?.Cancel();
        RequestStop(); Reply(new { status = "failed" });
    }
    private void BeginOwnerStop()
    {
        if (!ownerStopping)
        {
            pending = false; ownerStopping = true; preparation?.Cancel();
            RequestStop();
        }
        FinishIfJoined();
    }
    private void FinishIfJoined()
    {
        if (closed || !ownerStopping || !readerEnded || !serverEnded) return;
        try { emergency?.Dispose(); }
        catch { failed = true; status.Text = "本机紧急停止尚未完成释放"; return; } // retain unknown join
        certificate?.Dispose(); certificate = null;
        if (key is not null) { CryptographicOperations.ZeroMemory(key); key = null; }
        stop.Dispose(); input.Dispose(); closed = true;
        Reply(new { status = "closed" });
        Application.Current.Shutdown(failed ? 1 : 0);
    }
    private void OnClosing(object? sender, CancelEventArgs e) { if (!closed) { e.Cancel = true; RequestStop(); } }
}
