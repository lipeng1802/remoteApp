using System.Buffers.Binary;

namespace RemoteProtocol;

public enum KeyAction : byte { Down = 1, Up = 2 }

// Basic set-1 make code only; E0 is a separate flag. E1/Pause sequences are not supported.
public readonly record struct KeyEventPayload(ushort ScanCode, bool Extended, KeyAction Action)
{
    public byte[] Encode()
    {
        if (ScanCode is < 1 or > 0x7f || Action is not (KeyAction.Down or KeyAction.Up)) throw Invalid();
        var bytes = new byte[4];
        BinaryPrimitives.WriteUInt16BigEndian(bytes, ScanCode);
        bytes[2] = Extended ? (byte)1 : (byte)0;
        bytes[3] = (byte)Action;
        return bytes;
    }

    public static KeyEventPayload Decode(ReadOnlySpan<byte> bytes)
    {
        if (bytes.Length != 4 || bytes[2] > 1 || bytes[3] is < 1 or > 2) throw Invalid();
        var code = BinaryPrimitives.ReadUInt16BigEndian(bytes);
        if (code is < 1 or > 0x7f) throw Invalid();
        return new(code, bytes[2] == 1, (KeyAction)bytes[3]);
    }

    private static ProtocolException Invalid() => new(ProtocolError.InvalidPayload, "Invalid keyboard payload.");
}
