using System.Net;
using System.Net.Security;
using System.Net.Sockets;
using System.Security.Authentication;
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using RemoteProtocol;

internal static class OwnedDesktopTests
{
    public static void ConsentBeforeListen()
    {
        using var certificate = AgentCertificateFactory.CreateSelfSigned();
        // Occupy the fixed port: absence of consent must reject BEFORE bind,
        // not appear to pass because a listener failed for another reason.
        var reservation = new TcpListener(IPAddress.Loopback, 47478);
        reservation.Start();
        try
        {
            foreach (var sharingAllowed in new[] { false, true })
            {
                try
                {
                    _ = TlsProbeServer.RunOwnedDesktopOnceAsync(certificate, new byte[32], new byte[16],
                        DesktopSessionPermission.Control, sharingAllowed, () => throw new Exception("Capture constructed without consent."),
                        CancellationToken.None, sharingAllowed
                            ? new InputSimulationOptions(() => throw new Exception("Input constructed without consent."))
                            : null);
                    throw new Exception("Missing sharing/input consent was accepted.");
                }
                catch (InvalidOperationException) { }
            }
        }
        finally { reservation.Stop(); }
    }

    public static void WrongKey() => RejectWrongKeyAsync().GetAwaiter().GetResult();

    private static async Task RejectWrongKeyAsync()
    {
        using var deadline = new CancellationTokenSource(TimeSpan.FromSeconds(12));
        using var certificate = AgentCertificateFactory.CreateSelfSigned();
        var sourceOpened = false;
        var sinkOpened = false;
        var server = TlsProbeServer.RunOwnedDesktopOnceAsync(certificate, new byte[32], new byte[16],
            DesktopSessionPermission.Control,
            localSharingAllowed: true,
            createJpegSource: () => { sourceOpened = true; throw new Exception("Unauthenticated capture."); },
            deadline.Token,
            new InputSimulationOptions(() => { sinkOpened = true; throw new Exception("Unauthenticated input."); },
                LocalControlAllowed: true));
        try
        {
            using var client = new TcpClient();
            await client.ConnectAsync(IPAddress.Loopback, 47478, deadline.Token);
            using var tls = new SslStream(client.GetStream(), false, (_, peer, _, _) =>
                peer is not null && peer.GetRawCertData().AsSpan().SequenceEqual(certificate.RawData));
            await tls.AuthenticateAsClientAsync(new SslClientAuthenticationOptions
            {
                TargetHost = "localhost", EnabledSslProtocols = SslProtocols.Tls12 | SslProtocols.Tls13,
                CertificateRevocationCheckMode = X509RevocationMode.NoCheck,
            }, deadline.Token);
            var wire = new ProbeFrameStream(tls);
            _ = await wire.ReadAsync(deadline.Token);
            await wire.WriteAsync(MessageType.Hello, new HelloPayload(PeerRole.Controller, 1, 1,
                Capabilities.Jpeg | Capabilities.Input, RandomNumberGenerator.GetBytes(32)).Encode(), deadline.Token);
            _ = await wire.ReadAsync(deadline.Token);
            await wire.WriteAsync(MessageType.AuthResponse, new byte[32], deadline.Token);
            var result = await wire.ReadAsync(deadline.Token);
            if (result.Type != MessageType.AuthResult ||
                AuthResultPayload.Decode(result.Payload).Status != AuthResultStatus.Rejected)
                throw new Exception("Wrong key was not explicitly rejected.");
            try { await server; throw new Exception("Wrong key accepted by server."); }
            catch (AuthenticationException) { }
            if (sourceOpened || sinkOpened) throw new Exception("Wrong key opened protected resources.");
            var reuse = new TcpListener(IPAddress.Loopback, 47478);
            reuse.Start();
            reuse.Stop();
        }
        finally
        {
            deadline.Cancel();
            try { await server; } catch { }
        }
    }

    public static void ViewGrantCannotEnableInput()
    {
        using var certificate = AgentCertificateFactory.CreateSelfSigned();
        var reservation = new TcpListener(IPAddress.Loopback, 47478);
        reservation.Start();
        try
        {
            foreach (var scope in new[] { DesktopSessionPermission.View, (DesktopSessionPermission)0, (DesktopSessionPermission)3 })
            {
                try
                {
                    _ = TlsProbeServer.RunOwnedDesktopOnceAsync(certificate, new byte[32], new byte[16], scope,
                        localSharingAllowed: true, () => throw new Exception("Unauthorized capture."), CancellationToken.None,
                        new InputSimulationOptions(() => throw new Exception("Unauthorized input."), LocalControlAllowed: true));
                    throw new Exception("Scope escalation was accepted.");
                }
                catch (InvalidOperationException) { }
            }
        }
        finally { reservation.Stop(); }
    }

    public static void DesktopGrantCannotEnableClipboard()
    {
        using var certificate = AgentCertificateFactory.CreateSelfSigned();
        var reservation = new TcpListener(IPAddress.Loopback, 47478);
        reservation.Start();
        try
        {
            foreach (var options in new[] {
                new InputSimulationOptions(() => throw new Exception("Unauthorized input."), LocalControlAllowed: true,
                    ReadClipboardText: _ => throw new Exception("Unauthorized clipboard read.")),
                new InputSimulationOptions(() => throw new Exception("Unauthorized input."), LocalControlAllowed: true,
                    WriteClipboardText: (_, _) => throw new Exception("Unauthorized clipboard write.")),
            })
            {
                try
                {
                    _ = TlsProbeServer.RunOwnedDesktopOnceAsync(certificate, new byte[32], new byte[16],
                        DesktopSessionPermission.Control, localSharingAllowed: true,
                        () => throw new Exception("Unauthorized capture."), CancellationToken.None, options);
                    throw new Exception("Clipboard permission was inferred from desktop control.");
                }
                catch (InvalidOperationException) { }
            }
        }
        finally { reservation.Stop(); }
    }
}
