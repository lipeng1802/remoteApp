using System.Text.Json;
using RemoteProtocol;

var tests = new (string Name, Action Run)[]
{
    ("golden vectors decode and re-encode", GoldenVectors),
    ("one-byte stream splits", OneByteSplits),
    ("coalesced frames", CoalescedFrames),
    ("invalid magic", InvalidMagic),
    ("oversized payload rejected from header", OversizedPayload),
    ("incomplete frame rejected at end", IncompleteFrame),
    ("authentication golden vector", AuthenticationGoldenVector),
    ("hello payload round trip", HelloRoundTrip),
    ("malformed handshake payloads", MalformedHandshakePayloads),
    ("video rejected before authentication", VideoBeforeAuthentication),
    ("controller authentication transition", ControllerAuthenticationTransition),
    ("agent authentication transition", AgentAuthenticationTransition),
};

var failures = 0;
foreach (var test in tests)
{
    try
    {
        test.Run();
        Console.WriteLine($"PASS {test.Name}");
    }
    catch (Exception exception)
    {
        failures++;
        Console.Error.WriteLine($"FAIL {test.Name}: {exception.Message}");
    }
}

Console.WriteLine($"{tests.Length - failures}/{tests.Length} tests passed");
return failures == 0 ? 0 : 1;

static Manifest LoadManifest()
{
    var path = Path.Combine(AppContext.BaseDirectory, "testdata", "v1.json");
    var json = File.ReadAllText(path);
    return JsonSerializer.Deserialize<Manifest>(json, new JsonSerializerOptions
    {
        PropertyNameCaseInsensitive = true,
    }) ?? throw new InvalidOperationException("Unable to decode the golden vector manifest.");
}

static AuthVector LoadAuthVector()
{
    var path = Path.Combine(AppContext.BaseDirectory, "testdata", "auth-v1.json");
    var json = File.ReadAllText(path);
    return JsonSerializer.Deserialize<AuthVector>(json, new JsonSerializerOptions
    {
        PropertyNameCaseInsensitive = true,
    }) ?? throw new InvalidOperationException("Unable to decode the authentication vector.");
}

static void GoldenVectors()
{
    foreach (var vector in LoadManifest().Vectors)
    {
        var wire = Convert.FromHexString(vector.FrameHex);
        var decoder = new FrameDecoder();
        var frames = decoder.Append(wire);
        Equal(1, frames.Count, vector.Name);
        var frame = frames[0];
        Equal(vector.MessageType, (ushort)frame.Type, vector.Name);
        Equal(vector.Flags, frame.Flags, vector.Name);
        Equal(vector.PayloadLength, frame.Payload.Length, vector.Name);
        Equal(vector.Sequence, frame.Sequence, vector.Name);
        Equal(vector.TimestampMicros, frame.TimestampMicros, vector.Name);
        SequenceEqual(Convert.FromHexString(vector.PayloadHex), frame.Payload, vector.Name);
        SequenceEqual(wire, FrameCodec.Encode(frame), vector.Name);
        decoder.Finish();
    }
}

static void OneByteSplits()
{
    var wire = Convert.FromHexString(LoadManifest().Vectors[0].FrameHex);
    var decoder = new FrameDecoder();
    var output = new List<Frame>();
    foreach (var value in wire)
    {
        output.AddRange(decoder.Append([value]));
    }
    Equal(1, output.Count, "split frame count");
    decoder.Finish();
}

static void CoalescedFrames()
{
    var vectors = LoadManifest().Vectors;
    var first = Convert.FromHexString(vectors[1].FrameHex);
    var second = Convert.FromHexString(vectors[2].FrameHex);
    var wire = first.Concat(second).ToArray();
    var decoder = new FrameDecoder();
    var output = decoder.Append(wire);
    Equal(2, output.Count, "coalesced frame count");
    Equal(MessageType.Ping, output[0].Type, "first type");
    Equal(MessageType.Disconnect, output[1].Type, "second type");
    decoder.Finish();
}

static void InvalidMagic()
{
    var wire = Convert.FromHexString(LoadManifest().Vectors[0].FrameHex);
    wire[0] = 0;
    Throws(ProtocolError.InvalidMagic, () => new FrameDecoder().Append(wire));
}

static void OversizedPayload()
{
    var wire = Convert.FromHexString(LoadManifest().Vectors[1].FrameHex);
    wire[12] = 0;
    wire[13] = 1;
    wire[14] = 0;
    wire[15] = 1;
    Throws(ProtocolError.MessageTooLarge, () => new FrameDecoder().Append(wire.AsSpan(0, 28)));
}

static void IncompleteFrame()
{
    var wire = Convert.FromHexString(LoadManifest().Vectors[0].FrameHex);
    var decoder = new FrameDecoder();
    Equal(0, decoder.Append(wire.AsSpan(0, wire.Length - 1)).Count, "premature decode");
    Throws(ProtocolError.IncompleteFrame, decoder.Finish);
}

static void AuthenticationGoldenVector()
{
    var vector = LoadAuthVector();
    var expected = Convert.FromHexString(vector.ResponseHex);
    var response = Authentication.CreateResponse(
        Convert.FromHexString(vector.DeviceKeyHex),
        Convert.FromHexString(vector.ControllerNonceHex),
        Convert.FromHexString(vector.AgentNonceHex),
        Convert.FromHexString(vector.ChallengeHex),
        Convert.FromHexString(vector.AgentIdentifierHex));

    SequenceEqual(expected, response, "authentication response");
    Equal(true, Authentication.ConstantTimeEquals(expected, response), "constant-time equal");
    Equal(false, Authentication.ConstantTimeEquals(expected, new byte[32]), "constant-time unequal");
}

static void HelloRoundTrip()
{
    var hello = new HelloPayload(
        PeerRole.Controller,
        ProtocolConstants.Version,
        ProtocolConstants.Version,
        Capabilities.Jpeg | Capabilities.Reconnect,
        Enumerable.Range(0, 32).Select(value => (byte)value).ToArray());
    var decoded = HelloPayload.Decode(hello.Encode());

    Equal(hello.Role, decoded.Role, "hello role");
    Equal(hello.MinimumVersion, decoded.MinimumVersion, "hello minimum version");
    Equal(hello.MaximumVersion, decoded.MaximumVersion, "hello maximum version");
    Equal(hello.Capabilities, decoded.Capabilities, "hello capabilities");
    SequenceEqual(hello.Nonce, decoded.Nonce, "hello nonce");
}

static void MalformedHandshakePayloads()
{
    Throws(ProtocolError.InvalidPayload, () => HelloPayload.Decode(new byte[40]));
    Throws(ProtocolError.InvalidPayload, () => AuthChallengePayload.Decode(new byte[47]));
    Throws(ProtocolError.InvalidPayload, () => AuthResultPayload.Decode([0, 0, 0, 0, 1]));
}

static void VideoBeforeAuthentication()
{
    var gate = new SessionGate(PeerRole.Controller);
    Throws(ProtocolError.AuthRequired, () => gate.Receive(new Frame(
        MessageType.VideoFrameJpeg, 0, 1, 0, [0xff, 0xd8])));
}

static void ControllerAuthenticationTransition()
{
    var gate = new SessionGate(PeerRole.Controller);
    gate.Receive(new Frame(MessageType.Hello, 0, 1, 0, new HelloPayload(
        PeerRole.Agent,
        ProtocolConstants.Version,
        ProtocolConstants.Version,
        Capabilities.Jpeg,
        Enumerable.Repeat((byte)1, 32).ToArray()).Encode()));
    Equal(SessionPhase.Authenticating, gate.Phase, "controller authenticating");

    gate.Receive(new Frame(MessageType.AuthChallenge, 0, 2, 0, new AuthChallengePayload(
        Enumerable.Repeat((byte)2, 32).ToArray(),
        Enumerable.Repeat((byte)3, 16).ToArray()).Encode()));
    gate.Receive(new Frame(MessageType.AuthResult, 0, 3, 0,
        new AuthResultPayload(AuthResultStatus.Success, 0).Encode()));

    Equal(SessionPhase.Authenticated, gate.Phase, "controller authenticated");
    gate.Receive(new Frame(MessageType.VideoFrameJpeg, 0, 4, 0, []));
}

static void AgentAuthenticationTransition()
{
    var gate = new SessionGate(PeerRole.Agent);
    gate.Receive(new Frame(MessageType.Hello, 0, 1, 0, new HelloPayload(
        PeerRole.Controller,
        ProtocolConstants.Version,
        ProtocolConstants.Version,
        Capabilities.Jpeg,
        Enumerable.Repeat((byte)4, 32).ToArray()).Encode()));
    Throws(ProtocolError.InvalidState, () => gate.CompleteAgentAuthentication(
        Enumerable.Repeat((byte)5, 32).ToArray()));

    gate.Receive(new Frame(MessageType.AuthResponse, 0, 2, 0,
        Enumerable.Repeat((byte)5, 32).ToArray()));
    gate.CompleteAgentAuthentication(Enumerable.Repeat((byte)5, 32).ToArray());

    Equal(SessionPhase.Authenticated, gate.Phase, "agent authenticated");

    var rejectedGate = AgentGateWithResponse(6);
    rejectedGate.CompleteAgentAuthentication(Enumerable.Repeat((byte)7, 32).ToArray());
    Equal(SessionPhase.Closing, rejectedGate.Phase, "agent rejects incorrect response");
}

static SessionGate AgentGateWithResponse(byte responseByte)
{
    var gate = new SessionGate(PeerRole.Agent);
    gate.Receive(new Frame(MessageType.Hello, 0, 1, 0, new HelloPayload(
        PeerRole.Controller,
        ProtocolConstants.Version,
        ProtocolConstants.Version,
        Capabilities.Jpeg,
        Enumerable.Repeat((byte)4, 32).ToArray()).Encode()));
    gate.Receive(new Frame(MessageType.AuthResponse, 0, 2, 0,
        Enumerable.Repeat(responseByte, 32).ToArray()));
    return gate;
}

static void Equal<T>(T expected, T actual, string context)
{
    if (!EqualityComparer<T>.Default.Equals(expected, actual))
    {
        throw new InvalidOperationException($"{context}: expected {expected}, got {actual}");
    }
}

static void SequenceEqual(byte[] expected, byte[] actual, string context)
{
    if (!expected.AsSpan().SequenceEqual(actual))
    {
        throw new InvalidOperationException($"{context}: byte sequences differ");
    }
}

static void Throws(ProtocolError expected, Action action)
{
    try
    {
        action();
        throw new InvalidOperationException($"Expected protocol error {expected}.");
    }
    catch (ProtocolException exception) when (exception.Error == expected)
    {
    }
}

internal sealed record Manifest(List<GoldenVector> Vectors);

internal sealed record GoldenVector(
    string Name,
    string FrameHex,
    ushort MessageType,
    ushort Flags,
    int PayloadLength,
    uint Sequence,
    ulong TimestampMicros,
    string PayloadHex);

internal sealed record AuthVector(
    string DeviceKeyHex,
    string ControllerNonceHex,
    string AgentNonceHex,
    string ChallengeHex,
    string AgentIdentifierHex,
    string ResponseHex);
