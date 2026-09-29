using System.Text.Json;
using RemoteProtocol;

internal static class KeyboardTests
{
    private static JsonDocument Vectors() => JsonDocument.Parse(File.ReadAllText(Path.Combine(AppContext.BaseDirectory, "testdata", "keyboard-v1.json")));
    private static void Check(bool condition) { if (!condition) throw new Exception("Keyboard foundation assertion failed."); }
    private static void Reject(Action action) { try { action(); } catch (ProtocolException) { return; } throw new Exception("Invalid keyboard input accepted."); }
    private static Frame F(MessageType type, byte[] payload) => new(type, 0, 1, 0, payload);
    private static Frame Key(ushort code, bool extended = false, KeyAction action = KeyAction.Down) =>
        F(MessageType.KeyEvent, new KeyEventPayload(code, extended, action).Encode());
    private static SessionGate Authenticated()
    {
        var gate = new SessionGate(PeerRole.Agent);
        gate.Receive(F(MessageType.Hello, new HelloPayload(PeerRole.Controller, 1, 1, Capabilities.Input, new byte[32]).Encode()));
        gate.Receive(F(MessageType.AuthResponse, new byte[32]));
        gate.CompleteAgentAuthentication(new byte[32]); // Isolated synthetic handshake, not production credentials.
        return gate;
    }
    public static void Golden()
    {
        using var doc = Vectors();
        foreach (var v in doc.RootElement.GetProperty("valid").EnumerateArray())
        {
            var bytes = Convert.FromHexString(v.GetProperty("hex").GetString()!);
            var key = KeyEventPayload.Decode(bytes);
            Check(key.ScanCode == v.GetProperty("scanCode").GetUInt16() &&
                key.Extended == v.GetProperty("extended").GetBoolean() &&
                (byte)key.Action == v.GetProperty("action").GetByte());
            Check(key.Encode().SequenceEqual(bytes));
        }
    }
    public static void Malformed()
    {
        using var doc = Vectors();
        foreach (var v in doc.RootElement.GetProperty("invalidHex").EnumerateArray())
            Reject(() => KeyEventPayload.Decode(Convert.FromHexString(v.GetString()!)));
        foreach (var code in new ushort[] { 0, 0x80, 0xe01d, ushort.MaxValue })
            Reject(() => new KeyEventPayload(code, false, KeyAction.Down).Encode());
        Reject(() => new KeyEventPayload(30, false, (KeyAction)0).Encode());
    }
    public static void Gates()
    {
        var sink = new FakeSink();
        using var unauth = new InputDispatcher(new SessionGate(PeerRole.Agent), sink, true);
        Reject(unauth.GrantLocalControl);
        Reject(() => unauth.Apply(Key(30)));
        using var unnegotiated = new InputDispatcher(Authenticated(), sink);
        Reject(unnegotiated.GrantLocalControl);
        Reject(() => unnegotiated.Apply(Key(30)));
        using var disabled = new InputDispatcher(Authenticated(), sink, true);
        Reject(() => disabled.Apply(Key(30)));
        var closing = Authenticated();
        closing.Receive(F(MessageType.Disconnect, [0, 0]));
        using var ended = new InputDispatcher(closing, sink, true);
        Reject(ended.GrantLocalControl);
        Reject(() => ended.Apply(Key(30)));
        Check(sink.KeyEvents == 0 && sink.Held.Count == 0);
    }
    public static void RepeatAndExtended()
    {
        var sink = new FakeSink();
        using var dispatcher = new InputDispatcher(Authenticated(), sink, true);
        dispatcher.GrantLocalControl();
        dispatcher.Apply(Key(30, false, KeyAction.Up)); // An unmatched release is ignored.
        dispatcher.Apply(Key(30)); dispatcher.Apply(Key(30)); dispatcher.Apply(Key(30));
        dispatcher.Apply(Key(29)); dispatcher.Apply(Key(29, true));
        Check(sink.KeyEvents == 5 && sink.Held.Count == 3);
        dispatcher.Apply(Key(29, false, KeyAction.Up));
        Check(!sink.Held.Contains((29, false)) && sink.Held.Contains((29, true)));
        dispatcher.Apply(Key(30, false, KeyAction.Up));
        dispatcher.Apply(Key(30, false, KeyAction.Up));
        Check(sink.KeyEvents == 7 && sink.Held.Count == 1);
        dispatcher.RevokeControl();
        Check(sink.Held.Count == 0 && sink.KeyReleaseAttempts == 1);
    }
    public static void Lifecycle()
    {
        foreach (var ending in new[] { "stop", "disconnect", "error", "invalid", "dispose" })
        {
            var sink = new FakeSink();
            var dispatcher = new InputDispatcher(Authenticated(), sink, true);
            dispatcher.GrantLocalControl();
            dispatcher.Apply(Key(29)); dispatcher.Apply(Key(42)); dispatcher.Apply(Key(56)); dispatcher.Apply(Key(30));
            dispatcher.Apply(F(MessageType.MouseButton, [1, 1]));
            if (ending == "stop") dispatcher.RevokeControl();
            if (ending == "disconnect") dispatcher.Apply(F(MessageType.Disconnect, [0, 0]));
            if (ending == "error") dispatcher.Apply(F(MessageType.Error, [0, 1]));
            if (ending == "invalid") Reject(() => dispatcher.Apply(F(MessageType.KeyEvent, [0, 30, 2, 1])));
            if (ending == "dispose") dispatcher.Dispose();
            Check(sink.Held.Count == 0 && !sink.MouseHeld && sink.KeyReleaseAttempts == 1 && sink.MouseReleaseAttempts == 1);
            Reject(() => dispatcher.Apply(Key(30)));
            dispatcher.Dispose(); dispatcher.Dispose();
            Check(sink.KeyReleaseAttempts == 1 && sink.MouseReleaseAttempts == 1);
        }
    }
    public static void SinkFailures()
    {
        var sink = new FakeSink { FailKey = true };
        using (var dispatcher = new InputDispatcher(Authenticated(), sink, true))
        {
            dispatcher.GrantLocalControl();
            try { dispatcher.Apply(Key(30)); throw new Exception("Expected sink failure."); }
            catch (InvalidOperationException) { }
            Check(sink.Held.Count == 0 && sink.KeyReleaseAttempts == 1);
            Reject(() => dispatcher.Apply(Key(30)));
        }
        foreach (var failKeyboard in new[] { true, false })
        {
            sink = new FakeSink();
            var dispatcher = new InputDispatcher(Authenticated(), sink, true);
            dispatcher.GrantLocalControl();
            dispatcher.Apply(Key(29));
            dispatcher.Apply(F(MessageType.MouseButton, [1, 1]));
            sink.FailKeyRelease = failKeyboard;
            sink.FailMouseRelease = !failKeyboard;
            try { dispatcher.RevokeControl(); throw new Exception("Expected cleanup failure."); }
            catch (AggregateException) { }
            Check(sink.KeyReleaseAttempts == 1 && sink.MouseReleaseAttempts == 1);
            Reject(dispatcher.GrantLocalControl);
            sink.FailKeyRelease = false; sink.FailMouseRelease = false;
            dispatcher.Dispose(); dispatcher.Dispose();
            Check(sink.Held.Count == 0 && !sink.MouseHeld);
            Check(sink.KeyReleaseAttempts == (failKeyboard ? 2 : 1));
            Check(sink.MouseReleaseAttempts == (failKeyboard ? 1 : 2));
        }
    }
    public static void Framing()
    {
        var frames = new[] {
            new Frame(MessageType.KeyEvent, 0, 7, 0, new KeyEventPayload(30, false, KeyAction.Down).Encode()),
            new Frame(MessageType.KeyEvent, 0, 8, 0, new KeyEventPayload(30, false, KeyAction.Up).Encode())
        };
        var wire = frames.SelectMany(FrameCodec.Encode).ToArray();
        var decoder = new FrameDecoder();
        var decoded = new List<Frame>();
        foreach (var b in wire) decoded.AddRange(decoder.Append([b]));
        decoder.Finish();
        Check(decoded.Count == 2 && decoded[0].Sequence == 7 && decoded[1].Sequence == 8);
        Check(KeyEventPayload.Decode(decoded[1].Payload).Action == KeyAction.Up);
    }
    private sealed class FakeSink : IInputSink
    {
        public readonly HashSet<(ushort, bool)> Held = [];
        public int KeyEvents, KeyReleaseAttempts, MouseReleaseAttempts;
        public bool MouseHeld, FailKey, FailKeyRelease, FailMouseRelease;
        public void Key(KeyEventPayload key)
        {
            KeyEvents++;
            if (key.Action == KeyAction.Down) Held.Add((key.ScanCode, key.Extended));
            else Held.Remove((key.ScanCode, key.Extended));
            if (FailKey) throw new InvalidOperationException("Synthetic key sink failure.");
        }
        public void ReleaseAllKeys()
        {
            KeyReleaseAttempts++;
            if (FailKeyRelease) throw new InvalidOperationException("Synthetic keyboard cleanup failure.");
            Held.Clear();
        }
        public void ReleaseAllButtons()
        {
            MouseReleaseAttempts++;
            if (FailMouseRelease) throw new InvalidOperationException("Synthetic mouse cleanup failure.");
            MouseHeld = false;
        }
        public void Move(MouseMovePayload point) { }
        public void Wheel(MouseWheelPayload delta) { }
        public void Button(MouseButtonPayload button) { MouseHeld = button.Action == ButtonAction.Down; }
    }
}
