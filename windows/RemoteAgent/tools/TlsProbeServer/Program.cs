using System.Net;
using RemoteProtocol;

if (args.Length == 1 && args[0] == "--show-pairing")
{
    if (Console.IsOutputRedirected || Console.IsInputRedirected)
    {
        Console.Error.WriteLine("Pairing key can only be displayed in an interactive local terminal.");
        return 2;
    }
    using var pairing = AgentCredentialStore.LoadOrCreate();
    Console.WriteLine("Copy this key privately into the Mac pairing prompt. Do not paste it into chat or logs.");
    Console.WriteLine(Convert.ToBase64String(pairing.DeviceKey));
    return 0;
}

if (args.Length is < 2 or > 3 ||
    !IPAddress.TryParse(args[0], out var bindAddress) ||
    !IPAddress.TryParse(args[1], out var expectedRemoteAddress) ||
    (args.Length == 3 && !int.TryParse(args[2], out _)))
{
    Console.Error.WriteLine("Usage: TlsProbeServer <bind-address> <expected-peer-address> [port]");
    return 2;
}

var bindOctets = bindAddress.GetAddressBytes();
if (bindOctets.Length != 4 || bindOctets[0] != 100 || bindOctets[1] is < 64 or > 127)
{
    Console.Error.WriteLine("The TLS probe server only binds a Tailscale IPv4 address.");
    return 2;
}
var port = args.Length == 3 ? int.Parse(args[2]) : 47475;
if (port is < 1024 or > 65535)
{
    Console.Error.WriteLine("Port must be between 1024 and 65535.");
    return 2;
}
using var credentials = AgentCredentialStore.LoadOrCreate();
using var timeout = new CancellationTokenSource(TimeSpan.FromMinutes(5));
using var certificate = AgentCertificateStore.LoadOrCreate();
var fingerprint = CertificateFingerprint.FromCertificateDer(certificate.RawData);

Console.WriteLine($"READY tls-auth port={port} wait_seconds=300");
Console.WriteLine($"CERTIFICATE_SHA256 {fingerprint.Hexadecimal}");
try
{
    await TlsProbeServer.RunOnceAsync(
        bindAddress,
        expectedRemoteAddress,
        port,
        certificate,
        credentials.DeviceKey,
        credentials.AgentIdentifier,
        timeout.Token);
    Console.WriteLine("PASS TLS application authentication; protected PONG sent");
    return 0;
}
catch (ProtocolException exception)
{
    Console.Error.WriteLine($"FAIL TLS protocol rejected ({exception.Error})");
    return 1;
}
catch (Exception exception)
{
    Console.Error.WriteLine($"FAIL TLS probe stopped ({exception.GetType().Name})");
    return 1;
}
finally
{
    Console.WriteLine("CLOSED temporary TLS listener");
}
