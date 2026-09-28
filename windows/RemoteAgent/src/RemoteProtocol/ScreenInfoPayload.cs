using System.Buffers.Binary;

namespace RemoteProtocol;

public sealed record ScreenInfoPayload(uint Width, uint Height, uint DpiX100, uint DpiY100)
{
    public byte[] Encode()
    {
        Validate();
        var data = new byte[18];
        BinaryPrimitives.WriteUInt32BigEndian(data, Width);
        BinaryPrimitives.WriteUInt32BigEndian(data.AsSpan(4), Height);
        BinaryPrimitives.WriteUInt32BigEndian(data.AsSpan(8), DpiX100);
        BinaryPrimitives.WriteUInt32BigEndian(data.AsSpan(12), DpiY100);
        data[16] = 1;
        return data;
    }
    public static ScreenInfoPayload Decode(ReadOnlySpan<byte> data)
    {
        if (data.Length != 18 || data[16] != 1 || data[17] != 0) throw Invalid();
        var info = new ScreenInfoPayload(BinaryPrimitives.ReadUInt32BigEndian(data),
            BinaryPrimitives.ReadUInt32BigEndian(data[4..]), BinaryPrimitives.ReadUInt32BigEndian(data[8..]),
            BinaryPrimitives.ReadUInt32BigEndian(data[12..]));
        info.Validate();
        return info;
    }
    private void Validate()
    {
        if (Width is 0 or > 16384 || Height is 0 or > 16384 ||
            DpiX100 is < 4800 or > 96000 || DpiY100 is < 4800 or > 96000) throw Invalid();
    }
    private static ProtocolException Invalid() => new(ProtocolError.InvalidPayload, "Invalid screen metadata.");
}

public sealed record CapturedJpeg(ScreenInfoPayload Screen, byte[] Jpeg);
public interface IJpegFrameSource : IDisposable
{
    CapturedJpeg Capture(CancellationToken cancellationToken);
}