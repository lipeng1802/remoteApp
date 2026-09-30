using System.Net;
using System.Text.Json;
using RemoteProtocol;

const string consent = "--allow-local-mock";
var values = args.Where(value => value != consent).ToArray();
if (!args.Contains(consent, StringComparer.Ordinal) || values.Length is < 2 or > 3 ||
    !IPAddress.TryParse(values[0], out var bindAddress) ||
    !IPAddress.TryParse(values[1], out var expectedRemoteAddress) ||
    (values.Length == 3 && !int.TryParse(values[2], out _)))
{
    Console.Error.WriteLine("Usage: InputMockServer --allow-local-mock <bind-address> <expected-peer-address> [port]");
    return 2;
}
if (!TailscaleEndpoints.IsTailscaleIPv4(bindAddress) ||
    !TailscaleEndpoints.IsTailscaleIPv4(expectedRemoteAddress))
{
    Console.Error.WriteLine("Both input mock endpoints must be Tailscale IPv4 addresses.");
    return 2;
}
var port = values.Length == 3 ? int.Parse(values[2]) : 47475;
if (port is < 1024 or > 65535)
{
    Console.Error.WriteLine("Port must be between 1024 and 65535.");
    return 2;
}

using var credentials = AgentCredentialStore.LoadOrCreate();
using var certificate = AgentCertificateStore.LoadOrCreate();
using var timeout = new CancellationTokenSource(TimeSpan.FromMinutes(5));
var sink = new VerifyingInputSink();
var fingerprint = CertificateFingerprint.FromCertificateDer(certificate.RawData);

Console.WriteLine($"READY input-mock port={port} wait_seconds=300 expected_events={VerifyingInputSink.Expected.Count}");
Console.WriteLine($"CERTIFICATE_SHA256 {fingerprint.Hexadecimal}");
Console.WriteLine("NO_NATIVE_INPUT memory sink only");
try
{
    var options = new InputSimulationOptions(() => sink, LocalControlAllowed: true);
    await TlsProbeServer.RunRestrictedInputSimulationOnceAsync(bindAddress,
        expectedRemoteAddress, port, certificate, credentials.DeviceKey,
        credentials.AgentIdentifier, options, timeout.Token);
    sink.VerifyComplete();
    Console.WriteLine($"PASS input mock authenticated; {sink.Commands.Count} synthetic events verified; all held input released");
    return 0;
}
catch (ProtocolException exception)
{
    Console.Error.WriteLine($"FAIL input mock protocol rejected ({exception.Error})");
    return 1;
}
catch (Exception exception)
{
    Console.Error.WriteLine($"FAIL input mock stopped ({exception.GetType().Name})");
    return 1;
}
finally
{
    Console.WriteLine("CLOSED temporary input mock listener");
}

internal sealed class VerifyingInputSink : IInputSink
{
    internal static readonly IReadOnlyList<(MessageType Type, string Payload)> Expected = LoadExpected();

    internal List<(MessageType Type, string Payload)> Commands { get; } = [];
    private readonly HashSet<(ushort ScanCode, bool Extended)> heldKeys = [];
    private readonly HashSet<MouseButton> heldButtons = [];
    private int forcedKeyReleases;
    private int forcedButtonReleases;

    public void Move(MouseMovePayload point) => Add(MessageType.MouseMove, point.Encode());
    public void Wheel(MouseWheelPayload delta) => Add(MessageType.MouseWheel, delta.Encode());
    public void Button(MouseButtonPayload button)
    {
        Add(MessageType.MouseButton, button.Encode());
        if (button.Action == ButtonAction.Down) heldButtons.Add(button.Button);
        else heldButtons.Remove(button.Button);
    }
    public void Key(KeyEventPayload key)
    {
        Add(MessageType.KeyEvent, key.Encode());
        var identity = (key.ScanCode, key.Extended);
        if (key.Action == KeyAction.Down) heldKeys.Add(identity);
        else heldKeys.Remove(identity);
    }
    public void ReleaseAllKeys() { forcedKeyReleases++; heldKeys.Clear(); }
    public void ReleaseAllButtons() { forcedButtonReleases++; heldButtons.Clear(); }

    internal void VerifyComplete()
    {
        if (!Commands.SequenceEqual(Expected))
            throw new InvalidDataException("Synthetic input sequence did not match the expected mock vector.");
        if (heldKeys.Count != 0 || heldButtons.Count != 0)
            throw new InvalidDataException("Synthetic input remained held after the client stopped.");
        if (forcedKeyReleases != 0 || forcedButtonReleases != 0)
            throw new InvalidDataException("Server cleanup was required; client release frames were missing.");
    }

    private void Add(MessageType type, byte[] payload) =>
        Commands.Add((type, Convert.ToHexString(payload)));

    private static IReadOnlyList<(MessageType Type, string Payload)> LoadExpected()
    {
        using var document = JsonDocument.Parse(File.ReadAllText(Path.Combine(
            AppContext.BaseDirectory, "testdata", "controller-input-v1.json")));
        return document.RootElement.GetProperty("events").EnumerateArray().Select(item =>
            (Enum.Parse<MessageType>(item.GetProperty("type").GetString()!, ignoreCase: true),
             item.GetProperty("payloadHex").GetString()!.ToUpperInvariant())).ToArray();
    }
}
