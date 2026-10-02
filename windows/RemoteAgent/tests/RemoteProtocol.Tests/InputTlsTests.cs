using System.Net;
using System.Net.Security;
using System.Net.Sockets;
using System.Security.Authentication;
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using System.Text.Json;
using RemoteProtocol;

internal static class InputTlsTests
{
    public static void CoalescedQueue() => Run("queue");
    public static void CaptureLifecycle() => Run("capture");
    public static void Success() => Run("success");
    public static void Clipboard() => Run("clipboard");
    public static void WrongKey() => Run("wrong-key");
    public static void PreAuth() => Run("preauth");
    public static void NoCapability() => Run("no-capability");
    public static void NoLocalPermission() => Run("no-local");
    public static void InvalidFrames()
    {
        foreach (var mode in new[] { "sequence", "malformed", "oversized", "bad-ping" }) Run(mode);
    }
    public static void Eof() => Run("eof");
    public static void Cancel() => Run("cancel");
    public static void Idle() => Run("idle");
    public static void Rate() => Run("rate");
    public static void Error() => Run("error");
    public static void RestrictedEndpoints()
    {
        Check(TailscaleEndpoints.IsTailscaleIPv4(IPAddress.Parse("100.64.0.1")), "Tailscale range start rejected.");
        Check(TailscaleEndpoints.IsTailscaleIPv4(IPAddress.Parse("100.127.255.255")), "Tailscale range end rejected.");
        foreach (var address in new[] { "100.63.255.255", "100.128.0.1", "127.0.0.1", "::1" })
            Check(!TailscaleEndpoints.IsTailscaleIPv4(IPAddress.Parse(address)), "Non-Tailscale endpoint accepted.");

        using var certificate = AgentCertificateFactory.CreateSelfSigned();
        var options = new InputSimulationOptions(() => new FakeSink(), LocalControlAllowed: true);
        try
        {
            _ = TlsProbeServer.RunRestrictedInputSimulationOnceAsync(IPAddress.Loopback,
                IPAddress.Parse("100.64.0.2"), 47475, certificate, new byte[32],
                new byte[16], options);
            throw new Exception("Restricted input mock accepted a loopback bind address.");
        }
        catch (ArgumentException) { }

        try
        {
            _ = TlsProbeServer.RunRestrictedInputSimulationOnceAsync(IPAddress.Parse("100.64.0.1"),
                IPAddress.Parse("100.64.0.2"), 47475, certificate, new byte[32],
                new byte[16], new InputSimulationOptions(() => new FakeSink()));
            throw new Exception("Restricted input mock listened without explicit local consent.");
        }
        catch (InvalidOperationException) { }
    }
    private static void Run(string mode) => RunAsync(mode).GetAwaiter().GetResult();
    private static void Check(bool value, string message) { if (!value) throw new Exception(message); }

    private static async Task RunAsync(string mode)
    {
        using var deadline = new CancellationTokenSource(TimeSpan.FromSeconds(12));
        using var stop = CancellationTokenSource.CreateLinkedTokenSource(deadline.Token);
        using var certificate = AgentCertificateFactory.CreateSelfSigned();
        var key = RandomNumberGenerator.GetBytes(32);
        var sink = new FakeSink();
        var constructed = false;
        var reservation = new TcpListener(IPAddress.Loopback, 0);
        reservation.Start();
        var port = ((IPEndPoint)reservation.LocalEndpoint).Port;
        reservation.Stop();
        var options = new InputSimulationOptions(() => { constructed = true; return sink; },
            LocalControlAllowed: mode != "no-local", BurstLimit: mode == "rate" ? 3 : 120,
            EventsPerSecond: mode == "rate" ? 1 : 240,
            ReadTimeout: mode == "idle" ? TimeSpan.FromSeconds(1) : TimeSpan.FromSeconds(5),
            ReadClipboardText: mode == "clipboard"
                ? _ => Task.FromResult(new ClipboardTextPayload(ClipboardTextStatus.Success, "跨设备 clipboard"))
                : null);
        var server = TlsProbeServer.RunInputSimulationOnceAsync(port, certificate, key, new byte[16], options, stop.Token);
        using var client = new TcpClient { NoDelay = true };
        try
        {
            await client.ConnectAsync(IPAddress.Loopback, port, deadline.Token);
            using var tls = new SslStream(client.GetStream(), false, (_, peer, _, _) =>
                peer is not null && peer.GetRawCertData().AsSpan().SequenceEqual(certificate.RawData));
            await tls.AuthenticateAsClientAsync(new SslClientAuthenticationOptions {
                TargetHost = "localhost", EnabledSslProtocols = SslProtocols.Tls12 | SslProtocols.Tls13,
                CertificateRevocationCheckMode = X509RevocationMode.NoCheck
            }, deadline.Token);
            var wire = new ProbeFrameStream(tls);
            uint nextSequence = 1;
            var hello = HelloPayload.Decode((await wire.ReadAsync(deadline.Token)).Payload);
            var expectedCapabilities = mode == "clipboard"
                ? Capabilities.Input | Capabilities.ClipboardText
                : Capabilities.Input;
            Check(hello.Capabilities == expectedCapabilities && !constructed,
                "Simulation advertised unexpected capabilities or created sink before auth.");
            if (mode == "preauth")
            {
                await Send(MessageType.KeyEvent, new KeyEventPayload(30, false, KeyAction.Down).Encode());
                await ExpectFailure(e => e is ProtocolException { Error: ProtocolError.AuthRequired });
                Check(!constructed && sink.Events.Count == 0, "Preauth must not construct input.");
                return;
            }
            var nonce = RandomNumberGenerator.GetBytes(32);
            await Send(MessageType.Hello, new HelloPayload(PeerRole.Controller, 1, 1,
                mode == "no-capability" ? Capabilities.Jpeg : expectedCapabilities, nonce).Encode());
            if (mode == "no-capability")
            {
                await ExpectFailure(e => e is ProtocolException { Error: ProtocolError.InvalidPayload });
                Check(!constructed, "Missing capability must not construct input.");
                return;
            }
            var challenge = AuthChallengePayload.Decode((await wire.ReadAsync(deadline.Token)).Payload);
            Check(!constructed, "Sink created before HMAC.");
            await Send(MessageType.AuthResponse, Authentication.CreateResponse(mode == "wrong-key" ? new byte[32] : key,
                nonce, hello.Nonce, challenge.Challenge, challenge.AgentIdentifier));
            var result = AuthResultPayload.Decode((await wire.ReadAsync(deadline.Token)).Payload);
            if (mode == "wrong-key")
            {
                Check(result.Status == AuthResultStatus.Rejected, "Wrong key rejected.");
                await ExpectFailure(e => e is AuthenticationException);
                Check(!constructed && sink.Events.Count == 0, "Wrong key must not construct input.");
                return;
            }
            Check(result.Status == AuthResultStatus.Success, "Authentication failed.");
            if (mode == "no-local")
            {
                await ExpectFailure(e => e is ProtocolException { Error: ProtocolError.InvalidState });
                Check(!constructed, "No local permission must not construct input.");
                return;
            }
            if (mode == "clipboard")
            {
                await Send(MessageType.ClipboardRequest, []);
                var response = await wire.ReadAsync(deadline.Token);
                Check(response.Type == MessageType.ClipboardText, "Clipboard response type changed.");
                Check(ClipboardTextPayload.Decode(response.Payload) ==
                    new ClipboardTextPayload(ClipboardTextStatus.Success, "跨设备 clipboard"),
                    "Clipboard response content changed.");
                await Send(MessageType.Disconnect, [0, 0]);
                await server;
                return;
            }
            if (mode is "capture" or "queue")
            {
                using var fixture = JsonDocument.Parse(File.ReadAllText(Path.Combine(
                    AppContext.BaseDirectory, "testdata", mode == "queue" ? "input-queue-v1.json" : "controller-input-v1.json")));
                var expected = new List<(MessageType Type, string Hex)>();
                foreach (var input in fixture.RootElement.GetProperty(mode == "queue" ? "expected" : "events").EnumerateArray())
                {
                    var type = Enum.Parse<MessageType>(input.GetProperty("type").GetString()!, ignoreCase: true);
                    var payload = Convert.FromHexString(input.GetProperty("payloadHex").GetString()!);
                    expected.Add((type, Convert.ToHexString(payload)));
                    await Send(type, payload);
                }
                var barrier = RandomNumberGenerator.GetBytes(8);
                await Send(MessageType.Ping, barrier);
                var ack = await wire.ReadAsync(deadline.Token);
                Check(ack.Type == MessageType.Pong && ack.Payload.SequenceEqual(barrier), "Capture order barrier failed.");
                Check(sink.Commands.SequenceEqual(expected), "Capture commands changed across TLS.");
                Check(sink.HeldKeys.Count == 0 && !sink.MouseHeld, "Focus loss left input held before disconnect.");
                Check(sink.KeyReleases == 0 && sink.MouseReleases == 0, "Client releases relied on server disposal.");
                await Send(MessageType.Disconnect, [0, 0]);
                await server;
                Check(sink.KeyReleases == 0 && sink.MouseReleases == 0, "Already released inputs cleaned twice.");
                var reuseCapturePort = new TcpListener(IPAddress.Loopback, port);
                reuseCapturePort.Start(); reuseCapturePort.Stop();
                return;
            }
            if (mode == "success")
            {
                using var fixture = JsonDocument.Parse(File.ReadAllText(Path.Combine(AppContext.BaseDirectory, "testdata", "mac-keymap-v1.json")));
                foreach (var payload in fixture.RootElement.GetProperty("shortcut").EnumerateArray())
                    await Send(MessageType.KeyEvent, Convert.FromHexString(payload.GetString()!));
                await Send(MessageType.MouseMove, new MouseMovePayload(32768, 32768).Encode());
                await Send(MessageType.MouseWheel, new MouseWheelPayload(0, -120).Encode());
            }
            await Send(MessageType.KeyEvent, new KeyEventPayload(29, true, KeyAction.Down).Encode());
            await Send(MessageType.MouseButton, new MouseButtonPayload(MouseButton.Left, ButtonAction.Down).Encode());
            var ping = RandomNumberGenerator.GetBytes(8);
            await Send(MessageType.Ping, ping);
            var pong = await wire.ReadAsync(deadline.Token);
            Check(pong.Type == MessageType.Pong && pong.Payload.SequenceEqual(ping), "Heartbeat/order barrier failed.");
            Check(sink.HeldKeys.Count == 1 && sink.MouseHeld, "Held input missing before cleanup.");

            if (mode == "success") await Send(MessageType.Disconnect, [0, 0]);
            if (mode == "error") await Send(MessageType.Error, [0, 1]);
            if (mode == "sequence")
                await tls.WriteAsync(FrameCodec.Encode(new Frame(MessageType.KeyEvent, 0, nextSequence + 1, 0, new KeyEventPayload(30, false, KeyAction.Down).Encode())), deadline.Token);
            if (mode == "malformed") await Send(MessageType.KeyEvent, [0, 30, 2, 1]);
            if (mode == "bad-ping") await Send(MessageType.Ping, [0]);
            if (mode == "oversized")
                await tls.WriteAsync(FrameCodec.Encode(new Frame(MessageType.KeyEvent, 0, nextSequence, 0, new byte[65])).AsMemory(0, 28), deadline.Token);
            if (mode == "eof")
            {
                await tls.WriteAsync(new byte[] { 0x50 }, deadline.Token);
                tls.Dispose(); // EOF while part of a header is pending.
            }
            if (mode == "cancel") stop.Cancel();
            if (mode == "rate")
            {
                var burst = Enumerable.Range(0, 16).SelectMany(_ => FrameCodec.Encode(
                    new Frame(MessageType.KeyEvent, 0, nextSequence++, 0, new KeyEventPayload(30, false, KeyAction.Down).Encode()))).ToArray();
                await tls.WriteAsync(burst, deadline.Token);
            }
            if (mode is "success" or "error") await server;
            else await ExpectFailure(e => mode switch {
                "sequence" or "rate" => e is ProtocolException { Error: ProtocolError.InvalidState },
                "malformed" or "bad-ping" => e is ProtocolException { Error: ProtocolError.InvalidPayload },
                "oversized" => e is ProtocolException { Error: ProtocolError.MessageTooLarge },
                "eof" => e is IOException,
                "cancel" or "idle" => e is OperationCanceledException,
                _ => false
            });
            Check(sink.HeldKeys.Count == 0 && !sink.MouseHeld, "Input remained held after TLS exit.");
            Check(sink.KeyReleases == 1 && sink.MouseReleases == 1, "Cleanup must run exactly once per held group.");
            if (mode == "success")
                Check(sink.Events.Take(4).SequenceEqual(new[] { "001D0001", "002E0001", "002E0002", "001D0002" }),
                    "Shared Mac shortcut changed across TLS.");
            var reuse = new TcpListener(IPAddress.Loopback, port);
            reuse.Start(); reuse.Stop();

            async Task Send(MessageType type, byte[] payload)
            {
                await wire.WriteAsync(type, payload, deadline.Token);
                nextSequence++;
            }
            async Task ExpectFailure(Func<Exception, bool> accepts)
            {
                try { await server; } catch (Exception ex) when (accepts(ex)) { return; }
                throw new Exception("Expected simulation failure was not observed.");
            }
        }
        finally
        {
            stop.Cancel();
            // Observe every server task even when a client-side assertion fails.
            try { await server; } catch { }
        }
    }

    private sealed class FakeSink : IInputSink
    {
        public readonly List<string> Events = [];
        public readonly List<(MessageType Type, string Hex)> Commands = [];
        public readonly HashSet<(ushort, bool)> HeldKeys = [];
        public bool MouseHeld;
        public int KeyReleases, MouseReleases;
        public void Key(KeyEventPayload key)
        {
            Commands.Add((MessageType.KeyEvent, Convert.ToHexString(key.Encode())));
            Events.Add(Convert.ToHexString(key.Encode()));
            if (key.Action == KeyAction.Down) HeldKeys.Add((key.ScanCode, key.Extended));
            else HeldKeys.Remove((key.ScanCode, key.Extended));
        }
        public void Move(MouseMovePayload point) { Commands.Add((MessageType.MouseMove, Convert.ToHexString(point.Encode()))); Events.Add("move"); }
        public void Wheel(MouseWheelPayload delta) { Commands.Add((MessageType.MouseWheel, Convert.ToHexString(delta.Encode()))); Events.Add("wheel"); }
        public void Button(MouseButtonPayload button) { Commands.Add((MessageType.MouseButton, Convert.ToHexString(button.Encode()))); MouseHeld = button.Action == ButtonAction.Down; }
        public void ReleaseAllKeys() { KeyReleases++; HeldKeys.Clear(); }
        public void ReleaseAllButtons() { MouseReleases++; MouseHeld = false; }
    }
}
