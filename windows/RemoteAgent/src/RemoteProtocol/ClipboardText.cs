using System.Text;

namespace RemoteProtocol;

public enum ClipboardTextStatus : byte
{
    Success = 0,
    Unavailable = 1,
    TooLarge = 2,
}

public sealed record ClipboardTextPayload(ClipboardTextStatus Status, string Text)
{
    public const int MaximumTextBytes = 32 * 1024;

    public byte[] Encode()
    {
        if (!Enum.IsDefined(Status) || (Status != ClipboardTextStatus.Success && Text.Length != 0))
            throw Invalid();
        var text = Encoding.UTF8.GetBytes(Text);
        if (text.Length > MaximumTextBytes) throw Invalid();
        var output = new byte[text.Length + 1];
        output[0] = (byte)Status;
        text.CopyTo(output, 1);
        return output;
    }

    public static ClipboardTextPayload Decode(ReadOnlySpan<byte> payload)
    {
        if (payload.Length is < 1 or > MaximumTextBytes + 1 ||
            !Enum.IsDefined(typeof(ClipboardTextStatus), payload[0])) throw Invalid();
        var status = (ClipboardTextStatus)payload[0];
        if (status != ClipboardTextStatus.Success && payload.Length != 1) throw Invalid();
        try
        {
            var text = status == ClipboardTextStatus.Success
                ? new UTF8Encoding(false, true).GetString(payload[1..])
                : string.Empty;
            return new ClipboardTextPayload(status, text);
        }
        catch (DecoderFallbackException) { throw Invalid(); }
    }

    private static ProtocolException Invalid() =>
        new(ProtocolError.InvalidPayload, "Invalid clipboard text payload.");
}
