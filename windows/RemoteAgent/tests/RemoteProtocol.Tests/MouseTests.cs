using System.Text.Json;
using RemoteProtocol;

internal static class MouseTests
{
    private static JsonDocument Vectors() => JsonDocument.Parse(File.ReadAllText(Path.Combine(AppContext.BaseDirectory, "testdata", "mouse-v1.json")));
    private static void Check(bool condition) { if (!condition) throw new Exception("Mouse foundation assertion failed."); }
    private static void Reject(Action action) { try { action(); } catch (ProtocolException) { return; } throw new Exception("Invalid input accepted."); }
    private static Frame F(MessageType type, byte[] payload) => new(type, 0, 1, 0, payload);

    public static void Golden()
    {
        using var doc = Vectors();
        foreach (var v in doc.RootElement.GetProperty("move").EnumerateArray())
        {
            var bytes = Convert.FromHexString(v.GetProperty("hex").GetString()!);
            var value = MouseMovePayload.Decode(bytes);
            Check(value.X == v.GetProperty("x").GetUInt16() && value.Y == v.GetProperty("y").GetUInt16());
            Check(value.Encode().SequenceEqual(bytes));
        }
        foreach (var v in doc.RootElement.GetProperty("button").EnumerateArray())
        {
            var bytes = Convert.FromHexString(v.GetProperty("hex").GetString()!);
            var value = MouseButtonPayload.Decode(bytes);
            Check((byte)value.Button == v.GetProperty("button").GetByte() && (byte)value.Action == v.GetProperty("action").GetByte());
            Check(value.Encode().SequenceEqual(bytes));
        }
        foreach (var v in doc.RootElement.GetProperty("wheel").EnumerateArray())
        {
            var bytes = Convert.FromHexString(v.GetProperty("hex").GetString()!);
            var value = MouseWheelPayload.Decode(bytes);
            Check(value.Horizontal == v.GetProperty("horizontal").GetInt32() && value.Vertical == v.GetProperty("vertical").GetInt32());
            Check(value.Encode().SequenceEqual(bytes));
        }
    }
    public static void Invalid()
    {
        foreach (var n in new[] { 0, 3, 5 }) Reject(() => MouseMovePayload.Decode(new byte[n]));
        foreach (var n in new[] { 0, 7, 9 }) Reject(() => MouseWheelPayload.Decode(new byte[n]));
        foreach (var bytes in new byte[][] { [], [1], [1, 1, 0], [0, 1], [4, 1], [1, 0], [1, 3] })
            Reject(() => MouseButtonPayload.Decode(bytes));
        Reject(() => new MouseButtonPayload((MouseButton)255, ButtonAction.Down).Encode());
    }
    public static void Mapping()
    {
        using var doc = Vectors();
        foreach (var v in doc.RootElement.GetProperty("mapping").EnumerateArray())
        {
            var success = MouseCoordinates.TryMap(v.GetProperty("x").GetDouble(), v.GetProperty("y").GetDouble(),
                v.GetProperty("viewWidth").GetDouble(), v.GetProperty("viewHeight").GetDouble(),
                v.GetProperty("screenWidth").GetInt32(), v.GetProperty("screenHeight").GetInt32(), out var point);
            var expected = v.GetProperty("hex");
            Check(success == (expected.ValueKind != JsonValueKind.Null));
            if (success) Check(Convert.ToHexString(point.Encode()).Equals(expected.GetString(), StringComparison.OrdinalIgnoreCase));
        }
        Check(!MouseCoordinates.TryMap(double.NaN, 1, 800, 800, 1920, 1080, out _));
        Check(!MouseCoordinates.TryMap(1, 1, double.PositiveInfinity, 800, 1920, 1080, out _));
        Check(!MouseCoordinates.TryMap(1, 1, 0, 800, 1920, 1080, out _));
        Check(!MouseCoordinates.TryMap(1, 1, 800, 800, 0, 1080, out _));
    }
    public static void Physical()
    {
        Check(new MouseMovePayload(0, 0).ToPhysicalPixels(3840, 2160) == (0, 0));
        Check(new MouseMovePayload(65535, 65535).ToPhysicalPixels(3840, 2160) == (3839, 2159));
        Check(new MouseMovePayload(32768, 32768).ToPhysicalPixels(3840, 2160) == (1920, 1080));
        Check(new MouseMovePayload(65535, 65535).ToPhysicalPixels(1, 1) == (0, 0));
        Reject(() => new MouseMovePayload(0, 0).ToPhysicalPixels(0, 1080));
    }
    private static SessionGate Authenticated()
    {
        var gate = new SessionGate(PeerRole.Agent);
        gate.Receive(F(MessageType.Hello, new HelloPayload(PeerRole.Controller, 1, 1, Capabilities.Input, new byte[32]).Encode()));
        // Isolated test gate; production must supply the verified HMAC from its handshake.
        gate.Receive(F(MessageType.AuthResponse, new byte[32]));
        gate.CompleteAgentAuthentication(new byte[32]);
        return gate;
    }
    public static void Authorization()
    {
        var sink = new FakeSink();
        using var unauth = new InputDispatcher(new SessionGate(PeerRole.Agent), sink, true);
        Reject(unauth.GrantLocalControl);
        Reject(() => unauth.Apply(F(MessageType.MouseMove, new MouseMovePayload(1, 2).Encode())));
        using var noCapability = new InputDispatcher(Authenticated(), sink);
        Reject(noCapability.GrantLocalControl);
        using var localOff = new InputDispatcher(Authenticated(), sink, true);
        Reject(() => localOff.Apply(F(MessageType.MouseMove, new MouseMovePayload(1, 2).Encode())));
        foreach (var type in new[] { MessageType.MouseButton, MessageType.MouseWheel })
            Reject(() => unauth.Apply(F(type, [])));
        Check(sink.Events.Count == 0);
        localOff.GrantLocalControl();
        localOff.Apply(F(MessageType.MouseMove, new MouseMovePayload(1, 2).Encode()));
        localOff.Apply(F(MessageType.MouseWheel, new MouseWheelPayload(-120, 240).Encode()));
        Check(sink.Events.SequenceEqual(new[] { "move:1,2", "wheel:-120,240" }));
        localOff.RevokeControl();
        Reject(() => localOff.Apply(F(MessageType.MouseMove, new MouseMovePayload(1, 2).Encode())));
        Check(sink.Events.Count == 2);
    }
    public static void Releases()
    {
        foreach (var ending in new[] { "stop", "disconnect", "invalid", "dispose" })
        {
            var sink = new FakeSink();
            using var dispatcher = new InputDispatcher(Authenticated(), sink, true);
            dispatcher.GrantLocalControl();
            var down = F(MessageType.MouseButton, new MouseButtonPayload(MouseButton.Left, ButtonAction.Down).Encode());
            dispatcher.Apply(down); dispatcher.Apply(down);
            Check(sink.Events.Count == 1); // No duplicated down transition.
            if (ending == "stop") dispatcher.RevokeControl();
            if (ending == "disconnect") dispatcher.Apply(F(MessageType.Disconnect, [0, 0]));
            if (ending == "invalid") Reject(() => dispatcher.Apply(F(MessageType.MouseButton, [9, 1])));
            if (ending == "dispose") dispatcher.Dispose();
            dispatcher.Dispose();
            Check(sink.Releases == 1);
            Reject(() => dispatcher.Apply(down));
        }
    }
    public static void BalancedButtons()
    {
        var sink = new FakeSink();
        using var dispatcher = new InputDispatcher(Authenticated(), sink, true);
        dispatcher.GrantLocalControl();
        var down = F(MessageType.MouseButton, new MouseButtonPayload(MouseButton.Right, ButtonAction.Down).Encode());
        var up = F(MessageType.MouseButton, new MouseButtonPayload(MouseButton.Right, ButtonAction.Up).Encode());
        dispatcher.Apply(up); // Ignore an unmatched release.
        dispatcher.Apply(down);
        dispatcher.Apply(up);
        dispatcher.Apply(up);
        dispatcher.RevokeControl();
        Check(sink.Events.Count == 2 && sink.Releases == 0);
    }
    private sealed class FakeSink : IInputSink
    {
        public List<string> Events { get; } = [];
        public int Releases;
        public void Key(KeyEventPayload key) => throw new Exception("Unexpected keyboard input in mouse test.");
        public void ReleaseAllKeys() => throw new Exception("Unexpected keyboard cleanup in mouse test.");
        public void Move(MouseMovePayload p) => Events.Add($"move:{p.X},{p.Y}");
        public void Button(MouseButtonPayload p) => Events.Add($"button:{p.Button},{p.Action}");
        public void Wheel(MouseWheelPayload p) => Events.Add($"wheel:{p.Horizontal},{p.Vertical}");
        public void ReleaseAllButtons() => Releases++;
    }
}
