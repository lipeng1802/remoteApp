using System.ComponentModel;
using System.Runtime.InteropServices;
using RemoteProtocol;

internal static class CredentialStoreTests
{
    public static void RoundTrip()
    {
        if (!OperatingSystem.IsWindows()) throw new PlatformNotSupportedException();
        var target = "PersonalRemoteDesktop/Tests/" + Guid.NewGuid().ToString("N");
        try
        {
            using var first = AgentCredentialStore.LoadOrCreate(target);
            using var second = AgentCredentialStore.LoadOrCreate(target);
            if (first.DeviceKey.Length != 32 || first.AgentIdentifier.Length != 16 ||
                !first.DeviceKey.AsSpan().SequenceEqual(second.DeviceKey) ||
                !first.AgentIdentifier.AsSpan().SequenceEqual(second.AgentIdentifier))
                throw new Exception("Credential identity did not persist.");
            first.Dispose();
            if (first.DeviceKey.Any(value => value != 0)) throw new Exception("Disposed key was not cleared.");
            if (second.DeviceKey.All(value => value == 0)) throw new Exception("Loaded keys alias mutable storage.");
        }
        finally
        {
            if (!CredDelete(target, 1, 0) && Marshal.GetLastWin32Error() != 1168)
                throw new Win32Exception(Marshal.GetLastWin32Error());
        }
    }
    [DllImport("advapi32.dll", EntryPoint = "CredDeleteW", CharSet = CharSet.Unicode, SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool CredDelete(string target, uint type, uint flags);
}