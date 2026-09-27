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

static void Equal<T>(T expected, T actual, string context) where T : IEquatable<T>
{
    if (!expected.Equals(actual))
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
