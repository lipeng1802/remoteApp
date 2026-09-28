using System.ComponentModel;
using System.Runtime.InteropServices;
using System.Security.Cryptography;

namespace RemoteProtocol;

public sealed class AgentCredentials : IDisposable
{
    public byte[] DeviceKey { get; }
    public byte[] AgentIdentifier { get; }
    internal AgentCredentials(byte[] blob)
    {
        if (blob.Length != 48) throw new InvalidDataException("Invalid stored credential.");
        DeviceKey = blob[..32];
        AgentIdentifier = blob[32..];
    }
    public void Dispose() => CryptographicOperations.ZeroMemory(DeviceKey);
}

// Generic credential, current Windows user, persisted on this machine only.
public static class AgentCredentialStore
{
    public const string DefaultTarget = "PersonalRemoteDesktop/Agent/v1";
    public static AgentCredentials LoadOrCreate(string target = DefaultTarget)
    {
        if (!OperatingSystem.IsWindows()) throw new PlatformNotSupportedException();
        // Serialize initialization across probe processes so pairing never races key creation.
        using var mutex = new Mutex(false, "Local\\PersonalRemoteDesktop-AgentCredentials-v1");
        try { mutex.WaitOne(); } catch (AbandonedMutexException) { }
        try
        {
            if (CredRead(target, 1, 0, out var pointer))
            {
                try
                {
                    var credential = Marshal.PtrToStructure<NativeCredential>(pointer);
                    if (credential.BlobSize != 48) throw new InvalidDataException("Invalid stored credential size.");
                    var blob = new byte[48];
                    Marshal.Copy(credential.Blob, blob, 0, blob.Length);
                    try { return new AgentCredentials(blob); }
                    finally { CryptographicOperations.ZeroMemory(blob); }
                }
                finally { CredFree(pointer); }
            }
            var error = Marshal.GetLastWin32Error();
            if (error != 1168) throw new Win32Exception(error);
            var generated = RandomNumberGenerator.GetBytes(48);
            var memory = Marshal.AllocHGlobal(generated.Length);
            try
            {
                Marshal.Copy(generated, 0, memory, generated.Length);
                var credential = new NativeCredential {
                    Type = 1, TargetName = target, BlobSize = (uint)generated.Length,
                    Blob = memory, Persist = 2, UserName = "PersonalRemoteDesktop"
                };
                if (!CredWrite(ref credential, 0)) throw new Win32Exception(Marshal.GetLastWin32Error());
                return new AgentCredentials(generated);
            }
            finally
            {
                CryptographicOperations.ZeroMemory(generated);
                Marshal.Copy(generated, 0, memory, generated.Length);
                Marshal.FreeHGlobal(memory);
            }
        }
        finally { mutex.ReleaseMutex(); }
    }

    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    private struct NativeCredential
    {
        public uint Flags;
        public uint Type;
        public string? TargetName;
        public string? Comment;
        public System.Runtime.InteropServices.ComTypes.FILETIME LastWritten;
        public uint BlobSize;
        public IntPtr Blob;
        public uint Persist;
        public uint AttributeCount;
        public IntPtr Attributes;
        public string? TargetAlias;
        public string? UserName;
    }
    [DllImport("advapi32.dll", EntryPoint = "CredReadW", CharSet = CharSet.Unicode, SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool CredRead(string target, uint type, uint flags, out IntPtr credential);
    [DllImport("advapi32.dll", EntryPoint = "CredWriteW", CharSet = CharSet.Unicode, SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool CredWrite(ref NativeCredential credential, uint flags);
    [DllImport("advapi32.dll")]
    private static extern void CredFree(IntPtr buffer);
}