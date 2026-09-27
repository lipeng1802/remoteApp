using System.Net;
using System.Net.Security;
using System.Net.Sockets;
using System.Security.Authentication;
using System.Security.Cryptography.X509Certificates;

namespace RemoteProtocol;

public static class TlsProbeServer
{
    public static ReadOnlyMemory<byte> Request => "prd-tls-test\n"u8.ToArray();
    public static ReadOnlyMemory<byte> Response => "prd-tls-ok\n"u8.ToArray();
    public const int MaximumRequestLength = 32;

    public static async Task RunOnceAsync(
        IPAddress bindAddress,
        IPAddress expectedRemoteAddress,
        int port,
        X509Certificate2 certificate,
        CancellationToken cancellationToken = default)
    {
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
            using var client = await listener.AcceptTcpClientAsync(cancellationToken).ConfigureAwait(false);
            var remoteEndPoint = client.Client.RemoteEndPoint as IPEndPoint;
            if (remoteEndPoint is null || !remoteEndPoint.Address.Equals(expectedRemoteAddress))
            {
                throw new InvalidDataException("Unexpected TLS probe peer.");
            }
            await using var tls = new SslStream(client.GetStream(), false);
            await tls.AuthenticateAsServerAsync(new SslServerAuthenticationOptions
            {
                ServerCertificate = certificate,
                ClientCertificateRequired = false,
                EnabledSslProtocols = SslProtocols.Tls12 | SslProtocols.Tls13,
                CertificateRevocationCheckMode = X509RevocationMode.NoCheck,
            }, cancellationToken).ConfigureAwait(false);

            var request = await ReadBoundedLineAsync(tls, cancellationToken).ConfigureAwait(false);
            if (!request.AsSpan().SequenceEqual(Request.Span))
            {
                throw new InvalidDataException("Unexpected TLS probe request.");
            }
            await tls.WriteAsync(Response, cancellationToken).ConfigureAwait(false);
            await tls.FlushAsync(cancellationToken).ConfigureAwait(false);
        }
        finally
        {
            listener.Stop();
        }
    }

    private static async Task<byte[]> ReadBoundedLineAsync(Stream stream, CancellationToken cancellationToken)
    {
        var result = new byte[MaximumRequestLength];
        var count = 0;
        while (count < result.Length)
        {
            var read = await stream.ReadAsync(result.AsMemory(count, 1), cancellationToken).ConfigureAwait(false);
            if (read == 0)
            {
                throw new EndOfStreamException("TLS probe request ended before a newline.");
            }
            count += read;
            if (result[count - 1] == (byte)'\n')
            {
                return result.AsSpan(0, count).ToArray();
            }
        }
        throw new InvalidDataException("TLS probe request exceeds the maximum length.");
    }
}
