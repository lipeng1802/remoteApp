using System.Buffers.Binary;

namespace RemoteProtocol;

public readonly record struct MouseMovePayload(ushort X, ushort Y)
{
    public byte[] Encode()
    {
        var bytes = new byte[4];
        BinaryPrimitives.WriteUInt16BigEndian(bytes, X);
        BinaryPrimitives.WriteUInt16BigEndian(bytes.AsSpan(2), Y);
        return bytes;
    }
    public static MouseMovePayload Decode(ReadOnlySpan<byte> bytes)
    {
        if (bytes.Length != 4) throw MousePayload.Invalid();
        return new(BinaryPrimitives.ReadUInt16BigEndian(bytes), BinaryPrimitives.ReadUInt16BigEndian(bytes[2..]));
    }
    public (int X, int Y) ToPhysicalPixels(int width, int height)
    {
        if (width is < 1 or > 16384 || height is < 1 or > 16384) throw MousePayload.Invalid();
        return ((int)Math.Round(X * (width - 1) / 65535.0, MidpointRounding.AwayFromZero),
                (int)Math.Round(Y * (height - 1) / 65535.0, MidpointRounding.AwayFromZero));
    }
}

public enum MouseButton : byte { Left = 1, Right = 2, Middle = 3 }
public enum ButtonAction : byte { Down = 1, Up = 2 }
public readonly record struct MouseButtonPayload(MouseButton Button, ButtonAction Action)
{
    public byte[] Encode()
    {
        var bytes = new[] { (byte)Button, (byte)Action };
        _ = Decode(bytes);
        return bytes;
    }
    public static MouseButtonPayload Decode(ReadOnlySpan<byte> bytes)
    {
        if (bytes.Length != 2 || bytes[0] is < 1 or > 3 || bytes[1] is < 1 or > 2)
            throw MousePayload.Invalid();
        return new((MouseButton)bytes[0], (ButtonAction)bytes[1]);
    }
}

public readonly record struct MouseWheelPayload(int Horizontal, int Vertical)
{
    public byte[] Encode()
    {
        var bytes = new byte[8];
        BinaryPrimitives.WriteInt32BigEndian(bytes, Horizontal);
        BinaryPrimitives.WriteInt32BigEndian(bytes.AsSpan(4), Vertical);
        return bytes;
    }
    public static MouseWheelPayload Decode(ReadOnlySpan<byte> bytes)
    {
        if (bytes.Length != 8) throw MousePayload.Invalid();
        return new(BinaryPrimitives.ReadInt32BigEndian(bytes), BinaryPrimitives.ReadInt32BigEndian(bytes[4..]));
    }
}

public static class MouseCoordinates
{
    // Coordinates use a top-left origin in the same logical units as the viewport.
    // DPI is not applied here: normalized values target physical SCREEN_INFO pixels.
    public static bool TryMap(double x, double y, double viewWidth, double viewHeight,
        int screenWidth, int screenHeight, out MouseMovePayload point)
    {
        point = default;
        if (!double.IsFinite(x) || !double.IsFinite(y) || !double.IsFinite(viewWidth) ||
            !double.IsFinite(viewHeight) || viewWidth <= 0 || viewHeight <= 0 ||
            screenWidth is < 1 or > 16384 || screenHeight is < 1 or > 16384) return false;
        var scale = Math.Min(viewWidth / screenWidth, viewHeight / screenHeight);
        var width = screenWidth * scale;
        var height = screenHeight * scale;
        if (width <= 0 || height <= 0) return false;
        var left = (viewWidth - width) / 2;
        var top = (viewHeight - height) / 2;
        if (x < left || x > left + width || y < top || y > top + height) return false;
        point = new((ushort)Math.Round(Math.Clamp((x - left) / width, 0, 1) * 65535, MidpointRounding.AwayFromZero),
                    (ushort)Math.Round(Math.Clamp((y - top) / height, 0, 1) * 65535, MidpointRounding.AwayFromZero));
        return true;
    }
}

internal static class MousePayload
{
    internal static ProtocolException Invalid() => new(ProtocolError.InvalidPayload, "Invalid mouse payload.");
}
