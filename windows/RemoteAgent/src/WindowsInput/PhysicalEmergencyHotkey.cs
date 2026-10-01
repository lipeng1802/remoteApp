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
