using System.Buffers.Binary;
using System.Net;
using System.Net.Security;
using System.Net.Sockets;
using System.Security.Authentication;
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using System.Text.Json;
using RemoteProtocol;

internal static class DuplexSessionTests
{
    public static void VideoAndInputShareOneAuthenticatedConnection() => RunAsync().GetAwaiter().GetResult();

    private static async Task RunAsync()
    {
        using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(12));
        using var certificate = AgentCertificateFactory.CreateSelfSigned();
        var key = RandomNumberGenerator.GetBytes(32);
        var sink = new FakeSink();
        var source = new FakeSource();
        var sinkConstructed = false;
        var sourceConstructed = false;
        var reservation = new TcpListener(IPAddress.Loopback, 0);
        reservation.Start();
        var port = ((IPEndPoint)reservation.LocalEndpoint).Port;
        reservation.Stop();
        using (var rejectDeadline = new CancellationTokenSource(TimeSpan.FromMilliseconds(250)))
        {
            try
            {
                await TlsProbeServer.RunOnceAsync(IPAddress.Loopback, IPAddress.Loopback, port,
                    certificate, key, new byte[16], rejectDeadline.Token,
                    createJpegSource: () => { sourceConstructed = true; return source; },
                    inputSession: new InputSimulationOptions(() => { sinkConstructed = true; return sink; }));
                throw new Exception("Duplex listener accepted missing local consent.");
            }
            catch (InvalidOperationException) { }
        }
        Check(!sourceConstructed && !sinkConstructed,
            "Rejected duplex consent constructed protected resources.");
        var server = TlsProbeServer.RunOnceAsync(IPAddress.Loopback, IPAddress.Loopback, port,
            certificate, key, new byte[16], timeout.Token,
            createJpegSource: () => { sourceConstructed = true; return source; },
            inputSession: new InputSimulationOptions(() => { sinkConstructed = true; return sink; }, LocalControlAllowed: true,
                ReadTimeout: TimeSpan.FromSeconds(5)));

        using var client = new TcpClient { NoDelay = true };
        try
        {
            await client.ConnectAsync(IPAddress.Loopback, port, timeout.Token);
            using var tls = new SslStream(client.GetStream(), false, (_, peer, _, _) =>
                peer is not null && peer.GetRawCertData().AsSpan().SequenceEqual(certificate.RawData));
            await tls.AuthenticateAsClientAsync(new SslClientAuthenticationOptions
            {
                TargetHost = "localhost",
                EnabledSslProtocols = SslProtocols.Tls12 | SslProtocols.Tls13,
                CertificateRevocationCheckMode = X509RevocationMode.NoCheck,
            }, timeout.Token);
            var writer = new ProbeFrameStream(tls);
            uint incomingSequence = 1;
            var hello = HelloPayload.Decode((await Read()).Payload);
            Check(hello.Capabilities == (Capabilities.Jpeg | Capabilities.Input),
                "Duplex server did not advertise JPEG and input together.");
            Check(!sourceConstructed && !sinkConstructed, "Protected resources were created before authentication.");
            var nonce = RandomNumberGenerator.GetBytes(32);
            await writer.WriteAsync(MessageType.Hello, new HelloPayload(PeerRole.Controller, 1, 1,
                Capabilities.Jpeg | Capabilities.Input, nonce).Encode(), timeout.Token);
            var challenge = AuthChallengePayload.Decode((await Read()).Payload);
            await writer.WriteAsync(MessageType.AuthResponse, Authentication.CreateResponse(key, nonce,
                hello.Nonce, challenge.Challenge, challenge.AgentIdentifier), timeout.Token);
            var result = AuthResultPayload.Decode((await Read()).Payload);
            Check(result.Status == AuthResultStatus.Success, "Duplex authentication failed.");

            var sawScreen = false;
            var sawVideo = false;
            for (var count = 0; count < 10 && !(sawScreen && sawVideo); count++)
            {
                var frame = await Read();
                if (frame.Type == MessageType.ScreenInfo) sawScreen = true;
                if (frame.Type == MessageType.VideoFrameJpeg) sawVideo = true;
            }
            Check(sawScreen && sawVideo, "Duplex video did not start after authentication.");

            await writer.WriteAsync(MessageType.KeyEvent,
                new KeyEventPayload(0x1d, true, KeyAction.Down).Encode(), timeout.Token);
            await writer.WriteAsync(MessageType.MouseButton,
                new MouseButtonPayload(MouseButton.Left, ButtonAction.Down).Encode(), timeout.Token);
            var heartbeat = RandomNumberGenerator.GetBytes(8);
            await writer.WriteAsync(MessageType.Ping, heartbeat, timeout.Token);

            var sawPong = false;
            for (var count = 0; count < 30 && !sawPong; count++)
            {
                var frame = await Read();
                if (frame.Type == MessageType.ScreenInfo) sawScreen = true;
                if (frame.Type == MessageType.VideoFrameJpeg) sawVideo = true;
                if (frame.Type == MessageType.Pong)
                {
                    Check(frame.Payload.SequenceEqual(heartbeat), "Duplex heartbeat payload changed.");
                    sawPong = true;
                }
            }
            Check(sawPong, "Video and PONG did not share the outgoing sequence.");
            Check(sink.HeldKeys.Count == 1 && sink.HeldButtons.Contains(MouseButton.Left),
                "Input was not applied before the PONG ordering barrier.");
            await writer.WriteAsync(MessageType.Disconnect, [0, 0], timeout.Token);
            await server;
            Check(sourceConstructed && sinkConstructed && source.Started && source.Disposed && source.Count > 0,
                "Protected resource lifecycle did not close.");
            Check(sink.HeldKeys.Count == 0 && sink.HeldButtons.Count == 0,
                "Disconnect left duplex input held.");
            Check(sink.KeyReleases == 1 && sink.ButtonReleases == 1,
                "Disconnect did not release each held input group exactly once.");
            var reuse = new TcpListener(IPAddress.Loopback, port);
            reuse.Start();
            reuse.Stop();

            async Task<Frame> Read()
            {
                var header = new byte[ProtocolConstants.HeaderLength];
                await tls.ReadExactlyAsync(header, timeout.Token);
                var length = BinaryPrimitives.ReadUInt32BigEndian(header.AsSpan(12));
                Check(length <= ProtocolConstants.MaximumPayloadLength, "Duplex frame exceeded the wire bound.");
                var decoder = new FrameDecoder();
                var frames = decoder.Append(header);
                if (length != 0)
                {
                    var body = new byte[(int)length];
                    await tls.ReadExactlyAsync(body, timeout.Token);
                    frames = decoder.Append(body);
                }
                decoder.Finish();
                var frame = frames.Single();
                Check(frame.Sequence == incomingSequence++, "Duplex outgoing sequence was not continuous.");
                return frame;
            }
        }
        finally
        {
            timeout.Cancel();
            try { await server; } catch { }
        }
    }

    private static void Check(bool value, string message)
    {
        if (!value) throw new Exception(message);
    }

    private sealed class FakeSource : IJpegFrameSource
    {
        public bool Started;
        public bool Disposed;
        public int Count;

        public CapturedJpeg Capture(CancellationToken cancellationToken)
        {
            Started = true;
            Count++;
            using var vector = JsonDocument.Parse(File.ReadAllText(Path.Combine(
                AppContext.BaseDirectory, "testdata", "jpeg-v1.json")));
            return new CapturedJpeg(new ScreenInfoPayload(1280, 720, 9600, 9600),
                Convert.FromHexString(vector.RootElement.GetProperty("jpegHex").GetString()!));
        }

        public void Dispose() => Disposed = true;
    }

    private sealed class FakeSink : IInputSink
    {
        public bool Started;
        public HashSet<(ushort, bool)> HeldKeys { get; } = [];
        public HashSet<MouseButton> HeldButtons { get; } = [];
        public int KeyReleases;
        public int ButtonReleases;

        public void Key(KeyEventPayload key)
        {
            Started = true;
            var identity = (key.ScanCode, key.Extended);
            if (key.Action == KeyAction.Down) HeldKeys.Add(identity); else HeldKeys.Remove(identity);
        }
        public void Move(MouseMovePayload point) => Started = true;
        public void Wheel(MouseWheelPayload delta) => Started = true;
        public void Button(MouseButtonPayload button)
        {
            Started = true;
            if (button.Action == ButtonAction.Down) HeldButtons.Add(button.Button);
            else HeldButtons.Remove(button.Button);
        }
        public void ReleaseAllKeys() { KeyReleases++; HeldKeys.Clear(); }
        public void ReleaseAllButtons() { ButtonReleases++; HeldButtons.Clear(); }
    }
}
