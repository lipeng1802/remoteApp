using System.Net;
using System.Net.Security;
using System.Net.Sockets;
using System.Security.Authentication;
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using RemoteProtocol;

internal static class AuthenticatedTlsTests
{
    public static void Success() => Run("success");
    public static void WrongKey() => Run("wrong-key");
    public static void BeforeAuth() => Run("preauth");
    public static void Replay()
    {
        byte[]? captured = null;
        Run("success", response => captured = response);
        Run("replay", replay: captured);
    }
    public static void BadSequence() => Run("sequence");
    public static void OversizedHeader() => Run("oversized");
    public static void TruncatedFrame() => Run("truncated");
    public static void Timeout() => Run("timeout");

    private static void Run(string mode, Action<byte[]>? capture = null, byte[]? replay = null)
    {
        using var certificate = AgentCertificateFactory.CreateSelfSigned();
        using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(10));
        // A stable test credential is deliberately never used by the executable.
        var key = Enumerable.Repeat((byte)0x51, 32).ToArray();
        var identity = Enumerable.Repeat((byte)0x12, 16).ToArray();
        var reservation = new TcpListener(IPAddress.Loopback, 0);
        reservation.Start();
        var port = ((IPEndPoint)reservation.LocalEndpoint).Port;
        reservation.Stop();
        var server = TlsProbeServer.RunOnceAsync(IPAddress.Loopback, IPAddress.Loopback,
            port, certificate, key, identity, timeout.Token,
            mode == "timeout" ? TimeSpan.FromMilliseconds(150) : TimeSpan.FromSeconds(5));
        using var client = new TcpClient();
        client.ConnectAsync(IPAddress.Loopback, port, timeout.Token).GetAwaiter().GetResult();
        using var tls = new SslStream(client.GetStream(), false, (_, peer, _, _) =>
            peer is not null && peer.GetRawCertData().AsSpan().SequenceEqual(certificate.RawData));
        tls.AuthenticateAsClientAsync(new SslClientAuthenticationOptions {
            TargetHost = "localhost", EnabledSslProtocols = SslProtocols.Tls12 | SslProtocols.Tls13,
            CertificateRevocationCheckMode = X509RevocationMode.NoCheck
        }, timeout.Token).GetAwaiter().GetResult();
        var wire = new ProbeFrameStream(tls);
        var hello = HelloPayload.Decode(Read().Payload);
        if (mode == "timeout")
        {
            try { server.GetAwaiter().GetResult(); throw new Exception("Expected timeout"); }
            catch (OperationCanceledException) { return; }
        }
        if (mode is "preauth" or "sequence" or "oversized" or "truncated")
        {
            if (mode == "preauth") Send(MessageType.Ping, new byte[8]);
            if (mode == "sequence") tls.Write(FrameCodec.Encode(new Frame(MessageType.Hello, 0, 2, 0,
                new HelloPayload(PeerRole.Controller, 1, 1, Capabilities.None, new byte[32]).Encode())));
            if (mode == "oversized") tls.Write(FrameCodec.Encode(new Frame(MessageType.Hello, 0, 1, 0, new byte[65]))[..28]);
            if (mode == "truncated") { tls.Write(new byte[] { 0x50 }); tls.Dispose(); }
            try { server.GetAwaiter().GetResult(); throw new Exception("Invalid peer was accepted"); }
            catch (ProtocolException ex) when (
                (mode == "preauth" && ex.Error == ProtocolError.AuthRequired) ||
                (mode == "sequence" && ex.Error == ProtocolError.InvalidState) ||
                (mode == "oversized" && ex.Error == ProtocolError.MessageTooLarge)) { return; }
            catch (EndOfStreamException) when (mode == "truncated") { return; }
            catch (IOException) when (mode == "truncated") { return; }
        }
        var nonce = RandomNumberGenerator.GetBytes(32);
        Send(MessageType.Hello, new HelloPayload(PeerRole.Controller, 1, 1, Capabilities.None, nonce).Encode());
        var challenge = AuthChallengePayload.Decode(Read().Payload);
        var response = replay ?? Authentication.CreateResponse(
            mode == "wrong-key" ? new byte[32] : key, nonce, hello.Nonce, challenge.Challenge, challenge.AgentIdentifier);
        capture?.Invoke(response);
        Send(MessageType.AuthResponse, response);
        var result = AuthResultPayload.Decode(Read().Payload);
        if (mode is "wrong-key" or "replay")
        {
            if (result.Status != AuthResultStatus.Rejected) throw new Exception("Invalid response accepted");
            try { server.GetAwaiter().GetResult(); throw new Exception("Expected authentication failure"); }
            catch (AuthenticationException) { }
            var trailing = new byte[1];
            if (tls.ReadAsync(trailing, timeout.Token).AsTask().GetAwaiter().GetResult() != 0)
                throw new Exception("Unexpected data after rejection");
            return;
        }
        if (result.Status != AuthResultStatus.Success) throw new Exception("Authentication rejected");
        var ping = RandomNumberGenerator.GetBytes(8);
        Send(MessageType.Ping, ping);
        var pong = Read();
        if (pong.Type != MessageType.Pong || !ping.AsSpan().SequenceEqual(pong.Payload))
            throw new Exception("Missing authenticated PONG");
        server.GetAwaiter().GetResult();
        Frame Read() => wire.ReadAsync(timeout.Token).GetAwaiter().GetResult();
        void Send(MessageType type, byte[] payload) => wire.WriteAsync(type, payload, timeout.Token).GetAwaiter().GetResult();
    }
}