using System.Net;
using System.Security.Cryptography;
using System.Net.Security;
using System.Net.Sockets;
using System.Security.Authentication;
using System.Security.Cryptography.X509Certificates;

namespace RemoteProtocol;

public static class TlsProbeServer
{
    public static async Task RunContinuousAsync(
        IPAddress bindAddress, IPAddress expectedRemoteAddress, int port, X509Certificate2 certificate,
        byte[] deviceKey, byte[] agentIdentifier, CancellationToken cancellationToken = default,
        TimeSpan? sessionTimeout = null, Func<IJpegFrameSource>? createJpegSource = null,
        Action<string>? reportStatus = null, Action<JpegTransferMetrics>? reportMetrics = null,
        TimeSpan? frameTimeout = null, InputSimulationOptions? inputSession = null)
    {
        while (true)
        {
            cancellationToken.ThrowIfCancellationRequested();
            try
            {
                await RunCoreAsync(bindAddress, expectedRemoteAddress, port, certificate, deviceKey,
                    agentIdentifier, cancellationToken, sessionTimeout, createJpegSource, reportStatus,
                    reportMetrics, frameTimeout, inputSession).ConfigureAwait(false);
            }
            catch (OperationCanceledException) when (!cancellationToken.IsCancellationRequested) { }
            catch (JpegTransferTimeoutException) { }
            catch (AuthenticationException) { }
            catch (ProtocolException) { }
            catch (InvalidDataException) { }
            catch (IOException) { }

            reportStatus?.Invoke("Mac 会话已结束 · 等待已配对的 Mac 重新连接");
        }
    }

    public static Task RunOnceAsync(
        IPAddress bindAddress, IPAddress expectedRemoteAddress, int port, X509Certificate2 certificate,
        byte[] deviceKey, byte[] agentIdentifier, CancellationToken cancellationToken = default,
        TimeSpan? sessionTimeout = null, Func<IJpegFrameSource>? createJpegSource = null,
        Action<string>? reportStatus = null, Action<JpegTransferMetrics>? reportMetrics = null,
        TimeSpan? frameTimeout = null, InputSimulationOptions? inputSession = null) =>
        RunCoreAsync(bindAddress, expectedRemoteAddress, port, certificate, deviceKey, agentIdentifier,
            cancellationToken, sessionTimeout, createJpegSource, reportStatus, reportMetrics, frameTimeout,
            inputSession);

    // Test-only profile: bind and peer are fixed to loopback, with no video/native sink.
    public static Task RunInputSimulationOnceAsync(
        int port, X509Certificate2 certificate, byte[] deviceKey, byte[] agentIdentifier,
        InputSimulationOptions options, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(options);
        options.Validate();
        return RunCoreAsync(IPAddress.Loopback, IPAddress.Loopback, port, certificate, deviceKey,
            agentIdentifier, cancellationToken, inputSimulation: options);
    }

    // Development-only cross-device mock. The sink remains caller-provided and
    // must not inject native input. Both endpoints are restricted to Tailscale CGNAT.
    public static Task RunRestrictedInputSimulationOnceAsync(
        IPAddress bindAddress, IPAddress expectedRemoteAddress, int port,
        X509Certificate2 certificate, byte[] deviceKey, byte[] agentIdentifier,
        InputSimulationOptions options, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(bindAddress);
        ArgumentNullException.ThrowIfNull(expectedRemoteAddress);
        ArgumentNullException.ThrowIfNull(options);
        if (!TailscaleEndpoints.IsTailscaleIPv4(bindAddress) ||
            !TailscaleEndpoints.IsTailscaleIPv4(expectedRemoteAddress))
            throw new ArgumentException("Input mock endpoints must be Tailscale IPv4 addresses.");
        options.Validate();
        if (!options.LocalControlAllowed)
            throw new InvalidOperationException("Explicit local input mock consent is required before listening.");
        return RunCoreAsync(bindAddress, expectedRemoteAddress, port, certificate, deviceKey,
            agentIdentifier, cancellationToken, inputSimulation: options);
    }

    private static async Task RunCoreAsync(
        IPAddress bindAddress,
        IPAddress expectedRemoteAddress,
        int port,
        X509Certificate2 certificate,
        byte[] deviceKey,
        byte[] agentIdentifier,
        CancellationToken cancellationToken = default,
        TimeSpan? sessionTimeout = null,
        Func<IJpegFrameSource>? createJpegSource = null,
        Action<string>? reportStatus = null,
        Action<JpegTransferMetrics>? reportMetrics = null,
        TimeSpan? frameTimeout = null, InputSimulationOptions? inputSimulation = null)
    {
        if (deviceKey.Length != 32 || agentIdentifier.Length != 16)
            throw new ArgumentException("Invalid device credential length.");
        ArgumentNullException.ThrowIfNull(bindAddress);
        ArgumentNullException.ThrowIfNull(expectedRemoteAddress);
        ArgumentNullException.ThrowIfNull(certificate);
        inputSimulation?.Validate();
        if (createJpegSource is not null && inputSimulation is { LocalControlAllowed: false })
            throw new InvalidOperationException("Explicit local input consent is required before listening.");
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
            client.NoDelay = true;
            // Keep kernel buffering bounded. Writes remain serial, so a slow receiver
            // applies backpressure without an application-level frame queue.
            client.SendBufferSize = 256 * 1024;
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
            await AuthenticateAndProbeAsync(tls, deviceKey, agentIdentifier, sessionDeadline.Token, sessionDeadline, createJpegSource, reportStatus, reportMetrics, frameTimeout ?? TimeSpan.FromSeconds(10), inputSimulation).ConfigureAwait(false);
        }
        finally
        {
            listener.Stop();
        }
    }

    private static async Task AuthenticateAndProbeAsync(
        SslStream tls, byte[] deviceKey, byte[] agentIdentifier, CancellationToken cancellationToken,
        CancellationTokenSource sessionDeadline, Func<IJpegFrameSource>? createJpegSource, Action<string>? reportStatus,
        Action<JpegTransferMetrics>? reportMetrics, TimeSpan frameTimeout, InputSimulationOptions? inputSimulation)
    {
        var wire = new ProbeFrameStream(tls, createJpegSource is not null);
        var gate = new SessionGate(PeerRole.Agent);
        var agentNonce = RandomNumberGenerator.GetBytes(32);
        var capabilities = (createJpegSource is null ? Capabilities.None : Capabilities.Jpeg) |
            (inputSimulation is null ? Capabilities.None : Capabilities.Input);
        await wire.WriteAsync(MessageType.Hello,
            new HelloPayload(PeerRole.Agent, 1, 1, capabilities, agentNonce).Encode(), cancellationToken);
        var helloFrame = await Receive(MessageType.Hello);
        var controllerHello = HelloPayload.Decode(helloFrame.Payload);
        if (createJpegSource is not null && !controllerHello.Capabilities.HasFlag(Capabilities.Jpeg))
            throw new ProtocolException(ProtocolError.InvalidPayload, "Peer does not support JPEG.");
        if (inputSimulation is not null && !controllerHello.Capabilities.HasFlag(Capabilities.Input))
            throw new ProtocolException(ProtocolError.InvalidPayload, "Input capability required for simulation.");
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
        if (inputSimulation is not null && createJpegSource is not null)
        {
            sessionDeadline.CancelAfter(Timeout.InfiniteTimeSpan);
            await InteractiveJpegSession.RunAsync(wire, gate, createJpegSource, inputSimulation,
                reportStatus, reportMetrics, frameTimeout, cancellationToken).ConfigureAwait(false);
            return;
        }
        if (inputSimulation is not null)
        {
            sessionDeadline.CancelAfter(Timeout.InfiniteTimeSpan);
            await InputSimulationSession.RunAsync(wire, gate, inputSimulation, cancellationToken).ConfigureAwait(false);
            return;
        }
        if (createJpegSource is not null)
        {
            sessionDeadline.CancelAfter(Timeout.InfiniteTimeSpan);
            reportStatus?.Invoke("正在只读共享主屏 · 可随时停止");
            // No capture object is constructed until TLS and HMAC authentication pass.
            using var source = createJpegSource();
            ScreenInfoPayload? previous = null;
            long lastReport = 0;
            long frameNumber = 0;
            while (true)
            {
                cancellationToken.ThrowIfCancellationRequested();
                var started = System.Diagnostics.Stopwatch.GetTimestamp();
                var captured = source.Capture(cancellationToken);
                frameNumber++;
                if (captured.Jpeg.Length < 4 || captured.Jpeg[0] != 0xff || captured.Jpeg[1] != 0xd8 ||
                    captured.Jpeg[^2] != 0xff || captured.Jpeg[^1] != 0xd9)
                    throw new ProtocolException(ProtocolError.InvalidPayload, "Invalid captured JPEG.");
                using var frameDeadline = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
                frameDeadline.CancelAfter(frameTimeout);
                var captureMs = System.Diagnostics.Stopwatch.GetElapsedTime(started).TotalMilliseconds;
                var sendStarted = System.Diagnostics.Stopwatch.GetTimestamp();
                var stage = JpegTransferStage.Sending;
                try
                {
                    if (previous != captured.Screen)
                    {
                        await wire.WriteAsync(MessageType.ScreenInfo, captured.Screen.Encode(), frameDeadline.Token);
                        previous = captured.Screen;
                    }
                    await wire.WriteAsync(MessageType.VideoFrameJpeg, captured.Jpeg, frameDeadline.Token);
                    var sendMs = System.Diagnostics.Stopwatch.GetElapsedTime(sendStarted).TotalMilliseconds;
                    if (lastReport == 0 || System.Diagnostics.Stopwatch.GetElapsedTime(lastReport) >= TimeSpan.FromSeconds(1))
                    {
                        reportMetrics?.Invoke(new JpegTransferMetrics(captured.Jpeg.Length, captureMs, sendMs,
                            System.Diagnostics.Stopwatch.GetElapsedTime(started).TotalMilliseconds, frameNumber));
                        lastReport = System.Diagnostics.Stopwatch.GetTimestamp();
                    }
                }
                catch (OperationCanceledException) when (!cancellationToken.IsCancellationRequested && frameDeadline.IsCancellationRequested)
                {
                    throw new JpegTransferTimeoutException(stage);
                }
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
