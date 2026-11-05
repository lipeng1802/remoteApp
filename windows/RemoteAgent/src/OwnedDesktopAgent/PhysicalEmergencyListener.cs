using System.Runtime.InteropServices;

namespace RemoteAgent;

// Runs on the WPF message thread. Injected key transitions cannot authorize a
// local emergency stop. Keep the delegate alive until the hook is unregistered.
internal sealed class PhysicalEmergencyListener : IDisposable
{
    private readonly PhysicalEmergencyChord chord = new();
    private readonly HookProcedure callback;
    private readonly Action stop;
    private nint hook;
    internal bool Available => hook != 0;
    internal PhysicalEmergencyListener(Action stop)
    {
        this.stop = stop;
        callback = Process;
        hook = SetWindowsHookEx(13, callback, GetModuleHandle(null), 0);
    }
    private nint Process(int code, nint message, nint pointer)
    {
        if (code >= 0 && (message == 0x100 || message == 0x104 || message == 0x101 || message == 0x105))
        {
            var data = Marshal.PtrToStructure<KeyData>(pointer);
            if (chord.Process(data.Key, data.Flags, message == 0x100 || message == 0x104)) stop();
        }
        return CallNextHookEx(hook, code, message, pointer);
    }
    public void Dispose()
    {
        if (hook != 0)
        {
            if (!UnhookWindowsHookEx(hook)) throw new InvalidOperationException("Emergency hook join failed.");
            hook = 0;
        }
        GC.KeepAlive(callback);
    }
    [StructLayout(LayoutKind.Sequential)]
    private struct KeyData { internal uint Key, Scan, Flags, Time; internal nuint Extra; }
    private delegate nint HookProcedure(int code, nint message, nint pointer);
    [DllImport("user32.dll", SetLastError = true)]
    private static extern nint SetWindowsHookEx(int id, HookProcedure callback, nint module, uint thread);
    [DllImport("user32.dll")]
    private static extern nint CallNextHookEx(nint hook, int code, nint message, nint pointer);
    [DllImport("user32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)] private static extern bool UnhookWindowsHookEx(nint hook);
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode)] private static extern nint GetModuleHandle(string? module);
}
