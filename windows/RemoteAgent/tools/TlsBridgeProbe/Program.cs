// Private development pipe. No production credentials/keychain/native actions.
using System.Security.Cryptography;
using System.Text.Json;
using RemoteProtocol;

static void Output(object value) => Console.WriteLine(JsonSerializer.Serialize(value));
static string? Line()
{
    var chars = new List<char>();
    while (chars.Count < 8192)
    {
        var c = Console.Read();
        if (c < 0) { if (chars.Count > 0) throw new InvalidDataException(); return null; }
        if (c == '\n') return new string(chars.ToArray());
        if (c != '\r') chars.Add((char)c);
    }
    throw new InvalidDataException();
}
using var certificate = AgentCertificateFactory.CreateSelfSigned();
var key = RandomNumberGenerator.GetBytes(32);
var identity = RandomNumberGenerator.GetBytes(16);
using var stop = new CancellationTokenSource(TimeSpan.FromMinutes(5));
long pongs = 0;
long preauthRejected = 0;
long sessionsClosed = 0;
var server = TlsProbeServer.RunFixedPayloadBridgeAsync(47478, certificate, key, identity,
    () => Interlocked.Increment(ref pongs), stop.Token,
    reason => { if (reason == "AuthRequired") Interlocked.Increment(ref preauthRejected); },
    () => Interlocked.Increment(ref sessionsClosed));
Output(new { status = "ready", Key = Convert.ToBase64String(key),
    Fingerprint = CertificateFingerprint.FromCertificateDer(certificate.RawData).Hexadecimal });
try
{
    string? line;
    while ((line = Line()) is not null)
    {
        using var d = JsonDocument.Parse(line);
        var op = d.RootElement.GetProperty("Op").GetString();
        if (op == "stop") break;
        if (op != "count") throw new InvalidDataException();
        Output(new { status = "count", pongs = Interlocked.Read(ref pongs), preauth_rejected = Interlocked.Read(ref preauthRejected), sessions_closed = Interlocked.Read(ref sessionsClosed) });
    }
    stop.Cancel();
    try { await server.WaitAsync(TimeSpan.FromSeconds(5)); }
    catch (OperationCanceledException) when (stop.IsCancellationRequested) { }
    Output(new { status = "closed" });
    return 0;
}
catch { stop.Cancel(); Output(new { status = "failed" }); return 1; }
finally { CryptographicOperations.ZeroMemory(key); }
