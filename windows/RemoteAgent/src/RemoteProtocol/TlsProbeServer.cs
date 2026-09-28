using System.Net;
using System.Security.Cryptography;
using System.Net.Security;
using System.Net.Sockets;
using System.Security.Authentication;
using System.Security.Cryptography.X509Certificates;

namespace RemoteProtocol;

public static class TlsProbeServer
{
    public static async Task RunOnceAsync(
        IPAddress bindAddress,
        IPAddress expectedRemoteAddress,
        int port,
        X509Certificate2 certificate,
        byte[] deviceKey,
        byte[] agentIdentifier,
        CancellationToken cancellationToken = default,
        TimeSpan? sessionTimeout = null,
        Func<IJpegFrameSource>? createJpegSource = null,
        Action<string>? reportStatus = null)
    {
        if (deviceKey.Length != 32 || agentIdentifier.Length != 16)
            throw new ArgumentException("Invalid device credential length.");
        ArgumentNullException.ThrowIfNull(bindAddress);
        ArgumentNullException.ThrowIfNull(expectedRemoteAddress);
        ArgumentNullException.ThrowIfNull(certificate);
        if (port is < IPEndPoint.MinPort or > IPEndPoint.MaxPort)
        {
            throw new ArgumentOutOfRangeException(nameof(port));
        }
        if (!certificate.HasPrivateKey)
        {
            throw new ArgumentException("The TLS server certificate must include a private key.", nameof(certificate));
        }

        var listener = new TcpListener(bindAddress, port);
        listener.Start(1);
        try
        {
            using var connectionDeadline = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
            connectionDeadline.CancelAfter(TimeSpan.FromMinutes(5));
            using var client = await listener.AcceptTcpClientAsync(connectionDeadline.Token).ConfigureAwait(false);
            var remoteEndPoint = client.Client.RemoteEndPoint as IPEndPoint;
            if (remoteEndPoint is null || !remoteEndPoint.Address.Equals(expectedRemoteAddress))
            {
                throw new InvalidDataException("Unexpected TLS probe peer.");
            }
            reportStatus?.Invoke("正在建立 TLS" );
            await using var tls = new SslStream(client.GetStream(), false);
            await tls.AuthenticateAsServerAsync(new SslServerAuthenticationOptions
            {
                ServerCertificate = certificate,
                ClientCertificateRequired = false,
                EnabledSslProtocols = SslProtocols.Tls12 | SslProtocols.Tls13,
                CertificateRevocationCheckMode = X509RevocationMode.NoCheck,
            }, connectionDeadline.Token).ConfigureAwait(false);

            using var sessionDeadline = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
            sessionDeadline.CancelAfter(sessionTimeout ?? TimeSpan.FromSeconds(20));
            await AuthenticateAndProbeAsync(tls, deviceKey, agentIdentifier, sessionDeadline.Token, sessionDeadline, createJpegSource, reportStatus).ConfigureAwait(false);
        }
        finally
        {
            listener.Stop();
        }
    }

    private static async Task AuthenticateAndProbeAsync(
        SslStream tls, byte[] deviceKey, byte[] agentIdentifier, CancellationToken cancellationToken,
        CancellationTokenSource sessionDeadline, Func<IJpegFrameSource>? createJpegSource, Action<string>? reportStatus)
    {
        var wire = new ProbeFrameStream(tls, createJpegSource is not null);
        var gate = new SessionGate(PeerRole.Agent);
        var agentNonce = RandomNumberGenerator.GetBytes(32);
        await wire.WriteAsync(MessageType.Hello,
            new HelloPayload(PeerRole.Agent, 1, 1, createJpegSource is null ? Capabilities.None : Capabilities.Jpeg, agentNonce).Encode(), cancellationToken);
        var helloFrame = await Receive(MessageType.Hello);
        var controllerHello = HelloPayload.Decode(helloFrame.Payload);
        if (createJpegSource is not null && !controllerHello.Capabilities.HasFlag(Capabilities.Jpeg))
            throw new ProtocolException(ProtocolError.InvalidPayload, "Peer does not support JPEG.");
        reportStatus?.Invoke("正在验证应用密钥");
        var challenge = new AuthChallengePayload(RandomNumberGenerator.GetBytes(32), agentIdentifier);
        await wire.WriteAsync(MessageType.AuthChallenge, challenge.Encode(), cancellationToken);
        _ = await Receive(MessageType.AuthResponse);
        var expected = Authentication.CreateResponse(deviceKey, controllerHello.Nonce, agentNonce,
            challenge.Challenge, challenge.AgentIdentifier);
        try { gate.CompleteAgentAuthentication(expected); }
        finally { CryptographicOperations.ZeroMemory(expected); }
        var accepted = gate.Phase == SessionPhase.Authenticated;
        // Each process accepts only one connection. Delay rejection before closing;
        // a persistent server will need a failure counter across connections.
        if (!accepted) await Task.Delay(TimeSpan.FromSeconds(1), cancellationToken);
        await wire.WriteAsync(MessageType.AuthResult,
            new AuthResultPayload(accepted ? AuthResultStatus.Success : AuthResultStatus.Rejected,
                accepted ? 0u : 1000u).Encode(), cancellationToken);
        if (!accepted) throw new AuthenticationException("Application authentication rejected.");
        if (createJpegSource is not null)
        {
            sessionDeadline.CancelAfter(Timeout.InfiniteTimeSpan);
            reportStatus?.Invoke("正在只读共享主屏 · 可随时停止");
            // No capture object is constructed until TLS and HMAC authentication pass.
            using var source = createJpegSource();
            ScreenInfoPayload? previous = null;
            while (true)
            {
                cancellationToken.ThrowIfCancellationRequested();
                var started = System.Diagnostics.Stopwatch.GetTimestamp();
                var captured = source.Capture(cancellationToken);
                if (captured.Jpeg.Length < 4 || captured.Jpeg[0] != 0xff || captured.Jpeg[1] != 0xd8 ||
                    captured.Jpeg[^2] != 0xff || captured.Jpeg[^1] != 0xd9)
                    throw new ProtocolException(ProtocolError.InvalidPayload, "Invalid captured JPEG.");
                using var frameDeadline = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
                frameDeadline.CancelAfter(TimeSpan.FromSeconds(10));
                if (previous != captured.Screen)
                {
                    await wire.WriteAsync(MessageType.ScreenInfo, captured.Screen.Encode(), frameDeadline.Token);
                    previous = captured.Screen;
                }
                await wire.WriteAsync(MessageType.VideoFrameJpeg, captured.Jpeg, frameDeadline.Token);
                var token = RandomNumberGenerator.GetBytes(8);
                await wire.WriteAsync(MessageType.Ping, token, frameDeadline.Token);
                var ack = await wire.ReadAsync(frameDeadline.Token);
                gate.Receive(ack);
                if (ack.Type == MessageType.Disconnect) return;
                if (ack.Type != MessageType.Pong || !ack.Payload.AsSpan().SequenceEqual(token))
                    throw new ProtocolException(ProtocolError.InvalidPayload, "Invalid frame acknowledgement.");
                var delay = TimeSpan.FromMilliseconds(100) - System.Diagnostics.Stopwatch.GetElapsedTime(started);
                if (delay > TimeSpan.Zero) await Task.Delay(delay, cancellationToken);
            }
        }
        var ping = await Receive(MessageType.Ping);
        if (ping.Payload.Length != 8)
            throw new ProtocolException(ProtocolError.InvalidPayload, "PING must contain eight bytes.");
        await wire.WriteAsync(MessageType.Pong, ping.Payload, cancellationToken);

        async Task<Frame> Receive(MessageType expectedType)
        {
            var frame = await wire.ReadAsync(cancellationToken);
            gate.Receive(frame);
            if (frame.Type != expectedType)
                throw new ProtocolException(ProtocolError.InvalidState, "Unexpected probe message.");
            return frame;
        }
    }
}