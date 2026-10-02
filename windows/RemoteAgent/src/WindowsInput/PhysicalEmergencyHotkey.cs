namespace RemoteAgent;

public static class PhysicalEmergencyHotkey
{
    public const uint EscapeVirtualKey = 0x1b;
    public const uint InjectedFlag = 0x10;
    public const uint LowerIntegrityInjectedFlag = 0x02;

    public static bool Matches(uint virtualKey, uint flags, bool controlDown, bool altDown) =>
        virtualKey == EscapeVirtualKey &&
        (flags & (InjectedFlag | LowerIntegrityInjectedFlag)) == 0 &&
        controlDown && altDown;
}


// Track only non-injected transitions; GetAsyncKeyState includes remote SendInput.
public sealed class PhysicalEmergencyChord
{
    private readonly HashSet<uint> held = [];
    public bool ControlDown => held.Contains(0xa2) || held.Contains(0xa3) || held.Contains(0x11);
    public bool AltDown => held.Contains(0xa4) || held.Contains(0xa5) || held.Contains(0x12);

    public bool Process(uint key, uint flags, bool down)
    {
        if ((flags & (PhysicalEmergencyHotkey.InjectedFlag |
            PhysicalEmergencyHotkey.LowerIntegrityInjectedFlag)) != 0) return false;
        if (key is not (0xa2 or 0xa3 or 0x11 or 0xa4 or 0xa5 or 0x12 or 0x1b)) return false;
        if (!down) { held.Remove(key); return false; }
        var firstDown = held.Add(key);
        return firstDown && PhysicalEmergencyHotkey.Matches(key, flags, ControlDown, AltDown);
    }
}
