using RemoteAgent;
using RemoteProtocol;

var tests = new (string Name, Action Run)[]
{
    ("absolute mouse movement preserves normalized coordinates", AbsoluteMove),
    ("mouse buttons map and release in stable order", Buttons),
    ("horizontal and vertical wheel deltas stay signed", Wheel),
    ("scan-code keyboard flags include extended and key-up", Keyboard),
    ("release attempts every held input and can retry failures", CleanupRetry),
    ("win-x64 SendInput ABI layout is stable", NativeLayout),
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

static void AbsoluteMove()
{
    var api = new FakeApi();
    var sink = new WindowsInputSink(api);
    sink.Move(new MouseMovePayload(1234, 65000));
    Equal(new WindowsMouseEvent(1234, 65000, 0,
        WindowsMouseFlags.Move | WindowsMouseFlags.Absolute), api.Events.Single());
}

static void Buttons()
{
    var api = new FakeApi();
    var sink = new WindowsInputSink(api);
    sink.Button(new MouseButtonPayload(MouseButton.Right, ButtonAction.Down));
    sink.Button(new MouseButtonPayload(MouseButton.Left, ButtonAction.Down));
    sink.ReleaseAllButtons();
    Equal(new WindowsMouseEvent(0, 0, 0, WindowsMouseFlags.RightDown), api.Events[0]);
    Equal(new WindowsMouseEvent(0, 0, 0, WindowsMouseFlags.LeftDown), api.Events[1]);
    Equal(new WindowsMouseEvent(0, 0, 0, WindowsMouseFlags.LeftUp), api.Events[2]);
    Equal(new WindowsMouseEvent(0, 0, 0, WindowsMouseFlags.RightUp), api.Events[3]);
    var count = api.Events.Count;
    sink.ReleaseAllButtons();
    Equal(count, api.Events.Count);
}

static void Wheel()
{
    var api = new FakeApi();
    var sink = new WindowsInputSink(api);
    sink.Wheel(new MouseWheelPayload(-120, 240));
    Equal(new WindowsMouseEvent(0, 0, -120, WindowsMouseFlags.HorizontalWheel), api.Events[0]);
    Equal(new WindowsMouseEvent(0, 0, 240, WindowsMouseFlags.Wheel), api.Events[1]);
}

static void Keyboard()
{
    var api = new FakeApi();
    var sink = new WindowsInputSink(api);
    sink.Key(new KeyEventPayload(0x1d, true, KeyAction.Down));
    sink.Key(new KeyEventPayload(0x1d, true, KeyAction.Down));
    sink.ReleaseAllKeys();
    Equal(new WindowsKeyboardEvent(0x1d,
        WindowsKeyboardFlags.ScanCode | WindowsKeyboardFlags.ExtendedKey), api.Events[0]);
    Equal(api.Events[0], api.Events[1]);
    Equal(new WindowsKeyboardEvent(0x1d,
        WindowsKeyboardFlags.ScanCode | WindowsKeyboardFlags.ExtendedKey | WindowsKeyboardFlags.KeyUp), api.Events[2]);
}

static void CleanupRetry()
{
    var api = new FakeApi();
    var sink = new WindowsInputSink(api);
    sink.Key(new KeyEventPayload(0x1e, false, KeyAction.Down));
    sink.Key(new KeyEventPayload(0x30, false, KeyAction.Down));
    sink.Button(new MouseButtonPayload(MouseButton.Left, ButtonAction.Down));
    sink.Button(new MouseButtonPayload(MouseButton.Right, ButtonAction.Down));
    api.FailOnce = input => input is WindowsKeyboardEvent { ScanCode: 0x1e, Flags: var flags } &&
        flags.HasFlag(WindowsKeyboardFlags.KeyUp);
    Throws<AggregateException>(sink.ReleaseAllKeys);
    api.FailOnce = input => input is WindowsMouseEvent { Flags: WindowsMouseFlags.LeftUp };
    Throws<AggregateException>(sink.ReleaseAllButtons);
    api.FailOnce = null;
    sink.ReleaseAllKeys();
    sink.ReleaseAllButtons();
    var before = api.Events.Count;
    sink.ReleaseAllKeys();
    sink.ReleaseAllButtons();
    Equal(before, api.Events.Count);
}

static void NativeLayout()
{
    Equal(8, IntPtr.Size);
    Equal(40, NativeWindowsInputApi.InputSize);
    Equal(32, NativeWindowsInputApi.MouseInputSize);
    Equal(24, NativeWindowsInputApi.KeyboardInputSize);
    var mouse = NativeWindowsInputApi.Inspect(new WindowsMouseEvent(12, 34, -120,
        WindowsMouseFlags.Move | WindowsMouseFlags.Wheel));
    Equal(0u, mouse.Type);
    Equal(12, mouse.X);
    Equal(34, mouse.Y);
    Equal(unchecked((uint)-120), mouse.MouseData);
    Equal((uint)(WindowsMouseFlags.Move | WindowsMouseFlags.Wheel), mouse.MouseFlags);
    var keyboard = NativeWindowsInputApi.Inspect(new WindowsKeyboardEvent(0x1d,
        WindowsKeyboardFlags.ScanCode | WindowsKeyboardFlags.ExtendedKey | WindowsKeyboardFlags.KeyUp));
    Equal(1u, keyboard.Type);
    Equal((ushort)0x1d, keyboard.ScanCode);
    Equal((uint)(WindowsKeyboardFlags.ScanCode | WindowsKeyboardFlags.ExtendedKey |
        WindowsKeyboardFlags.KeyUp), keyboard.KeyboardFlags);
}

static void Equal<T>(T expected, T actual)
{
    if (!EqualityComparer<T>.Default.Equals(expected, actual))
        throw new Exception($"Expected {expected}; actual {actual}.");
}

static void Throws<T>(Action action) where T : Exception
{
    try { action(); }
    catch (T) { return; }
    throw new Exception($"Expected {typeof(T).Name}.");
}

sealed class FakeApi : IWindowsInputApi
{
    public List<WindowsInputEvent> Events { get; } = [];
    public Func<WindowsInputEvent, bool>? FailOnce { get; set; }

    public void Send(WindowsInputEvent input)
    {
        if (FailOnce?.Invoke(input) == true)
        {
            FailOnce = null;
            throw new InvalidOperationException("Synthetic SendInput failure.");
        }
        Events.Add(input);
    }
}
