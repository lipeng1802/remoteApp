using System.Buffers.Binary;
using System.Net;
using System.Net.Security;
using System.Net.Sockets;
using System.Security.Authentication;
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using System.Text.Json;
using RemoteProtocol;

internal static class JpegTests
{
    public static void ScreenVector()
    {
        using var vector = JsonDocument.Parse(File.ReadAllText(Path.Combine(AppContext.BaseDirectory, "testdata", "jpeg-v1.json")));
        var bytes = Convert.FromHexString(vector.RootElement.GetProperty("screenInfoHex").GetString()!);
        var info = ScreenInfoPayload.Decode(bytes);
        Check(info.Width == 3840 && info.Height == 2160 && info.DpiX100 == 14400, "Screen vector fields");
        Check(info.Encode().AsSpan().SequenceEqual(bytes), "Screen vector round trip");
        foreach (var invalid in new[] { new ScreenInfoPayload(0, 1, 9600, 9600), new ScreenInfoPayload(1, 20000, 9600, 9600), new ScreenInfoPayload(1, 1, 0, 9600) })
        {
            try { invalid.Encode(); throw new Exception("Invalid metadata accepted"); }
            catch (ProtocolException) { }
        }
        bytes[17] = 1;
        try { ScreenInfoPayload.Decode(bytes); throw new Exception("Secondary display accepted"); }
        catch (ProtocolException) { }
    }
    public static void StreamAndBackpressure() => Run(false, false);
    public static void NeverCaptureWithWrongKey() => Run(true, false);
    public static void CancelReleasesCapture() => Run(false, true);
    public static void NetworkBackpressureTimeout() => RunAsync(false, false, true, true).GetAwaiter().GetResult();
    public static void LargeFrame() => RunAsync(false, false, false, true).GetAwaiter().GetResult();
    public static void Discovery()
    {
        const string json = """{"BackendState":"Running","Self":{"Online":true,"TailscaleIPs":["100.64.0.1"]},"Peer":{"fixture":{"OS":"macOS","Online":true,"TailscaleIPs":["100.64.0.2"]}}}""";
        var endpoints = TailscaleEndpoints.Parse(json);
        Check(endpoints.Local.ToString() == "100.64.0.1" && endpoints.Peer.ToString() == "100.64.0.2", "Synthetic discovery");
        try { TailscaleEndpoints.Parse(json.Replace("\"Online\":true", "\"Online\":false")); throw new Exception("Offline accepted"); }
        catch (InvalidOperationException) { }
    }
    private static void Run(bool wrongKey, bool cancel) => RunAsync(wrongKey, cancel).GetAwaiter().GetResult();
    private static async Task RunAsync(bool wrongKey, bool cancel, bool stall = false, bool large = false)
    {
        using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(10));
        using var stop = CancellationTokenSource.CreateLinkedTokenSource(timeout.Token);
        using var certificate = AgentCertificateFactory.CreateSelfSigned();
        var key = RandomNumberGenerator.GetBytes(32);
        var source = new FakeSource { Large = large };
        var metrics = new List<JpegTransferMetrics>();
        var constructed = false;
        var reserve = new TcpListener(IPAddress.Loopback, 0);
        reserve.Start();
        var port = ((IPEndPoint)reserve.LocalEndpoint).Port;
        reserve.Stop();
        var server = TlsProbeServer.RunOnceAsync(IPAddress.Loopback, IPAddress.Loopback, port,
            certificate, key, new byte[16], stop.Token,
            createJpegSource: () => { constructed = true; return source; },
            reportMetrics: value => metrics.Add(value),
            frameTimeout: stall ? TimeSpan.FromSeconds(1) : null);
        using var tcp = new TcpClient();
        await tcp.ConnectAsync(IPAddress.Loopback, port, timeout.Token);
        using var tls = new SslStream(tcp.GetStream(), false, (_, peer, _, _) =>
            peer is not null && peer.GetRawCertData().AsSpan().SequenceEqual(certificate.RawData));
        await tls.AuthenticateAsClientAsync(new SslClientAuthenticationOptions {
            TargetHost = "localhost", EnabledSslProtocols = SslProtocols.Tls12 | SslProtocols.Tls13,
            CertificateRevocationCheckMode = X509RevocationMode.NoCheck
        }, timeout.Token);
        var writer = new ProbeFrameStream(tls);
        uint incoming = 1;
        var hello = HelloPayload.Decode((await Read()).Payload);
        Check(hello.Capabilities.HasFlag(Capabilities.Jpeg), "JPEG capability");
        Check(!constructed, "No capture before HELLO");
        var nonce = RandomNumberGenerator.GetBytes(32);
        await Send(MessageType.Hello, new HelloPayload(PeerRole.Controller, 1, 1, Capabilities.Jpeg, nonce).Encode());
        var challenge = AuthChallengePayload.Decode((await Read()).Payload);
        Check(!constructed, "No capture before HMAC");
        await Send(MessageType.AuthResponse, Authentication.CreateResponse(wrongKey ? new byte[32] : key,
            nonce, hello.Nonce, challenge.Challenge, challenge.AgentIdentifier));
        var auth = AuthResultPayload.Decode((await Read()).Payload);
        if (wrongKey)
        {
            Check(auth.Status == AuthResultStatus.Rejected, "Wrong key rejected");
            try { await server; throw new Exception("Expected rejection"); } catch (AuthenticationException) { }
            Check(!constructed && source.Count == 0, "Failed authentication never constructs capture");
            return;
        }
        Check(auth.Status == AuthResultStatus.Success, "Authenticated");
        Check((await Read()).Type == MessageType.ScreenInfo, "Screen metadata first");
        var frame = await Read();
        Check(frame.Type == MessageType.VideoFrameJpeg && frame.Payload.Length > 64 && frame.TimestampMicros != 0, "Bounded JPEG frame");
        await Task.Delay(230, timeout.Token);
        Check(source.Count >= 2, "Streaming continues without per-frame round trip");
        if (large) Check(frame.Payload.Length == 512 * 1024, "Large JPEG crosses TLS record boundaries");
        if (stall)
        {
            try { await server; throw new Exception("Expected frame deadline"); }
            catch (JpegTransferTimeoutException ex) { Check(ex.Stage == JpegTransferStage.Sending, "Timeout reports network send stage"); }
            Check(source.Disposed && source.Count >= 2, "Backpressure timeout closes and disposes capture");
            return;
        }
        if (cancel)
        {
            stop.Cancel();
            try { await server; throw new Exception("Expected local stop"); } catch (OperationCanceledException) { }
        }
        else
        {
            Check((await Read()).Type == MessageType.ScreenInfo, "Resolution change metadata precedes frame");
            Check((await Read()).Type == MessageType.VideoFrameJpeg, "Second JPEG");
            stop.Cancel();
            try { await server; throw new Exception("Expected local stop"); } catch (OperationCanceledException) { }
            Check(metrics.Count >= 1 && metrics[0].JpegBytes == frame.Payload.Length && metrics[0].CaptureMilliseconds >= 0 && metrics[0].SendMilliseconds >= 0 && metrics[0].FrameMilliseconds >= 0, "Metrics report bounded streaming work");
        }
        Check(source.Disposed, "Capture disposed");
        var reuse = new TcpListener(IPAddress.Loopback, port);
        reuse.Start(); reuse.Stop();
        async Task Send(MessageType type, byte[] payload) => await writer.WriteAsync(type, payload, timeout.Token);
        async Task<Frame> Read()
        {
            var header = new byte[28];
            await tls.ReadExactlyAsync(header, timeout.Token);
            var decoder = new FrameDecoder();
            var frames = decoder.Append(header);
            var length = BinaryPrimitives.ReadUInt32BigEndian(header.AsSpan(12));
            Check(length <= ProtocolConstants.MaximumPayloadLength, "Test read bounded");
            if (length > 0) { var body = new byte[(int)length]; await tls.ReadExactlyAsync(body, timeout.Token); frames = decoder.Append(body); }
            var received = frames.Single();
            Check(received.Sequence == incoming++, "Continuous wire sequence");
            return received;
        }
    }
    private static void Check(bool condition, string description) { if (!condition) throw new Exception(description); }
    private sealed class FakeSource : IJpegFrameSource
    {
        public bool Large;
        public int Count;
        public bool Disposed;
        public CapturedJpeg Capture(CancellationToken cancellationToken)
        {
            var count = Interlocked.Increment(ref Count);
            using var vector = JsonDocument.Parse(File.ReadAllText(Path.Combine(AppContext.BaseDirectory, "testdata", "jpeg-v1.json")));
            var jpeg = Convert.FromHexString(vector.RootElement.GetProperty("jpegHex").GetString()!);
            if (Large)
            {
                jpeg = new byte[512 * 1024];
                jpeg[0] = 0xff; jpeg[1] = 0xd8; jpeg[^2] = 0xff; jpeg[^1] = 0xd9;
            }
            return new CapturedJpeg(new ScreenInfoPayload(count == 1 ? 3840u : 1920u, count == 1 ? 2160u : 1080u, 14400, 14400), jpeg);
        }
        public void Dispose() => Disposed = true;
    }
}
