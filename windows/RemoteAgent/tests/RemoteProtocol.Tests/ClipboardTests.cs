using RemoteProtocol;

internal static class ClipboardTests
{
    public static void Payloads()
    {
        var value = new ClipboardTextPayload(ClipboardTextStatus.Success, "Hello，世界\n");
        Check(ClipboardTextPayload.Decode(value.Encode()) == value, "Unicode clipboard round trip failed.");
        foreach (var status in new[] { ClipboardTextStatus.Unavailable, ClipboardTextStatus.TooLarge })
        {
            var payload = new ClipboardTextPayload(status, string.Empty);
            Check(ClipboardTextPayload.Decode(payload.Encode()) == payload, "Clipboard status round trip failed.");
        }
        Reject([]);
        Reject([9]);
        Reject([(byte)ClipboardTextStatus.Unavailable, 1]);
        Reject([(byte)ClipboardTextStatus.Success, 0xff]);
        try
        {
            _ = new ClipboardTextPayload(ClipboardTextStatus.Success,
                new string('a', ClipboardTextPayload.MaximumTextBytes + 1)).Encode();
            throw new Exception("Oversized clipboard text was accepted.");
        }
        catch (ProtocolException exception) when (exception.Error == ProtocolError.InvalidPayload) { }
    }

    private static void Reject(byte[] payload)
    {
        try { _ = ClipboardTextPayload.Decode(payload); }
        catch (ProtocolException exception) when (exception.Error == ProtocolError.InvalidPayload) { return; }
        throw new Exception("Malformed clipboard payload was accepted.");
    }

    private static void Check(bool value, string message)
    {
        if (!value) throw new Exception(message);
    }
}
