using System.ComponentModel;
using System.Runtime.InteropServices;
using RemoteProtocol;

namespace RemoteAgent;

[Flags]
internal enum WindowsMouseFlags : uint
{
    Move = 0x0001,
    LeftDown = 0x0002,
    LeftUp = 0x0004,
    RightDown = 0x0008,
    RightUp = 0x0010,
    MiddleDown = 0x0020,
    MiddleUp = 0x0040,
    Wheel = 0x0800,
    HorizontalWheel = 0x1000,
    Absolute = 0x8000,
}

[Flags]
internal enum WindowsKeyboardFlags : uint
{
    ExtendedKey = 0x0001,
    KeyUp = 0x0002,
    ScanCode = 0x0008,
}

internal abstract record WindowsInputEvent;
internal sealed record WindowsMouseEvent(int X, int Y, int Data, WindowsMouseFlags Flags) : WindowsInputEvent;
internal sealed record WindowsKeyboardEvent(ushort ScanCode, WindowsKeyboardFlags Flags) : WindowsInputEvent;

internal interface IWindowsInputApi
{
    void Send(WindowsInputEvent input);
}

// This class only translates already-authorized protocol input. Authentication,
// capability negotiation and explicit local consent remain the caller's duty.
public sealed class WindowsInputSink : IInputSink
{
    private readonly IWindowsInputApi api;
    private readonly HashSet<MouseButton> heldButtons = [];
    private readonly HashSet<(ushort ScanCode, bool Extended)> heldKeys = [];

    public WindowsInputSink() : this(new NativeWindowsInputApi()) { }

    internal WindowsInputSink(IWindowsInputApi api)
    {
        this.api = api ?? throw new ArgumentNullException(nameof(api));
    }

    public void Move(MouseMovePayload point) =>
        api.Send(new WindowsMouseEvent(point.X, point.Y, 0,
            WindowsMouseFlags.Move | WindowsMouseFlags.Absolute));

    public void Button(MouseButtonPayload button)
    {
        var flags = (button.Button, button.Action) switch
        {
            (MouseButton.Left, ButtonAction.Down) => WindowsMouseFlags.LeftDown,
            (MouseButton.Left, ButtonAction.Up) => WindowsMouseFlags.LeftUp,
            (MouseButton.Right, ButtonAction.Down) => WindowsMouseFlags.RightDown,
            (MouseButton.Right, ButtonAction.Up) => WindowsMouseFlags.RightUp,
            (MouseButton.Middle, ButtonAction.Down) => WindowsMouseFlags.MiddleDown,
            (MouseButton.Middle, ButtonAction.Up) => WindowsMouseFlags.MiddleUp,
            _ => throw new ArgumentOutOfRangeException(nameof(button)),
        };
        api.Send(new WindowsMouseEvent(0, 0, 0, flags));
        if (button.Action == ButtonAction.Down) heldButtons.Add(button.Button);
        else heldButtons.Remove(button.Button);
    }

    public void Wheel(MouseWheelPayload delta)
    {
        if (delta.Horizontal != 0)
            api.Send(new WindowsMouseEvent(0, 0, delta.Horizontal, WindowsMouseFlags.HorizontalWheel));
        if (delta.Vertical != 0)
            api.Send(new WindowsMouseEvent(0, 0, delta.Vertical, WindowsMouseFlags.Wheel));
    }

    public void Key(KeyEventPayload key)
    {
        var flags = WindowsKeyboardFlags.ScanCode;
        if (key.Extended) flags |= WindowsKeyboardFlags.ExtendedKey;
        if (key.Action == KeyAction.Up) flags |= WindowsKeyboardFlags.KeyUp;
        api.Send(new WindowsKeyboardEvent(key.ScanCode, flags));
        var identity = (key.ScanCode, key.Extended);
        if (key.Action == KeyAction.Down) heldKeys.Add(identity);
        else heldKeys.Remove(identity);
    }

    public void ReleaseAllKeys()
    {
        List<Exception>? errors = null;
        foreach (var key in heldKeys.OrderBy(value => value.ScanCode).ThenBy(value => value.Extended).ToArray())
        {
            try
            {
                var flags = WindowsKeyboardFlags.ScanCode | WindowsKeyboardFlags.KeyUp;
                if (key.Extended) flags |= WindowsKeyboardFlags.ExtendedKey;
                api.Send(new WindowsKeyboardEvent(key.ScanCode, flags));
                heldKeys.Remove(key);
            }
            catch (Exception exception)
            {
                (errors ??= []).Add(exception);
            }
        }
        if (errors is not null) throw new AggregateException("Failed to release one or more keys.", errors);
    }

    public void ReleaseAllButtons()
    {
        List<Exception>? errors = null;
        foreach (var button in heldButtons.OrderBy(value => value).ToArray())
        {
            try
            {
                Button(new MouseButtonPayload(button, ButtonAction.Up));
            }
            catch (Exception exception)
            {
                (errors ??= []).Add(exception);
            }
        }
        if (errors is not null) throw new AggregateException("Failed to release one or more mouse buttons.", errors);
    }
}

internal sealed class NativeWindowsInputApi : IWindowsInputApi
{
    internal static int InputSize => Marshal.SizeOf<Input>();
    internal static int MouseInputSize => Marshal.SizeOf<MouseInput>();
    internal static int KeyboardInputSize => Marshal.SizeOf<KeyboardInput>();

    internal static NativeInputView Inspect(WindowsInputEvent input)
    {
        var native = CreateInput(input);
        return native.Type == 0
            ? new(native.Type, native.Data.Mouse.X, native.Data.Mouse.Y, native.Data.Mouse.MouseData,
                native.Data.Mouse.Flags, 0, 0)
            : new(native.Type, 0, 0, 0, 0, native.Data.Keyboard.ScanCode, native.Data.Keyboard.Flags);
    }

    public void Send(WindowsInputEvent input)
    {
        var native = CreateInput(input);
        if (SendInput(1, [native], InputSize) != 1)
            throw new Win32Exception(Marshal.GetLastWin32Error(), "Windows SendInput rejected an input event.");
    }

    private static Input CreateInput(WindowsInputEvent input) => input switch
        {
            WindowsMouseEvent mouse => new Input
            {
                Type = 0,
                Data = new InputUnion
                {
                    Mouse = new MouseInput
                    {
                        X = mouse.X,
                        Y = mouse.Y,
                        MouseData = unchecked((uint)mouse.Data),
                        Flags = (uint)mouse.Flags,
                    },
                },
            },
            WindowsKeyboardEvent keyboard => new Input
            {
                Type = 1,
                Data = new InputUnion
                {
                    Keyboard = new KeyboardInput
                    {
                        VirtualKey = 0,
                        ScanCode = keyboard.ScanCode,
                        Flags = (uint)keyboard.Flags,
                    },
                },
            },
            _ => throw new ArgumentOutOfRangeException(nameof(input)),
        };

    [StructLayout(LayoutKind.Sequential)]
    private struct Input
    {
        public uint Type;
        public InputUnion Data;
    }

    [StructLayout(LayoutKind.Explicit)]
    private struct InputUnion
    {
        [FieldOffset(0)] public MouseInput Mouse;
        [FieldOffset(0)] public KeyboardInput Keyboard;
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct MouseInput
    {
        public int X;
        public int Y;
        public uint MouseData;
        public uint Flags;
        public uint Time;
        public nuint ExtraInfo;
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct KeyboardInput
    {
        public ushort VirtualKey;
        public ushort ScanCode;
        public uint Flags;
        public uint Time;
        public nuint ExtraInfo;
    }

    [DllImport("user32.dll", SetLastError = true)]
    private static extern uint SendInput(uint inputCount, [In] Input[] inputs, int inputSize);
}

internal readonly record struct NativeInputView(
    uint Type, int X, int Y, uint MouseData, uint MouseFlags, ushort ScanCode, uint KeyboardFlags);
